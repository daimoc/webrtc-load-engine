package execution

import (
	"fmt"
	"log/slog"
	"slices"
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
	stallTimeout  time.Duration                          // Caps the freezes of received video
	sendTimes     map[string]*media.SendTimes            // By kind: the scenario's send times, for latency
	videoHeight   int                                    // Video height asked for each received video
	localTracks   map[string]*webrtc.TrackLocalStaticRTP // By kind; written by the scenario's media sources
	simulcast     bool                                   // Video sent as simulcastLayers layers
	layerTracks   []*webrtc.TrackLocalStaticRTP          // Video layers, low to high, when simulcast
	layerSSRCs    []uint32                               // Their SSRCs, for the SIM group of the answer
	layerReaders  sync.WaitGroup                         // RTCP readers of the non-base layers
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

	keyframesSent     atomic.Uint64 // PLI sent about received video
	keyframesReceived atomic.Uint64 // PLI and FIR received about the peer's video

	trickleMu  sync.Mutex
	answered   bool                 // Guarded by trickleMu: answer SDP handed to signaling
	trickle    platform.TrickleFunc // Guarded by trickleMu
	answeredAt time.Time            // Guarded by trickleMu
}

var _ platform.SignalingHandler = (*Peer)(nil)

// PeerMedia is what a peer sends and receives.
type PeerMedia struct {
	Kinds        []string      // Media kinds sent ("audio", "video"); none for a receive-only peer
	ReceiveLimit int           // Remote streams received per kind; 0 means no limit
	StallTimeout time.Duration // Longest freeze counted on received video; 0 means 10 s
	// SendTimes are the scenario's send times by kind: received media
	// found there gets a latency (research R5). Nil: no latency.
	SendTimes map[string]*media.SendTimes
	// RequestedHeight is the video height the peer asks the media server
	// for each received video; 0 means 360 (the height sent without
	// simulcast).
	RequestedHeight int
	// Simulcast sends the video as three layers (RIDs l, m, h), announced
	// in the answer as an SSRC SIM group (006 research R6).
	Simulcast bool
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
		stallTimeout:  m.StallTimeout,
		sendTimes:     m.SendTimes,
		videoHeight:   m.RequestedHeight,
		simulcast:     m.Simulcast && slices.Contains(m.Kinds, media.KindVideo),
		offered:       map[string]int{},
		rttSamps:      map[string][]time.Duration{},
		logger:        logger.With(slog.String("scenario_id", scenarioID), slog.String("peer_id", id)),
	}
	if p.videoHeight <= 0 {
		p.videoHeight = 360
	}
	if p.stallTimeout <= 0 {
		p.stallTimeout = 10 * time.Second
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
		if kind == media.KindVideo && p.simulcast {
			if err := p.addLayerTracks(content.Codec); err != nil {
				return err
			}
			continue
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
		go p.readRTCP(kind, sender, "")
	}
	return nil
}

// simulcastRIDs name the video layers, low to high.
var simulcastRIDs = []string{"l", "m", "h"}

// addLayerTracks adds the video as one sender with an encoding per layer;
// each encoding has its own SSRC (006 research R6).
func (p *Peer) addLayerTracks(codec webrtc.RTPCodecCapability) error {
	var sender *webrtc.RTPSender
	for _, rid := range simulcastRIDs {
		track, err := webrtc.NewTrackLocalStaticRTP(codec, media.KindVideo, p.id, webrtc.WithRTPStreamID(rid))
		if err != nil {
			return fmt.Errorf("failed to create video layer %s: %w", rid, err)
		}
		if sender == nil {
			sender, err = p.pc.AddTrack(track)
		} else {
			err = sender.AddEncoding(track)
		}
		if err != nil {
			return fmt.Errorf("failed to add video layer %s: %w", rid, err)
		}
		p.layerTracks = append(p.layerTracks, track)
	}
	p.senders[media.KindVideo] = sender
	for i, enc := range sender.GetParameters().Encodings {
		p.localSSRCs = append(p.localSSRCs, uint32(enc.SSRC))
		p.layerSSRCs = append(p.layerSSRCs, uint32(enc.SSRC))
		if i == 0 {
			go p.readRTCP(media.KindVideo, sender, "") // The base encoding is read with ReadRTCP
			continue
		}
		p.layerReaders.Add(1)
		go func(rid string) {
			defer p.layerReaders.Done()
			p.readRTCP(media.KindVideo, sender, rid)
		}(simulcastRIDs[i])
	}
	return nil
}

// stopLayerReaders ends the RTCP readers of the non-base video layers
// before the connection closes: Pion's ReadSimulcast scans the sender's
// encodings without its lock on every call, which races with the encodings
// being reset when the sender stops. A past read deadline (set under the
// lock) makes their pending and next reads fail, so they return.
func (p *Peer) stopLayerReaders() {
	sender := p.senders[media.KindVideo]
	if sender == nil || len(p.layerTracks) == 0 {
		return
	}
	for _, rid := range simulcastRIDs[1:] {
		_ = sender.SetReadDeadlineSimulcast(time.Unix(1, 0), rid)
	}
	done := make(chan struct{})
	go func() { p.layerReaders.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond): // Not sending yet: readers wait for Send or Stop, without scanning
	}
}

// LocalLayerTracks returns the video layer tracks, low to high, when the
// peer sends simulcast; nil otherwise.
func (p *Peer) LocalLayerTracks() []*webrtc.TrackLocalStaticRTP {
	return p.layerTracks
}

