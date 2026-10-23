package janus

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pion/logging"
	"github.com/pion/webrtc/v3"
	"nhooyr.io/websocket"

	"webrtc-load-engine/src/platform"
)

// fakeJanus is an httptest WebSocket server that speaks Janus 1.4.3's API
// and VideoRoom plugin as T002 recorded them (research R11), with real Pion
// PeerConnections: it answers publish offers, offers subscriptions (the
// dummy publisher's silent audio section first) and forwards publishers'
// RTP to the sections that subscribe to them.
type fakeJanus struct {
	t   *testing.T
	srv *httptest.Server

	mu             sync.Mutex
	nextID         uint64
	sessions       map[uint64]*fakeSession
	rooms          map[uint64]*fakeRoom
	created        []map[string]any       // Room create bodies, in order
	fail           map[string]*janusError // VideoRoom request -> error to answer
	apiSecret      string                 // Required on every request if set
	adminKey       string                 // Required on room create if set
	sessionTimeout time.Duration          // 0: sessions never expire
	updating       int                    // update requests received while an offer was unanswered
	answers        map[string]string      // Latest subscriber answer by display
	refuseStatus   atomic.Int32           // Refuse handshakes with this status (0: accept)
	destroys       atomic.Int32           // destroy requests received
	subprotocols   chan string            // Handshake subprotocols
	logger         logging.LoggerFactory  // Quiet Pion logs
	done           chan struct{}          // Closed with the server
}

type fakeSession struct {
	id       uint64
	conn     *websocket.Conn
	writeMu  sync.Mutex
	handles  map[uint64]*fakeHandle
	lastSeen time.Time
	frames   []string // Raw frames the client sent
	gone     bool
}

type fakeRoom struct {
	id      uint64
	max     int
	pin     string
	dummyID uint64 // 0: no dummy publisher
	members []*fakeHandle
}

type fakeHandle struct {
	id      uint64
	sess    *fakeSession
	room    *fakeRoom
	ptype   string // publisher or subscriber; "" before join
	pid     uint64 // Participant ID (publishers)
	display string

	pcMu    sync.Mutex // Serializes this handle's negotiations
	pc      *webrtc.PeerConnection
	pending []webrtc.ICECandidateInit // Trickled before the remote description

	// Publisher
	publishing bool
	streams    []PublisherStream
	fans       map[string]*fanout // By the publisher's mid

	// Subscriber
	owner     *fakeHandle // The publisher handle of the same session
	subs      []*fakeSub
	offerOut  bool        // An offer was sent and not answered yet
	queued    []updateReq // Updates received while offerOut
	updates   []updateReq // Every update request, in order
	connected chan struct{}
}

type fanout struct {
	mu      sync.Mutex
	targets []*fakeSub
}

type fakeSub struct {
	feed     uint64
	feedMID  string
	kind     string
	display  string // The feed's display
	track    *webrtc.TrackLocalStaticRTP
	sender   *webrtc.RTPSender
	tr       *webrtc.RTPTransceiver
	answered atomic.Bool // The bot answered an offer with this section
}

func newFakeJanus(t *testing.T) *fakeJanus {
	t.Helper()
	lf := logging.NewDefaultLoggerFactory()
	lf.DefaultLogLevel = logging.LogLevelDisabled
	s := &fakeJanus{
		t: t, nextID: 1000, sessions: map[uint64]*fakeSession{}, rooms: map[uint64]*fakeRoom{},
		fail: map[string]*janusError{}, answers: map[string]string{}, subprotocols: make(chan string, 64), logger: lf,
		done: make(chan struct{}),
	}
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(s.close)
	go s.expireLoop()
	return s
}

func (s *fakeJanus) url() string { return "ws" + strings.TrimPrefix(s.srv.URL, "http") }

func (s *fakeJanus) config(room uint64) map[string]any {
	return map[string]any{"url": s.url(), "room": float64(room)}
}

