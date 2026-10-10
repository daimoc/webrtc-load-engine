package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"webrtc-load-engine/src/media"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestPeer(t *testing.T) *Peer {
	t.Helper()
	p, err := NewPeer("peer-test", "scenario-test", PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatalf("NewPeer: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// newOfferer returns a remote PeerConnection with an audio, a video and a data
// section, standing in for the platform's media server.
func newOfferer(t *testing.T) *webrtc.PeerConnection {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection: %v", err)
	}
	t.Cleanup(func() { pc.Close() })
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeAudio, webrtc.RTPCodecTypeVideo} {
		if _, err := pc.AddTransceiverFromKind(kind); err != nil {
			t.Fatalf("AddTransceiverFromKind: %v", err)
		}
	}
	if _, err := pc.CreateDataChannel("data", nil); err != nil {
		t.Fatalf("CreateDataChannel: %v", err)
	}
	return pc
}

func waitForState(t *testing.T, p *Peer, want PeerState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("peer did not reach state %s within %s (state %s)", want, timeout, p.State())
}

func TestPeer_TransitionFollowsLifecycle(t *testing.T) {
	p := newTestPeer(t)

	for _, to := range []PeerState{PeerStateConnecting, PeerStateConnected, PeerStateEstablished, PeerStateDisconnected} {
		if err := p.transition(to); err != nil {
			t.Fatalf("transition to %s: %v", to, err)
		}
	}
}

func TestPeer_TransitionRejectsIllegal(t *testing.T) {
	p := newTestPeer(t)
	if err := p.transition(PeerStateDisconnected); err != nil {
		t.Fatalf("transition to disconnected: %v", err)
	}

	if err := p.transition(PeerStateConnected); err == nil {
		t.Fatal("expected an error for disconnected -> connected, got nil")
	}
	if got := p.State(); got != PeerStateDisconnected {
		t.Errorf("illegal transition must not change state, got %s", got)
	}
}

func TestPeer_TransitionIfSkipsWhenNotInFromState(t *testing.T) {
	p := newTestPeer(t)
	p.transition(PeerStateConnecting)
	p.transition(PeerStateEstablished)

	moved, err := p.transitionIf(PeerStateConnecting, PeerStateConnected)
	if moved || err != nil {
		t.Errorf("expected no transition and no error, got moved=%v err=%v", moved, err)
	}
	if got := p.State(); got != PeerStateEstablished {
		t.Errorf("expected state established, got %s", got)
	}
}

func TestPeer_OnSignalingLostMarksFailed(t *testing.T) {
	p := newTestPeer(t)
	p.transition(PeerStateConnecting)
	p.transition(PeerStateConnected)

	p.OnSignalingLost(errors.New("kicked"))

	if got := p.State(); got != PeerStateFailed {
		t.Errorf("expected state failed, got %s", got)
	}
}

func TestPeer_OnOfferAnswersWithCandidatesAndEstablishes(t *testing.T) {
	offerer := newOfferer(t)
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gathered := webrtc.GatheringCompletePromise(offerer)
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gathered

	p := newTestPeer(t)
	p.transition(PeerStateConnecting)
	p.transition(PeerStateConnected)

	answer, err := p.OnOffer(offerer.LocalDescription().SDP, func(string, string) {})
	if err != nil {
		t.Fatalf("OnOffer: %v", err)
	}
	if !strings.Contains(answer, "a=candidate:") {
		t.Errorf("expected gathered candidates in the answer, got:\n%s", answer)
	}
	for _, media := range []string{"m=audio", "m=video", "m=application"} {
		if !strings.Contains(answer, media) {
			t.Errorf("expected %s section in the answer", media)
		}
	}

	if err := offerer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatalf("offerer SetRemoteDescription: %v", err)
	}
	waitForState(t, p, PeerStateEstablished, 10*time.Second)
}

