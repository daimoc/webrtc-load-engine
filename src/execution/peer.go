package execution

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/platform"
)

// PeerState represents the state of a peer in the execution lifecycle.
type PeerState string

const (
	PeerStateNew          PeerState = "new"
	PeerStateConnecting   PeerState = "connecting"   // Signaling connection/join in progress
	PeerStateConnected    PeerState = "connected"    // Joined the room on the platform
	PeerStateEstablished  PeerState = "established"  // Media transport (ICE + DTLS) connected
	PeerStateDisconnected PeerState = "disconnected" // Terminal
	PeerStateFailed       PeerState = "failed"
)

// allowedTransitions is the shared peer lifecycle state machine (Constitution V).
// Established may be reached from Connecting because the transport can
// connect before the adapter reports the room join.
var allowedTransitions = map[PeerState][]PeerState{
	PeerStateNew:         {PeerStateConnecting, PeerStateFailed, PeerStateDisconnected},
	PeerStateConnecting:  {PeerStateConnected, PeerStateEstablished, PeerStateFailed, PeerStateDisconnected},
	PeerStateConnected:   {PeerStateEstablished, PeerStateFailed, PeerStateDisconnected},
	PeerStateEstablished: {PeerStateFailed, PeerStateDisconnected},
	PeerStateFailed:      {PeerStateDisconnected},
}

// defaultGatherTimeout bounds how long OnOffer waits for ICE gathering before
// answering; candidates found later are trickled.
const defaultGatherTimeout = time.Second

// Peer encapsulates a WebRTC peer connection and its state.
// It implements platform.SignalingHandler.
type Peer struct {
	id         string
	scenarioID string
	pc         *webrtc.PeerConnection
	logger     *slog.Logger

	gatherTimeout time.Duration
	kinds         []string                               // Media kinds the peer sends, fixed at construction
	receiveLimit  int                                    // Remote streams received per kind (0: no limit)
	localTracks   map[string]*webrtc.TrackLocalStaticRTP // By kind; written by the scenario's media sources
	senders       map[string]*webrtc.RTPSender           // By kind

	stateMu sync.Mutex
	state   PeerState // Guarded by stateMu: written by connect goroutines, Pion callbacks and Engine.Stop
	onState func(to PeerState)

	candMu        sync.Mutex
	remoteSet     bool                      // Guarded by candMu
	pendingRemote []webrtc.ICECandidateInit // Remote candidates received before the offer, guarded by candMu

	negMu sync.Mutex // Serializes OnOffer and Renegotiate

	feedback *transportFeedback // Transport-wide congestion control feedback (research R9 of 005)

	channelsMu sync.Mutex
	channels   []*dataChannel // Opened by OpenDataChannel, guarded by channelsMu

	// SSRCs whose incoming packets are kept: the applied offer's and the
	// peer's own (RTCP about them). Read on every incoming packet.
	allowed        atomic.Pointer[map[uint32]bool]
	localSSRCs     []uint32      // Set before the first offer
	droppedPackets atomic.Uint64 // Packets of streams not allowed
	receivedBytes  atomic.Uint64 // RTP and RTCP bytes that reached the read buffers, kept or not

	mediaMu  sync.Mutex
	inbound  []inboundStream            // Remote streams received, guarded by mediaMu
	offered  map[string]int             // Remote sources per kind in the last applied offer, guarded by mediaMu
	rttSamps map[string][]time.Duration // Round trips not yet collected, guarded by mediaMu

	trickleMu  sync.Mutex
	answered   bool                 // Guarded by trickleMu: answer SDP handed to signaling
	trickle    platform.TrickleFunc // Guarded by trickleMu
	answeredAt time.Time            // Guarded by trickleMu
}

var _ platform.SignalingHandler = (*Peer)(nil)