func (s *fakeJanus) close() {
	close(s.done)
	s.srv.CloseClientConnections()
	s.srv.Close()
	s.mu.Lock()
	var hs []*fakeHandle
	for _, sess := range s.sessions {
		for _, h := range sess.handles {
			hs = append(hs, h)
		}
	}
	s.mu.Unlock()
	for _, h := range hs {
		if h.pc != nil {
			h.pc.Close()
		}
	}
}

func (s *fakeJanus) id() uint64 {
	s.nextID++
	return s.nextID * 7919 // Large, distinct IDs, as Janus's
}

func (s *fakeJanus) serve(w http.ResponseWriter, r *http.Request) {
	s.subprotocols <- r.Header.Get("Sec-WebSocket-Protocol")
	if st := s.refuseStatus.Load(); st != 0 {
		http.Error(w, "refused", int(st))
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{subprotocol}})
	if err != nil {
		return
	}
	sess := &fakeSession{conn: c, handles: map[uint64]*fakeHandle{}, lastSeen: time.Now()}
	defer s.dropSession(sess)
	for {
		_, b, err := c.Read(context.Background())
		if err != nil {
			return
		}
		var f map[string]any
		if json.Unmarshal(b, &f) != nil {
			continue
		}
		s.mu.Lock()
		sess.frames = append(sess.frames, string(b))
		sess.lastSeen = time.Now()
		s.mu.Unlock()
		s.handle(sess, f)
	}
}

func (s *fakeJanus) send(sess *fakeSession, m any) {
	b, _ := json.Marshal(m)
	sess.writeMu.Lock()
	defer sess.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sess.conn.Write(ctx, websocket.MessageText, b)
}

func num(v any) uint64 {
	f, _ := v.(float64)
	return uint64(f)
}

// handle answers one client frame.
func (s *fakeJanus) handle(sess *fakeSession, f map[string]any) {
	tx := f["transaction"]
	reply := func(m map[string]any) {
		m["transaction"] = tx
		if sess.id != 0 {
			m["session_id"] = sess.id
		}
		s.send(sess, m)
	}
	if s.apiSecret != "" && f["apisecret"] != s.apiSecret {
		reply(map[string]any{"janus": "error", "error": map[string]any{"code": 403, "reason": "Unauthorized request (wrong or missing secret/token)"}})
		return
	}
	switch f["janus"] {
	case "create":
		s.mu.Lock()
		sess.id = s.id()
		s.sessions[sess.id] = sess
		s.mu.Unlock()
		reply(map[string]any{"janus": "success", "data": map[string]any{"id": sess.id}})
	case "attach":
		s.mu.Lock()
		h := &fakeHandle{id: s.id(), sess: sess, fans: map[string]*fanout{}, connected: make(chan struct{})}
		sess.handles[h.id] = h
		s.mu.Unlock()
		reply(map[string]any{"janus": "success", "data": map[string]any{"id": h.id}})
	case "keepalive":
		reply(map[string]any{"janus": "ack"})
	case "trickle":
		reply(map[string]any{"janus": "ack"})
		s.trickle(sess, num(f["handle_id"]), f["candidate"])
	case "destroy":
		s.destroys.Add(1)
		reply(map[string]any{"janus": "success"})
		s.dropSession(sess)
	case "message":
		s.mu.Lock()
		h := sess.handles[num(f["handle_id"])]
		s.mu.Unlock()
		body, _ := f["body"].(map[string]any)
		var jsep *JSEP
		if j, ok := f["jsep"].(map[string]any); ok {
			jsep = &JSEP{Type: j["type"].(string), SDP: j["sdp"].(string)}
		}
		if h == nil {
			reply(map[string]any{"janus": "error", "error": map[string]any{"code": 459, "reason": "No such handle"}})
			return
		}
		s.message(h, body, jsep, reply)
	}
}

func pluginReply(janus string, h *fakeHandle, data map[string]any, jsep *JSEP) map[string]any {
	m := map[string]any{"janus": janus, "sender": h.id, "plugindata": map[string]any{"plugin": pluginName, "data": data}}
	if jsep != nil {
		m["jsep"] = jsep
	}
	return m
}

