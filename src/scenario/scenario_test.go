package scenario

import (
	"reflect"
	"testing"
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
			if sc.TestPlan.Media == nil || *sc.TestPlan.Media != want {
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