// PeerMedia is what a peer sends and receives.
type PeerMedia struct {
	Kinds        []string // Media kinds sent ("audio", "video"); none for a receive-only peer
	ReceiveLimit int      // Remote streams received per kind; 0 means no limit
}

// NewPeer creates a new Peer in the "new" state.
func NewPeer(id, scenarioID string, m PeerMedia, logger *slog.Logger) (*Peer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	p := &Peer{
		id:            id,
		scenarioID:    scenarioID,
		state:         PeerStateNew,
		gatherTimeout: defaultGatherTimeout,
		kinds:         m.Kinds,
		receiveLimit:  m.ReceiveLimit,
		offered:       map[string]int{},
		rttSamps:      map[string][]time.Duration{},
		logger:        logger.With(slog.String("scenario_id", scenarioID), slog.String("peer_id", id)),
	}
	p.allowed.Store(&map[uint32]bool{})
	p.feedback = newTransportFeedback()

	// Default codecs and interceptors, so the peer can answer audio/video
	// offers; see newPeerConnection for the receive settings.
	pc, err := newPeerConnection(p)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}
	p.pc = pc
	if err := p.addLocalTracks(); err != nil {
		pc.Close()
		return nil, err
	}
	peersByState.WithLabelValues(scenarioID, string(PeerStateNew)).Inc()
	pc.OnConnectionStateChange(p.onConnectionStateChange)
	pc.OnICECandidate(p.onLocalCandidate)
	pc.OnTrack(p.onTrack)
	return p, nil
}

// addLocalTracks adds one track per sent kind before any offer, so that the
// answer reuses their transceivers for the offered sections (sendrecv).
func (p *Peer) addLocalTracks() error {
	p.localTracks = map[string]*webrtc.TrackLocalStaticRTP{}
	p.senders = map[string]*webrtc.RTPSender{}
	for _, kind := range p.kinds {
		content, err := media.LoadContent(kind)
		if err != nil {
			return err
		}
		track, err := webrtc.NewTrackLocalStaticRTP(content.Codec, kind, p.id)
		if err != nil {
			return fmt.Errorf("failed to create %s track: %w", kind, err)
		}
		sender, err := p.pc.AddTrack(track)
		if err != nil {
			return fmt.Errorf("failed to add %s track: %w", kind, err)
		}
		p.localTracks[kind] = track
		p.senders[kind] = sender
		for _, enc := range sender.GetParameters().Encodings {
			p.localSSRCs = append(p.localSSRCs, uint32(enc.SSRC))
		}
		go p.readRTCP(kind, sender)
	}
	return nil
}

// LocalTracks returns the peer's sending tracks by kind, for the scenario's
// media sources to write to once the transport is established.
func (p *Peer) LocalTracks() map[string]*webrtc.TrackLocalStaticRTP {
	return p.localTracks
}

// readRTCP reads the RTCP the remote side sends about a local track (Pion's
// interceptors, e.g. the NACK responder, only see packets that are read) and
// takes round-trip samples from its reception reports.
func (p *Peer) readRTCP(kind string, sender *webrtc.RTPSender) {
	for {
		packets, _, err := sender.ReadRTCP()
		if err != nil {
			return // Connection closed
		}
		var ssrc uint32
		if enc := sender.GetParameters().Encodings; len(enc) > 0 {
			ssrc = uint32(enc[0].SSRC)
		}
		now := time.Now()
		for _, pkt := range packets {
			var reports []rtcp.ReceptionReport
			switch r := pkt.(type) {
			case *rtcp.ReceiverReport:
				reports = r.Reports
			case *rtcp.SenderReport:
				reports = r.Reports
			}
			for _, rr := range reports {
				if rr.SSRC != ssrc {
					continue
				}
				if rtt, ok := media.ReportRoundTrip(now, rr.LastSenderReport, rr.Delay); ok {
					p.mediaMu.Lock()
					p.rttSamps[kind] = append(p.rttSamps[kind], rtt)
					p.mediaMu.Unlock()
				}
			}
		}
	}
}