func vrError(code int, reason string) map[string]any {
	return map[string]any{"videoroom": "event", "error_code": code, "error": reason}
}

// message runs a VideoRoom request: create is synchronous, the others get
// an ack then their event.
func (s *fakeJanus) message(h *fakeHandle, body map[string]any, jsep *JSEP, reply func(map[string]any)) {
	req, _ := body["request"].(string)
	s.mu.Lock()
	fail := s.fail[req]
	s.mu.Unlock()
	if req == "create" {
		data := s.createRoom(body)
		if fail != nil {
			data = vrError(fail.Code, fail.Reason)
		}
		reply(pluginReply("success", h, data, nil))
		return
	}
	reply(map[string]any{"janus": "ack"})
	event := func(data map[string]any, j *JSEP) { reply(pluginReply("event", h, data, j)) }
	if fail != nil {
		event(vrError(fail.Code, fail.Reason), nil)
		return
	}
	switch req {
	case "join":
		if body["ptype"] == "subscriber" {
			s.joinSubscriber(h, body, event)
		} else {
			s.joinPublisher(h, body, event)
		}
	case "publish":
		s.publish(h, jsep, event)
	case "start":
		s.start(h, jsep, event)
	case "update":
		s.update(h, body, event)
	case "leave":
		event(map[string]any{"videoroom": "event", "room": roomID0(h), "leaving": "ok"}, nil)
		s.leave(h)
	default:
		event(vrError(423, "Unknown request '"+req+"'"), nil)
	}
}

func roomID0(h *fakeHandle) uint64 {
	if h.room == nil {
		return 0
	}
	return h.room.id
}

func (s *fakeJanus) createRoom(body map[string]any) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, body)
	id := num(body["room"])
	if s.adminKey != "" && body["admin_key"] != s.adminKey {
		return vrError(433, "Unauthorized (wrong admin_key)")
	}
	if _, ok := s.rooms[id]; ok {
		return vrError(427, "Room "+jsonNum(id)+" already exists")
	}
	r := &fakeRoom{id: id, max: int(num(body["publishers"]))}
	r.pin, _ = body["pin"].(string)
	if body["dummy_publisher"] == true {
		r.dummyID = s.id()
	}
	s.rooms[id] = r
	return map[string]any{"videoroom": "created", "room": id, "permanent": false}
}

func jsonNum(id uint64) string { b, _ := json.Marshal(id); return string(b) }

// prepareRoom creates a room as another client would have, before the test.
func (s *fakeJanus) prepareRoom(id uint64, max int, dummy bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r := &fakeRoom{id: id, max: max}
	if dummy {
		r.dummyID = s.id()
	}
	s.rooms[id] = r
}

func (r *fakeRoom) publisherList(except *fakeHandle) []map[string]any {
	var list []map[string]any
	if r.dummyID != 0 {
		list = append(list, map[string]any{"id": r.dummyID, "display": "Dummy publisher", "dummy": true,
			"streams": []map[string]any{{"type": "audio", "mindex": 0, "mid": "0", "codec": "opus"}}})
	}
	for _, m := range r.members {
		if m != except && m.publishing {
			list = append(list, m.publisherInfo())
		}
	}
	return list
}

func (h *fakeHandle) publisherInfo() map[string]any {
	return map[string]any{"id": h.pid, "display": h.display, "streams": h.streams}
}

func (s *fakeJanus) joinPublisher(h *fakeHandle, body map[string]any, event func(map[string]any, *JSEP)) {
	s.mu.Lock()
	r := s.rooms[num(body["room"])]
	switch {
	case r == nil:
		s.mu.Unlock()
		event(vrError(426, "No such room"), nil)
		return
	case r.pin != "" && body["pin"] != r.pin:
		s.mu.Unlock()
		event(vrError(433, "Unauthorized (wrong pin)"), nil)
		return
	}
	h.room, h.ptype, h.pid = r, "publisher", s.id()
	h.display, _ = body["display"].(string)
	r.members = append(r.members, h)
	pubs := r.publisherList(h)
	s.mu.Unlock()
	event(map[string]any{"videoroom": "joined", "room": r.id, "id": h.pid, "private_id": h.pid + 1, "publishers": pubs}, nil)
}

