// Package mediasoup is the platform adapter for the mediasoup-demo server
// (specs/010-mediasoup-platform): each bot joins a room over the demo web
// client's protoo signaling (JSON over WebSocket), produces its tracks on a
// session it offers and consumes the others' on a session whose offer the
// adapter builds from the server's transport and consumer parameters.
package mediasoup

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"

	"webrtc-load-engine/src/scenario"
)

// Config is the platform_config of a mediasoup test plan
// (specs/010-mediasoup-platform/contracts/mediasoup-config.yaml).
type Config struct {
	URL                 string // ws:// or wss:// base URL, without query
	Room                string
	Origin              string // Origin header of the handshake
	InsecureTLS         bool
	UnlimitedRoomSecret string // Never logged
	UsePipeTransports   bool
}

var configKeys = []string{"url", "room", "origin", "insecure_tls", "unlimited_room_secret", "use_pipe_transports"}

// String shows the config without its secret.
func (c Config) String() string {
	return fmt.Sprintf("{URL:%s Room:%s Origin:%s InsecureTLS:%t UsePipeTransports:%t unlimited_room_secret:%t}",
		c.URL, c.Room, c.Origin, c.InsecureTLS, c.UsePipeTransports, c.UnlimitedRoomSecret != "")
}

// LogValue logs the config without its secret.
func (c Config) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("url", c.URL), slog.String("room", c.Room), slog.String("origin", c.Origin),
		slog.Bool("insecure_tls", c.InsecureTLS), slog.Bool("use_pipe_transports", c.UsePipeTransports),
		slog.Bool("has_unlimited_room_secret", c.UnlimitedRoomSecret != ""),
	)
}

// ParseConfig reads and checks a mediasoup platform_config.
func ParseConfig(m map[string]any) (Config, error) {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		if !slices.Contains(configKeys, k) {
			return Config{}, fmt.Errorf("platform_config: unknown key %q for platform mediasoup", k)
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
	boolean := func(key string) (bool, error) {
		v, ok := m[key]
		if !ok || v == nil {
			return false, nil
		}
		b, ok := v.(bool)
		if !ok {
			return false, fmt.Errorf("platform_config.%s must be a boolean", key)
		}
		return b, nil
	}

	var c Config
	var raw string
	var err error
	for _, f := range []struct {
		key string
		dst *string
	}{{"url", &raw}, {"room", &c.Room}, {"origin", &c.Origin}, {"unlimited_room_secret", &c.UnlimitedRoomSecret}} {
		if *f.dst, err = str(f.key); err != nil {
			return Config{}, err
		}
	}
	if c.InsecureTLS, err = boolean("insecure_tls"); err != nil {
		return Config{}, err
	}
	if c.UsePipeTransports, err = boolean("use_pipe_transports"); err != nil {
		return Config{}, err
	}

	if raw == "" {
		return Config{}, errors.New("platform_config.url is required")
	}
	u, err := wsURL(raw)
	if err != nil {
		return Config{}, err
	}
	c.URL = u.String()
	if c.Room == "" {
		return Config{}, errors.New("platform_config.room is required")
	}
	if c.Origin == "" {
		scheme := "http"
		if u.Scheme == "wss" {
			scheme = "https"
		}
		c.Origin = scheme + "://" + u.Host
	} else if o, err := url.Parse(c.Origin); err != nil || o.Host == "" || (o.Scheme != "http" && o.Scheme != "https") {
		return Config{}, fmt.Errorf("platform_config.origin must be an http:// or https:// URL, got %q", c.Origin)
	}
	return c, nil
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

// Validate checks a mediasoup test plan when its scenario is created: its
// platform_config, and the media options this version does not support on
// mediasoup (FR-008).
func Validate(tp scenario.TestPlan) error {
	if _, err := ParseConfig(tp.PlatformConfig); err != nil {
		return err
	}
	if m := tp.Media; m != nil {
		if m.Simulcast != nil {
			return errors.New("media.simulcast is not supported on platform mediasoup")
		}
		if len(m.RequestedHeights) > 0 {
			return errors.New("media.requested_heights is not supported on platform mediasoup")
		}
	}
	return nil
}
