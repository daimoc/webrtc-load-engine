package mediasoup

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	mrand "math/rand/v2"
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
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
	"nhooyr.io/websocket"

	"webrtc-load-engine/src/platform"
)

// fakeMediasoup plays mediasoup-demo's side for the adapter's tests
// (research R12): protoo over WebSocket, one room, and per transport a
// real Pion PeerConnection in ICE-lite mode on its own loopback port, with
// the ICE credentials and DTLS fingerprint it hands out in
// createWebRtcTransport. Producers' RTP is forwarded to the consumers that
// are active, with each consumer's SSRC and the router's payload type,
// timestamps untouched.
//
// mediasoup has no SDP; the fake's Pion side needs the bot's. It learns the
// bot's answers and publish offer through capture, a wrapper the tests put
// around each bot: it stands for what mediasoup learns from STUN and DTLS.
type fakeMediasoup struct {
	t   *testing.T
	srv *httptest.Server

	mu    sync.Mutex
	peers map[string]*fakePeer // By peer ID, from the handshake

	caps         RtpCapabilities
	refuseStatus atomic.Int32 // Refuse handshakes with this status (0: accept)
	failMethod   atomic.Value // Request method answered with an error
	urls         chan string  // Handshake URLs (with their query)
	origins      chan string  // Handshake Origin headers
	nextID       atomic.Int64
	joinSeq      atomic.Int64
	early        atomic.Int32 // Accepts or resumes before the bot's answer had the consumer's section
}

type fakePeer struct {
	s    *fakeMediasoup
	id   string
	conn *protooConn

	mu        sync.Mutex
	cond      *sync.Cond
	joined    bool
	joinSeq   int64 // Join order: the fake consumes others' producers in it
	gone      bool
	recv      *fakeTransport
	send      *fakeTransport
	producers []*fakeProducer
	consumers map[string]*fakeConsumer
	nextMid   int
	answer    string // Latest answer of the bot's receiving session
	pubOffer  string // The bot's publish offer
	declined  []string
	resumed   []string
	connected map[string]DtlsParameters // By transport direction
	order     []string                  // Requests received, in order
	negMu     sync.Mutex                // Serializes the receiving PC's negotiations
	appMid    string                    // The fake PC's application mid
}

type fakeTransport struct {
	direction string
	pc        *webrtc.PeerConnection
	sock      net.PacketConn
	info      TransportInfo
	answered  bool // Publish side: the bot's offer applied
}

type fakeProducer struct {
	id    string
	kind  string
	mid   string
	owner *fakePeer
	rtp   RtpParameters

	mu      sync.Mutex
	targets []*fakeConsumer // Active consumers
}

type fakeConsumer struct {
	id       string
	producer *fakeProducer
	peer     *fakePeer
	mid      string
	ssrc     uint32
	pt       uint8
	track    *ssrcTrack
	state    string // offered, active, declined, closed
}

func newFakeMediasoup(t *testing.T) *fakeMediasoup {
	t.Helper()
	s := &fakeMediasoup{t: t, peers: map[string]*fakePeer{}, caps: routerCaps(t), urls: make(chan string, 64), origins: make(chan string, 64)}
	s.failMethod.Store("")
	s.srv = httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(func() {
		s.mu.Lock()
		ps := make([]*fakePeer, 0, len(s.peers))
		for _, p := range s.peers {
			ps = append(ps, p)
		}
		s.mu.Unlock()
		for _, p := range ps {
			p.conn.Close()
			p.closeTransports()
		}
		s.srv.Close()
	})
	return s
}

// config returns the platform_config of the fake.
func (s *fakeMediasoup) config(room string) map[string]any {
	return map[string]any{"url": s.srv.URL, "room": room}
}

func (s *fakeMediasoup) peer(id string) *fakePeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.peers[id]
}

func (s *fakeMediasoup) newID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, s.nextID.Add(1))
}