// notify sends an event to every publisher handle of r but except.
func (s *fakeJanus) notify(r *fakeRoom, except *fakeHandle, data map[string]any) {
	s.mu.Lock()
	var to []*fakeHandle
	for _, m := range r.members {
		if m != except && !m.sess.gone {
			to = append(to, m)
		}
	}
	s.mu.Unlock()
	for _, m := range to {
		s.send(m.sess, pluginReply("event", m, data, nil))
	}
}

func (s *fakeJanus) newPC() (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, PayloadType: 111}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, err
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, PayloadType: 96}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	var se webrtc.SettingEngine
	se.LoggerFactory = s.logger
	se.SetIncludeLoopbackCandidate(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetIPFilter(func(ip net.IP) bool { return ip.IsLoopback() })
	return webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
}

// trickle applies a candidate the bot trickled, or keeps it until the
// handle's remote description is set.
func (s *fakeJanus) trickle(sess *fakeSession, handle uint64, c any) {
	cm, _ := c.(map[string]any)
	cand, _ := cm["candidate"].(string)
	if cand == "" {
		return // completed
	}
	mid, _ := cm["sdpMid"].(string)
	init := webrtc.ICECandidateInit{Candidate: cand, SDPMid: &mid}
	s.mu.Lock()
	h := sess.handles[handle]
	s.mu.Unlock()
	if h == nil {
		return
	}
	h.pcMu.Lock()
	defer h.pcMu.Unlock()
	if h.pc == nil || h.pc.RemoteDescription() == nil {
		h.pending = append(h.pending, init)
		return
	}
	h.pc.AddICECandidate(init)
}

func (h *fakeHandle) flushCandidates() {
	for _, c := range h.pending {
		h.pc.AddICECandidate(c)
	}
	h.pending = nil
}

func (s *fakeJanus) publish(h *fakeHandle, jsep *JSEP, event func(map[string]any, *JSEP)) {
	s.mu.Lock()
	r := h.room
	active := 0
	if r != nil {
		for _, m := range r.members {
			if m.publishing {
				active++
			}
		}
	}
	s.mu.Unlock()
	switch {
	case r == nil || h.ptype != "publisher":
		event(vrError(424, "Join first"), nil)
		return
	case jsep == nil || jsep.Type != "offer":
		event(vrError(429, "Missing mandatory element (jsep)"), nil)
		return
	case active >= r.max:
		event(vrError(432, "Maximum number of active publishers ("+jsonNum(uint64(r.max))+") already reached"), nil)
		return
	}
	pc, err := s.newPC()
	if err != nil {
		event(vrError(499, err.Error()), nil)
		return
	}
	h.pcMu.Lock()
	h.pc = pc
	answer, err := func() (string, error) {
		if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: jsep.SDP}); err != nil {
			return "", err
		}
		h.flushCandidates()
		a, err := pc.CreateAnswer(nil)
		if err != nil {
			return "", err
		}
		gathered := webrtc.GatheringCompletePromise(pc)
		if err := pc.SetLocalDescription(a); err != nil {
			return "", err
		}
		<-gathered
		return pc.LocalDescription().SDP, nil
	}()
	h.pcMu.Unlock()
	if err != nil {
		event(vrError(437, err.Error()), nil)
		return
	}
	var streams []PublisherStream
	s.mu.Lock()
	for _, tr := range pc.GetTransceivers() {
		kind := tr.Kind().String()
		streams = append(streams, PublisherStream{Type: kind, MID: tr.Mid(), Codec: map[string]string{"audio": "opus", "video": "vp8"}[kind]})
		h.fans[tr.Mid()] = &fanout{}
	}
	h.streams, h.publishing = streams, true
	s.mu.Unlock()
	pc.OnTrack(func(track *webrtc.TrackRemote, recv *webrtc.RTPReceiver) {
		var mid string
		for _, tr := range pc.GetTransceivers() {
			if tr.Receiver() == recv {
				mid = tr.Mid()
			}
		}
		s.mu.Lock()
		fan := h.fans[mid]
		s.mu.Unlock()
		if fan == nil {
			return
		}
		for {
			pkt, _, err := track.ReadRTP()
			if err != nil {
				return
			}
			fan.mu.Lock()
			targets := slices.Clone(fan.targets)
			fan.mu.Unlock()
			for _, sub := range targets {
				if !sub.answered.Load() {
					continue // As Janus: nothing before the section is negotiated
				}
				sub.track.WriteRTP(pkt)
			}
		}
	})
	event(map[string]any{"videoroom": "event", "room": r.id, "configured": "ok", "streams": streams}, &JSEP{Type: "answer", SDP: answer})
	s.notify(r, h, map[string]any{"videoroom": "event", "room": r.id, "publishers": []map[string]any{h.publisherInfo()}})
}

