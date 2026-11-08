package scenario

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"webrtc-load-engine/src/media"
)

func validPlan(media *MediaConfig) TestPlan {
	return TestPlan{Participants: 1, DurationSeconds: 1, Media: media}
}

func TestNewScenario_MediaDefaults(t *testing.T) {
	for name, media := range map[string]*MediaConfig{
		"no media object": nil,
		"empty object":    {},
	} {
		t.Run(name, func(t *testing.T) {
			sc, err := NewScenario("s", "jitsi", validPlan(media))
			if err != nil {
				t.Fatalf("NewScenario: %v", err)
			}
			want := MediaConfig{Profile: MediaProfileReceiveOnly, StallTimeoutSeconds: 10}
			if sc.TestPlan.Media == nil || !reflect.DeepEqual(*sc.TestPlan.Media, want) {
				t.Fatalf("expected media %+v, got %+v", want, sc.TestPlan.Media)
			}
		})
	}
}

func TestNewScenario_MediaValidation(t *testing.T) {
	cases := map[string]struct {
		media   MediaConfig
		wantErr bool
	}{
		"receive_only":     {MediaConfig{Profile: "receive_only"}, false},
		"audio":            {MediaConfig{Profile: "audio"}, false},
		"audio_video":      {MediaConfig{Profile: "audio_video", StallTimeoutSeconds: 3}, false},
		"unknown profile":  {MediaConfig{Profile: "video"}, true},
		"negative timeout": {MediaConfig{Profile: "audio", StallTimeoutSeconds: -1}, true},
		"receive cap":      {MediaConfig{Profile: "audio", MaxReceivedStreams: 10}, false},
		"negative cap":     {MediaConfig{Profile: "audio", MaxReceivedStreams: -1}, true},

		// Simulcast and requested heights (specs/006-media-quality/contracts/test-plan-quality.yaml).
		"simulcast 360":           {MediaConfig{Profile: "audio_video", Simulcast: &SimulcastConfig{MaxHeight: 360}}, false},
		"simulcast 720":           {MediaConfig{Profile: "audio_video", Simulcast: &SimulcastConfig{MaxHeight: 720}}, false},
		"simulcast default":       {MediaConfig{Profile: "audio_video", Simulcast: &SimulcastConfig{}}, false},
		"simulcast without video": {MediaConfig{Profile: "audio", Simulcast: &SimulcastConfig{MaxHeight: 360}}, true},
		"simulcast 480":           {MediaConfig{Profile: "audio_video", Simulcast: &SimulcastConfig{MaxHeight: 480}}, true},
		"heights":                 {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{{360, 0.5}, {90, 0.5}}}, false},
		"heights rounding":        {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{{360, 0.3333}, {180, 0.3333}, {90, 0.3334}}}, false},
		"heights empty":           {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{}}, true},
		"heights zero share":      {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{{360, 1}, {90, 0}}}, true},
		"heights sum":             {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{{360, 0.5}, {90, 0.4}}}, true},
		"heights zero height":     {MediaConfig{Profile: "audio_video", RequestedHeights: []RequestedHeight{{0, 1}}}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			media := tc.media
			_, err := NewScenario("s", "jitsi", validPlan(&media))
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNewScenario_SourceFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		media   MediaConfig
		wantErr bool
	}{
		"audio_video":  {MediaConfig{Profile: "audio_video", SourceFile: file}, false},
		"audio":        {MediaConfig{Profile: "audio", SourceFile: file}, false},
		"simulcast":    {MediaConfig{Profile: "audio_video", SourceFile: file, Simulcast: &SimulcastConfig{MaxHeight: 720}}, false},
		"receive_only": {MediaConfig{Profile: "receive_only", SourceFile: file}, true},
		"missing":      {MediaConfig{Profile: "audio_video", SourceFile: file + ".missing"}, true},
		"directory":    {MediaConfig{Profile: "audio_video", SourceFile: filepath.Dir(file)}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			media := tc.media
			_, err := NewScenario("s", "jitsi", validPlan(&media))
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestMediaConfig_Kinds(t *testing.T) {
	cases := map[string][]string{
		MediaProfileReceiveOnly: nil,
		MediaProfileAudio:       {"audio"},
		MediaProfileAudioVideo:  {"audio", "video"},
	}
	for profile, want := range cases {
		if got := (&MediaConfig{Profile: profile}).Kinds(); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: expected %v, got %v", profile, want, got)
		}
	}
	var none *MediaConfig
	if got := none.Kinds(); got != nil {
		t.Errorf("nil config: expected no kinds, got %v", got)
	}
}

func TestMediaConfig_SimulcastDefault(t *testing.T) {
	sc, err := NewScenario("s", "jitsi", validPlan(&MediaConfig{Profile: MediaProfileAudioVideo, Simulcast: &SimulcastConfig{}}))
	if err != nil {
		t.Fatal(err)
	}
	if got := sc.TestPlan.Media.Simulcast.MaxHeight; got != 360 {
		t.Fatalf("max_height %d, want 360", got)
	}
}

func TestMediaConfig_HeightFor(t *testing.T) {
	var none *MediaConfig
	if h := none.HeightFor(0, 1); h != 360 {
		t.Errorf("nil config: %d", h)
	}
	if h := (&MediaConfig{Profile: MediaProfileAudioVideo}).HeightFor(3, 10); h != 360 {
		t.Errorf("no simulcast, no heights: %d, want 360", h)
	}
	if h := (&MediaConfig{Profile: MediaProfileAudioVideo, Simulcast: &SimulcastConfig{MaxHeight: 720}}).HeightFor(3, 10); h != 720 {
		t.Errorf("simulcast 720: %d, want the top layer", h)
	}
	m := &MediaConfig{Profile: MediaProfileAudioVideo, RequestedHeights: []RequestedHeight{{360, 0.5}, {90, 0.5}}}
	var got []int
	for i := 0; i < 4; i++ {
		got = append(got, m.HeightFor(i, 4))
	}
	if !reflect.DeepEqual(got, []int{360, 360, 90, 90}) {
		t.Errorf("shares 0.5/0.5 over 4 bots: %v", got)
	}
	m = &MediaConfig{Profile: MediaProfileAudioVideo, RequestedHeights: []RequestedHeight{{720, 0.3}, {360, 0.3}, {180, 0.4}}}
	got = got[:0]
	for i := 0; i < 10; i++ {
		got = append(got, m.HeightFor(i, 10))
	}
	if !reflect.DeepEqual(got, []int{720, 720, 720, 360, 360, 360, 180, 180, 180, 180}) {
		t.Errorf("shares 0.3/0.3/0.4 over 10 bots: %v", got)
	}
}

func TestNewScenario_ServerMonitoring(t *testing.T) {
	plan := validPlan(nil)
	sc, err := NewScenario("s", "jitsi", plan)
	if err != nil {
		t.Fatal(err)
	}
	if sc.TestPlan.ServerMonitoring != nil {
		t.Errorf("absent section: got %+v, want nil", sc.TestPlan.ServerMonitoring)
	}

	plan.ServerMonitoring = &ServerMonitoringConfig{Containers: []string{"jvb", "jicofo"}}
	sc, err = NewScenario("s", "jitsi", plan)
	if err != nil {
		t.Fatal(err)
	}
	if got := sc.TestPlan.ServerMonitoring.IntervalSeconds; got != 5 {
		t.Errorf("default interval: %d, want 5", got)
	}

	many := make([]string, 17)
	for i := range many {
		many[i] = string(rune('a' + i))
	}
	cases := map[string]struct {
		cfg     ServerMonitoringConfig
		wantErr string
	}{
		"one container":  {ServerMonitoringConfig{Containers: []string{"jvb"}, IntervalSeconds: 1}, ""},
		"sixteen":        {ServerMonitoringConfig{Containers: many[:16], IntervalSeconds: 60}, ""},
		"no container":   {ServerMonitoringConfig{}, "server_monitoring.containers must list 1 to 16 containers"},
		"seventeen":      {ServerMonitoringConfig{Containers: many}, "server_monitoring.containers must list 1 to 16 containers"},
		"empty name":     {ServerMonitoringConfig{Containers: []string{"jvb", ""}}, "server_monitoring.containers: empty name at index 1"},
		"duplicate":      {ServerMonitoringConfig{Containers: []string{"jvb", "jvb"}}, `server_monitoring.containers: duplicate name "jvb"`},
		"interval 61":    {ServerMonitoringConfig{Containers: []string{"jvb"}, IntervalSeconds: 61}, "server_monitoring.interval_seconds must be between 1 and 60, got 61"},
		"interval below": {ServerMonitoringConfig{Containers: []string{"jvb"}, IntervalSeconds: -1}, "server_monitoring.interval_seconds must be between 1 and 60, got -1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := tc.cfg
			p := validPlan(nil)
			p.ServerMonitoring = &cfg
			_, err := NewScenario("s", "jitsi", p)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && (err == nil || err.Error() != tc.wantErr):
				t.Fatalf("error %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// media.video_codec (specs/014-video-codec-comparison/contracts/test-plan-codec.md).
func TestNewScenario_VideoCodec(t *testing.T) {
	file := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sim := &SimulcastConfig{MaxHeight: 720}
	cases := map[string]struct {
		media   MediaConfig
		wantErr string // "" accepts
	}{
		"vp8":             {MediaConfig{Profile: "audio_video", VideoCodec: "vp8"}, ""},
		"vp9":             {MediaConfig{Profile: "audio_video", VideoCodec: "vp9"}, ""},
		"av1":             {MediaConfig{Profile: "audio_video", VideoCodec: "av1"}, ""},
		"h264":            {MediaConfig{Profile: "audio_video", VideoCodec: "h264"}, ""},
		"absent":          {MediaConfig{Profile: "audio_video"}, ""},
		"unknown":         {MediaConfig{Profile: "audio_video", VideoCodec: "vp10"}, "vp8, vp9, av1, h264"},
		"upper case":      {MediaConfig{Profile: "audio_video", VideoCodec: "VP9"}, "media.video_codec"},
		"audio profile":   {MediaConfig{Profile: "audio", VideoCodec: "vp9"}, "media.video_codec"},
		"receive only":    {MediaConfig{Profile: "receive_only", VideoCodec: "vp8"}, "media.video_codec"},
		"vp9 svc":         {MediaConfig{Profile: "audio_video", VideoCodec: "vp9", Simulcast: sim}, ""},
		"h264 simulcast":  {MediaConfig{Profile: "audio_video", VideoCodec: "h264", Simulcast: sim}, ""},
		"h264 480":        {MediaConfig{Profile: "audio_video", VideoCodec: "h264", Simulcast: &SimulcastConfig{MaxHeight: 480}}, "360 or 720"},
		"file vp9 single": {MediaConfig{Profile: "audio_video", VideoCodec: "vp9", SourceFile: file}, ""},
		"file h264 simul": {MediaConfig{Profile: "audio_video", VideoCodec: "h264", SourceFile: file, Simulcast: sim}, ""},
		"file vp9 svc":    {MediaConfig{Profile: "audio_video", VideoCodec: "vp9", SourceFile: file, Simulcast: sim}, "media.source_file"},
		"file av1 svc":    {MediaConfig{Profile: "audio_video", VideoCodec: "av1", SourceFile: file, Simulcast: sim}, "media.source_file"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			media := tc.media
			sc, err := NewScenario("s", "jitsi", validPlan(&media))
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatal("expected an error")
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("error %q does not contain %q", err, tc.wantErr)
			case err == nil && sc.TestPlan.Media.VideoCodec != tc.media.VideoCodec:
				t.Fatalf("video_codec changed to %q", sc.TestPlan.Media.VideoCodec)
			}
		})
	}
}

func TestMediaConfig_CodecAndArrangement(t *testing.T) {
	sim := &SimulcastConfig{MaxHeight: 360}
	cases := []struct {
		m     *MediaConfig
		codec media.VideoCodec
		arr   media.Arrangement
	}{
		{nil, media.CodecVP8, media.ArrangementSingle},
		{&MediaConfig{}, media.CodecVP8, media.ArrangementSingle},
		{&MediaConfig{Simulcast: sim}, media.CodecVP8, media.ArrangementSimulcast},
		{&MediaConfig{VideoCodec: "av1"}, media.CodecAV1, media.ArrangementSingle},
		{&MediaConfig{VideoCodec: "av1", Simulcast: sim}, media.CodecAV1, media.ArrangementSVC},
		{&MediaConfig{VideoCodec: "h264", Simulcast: sim}, media.CodecH264, media.ArrangementSimulcast},
	}
	for _, c := range cases {
		if got := c.m.Codec(); got != c.codec {
			t.Errorf("%+v: Codec() = %q, want %q", c.m, got, c.codec)
		}
		if got := c.m.Arrangement(); got != c.arr {
			t.Errorf("%+v: Arrangement() = %q, want %q", c.m, got, c.arr)
		}
	}
}

func TestMediaConfig_NoVideoCodecInJSON(t *testing.T) {
	b, err := json.Marshal(MediaConfig{Profile: "audio_video"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "video_codec") {
		t.Fatalf("a plan without video_codec gained it: %s", b)
	}
}
