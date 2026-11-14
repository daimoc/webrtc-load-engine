package scenario

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

// The browser and server_settings sections of a test plan
// (specs/015-bandwidth-estimate-parity/contracts/test-plan.md). Both are
// platform-independent: no adapter reads them.

// DefaultBrowserWindow is the browser's window size when none is given.
const DefaultBrowserWindow = "1280x720"

// Limits of the browser section.
const (
	maxBrowserExpectedHeight = 2160
	minBrowserWidth          = 320
	minBrowserHeight         = 240
	maxBrowserWidth          = 3840
	maxBrowserHeight         = 2160
)

// BrowserConfig asks for one headless browser beside the bots: the engine
// starts it, opens URL, reads what the browser measures and stops it at the
// end of the run. The page at URL must join the room by itself: the engine
// performs no step in it.
type BrowserConfig struct {
	URL string `json:"url"`
	// ExpectedHeight is the video height the browser's share of time is
	// measured against; 0: not stated.
	ExpectedHeight int `json:"expected_height,omitempty"`
	// Window is "<width>x<height>"; DefaultBrowserWindow when empty.
	Window string `json:"window,omitempty"`
	// InsecureTLS accepts a certificate the browser does not trust.
	InsecureTLS bool `json:"insecure_tls,omitempty"`
	// Executable is the browser's path; found in PATH when empty. It is not
	// looked for at creation: a missing browser is a reason in the report.
	Executable string `json:"executable,omitempty"`
}

// UnmarshalJSON refuses unknown fields, so that a misspelt setting is not
// silently ignored.
func (b *BrowserConfig) UnmarshalJSON(data []byte) error {
	type plain BrowserConfig
	var p plain
	if err := strictUnmarshal(data, &p); err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	*b = BrowserConfig(p)
	return nil
}

// WindowSize returns the window's width and height; 0, 0 when the window
// is not valid.
func (b *BrowserConfig) WindowSize() (width, height int) {
	window := b.Window
	if window == "" {
		window = DefaultBrowserWindow
	}
	w, h, ok := strings.Cut(window, "x")
	if !ok {
		return 0, 0
	}
	width, err1 := strconv.Atoi(w)
	height, err2 := strconv.Atoi(h)
	if err1 != nil || err2 != nil {
		return 0, 0
	}
	return width, height
}

// withDefaults validates b and returns a copy with defaults applied; nil
// stays nil.
func (b *BrowserConfig) withDefaults() (*BrowserConfig, error) {
	if b == nil {
		return nil, nil
	}
	c := *b
	u, err := url.Parse(c.URL)
	switch {
	case c.URL == "":
		return nil, fmt.Errorf("browser.url is required")
	case err != nil:
		return nil, fmt.Errorf("browser.url: %w", err)
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, fmt.Errorf("browser.url must be an http or https address, got %q", c.URL)
	case u.Host == "":
		return nil, fmt.Errorf("browser.url has no host: %q", c.URL)
	}
	if c.ExpectedHeight < 0 || c.ExpectedHeight > maxBrowserExpectedHeight {
		return nil, fmt.Errorf("browser.expected_height must be between 1 and %d, got %d", maxBrowserExpectedHeight, c.ExpectedHeight)
	}
	if c.Window == "" {
		c.Window = DefaultBrowserWindow
	}
	w, h := c.WindowSize()
	if w < minBrowserWidth || h < minBrowserHeight || w > maxBrowserWidth || h > maxBrowserHeight {
		return nil, fmt.Errorf("browser.window must be <width>x<height> from %dx%d to %dx%d, got %q",
			minBrowserWidth, minBrowserHeight, maxBrowserWidth, maxBrowserHeight, c.Window)
	}
	return &c, nil
}

// Sources of server settings.
const (
	ServerSettingsOperator   = "operator"
	ServerSettingsDeployment = "deployment"
)

const maxServerSettings = 16

// ServerSettingsConfig states the settings of the server under test that
// differ from its defaults and change results. The engine does not check
// them: the report gives them as stated.
type ServerSettingsConfig struct {
	// Source is who stated them: ServerSettingsOperator (default) or
	// ServerSettingsDeployment.
	Source string `json:"source,omitempty"`
	// Values maps a setting's name, as the server names it, to its value.
	Values map[string]string `json:"values,omitempty"`
}

// UnmarshalJSON refuses unknown fields.
func (s *ServerSettingsConfig) UnmarshalJSON(data []byte) error {
	type plain ServerSettingsConfig
	var p plain
	if err := strictUnmarshal(data, &p); err != nil {
		return fmt.Errorf("server_settings: %w", err)
	}
	*s = ServerSettingsConfig(p)
	return nil
}

// withDefaults validates s and returns a copy with defaults applied; nil
// stays nil.
func (s *ServerSettingsConfig) withDefaults() (*ServerSettingsConfig, error) {
	if s == nil {
		return nil, nil
	}
	c := ServerSettingsConfig{Source: s.Source, Values: maps.Clone(s.Values)}
	switch c.Source {
	case "":
		c.Source = ServerSettingsOperator
	case ServerSettingsOperator, ServerSettingsDeployment:
	default:
		return nil, fmt.Errorf("server_settings.source must be %s or %s, got %q", ServerSettingsOperator, ServerSettingsDeployment, c.Source)
	}
	if len(c.Values) > maxServerSettings {
		return nil, fmt.Errorf("server_settings.values: at most %d settings, got %d", maxServerSettings, len(c.Values))
	}
	for name := range c.Values {
		if name == "" {
			return nil, fmt.Errorf("server_settings.values: empty setting name")
		}
	}
	if c.Source == ServerSettingsOperator && len(c.Values) == 0 {
		return nil, fmt.Errorf("server_settings from the operator states no value: give values, or leave the section out")
	}
	return &c, nil
}

func strictUnmarshal(data []byte, v any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(v)
}

// secretURLParams are the browser address's query and fragment parameters
// whose values are hidden in what the service returns and logs.
var secretURLParams = []string{"jwt", "token", "access_token"}

// RedactBrowserURL hides the values of secretURLParams in the query and in
// the fragment of address, keeping the rest of it as written.
func RedactBrowserURL(address string) string {
	base, fragment, hasFragment := strings.Cut(address, "#")
	path, query, hasQuery := strings.Cut(base, "?")
	out := path
	if hasQuery {
		out += "?" + redactParams(query)
	}
	if hasFragment {
		out += "#" + redactParams(fragment)
	}
	return out
}

func redactParams(params string) string {
	parts := strings.Split(params, "&")
	for i, p := range parts {
		name, value, ok := strings.Cut(p, "=")
		if !ok || value == "" {
			continue
		}
		for _, secret := range secretURLParams {
			if name == secret {
				parts[i] = name + "=" + Redacted
			}
		}
	}
	return strings.Join(parts, "&")
}

// ParseServerSettings reads "name=value" pairs separated by commas, as the
// settings file of sfu-remote and --server-settings write them; an empty
// string gives an empty map (the deployment changed nothing).
func ParseServerSettings(s string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(s) == "" {
		return out, nil
	}
	for _, pair := range strings.Split(s, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("server settings: %q is not name=value", pair)
		}
		out[name] = value
	}
	return out, nil
}

// FormatServerSettings writes settings as ParseServerSettings reads them,
// sorted by name.
func FormatServerSettings(values map[string]string) string {
	var pairs []string
	for _, k := range slices.Sorted(maps.Keys(values)) {
		pairs = append(pairs, k+"="+values[k])
	}
	return strings.Join(pairs, ",")
}
