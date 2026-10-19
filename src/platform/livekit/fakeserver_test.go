package livekit

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/webrtc/v3"
	"nhooyr.io/websocket"
)

// fakeLiveKit is a LiveKit signaling server for the adapter's tests: one
// room, real Pion PeerConnections on the server side (a publish one that
// answers, a subscribe one that offers), media forwarded from publishers to
// the subscribers that asked for each track. It sends server candidates in
// its SDPs and, like the real server, trickles the subscriber's before its
// first offer.
type fakeLiveKit struct {
	t      *testing.T
	key    string
	secret string
	srv    *httptest.Server

	mu           sync.Mutex
	participants map[string]*fakeParticipant // By identity
	nextID       int

	dropPongs atomic.Bool  // Stop answering anything (liveness test)
	leaves    chan string  // Identities that sent a LeaveRequest
	joins     atomic.Int32 // Accepted joins
	refused   atomic.Int32 // Joins refused for their token
	offers    atomic.Int32 // Publish offers received
	refuseSub atomic.Value // Track SID whose subscription is refused
}

type fakeTrack struct {
	sid   string
	cid   string
	kind  int32
	owner *fakeParticipant
	local *webrtc.TrackLocalStaticRTP // Forwarded copy, once media flows
}

type fakeParticipant struct {
	s        *fakeLiveKit
	order    int // Join order: the JoinResponse lists others by it
	sid      string
	identity string
	conn     *websocket.Conn
	ctx      context.Context
	wmu      sync.Mutex
	pub      *webrtc.PeerConnection
	sub      *webrtc.PeerConnection

	mu          sync.Mutex
	tracks      map[string]*fakeTrack        // Published, by cid
	senders     map[string]*webrtc.RTPSender // Subscribed track SID -> sender on sub
	negotiating bool
	pending     bool
	offerID     uint32
	gone        bool
}

func newFakeLiveKit(t *testing.T) *fakeLiveKit {
	t.Helper()
	s := &fakeLiveKit{t: t, key: "devkey", secret: "fake-secret-0123456789abcdef0123", participants: map[string]*fakeParticipant{}, leaves: make(chan string, 64)}
	s.refuseSub.Store("")
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(func() {
		s.mu.Lock()
		ps := make([]*fakeParticipant, 0, len(s.participants))
		for _, p := range s.participants {
			ps = append(ps, p)
		}
		s.mu.Unlock()
		for _, p := range ps {
			p.close()
		}
		s.srv.Close()
	})
	return s
}

// config returns the platform_config of the fake.
func (s *fakeLiveKit) config(room string) map[string]any {
	return map[string]any{"url": s.srv.URL, "room": room, "api_key": s.key, "api_secret": s.secret}
}

func (s *fakeLiveKit) participant(identity string) *fakeParticipant {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.participants[identity]
}