func TestPeer_BuffersRemoteCandidatesReceivedBeforeOffer(t *testing.T) {
	offerer := newOfferer(t)
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	// Offer without gathering: it carries no candidates.
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}

	p := newTestPeer(t)
	const early = "candidate:1 1 udp 2130706431 192.0.2.10 5000 typ host"
	if err := p.OnICECandidate("", early); err != nil {
		t.Fatalf("OnICECandidate before offer must be buffered, got %v", err)
	}
	if _, err := p.OnOffer(offer.SDP, func(string, string) {}); err != nil {
		t.Fatalf("OnOffer: %v", err)
	}

	found := false
	for _, s := range p.pc.GetStats() {
		if c, ok := s.(webrtc.ICECandidateStats); ok && c.Type == webrtc.StatsTypeRemoteCandidate && c.IP == "192.0.2.10" {
			found = true
		}
	}
	if !found {
		t.Error("buffered remote candidate was not applied after the offer")
	}
}

func TestPeer_TricklesCandidatesGatheredAfterAnswer(t *testing.T) {
	offerer := newOfferer(t)
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}

	p := newTestPeer(t)
	p.gatherTimeout = 0 // Answer immediately; candidates arrive afterwards

	var mu sync.Mutex
	var trickled []string
	answer, err := p.OnOffer(offer.SDP, func(_, c string) {
		mu.Lock()
		defer mu.Unlock()
		trickled = append(trickled, c)
	})
	if err != nil {
		t.Fatalf("OnOffer: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(trickled)
		mu.Unlock()
		if n > 0 || strings.Contains(answer, "a=candidate:") {
			for _, c := range trickled {
				if !strings.HasPrefix(c, "candidate:") {
					t.Errorf("trickled candidate must be an SDP candidate attribute value, got %q", c)
				}
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("local candidates were neither embedded in the answer nor trickled")
}

// recordingChannelHandler records a data channel's events.
type recordingChannelHandler struct {
	opened, closed chan struct{}
	messages       chan string
}

func newRecordingChannelHandler() *recordingChannelHandler {
	return &recordingChannelHandler{opened: make(chan struct{}, 4), closed: make(chan struct{}, 4), messages: make(chan string, 4)}
}

func (h *recordingChannelHandler) OnOpen()               { h.opened <- struct{}{} }
func (h *recordingChannelHandler) OnClose()              { h.closed <- struct{}{} }
func (h *recordingChannelHandler) OnMessage(text string) { h.messages <- text }

// The data channel opened after the answer reaches the remote side, with its
// protocol, once the transport is up; it sends and receives text, reports
// its events and is counted in webrtc_data_channels_open while open
// (contracts/signaling-handler.md of 005).
func TestPeer_OpenDataChannelReachesRemote(t *testing.T) {
	offerer := newOfferer(t)
	remote := make(chan *webrtc.DataChannel, 1)
	remoteMessages := make(chan string, 4)
	offerer.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == "JVB data channel" {
			dc.OnMessage(func(m webrtc.DataChannelMessage) { remoteMessages <- string(m.Data) })
			remote <- dc
		}
	})
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gathered := webrtc.GatheringCompletePromise(offerer)
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gathered

	scenarioID := uuid.NewString()
	p, err := NewPeer("peer-test", scenarioID, PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatalf("NewPeer: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	open := func() float64 { return testutil.ToFloat64(dataChannelsOpen.WithLabelValues(scenarioID)) }

	answer, err := p.OnOffer(offerer.LocalDescription().SDP, func(string, string) {})
	if err != nil {
		t.Fatalf("OnOffer: %v", err)
	}
	h := newRecordingChannelHandler()
	dc, err := p.OpenDataChannel("JVB data channel", "http://jitsi.org/protocols/colibri", h)
	if err != nil {
		t.Fatalf("OpenDataChannel: %v", err)
	}
	if err := dc.SendText("too early"); err == nil {
		t.Error("SendText before open must fail")
	}
	if err := offerer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatalf("offerer SetRemoteDescription: %v", err)
	}

	var rdc *webrtc.DataChannel
	select {
	case rdc = <-remote:
		if rdc.Protocol() != "http://jitsi.org/protocols/colibri" {
			t.Errorf("expected the colibri protocol, got %q", rdc.Protocol())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the data channel did not reach the remote side")
	}
	select {
	case <-h.opened:
	case <-time.After(10 * time.Second):
		t.Fatal("OnOpen was not called")
	}
	if got := open(); got != 1 {
		t.Errorf("webrtc_data_channels_open = %v while open, want 1", got)
	}

	if err := dc.SendText("hello"); err != nil {
		t.Fatalf("SendText: %v", err)
	}
	select {
	case m := <-remoteMessages:
		if m != "hello" {
			t.Errorf("remote received %q, want hello", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the text did not reach the remote side")
	}

	// Wait until the remote end is open too before it sends.
	deadline := time.Now().Add(5 * time.Second)
	for rdc.ReadyState() != webrtc.DataChannelStateOpen && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := rdc.Send([]byte{1, 2, 3}); err != nil {
		t.Fatalf("remote Send: %v", err)
	}
	if err := rdc.SendText("from remote"); err != nil {
		t.Fatalf("remote SendText: %v", err)
	}
	select {
	case m := <-h.messages:
		if m != "from remote" {
			t.Errorf("OnMessage got %q, want the text message only", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnMessage was not called")
	}

	if err := rdc.Close(); err != nil {
		t.Fatalf("remote Close: %v", err)
	}
	select {
	case <-h.closed:
	case <-time.After(10 * time.Second):
		t.Fatal("OnClose was not called")
	}
	if got := open(); got != 0 {
		t.Errorf("webrtc_data_channels_open = %v after close, want 0", got)
	}
	p.Close()
	if got := open(); got != 0 {
		t.Errorf("webrtc_data_channels_open = %v after the peer closed, want 0", got)
	}
}

// Closing the peer with its data channel open removes it from
// webrtc_data_channels_open, once.
func TestPeer_CloseCountsOpenDataChannelOnce(t *testing.T) {
	offerer := newOfferer(t)
	scenarioID := uuid.NewString()
	p, err := NewPeer("peer-test", scenarioID, PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatalf("NewPeer: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	offerer.OnDataChannel(func(*webrtc.DataChannel) {})
	if _, err := offerer.CreateDataChannel("offerer", nil); err != nil {
		t.Fatalf("CreateDataChannel: %v", err)
	}
	h := newRecordingChannelHandler()
	negotiate(t, offerer, p)
	if _, err := p.OpenDataChannel("JVB data channel", "", h); err != nil {
		t.Fatalf("OpenDataChannel: %v", err)
	}
	select {
	case <-h.opened:
	case <-time.After(10 * time.Second):
		t.Fatal("OnOpen was not called")
	}
	p.Close()
	time.Sleep(100 * time.Millisecond) // Let a late OnClose run
	if got := testutil.ToFloat64(dataChannelsOpen.WithLabelValues(scenarioID)); got != 0 {
		t.Errorf("webrtc_data_channels_open = %v after the peer closed, want 0", got)
	}
}

// negotiate runs the offer/answer exchange between offerer and p, with all
// candidates in the SDPs, and returns the answer.
func negotiate(t *testing.T, offerer *webrtc.PeerConnection, p *Peer) string {
	t.Helper()
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatalf("CreateOffer: %v", err)
	}
	gathered := webrtc.GatheringCompletePromise(offerer)
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gathered
	answer, err := p.OnOffer(offerer.LocalDescription().SDP, func(string, string) {})
	if err != nil {
		t.Fatalf("OnOffer: %v", err)
	}
	if err := offerer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatalf("offerer SetRemoteDescription: %v", err)
	}
	return answer
}

// mediaSection returns the SDP media section starting with prefix ("m=audio").
func mediaSection(sdp, prefix string) string {
	i := strings.Index(sdp, prefix)
	if i < 0 {
		return ""
	}
	rest := sdp[i:]
	if j := strings.Index(rest[1:], "\nm="); j >= 0 {
		return rest[:j+1]
	}
	return rest
}

func TestPeer_SendingPeerAnswersSendrecvWithSources(t *testing.T) {
	offerer := newOfferer(t)
	received := make(chan string, 2)
	offerer.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, _, err := tr.ReadRTP(); err == nil {
			received <- tr.Kind().String()
		}
	})

	p, err := NewPeer("peer-send", "scenario-send", PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}}, quietLogger)
	if err != nil {
		t.Fatalf("NewPeer: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	if got := p.LocalMediaKinds(); len(got) != 2 {
		t.Fatalf("expected 2 local kinds, got %v", got)
	}
	p.transition(PeerStateConnecting)
	answer := negotiate(t, offerer, p)

	for _, prefix := range []string{"m=audio", "m=video"} {
		section := mediaSection(answer, prefix)
		if !strings.Contains(section, "a=sendrecv") {
			t.Errorf("%s: expected sendrecv, got:\n%s", prefix, section)
		}
		if !strings.Contains(section, " msid:peer-send ") || !strings.Contains(section, " cname:") {
			t.Errorf("%s: expected a source with msid and cname, got:\n%s", prefix, section)
		}
	}
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for kind, track := range p.LocalTracks() {
		c, err := media.LoadContent(kind)
		if err != nil {
			t.Fatalf("LoadContent: %v", err)
		}
		src := media.NewSource("scenario-send", c)
		src.Subscribe(track)
		go src.Run(ctx)
	}
	got := map[string]bool{}
	for len(got) < 2 {
		select {
		case kind := <-received:
			got[kind] = true
		case <-time.After(10 * time.Second):
			t.Fatalf("remote side received only %v", got)
		}
	}
}

func TestPeer_ReceiveOnlyPeerAnswersRecvonly(t *testing.T) {
	offerer := newOfferer(t)
	p := newTestPeer(t)
	answer := negotiate(t, offerer, p)
	for _, prefix := range []string{"m=audio", "m=video"} {
		section := mediaSection(answer, prefix)
		if !strings.Contains(section, "a=recvonly") || strings.Contains(section, "a=ssrc:") {
			t.Errorf("%s: expected recvonly without sources, got:\n%s", prefix, section)
		}
	}
}

// sendingOfferer returns a remote PeerConnection that sends one audio and one
// video track, standing in for a media server forwarding two participants'
// streams, and the tracks to write to.
func sendingOfferer(t *testing.T) (*webrtc.PeerConnection, []*webrtc.TrackLocalStaticRTP) {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection: %v", err)
	}
	t.Cleanup(func() { pc.Close() })
	var tracks []*webrtc.TrackLocalStaticRTP
	for _, codec := range []webrtc.RTPCodecCapability{
		{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2},
		{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000},
	} {
		tr, err := webrtc.NewTrackLocalStaticRTP(codec, codec.MimeType, "remote")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pc.AddTrack(tr); err != nil {
			t.Fatal(err)
		}
		tracks = append(tracks, tr)
	}
	return pc, tracks
}

// pump writes numbered RTP packets to the tracks until stop is closed.
func pump(stop <-chan struct{}, tracks ...*webrtc.TrackLocalStaticRTP) {
	var seq uint16
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			seq++
			for _, tr := range tracks {
				tr.WriteRTP(&rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: seq, Timestamp: uint32(seq) * 900}, Payload: []byte{0x10, 1, 2, 3}})
			}
		}
	}
}