func (s *fakeMediasoup) serve(w http.ResponseWriter, r *http.Request) {
	select {
	case s.urls <- r.URL.String():
	default:
	}
	select {
	case s.origins <- r.Header.Get("Origin"):
	default:
	}
	if st := s.refuseStatus.Load(); st != 0 {
		http.Error(w, "refused by the fake", int(st))
		return
	}
	q := r.URL.Query()
	if q.Get("roomId") == "" || q.Get("peerId") == "" {
		http.Error(w, "Missing roomId and/or peerId", http.StatusBadRequest)
		return
	}
	if r.Header.Get("Origin") == "" {
		http.Error(w, "GO TO HELL", http.StatusForbidden)
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"protoo"}, InsecureSkipVerify: true})
	if err != nil {
		return
	}
	if c.Subprotocol() != "protoo" {
		c.Close(websocket.StatusPolicyViolation, "protoo required")
		return
	}
	c.SetReadLimit(readLimit)
	p := &fakePeer{s: s, id: q.Get("peerId"), consumers: map[string]*fakeConsumer{}, nextMid: 1, connected: map[string]DtlsParameters{}}
	p.cond = sync.NewCond(&p.mu)
	p.conn = newProtooConn(c, quietLogger)
	p.conn.onRequest = p.onRequest
	p.conn.onNotification = p.onNotification
	s.mu.Lock()
	s.peers[p.id] = p
	s.mu.Unlock()
	p.conn.readLoop()
	p.left()
}

// kick closes a peer's WebSocket, as a server closing a room does.
func (s *fakeMediasoup) kick(id string) {
	if p := s.peer(id); p != nil {
		p.conn.Close()
	}
}

// declined and resumed return the consumers a peer declined or resumed.
func (s *fakeMediasoup) declined(id string) []string {
	p := s.peer(id)
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.declined)
}

func (s *fakeMediasoup) resumed(id string) []string {
	p := s.peer(id)
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.resumed)
}

// activeFrom returns the IDs of the peers whose producers id actively
// consumes, sorted, one per consumer.
func (s *fakeMediasoup) activeFrom(id string) []string {
	p := s.peer(id)
	p.mu.Lock()
	defer p.mu.Unlock()
	var owners []string
	for _, c := range p.consumers {
		if c.state == "active" {
			owners = append(owners, c.producer.owner.id+"/"+c.producer.kind)
		}
	}
	slices.Sort(owners)
	return owners
}

func (p *fakePeer) onRequest(method string, data json.RawMessage, reply replyFunc) {
	p.mu.Lock()
	p.order = append(p.order, method)
	p.mu.Unlock()
	if m, _ := p.s.failMethod.Load().(string); m == method {
		reply(nil, &requestError{Code: 500, Reason: "fake failure of " + method})
		return
	}
	go func() {
		data, err := p.handle(method, data)
		if err != nil {
			reply(nil, &requestError{Code: 500, Reason: err.Error()})
			return
		}
		reply(data, nil)
	}()
}

func (p *fakePeer) handle(method string, data json.RawMessage) (any, error) {
	switch method {
	case "getRouterRtpCapabilities":
		return routerCapsResponse{RouterRtpCapabilities: p.s.caps}, nil
	case "createWebRtcTransport":
		var req createTransportRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		dir := req.AppData["direction"]
		ft, err := newFakeTransport(dir)
		if err != nil {
			return nil, err
		}
		ft.info.TransportID = p.s.newID("transport")
		p.mu.Lock()
		if dir == "consumer" {
			p.recv = ft
		} else {
			p.send = ft
			p.watchProducerTracks(ft)
		}
		p.mu.Unlock()
		return ft.info, nil
	case "connectWebRtcTransport":
		var req connectTransportRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		return nil, p.connect(req)
	case "join":
		var req joinRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		p.mu.Lock()
		p.joined = true
		p.joinSeq = p.s.joinSeq.Add(1)
		p.mu.Unlock()
		var resp joinResponse
		for _, o := range p.s.others(p) {
			resp.Peers = append(resp.Peers, struct {
				ID string `json:"id"`
			}{ID: o.id})
		}
		go func() {
			time.Sleep(10 * time.Millisecond) // After the join response
			for _, o := range p.s.others(p) {
				for _, prod := range o.producerList() {
					p.s.consume(p, prod)
				}
			}
			p.conn.Request(context.Background(), "newDataConsumer", map[string]any{"dataConsumerId": p.s.newID("dc")}, nil)
		}()
		return resp, nil
	case "produce":
		var req produceRequest
		if err := json.Unmarshal(data, &req); err != nil {
			return nil, err
		}
		prod, err := p.produce(req)
		if err != nil {
			return nil, err
		}
		go func() {
			for _, o := range p.s.others(p) {
				p.s.consume(o, prod)
			}
		}()
		return produceResponse{ProducerID: prod.id}, nil
	}
	return nil, fmt.Errorf("unknown request method %q", method)
}

