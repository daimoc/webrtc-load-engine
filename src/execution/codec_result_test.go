package execution

import (
	"reflect"
	"testing"

	"webrtc-load-engine/src/scenario"
)

type fakeCodecPeer struct {
	id       string
	received []string
	mismatch *scenario.CodecMismatch
}

func (f fakeCodecPeer) ID() string { return f.id }
func (f fakeCodecPeer) CodecReport() ([]string, *scenario.CodecMismatch) {
	return f.received, f.mismatch
}

func TestCodecResult(t *testing.T) {
	m := &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, VideoCodec: "vp9"}
	peers := []codecPeer{
		fakeCodecPeer{id: "b", received: []string{"vp9"}},
		fakeCodecPeer{id: "a", received: []string{"vp9"}},
		fakeCodecPeer{id: "c", received: []string{"vp8"}, mismatch: &scenario.CodecMismatch{Bot: "c", Direction: "received", Codec: "vp8"}},
		fakeCodecPeer{id: "d"},
	}
	got := codecResult(m, peers)
	want := &scenario.VideoCodecResult{
		Sent: "vp9", Arrangement: "single",
		ContentBitrateKbps: map[string]int{"360": 200},
		Received:           map[string]int{"vp9": 2, "vp8": 1},
		Mismatch:           []scenario.CodecMismatch{{Bot: "c", Direction: "received", Codec: "vp8"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestCodecResult_DefaultsAndNoVideo(t *testing.T) {
	got := codecResult(&scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, Simulcast: &scenario.SimulcastConfig{MaxHeight: 720}}, nil)
	if got.Sent != "vp8" || got.Arrangement != "simulcast" || len(got.ContentBitrateKbps) != 3 || got.ContentBitrateKbps["720"] != 1500 {
		t.Fatalf("VP8 simulcast: %+v", got)
	}
	if got.Mismatch == nil || got.Received == nil {
		t.Fatal("empty lists must be [] and {}, not null")
	}
	if codecResult(&scenario.MediaConfig{Profile: scenario.MediaProfileAudio}, nil) != nil {
		t.Fatal("an audio run has a codec result")
	}
	if codecResult(nil, nil) != nil {
		t.Fatal("a receive-only run has a codec result")
	}
}

func TestMaxRequestedHeight(t *testing.T) {
	for _, tc := range []struct {
		m    *scenario.MediaConfig
		want int
	}{
		{nil, 360},
		{&scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{MaxHeight: 720}}, 720},
		{&scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{MaxHeight: 720}, RequestedHeights: []scenario.RequestedHeight{{Height: 720, Share: 0.5}, {Height: 180, Share: 0.5}}}, 720},
		{&scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{MaxHeight: 720}, RequestedHeights: []scenario.RequestedHeight{{Height: 180, Share: 1}}}, 180},
	} {
		if got := maxRequestedHeight(tc.m); got != tc.want {
			t.Errorf("maxRequestedHeight(%+v) = %d, want %d", tc.m, got, tc.want)
		}
	}
}