// withSimulcastGroup adds to an answer the SSRC SIM group of the video
// layers, low to high, before the video section's first source, as
// browsers announce SSRC simulcast (006 research R6). Unchanged without
// simulcast.
func (p *Peer) withSimulcastGroup(sdp string) string {
	if len(p.layerSSRCs) < 2 {
		return sdp
	}
	ids := make([]string, len(p.layerSSRCs))
	for i, s := range p.layerSSRCs {
		ids[i] = strconv.FormatUint(uint64(s), 10)
	}
	group := "a=ssrc-group:SIM " + strings.Join(ids, " ")
	first := "a=ssrc:" + ids[0] + " "
	lines := strings.Split(sdp, "\r\n")
	for i, l := range lines {
		if strings.HasPrefix(l, first) {
			lines = append(lines[:i], append([]string{group}, lines[i:]...)...)
			break
		}
	}
	return strings.Join(lines, "\r\n")
}

// LocalTracks returns the peer's sending tracks by kind, for the scenario's
// media sources to write to once the transport is established.
func (p *Peer) LocalTracks() map[string]*webrtc.TrackLocalStaticRTP {
	return p.localTracks
}

// readRTCP reads the RTCP the remote side sends about a local track, or
// about its simulcast encoding rid (Pion's interceptors, e.g. the NACK
// responder, only see packets that are read) and takes round-trip samples
// from its reception reports.
func (p *Peer) readRTCP(kind string, sender *webrtc.RTPSender, rid string) {
	var ssrc uint32
	for _, enc := range sender.GetParameters().Encodings {
		if enc.RID == rid || rid == "" {
			ssrc = uint32(enc.SSRC)
			break
		}
	}
	for {
		var packets []rtcp.Packet
		var err error
		if rid == "" {
			packets, _, err = sender.ReadRTCP()
		} else {
			packets, _, err = sender.ReadSimulcastRTCP(rid)
		}
		if err != nil {
			return // Connection closed
		}
		now := time.Now()
		for _, pkt := range packets {
			var reports []rtcp.ReceptionReport
			switch r := pkt.(type) {
			case *rtcp.ReceiverReport:
				reports = r.Reports
			case *rtcp.SenderReport:
				reports = r.Reports
			case *rtcp.PictureLossIndication, *rtcp.FullIntraRequest:
				if kind == media.KindVideo {
					p.keyframesReceived.Add(1)
				}
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

// inboundStream is a received remote stream, its accounting and its
// quality measures (video or audio, by kind).
type inboundStream struct {
	kind  string
	stats *media.InboundStats
	video *media.VideoQuality
	audio *media.AudioQuality
}

// onTrack reads a remote track until it ends, accounting for every packet
// and measuring its quality. Payloads are not decoded (constitution IV).
func (p *Peer) onTrack(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
	kind := track.Kind().String()
	in := inboundStream{kind: kind, stats: media.NewInboundStats(track.Codec().ClockRate)}
	switch {
	case kind == media.KindVideo && strings.EqualFold(track.Codec().MimeType, webrtc.MimeTypeVP8):
		in.video = media.NewVideoQuality(p.stallTimeout)
	case kind == media.KindAudio:
		in.audio = media.NewAudioQuality(track.Codec().ClockRate, media.OpusFrame)
	}
	if st := p.sendTimes[kind]; st != nil {
		latency := media.Latency.WithLabelValues(p.scenarioID, kind)
		switch {
		case in.video != nil:
			in.video.SetLatency(st, latency)
		case in.audio != nil:
			in.audio.SetLatency(st, latency)
		}
	}
	stats := in.stats
	ssrc := uint32(track.SSRC())
	p.mediaMu.Lock()
	p.inbound = append(p.inbound, in)
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
			now := time.Now()
			stats.Update(h.SequenceNumber, h.Timestamp, payload, now)
			switch {
			case in.video != nil:
				if payload > 0 && in.video.UpdatePacket(h.SequenceNumber, h.Timestamp, buf[hlen:hlen+payload], now) {
					p.requestKeyframe(ssrc)
				}
			case in.audio != nil:
				in.audio.Update(h.SequenceNumber, h.Timestamp, now)
			}
		}
	}()
}

// requestKeyframe sends a picture loss indication about a received stream
// (research R8).
func (p *Peer) requestKeyframe(ssrc uint32) {
	if err := p.pc.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: ssrc}}); err != nil {
		return // Connection closing
	}
	p.keyframesSent.Add(1)
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

	// Quality of the streams that received at least one frame; their period
	// values cover the time since the previous snapshot.
	Video []media.VideoQualitySnapshot
	Audio []media.AudioQualitySnapshot
	// Keyframe requests (video only): PLI sent about received streams, PLI
	// and FIR received about the peer's own video.
	KeyframeRequestsSent     uint64
	KeyframeRequestsReceived uint64
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
		if in.video != nil {
			if q := in.video.Snapshot(true); q.Frames > 0 {
				k.Video = append(k.Video, q)
			}
		}
		if in.audio != nil {
			if q := in.audio.Snapshot(true); q.Expected > 0 {
				k.Audio = append(k.Audio, q)
			}
		}
		out[in.kind] = k
	}
	v := out[media.KindVideo]
	v.KeyframeRequestsSent = p.keyframesSent.Load()
	v.KeyframeRequestsReceived = p.keyframesReceived.Load()
	out[media.KindVideo] = v
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
	p.stopLayerReaders()
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

	return p.withSimulcastGroup(local.SDP), nil
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

// RequestedVideoHeight implements platform.SignalingHandler.
func (p *Peer) RequestedVideoHeight() int {
	return p.videoHeight
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
	return p.withSimulcastGroup(p.pc.LocalDescription().SDP), nil
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