func (p *fakePeer) onNotification(method string, data json.RawMessage) {
	if method != "resumeConsumer" {
		return
	}
	var ref consumerRef
	if json.Unmarshal(data, &ref) != nil {
		return
	}
	p.mu.Lock()
	c := p.consumers[ref.ConsumerID]
	if c == nil || c.state != "declined" {
		p.mu.Unlock()
		return
	}
	p.resumed = append(p.resumed, ref.ConsumerID)
	if !strings.Contains(p.answer, "a=mid:"+c.mid+"\r\n") {
		p.s.early.Add(1)
	}
	p.mu.Unlock()
	go p.activate(c)
}

func (s *fakeMediasoup) others(p *fakePeer) []*fakePeer {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*fakePeer
	for _, o := range s.peers {
		if o == p {
			continue
		}
		o.mu.Lock()
		ok := o.joined && !o.gone
		o.mu.Unlock()
		if ok {
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b *fakePeer) int { return int(a.joinSeq - b.joinSeq) })
	return out
}

func (p *fakePeer) producerList() []*fakeProducer {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Clone(p.producers)
}

// newFakeTransport builds a transport's Pion side: ICE-lite on its own
// 127.0.0.1 port, known credentials and certificate.
func newFakeTransport(direction string) (*fakeTransport, error) {
	sock, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	cert, err := webrtc.GenerateCertificate(key)
	if err != nil {
		return nil, err
	}
	fps, err := cert.GetFingerprints()
	if err != nil {
		return nil, err
	}
	ufrag, pwd := randomString(16), randomString(32)

	m := &webrtc.MediaEngine{}
	for _, c := range []webrtc.RTPCodecParameters{
		{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1"}, PayloadType: 100},
	} {
		if err := m.RegisterCodec(c, webrtc.RTPCodecTypeAudio); err != nil {
			return nil, err
		}
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, PayloadType: 101}, webrtc.RTPCodecTypeVideo); err != nil {
		return nil, err
	}
	var se webrtc.SettingEngine
	quiet := logging.NewDefaultLoggerFactory()
	quiet.DefaultLogLevel = logging.LogLevelDisabled
	se.LoggerFactory = quiet
	se.SetLite(true)
	se.SetICECredentials(ufrag, pwd)
	se.SetICEUDPMux(webrtc.NewICEUDPMux(quiet.NewLogger("udpmux"), sock))
	se.SetIncludeLoopbackCandidate(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	if direction == "producer" {
		if err := se.SetAnsweringDTLSRole(webrtc.DTLSRoleServer); err != nil {
			return nil, err
		}
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se))
	pc, err := api.NewPeerConnection(webrtc.Configuration{Certificates: []webrtc.Certificate{*cert}})
	if err != nil {
		return nil, err
	}
	port := sock.LocalAddr().(*net.UDPAddr).Port
	info := TransportInfo{
		IceParameters: IceParameters{UsernameFragment: ufrag, Password: pwd, IceLite: true},
		IceCandidates: []IceCandidate{
			{Foundation: "udpcandidate", Priority: 1076302079, Address: "127.0.0.1", Protocol: "udp", Port: port, Type: "host"},
			{Foundation: "tcpcandidate", Priority: 1076276479, Address: "127.0.0.1", Protocol: "tcp", Port: port, Type: "host", TCPType: "passive"},
		},
		DtlsParameters: DtlsParameters{Role: "auto"},
		SctpParameters: &SctpParameters{Port: 5000, OS: 65535, MIS: 65535, MaxMessageSize: 262144},
	}
	for _, f := range fps {
		info.DtlsParameters.Fingerprints = append(info.DtlsParameters.Fingerprints, DtlsFingerprint{Algorithm: f.Algorithm, Value: strings.ToUpper(f.Value)})
	}
	return &fakeTransport{direction: direction, pc: pc, sock: sock, info: info}, nil
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[mrand.IntN(len(letters))]
	}
	return string(b)
}