// checkToken verifies the HS256 token and returns its identity.
func (s *fakeLiveKit) checkToken(tok string) (string, error) {
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	mac := hmac.New(sha256.New, []byte(s.secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	if base64.RawURLEncoding.EncodeToString(mac.Sum(nil)) != parts[2] {
		return "", errors.New("bad signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var c tokenClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return "", err
	}
	if c.Issuer != s.key || !c.Video.RoomJoin || c.Subject == "" || time.Now().Unix() > c.Expires {
		return "", errors.New("bad claims")
	}
	return c.Subject, nil
}

func (s *fakeLiveKit) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/rtc" || r.URL.Query().Get("auto_subscribe") != "0" {
		http.Error(w, "unexpected join URL", http.StatusBadRequest)
		return
	}
	identity, err := s.checkToken(r.URL.Query().Get("access_token"))
	if err != nil {
		s.refused.Add(1)
		http.Error(w, "invalid token: "+err.Error(), http.StatusUnauthorized)
		return
	}
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	conn.SetReadLimit(readLimit)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := &fakeParticipant{s: s, identity: identity, conn: conn, ctx: ctx, tracks: map[string]*fakeTrack{}, senders: map[string]*webrtc.RTPSender{}}
	if err := p.start(); err != nil {
		s.t.Errorf("fake LiveKit: %v", err)
		conn.Close(websocket.StatusInternalError, "")
		return
	}
	s.joins.Add(1)
	p.readLoop()
	p.close()
}

func (p *fakeParticipant) send(r signalResponse) {
	b := encodeSignalResponse(r)
	p.wmu.Lock()
	defer p.wmu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p.conn.Write(ctx, websocket.MessageBinary, b)
}

// info is the participant as others see it: tracks once their media flows.
func (p *fakeParticipant) info() participantInfo {
	p.mu.Lock()
	defer p.mu.Unlock()
	pi := participantInfo{SID: p.sid, Identity: p.identity, State: stateActive}
	if p.gone {
		pi.State = stateDisconnected
		return pi
	}
	for _, t := range p.tracks {
		if t.local != nil {
			src := sourceMicrophone
			if t.kind == trackVideo {
				src = sourceCamera
			}
			pi.Tracks = append(pi.Tracks, trackInfo{SID: t.sid, Type: t.kind, Source: src})
		}
	}
	return pi
}

// start registers the participant, sends the JoinResponse, then trickles
// subscriber candidates and the first subscriber offer.
func (p *fakeParticipant) start() error {
	var err error
	if p.pub, err = webrtc.NewPeerConnection(webrtc.Configuration{}); err != nil {
		return err
	}
	if p.sub, err = webrtc.NewPeerConnection(webrtc.Configuration{}); err != nil {
		return err
	}
	p.pub.OnTrack(p.onPublishedTrack)
	if _, err := p.sub.CreateDataChannel("_reliable", nil); err != nil {
		return err
	}

	s := p.s
	s.mu.Lock()
	s.nextID++
	p.order = s.nextID
	p.sid = fmt.Sprintf("PA_%d", s.nextID)
	if old := s.participants[p.identity]; old != nil {
		s.mu.Unlock()
		return fmt.Errorf("duplicate identity %s", p.identity)
	}
	var others []*fakeParticipant
	for _, o := range s.participants {
		others = append(others, o)
	}
	s.participants[p.identity] = p
	s.mu.Unlock()
	sort.Slice(others, func(i, j int) bool { return others[i].order < others[j].order })

	join := &joinResponse{Participant: participantInfo{SID: p.sid, Identity: p.identity, State: stateJoined}, SubscriberPrimary: true, PingInterval: 1, PingTimeout: 3, ServerVersion: "fake", ServerProtocol: 17}
	for _, o := range others {
		join.OtherParticipants = append(join.OtherParticipants, o.info())
	}
	p.send(signalResponse{Join: join})
	for _, o := range others {
		o.send(signalResponse{Update: &participantUpdate{Participants: []participantInfo{{SID: p.sid, Identity: p.identity, State: stateJoined}}}})
	}
	p.renegotiate(true)
	return nil
}

// onPublishedTrack forwards a publisher's track and announces it.
func (p *fakeParticipant) onPublishedTrack(remote *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	p.mu.Lock()
	t := p.tracks[remote.ID()]
	p.mu.Unlock()
	if t == nil {
		p.s.t.Errorf("fake LiveKit: track %q of %s was not announced with AddTrackRequest", remote.ID(), p.identity)
		return
	}
	local, err := webrtc.NewTrackLocalStaticRTP(remote.Codec().RTPCodecCapability, t.sid, p.sid)
	if err != nil {
		return
	}
	p.mu.Lock()
	t.local = local
	p.mu.Unlock()
	p.s.broadcast(p)
	go func() {
		for {
			pkt, _, err := remote.ReadRTP()
			if err != nil {
				return
			}
			local.WriteRTP(pkt)
		}
	}()
}

// broadcast sends p's info to everyone else.
func (s *fakeLiveKit) broadcast(p *fakeParticipant) {
	info := p.info()
	s.mu.Lock()
	var others []*fakeParticipant
	for _, o := range s.participants {
		if o != p {
			others = append(others, o)
		}
	}
	s.mu.Unlock()
	for _, o := range others {
		o.send(signalResponse{Update: &participantUpdate{Participants: []participantInfo{info}}})
	}
}

// renegotiate sends a subscriber offer, or marks one pending while the
// previous is unanswered. The first trickles its candidates before it.
func (p *fakeParticipant) renegotiate(first bool) {
	p.mu.Lock()
	if p.negotiating {
		p.pending = true
		p.mu.Unlock()
		return
	}
	p.negotiating = true
	p.offerID++
	id := p.offerID
	p.mu.Unlock()

	offer, err := p.sub.CreateOffer(nil)
	if err != nil {
		return
	}
	gathered := webrtc.GatheringCompletePromise(p.sub)
	if err := p.sub.SetLocalDescription(offer); err != nil {
		return
	}
	<-gathered
	sdp := p.sub.LocalDescription().SDP
	if first {
		for _, line := range strings.Split(sdp, "\r\n") {
			if c, ok := strings.CutPrefix(line, "a=candidate:"); ok {
				b, _ := json.Marshal(map[string]any{"candidate": "candidate:" + c, "sdpMid": "0", "sdpMLineIndex": 0})
				p.send(signalResponse{Trickle: &trickleRequest{CandidateInit: string(b), Target: targetSubscriber}})
			}
		}
	}
	p.send(signalResponse{Offer: &sessionDescription{Type: "offer", SDP: sdp, ID: id}})
}

func (p *fakeParticipant) readLoop() {
	for {
		_, data, err := p.conn.Read(p.ctx)
		if err != nil {
			return
		}
		r, err := decodeSignalRequest(data)
		if err != nil {
			p.s.t.Errorf("fake LiveKit: undecodable request: %v", err)
			return
		}
		if p.s.dropPongs.Load() && r.PingReq != nil {
			continue
		}
		switch {
		case r.AddTrack != nil:
			p.s.mu.Lock()
			p.s.nextID++
			sid := fmt.Sprintf("TR_%d", p.s.nextID)
			p.s.mu.Unlock()
			if (r.AddTrack.Type == trackVideo) != (r.AddTrack.Width > 0 && r.AddTrack.Height > 0) {
				p.s.t.Errorf("fake LiveKit: add track %+v: video needs a size, audio none", *r.AddTrack)
			}
			if r.AddTrack.Type == trackAudio && !r.AddTrack.DisableRed {
				p.s.t.Errorf("fake LiveKit: audio track without disable_red")
			}
			p.mu.Lock()
			p.tracks[r.AddTrack.CID] = &fakeTrack{sid: sid, cid: r.AddTrack.CID, kind: r.AddTrack.Type, owner: p}
			p.mu.Unlock()
			p.send(signalResponse{TrackPublished: &trackPublishedResponse{CID: r.AddTrack.CID, Track: trackInfo{SID: sid, Type: r.AddTrack.Type}}})
		case r.Offer != nil:
			p.s.offers.Add(1)
			p.mu.Lock()
			announced := len(p.tracks)
			p.mu.Unlock()
			if announced == 0 {
				p.s.t.Errorf("fake LiveKit: publish offer before any AddTrackRequest")
			}
			if err := p.pub.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: r.Offer.SDP}); err != nil {
				p.s.t.Errorf("fake LiveKit: publish offer: %v", err)
				continue
			}
			ans, _ := p.pub.CreateAnswer(nil)
			gathered := webrtc.GatheringCompletePromise(p.pub)
			p.pub.SetLocalDescription(ans)
			<-gathered
			p.send(signalResponse{Answer: &sessionDescription{Type: "answer", SDP: p.pub.LocalDescription().SDP, ID: r.Offer.ID}})
		case r.Answer != nil:
			if err := p.sub.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: r.Answer.SDP}); err != nil {
				p.s.t.Errorf("fake LiveKit: subscriber answer: %v", err)
			}
			p.mu.Lock()
			p.negotiating = false
			again := p.pending
			p.pending = false
			p.mu.Unlock()
			if again {
				p.renegotiate(false)
			}
		case r.Trickle != nil:
			var c webrtc.ICECandidateInit
			json.Unmarshal([]byte(r.Trickle.CandidateInit), &c)
			pc := p.pub
			if r.Trickle.Target == targetSubscriber {
				pc = p.sub
			}
			pc.AddICECandidate(c)
		case r.Subscription != nil:
			p.subscribe(r.Subscription.TrackSIDs)
		case r.Leave != nil:
			p.s.leaves <- p.identity
			return
		case r.PingReq != nil:
			p.send(signalResponse{Pong: true})
		}
	}
}

