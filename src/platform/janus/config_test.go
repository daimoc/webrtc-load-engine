package janus

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"webrtc-load-engine/src/scenario"
)

func TestParseConfig_Valid(t *testing.T) {
	c, err := ParseConfig(map[string]any{
		"url": "http://localhost:8188/", "room": float64(1011), "insecure_tls": true,
		"api_secret": "as", "token": "tk", "pin": "1234", "admin_key": "ak",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.URL != "ws://localhost:8188" || c.Room != 1011 || !c.InsecureTLS ||
		c.APISecret != "as" || c.Token != "tk" || c.PIN != "1234" || c.AdminKey != "ak" {
		t.Errorf("config = %+v", c)
	}
}

func TestParseConfig_RoomForms(t *testing.T) {
	for name, room := range map[string]any{"float": float64(7), "int": 7, "int64": int64(7), "string": "7"} {
		c, err := ParseConfig(map[string]any{"url": "wss://janus.example.org/ws", "room": room})
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if c.Room != 7 || c.URL != "wss://janus.example.org/ws" {
			t.Errorf("%s: config = %+v", name, c)
		}
	}
}

func TestParseConfig_Errors(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg  map[string]any
		want string
	}{
		"no url":          {map[string]any{"room": 1}, "platform_config.url is required"},
		"bad scheme":      {map[string]any{"url": "ftp://x", "room": 1}, `platform_config.url must be a ws://, wss://, http:// or https:// URL without query, got "ftp://x"`},
		"query":           {map[string]any{"url": "ws://x/?a=b", "room": 1}, "without query"},
		"no room":         {map[string]any{"url": "ws://x"}, "platform_config.room is required"},
		"room zero":       {map[string]any{"url": "ws://x", "room": float64(0)}, "platform_config.room must be a positive integer, got 0"},
		"room negative":   {map[string]any{"url": "ws://x", "room": -3}, "platform_config.room must be a positive integer, got -3"},
		"room fraction":   {map[string]any{"url": "ws://x", "room": 1.5}, "platform_config.room must be a positive integer, got 1.5"},
		"room text":       {map[string]any{"url": "ws://x", "room": "abc"}, "platform_config.room must be a positive integer, got abc"},
		"room bool":       {map[string]any{"url": "ws://x", "room": true}, "platform_config.room must be a positive integer, got true"},
		"unknown key":     {map[string]any{"url": "ws://x", "room": 1, "secret": "s"}, `platform_config: unknown key "secret" for platform janus`},
		"tls not bool":    {map[string]any{"url": "ws://x", "room": 1, "insecure_tls": "yes"}, "platform_config.insecure_tls must be a boolean"},
		"secret not str":  {map[string]any{"url": "ws://x", "room": 1, "api_secret": 3}, "platform_config.api_secret must be a string"},
		"pin not str":     {map[string]any{"url": "ws://x", "room": 1, "pin": 1234}, "platform_config.pin must be a string"},
		"url not str":     {map[string]any{"url": 3, "room": 1}, "platform_config.url must be a string"},
		"admin key empty": {map[string]any{"url": "ws://x", "room": 1, "admin_key": false}, "platform_config.admin_key must be a string"},
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
	cfg := map[string]any{"url": "ws://localhost:8188", "room": 1011}
	if err := Validate(scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Profile: "audio_video", MaxReceivedStreams: 10}}); err != nil {
		t.Errorf("valid plan: %v", err)
	}
	for name, tc := range map[string]struct {
		tp   scenario.TestPlan
		want string
	}{
		"simulcast": {scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{}}},
			"media.simulcast is not supported on platform janus"},
		"requested heights": {scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{RequestedHeights: []scenario.RequestedHeight{{Height: 360}}}},
			"media.requested_heights is not supported on platform janus"},
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
	c, err := ParseConfig(map[string]any{"url": "ws://x", "room": 1,
		"api_secret": "apisecret-value", "token": "token-value", "pin": "pin-value", "admin_key": "adminkey-value"})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewTextHandler(&buf, nil)).Info("cfg", slog.Any("config", c), slog.String("s", c.String()))
	for _, v := range []string{"apisecret-value", "token-value", "pin-value", "adminkey-value"} {
		if strings.Contains(buf.String(), v) {
			t.Errorf("secret %q logged: %s", v, buf.String())
		}
	}
	for _, k := range []string{"has_api_secret=true", "has_token=true", "has_pin=true", "has_admin_key=true"} {
		if !strings.Contains(buf.String(), k) {
			t.Errorf("log lacks %s: %s", k, buf.String())
		}
	}
}