func (p *fakePeer) closeTransports() {
	p.mu.Lock()
	ts := []*fakeTransport{p.recv, p.send}
	p.mu.Unlock()
	for _, t := range ts {
		if t != nil {
			t.pc.Close()
			t.sock.Close()
		}
	}
}

// waitBotSDP waits until pick returns a non-empty SDP from the peer's
// captured state.
func (p *fakePeer) waitBotSDP(pick func() string) (string, error) {
	deadline := time.Now().Add(5 * time.Second)
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		if s := pick(); s != "" {
			return s, nil
		}
		if time.Now().After(deadline) || p.gone {
			return "", fmt.Errorf("bot SDP not captured for %s", p.id)
		}
		p.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		p.mu.Lock()
	}
}

// connect records the bot's DTLS parameters. For the receiving transport,
// the fake's PC then makes its application-only offer and applies the bot's
// answer.
func (p *fakePeer) connect(req connectTransportRequest) error {
	p.mu.Lock()
	var ft *fakeTransport
	switch {
	case p.recv != nil && p.recv.info.TransportID == req.TransportID:
		ft = p.recv
	case p.send != nil && p.send.info.TransportID == req.TransportID:
		ft = p.send
	}
	p.mu.Unlock()
	if ft == nil {
		return fmt.Errorf("transport %q not found", req.TransportID)
	}
	p.mu.Lock()
	p.connected[ft.direction] = req.DtlsParameters
	p.mu.Unlock()
	if ft.direction != "consumer" {
		return nil
	}
	answer, err := p.waitBotSDP(func() string { return p.answer })
	if err != nil {
		return err
	}
	if fp, err := localFingerprint(answer); err != nil || len(req.DtlsParameters.Fingerprints) == 0 || !strings.EqualFold(fp.Value, req.DtlsParameters.Fingerprints[0].Value) {
		return fmt.Errorf("connect fingerprint %v does not match the bot's answer (%v)", req.DtlsParameters.Fingerprints, err)
	}
	if role, _ := localDtlsRole(answer); role != req.DtlsParameters.Role {
		return fmt.Errorf("connect role %q, the bot's answer says %q", req.DtlsParameters.Role, role)
	}
	p.negMu.Lock()
	defer p.negMu.Unlock()
	if _, err := ft.pc.CreateDataChannel("fake", nil); err != nil {
		return err
	}
	return p.negotiateRecv(ft)
}

// negotiateRecv makes the receiving PC's offer and applies the bot's latest
// answer, filtered to the fake's sections (the bot may be ahead).
func (p *fakePeer) negotiateRecv(ft *fakeTransport) error {
	offer, err := ft.pc.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := ft.pc.SetLocalDescription(offer); err != nil {
		return err
	}
	var local sdp.SessionDescription
	if err := local.Unmarshal([]byte(offer.SDP)); err != nil {
		return err
	}
	var mids []string
	for _, md := range local.MediaDescriptions {
		mid, _ := md.Attribute("mid")
		if md.MediaName.Media == "application" {
			p.appMid = mid
		}
		mids = append(mids, mid)
	}
	var filtered string
	_, err = p.waitBotSDP(func() string {
		filtered = filterAnswer(p.answer, mids, p.appMid)
		return filtered
	})
	if err != nil {
		return err
	}
	return ft.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: filtered})
}