// subscribe adds the forwarded tracks to the subscriber session.
func (p *fakeParticipant) subscribe(sids []string) {
	refuse, _ := p.s.refuseSub.Load().(string)
	added := false
	for _, sid := range sids {
		if sid == refuse {
			p.send(signalResponse{SubscriptionResponse: &subscriptionResponse{TrackSID: sid, Err: 2}})
			continue
		}
		t := p.s.track(sid)
		if t == nil || t.local == nil {
			p.send(signalResponse{SubscriptionResponse: &subscriptionResponse{TrackSID: sid, Err: 2}})
			continue
		}
		sender, err := p.sub.AddTrack(t.local)
		if err != nil {
			continue
		}
		go func() { // Drain RTCP for the interceptors
			for {
				if _, _, err := sender.ReadRTCP(); err != nil {
					return
				}
			}
		}()
		p.mu.Lock()
		p.senders[sid] = sender
		p.mu.Unlock()
		added = true
	}
	if added {
		p.renegotiate(false)
	}
}

// track finds a published track by SID.
func (s *fakeLiveKit) track(sid string) *fakeTrack {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.participants {
		p.mu.Lock()
		for _, t := range p.tracks {
			if t.sid == sid {
				p.mu.Unlock()
				return t
			}
		}
		p.mu.Unlock()
	}
	return nil
}

