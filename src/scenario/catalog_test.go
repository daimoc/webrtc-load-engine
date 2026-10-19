package scenario

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"webrtc-load-engine/src/platform"
)

// catalogStarter is an execution starter with a platform catalog (008
// research R10): two platforms, "alpha" validating its plan and "beta"
// with secrets.
type catalogStarter struct {
	started  chan TestPlan                 // Test plans handed to StartExecution
	adapters chan platform.PlatformAdapter // Adapters handed to StartExecution
}

type namedAdapter struct{ name string }

func (namedAdapter) Connect(context.Context, string, platform.SignalingHandler) error { return nil }
func (namedAdapter) Disconnect(string) error                                          { return nil }

func newCatalogStarter() *catalogStarter {
	return &catalogStarter{started: make(chan TestPlan, 1), adapters: make(chan platform.PlatformAdapter, 1)}
}

func (c *catalogStarter) Platforms() []string { return []string{"alpha", "beta"} }

func (c *catalogStarter) ValidateTestPlan(name string, tp TestPlan) error {
	if name == "alpha" && tp.PlatformConfig["room"] == nil {
		return errors.New("platform_config.room is required")
	}
	return nil
}

func (c *catalogStarter) SecretKeys(name string) []string {
	if name == "beta" {
		return []string{"api_secret", "token"}
	}
	return nil
}

func (c *catalogStarter) NewAdapter(name string, tp TestPlan) (platform.PlatformAdapter, error) {
	if tp.PlatformConfig["broken"] != nil {
		return nil, errors.New("broken config")
	}
	return namedAdapter{name: name}, nil
}

func (c *catalogStarter) StartExecution(id string, tp TestPlan, a platform.PlatformAdapter, onComplete func(string, RunResult)) (ExecutionEngine, error) {
	c.started <- tp
	c.adapters <- a
	return &completingEngine{id: id, onComplete: onComplete}, nil
}

func TestService_CatalogPlatforms(t *testing.T) {
	s := NewService(slog.New(&NoOpLogger{}), newCatalogStarter())
	plan := TestPlan{Participants: 1, DurationSeconds: 1}
	if _, err := s.CreateScenario("j", "jitsi", plan); err == nil || err.Error() != "unsupported platform adapter: jitsi" {
		t.Errorf("platform outside the catalog: err = %v", err)
	}
	if _, err := s.CreateScenario("a", "alpha", plan); err == nil || err.Error() != "platform_config.room is required" {
		t.Errorf("catalog validation error not returned: %v", err)
	}
	plan.PlatformConfig = map[string]any{"room": "r"}
	if _, err := s.CreateScenario("a", "alpha", plan); err != nil {
		t.Errorf("valid alpha plan rejected: %v", err)
	}
}

func TestService_WithoutCatalogOnlyJitsi(t *testing.T) {
	s := NewService(slog.New(&NoOpLogger{}), completingStarter{})
	if _, err := s.CreateScenario("j", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1}); err != nil {
		t.Errorf("jitsi without a catalog: %v", err)
	}
	if _, err := s.CreateScenario("l", "livekit", TestPlan{Participants: 1, DurationSeconds: 1}); err == nil {
		t.Error("livekit accepted without a catalog")
	}
}

