package execution

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"webrtc-load-engine/src/media"
)

// Peers with a publish session (008 research R4): the local tracks go on a
// session the peer offers; the session the remote side offers only
// receives.

func newPublishingPeer(t *testing.T, scenarioID string, kinds ...string) *Peer {
	t.Helper()
	p, err := NewPeer("peer-pub", scenarioID, PeerMedia{Kinds: kinds, PublishSession: true}, quietLogger)
	if err != nil {
		t.Fatalf("NewPeer: %v", err)
	}
	t.Cleanup(func() { p.Close() })
	return p
}

// publishReceiver stands in for the media server's side of the publish
// session: it answers the peer's offer and reports the kinds it receives.
type publishReceiver struct {
	pc       *webrtc.PeerConnection
	received chan string
}

func newPublishReceiver(t *testing.T) *publishReceiver {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatalf("NewPeerConnection: %v", err)
	}
	t.Cleanup(func() { pc.Close() })
	r := &publishReceiver{pc: pc, received: make(chan string, 4)}
	pc.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, _, err := tr.ReadRTP(); err == nil {
			r.received <- tr.Kind().String()
		}
	})
	return r
}

// answer applies the peer's publish offer and returns the answer with all
// candidates.
func (r *publishReceiver) answer(t *testing.T, offer string) string {
	t.Helper()
	if err := r.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		t.Fatalf("publish receiver SetRemoteDescription: %v", err)
	}
	ans, err := r.pc.CreateAnswer(nil)
	if err != nil {
		t.Fatalf("CreateAnswer: %v", err)
	}
	gathered := webrtc.GatheringCompletePromise(r.pc)
	if err := r.pc.SetLocalDescription(ans); err != nil {
		t.Fatalf("SetLocalDescription: %v", err)
	}
	<-gathered
	return r.pc.LocalDescription().SDP
}

// publish runs the peer's publish offer/answer with r.
func publish(t *testing.T, p *Peer, r *publishReceiver) string {
	t.Helper()
	offer, err := p.CreatePublishOffer(func(string, string) {})
	if err != nil {
		t.Fatalf("CreatePublishOffer: %v", err)
	}
	if err := p.ApplyPublishAnswer(r.answer(t, offer)); err != nil {
		t.Fatalf("ApplyPublishAnswer: %v", err)
	}
	return offer
}

func transportConnectSamples(t *testing.T, scenarioID string) uint64 {
	t.Helper()
	var m dto.Metric
	if err := transportConnectSeconds.WithLabelValues(scenarioID).(prometheus.Histogram).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount()
}

func waitConnected(t *testing.T, pc *webrtc.PeerConnection) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
		if time.Now().After(deadline) {
			t.Fatalf("remote PeerConnection not connected (%s)", pc.ConnectionState())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPeer_PublishSessionSendsAndReceives(t *testing.T) {
	scenarioID := uuid.NewString()
	p := newPublishingPeer(t, scenarioID, media.KindAudio, media.KindVideo)
	p.transition(PeerStateConnecting)

	// The receive session first: connected, but the peer is not
	// established until its publish session is too.
	offerer, remote := sendingOfferer(t)
	answer := negotiate(t, offerer, p)
	for _, prefix := range []string{"m=audio", "m=video"} {
		section := mediaSection(answer, prefix)
		if !strings.Contains(section, "a=recvonly") || strings.Contains(section, " msid:peer-pub ") {
			t.Errorf("receive session %s: expected recvonly without the peer's source, got:\n%s", prefix, section)
		}
	}
	waitConnected(t, offerer)
	time.Sleep(200 * time.Millisecond)
	if s := p.State(); s == PeerStateEstablished {
		t.Fatalf("peer established with only its receive session connected")
	}

	r := newPublishReceiver(t)
	offer := publish(t, p, r)
	for _, prefix := range []string{"m=audio", "m=video"} {
		section := mediaSection(offer, prefix)
		if !strings.Contains(section, "a=sendonly") || !strings.Contains(section, " msid:peer-pub ") {
			t.Errorf("publish offer %s: expected sendonly with the peer's source, got:\n%s", prefix, section)
		}
	}
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	// Local media flows on the publish session.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for kind, track := range p.LocalTracks() {
		c, err := media.LoadContent(kind)
		if err != nil {
			t.Fatal(err)
		}
		src := media.NewSource(scenarioID, c)
		src.Subscribe(track)
		go src.Run(ctx)
	}
	got := map[string]bool{}
	for len(got) < 2 {
		select {
		case kind := <-r.received:
			got[kind] = true
		case <-time.After(10 * time.Second):
			t.Fatalf("publish receiver got only %v", got)
		}
	}

	// Remote media arrives on the receive session.
	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, remote...)
	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return s["audio"].Packets > 10 && s["video"].Packets > 10
	}, "remote audio and video")

	if n := negotiationSamples(t, scenarioID); n != 2 {
		t.Errorf("negotiation_seconds samples = %d, want 2 (one per session)", n)
	}
	if n := transportConnectSamples(t, scenarioID); n != 1 {
		t.Errorf("transport_connect_seconds samples = %d, want 1 (when the last session connects)", n)
	}
}

