package browser

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func shortTimings(t *testing.T) {
	t.Helper()
	si, st, lt := sampleInterval, startTimeout, loadTimeout
	sampleInterval, startTimeout, loadTimeout = 100*time.Millisecond, 3*time.Second, 2*time.Second
	t.Cleanup(func() { sampleInterval, startTimeout, loadTimeout = si, st, lt })
}

func TestFindExecutable(t *testing.T) {
	dir := t.TempDir()
	chromium := filepath.Join(dir, "chromium")
	if err := os.WriteFile(chromium, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if got, err := FindExecutable(""); err != nil || got != chromium {
		t.Fatalf("FindExecutable(\"\") = %q, %v; want %s", got, err, chromium)
	}
	t.Setenv("PATH", "")
	if _, err := FindExecutable(""); err == nil {
		t.Fatal("found a browser with an empty PATH")
	}
	if _, err := FindExecutable("/no/such/chrome"); err == nil {
		t.Fatal("found /no/such/chrome")
	}
	if got, err := FindExecutable(chromium); err != nil || got != chromium {
		t.Fatalf("FindExecutable(path) = %q, %v", got, err)
	}
}

func run(t *testing.T, o Options, d time.Duration) *scenario.BrowserResult {
	t.Helper()
	s := Start(context.Background(), o, quietLogger())
	time.Sleep(d)
	return s.Stop()
}

func TestStart_NotFound(t *testing.T) {
	shortTimings(t)
	r := run(t, Options{URL: "https://h/r", Window: [2]int{1280, 720}, Executable: "/no/such/chrome"}, 0)
	if r.Status != scenario.BrowserUnavailable || r.Reason != scenario.BrowserNotFound {
		t.Fatalf("got %s/%s", r.Status, r.Reason)
	}
	if r.URL != "https://h/r" || r.Window != "1280x720" {
		t.Fatalf("url and window not kept: %+v", r)
	}
}

func TestStart_Failures(t *testing.T) {
	shortTimings(t)
	cases := map[string]string{
		"exit":       scenario.BrowserStartFailed,
		"silent":     scenario.BrowserStartFailed,
		"error-page": scenario.PageLoadFailed,
		"no-video":   scenario.NoVideoReceived,
	}
	for mode, reason := range cases {
		t.Run(mode, func(t *testing.T) {
			r := run(t, fakeOptions(t, mode), 3500*time.Millisecond)
			if r.Status != scenario.BrowserUnavailable || r.Reason != reason {
				t.Fatalf("got %s/%s, want unavailable/%s", r.Status, r.Reason, reason)
			}
		})
	}
}

func TestStart_Measured(t *testing.T) {
	shortTimings(t)
	o := fakeOptions(t, "ok")
	r := run(t, o, 1500*time.Millisecond)
	if r.Status != scenario.BrowserMeasured {
		t.Fatalf("status %s/%s", r.Status, r.Reason)
	}
	if r.BrowserVersion != "FakeChrome/1.0" || len(r.Samples) < 3 || r.VideoStreams != 2 {
		t.Fatalf("result %+v", r)
	}
	if r.HeightShare["720"] == 0 || r.HeightShare["360"] == 0 {
		t.Fatalf("height shares %v", r.HeightShare)
	}
	if r.ExpectedHeightShare == nil || r.SecondsToExpectedHeight == nil {
		t.Fatalf("expected height measures missing: %+v", r)
	}
	if r.FreezesTotal != 1 || r.FreezeSeconds != 0.5 || r.FreezeSource != "browser" {
		t.Fatalf("freezes %d %g %s", r.FreezesTotal, r.FreezeSeconds, r.FreezeSource)
	}
	if r.Period == nil || r.CPUThreadsAverage == nil {
		t.Fatalf("period or CPU missing: %+v", r)
	}
}

func TestStart_Exited(t *testing.T) {
	shortTimings(t)
	r := run(t, fakeOptions(t, "die"), 1500*time.Millisecond)
	if r.Status != scenario.BrowserPartial || r.Reason != scenario.BrowserExited {
		t.Fatalf("got %s/%s, want partial/%s", r.Status, r.Reason, scenario.BrowserExited)
	}
	if len(r.Samples) == 0 {
		t.Fatal("samples before the exit are lost")
	}
}

func TestStop_Twice(t *testing.T) {
	shortTimings(t)
	s := Start(context.Background(), fakeOptions(t, "ok"), quietLogger())
	time.Sleep(500 * time.Millisecond)
	a, b := s.Stop(), s.Stop()
	if a != b {
		t.Fatal("Stop twice gave two results")
	}
}

func TestStop_CleansUp(t *testing.T) {
	shortTimings(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv(fakeArgsEnv, argsFile)
	o := fakeOptions(t, "ok")
	o.InsecureTLS = true
	s := Start(context.Background(), o, quietLogger())
	time.Sleep(500 * time.Millisecond)
	pid := s.pid()
	s.Stop()
	if pid == 0 {
		t.Fatal("no process")
	}
	if err := syscall.Kill(-pid, 0); err != syscall.ESRCH {
		t.Fatalf("process group %d still there: %v", pid, err)
	}
	raw, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(string(raw), "\n")
	var profile string
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--user-data-dir="); ok {
			profile = v
		}
	}
	if profile == "" {
		t.Fatalf("no --user-data-dir in %v", args)
	}
	if _, err := os.Stat(profile); !os.IsNotExist(err) {
		t.Fatalf("profile folder %s not removed: %v", profile, err)
	}
	for _, want := range []string{"--headless=new", "--window-size=1280,720", "--remote-debugging-port=0",
		"--autoplay-policy=no-user-gesture-required", "--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream",
		"--no-first-run", "--disable-background-timer-throttling", "--disable-renderer-backgrounding", "--ignore-certificate-errors"} {
		if !slices.Contains(args, want) {
			t.Errorf("missing %s in %v", want, args)
		}
	}
}

func TestStart_NoInsecureFlagByDefault(t *testing.T) {
	shortTimings(t)
	argsFile := filepath.Join(t.TempDir(), "args")
	t.Setenv(fakeArgsEnv, argsFile)
	run(t, fakeOptions(t, "ok"), 300*time.Millisecond)
	raw, _ := os.ReadFile(argsFile)
	if strings.Contains(string(raw), "--ignore-certificate-errors") {
		t.Fatal("--ignore-certificate-errors without InsecureTLS")
	}
}
