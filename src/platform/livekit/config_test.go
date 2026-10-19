package livekit

import (
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

func TestParseConfig(t *testing.T) {
	base := func(extra map[string]any) map[string]any {
		m := map[string]any{"url": "ws://localhost:7880", "room": "load008", "api_key": "devkey", "api_secret": "secret"}
		for k, v := range extra {
			if v == nil {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
		return m
	}
	ok := []struct {
		name   string
		config map[string]any
		want   Config
	}{
		{"key and secret", base(nil), Config{URL: "ws://localhost:7880/rtc", Room: "load008", APIKey: "devkey", APISecret: "secret", TokenTTL: 6 * time.Hour}},
		{"https to wss with path", base(map[string]any{"url": "https://lk.example.org/base/"}), Config{URL: "wss://lk.example.org/base/rtc", Room: "load008", APIKey: "devkey", APISecret: "secret", TokenTTL: 6 * time.Hour}},
		{"http to ws", base(map[string]any{"url": "http://h:7880"}), Config{URL: "ws://h:7880/rtc", Room: "load008", APIKey: "devkey", APISecret: "secret", TokenTTL: 6 * time.Hour}},
		{"path already /rtc", base(map[string]any{"url": "wss://h/rtc"}), Config{URL: "wss://h/rtc", Room: "load008", APIKey: "devkey", APISecret: "secret", TokenTTL: 6 * time.Hour}},
		{"token ttl", base(map[string]any{"token_ttl": "30m"}), Config{URL: "ws://localhost:7880/rtc", Room: "load008", APIKey: "devkey", APISecret: "secret", TokenTTL: 30 * time.Minute}},
		{"token", base(map[string]any{"api_key": nil, "api_secret": nil, "token": "tok"}), Config{URL: "ws://localhost:7880/rtc", Room: "load008", Token: "tok"}},
	}
	for _, tt := range ok {
		got, err := ParseConfig(tt.config)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %+v, want %+v", tt.name, got, tt.want)
		}
	}

	bad := []struct {
		name   string
		config map[string]any
		msg    string
	}{
		{"no url", base(map[string]any{"url": nil}), "platform_config.url is required"},
		{"empty url", base(map[string]any{"url": ""}), "platform_config.url is required"},
		{"bad scheme", base(map[string]any{"url": "ftp://h"}), `platform_config.url must be a ws://, wss://, http:// or https:// URL, got "ftp://h"`},
		{"no scheme", base(map[string]any{"url": "localhost:7880"}), `platform_config.url must be a ws://, wss://, http:// or https:// URL, got "localhost:7880"`},
		{"no room", base(map[string]any{"room": nil}), "platform_config.room is required"},
		{"no credentials", base(map[string]any{"api_key": nil, "api_secret": nil}), "platform_config needs api_key and api_secret, or token"},
		{"key without secret", base(map[string]any{"api_secret": nil}), "platform_config.api_key and platform_config.api_secret must be set together"},
		{"secret without key", base(map[string]any{"api_key": nil}), "platform_config.api_key and platform_config.api_secret must be set together"},
		{"token and key", base(map[string]any{"token": "tok"}), "platform_config.token cannot be combined with api_key/api_secret"},
		{"bad ttl", base(map[string]any{"token_ttl": "soon"}), `platform_config.token_ttl must be a positive duration, got "soon"`},
		{"zero ttl", base(map[string]any{"token_ttl": "0s"}), `platform_config.token_ttl must be a positive duration, got "0s"`},
		{"ttl with token", base(map[string]any{"api_key": nil, "api_secret": nil, "token": "tok", "token_ttl": "1h"}), "platform_config.token_ttl applies only with api_key/api_secret"},
		{"unknown key", base(map[string]any{"xmpp_domain": "meet.jitsi"}), `platform_config: unknown key "xmpp_domain" for platform livekit`},
		{"not a string", base(map[string]any{"room": 42}), "platform_config.room must be a string"},
	}
	for _, tt := range bad {
		_, err := ParseConfig(tt.config)
		if err == nil || err.Error() != tt.msg {
			t.Errorf("%s: err = %v, want %q", tt.name, err, tt.msg)
		}
	}
	if _, err := ParseConfig(nil); err == nil || err.Error() != "platform_config.url is required" {
		t.Errorf("nil config: err = %v", err)
	}
}

func TestConfig_LogValueHidesSecrets(t *testing.T) {
	c := Config{URL: "ws://h/rtc", Room: "r", APIKey: "k", APISecret: "s3cret", Token: "t0ken"}
	for _, s := range []string{c.String(), c.LogValue().String()} {
		for _, secret := range []string{"s3cret", "t0ken"} {
			if contains(s, secret) {
				t.Errorf("%q shows %q", s, secret)
			}
		}
	}
}

func TestValidate(t *testing.T) {
	cfg := map[string]any{"url": "ws://localhost:7880", "room": "r", "api_key": "k", "api_secret": "s"}
	plan := func(m *scenario.MediaConfig) scenario.TestPlan {
		return scenario.TestPlan{Participants: 2, DurationSeconds: 10, PlatformConfig: cfg, Media: m}
	}
	if err := Validate(plan(&scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, MaxReceivedStreams: 10})); err != nil {
		t.Errorf("valid plan: %v", err)
	}
	if err := Validate(plan(nil)); err != nil {
		t.Errorf("plan without media: %v", err)
	}
	sim := &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, Simulcast: &scenario.SimulcastConfig{MaxHeight: 360}}
	if err := Validate(plan(sim)); err == nil || err.Error() != "media.simulcast is not supported on platform livekit" {
		t.Errorf("simulcast: err = %v", err)
	}
	heights := &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, RequestedHeights: []scenario.RequestedHeight{{Height: 180, Share: 1}}}
	if err := Validate(plan(heights)); err == nil || err.Error() != "media.requested_heights is not supported on platform livekit" {
		t.Errorf("requested heights: err = %v", err)
	}
	if err := Validate(scenario.TestPlan{Participants: 1, DurationSeconds: 1}); err == nil || err.Error() != "platform_config.url is required" {
		t.Errorf("no config: err = %v", err)
	}
}
