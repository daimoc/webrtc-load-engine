package execution

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtp"
	"github.com/pion/transport/v2/packetio"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/scenario"
)

// ssrcRecorder is a SettingEngine buffer factory that records the SSRCs of
// the RTP streams that arrive.
type ssrcRecorder struct {
	mu    sync.Mutex
	ssrcs map[uint32]bool
}

func (r *ssrcRecorder) factory(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
	if packetType == packetio.RTPBufferPacket {
		r.mu.Lock()
		r.ssrcs[ssrc] = true
		r.mu.Unlock()
	}
	return packetio.NewBuffer()
}

func (r *ssrcRecorder) has(ssrcs []uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range ssrcs {
		if !r.ssrcs[s] {
			return false
		}
	}
	return true
}

// Research R6 risk: a Pion sender with RID encodings, answering an offer
// that has no a=rid or a=simulcast (as Jicofo's), lists every encoding's
// SSRC in its answer and sends each of them.
func TestPeer_SimulcastEncodingsBound(t *testing.T) {
	rec := &ssrcRecorder{ssrcs: map[uint32]bool{}}
	var se webrtc.SettingEngine
	se.BufferFactory = rec.factory
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	offerer, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer offerer.Close()
	if _, err := offerer.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionRecvonly}); err != nil {
		t.Fatal(err)
	}

	answerer, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer answerer.Close()
	codec := webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}
	var tracks []*webrtc.TrackLocalStaticRTP
	var sender *webrtc.RTPSender
	for _, rid := range []string{"l", "m", "h"} {
		tr, err := webrtc.NewTrackLocalStaticRTP(codec, "video", "bot", webrtc.WithRTPStreamID(rid))
		if err != nil {
			t.Fatal(err)
		}
		if sender == nil {
			sender, err = answerer.AddTrack(tr)
		} else {
			err = sender.AddEncoding(tr)
		}
		if err != nil {
			t.Fatalf("encoding %s: %v", rid, err)
		}
		tracks = append(tracks, tr)
	}

	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(offer.SDP, "a=rid") || strings.Contains(offer.SDP, "a=simulcast") {
		t.Fatal("the offer must not mention simulcast")
	}
	gathered := webrtc.GatheringCompletePromise(offerer)
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-gathered
	if err := answerer.SetRemoteDescription(*offerer.LocalDescription()); err != nil {
		t.Fatal(err)
	}
	answer, err := answerer.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered = webrtc.GatheringCompletePromise(answerer)
	if err := answerer.SetLocalDescription(answer); err != nil {
		t.Fatal(err)
	}
	<-gathered
	if err := offerer.SetRemoteDescription(*answerer.LocalDescription()); err != nil {
		t.Fatal(err)
	}

	var ssrcs []uint32
	for _, enc := range sender.GetParameters().Encodings {
		ssrcs = append(ssrcs, uint32(enc.SSRC))
	}
	if len(ssrcs) != 3 {
		t.Fatalf("sender has %d encodings, want 3", len(ssrcs))
	}
	sdp := answerer.LocalDescription().SDP
	for _, s := range ssrcs {
		if !strings.Contains(sdp, "a=ssrc:"+itoa(s)+" ") {
			t.Errorf("SSRC %d not in the answer:\n%s", s, sdp)
		}
	}

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		var seq uint16
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				seq++
				for _, tr := range tracks {
					tr.WriteRTP(&rtp.Packet{Header: rtp.Header{Version: 2, SequenceNumber: seq, Timestamp: uint32(seq) * 3000}, Payload: []byte{0x10, 0x01, 2, 3}})
				}
			}
		}
	}()
	deadline := time.Now().Add(10 * time.Second)
	for !rec.has(ssrcs) {
		if time.Now().After(deadline) {
			t.Fatalf("the offerer did not receive all of %v: %v", ssrcs, rec.ssrcs)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func itoa(v uint32) string {
	var b [10]byte
	i := len(b)
	for {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
		if v == 0 {
			return string(b[i:])
		}
	}
}

// A simulcast peer answers with its three layer SSRCs and their SIM group,
// in OnOffer and in Renegotiate; a peer without simulcast has no group.
func TestPeer_SimulcastAnswer(t *testing.T) {
	offerer := newOfferer(t)
	p, err := NewPeer("peer-sim", "scenario-sim", PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}, Simulcast: true, RequestedHeight: 180}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	if h := p.RequestedVideoHeight(); h != 180 {
		t.Fatalf("requested height %d, want 180", h)
	}
	if n := len(p.LocalLayerTracks()); n != 3 {
		t.Fatalf("%d layer tracks, want 3", n)
	}
	p.transition(PeerStateConnecting)
	answer := negotiate(t, offerer, p)
	checkGroup := func(sdp string) {
		t.Helper()
		video := mediaSection(sdp, "m=video")
		want := "a=ssrc-group:SIM " + itoa(p.layerSSRCs[0]) + " " + itoa(p.layerSSRCs[1]) + " " + itoa(p.layerSSRCs[2])
		if !strings.Contains(video, want+"\r\n") {
			t.Fatalf("no %q in the video section:\n%s", want, video)
		}
		for _, s := range p.layerSSRCs {
			if !strings.Contains(video, "a=ssrc:"+itoa(s)+" ") {
				t.Fatalf("SSRC %d missing:\n%s", s, video)
			}
		}
		if strings.Contains(mediaSection(sdp, "m=audio"), "ssrc-group") {
			t.Fatal("SIM group in the audio section")
		}
	}
	checkGroup(answer)
	again, err := p.Renegotiate(offerer.LocalDescription().SDP)
	if err != nil {
		t.Fatal(err)
	}
	checkGroup(again)

	plain, err := NewPeer("peer-plain", "scenario-sim", PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { plain.Close() })
	if plain.RequestedVideoHeight() != 360 || plain.LocalLayerTracks() != nil {
		t.Fatal("peer without simulcast: unexpected layers or height")
	}
	if a := negotiate(t, newOfferer(t), plain); strings.Contains(a, "ssrc-group:SIM") {
		t.Fatal("SIM group without simulcast")
	}
}

// Bots ask for heights by share, in launch order.
func TestEngine_RequestedHeights(t *testing.T) {
	plan := scenario.TestPlan{Participants: 4, DurationSeconds: 1, Media: &scenario.MediaConfig{
		Profile:          scenario.MediaProfileAudioVideo,
		Simulcast:        &scenario.SimulcastConfig{MaxHeight: 360},
		RequestedHeights: []scenario.RequestedHeight{{Height: 360, Share: 0.5}, {Height: 90, Share: 0.5}},
	}}
	e := NewEngine("heights", plan, nil, quietLogger, func(string, string, *scenario.QualitySummary) {})
	var got []int
	for i := 0; i < 4; i++ {
		m := e.peerMedia(i)
		if !m.Simulcast {
			t.Fatal("simulcast not set")
		}
		got = append(got, m.RequestedHeight)
	}
	if fmt.Sprint(got) != "[360 360 90 90]" {
		t.Fatalf("heights %v", got)
	}
}

// A receiver whose stream switches from the 360 layer to the 180 layer (as
// the media server does) reports both heights and one switch.
func TestPeer_SimulcastLayersReceived(t *testing.T) {
	offerer, track, _ := videoOfferer(t)
	scenarioID := "scenario-switch-" + uuid.NewString()
	p := connectedReceiver(t, offerer, scenarioID, PeerMedia{})
	layers, err := media.LoadLayers(360)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	high, low := media.NewSource(scenarioID, layers[2]), media.NewSource(scenarioID, layers[1])
	go high.Run(ctx)
	go low.Run(ctx)

	high.Subscribe(track)
	waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return len(s["video"].Video) == 1 && s["video"].Video[0].Height == 360
	}, "the 360 layer")
	high.Unsubscribe(track)
	low.Subscribe(track)
	snap := waitMedia(t, p, func(s map[string]KindMediaSnapshot) bool {
		return len(s["video"].Video) == 1 && s["video"].Video[0].Height == 180
	}, "the 180 layer")
	v := snap["video"].Video[0]
	if v.Switches != 1 || v.HeightTime[360] == 0 {
		t.Fatalf("switches %d, time at 360 %s", v.Switches, v.HeightTime[360])
	}
}
