package execution

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"webrtc-load-engine/src/browser"
	"webrtc-load-engine/src/scenario"
)

// fakeBrowser replaces the browser package's Start in the engine (015 T018).
type fakeBrowser struct {
	mu       sync.Mutex
	starts   int
	opts     browser.Options
	connects int32 // The adapter's connects when the browser started
	stopped  atomic.Bool
	result   *scenario.BrowserResult
}

func (f *fakeBrowser) install(t *testing.T, adapter *MockPlatformAdapter) {
	t.Helper()
	old := startBrowser
	startBrowser = func(_ context.Context, o browser.Options, _ *slog.Logger) browserSession {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.starts++
		f.opts = o
		f.connects = adapter.connectCount.Load()
		return f
	}
	t.Cleanup(func() { startBrowser = old })
}

func (f *fakeBrowser) Stop() *scenario.BrowserResult {
	f.stopped.Store(true)
	return f.result
}

func runEngine(t *testing.T, plan scenario.TestPlan, adapter *MockPlatformAdapter, stopAfter time.Duration) scenario.RunResult {
	t.Helper()
	done := make(chan scenario.RunResult, 1)
	e := NewEngine("browser-"+uuid.NewString(), plan, adapter, quietLogger, func(_ string, r scenario.RunResult) { done <- r })
	go e.Start()
	if stopAfter > 0 {
		time.Sleep(stopAfter)
		e.Stop()
	}
	select {
	case r := <-done:
		return r
	case <-time.After(15 * time.Second):
		t.Fatal("run did not end")
	}
	return scenario.RunResult{}
}

func TestEngine_NoBrowserWithoutSection(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	var f fakeBrowser
	f.install(t, adapter)
	r := runEngine(t, scenario.TestPlan{Participants: 2, DurationSeconds: 1}, adapter, 0)
	if f.starts != 0 || r.Browser != nil {
		t.Fatalf("browser started %d times, result %+v, for a plan without one", f.starts, r.Browser)
	}
}

func browserTestPlan(duration int) scenario.TestPlan {
	return scenario.TestPlan{Participants: 3, DurationSeconds: duration,
		Browser: &scenario.BrowserConfig{URL: "https://h/r?jwt=secret", Window: "800x600", ExpectedHeight: 720, InsecureTLS: true}}
}

func TestEngine_BrowserStartedAndStopped(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	f := fakeBrowser{result: &scenario.BrowserResult{Status: scenario.BrowserMeasured, URL: "x"}}
	f.install(t, adapter)
	r := runEngine(t, browserTestPlan(1), adapter, 0)
	if f.starts != 1 || !f.stopped.Load() {
		t.Fatalf("starts %d, stopped %v", f.starts, f.stopped.Load())
	}
	if f.connects != 0 {
		t.Errorf("browser started after %d bots", f.connects)
	}
	want := browser.Options{URL: "https://h/r?jwt=secret", DisplayURL: "https://h/r?jwt=[redacted]", Window: [2]int{800, 600},
		InsecureTLS: true, ExpectedHeight: 720}
	if f.opts != want {
		t.Errorf("options %+v, want %+v", f.opts, want)
	}
	if r.Browser != f.result {
		t.Errorf("result %+v not the browser's", r.Browser)
	}
	// The mock's bots receive no media, so the summary counts none; the
	// browser, which "received" video, must not be counted either.
	if r.Quality == nil || r.Quality.Bots != 0 {
		t.Errorf("the browser is counted with the bots: %d bots", r.Quality.Bots)
	}
}

func TestEngine_BrowserStoppedWhenRunStopped(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	f := fakeBrowser{result: &scenario.BrowserResult{Status: scenario.BrowserPartial}}
	f.install(t, adapter)
	r := runEngine(t, browserTestPlan(60), adapter, 500*time.Millisecond)
	if r.Status != "stopped" || !f.stopped.Load() || r.Browser != f.result {
		t.Fatalf("status %s, browser stopped %v, result %+v", r.Status, f.stopped.Load(), r.Browser)
	}
}

func TestEngine_BrowserFailureLeavesRun(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	f := fakeBrowser{result: &scenario.BrowserResult{Status: scenario.BrowserUnavailable, Reason: scenario.BrowserNotFound}}
	f.install(t, adapter)
	r := runEngine(t, browserTestPlan(1), adapter, 0)
	if r.Status != "completed" || r.Error != "" || r.Browser.Reason != scenario.BrowserNotFound {
		t.Fatalf("status %s, error %q, browser %+v", r.Status, r.Error, r.Browser)
	}
	if got := adapter.connectCount.Load(); got != 3 {
		t.Fatalf("%d bots connected, want 3", got)
	}
}
