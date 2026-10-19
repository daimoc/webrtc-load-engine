package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

type noOpLogger struct{}

func (noOpLogger) Enabled(context.Context, slog.Level) bool  { return false }
func (noOpLogger) Handle(context.Context, slog.Record) error { return nil }
func (n noOpLogger) WithAttrs([]slog.Attr) slog.Handler      { return n }
func (n noOpLogger) WithGroup(string) slog.Handler           { return n }

func TestMainEngineStarter_RejectsMissingPlatformConfig(t *testing.T) {
	starter := NewMainEngineStarter(slog.New(noOpLogger{}))

	engine, err := starter.StartExecution("s1", scenario.TestPlan{Participants: 1, DurationSeconds: 1}, nil, func(string, scenario.RunResult) {})
	if err == nil {
		t.Fatal("expected an error when platform_config is missing, got nil")
	}
	if engine != nil {
		t.Errorf("expected no engine on error, got %v", engine)
	}
}

func TestMainEngineStarter_ConfiguresJitsiAdapterFromPlatformConfig(t *testing.T) {
	starter := NewMainEngineStarter(slog.New(noOpLogger{}))
	tp := scenario.TestPlan{
		Participants:    1,
		DurationSeconds: 1,
		PlatformConfig:  map[string]any{"xmpp_domain": "meet.example.org", "room_name": "load1"},
	}

	engine, err := starter.StartExecution("s1", tp, nil, func(string, scenario.RunResult) {})
	if err != nil {
		t.Fatalf("expected engine to start with valid platform_config, got %v", err)
	}
	if engine == nil {
		t.Fatal("expected an engine, got nil")
	}
}

// qualityStarter starts engines that complete at once with a summary.
type qualityStarter struct{}

type qualityEngine struct {
	id         string
	onComplete func(string, scenario.RunResult)
}

func (e *qualityEngine) Start() {
	e.onComplete(e.id, scenario.RunResult{Status: "completed", Quality: &scenario.QualitySummary{ScenarioID: e.id, Bots: 3}})
}
func (e *qualityEngine) Stop() {}

func (qualityStarter) StartExecution(id string, _ scenario.TestPlan, _ platform.PlatformAdapter, onComplete func(string, scenario.RunResult)) (scenario.ExecutionEngine, error) {
	return &qualityEngine{id: id, onComplete: onComplete}, nil
}

type noOpMetrics struct{}

func (noOpMetrics) IncScenarioCreationRequestsTotal(string)        {}
func (noOpMetrics) ObserveScenarioCreationDurationSeconds(float64) {}
func (noOpMetrics) IncScenarioExecutionRequestsTotal(string)       {}

func TestRouter_QualitySummary(t *testing.T) {
	logger := slog.New(noOpLogger{})
	service := scenario.NewService(logger, qualityStarter{})
	ts := httptest.NewServer(newRouter(handlers.NewScenarioHandler(service, logger, noOpMetrics{})))
	defer ts.Close()
	sc, err := service.CreateScenario("q", "jitsi", scenario.TestPlan{Participants: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	get := func(path string) (int, map[string]any) {
		t.Helper()
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var body map[string]any
		json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	if code, body := get("/api/v1/scenarios/" + sc.ID + "/quality"); code != http.StatusConflict || body["status"] != "created" {
		t.Fatalf("before a run: %d %v", code, body)
	}
	if code, _ := get("/api/v1/scenarios/unknown/quality"); code != http.StatusNotFound {
		t.Fatalf("unknown scenario: %d", code)
	}
	if err := service.ExecuteScenario(sc.ID); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		code, body := get("/api/v1/scenarios/" + sc.ID + "/quality")
		if code == http.StatusOK {
			if body["scenario_id"] != sc.ID || body["bots"] != float64(3) {
				t.Fatalf("summary: %v", body)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no summary: %d %v", code, body)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if code, body := get("/api/v1/scenarios/" + sc.ID); code != http.StatusOK || body["id"] != sc.ID {
		t.Fatalf("scenario route: %d %v", code, body)
	}
}
