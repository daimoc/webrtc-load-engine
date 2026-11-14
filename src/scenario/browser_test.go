package scenario

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// Browser and server settings sections of a test plan
// (specs/015-bandwidth-estimate-parity/contracts/test-plan.md).

func browserPlan(b *BrowserConfig) TestPlan {
	tp := validPlan(nil)
	tp.Browser = b
	return tp
}

func TestNewScenario_Browser(t *testing.T) {
	cases := map[string]struct {
		browser BrowserConfig
		wantErr string // Empty: accepted
	}{
		"url only":           {BrowserConfig{URL: "https://localhost:8443/room"}, ""},
		"http":               {BrowserConfig{URL: "http://localhost:5555/?roomId=r"}, ""},
		"no url":             {BrowserConfig{}, "browser.url"},
		"ftp":                {BrowserConfig{URL: "ftp://host/room"}, "browser.url"},
		"no host":            {BrowserConfig{URL: "https:///room"}, "browser.url"},
		"not a url":          {BrowserConfig{URL: "::"}, "browser.url"},
		"height 1":           {BrowserConfig{URL: "https://h/r", ExpectedHeight: 1}, ""},
		"height 2160":        {BrowserConfig{URL: "https://h/r", ExpectedHeight: 2160}, ""},
		"height -1":          {BrowserConfig{URL: "https://h/r", ExpectedHeight: -1}, "browser.expected_height"},
		"height 2161":        {BrowserConfig{URL: "https://h/r", ExpectedHeight: 2161}, "browser.expected_height"},
		"window smallest":    {BrowserConfig{URL: "https://h/r", Window: "320x240"}, ""},
		"window largest":     {BrowserConfig{URL: "https://h/r", Window: "3840x2160"}, ""},
		"window too narrow":  {BrowserConfig{URL: "https://h/r", Window: "319x240"}, "browser.window"},
		"window too wide":    {BrowserConfig{URL: "https://h/r", Window: "3841x2160"}, "browser.window"},
		"window one number":  {BrowserConfig{URL: "https://h/r", Window: "1280"}, "browser.window"},
		"window no height":   {BrowserConfig{URL: "https://h/r", Window: "1280x"}, "browser.window"},
		"window letters":     {BrowserConfig{URL: "https://h/r", Window: "ax720"}, "browser.window"},
		"missing executable": {BrowserConfig{URL: "https://h/r", Executable: "/no/such/chrome"}, ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			b := tc.browser
			sc, err := NewScenario("s", "jitsi", browserPlan(&b))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("NewScenario: %v", err)
				}
				if sc.TestPlan.Browser == nil || sc.TestPlan.Browser.Window == "" {
					t.Fatalf("browser not kept with a window: %+v", sc.TestPlan.Browser)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want an error naming %s, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestNewScenario_BrowserDefaults(t *testing.T) {
	sc, err := NewScenario("s", "jitsi", browserPlan(&BrowserConfig{URL: "https://h/r"}))
	if err != nil {
		t.Fatal(err)
	}
	b := sc.TestPlan.Browser
	if b.Window != DefaultBrowserWindow {
		t.Errorf("window: want %s, got %q", DefaultBrowserWindow, b.Window)
	}
	if w, h := b.WindowSize(); w != 1280 || h != 720 {
		t.Errorf("WindowSize: want 1280x720, got %dx%d", w, h)
	}
	if w, h := (&BrowserConfig{Window: "640x480"}).WindowSize(); w != 640 || h != 480 {
		t.Errorf("WindowSize 640x480: got %dx%d", w, h)
	}
}

func TestNewScenario_BrowserNotShared(t *testing.T) {
	in := &BrowserConfig{URL: "https://h/r"}
	sc, err := NewScenario("s", "jitsi", browserPlan(in))
	if err != nil {
		t.Fatal(err)
	}
	if sc.TestPlan.Browser == in {
		t.Fatal("the scenario shares the caller's browser section")
	}
}

func TestBrowserConfig_UnknownFields(t *testing.T) {
	var tp TestPlan
	err := json.Unmarshal([]byte(`{"participants":1,"duration_seconds":1,"browser":{"url":"https://h/r","headless":false}}`), &tp)
	if err == nil || !strings.Contains(err.Error(), "browser") {
		t.Fatalf("want an error naming browser for an unknown field, got %v", err)
	}
	if err := json.Unmarshal([]byte(`{"browser":{"url":"https://h/r","expected_height":720,"window":"800x600","insecure_tls":true,"executable":"/x"}}`), &tp); err != nil {
		t.Fatalf("known fields: %v", err)
	}
	want := BrowserConfig{URL: "https://h/r", ExpectedHeight: 720, Window: "800x600", InsecureTLS: true, Executable: "/x"}
	if !reflect.DeepEqual(*tp.Browser, want) {
		t.Fatalf("want %+v, got %+v", want, *tp.Browser)
	}
}

func TestTestPlan_WithoutBrowserRoundTrips(t *testing.T) {
	in := `{"participants":2,"ramp_up_seconds":0,"duration_seconds":5}`
	var tp TestPlan
	if err := json.Unmarshal([]byte(in), &tp); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(tp)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != in {
		t.Fatalf("round trip changed the plan:\n in %s\nout %s", in, out)
	}
	sc, err := NewScenario("s", "jitsi", tp)
	if err != nil {
		t.Fatal(err)
	}
	if sc.TestPlan.Browser != nil || sc.TestPlan.ServerSettings != nil {
		t.Fatalf("sections appeared: %+v %+v", sc.TestPlan.Browser, sc.TestPlan.ServerSettings)
	}
}

func settingsPlan(s *ServerSettingsConfig) TestPlan {
	tp := validPlan(nil)
	tp.ServerSettings = s
	return tp
}

func TestNewScenario_ServerSettings(t *testing.T) {
	many := map[string]string{}
	for i := 0; i < 17; i++ {
		many[strings.Repeat("K", i+1)] = "v"
	}
	cases := map[string]struct {
		settings   ServerSettingsConfig
		wantErr    string
		wantSource string
	}{
		"operator default":     {ServerSettingsConfig{Values: map[string]string{"JVB_CC_TRUST_BWE": "false"}}, "", ServerSettingsOperator},
		"deployment none":      {ServerSettingsConfig{Source: ServerSettingsDeployment}, "", ServerSettingsDeployment},
		"deployment values":    {ServerSettingsConfig{Source: ServerSettingsDeployment, Values: map[string]string{"A": "1"}}, "", ServerSettingsDeployment},
		"other source":         {ServerSettingsConfig{Source: "guess", Values: map[string]string{"A": "1"}}, "server_settings.source", ""},
		"17 values":            {ServerSettingsConfig{Values: many}, "server_settings.values", ""},
		"empty name":           {ServerSettingsConfig{Values: map[string]string{"": "1"}}, "server_settings.values", ""},
		"operator states none": {ServerSettingsConfig{Source: ServerSettingsOperator}, "server_settings", ""},
		"default states none":  {ServerSettingsConfig{}, "server_settings", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := tc.settings
			sc, err := NewScenario("s", "jitsi", settingsPlan(&s))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want an error naming %s, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewScenario: %v", err)
			}
			if got := sc.TestPlan.ServerSettings.Source; got != tc.wantSource {
				t.Errorf("source: want %s, got %s", tc.wantSource, got)
			}
		})
	}
}

func TestNewScenario_ServerSettingsCopied(t *testing.T) {
	values := map[string]string{"A": "1"}
	sc, err := NewScenario("s", "jitsi", settingsPlan(&ServerSettingsConfig{Values: values}))
	if err != nil {
		t.Fatal(err)
	}
	values["A"] = "2"
	if got := sc.TestPlan.ServerSettings.Values["A"]; got != "1" {
		t.Fatalf("the scenario shares the caller's values: got %s", got)
	}
}

func TestServerSettings_UnknownFields(t *testing.T) {
	var tp TestPlan
	err := json.Unmarshal([]byte(`{"server_settings":{"source":"operator","value":{"A":"1"}}}`), &tp)
	if err == nil || !strings.Contains(err.Error(), "server_settings") {
		t.Fatalf("want an error naming server_settings, got %v", err)
	}
}

func TestParseServerSettings(t *testing.T) {
	got, err := ParseServerSettings("JVB_CC_TRUST_BWE=false, B=x=y")
	if err != nil || !reflect.DeepEqual(got, map[string]string{"JVB_CC_TRUST_BWE": "false", "B": "x=y"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if got, err := ParseServerSettings(""); err != nil || len(got) != 0 || got == nil {
		t.Fatalf("empty: got %v, %v", got, err)
	}
	for _, bad := range []string{"A", "=1", "A=1,,B=2"} {
		if _, err := ParseServerSettings(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if got := FormatServerSettings(map[string]string{"B": "2", "A": "1"}); got != "A=1,B=2" {
		t.Errorf("format: %q", got)
	}
}