// feedOf finds the publisher handle of feed in r (nil for the dummy).
func (r *fakeRoom) feedOf(feed uint64) *fakeHandle {
	for _, m := range r.members {
		if m.pid == feed && m.publishing {
			return m
		}
	}
	return nil
}

// addSub adds a sending section for (feed, mid) to the subscriber's PC.
func (s *fakeJanus) addSub(h *fakeHandle, ref streamRef) error {
	s.mu.Lock()
	r := h.room
	var kind, display string
	var fan *fanout
	if ref.Feed == r.dummyID {
		kind, display = "audio", "Dummy publisher"
	} else if p := r.feedOf(ref.Feed); p != nil {
		display = p.display
		for _, st := range p.streams {
			if st.MID == ref.MID {
				kind = st.Type
			}
		}
		fan = p.fans[ref.MID]
	}
	s.mu.Unlock()
	if kind == "" {
		return &janusError{Code: 428, Reason: "No such feed (" + jsonNum(ref.Feed) + "/" + ref.MID + ")"}
	}
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}
	if kind == "video" {
		codec = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}
	}
	track, err := webrtc.NewTrackLocalStaticRTP(codec, kind+"-"+jsonNum(ref.Feed)+"-"+ref.MID, "janus")
	if err != nil {
		return err
	}
	tr, err := h.pc.AddTransceiverFromTrack(track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	if err != nil {
		return err
	}
	sub := &fakeSub{feed: ref.Feed, feedMID: ref.MID, kind: kind, display: display, track: track, sender: tr.Sender(), tr: tr}
	s.mu.Lock()
	h.subs = append(h.subs, sub)
	s.mu.Unlock()
	if fan != nil {
		fan.mu.Lock()
		fan.targets = append(fan.targets, sub)
		fan.mu.Unlock()
	}
	return nil
}

func (s *fakeJanus) removeSub(h *fakeHandle, ref streamRef) {
	s.mu.Lock()
	i := slices.IndexFunc(h.subs, func(sub *fakeSub) bool {
		return sub.feed == ref.Feed && (ref.MID == "" || sub.feedMID == ref.MID)
	})
	if i < 0 {
		s.mu.Unlock()
		return
	}
	sub := h.subs[i]
	h.subs = slices.Delete(h.subs, i, i+1)
	p := h.room.feedOf(ref.Feed)
	s.mu.Unlock()
	if p != nil {
		if fan := p.fans[sub.feedMID]; fan != nil {
			fan.mu.Lock()
			fan.targets = slices.DeleteFunc(fan.targets, func(t *fakeSub) bool { return t == sub })
			fan.mu.Unlock()
		}
	}
	h.pc.RemoveTrack(sub.sender)
}