func negotiationSamples(t *testing.T, scenarioID string) uint64 {
	t.Helper()
	var m dto.Metric
	if err := negotiationSeconds.WithLabelValues(scenarioID).(prometheus.Histogram).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}

func waitMedia(t *testing.T, p *Peer, ok func(map[string]KindMediaSnapshot) bool, what string) map[string]KindMediaSnapshot {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap := p.MediaSnapshot(time.Now())
		if ok(snap) {
			return snap
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s: %+v", what, snap)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPeer_ReceivesAndAccountsRemoteStreams(t *testing.T) {
	offerer, tracks := sendingOfferer(t)
	p, err := NewPeer("peer-recv", "scenario-recv", PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	negotiate(t, offerer, p)
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, tracks...)

	snap := waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return s["audio"].Packets > 10 && s["video"].Packets > 10
	}, "audio and video packets")
	for _, kind := range []string{"audio", "video"} {
		k := snap[kind]
		if k.Bytes != 4*k.Packets || k.Offered != 1 || k.LastReceived.IsZero() {
			t.Errorf("%s: unexpected snapshot %+v", kind, k)
		}
	}

	// A second remote video track arrives through renegotiation, without a
	// new negotiation sample.
	before := negotiationSamples(t, "scenario-recv")
	extra, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "video2", "remote2")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := offerer.AddTrack(extra); err != nil {
		t.Fatal(err)
	}
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	answer, err := p.Renegotiate(offerer.LocalDescription().SDP)
	if err != nil {
		t.Fatalf("Renegotiate: %v", err)
	}
	if err := offerer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatalf("offerer SetRemoteDescription: %v", err)
	}
	go pump(stop, extra)
	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return s["video"].Offered == 2 && s["video"].Streams == 2
	}, "the second video stream")
	if after := negotiationSamples(t, "scenario-recv"); after != before {
		t.Errorf("renegotiation must not record a negotiation sample (%d -> %d)", before, after)
	}
}

