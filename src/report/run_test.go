package report

import (
	"bytes"
	"encoding/json"
	"runtime"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// testScenario is a redacted scenario snapshot as the service hands it.
func testScenario() *scenario.Scenario {
	return &scenario.Scenario{
		ID:       "scenario-6d0c1e2a",
		Name:     "Réunion/10 bots",
		Platform: "jitsi",
		TestPlan: scenario.TestPlan{Participants: 10, DurationSeconds: 60,
			PlatformConfig: map[string]any{"room_name": "r009", "jwt_app_secret": scenario.Redacted}},
	}
}

func testResult() scenario.RunResult {
	start := time.Date(2026, 10, 18, 15, 29, 0, 0, time.UTC)
	return scenario.RunResult{
		Status:    "completed",
		StartedAt: start,
		EndedAt:   start.Add(72*time.Second + 123456789*time.Nanosecond),
		Quality: &scenario.QualitySummary{ScenarioID: "scenario-6d0c1e2a", Bots: 10,
			Audio: scenario.AudioQuality{KindQuality: scenario.KindQuality{Available: true, Bots: 10},
				MOS: &scenario.Percentiles{P50: 4.38, P95: 4.2, P99: 4.11}}},
	}
}

func TestNewRunJSON(t *testing.T) {
	run := NewRun(testScenario(), testResult())
	raw, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"format_version", "run_id", "scenario", "test_plan", "started_at", "ended_at", "duration_seconds", "outcome", "engine", "quality"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, raw)
		}
	}
	if string(got["format_version"]) != "1" {
		t.Errorf("format_version = %s", got["format_version"])
	}
	if string(got["duration_seconds"]) != "72.123" {
		t.Errorf("duration_seconds = %s", got["duration_seconds"])
	}
	if string(got["scenario"]) != `{"id":"scenario-6d0c1e2a","name":"Réunion/10 bots","platform":"jitsi"}` {
		t.Errorf("scenario = %s", got["scenario"])
	}
	if string(got["outcome"]) != `{"status":"completed"}` {
		t.Errorf("outcome = %s", got["outcome"])
	}
	if string(got["started_at"]) != `"2026-10-18T15:29:00Z"` {
		t.Errorf("started_at = %s", got["started_at"])
	}
	// The summary is the one the API serves (SC-002).
	want, _ := json.Marshal(testResult().Quality)
	if !bytes.Equal(got["quality"], want) {
		t.Errorf("quality = %s, want %s", got["quality"], want)
	}
	var engine struct {
		Version   string `json:"version"`
		GoVersion string `json:"go_version"`
	}
	json.Unmarshal(got["engine"], &engine)
	if engine.Version == "" || engine.GoVersion != runtime.Version() {
		t.Errorf("engine = %+v", engine)
	}
}

func TestNewRunFailed(t *testing.T) {
	r := testResult()
	r.Status, r.Error, r.Quality = "failed", "open clip.mp4: no such file", nil
	raw, _ := json.Marshal(NewRun(testScenario(), r))
	var got map[string]json.RawMessage
	json.Unmarshal(raw, &got)
	if string(got["quality"]) != "null" {
		t.Errorf("quality = %s", got["quality"])
	}
	if string(got["outcome"]) != `{"status":"failed","error":"open clip.mp4: no such file"}` {
		t.Errorf("outcome = %s", got["outcome"])
	}
}

func TestEngineVersion(t *testing.T) {
	if v := EngineVersion(); v == "" {
		t.Error("empty engine version")
	}
}