// inboundStream is a received remote stream and its accounting.
type inboundStream struct {
	kind  string
	stats *media.InboundStats
}

// onTrack reads a remote track until it ends, accounting for every packet.
// Payloads are not decoded (constitution IV).
func (p *Peer) onTrack(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	kind := track.Kind().String()
	stats := media.NewInboundStats(track.Codec().ClockRate)
	p.mediaMu.Lock()
	p.inbound = append(p.inbound, inboundStream{kind: kind, stats: stats})
	p.mediaMu.Unlock()
	p.logger.Debug("Receiving remote stream", slog.String("kind", kind), slog.Uint64("ssrc", uint64(track.SSRC())))

	go func() {
		buf := make([]byte, 1500)
		var h rtp.Header
		for {
			n, _, err := track.Read(buf)
			if err != nil {
				return // Track ended or connection closed
			}
			hlen, err := h.Unmarshal(buf[:n])
			if err != nil {
				continue
			}
			payload := n - hlen
			if h.Padding && n > 0 {
				payload -= int(buf[n-1])
			}
			stats.Update(h.SequenceNumber, h.Timestamp, payload, time.Now())
		}
	}()
}

// recordOffered counts, per kind, the remote sources (media sections with
// SSRCs that the remote side sends) of an offer about to be applied, and
// allows their SSRCs (packets of other SSRCs are dropped, research R12).
func (p *Peer) recordOffered(offer string) {
	parsed := &sdp.SessionDescription{}
	if err := parsed.Unmarshal([]byte(offer)); err != nil {
		return
	}
	offered := map[string]int{}
	allowed := map[uint32]bool{}
	for _, ssrc := range p.localSSRCs {
		allowed[ssrc] = true
	}
	for _, md := range parsed.MediaDescriptions {
		if _, ok := md.Attribute("ssrc"); !ok {
			continue
		}
		if _, ok := md.Attribute("inactive"); ok {
			continue
		}
		if _, ok := md.Attribute("recvonly"); ok {
			continue
		}
		offered[md.MediaName.Media]++
		for _, a := range md.Attributes {
			if a.Key != "ssrc" {
				continue
			}
			id, _, _ := strings.Cut(a.Value, " ")
			if ssrc, err := strconv.ParseUint(id, 10, 32); err == nil {
				allowed[uint32(ssrc)] = true
			}
		}
	}
	p.allowed.Store(&allowed)
	p.feedback.setExtension(parsed)
	if p.feedback.extID.Load() != 0 && p.pc != nil {
		p.feedback.run(p.pc.WriteRTCP)
	}
	p.mediaMu.Lock()
	p.offered = offered
	p.mediaMu.Unlock()
}

// allowedSSRC reports whether incoming packets of ssrc are kept.
func (p *Peer) allowedSSRC(ssrc uint32) bool {
	return (*p.allowed.Load())[ssrc]
}

// KindMediaSnapshot is the received media of one kind, for the scenario's
// collector. Counters are cumulative over the peer's life.
type KindMediaSnapshot struct {
	Packets      uint64
	Bytes        uint64
	Lost         int64
	Jitters      []float64       // Seconds, one per stream that received within the last 5 s
	Streams      int             // Remote streams that received at least one packet
	Offered      int             // Remote sources in the current offer
	LastReceived time.Time       // Zero if nothing received yet
	RoundTrips   []time.Duration // Samples since the previous snapshot (drained)
}

// recentStream bounds how long after its last packet a stream still
// contributes a jitter sample.
const recentStream = 5 * time.Second

