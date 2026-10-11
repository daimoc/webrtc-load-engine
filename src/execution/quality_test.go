package execution

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/transport/v2/vnet"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"webrtc-load-engine/src/media"
)

// videoOfferer returns a remote PeerConnection sending one VP8 track, and
// the track and its sender.
func videoOfferer(t *testing.T) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticRTP, *webrtc.RTPSender) {
	t.Helper()
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	tr, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "video", "remote")
	if err != nil {
		t.Fatal(err)
	}
	sender, err := pc.AddTrack(tr)
	if err != nil {
		t.Fatal(err)
	}
	return pc, tr, sender
}

// connectedReceiver returns an established receive-only Peer fed by
// offerer.
func connectedReceiver(t *testing.T, offerer *webrtc.PeerConnection, scenarioID string, m PeerMedia) *Peer {
	t.Helper()
	p, err := NewPeer("peer-q", scenarioID, m, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	negotiate(t, offerer, p)
	waitForState(t, p, PeerStateEstablished, 10*time.Second)
	return p
}

// Received video is measured, and a stream that starts on an interframe
// gets one keyframe request (PLI), not more than one per second.
func TestPeer_VideoQualityAndKeyframeRequests(t *testing.T) {
	offerer, track, sender := videoOfferer(t)
	plis := make(chan uint32, 10)
	go func() {
		for {
			pkts, _, err := sender.ReadRTCP()
			if err != nil {
				return
			}
			for _, pkt := range pkts {
				if pli, ok := pkt.(*rtcp.PictureLossIndication); ok {
					plis <- pli.MediaSSRC
				}
			}
		}
	}()
	p := connectedReceiver(t, offerer, "scenario-q-"+uuid.NewString(), PeerMedia{})

	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, track) // Interframes only (frame tag bit 0 set)

	select {
	case ssrc := <-plis:
		if want := uint32(sender.GetParameters().Encodings[0].SSRC); ssrc != want {
			t.Fatalf("PLI for SSRC %d, want %d", ssrc, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no PLI for a stream starting on an interframe")
	}
	select {
	case <-plis:
		t.Fatal("second PLI within 1 s")
	case <-time.After(500 * time.Millisecond):
	}
	snap := waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return len(s["video"].Video) == 1 && s["video"].Video[0].Frames > 10
	}, "video frames")
	if snap["video"].KeyframeRequestsSent < 1 {
		t.Fatalf("keyframe requests sent: %d", snap["video"].KeyframeRequestsSent)
	}
}

// PLI and FIR received about the peer's own video are counted.
func TestPeer_CountsKeyframeRequestsReceived(t *testing.T) {
	offerer := newOfferer(t)
	p, err := NewPeer("peer-kf", "scenario-kf-"+uuid.NewString(), PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	negotiate(t, offerer, p)
	waitForState(t, p, PeerStateEstablished, 10*time.Second)

	ssrc := uint32(p.senders[media.KindVideo].GetParameters().Encodings[0].SSRC)
	if err := offerer.WriteRTCP([]rtcp.Packet{
		&rtcp.PictureLossIndication{MediaSSRC: ssrc},
		&rtcp.FullIntraRequest{MediaSSRC: ssrc, FIR: []rtcp.FIREntry{{SSRC: ssrc, SequenceNumber: 1}}},
	}); err != nil {
		t.Fatal(err)
	}
	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return s["video"].KeyframeRequestsReceived == 2
	}, "2 keyframe requests received")
}