// filterAnswer keeps the answer's sections whose mid is in mids (the
// bot's "datachannel" is the fake's appMid), in mids' order. It returns ""
// while the answer lacks one of them.
func filterAnswer(answer string, mids []string, appMid string) string {
	if answer == "" {
		return ""
	}
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(answer)); err != nil {
		return ""
	}
	byMid := map[string]*sdp.MediaDescription{}
	for _, md := range d.MediaDescriptions {
		mid, _ := md.Attribute("mid")
		if mid == dataChannelMID {
			mid = appMid
			for i, a := range md.Attributes {
				if a.Key == "mid" {
					md.Attributes[i].Value = appMid
				}
			}
		}
		byMid[mid] = md
	}
	var kept []*sdp.MediaDescription
	for _, mid := range mids {
		md, ok := byMid[mid]
		if !ok {
			return ""
		}
		kept = append(kept, md)
	}
	d.MediaDescriptions = kept
	for i, a := range d.Attributes {
		if a.Key == "group" && strings.HasPrefix(a.Value, "BUNDLE") {
			d.Attributes[i].Value = "BUNDLE " + strings.Join(mids, " ")
		}
	}
	b, err := d.Marshal()
	if err != nil {
		return ""
	}
	return string(b)
}

// produce records a producer; the first one applies the bot's publish
// offer to the sending PC.
func (p *fakePeer) produce(req produceRequest) (*fakeProducer, error) {
	p.mu.Lock()
	ft := p.send
	p.mu.Unlock()
	if ft == nil || ft.info.TransportID != req.TransportID {
		return nil, fmt.Errorf("transport %q not found", req.TransportID)
	}
	p.negMu.Lock()
	if !ft.answered {
		offer, err := p.waitBotSDP(func() string { return p.pubOffer })
		if err != nil {
			p.negMu.Unlock()
			return nil, err
		}
		if err := ft.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
			p.negMu.Unlock()
			return nil, err
		}
		answer, err := ft.pc.CreateAnswer(nil)
		if err == nil {
			err = ft.pc.SetLocalDescription(answer)
		}
		if err != nil {
			p.negMu.Unlock()
			return nil, err
		}
		ft.answered = true
	}
	p.negMu.Unlock()
	prod := &fakeProducer{id: p.s.newID("producer"), kind: req.Kind, mid: req.RtpParameters.MID, owner: p, rtp: req.RtpParameters}
	p.mu.Lock()
	p.producers = append(p.producers, prod)
	p.mu.Unlock()
	return prod, nil
}

// watchProducerTracks forwards each received track to its producer's
// active consumers.
func (p *fakePeer) watchProducerTracks(ft *fakeTransport) {
	ft.pc.OnTrack(func(track *webrtc.TrackRemote, recv *webrtc.RTPReceiver) {
		var mid string
		for _, tr := range ft.pc.GetTransceivers() {
			if tr.Receiver() == recv {
				mid = tr.Mid()
			}
		}
		var prod *fakeProducer
		deadline := time.Now().Add(5 * time.Second)
		for prod == nil && time.Now().Before(deadline) {
			p.mu.Lock()
			for _, pr := range p.producers {
				if pr.mid == mid {
					prod = pr
				}
			}
			p.mu.Unlock()
			if prod == nil {
				time.Sleep(10 * time.Millisecond)
			}
		}
		if prod == nil {
			return
		}
		for {
			pkt, _, err := track.ReadRTP()
			if err != nil {
				return
			}
			prod.mu.Lock()
			targets := slices.Clone(prod.targets)
			prod.mu.Unlock()
			for _, c := range targets {
				c.track.write(pkt, c.ssrc, c.pt)
			}
		}
	})
}