// MediaSnapshot returns the received media per kind ("audio", "video") at
// now, and drains the round-trip samples.
func (p *Peer) MediaSnapshot(now time.Time) map[string]KindMediaSnapshot {
	p.mediaMu.Lock()
	inbound := append([]inboundStream(nil), p.inbound...)
	out := map[string]KindMediaSnapshot{}
	for _, kind := range []string{media.KindAudio, media.KindVideo} {
		out[kind] = KindMediaSnapshot{Offered: p.offered[kind], RoundTrips: p.rttSamps[kind]}
	}
	p.rttSamps = map[string][]time.Duration{}
	p.mediaMu.Unlock()

	for _, in := range inbound {
		k, ok := out[in.kind]
		if !ok {
			continue
		}
		snap := in.stats.Snapshot()
		k.Packets += snap.Packets
		k.Bytes += snap.Bytes
		k.Lost += snap.Lost
		if snap.Packets > 0 {
			k.Streams++
			if snap.LastArrival.After(k.LastReceived) {
				k.LastReceived = snap.LastArrival
			}
			if now.Sub(snap.LastArrival) < recentStream {
				k.Jitters = append(k.Jitters, snap.Jitter)
			}
		}
		out[in.kind] = k
	}
	return out
}

// logUnusedTracks reports sent kinds the offer has no section for.
func (p *Peer) logUnusedTracks() {
	for kind, sender := range p.senders {
		for _, tr := range p.pc.GetTransceivers() {
			if tr.Sender() == sender && tr.Mid() == "" {
				p.logger.Warn("Offer has no section for a sent media kind; not sending it", slog.String("kind", kind))
			}
		}
	}
}

// ID returns the peer's ID.
func (p *Peer) ID() string {
	return p.id
}

// State returns the peer's current lifecycle state.
func (p *Peer) State() PeerState {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return p.state
}

// transition moves the peer to state to, or returns an error (leaving the
// state unchanged) if the lifecycle does not allow it.
func (p *Peer) transition(to PeerState) error {
	p.stateMu.Lock()
	err := p.transitionLocked(to)
	p.stateMu.Unlock()
	if err == nil {
		p.notifyState(to)
	}
	return err
}

// OnStateChange registers a function called (outside the peer's locks)
// after each lifecycle transition. Set it before the peer connects.
func (p *Peer) OnStateChange(f func(to PeerState)) {
	p.onState = f
}

func (p *Peer) notifyState(to PeerState) {
	if p.onState != nil {
		p.onState(to)
	}
}

// transitionIf moves the peer from state from to state to. It reports false,
// without error, if the peer is not in state from.
func (p *Peer) transitionIf(from, to PeerState) (bool, error) {
	p.stateMu.Lock()
	if p.state != from {
		p.stateMu.Unlock()
		return false, nil
	}
	err := p.transitionLocked(to)
	p.stateMu.Unlock()
	if err == nil {
		p.notifyState(to)
	}
	return true, err
}

func (p *Peer) transitionLocked(to PeerState) error {
	from := p.state
	for _, allowed := range allowedTransitions[from] {
		if allowed == to {
			p.state = to
			peersByState.WithLabelValues(p.scenarioID, string(from)).Dec()
			peersByState.WithLabelValues(p.scenarioID, string(to)).Inc()
			p.logger.Debug("Peer state changed", slog.String("from", string(from)), slog.String("to", string(to)))
			return nil
		}
	}
	return fmt.Errorf("illegal peer state transition %s -> %s", from, to)
}

// fail moves the peer to Failed, counting the failure under stage.
func (p *Peer) fail(stage string, cause error) {
	peerFailures.WithLabelValues(p.scenarioID, stage).Inc()
	p.logger.Error("Peer failed", slog.String("stage", stage), slog.String("error", cause.Error()))
	if err := p.transition(PeerStateFailed); err != nil {
		p.logger.Error("Peer lifecycle error", slog.String("error", err.Error()))
	}
}

// Close releases the peer connection and its media resources.
func (p *Peer) Close() error {
	p.feedback.close()
	err := p.pc.Close()
	p.channelsMu.Lock()
	for _, c := range p.channels {
		c.uncount()
	}
	p.channelsMu.Unlock()
	return err
}