// offer makes the subscriber's next offer and the streams list of its
// attached or updated event.
func (s *fakeJanus) offer(h *fakeHandle) (string, []map[string]any, error) {
	o, err := h.pc.CreateOffer(nil)
	if err != nil {
		return "", nil, err
	}
	gathered := webrtc.GatheringCompletePromise(h.pc)
	if err := h.pc.SetLocalDescription(o); err != nil {
		return "", nil, err
	}
	<-gathered
	s.mu.Lock()
	defer s.mu.Unlock()
	var streams []map[string]any
	for _, sub := range h.subs {
		streams = append(streams, map[string]any{"mid": sub.tr.Mid(), "type": sub.kind, "feed_id": sub.feed,
			"feed_mid": sub.feedMID, "feed_display": sub.display, "active": true, "send": true})
	}
	h.offerOut = true
	return h.pc.LocalDescription().SDP, streams, nil
}

func refs(v any) []streamRef {
	var out []streamRef
	list, _ := v.([]any)
	for _, e := range list {
		m, _ := e.(map[string]any)
		mid, _ := m["mid"].(string)
		out = append(out, streamRef{Feed: num(m["feed"]), MID: mid})
	}
	return out
}

func (s *fakeJanus) joinSubscriber(h *fakeHandle, body map[string]any, event func(map[string]any, *JSEP)) {
	s.mu.Lock()
	r := s.rooms[num(body["room"])]
	var owner *fakeHandle
	for _, o := range h.sess.handles {
		if o.ptype == "publisher" {
			owner = o
		}
	}
	s.mu.Unlock()
	streams := refs(body["streams"])
	switch {
	case r == nil:
		event(vrError(426, "No such room"), nil)
		return
	case r.pin != "" && body["pin"] != r.pin:
		event(vrError(429, "Missing mandatory element (pin)"), nil)
		return
	case len(streams) == 0:
		event(vrError(429, "Missing mandatory element (feed)"), nil)
		return
	case body["autoupdate"] != false:
		event(vrError(499, "fake: the adapter must set autoupdate=false"), nil)
		return
	}
	pc, err := s.newPC()
	if err != nil {
		event(vrError(499, err.Error()), nil)
		return
	}
	pc.OnConnectionStateChange(func(st webrtc.PeerConnectionState) {
		if st == webrtc.PeerConnectionStateConnected {
			select {
			case <-h.connected:
			default:
				close(h.connected)
			}
		}
	})
	h.pcMu.Lock()
	defer h.pcMu.Unlock()
	s.mu.Lock()
	h.room, h.ptype, h.pc, h.owner = r, "subscriber", pc, owner
	s.mu.Unlock()
	for _, ref := range streams {
		if err := s.addSub(h, ref); err != nil {
			event(vrError(428, err.Error()), nil)
			return
		}
	}
	sdp, list, err := s.offer(h)
	if err != nil {
		event(vrError(499, err.Error()), nil)
		return
	}
	event(map[string]any{"videoroom": "attached", "room": r.id, "streams": list}, &JSEP{Type: "offer", SDP: sdp})
}

func (s *fakeJanus) start(h *fakeHandle, jsep *JSEP, event func(map[string]any, *JSEP)) {
	if jsep == nil || jsep.Type != "answer" {
		event(vrError(429, "Missing mandatory element (jsep)"), nil)
		return
	}
	h.pcMu.Lock()
	if err := h.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: jsep.SDP}); err != nil {
		h.pcMu.Unlock()
		event(vrError(437, err.Error()), nil)
		return
	}
	h.flushCandidates()
	s.mu.Lock()
	h.offerOut = false
	for _, sub := range h.subs {
		sub.answered.Store(true)
	}
	if h.owner != nil {
		s.answers[h.owner.display] = jsep.SDP
	}
	queued := h.queued
	h.queued = nil
	s.mu.Unlock()
	h.pcMu.Unlock()
	event(map[string]any{"videoroom": "event", "room": h.room.id, "started": "ok"}, nil)
	if len(queued) > 0 {
		// Janus sends the postponed renegotiation on its own (T002).
		var merged updateReq
		for _, q := range queued {
			merged.Subscribe = append(merged.Subscribe, q.Subscribe...)
			merged.Unsubscribe = append(merged.Unsubscribe, q.Unsubscribe...)
		}
		h.pcMu.Lock()
		data, sdp, err := s.applyUpdate(h, merged)
		h.pcMu.Unlock()
		if err == nil {
			s.send(h.sess, pluginReply("event", h, data, &JSEP{Type: "offer", SDP: sdp}))
		}
	}
}