// subscribedTo returns the track SIDs p is subscribed to.
func (p *fakeParticipant) subscribedTo() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var sids []string
	for sid := range p.senders {
		sids = append(sids, sid)
	}
	return sids
}

// close removes the participant: its tracks leave the subscribers that had
// them and everyone is told.
func (p *fakeParticipant) close() {
	p.mu.Lock()
	if p.gone {
		p.mu.Unlock()
		return
	}
	p.gone = true
	mine := map[string]bool{}
	for _, t := range p.tracks {
		mine[t.sid] = true
	}
	p.mu.Unlock()
	s := p.s
	s.mu.Lock()
	if s.participants[p.identity] == p {
		delete(s.participants, p.identity)
	}
	var others []*fakeParticipant
	for _, o := range s.participants {
		others = append(others, o)
	}
	s.mu.Unlock()
	for _, o := range others {
		o.mu.Lock()
		var removed []*webrtc.RTPSender
		for sid, sender := range o.senders {
			if mine[sid] {
				removed = append(removed, sender)
				delete(o.senders, sid)
			}
		}
		o.mu.Unlock()
		for _, sender := range removed {
			o.sub.RemoveTrack(sender)
		}
		o.send(signalResponse{Update: &participantUpdate{Participants: []participantInfo{{SID: p.sid, Identity: p.identity, State: stateDisconnected}}}})
		if len(removed) > 0 {
			o.renegotiate(false)
		}
	}
	p.conn.Close(websocket.StatusNormalClosure, "")
	p.pub.Close()
	p.sub.Close()
}

// kick closes a participant's socket without a LeaveRequest.
func (s *fakeLiveKit) kick(identity string) {
	if p := s.participant(identity); p != nil {
		p.conn.Close(websocket.StatusGoingAway, "kicked")
	}
}

// sendLeave sends a server LeaveRequest to a participant.
func (s *fakeLiveKit) sendLeave(identity string) {
	if p := s.participant(identity); p != nil {
		p.send(signalResponse{Leave: &leaveRequest{Reason: reasonServerShutdown}})
	}
}
