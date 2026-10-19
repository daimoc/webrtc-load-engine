package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/platform/catalog"
	"webrtc-load-engine/src/scenario"
)

// catalogStarter is the engine's platform catalog with a starter that runs
// nothing: creation-time rules and redaction as the real server has them
// (008 research R10).
type catalogStarter struct{ *catalog.Catalog }

func (catalogStarter) StartExecution(string, scenario.TestPlan, platform.PlatformAdapter, func(string, scenario.RunResult)) (scenario.ExecutionEngine, error) {
	return nil, nil
}

// newPlatformServer serves create and get with the catalog.
func newPlatformServer(t *testing.T) *httptest.Server {
	t.Helper()
	logger := slog.New(&NoOpLogger{})
	svc := scenario.NewService(logger, catalogStarter{catalog.New(logger)})
	h := handlers.NewScenarioHandler(svc, logger, &NoOpMetrics{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scenarios", h.CreateScenario)
	mux.HandleFunc("/api/v1/scenarios/", h.GetScenario)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// create posts a scenario and returns the status and the decoded body.
func create(t *testing.T, ts *httptest.Server, req handlers.ScenarioRequest) (int, map[string]any) {
	t.Helper()
	body, _ := json.Marshal(req)
	resp, err := http.Post(ts.URL+"/api/v1/scenarios", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// getBody gets a scenario and returns the raw answer.
func getBody(t *testing.T, ts *httptest.Server, id string) string {
	t.Helper()
	resp, err := http.Get(ts.URL + "/api/v1/scenarios/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET scenario: %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func TestJitsiSecretsNotInAnswers(t *testing.T) {
	ts := newPlatformServer(t)
	status, out := create(t, ts, handlers.ScenarioRequest{Name: "jwt", Platform: "jitsi", TestPlan: scenario.TestPlan{
		Participants: 1, DurationSeconds: 1,
		PlatformConfig: map[string]any{"xmpp_domain": "meet.jitsi", "jwt_app_id": "app", "jwt_app_secret": "very-secret"},
	}})
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, out)
	}
	if body := getBody(t, ts, out["id"].(string)); strings.Contains(body, "very-secret") {
		t.Errorf("GET answer contains the secret: %s", body)
	}
}

func TestLiveKitCreationRules(t *testing.T) {
	ts := newPlatformServer(t)
	cfg := func(change map[string]any) map[string]any {
		m := map[string]any{"url": "ws://localhost:7880", "room": "load008", "api_key": "devkey", "api_secret": "lk-secret-value"}
		for k, v := range change {
			if v == nil {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
		return m
	}
	plan := func(c map[string]any, m *scenario.MediaConfig) scenario.TestPlan {
		return scenario.TestPlan{Participants: 2, DurationSeconds: 10, PlatformConfig: c, Media: m}
	}
	av := func() *scenario.MediaConfig { return &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo} }
	for _, tt := range []struct {
		name string
		plan scenario.TestPlan
		msg  string
	}{
		{"missing url", plan(cfg(map[string]any{"url": nil}), nil), "platform_config.url is required"},
		{"missing room", plan(cfg(map[string]any{"room": nil}), nil), "platform_config.room is required"},
		{"key without secret", plan(cfg(map[string]any{"api_secret": nil}), nil), "platform_config.api_key and platform_config.api_secret must be set together"},
		{"token and key", plan(cfg(map[string]any{"token": "t"}), nil), "platform_config.token cannot be combined with api_key/api_secret"},
		{"simulcast", plan(cfg(nil), &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, Simulcast: &scenario.SimulcastConfig{MaxHeight: 360}}), "media.simulcast is not supported on platform livekit"},
		{"requested heights", plan(cfg(nil), &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo, RequestedHeights: []scenario.RequestedHeight{{Height: 360, Share: 1}}}), "media.requested_heights is not supported on platform livekit"},
	} {
		status, out := create(t, ts, handlers.ScenarioRequest{Name: tt.name, Platform: "livekit", TestPlan: tt.plan})
		if status != http.StatusBadRequest || out["error"] != tt.msg {
			t.Errorf("%s: %d %v, want 400 %q", tt.name, status, out, tt.msg)
		}
	}

	status, out := create(t, ts, handlers.ScenarioRequest{Name: "ok", Platform: "livekit", TestPlan: plan(cfg(nil), av())})
	if status != http.StatusCreated {
		t.Fatalf("valid livekit plan: %d %v", status, out)
	}
	if body := getBody(t, ts, out["id"].(string)); strings.Contains(body, "lk-secret-value") {
		t.Errorf("GET answer contains the secret: %s", body)
	}
}
