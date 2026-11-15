package report

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
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
	if string(got["format_version"]) != "4" {
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
	if _, ok := got["browser"]; ok {
		t.Errorf("browser given for a plan without one: %s", got["browser"])
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

func TestNewRunBrowser(t *testing.T) {
	r := testResult()
	r.Browser = &scenario.BrowserResult{Status: scenario.BrowserUnavailable, Reason: scenario.BrowserNotFound,
		URL: "https://h/r", Window: "1280x720"}
	raw, err := json.Marshal(NewRun(testScenario(), r))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := `{"status":"unavailable","reason":"browser_not_found","url":"https://h/r","window":"1280x720"}`
	if string(got["browser"]) != want {
		t.Errorf("browser = %s, want %s", got["browser"], want)
	}
}

// A run without network has no network key; one with it carries the
// contract's shape (016).
func TestNewRunNetwork(t *testing.T) {
	raw, err := json.Marshal(NewRun(testScenario(), testResult()))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["network"]; ok {
		t.Error("network key in a run without network")
	}
	if string(got["format_version"]) != "4" {
		t.Errorf("format_version = %s, want 4", got["format_version"])
	}

	r := testResult()
	f := func(v float64) *float64 { return &v }
	yes := true
	r.Network = &scenario.NetworkResult{Seed: 42, SeedSource: scenario.SeedFromTestPlan, SettleSeconds: 10, TimelineResolutionSeconds: 1,
		Groups: []scenario.GroupResult{{Name: "slow", Bots: []string{"b1"}, Steps: []scenario.AppliedStep{{Index: 0,
			Planned:          scenario.ScheduleStep{StartSeconds: 20, Downlink: &scenario.LinkCondition{RateKbps: 300, QueueMs: 200}},
			StartedAtSeconds: f(20), EndedAtSeconds: f(40)}}}},
		Bots: []scenario.BotNetwork{{Bot: "b1", Group: "slow", RequestedHeight: 720, Steps: []scenario.BotStep{{Index: 0,
			Downlink:        &scenario.DirectionMeasure{OfferedPackets: 10, DeliveredPackets: 9, DeliveredKbps: scenario.KbpsMean{Mean: 297, Max10s: 301}, DroppedByCapShare: f(0.1), AddedDelayMs: scenario.MeanMax{Mean: 150, Max: 201}},
			ReactionSeconds: f(2), RecoverySeconds: f(6), Recovered: &yes}},
			Sender:         &scenario.SenderResult{Reason: scenario.NoUplinkCondition},
			ServerEstimate: &scenario.ServerEstimate{Reason: scenario.NotGivenByServer}}}}
	raw, err = json.Marshal(NewRun(testScenario(), r))
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		Network scenario.NetworkResult `json:"network"`
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	n := back.Network
	if n.Seed != 42 || n.Groups[0].Steps[0].Planned.Downlink.RateKbps != 300 || *n.Bots[0].Steps[0].RecoverySeconds != 6 ||
		n.Bots[0].Steps[0].Downlink.DeliveredKbps.Max10s != 301 || n.Bots[0].ServerEstimate.Reason != scenario.NotGivenByServer {
		t.Errorf("network did not round-trip: %+v", n)
	}
	for _, key := range []string{`"max_10s":301`, `"during_start_up":false`, `"dropped_by_loss_share":null`, `"recovered":true`, `"timeline_resolution_seconds":1`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("report lacks %s", key)
		}
	}
}