// consume offers prod to q, as mediasoup's Peer.consume: a paused consumer,
// then newConsumer; resumed (forwarded) once accepted.
func (s *fakeMediasoup) consume(q *fakePeer, prod *fakeProducer) {
	q.mu.Lock()
	if q.recv == nil || !q.joined || q.gone {
		q.mu.Unlock()
		return
	}
	c := &fakeConsumer{id: s.newID("consumer"), producer: prod, peer: q, mid: fmt.Sprint(q.nextMid), ssrc: mrand.Uint32(), state: "offered"}
	q.nextMid++
	q.consumers[c.id] = c
	q.mu.Unlock()

	rp := RtpParameters{MID: c.mid, RTCP: RtcpParameters{CNAME: prod.rtp.RTCP.CNAME, ReducedSize: true}, Encodings: []RtpEncodingParameters{{SSRC: c.ssrc}}, HeaderExtensions: []RtpHeaderExtensionParameters{}}
	for _, cc := range s.caps.Codecs {
		switch {
		case prod.kind == "audio" && cc.MimeType == "audio/opus":
			rp.Codecs = append(rp.Codecs, RtpCodecParameters{MimeType: cc.MimeType, PayloadType: cc.PreferredPayloadType, ClockRate: cc.ClockRate, Channels: cc.Channels,
				Parameters: map[string]any{"minptime": float64(10), "useinbandfec": float64(1)}, RtcpFeedback: []RtcpFeedback{{Type: "nack"}}})
		case prod.kind == "video" && cc.MimeType == "video/VP8":
			rp.Codecs = append(rp.Codecs, RtpCodecParameters{MimeType: cc.MimeType, PayloadType: cc.PreferredPayloadType, ClockRate: cc.ClockRate, Parameters: map[string]any{}, RtcpFeedback: cc.RtcpFeedback})
		}
	}
	if len(rp.Codecs) == 0 {
		return
	}
	c.pt = uint8(rp.Codecs[0].PayloadType)
	req := NewConsumerRequest{PeerID: prod.owner.id, TransportID: q.recv.info.TransportID, ConsumerID: c.id, ProducerID: prod.id, Kind: prod.kind, RtpParameters: rp, Type: "simple"}
	// Sent in order, one at a time, as mediasoup offers a peer's producers.
	func() {
		err := q.conn.Request(context.Background(), "newConsumer", req, nil)
		q.mu.Lock()
		if c.state != "offered" {
			q.mu.Unlock()
			return
		}
		if err != nil {
			c.state = "declined"
			q.declined = append(q.declined, c.id)
			q.mu.Unlock()
			return
		}
		if !strings.Contains(q.answer, "a=mid:"+c.mid+"\r\n") {
			s.early.Add(1)
		}
		q.mu.Unlock()
		q.activate(c)
	}()
}

// activate adds c's sending track to the receiving PC, renegotiates with
// the bot's latest answer and starts forwarding.
func (q *fakePeer) activate(c *fakeConsumer) {
	q.negMu.Lock()
	defer q.negMu.Unlock()
	q.mu.Lock()
	ft := q.recv
	if c.state == "closed" || ft == nil {
		q.mu.Unlock()
		return
	}
	c.state = "active"
	q.mu.Unlock()
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}
	if c.producer.kind == "video" {
		codec = webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}
	}
	c.track = &ssrcTrack{id: c.id, kind: c.producer.kind, codec: codec}
	tr, err := ft.pc.AddTransceiverFromTrack(c.track, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly})
	if err != nil {
		q.s.t.Logf("fake: add transceiver: %v", err)
		return
	}
	if err := tr.SetMid(c.mid); err != nil {
		q.s.t.Logf("fake: set mid: %v", err)
		return
	}
	if err := q.negotiateRecv(ft); err != nil {
		q.s.t.Logf("fake: renegotiate %s: %v", q.id, err)
		return
	}
	c.producer.mu.Lock()
	c.producer.targets = append(c.producer.targets, c)
	c.producer.mu.Unlock()
}

// left closes a peer that went away: its consumers on other peers close
// (consumerClosed), they get peerClosed.
func (p *fakePeer) left() {
	p.mu.Lock()
	p.gone = true
	prods := slices.Clone(p.producers)
	p.mu.Unlock()
	p.s.mu.Lock()
	delete(p.s.peers, p.id)
	others := make([]*fakePeer, 0, len(p.s.peers))
	for _, o := range p.s.peers {
		others = append(others, o)
	}
	p.s.mu.Unlock()
	for _, o := range others {
		o.mu.Lock()
		var closed []string
		for _, c := range o.consumers {
			if slices.Contains(prods, c.producer) && c.state != "closed" {
				c.state = "closed"
				closed = append(closed, c.id)
			}
		}
		o.mu.Unlock()
		for _, prod := range prods {
			prod.mu.Lock()
			prod.targets = slices.DeleteFunc(prod.targets, func(c *fakeConsumer) bool { return c.peer == o })
			prod.mu.Unlock()
		}
		for _, id := range closed {
			o.conn.Notify("consumerClosed", consumerRef{ConsumerID: id})
		}
		o.conn.Notify("peerClosed", peerRef{PeerID: p.id})
	}
	p.closeTransports()
}