// withoutSSRC removes the source lines of SSRC ssrc from an SDP, as an
// adapter does for a remote stream beyond the receive cap.
func withoutSSRC(sdp string, ssrc uint32) string {
	var out []string
	prefix := fmt.Sprintf("a=ssrc:%d ", ssrc)
	for _, line := range strings.Split(sdp, "\r\n") {
		if !strings.HasPrefix(line, prefix) {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\r\n")
}

// Streams whose SSRC the applied offer does not list are dropped as soon as
// they arrive; once an offer lists them, they are received.
func TestPeer_DropsStreamsNotOffered(t *testing.T) {
	offerer, tracks := sendingOfferer(t)
	ignored, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2}, "audio2", "remote2")
	if err != nil {
		t.Fatal(err)
	}
	sender, err := offerer.AddTrack(ignored)
	if err != nil {
		t.Fatal(err)
	}
	ignoredSSRC := uint32(sender.GetParameters().Encodings[0].SSRC)

	p, err := NewPeer("peer-cap", "scenario-cap", PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)

	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(offerer)
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-gathered
	full := offerer.LocalDescription().SDP
	answer, err := p.OnOffer(withoutSSRC(full, ignoredSSRC), func(string, string) {})
	if err != nil {
		t.Fatalf("OnOffer: %v", err)
	}
	if err := offerer.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatal(err)
	}
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, append(tracks, ignored)...)

	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool { return s["audio"].Packets > 20 }, "the offered audio stream")
	if s := p.MediaSnapshot(time.Now())["audio"]; s.Streams != 1 || s.Offered != 1 {
		t.Fatalf("expected only the offered audio stream, got %+v", s)
	}
	deadline := time.Now().Add(5 * time.Second)
	for p.droppedPackets.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("packets of the stream not offered were not dropped")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Offering it (e.g. a promoted stream) makes it received.
	if _, err := p.Renegotiate(full); err != nil {
		t.Fatalf("Renegotiate: %v", err)
	}
	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return s["audio"].Streams == 2 && s["audio"].Offered == 2
	}, "the stream once offered")
}