func TestPeer_PublishSessionFailureFailsPeer(t *testing.T) {
	settingEngineHook = func(se *webrtc.SettingEngine) {
		se.SetICETimeouts(300*time.Millisecond, 600*time.Millisecond, 100*time.Millisecond)
	}
	defer func() { settingEngineHook = nil }()

	p := newPublishingPeer(t, uuid.NewString(), media.KindAudio)
	p.transition(PeerStateConnecting)
	offerer := newOfferer(t)
	negotiate(t, offerer, p)
	r := newPublishReceiver(t)
	publish(t, p, r)
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	r.pc.Close() // The server's publish side goes away
	waitForState(t, p, PeerStateFailed, 10*time.Second)
}

func TestPeer_PublishSessionBuffersEarlyCandidates(t *testing.T) {
	p := newPublishingPeer(t, uuid.NewString(), media.KindAudio)
	p.transition(PeerStateConnecting)
	offerer := newOfferer(t)
	negotiate(t, offerer, p)

	offer, err := p.CreatePublishOffer(func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	// The receiver's answer carries no candidate: they are trickled, and
	// the first arrives before the answer.
	r := newPublishReceiver(t)
	cands := make(chan string, 16)
	r.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c != nil {
			cands <- c.ToJSON().Candidate
		}
	})
	if err := r.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		t.Fatal(err)
	}
	ans, err := r.pc.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.pc.SetLocalDescription(ans); err != nil {
		t.Fatal(err)
	}
	first := <-cands
	if err := p.OnPublishICECandidate("0", first); err != nil {
		t.Fatalf("OnPublishICECandidate before the answer: %v", err)
	}
	if err := p.ApplyPublishAnswer(ans.SDP); err != nil {
		t.Fatal(err)
	}
	go func() {
		for c := range cands {
			p.OnPublishICECandidate("0", c)
		}
	}()
	waitForState(t, p, PeerStateEstablished, 10*time.Second)
}

func TestPeer_LocalVideoSize(t *testing.T) {
	p := newPublishingPeer(t, uuid.NewString(), media.KindAudio, media.KindVideo)
	if w, h := p.LocalVideoSize(); w != 640 || h != 360 {
		t.Errorf("LocalVideoSize() = %d×%d, want 640×360", w, h)
	}
	a := newPublishingPeer(t, uuid.NewString(), media.KindAudio)
	if w, h := a.LocalVideoSize(); w != 0 || h != 0 {
		t.Errorf("audio peer LocalVideoSize() = %d×%d, want 0×0", w, h)
	}
}

func TestPeer_PublishOfferNeedsPublishSession(t *testing.T) {
	p, err := NewPeer("peer-nopub", uuid.NewString(), PeerMedia{Kinds: []string{media.KindAudio}}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.CreatePublishOffer(func(string, string) {}); err == nil {
		t.Error("CreatePublishOffer without a publish session returned no error")
	}
	if err := p.ApplyPublishAnswer("v=0"); err == nil {
		t.Error("ApplyPublishAnswer without a publish session returned no error")
	}
	// A receive-only peer with a publish session has nothing to offer.
	r := newPublishingPeer(t, uuid.NewString())
	if _, err := r.CreatePublishOffer(func(string, string) {}); err == nil {
		t.Error("CreatePublishOffer of a receive-only peer returned no error")
	}
}
