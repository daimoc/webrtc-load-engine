package catalog

import (
	"slices"
	"testing"

	"webrtc-load-engine/src/platform/jitsi"
	"webrtc-load-engine/src/platform/livekit"
	"webrtc-load-engine/src/scenario"
)

func TestCatalog_Jitsi(t *testing.T) {
	c := New(nil)
	if !slices.Contains(c.Platforms(), "jitsi") {
		t.Fatalf("Platforms() = %v, want jitsi", c.Platforms())
	}
	// Jitsi configs are checked when the run starts, not at creation.
	if err := c.ValidateTestPlan("jitsi", scenario.TestPlan{}); err != nil {
		t.Errorf("ValidateTestPlan(jitsi, no config) = %v, want nil", err)
	}
	if got := c.SecretKeys("jitsi"); !slices.Equal(got, []string{"jwt_app_secret", "jwt"}) {
		t.Errorf("SecretKeys(jitsi) = %v", got)
	}
	if _, err := c.NewAdapter("jitsi", scenario.TestPlan{}); err == nil {
		t.Error("NewAdapter(jitsi) without xmpp_domain returned no error")
	}
	a, err := c.NewAdapter("jitsi", scenario.TestPlan{PlatformConfig: map[string]any{"xmpp_domain": "meet.example.org", "room_name": "r"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.(*jitsi.Adapter); !ok {
		t.Errorf("NewAdapter(jitsi) = %T, want *jitsi.Adapter", a)
	}
}

func TestCatalog_UnknownPlatform(t *testing.T) {
	c := New(nil)
	if err := c.ValidateTestPlan("nope", scenario.TestPlan{}); err == nil || err.Error() != "unsupported platform adapter: nope" {
		t.Errorf("ValidateTestPlan(nope) = %v", err)
	}
	if _, err := c.NewAdapter("nope", scenario.TestPlan{}); err == nil {
		t.Error("NewAdapter(nope) returned no error")
	}
	if c.SecretKeys("nope") != nil {
		t.Error("SecretKeys(nope) is not nil")
	}
}

func TestCatalog_LiveKit(t *testing.T) {
	c := New(nil)
	if !slices.Equal(c.Platforms(), []string{"jitsi", "livekit"}) {
		t.Fatalf("Platforms() = %v", c.Platforms())
	}
	cfg := map[string]any{"url": "ws://localhost:7880", "room": "r", "api_key": "k", "api_secret": "s"}
	if err := c.ValidateTestPlan("livekit", scenario.TestPlan{PlatformConfig: cfg}); err != nil {
		t.Errorf("valid livekit plan: %v", err)
	}
	if err := c.ValidateTestPlan("livekit", scenario.TestPlan{}); err == nil {
		t.Error("livekit plan without config accepted")
	}
	if got := c.SecretKeys("livekit"); !slices.Equal(got, []string{"api_secret", "token"}) {
		t.Errorf("SecretKeys(livekit) = %v", got)
	}
	a, err := c.NewAdapter("livekit", scenario.TestPlan{PlatformConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.(*livekit.Adapter); !ok {
		t.Errorf("NewAdapter(livekit) = %T", a)
	}
}