func (s *fakeJanus) update(h *fakeHandle, body map[string]any, event func(map[string]any, *JSEP)) {
	u := updateReq{Subscribe: refs(body["subscribe"]), Unsubscribe: refs(body["unsubscribe"])}
	if len(u.Subscribe) == 0 && len(u.Unsubscribe) == 0 {
		event(vrError(429, "At least one of either 'subscribe' or 'unsubscribe' must be present"), nil)
		return
	}
	h.pcMu.Lock()
	defer h.pcMu.Unlock()
	s.mu.Lock()
	h.updates = append(h.updates, u)
	if h.offerOut {
		h.queued = append(h.queued, u)
		s.updating++
		s.mu.Unlock()
		event(map[string]any{"videoroom": "updating", "room": h.room.id}, nil)
		return
	}
	s.mu.Unlock()
	data, sdp, err := s.applyUpdate(h, u)
	if err != nil {
		event(vrError(errorCode(err), err.Error()), nil)
		return
	}
	event(data, &JSEP{Type: "offer", SDP: sdp})
}

func (s *fakeJanus) applyUpdate(h *fakeHandle, u updateReq) (map[string]any, string, error) {
	for _, ref := range u.Unsubscribe {
		s.removeSub(h, ref)
	}
	for _, ref := range u.Subscribe {
		if err := s.addSub(h, ref); err != nil {
			return nil, "", err
		}
	}
	sdp, list, err := s.offer(h)
	if err != nil {
		return nil, "", err
	}
	return map[string]any{"videoroom": "updated", "room": h.room.id, "streams": list}, sdp, nil
}

// leave removes a handle from its room: a publisher's feed ends (Janus
// sends unpublished twice, then leaving; T002) and subscriptions to it stop
// receiving; a subscriber's PC closes.
func (s *fakeJanus) leave(h *fakeHandle) {
	s.mu.Lock()
	r := h.room
	wasPublishing := h.publishing
	h.publishing = false
	if r != nil {
		r.members = slices.DeleteFunc(r.members, func(m *fakeHandle) bool { return m == h })
	}
	h.room = nil
	pc := h.pc
	s.mu.Unlock()
	if pc != nil {
		pc.Close()
	}
	if r == nil || h.ptype != "publisher" {
		return
	}
	if wasPublishing {
		for range 2 {
			s.notify(r, h, map[string]any{"videoroom": "event", "room": r.id, "display": h.display, "unpublished": h.pid})
		}
	}
	s.notify(r, h, map[string]any{"videoroom": "event", "room": r.id, "display": h.display, "leaving": h.pid})
}

func (s *fakeJanus) dropSession(sess *fakeSession) {
	s.mu.Lock()
	if sess.gone {
		s.mu.Unlock()
		return
	}
	sess.gone = true
	delete(s.sessions, sess.id)
	var hs []*fakeHandle
	for _, h := range sess.handles {
		hs = append(hs, h)
	}
	s.mu.Unlock()
	for _, h := range hs {
		s.leave(h)
	}
}

func (s *fakeJanus) expireLoop() {
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		s.mu.Lock()
		timeout := s.sessionTimeout
		var expired []*fakeSession
		for _, sess := range s.sessions {
			if timeout > 0 && time.Since(sess.lastSeen) > timeout {
				expired = append(expired, sess)
			}
		}
		s.mu.Unlock()
		for _, sess := range expired {
			s.send(sess, map[string]any{"janus": "timeout", "session_id": sess.id})
			s.dropSession(sess)
		}
	}
}

// --- Hooks and readers for tests -------------------------------------------

// publisherHandle finds the publisher handle of display (nil if none).
func (s *fakeJanus) publisherHandle(display string) *fakeHandle {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sess := range s.sessions {
		for _, h := range sess.handles {
			if h.ptype == "publisher" && h.display == display {
				return h
			}
		}
	}
	return nil
}

