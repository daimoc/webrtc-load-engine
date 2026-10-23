package catalog

import (
	"slices"
	"testing"

	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/platform/janus"
	"webrtc-load-engine/src/platform/jitsi"
	"webrtc-load-engine/src/platform/livekit"
	"webrtc-load-engine/src/platform/mediasoup"
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
	if !slices.Contains(c.Platforms(), "livekit") {
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

func TestCatalog_Mediasoup(t *testing.T) {
	c := New(nil)
	if !slices.Contains(c.Platforms(), "mediasoup") {
		t.Fatalf("Platforms() = %v", c.Platforms())
	}
	cfg := map[string]any{"url": "ws://localhost:4443", "room": "load010", "unlimited_room_secret": "s"}
	if err := c.ValidateTestPlan("mediasoup", scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, MaxReceivedStreams: 10}}); err != nil {
		t.Errorf("valid mediasoup plan: %v", err)
	}
	for _, tc := range []struct {
		tp   scenario.TestPlan
		want string
	}{
		{scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{}}}, "media.simulcast is not supported on platform mediasoup"},
		{scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{RequestedHeights: []scenario.RequestedHeight{{Height: 180, Share: 1}}}}, "media.requested_heights is not supported on platform mediasoup"},
		{scenario.TestPlan{PlatformConfig: map[string]any{"url": "ws://localhost:4443"}}, "platform_config.room is required"},
	} {
		if err := c.ValidateTestPlan("mediasoup", tc.tp); err == nil || err.Error() != tc.want {
			t.Errorf("ValidateTestPlan = %v, want %q", err, tc.want)
		}
	}
	if got := c.SecretKeys("mediasoup"); !slices.Equal(got, []string{"unlimited_room_secret"}) {
		t.Errorf("SecretKeys(mediasoup) = %v", got)
	}
	a, err := c.NewAdapter("mediasoup", scenario.TestPlan{PlatformConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.(*mediasoup.Adapter); !ok {
		t.Errorf("NewAdapter(mediasoup) = %T", a)
	}
	if ps, ok := a.(platform.PublishSessionAdapter); !ok || !ps.PublishSession() {
		t.Error("the mediasoup adapter does not use a publish session")
	}
}

func TestCatalog_Janus(t *testing.T) {
	c := New(nil)
	if !slices.Equal(c.Platforms(), []string{"jitsi", "livekit", "mediasoup", "janus"}) {
		t.Fatalf("Platforms() = %v", c.Platforms())
	}
	cfg := map[string]any{"url": "ws://localhost:8188", "room": float64(1011), "api_secret": "s", "pin": "p"}
	if err := c.ValidateTestPlan("janus", scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, MaxReceivedStreams: 10}}); err != nil {
		t.Errorf("valid janus plan: %v", err)
	}
	for _, tc := range []struct {
		tp   scenario.TestPlan
		want string
	}{
		{scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{Simulcast: &scenario.SimulcastConfig{}}}, "media.simulcast is not supported on platform janus"},
		{scenario.TestPlan{PlatformConfig: cfg, Media: &scenario.MediaConfig{RequestedHeights: []scenario.RequestedHeight{{Height: 180, Share: 1}}}}, "media.requested_heights is not supported on platform janus"},
		{scenario.TestPlan{PlatformConfig: map[string]any{"url": "ws://localhost:8188"}}, "platform_config.room is required"},
		{scenario.TestPlan{PlatformConfig: map[string]any{"url": "ws://localhost:8188", "room": "abc"}}, "platform_config.room must be a positive integer, got abc"},
	} {
		if err := c.ValidateTestPlan("janus", tc.tp); err == nil || err.Error() != tc.want {
			t.Errorf("ValidateTestPlan = %v, want %q", err, tc.want)
		}
	}
	if got := c.SecretKeys("janus"); !slices.Equal(got, []string{"api_secret", "token", "pin", "admin_key"}) {
		t.Errorf("SecretKeys(janus) = %v", got)
	}
	a, err := c.NewAdapter("janus", scenario.TestPlan{Participants: 50, PlatformConfig: cfg})
	if err != nil {
		t.Fatal(err)
	}
	ja, ok := a.(*janus.Adapter)
	if !ok {
		t.Fatalf("NewAdapter(janus) = %T", a)
	}
	if got := ja.Config().Participants; got != 50 {
		t.Errorf("adapter sized for %d participants, want the plan's 50", got)
	}
	if ps, ok := a.(platform.PublishSessionAdapter); !ok || !ps.PublishSession() {
		t.Error("the janus adapter does not use a publish session")
	}
	if _, err := c.NewAdapter("janus", scenario.TestPlan{PlatformConfig: map[string]any{"url": "ws://x"}}); err == nil {
		t.Error("NewAdapter(janus) without room returned no error")
	}
}