// OnOffer handles an incoming SDP offer and returns the SDP answer.
func (p *Peer) OnOffer(sdp string, trickle platform.TrickleFunc) (string, error) {
	start := time.Now()
	p.logger.Info("Received SDP offer")

	answer, err := p.answer(sdp, trickle)
	if err != nil {
		peerFailures.WithLabelValues(p.scenarioID, stageNegotiation).Inc()
		return "", err
	}
	negotiationSeconds.WithLabelValues(p.scenarioID).Observe(time.Since(start).Seconds())
	return answer, nil
}

func (p *Peer) answer(sdp string, trickle platform.TrickleFunc) (string, error) {
	p.negMu.Lock()
	defer p.negMu.Unlock()
	p.recordOffered(sdp)
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}); err != nil {
		return "", fmt.Errorf("failed to set remote description: %w", err)
	}

	p.candMu.Lock()
	p.remoteSet = true
	pending := p.pendingRemote
	p.pendingRemote = nil
	p.candMu.Unlock()
	for _, c := range pending {
		if err := p.pc.AddICECandidate(c); err != nil {
			p.logger.Warn("Failed to apply buffered remote candidate", slog.String("error", err.Error()))
		}
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create answer: %w", err)
	}
	p.logUnusedTracks()

	gathered := webrtc.GatheringCompletePromise(p.pc)
	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}
	select {
	case <-gathered:
	case <-time.After(p.gatherTimeout):
		p.logger.Info("ICE gathering not complete, remaining candidates will be trickled")
	}

	// Snapshot the answer (with every candidate gathered so far) and switch
	// to trickling atomically, so each candidate is either in the SDP or
	// trickled (onLocalCandidate runs after the candidate is gathered).
	p.trickleMu.Lock()
	local := p.pc.LocalDescription()
	p.answered = true
	p.trickle = trickle
	p.answeredAt = time.Now()
	p.trickleMu.Unlock()

	return local.SDP, nil
}

// OpenDataChannel opens a data channel; it is established with the
// transport. Text messages and events go to h; incoming messages are also
// counted (debug log). While open, the channel is counted in
// webrtc_data_channels_open.
func (p *Peer) OpenDataChannel(label, protocol string, h platform.DataChannelHandler) (platform.DataChannel, error) {
	dc, err := p.pc.CreateDataChannel(label, &webrtc.DataChannelInit{Protocol: &protocol})
	if err != nil {
		return nil, fmt.Errorf("failed to open data channel %q: %w", label, err)
	}
	c := &dataChannel{dc: dc, scenarioID: p.scenarioID}
	p.channelsMu.Lock()
	p.channels = append(p.channels, c)
	p.channelsMu.Unlock()

	logger := p.logger.With(slog.String("data_channel", label))
	var messages atomic.Int64
	dc.OnOpen(func() {
		logger.Info("Data channel open")
		c.count()
		h.OnOpen()
	})
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		if n := messages.Add(1); n == 1 || n%100 == 0 {
			logger.Debug("Data channel messages received", slog.Int64("count", n))
		}
		if m.IsString {
			h.OnMessage(string(m.Data))
		}
	})
	dc.OnClose(func() {
		logger.Debug("Data channel closed", slog.Int64("messages", messages.Load()))
		c.uncount()
		h.OnClose()
	})
	return c, nil
}

// dataChannel implements platform.DataChannel over Pion's.
type dataChannel struct {
	dc         *webrtc.DataChannel
	scenarioID string
	counted    atomic.Bool // In webrtc_data_channels_open
}

func (c *dataChannel) SendText(msg string) error {
	return c.dc.SendText(msg)
}

func (c *dataChannel) count() {
	if c.counted.CompareAndSwap(false, true) {
		dataChannelsOpen.WithLabelValues(c.scenarioID).Inc()
	}
}