func (s *fakeJanus) subscriberHandle(display string) *fakeHandle {
	p := s.publisherHandle(display)
	if p == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range p.sess.handles {
		if h.ptype == "subscriber" {
			return h
		}
	}
	return nil
}

// failRequest makes the server answer a VideoRoom request with an error.
func (s *fakeJanus) failRequest(req string, code int, reason string) {
	s.mu.Lock()
	s.fail[req] = &janusError{Code: code, Reason: reason}
	s.mu.Unlock()
}

// kick closes the WebSocket of display's session.
func (s *fakeJanus) kick(display string) {
	if h := s.publisherHandle(display); h != nil {
		h.sess.conn.Close(websocket.StatusGoingAway, "kicked by the test")
	}
}

// hangup sends hangup on display's subscriber handle.
func (s *fakeJanus) hangup(display string) {
	if h := s.subscriberHandle(display); h != nil {
		s.send(h.sess, map[string]any{"janus": "hangup", "session_id": h.sess.id, "sender": h.id, "reason": "test"})
	}
}

// subscribed lists display's subscribed streams as "<feed display>/<kind>",
// sorted, the dummy excluded.
func (s *fakeJanus) subscribed(display string) []string {
	h := s.subscriberHandle(display)
	if h == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, sub := range h.subs {
		if sub.feed != h.room.dummyID {
			out = append(out, sub.display+"/"+sub.kind)
		}
	}
	slices.Sort(out)
	return out
}

func (s *fakeJanus) updatesOf(display string) []updateReq {
	h := s.subscriberHandle(display)
	if h == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(h.updates)
}

// framesOf returns the raw frames display's session sent.
func (s *fakeJanus) framesOf(display string) []string {
	h := s.publisherHandle(display)
	if h == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(h.sess.frames)
}

// requestsOf lists the janus verbs and VideoRoom requests display's
// session sent, keepalives and trickles excluded.
func (s *fakeJanus) requestsOf(display string) []string {
	var out []string
	for _, raw := range s.framesOf(display) {
		var f map[string]any
		json.Unmarshal([]byte(raw), &f)
		switch f["janus"] {
		case "keepalive", "trickle":
		case "message":
			body, _ := f["body"].(map[string]any)
			r, _ := body["request"].(string)
			if r == "join" {
				r += ":" + body["ptype"].(string)
			}
			out = append(out, r)
		default:
			out = append(out, f["janus"].(string))
		}
	}
	return out
}

// subConnected waits for display's subscriber PC to connect.
func (s *fakeJanus) subConnected(t *testing.T, display string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if h := s.subscriberHandle(display); h != nil {
			select {
			case <-h.connected:
				return
			case <-time.After(20 * time.Millisecond):
			}
		} else {
			time.Sleep(20 * time.Millisecond)
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: subscriber not connected", display)
		}
	}
}

// capture wraps a bot's SignalingHandler to count SDP calls, so tests can
// check that none runs after Disconnect.
type capture struct {
	platform.SignalingHandler
	sdpCalls atomic.Int32
	closed   atomic.Bool // Set by the test after Disconnect: SDP calls then are late
	late     atomic.Int32
}

func (c *capture) count() {
	c.sdpCalls.Add(1)
	if c.closed.Load() {
		c.late.Add(1)
	}
}

func (c *capture) OnOffer(offer string, trickle platform.TrickleFunc) (string, error) {
	c.count()
	return c.SignalingHandler.OnOffer(offer, trickle)
}

func (c *capture) Renegotiate(offer string) (string, error) {
	c.count()
	return c.SignalingHandler.Renegotiate(offer)
}

func (c *capture) CreatePublishOffer(trickle platform.TrickleFunc) (string, error) {
	c.count()
	return c.SignalingHandler.CreatePublishOffer(trickle)
}

func (c *capture) ApplyPublishAnswer(answer string) error {
	c.count()
	return c.SignalingHandler.ApplyPublishAnswer(answer)
}