func TestService_RedactsSecrets(t *testing.T) {
	var logs lockedBuffer // The completing run logs from its own goroutine
	starter := newCatalogStarter()
	s := NewService(slog.New(slog.NewJSONHandler(&logs, nil)), starter)
	plan := TestPlan{Participants: 1, DurationSeconds: 1, PlatformConfig: map[string]any{
		"url": "ws://x", "api_key": "k", "api_secret": "s3cret-value", "token": "t0ken-value",
	}}
	created, err := s.CreateScenario("b", "beta", plan)
	if err != nil {
		t.Fatal(err)
	}
	check := func(what string, sc *Scenario) {
		t.Helper()
		pc := sc.TestPlan.PlatformConfig
		if pc["api_secret"] != Redacted || pc["token"] != Redacted {
			t.Errorf("%s: secrets not redacted: %v", what, pc)
		}
		if pc["api_key"] != "k" || pc["url"] != "ws://x" {
			t.Errorf("%s: other keys changed: %v", what, pc)
		}
	}
	check("CreateScenario", created)
	got, err := s.GetScenario(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	check("GetScenario", got)
	if plan.PlatformConfig["api_secret"] != "s3cret-value" {
		t.Error("the caller's test plan was changed")
	}

	// The run gets the real values, and an adapter from the catalog.
	if err := s.ExecuteScenario(created.ID); err != nil {
		t.Fatal(err)
	}
	if tp := <-starter.started; tp.PlatformConfig["api_secret"] != "s3cret-value" || tp.PlatformConfig["token"] != "t0ken-value" {
		t.Errorf("execution got redacted secrets: %v", tp.PlatformConfig)
	}
	if a, ok := (<-starter.adapters).(namedAdapter); !ok || a.name != "beta" {
		t.Errorf("execution adapter = %v, want the catalog's beta adapter", a)
	}
	for _, secret := range []string{"s3cret-value", "t0ken-value"} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("logs contain %q", secret)
		}
	}
}

func TestService_CatalogAdapterErrorAtExecution(t *testing.T) {
	s := NewService(slog.New(&NoOpLogger{}), newCatalogStarter())
	sc, err := s.CreateScenario("b", "beta", TestPlan{Participants: 1, DurationSeconds: 1, PlatformConfig: map[string]any{"broken": true}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ExecuteScenario(sc.ID); err == nil || !strings.Contains(err.Error(), "broken config") {
		t.Errorf("ExecuteScenario err = %v, want the catalog's adapter error", err)
	}
	if got, _ := s.GetScenario(sc.ID); got.Status != "created" {
		t.Errorf("status after a failed start = %q, want created", got.Status)
	}
}

func TestRedactPlatformConfig(t *testing.T) {
	in := map[string]any{"jwt_app_secret": "x", "jwt": "", "room": "r"}
	out := redactPlatformConfig(in, []string{"jwt_app_secret", "jwt", "absent"})
	if out["jwt_app_secret"] != Redacted || out["room"] != "r" || in["jwt_app_secret"] != "x" {
		t.Errorf("redactPlatformConfig = %v (input %v)", out, in)
	}
	if out["jwt"] != "" {
		t.Errorf("an empty secret is shown as is (nothing to hide), got %v", out["jwt"])
	}
	if _, ok := out["absent"]; ok || !slices.Equal(sortedKeys(out), []string{"jwt", "jwt_app_secret", "room"}) {
		t.Errorf("keys changed: %v", out)
	}
	if redactPlatformConfig(nil, []string{"x"}) != nil {
		t.Error("nil config must stay nil")
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

func sortedKeys(m map[string]any) []string {
	var k []string
	for key := range m {
		k = append(k, key)
	}
	slices.Sort(k)
	return k
}

func TestRedactTestPlan(t *testing.T) {
	tp := TestPlan{Participants: 3, PlatformConfig: map[string]any{"room": "r", "api_secret": "s3cr3t", "token": ""}}
	got := RedactTestPlan(tp, []string{"api_secret", "token"})
	if got.PlatformConfig["api_secret"] != Redacted || got.PlatformConfig["room"] != "r" || got.PlatformConfig["token"] != "" {
		t.Errorf("redacted config = %v", got.PlatformConfig)
	}
	if got.Participants != 3 {
		t.Errorf("participants = %d", got.Participants)
	}
	if tp.PlatformConfig["api_secret"] != "s3cr3t" {
		t.Error("input config modified")
	}
	if RedactTestPlan(TestPlan{}, []string{"api_secret"}).PlatformConfig != nil {
		t.Error("nil config is not nil")
	}
}