// SC-002: a sender's video interrupted for 1 s gives exactly one freeze of
// 0.9–1.2 s at the receiver, and no stall.
func TestPeer_VideoFreeze(t *testing.T) {
	offerer, track, _ := videoOfferer(t)
	scenarioID := "scenario-freeze-" + uuid.NewString()
	p := connectedReceiver(t, offerer, scenarioID, PeerMedia{StallTimeout: 10 * time.Second})

	content, err := media.LoadContent(media.KindVideo)
	if err != nil {
		t.Fatal(err)
	}
	src := media.NewSource(scenarioID, content)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go src.Run(ctx)

	collector := newMediaCollector(scenarioID, 10*time.Second, quietLogger)
	src.Subscribe(track)
	time.Sleep(time.Second)
	collector.collect(time.Now(), []mediaPeer{p})
	src.Unsubscribe(track)
	time.Sleep(time.Second)
	collector.collect(time.Now(), []mediaPeer{p})
	src.Subscribe(track)
	time.Sleep(time.Second)
	collector.collect(time.Now(), []mediaPeer{p})

	snap := p.MediaSnapshot(time.Now())["video"]
	if len(snap.Video) != 1 {
		t.Fatalf("expected one video stream, got %d", len(snap.Video))
	}
	v := snap.Video[0]
	if v.Freezes != 1 || v.FreezeTime < 900*time.Millisecond || v.FreezeTime > 1200*time.Millisecond {
		t.Fatalf("got %d freezes lasting %s, want 1 of 0.9–1.2 s", v.Freezes, v.FreezeTime)
	}
	if v.Height != 360 {
		t.Fatalf("height %d, want 360", v.Height)
	}
	if got := testutil.ToFloat64(media.Stalls.WithLabelValues(scenarioID, "video")); got != 0 {
		t.Fatalf("stalls %v, want 0", got)
	}
	if got := testutil.ToFloat64(media.Freezes.WithLabelValues(scenarioID)); got != 1 {
		t.Fatalf("webrtc_media_freezes_total %v, want 1", got)
	}
}

// vnetPeerConnection returns a PeerConnection on the virtual network net.
func vnetPeerConnection(t *testing.T, net *vnet.Net) *webrtc.PeerConnection {
	t.Helper()
	var se webrtc.SettingEngine
	se.SetVNet(net)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	return pc
}

// SC-004: through a path delayed by 50 ms, the median latency the receiver
// measures differs from 50 ms by at most 5 ms, for audio and video.
func TestPeer_LatencyThroughDelay(t *testing.T) {
	const delay = 50 * time.Millisecond
	router, err := vnet.NewRouter(&vnet.RouterConfig{CIDR: "10.0.0.0/24", MinDelay: delay, LoggerFactory: logging.NewDefaultLoggerFactory()})
	if err != nil {
		t.Fatal(err)
	}
	nets := make([]*vnet.Net, 2)
	for i := range nets {
		if nets[i], err = vnet.NewNet(&vnet.NetConfig{StaticIPs: []string{fmt.Sprintf("10.0.0.%d", i+1)}}); err != nil {
			t.Fatal(err)
		}
		if err := router.AddNet(nets[i]); err != nil {
			t.Fatal(err)
		}
	}
	if err := router.Start(); err != nil {
		t.Fatal(err)
	}
	defer router.Stop()

	scenarioID := "scenario-latency-" + uuid.NewString()
	offerer := vnetPeerConnection(t, nets[0])
	sources := map[string]*media.Source{}
	sendTimes := map[string]*media.SendTimes{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, kind := range []string{media.KindAudio, media.KindVideo} {
		content, err := media.LoadContent(kind)
		if err != nil {
			t.Fatal(err)
		}
		tr, err := webrtc.NewTrackLocalStaticRTP(content.Codec, kind, "remote")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := offerer.AddTrack(tr); err != nil {
			t.Fatal(err)
		}
		src := media.NewSource(scenarioID, content)
		src.Subscribe(tr)
		sources[kind], sendTimes[kind] = src, src.SendTimes()
	}

	settingEngineHook = func(se *webrtc.SettingEngine) {
		se.SetVNet(nets[1])
		se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	}
	p := connectedReceiver(t, offerer, scenarioID, PeerMedia{SendTimes: sendTimes})
	settingEngineHook = nil
	for _, src := range sources {
		go src.Run(ctx)
	}

	snap := waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return len(s["audio"].Audio) == 1 && s["audio"].Audio[0].Latency.N >= 100 &&
			len(s["video"].Video) == 1 && s["video"].Video[0].Latency.N >= 60
	}, "latency samples")
	for kind, l := range map[string]media.LatencyBuckets{"audio": snap["audio"].Audio[0].Latency, "video": snap["video"].Video[0].Latency} {
		median, _ := l.Quantile(0.5)
		t.Logf("%s: median latency %s over %d samples", kind, median, l.N)
		if d := median - delay; d < -5*time.Millisecond || d > 5*time.Millisecond {
			t.Errorf("%s: median latency %s, want %s ± 5 ms", kind, median, delay)
		}
	}
}
