package mediasoup

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"webrtc-load-engine/src/scenario"
)

func TestParseConfig_Valid(t *testing.T) {
	c, err := ParseConfig(map[string]any{
		"url": "http://localhost:4443/", "room": "load010",
		"insecure_tls": true, "unlimited_room_secret": "s3cret", "use_pipe_transports": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.URL != "ws://localhost:4443" || c.Room != "load010" || !c.InsecureTLS || c.UnlimitedRoomSecret != "s3cret" || !c.UsePipeTransports {
		t.Errorf("config = %+v", c)
	}
	if c.Origin != "http://localhost:4443" {
		t.Errorf("default origin = %q, want http://localhost:4443", c.Origin)
	}
}

func TestParseConfig_Defaults(t *testing.T) {
	c, err := ParseConfig(map[string]any{"url": "wss://ms.example.org:4443", "room": "r"})
	if err != nil {
		t.Fatal(err)
	}
	if c.URL != "wss://ms.example.org:4443" || c.InsecureTLS || c.UsePipeTransports || c.UnlimitedRoomSecret != "" {
		t.Errorf("config = %+v", c)
	}
	if c.Origin != "https://ms.example.org:4443" {
		t.Errorf("default origin = %q", c.Origin)
	}
	c, err = ParseConfig(map[string]any{"url": "ws://127.0.0.1:4443", "room": "r", "origin": "http://localhost"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Origin != "http://localhost" {
		t.Errorf("origin = %q", c.Origin)
	}
}

func TestParseConfig_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  map[string]any
		want string
	}{
		"no url":         {map[string]any{"room": "r"}, "platform_config.url is required"},
		"bad scheme":     {map[string]any{"url": "ftp://x", "room": "r"}, `platform_config.url must be a ws://, wss://, http:// or https:// URL without query, got "ftp://x"`},
		"query":          {map[string]any{"url": "ws://x/?roomId=a", "room": "r"}, "without query"},
		"no host":        {map[string]any{"url": "ws://", "room": "r"}, "without query"},
		"no room":        {map[string]any{"url": "ws://x"}, "platform_config.room is required"},
		"empty room":     {map[string]any{"url": "ws://x", "room": ""}, "platform_config.room is required"},
		"unknown key":    {map[string]any{"url": "ws://x", "room": "r", "token": "t"}, `platform_config: unknown key "token" for platform mediasoup`},
		"tls not bool":   {map[string]any{"url": "ws://x", "room": "r", "insecure_tls": "yes"}, "platform_config.insecure_tls must be a boolean"},
		"pipe not bool":  {map[string]any{"url": "ws://x", "room": "r", "use_pipe_transports": 1}, "platform_config.use_pipe_transports must be a boolean"},
		"room not str":   {map[string]any{"url": "ws://x", "room": 3}, "platform_config.room must be a string"},
		"secret not str": {map[string]any{"url": "ws://x", "room": "r", "unlimited_room_secret": true}, "platform_config.unlimited_room_secret must be a string"},
		"bad origin":     {map[string]any{"url": "ws://x", "room": "r", "origin": "ws://x"}, `platform_config.origin must be an http:// or https:// URL, got "ws://x"`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseConfig(tc.cfg)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	cfg := map[string]any{"url": "ws://localhost:4443", "room": "r"}
	if err := Validate(scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Profile: "audio_video", MaxReceivedStreams: 10}}); err != nil {
		t.Errorf("valid plan: %v", err)
	}
	for name, tc := range map[string]struct {
		tp   scenario.TestPlan
		want string
	}{
		"simulcast": {scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{}}},
			"media.simulcast is not supported on platform mediasoup"},
		"requested heights": {scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{RequestedHeights: []scenario.RequestedHeight{{Height: 360}}}},
			"media.requested_heights is not supported on platform mediasoup"},
		"config": {scenario.TestPlan{PlatformConfig: map[string]any{"url": "ws://x"}}, "platform_config.room is required"},
	} {
		t.Run(name, func(t *testing.T) {
			err := Validate(tc.tp)
			if err == nil || err.Error() != tc.want {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestConfig_LogsNoSecret(t *testing.T) {
	c, err := ParseConfig(map[string]any{"url": "ws://x", "room": "r", "unlimited_room_secret": "s3cret-value"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("cfg", slog.Any("config", c), slog.String("s", c.String()))
	if strings.Contains(buf.String(), "s3cret-value") {
		t.Errorf("secret logged: %s", buf.String())
	}
	if !strings.Contains(buf.String(), "has_unlimited_room_secret=true") {
		t.Errorf("log lacks has_unlimited_room_secret: %s", buf.String())
	}
}