func (c *dataChannel) uncount() {
	if c.counted.CompareAndSwap(true, false) {
		dataChannelsOpen.WithLabelValues(c.scenarioID).Dec()
	}
}

// TransportBytesReceived returns the RTP and RTCP bytes (decrypted) that
// reached the peer, whatever they carry: kept or dropped streams, padding,
// reports (research R5 of 005).
func (p *Peer) TransportBytesReceived() uint64 {
	return p.receivedBytes.Load()
}

// LocalMediaKinds returns the media kinds the peer sends.
func (p *Peer) LocalMediaKinds() []string {
	return p.kinds
}

// ReceiveLimit returns the maximum number of remote streams received per
// media kind (0: no limit).
func (p *Peer) ReceiveLimit() int {
	return p.receiveLimit
}

// Renegotiate applies an updated remote offer (remote sources added or
// removed) and returns the answer. Candidates are not re-gathered (same ICE
// credentials) and no negotiation metric is recorded.
func (p *Peer) Renegotiate(offer string) (string, error) {
	p.negMu.Lock()
	defer p.negMu.Unlock()
	p.recordOffered(offer)
	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		return "", fmt.Errorf("failed to set remote description: %w", err)
	}
	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create answer: %w", err)
	}
	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}
	return p.pc.LocalDescription().SDP, nil
}

// onLocalCandidate trickles candidates gathered after the answer was returned.
func (p *Peer) onLocalCandidate(c *webrtc.ICECandidate) {
	if c == nil {
		return // Gathering complete
	}
	p.trickleMu.Lock()
	answered, trickle := p.answered, p.trickle
	p.trickleMu.Unlock()
	if !answered || trickle == nil {
		return // Will be embedded in the answer SDP
	}
	init := c.ToJSON()
	mid := ""
	if init.SDPMid != nil {
		mid = *init.SDPMid
	}
	trickle(mid, init.Candidate)
}

// OnICECandidate handles an incoming ICE candidate. Candidates received
// before the offer are buffered and applied once it is set.
func (p *Peer) OnICECandidate(mid, candidate string) error {
	p.logger.Debug("Received ICE candidate", slog.String("candidate", candidate))
	init := webrtc.ICECandidateInit{Candidate: candidate}
	if mid != "" {
		init.SDPMid = &mid
	}

	p.candMu.Lock()
	if !p.remoteSet {
		p.pendingRemote = append(p.pendingRemote, init)
		p.candMu.Unlock()
		return nil
	}
	p.candMu.Unlock()

	if err := p.pc.AddICECandidate(init); err != nil {
		return fmt.Errorf("failed to add ICE candidate: %w", err)
	}
	return nil
}

// OnSignalingLost marks the peer failed when its signaling connection drops.
func (p *Peer) OnSignalingLost(err error) {
	p.fail(stageSignalingLost, err)
}

// onConnectionStateChange maps the media transport state (ICE + DTLS) to the
// peer lifecycle.
func (p *Peer) onConnectionStateChange(state webrtc.PeerConnectionState) {
	p.logger.Info("Peer connection state changed", slog.String("transport_state", state.String()))
	switch state {
	case webrtc.PeerConnectionStateConnected:
		if p.State() == PeerStateEstablished {
			return // Recovered from a transient "disconnected" state
		}
		p.trickleMu.Lock()
		answeredAt := p.answeredAt
		p.trickleMu.Unlock()
		if !answeredAt.IsZero() {
			transportConnectSeconds.WithLabelValues(p.scenarioID).Observe(time.Since(answeredAt).Seconds())
		}
		if err := p.transition(PeerStateEstablished); err != nil {
			p.logger.Error("Peer lifecycle error", slog.String("error", err.Error()))
		}
	case webrtc.PeerConnectionStateFailed:
		p.fail(stageTransport, fmt.Errorf("ICE/DTLS transport failed"))
	}
}
