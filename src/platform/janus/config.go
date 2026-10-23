// Package janus is the platform adapter for the Janus WebRTC Server's
// VideoRoom plugin (specs/011-janus-platform): each bot holds one Janus
// session over the WebSocket transport, publishes its tracks on a session it
// offers and receives the others' through one multistream subscription
// whose offers Janus makes.
package janus

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"webrtc-load-engine/src/scenario"
)

// Config is the platform_config of a Janus test plan
// (specs/011-janus-platform/contracts/janus-config.yaml).
type Config struct {
	URL         string // ws:// or wss:// URL, without query
	Room        uint64 // Numeric VideoRoom ID
	InsecureTLS bool
	APISecret   string // Never logged
	Token       string // Never logged
	PIN         string // Never logged
	AdminKey    string // Never logged
	// Participants is the test plan's bot count, not a platform_config key:
	// the room is created with that many publisher places (research R4).
	Participants int
}

var configKeys = []string{"url", "room", "insecure_tls", "api_secret", "token", "pin", "admin_key"}

// String shows the config without its secrets.
func (c Config) String() string {
	return fmt.Sprintf("{URL:%s Room:%d InsecureTLS:%t Participants:%d api_secret:%t token:%t pin:%t admin_key:%t}",
		c.URL, c.Room, c.InsecureTLS, c.Participants, c.APISecret != "", c.Token != "", c.PIN != "", c.AdminKey != "")
}

// LogValue logs the config without its secrets.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("url", c.URL), slog.Uint64("room", c.Room), slog.Bool("insecure_tls", c.InsecureTLS),
		slog.Int("participants", c.Participants),
		slog.Bool("has_api_secret", c.APISecret != ""), slog.Bool("has_token", c.Token != ""),
		slog.Bool("has_pin", c.PIN != ""), slog.Bool("has_admin_key", c.AdminKey != ""),
	)
}

// ParseConfig reads and checks a Janus platform_config.
func ParseConfig(m map[string]any) (Config, error) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(configKeys, k) {
			return Config{}, fmt.Errorf("platform_config: unknown key %q for platform janus", k)
		}
	}
	str := func(key string) (string, error) {
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
	var raw string
	var err error
	for _, f := range []struct {
		key string
		dst *string
	}{{"url", &raw}, {"api_secret", &c.APISecret}, {"token", &c.Token}, {"pin", &c.PIN}, {"admin_key", &c.AdminKey}} {
		if *f.dst, err = str(f.key); err != nil {
			return Config{}, err
		}
	}
	if v, ok := m["insecure_tls"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return Config{}, errors.New("platform_config.insecure_tls must be a boolean")
		}
		c.InsecureTLS = b
	}

	if raw == "" {
		return Config{}, errors.New("platform_config.url is required")
	}
	u, err := wsURL(raw)
	if err != nil {
		return Config{}, err
	}
	c.URL = u.String()
	v, ok := m["room"]
	if !ok || v == nil {
		return Config{}, errors.New("platform_config.room is required")
	}
	if c.Room, ok = roomID(v); !ok {
		return Config{}, fmt.Errorf("platform_config.room must be a positive integer, got %v", v)
	}
	return c, nil
}

// roomID reads a room ID from a JSON number (float64), a Go integer or a
// string of digits.
func roomID(v any) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n >= 1 && n == math.Trunc(n) && n < 1<<53 {
			return uint64(n), true
		}
	case int:
		if n > 0 {
			return uint64(n), true
		}
	case int64:
		if n > 0 {
			return uint64(n), true
		}
	case uint64:
		return n, n > 0
	case string:
		if id, err := strconv.ParseUint(n, 10, 64); err == nil && id > 0 {
			return id, true
		}
	}
	return 0, false
}

// wsURL checks the server URL and turns http(s) into ws(s); the path's
// trailing slash is dropped.
func wsURL(raw string) (*url.URL, error) {
	bad := fmt.Errorf("platform_config.url must be a ws://, wss://, http:// or https:// URL without query, got %q", raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, bad
	}
	switch u.Scheme {
	case "ws", "wss":
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, bad
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	return u, nil
}

// Validate checks a Janus test plan when its scenario is created: its
// platform_config, and the media options this version does not support on
// Janus (FR-010).
func Validate(tp scenario.TestPlan) error {
	if _, err := ParseConfig(tp.PlatformConfig); err != nil {
		return err
	}
	if m := tp.Media; m != nil {
		if m.Simulcast != nil {
			return errors.New("media.simulcast is not supported on platform janus")
		}
		if len(m.RequestedHeights) > 0 {
			return errors.New("media.requested_heights is not supported on platform janus")
		}
	}
	return nil
}
