package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"log/slog"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/report"
	"webrtc-load-engine/src/scenario"
)

// releaseStarter runs engines that end with a summary when released.
type releaseStarter struct{ release chan struct{} }

type releaseEngine struct {
	id         string
	release    chan struct{}
	onComplete func(string, scenario.RunResult)
}

func (e *releaseEngine) Start() {
	start := time.Now().UTC()
	<-e.release
	e.onComplete(e.id, scenario.RunResult{Status: "completed", StartedAt: start, EndedAt: time.Now().UTC(),
		Quality: &scenario.QualitySummary{ScenarioID: e.id, GeneratedAt: time.Now().UTC(), Bots: 2,
			Audio: scenario.AudioQuality{KindQuality: scenario.KindQuality{Available: true, Bots: 2},
				MOS: &scenario.Percentiles{P50: 4.4, P95: 4.3, P99: 4.2}}}})
}
func (e *releaseEngine) Stop() {}

func (s releaseStarter) StartExecution(id string, _ scenario.TestPlan, _ platform.PlatformAdapter, onComplete func(string, scenario.RunResult)) (scenario.ExecutionEngine, error) {
	return &releaseEngine{id: id, release: s.release, onComplete: onComplete}, nil
}

// newReportServer serves the scenario API with a report writer on dir.
func newReportServer(t *testing.T, dir string) (*httptest.Server, chan struct{}) {
	t.Helper()
	logger := slog.New(&NoOpLogger{})
	release := make(chan struct{})
	svc := scenario.NewService(logger, releaseStarter{release: release})
	svc.SetReporter(report.Writer{Dir: dir, Logger: logger})
	h := handlers.NewScenarioHandler(svc, logger, &NoOpMetrics{})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scenarios", h.CreateScenario)
	mux.HandleFunc("/api/v1/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/_execute"):
			h.ExecuteScenario(w, r)
		case strings.HasSuffix(r.URL.Path, "/quality"):
			h.GetQuality(w, r)
		default:
			h.GetScenario(w, r)
		}
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, release
}

type scenarioAnswer struct {
	ID     string                `json:"id"`
	Status string                `json:"status"`
	Report *scenario.ReportFiles `json:"report"`
}

func getAnswer(t *testing.T, ts *httptest.Server, id string) scenarioAnswer {
	t.Helper()
	var a scenarioAnswer
	if err := json.Unmarshal([]byte(getBody(t, ts, id)), &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func execute(t *testing.T, ts *httptest.Server, id string) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/api/v1/scenarios/"+id+"/_execute", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		t.Fatalf("execute: %d", resp.StatusCode)
	}
}

func waitReport(t *testing.T, ts *httptest.Server, id string) scenarioAnswer {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if a := getAnswer(t, ts, id); a.Report != nil {
			return a
		}
		if time.Now().After(deadline) {
			t.Fatal("no report in the scenario answer")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestRunReportInScenarioAnswer(t *testing.T) {
	dir := t.TempDir()
	ts, release := newReportServer(t, dir)
	status, out := create(t, ts, handlers.ScenarioRequest{Name: "two bots", Platform: "jitsi",
		TestPlan: scenario.TestPlan{Participants: 2, DurationSeconds: 30}})
	if status != http.StatusCreated {
		t.Fatalf("create: %d %v", status, out)
	}
	id := out["id"].(string)
	if body := getBody(t, ts, id); strings.Contains(body, `"report"`) {
		t.Errorf("report before the first run: %s", body)
	}
	execute(t, ts, id)
	if body := getBody(t, ts, id); strings.Contains(body, `"report"`) {
		t.Errorf("report while running: %s", body)
	}
	close(release)

	a := waitReport(t, ts, id)
	if a.Status != "completed" || !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(a.Report.RunID) {
		t.Fatalf("answer %+v", a)
	}
	if filepath.Dir(a.Report.JSON.Path) != dir {
		t.Fatalf("JSON report %+v not in %s", a.Report.JSON, dir)
	}
	raw, err := os.ReadFile(a.Report.JSON.Path)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Quality map[string]any `json:"quality"`
	}
	json.Unmarshal(raw, &file)

	resp, err := http.Get(ts.URL + "/api/v1/scenarios/" + id + "/quality")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var served map[string]any
	json.Unmarshal(body, &served)
	if !reflect.DeepEqual(file.Quality, served) {
		t.Errorf("report quality %v\nserved %v", file.Quality, served)
	}
	if _, ok := served["report"]; ok {
		t.Error("the quality answer gained a report field")
	}
}

func TestRunReportNotWritten(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	parent := t.TempDir()
	os.Chmod(parent, 0o555)
	t.Cleanup(func() { os.Chmod(parent, 0o755) })
	ts, release := newReportServer(t, filepath.Join(parent, "reports"))
	_, out := create(t, ts, handlers.ScenarioRequest{Name: "ro", Platform: "jitsi",
		TestPlan: scenario.TestPlan{Participants: 1, DurationSeconds: 1}})
	id := out["id"].(string)
	execute(t, ts, id)
	close(release)
	a := waitReport(t, ts, id)
	if a.Status != "completed" || a.Report.JSON.Path != "" || !strings.Contains(a.Report.JSON.Error, "permission denied") {
		t.Errorf("answer %+v, report %+v", a, a.Report)
	}
}
