// Package livekit is the platform adapter for LiveKit
// (specs/008-livekit-platform): each bot joins a room over LiveKit's client
// signaling protocol (protobuf over WebSocket), publishes its tracks on a
// session it offers and receives on the session the server offers.
package livekit

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"time"

	"webrtc-load-engine/src/scenario"
)

// defaultTokenTTL is the lifetime of the tokens the adapter signs: longer
// than any run (there is no reconnect that would need a fresh one).
const defaultTokenTTL = 6 * time.Hour

// Config is the platform_config of a LiveKit test plan
// (specs/008-livekit-platform/contracts/livekit-config.yaml).
type Config struct {
	URL       string // WebSocket URL of the server's /rtc endpoint
	Room      string
	APIKey    string
	APISecret string // Never logged
	Token     string // Ready-made token for every bot; never logged
	TokenTTL  time.Duration
}

var configKeys = []string{"url", "room", "api_key", "api_secret", "token", "token_ttl"}

// String shows the config without its secrets.
func (c Config) String() string {
	return fmt.Sprintf("{URL:%s Room:%s APIKey:%s TokenTTL:%s secret:%t token:%t}", c.URL, c.Room, c.APIKey, c.TokenTTL, c.APISecret != "", c.Token != "")
}

// LogValue logs the config without its secrets.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("url", c.URL), slog.String("room", c.Room), slog.String("api_key", c.APIKey),
		slog.Bool("has_api_secret", c.APISecret != ""), slog.Bool("has_token", c.Token != ""),
	)
}

// ParseConfig reads and checks a LiveKit platform_config.
func ParseConfig(m map[string]any) (Config, error) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(configKeys, k) {
			return Config{}, fmt.Errorf("platform_config: unknown key %q for platform livekit", k)
		}
	}
	get := func(key string) (string, error) {
		v, ok := m[key]
		if !ok || v == nil {
			return "", nil
		}
		s, ok := v.(string)
		if !ok {
			return "", fmt.Errorf("platform_config.%s must be a string", key)
		}
		return s, nil
	}
	var c Config
	var raw, ttl string
	for _, f := range []struct {
		key string
		dst *string
	}{{"url", &raw}, {"room", &c.Room}, {"api_key", &c.APIKey}, {"api_secret", &c.APISecret}, {"token", &c.Token}, {"token_ttl", &ttl}} {
		v, err := get(f.key)
		if err != nil {
			return Config{}, err
		}
		*f.dst = v
	}

	if raw == "" {
		return Config{}, errors.New("platform_config.url is required")
	}
	u, err := rtcURL(raw)
	if err != nil {
		return Config{}, err
	}
	c.URL = u
	if c.Room == "" {
		return Config{}, errors.New("platform_config.room is required")
	}
	keyed := c.APIKey != "" || c.APISecret != ""
	switch {
	case c.Token != "" && keyed:
		return Config{}, errors.New("platform_config.token cannot be combined with api_key/api_secret")
	case c.Token == "" && !keyed:
		return Config{}, errors.New("platform_config needs api_key and api_secret, or token")
	case keyed && (c.APIKey == "" || c.APISecret == ""):
		return Config{}, errors.New("platform_config.api_key and platform_config.api_secret must be set together")
	}
	if c.Token != "" {
		if ttl != "" {
			return Config{}, errors.New("platform_config.token_ttl applies only with api_key/api_secret")
		}
		return c, nil
	}
	c.TokenTTL = defaultTokenTTL
	if ttl != "" {
		d, err := time.ParseDuration(ttl)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("platform_config.token_ttl must be a positive duration, got %q", ttl)
		}
		c.TokenTTL = d
	}
	return c, nil
}

// rtcURL turns the server URL into its /rtc WebSocket URL: http(s) becomes
// ws(s), and /rtc is appended unless the path already ends with it.
func rtcURL(raw string) (string, error) {
	bad := fmt.Errorf("platform_config.url must be a ws://, wss://, http:// or https:// URL, got %q", raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", bad
	}
	switch u.Scheme {
	case "ws", "wss":
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", bad
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(u.Path, "/rtc") {
		u.Path += "/rtc"
	}
	u.RawQuery, u.Fragment = "", ""
	return u.String(), nil
}

// Validate checks a LiveKit test plan when its scenario is created: its
// platform_config, and the media options this version does not support on
// LiveKit (FR-008).
func Validate(tp scenario.TestPlan) error {
	if _, err := ParseConfig(tp.PlatformConfig); err != nil {
		return err
	}
	if m := tp.Media; m != nil {
		if m.Simulcast != nil {
			return errors.New("media.simulcast is not supported on platform livekit")
		}
		if len(m.RequestedHeights) > 0 {
			return errors.New("media.requested_heights is not supported on platform livekit")
		}
	}
	return nil
}