// ssrcTrack is a sending track that writes packets with the SSRC and
// payload type the fake announced in newConsumer (Pion's static tracks
// would use the sender's own SSRC).
type ssrcTrack struct {
	id    string
	kind  string
	codec webrtc.RTPCodecCapability

	mu sync.Mutex
	ws webrtc.TrackLocalWriter
}

func (t *ssrcTrack) Bind(ctx webrtc.TrackLocalContext) (webrtc.RTPCodecParameters, error) {
	for _, c := range ctx.CodecParameters() {
		if strings.EqualFold(c.MimeType, t.codec.MimeType) {
			t.mu.Lock()
			t.ws = ctx.WriteStream()
			t.mu.Unlock()
			return c, nil
		}
	}
	return webrtc.RTPCodecParameters{}, webrtc.ErrUnsupportedCodec
}

func (t *ssrcTrack) Unbind(webrtc.TrackLocalContext) error {
	t.mu.Lock()
	t.ws = nil
	t.mu.Unlock()
	return nil
}

func (t *ssrcTrack) ID() string       { return t.id }
func (t *ssrcTrack) RID() string      { return "" }
func (t *ssrcTrack) StreamID() string { return "fake" }
func (t *ssrcTrack) Kind() webrtc.RTPCodecType {
	if t.kind == "video" {
		return webrtc.RTPCodecTypeVideo
	}
	return webrtc.RTPCodecTypeAudio
}

func (t *ssrcTrack) write(pkt *rtp.Packet, ssrc uint32, pt uint8) {
	t.mu.Lock()
	ws := t.ws
	t.mu.Unlock()
	if ws == nil {
		return
	}
	h := pkt.Header
	h.SSRC, h.PayloadType = ssrc, pt
	h.Extensions, h.Extension, h.ExtensionProfile = nil, false, 0
	ws.WriteRTP(&h, pkt.Payload)
}

// capture wraps a bot's SignalingHandler for the fake: it records the
// bot's answers and publish offer, and counts the SDP calls.
type capture struct {
	platform.SignalingHandler
	s        *fakeMediasoup
	id       string
	sdpCalls atomic.Int32
	closed   atomic.Bool // Set by the test after Disconnect: SDP calls then are errors
	late     atomic.Int32
}

func (c *capture) record(f func(p *fakePeer)) {
	if p := c.s.peer(c.id); p != nil {
		p.mu.Lock()
		f(p)
		p.mu.Unlock()
	}
}

func (c *capture) count() {
	c.sdpCalls.Add(1)
	if c.closed.Load() {
		c.late.Add(1)
	}
}

func (c *capture) OnOffer(offer string, trickle platform.TrickleFunc) (string, error) {
	c.count()
	a, err := c.SignalingHandler.OnOffer(offer, trickle)
	if err == nil {
		c.record(func(p *fakePeer) { p.answer = a })
	}
	return a, err
}

func (c *capture) Renegotiate(offer string) (string, error) {
	c.count()
	a, err := c.SignalingHandler.Renegotiate(offer)
	if err == nil {
		c.record(func(p *fakePeer) { p.answer = a })
	}
	return a, err
}

func (c *capture) CreatePublishOffer(trickle platform.TrickleFunc) (string, error) {
	c.count()
	o, err := c.SignalingHandler.CreatePublishOffer(trickle)
	if err == nil {
		c.record(func(p *fakePeer) { p.pubOffer = o })
	}
	return o, err
}

func (c *capture) ApplyPublishAnswer(answer string) error {
	c.count()
	return c.SignalingHandler.ApplyPublishAnswer(answer)
}
