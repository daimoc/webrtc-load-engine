package scenario

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"webrtc-load-engine/src/platform"
)

// fakeReporter records the runs it is asked to report.
type fakeReporter struct {
	mu    sync.Mutex
	calls []fakeReport
	delay time.Duration
	files ReportFiles
	done  atomic.Int32 // WriteReport calls that returned
}

type fakeReport struct {
	sc *Scenario
	r  RunResult
}

func (f *fakeReporter) WriteReport(sc *Scenario, r RunResult) ReportFiles {
	f.mu.Lock()
	f.calls = append(f.calls, fakeReport{sc, r})
	f.mu.Unlock()
	time.Sleep(f.delay)
	f.done.Add(1)
	return f.files
}

func (f *fakeReporter) reports() []fakeReport {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]fakeReport(nil), f.calls...)
}

// gateStarter starts engines that run until released or stopped.
type gateStarter struct{ engines chan *gateEngine }

type gateEngine struct {
	id         string
	onComplete func(string, RunResult)
	release    chan string // Status to end with
	stopOnce   sync.Once
}

func (e *gateEngine) Start() { e.onComplete(e.id, RunResult{Status: <-e.release}) }
func (e *gateEngine) Stop()  { e.stopOnce.Do(func() { e.release <- "stopped" }) }

func (g gateStarter) StartExecution(id string, _ TestPlan, _ platform.PlatformAdapter, onComplete func(string, RunResult)) (ExecutionEngine, error) {
	e := &gateEngine{id: id, onComplete: onComplete, release: make(chan string, 1)}
	g.engines <- e
	return e, nil
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestService_ReportsEachRun(t *testing.T) {
	reporter := &fakeReporter{files: ReportFiles{RunID: "3f9a1c2e", JSON: ReportFile{Path: "/r/a.json"}, Markdown: ReportFile{Path: "/r/a.md"}}}
	service := NewService(slog.New(&NoOpLogger{}), newCatalogStarter())
	service.SetReporter(reporter)
	created, err := service.CreateScenario("lk", "beta", TestPlan{Participants: 1, DurationSeconds: 1,
		PlatformConfig: map[string]any{"room": "r", "api_secret": "s3cr3t"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := service.GetScenario(created.ID); got.Report() != nil {
		t.Errorf("report before any run: %+v", got.Report())
	}
	if err := service.ExecuteScenario(created.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the report", func() bool { got, _ := service.GetScenario(created.ID); return got.Report() != nil })

	calls := reporter.reports()
	if len(calls) != 1 {
		t.Fatalf("%d WriteReport calls", len(calls))
	}
	c := calls[0]
	if c.sc.ID != created.ID || c.sc.Name != "lk" || c.sc.Platform != "beta" || c.r.Status != "completed" {
		t.Errorf("reported %+v, %+v", c.sc, c.r)
	}
	if c.sc.TestPlan.PlatformConfig["api_secret"] != Redacted || c.sc.TestPlan.PlatformConfig["room"] != "r" {
		t.Errorf("test plan not redacted: %v", c.sc.TestPlan.PlatformConfig)
	}
	got, _ := service.GetScenario(created.ID)
	if *got.Report() != reporter.files {
		t.Errorf("report %+v, want %+v", got.Report(), reporter.files)
	}
}

func TestService_ReportResetWhileRunning(t *testing.T) {
	starter := gateStarter{engines: make(chan *gateEngine, 2)}
	reporter := &fakeReporter{files: ReportFiles{RunID: "00000001"}}
	service := NewService(slog.New(&NoOpLogger{}), starter)
	service.SetReporter(reporter)
	created, _ := service.CreateScenario("re", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})

	service.ExecuteScenario(created.ID)
	(<-starter.engines).release <- "completed"
	waitFor(t, "the first report", func() bool { got, _ := service.GetScenario(created.ID); return got.Report() != nil })

	if err := service.ExecuteScenario(created.ID); err != nil {
		t.Fatal(err)
	}
	second := <-starter.engines
	if got, _ := service.GetScenario(created.ID); got.Status != "running" || got.Report() != nil {
		t.Errorf("while running: status %q, report %+v", got.Status, got.Report())
	}
	second.release <- "completed"
	waitFor(t, "the second report", func() bool { return reporter.done.Load() == 2 })
}

func TestService_SlowReportDoesNotBlockReaders(t *testing.T) {
	reporter := &fakeReporter{delay: 500 * time.Millisecond}
	service := NewService(slog.New(&NoOpLogger{}), completingStarter{})
	service.SetReporter(reporter)
	created, _ := service.CreateScenario("slow", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})
	service.ExecuteScenario(created.ID)
	waitFor(t, "the write to start", func() bool { return len(reporter.reports()) == 1 })

	start := time.Now()
	got, err := service.GetScenario(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	service.GetQuality(created.ID)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("readers blocked %v by the report write", d)
	}
	if got.Status != "completed" || got.Report() != nil {
		t.Errorf("during the write: status %q, report %+v", got.Status, got.Report())
	}
}

func TestService_ReportFailureKeepsStatus(t *testing.T) {
	reporter := &fakeReporter{files: ReportFiles{RunID: "00000002",
		JSON: ReportFile{Error: "permission denied"}, Markdown: ReportFile{Error: "permission denied"}}}
	service := NewService(slog.New(&NoOpLogger{}), completingStarter{})
	service.SetReporter(reporter)
	created, _ := service.CreateScenario("ro", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})
	service.ExecuteScenario(created.ID)
	waitFor(t, "the report", func() bool { got, _ := service.GetScenario(created.ID); return got.Report() != nil })
	got, _ := service.GetScenario(created.ID)
	if got.Status != "completed" || got.Report().JSON.Error != "permission denied" {
		t.Errorf("status %q, report %+v", got.Status, got.Report())
	}
}

func TestService_ShutdownWaitsForReports(t *testing.T) {
	starter := gateStarter{engines: make(chan *gateEngine, 1)}
	reporter := &fakeReporter{delay: 200 * time.Millisecond}
	service := NewService(slog.New(&NoOpLogger{}), starter)
	service.SetReporter(reporter)
	created, _ := service.CreateScenario("shutdown", "jitsi", TestPlan{Participants: 1, DurationSeconds: 60})
	service.ExecuteScenario(created.ID)
	<-starter.engines
	waitFor(t, "the run to start", func() bool { got, _ := service.GetScenario(created.ID); return got.Status == "running" })

	service.Shutdown()
	if reporter.done.Load() != 1 {
		t.Fatalf("Shutdown returned before the report was written (%d writes)", reporter.done.Load())
	}
	if c := reporter.reports()[0]; c.r.Status != "stopped" {
		t.Errorf("reported status %q", c.r.Status)
	}
}
