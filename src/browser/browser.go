// Package browser runs the one headless browser a test plan may ask for
// beside the bots (specs/015-bandwidth-estimate-parity, research R1–R4).
// The engine starts it, opens the plan's address and reads what the
// browser measures itself; the page must join the room by itself. The
// browser is a reference, never a bot: it knows no platform and no Pion.
//
// Every failure becomes a reason in the result, never an error: a browser
// that fails does not change the run (FR-019).
package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"webrtc-load-engine/src/scenario"
)

// Timings; variables for tests.
var (
	sampleInterval = time.Second
	startTimeout   = 15 * time.Second // For DevTools to answer
	loadTimeout    = 15 * time.Second // For the page to load
	// evaluateTimeout bounds one sample, so that a page that never answers
	// costs samples, not the session.
	evaluateTimeout = 5 * time.Second
)

// executables are the browser programs looked for in PATH, in order.
var executables = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}

// Options is what the session needs from the test plan.
type Options struct {
	URL string
	// DisplayURL is the address as the report shows it (tokens hidden);
	// URL when empty.
	DisplayURL     string
	Window         [2]int // Width, height
	InsecureTLS    bool
	Executable     string // Empty: looked for in PATH
	ExpectedHeight int    // 0: not stated
}

// FindExecutable returns the browser to run: given when set, else the
// first of executables found in PATH.
func FindExecutable(given string) (string, error) {
	if given != "" {
		info, err := os.Stat(given)
		if err != nil {
			return "", err
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return "", fmt.Errorf("%s is not an executable file", given)
		}
		return given, nil
	}
	for _, name := range executables {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("none of %s found in PATH", strings.Join(executables, ", "))
}

// Session is one browser of a run.
type Session struct {
	opts   Options
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // Closed when run returns

	// Set by run before done is closed.
	cmd       *exec.Cmd
	started   time.Time
	exited    chan struct{} // Closed when the process is waited for; nil before it starts
	profile   string
	reason    string // A start or load failure
	gone      bool   // The browser ended before Stop
	version   string
	loadedAt  time.Time
	lastAt    time.Time
	acc       accumulator
	cpu       groupCPU
	stopOnce  sync.Once
	result    *scenario.BrowserResult
	processMu sync.Mutex
}

// Start starts the browser and returns at once; the session runs until
// Stop. It never fails: a failure is held in the session's result.
func Start(ctx context.Context, o Options, logger *slog.Logger) *Session {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{opts: o, logger: logger, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		s.run()
	}()
	return s
}

// pid returns the browser's process id (its process group), 0 before it
// starts.
func (s *Session) pid() int {
	s.processMu.Lock()
	defer s.processMu.Unlock()
	if s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

func (s *Session) fail(reason string, err error) {
	s.reason = reason
	s.logger.Warn("Browser not measured", slog.String("reason", reason), slog.String("error", err.Error()))
}

func (s *Session) run() {
	exe, err := FindExecutable(s.opts.Executable)
	if err != nil {
		s.fail(scenario.BrowserNotFound, err)
		return
	}
	s.profile, err = os.MkdirTemp("", "wle-browser-")
	if err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	cmd := exec.Command(exe, s.args()...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	s.processMu.Lock()
	s.cmd = cmd
	s.processMu.Unlock()
	s.started = time.Now()
	s.exited = make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(s.exited)
	}()

	port, err := s.waitPort()
	if err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	d, err := connectDevtools(s.ctx, port)
	if err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	defer d.close()
	s.version = d.version(s.ctx)
	s.logger.Info("Browser started", slog.String("browser", s.version), slog.Int("pid", cmd.Process.Pid))

	if _, err := d.call(s.ctx, "Page.enable", map[string]any{}); err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	if _, err := d.call(s.ctx, "Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": injectedScript}); err != nil {
		s.fail(scenario.BrowserStartFailed, err)
		return
	}
	// The navigation's errorText is not trusted: Chrome reported one for a
	// page that then loaded (R12 fact 3). The page's state decides.
	if _, err := d.call(s.ctx, "Page.navigate", map[string]any{"url": s.opts.URL}); err != nil {
		s.fail(scenario.PageLoadFailed, err)
		return
	}
	if err := s.waitLoad(d); err != nil {
		if s.ctx.Err() == nil && !s.processGone() {
			s.fail(scenario.PageLoadFailed, err)
		} else if s.processGone() {
			s.fail(scenario.BrowserExited, err)
		}
		return
	}
	s.loadedAt = time.Now()
	s.logger.Info("Browser page loaded", slog.String("url", s.displayURL()))
	s.sample(d)
}

func (s *Session) args() []string {
	a := []string{
		"--headless=new",
		"--user-data-dir=" + s.profile,
		"--remote-debugging-port=0",
		fmt.Sprintf("--window-size=%d,%d", s.opts.Window[0], s.opts.Window[1]),
		"--autoplay-policy=no-user-gesture-required",
		"--use-fake-ui-for-media-stream",
		"--use-fake-device-for-media-stream",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-background-timer-throttling",
		"--disable-renderer-backgrounding",
		"--disable-backgrounding-occluded-windows",
	}
	if s.opts.InsecureTLS {
		a = append(a, "--ignore-certificate-errors")
	}
	return append(a, "about:blank")
}

func (s *Session) processGone() bool {
	select {
	case <-s.exited:
		return true
	default:
		return false
	}
}

func (s *Session) waitPort() (int, error) {
	deadline := time.Now().Add(startTimeout)
	for {
		if port, ok := readActivePort(s.profile); ok {
			return port, nil
		}
		select {
		case <-s.exited:
			return 0, errors.New("the browser exited at start")
		case <-s.ctx.Done():
			return 0, s.ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return 0, fmt.Errorf("no DevTools port after %s", startTimeout)
		}
	}
}

// waitLoad waits for the page to be loaded, or to be Chrome's error page.
func (s *Session) waitLoad(d *devtools) error {
	deadline := time.Now().Add(loadTimeout)
	for {
		v, err := d.evaluate(s.ctx, `JSON.stringify({href: location.href, ready: document.readyState})`)
		if err == nil {
			var page struct {
				Href  string `json:"href"`
				Ready string `json:"ready"`
			}
			if json.Unmarshal([]byte(v), &page) == nil {
				if strings.HasPrefix(page.Href, "chrome-error://") {
					return errors.New("the browser shows its error page")
				}
				if page.Href != "about:blank" && (page.Ready == "complete" || page.Ready == "interactive") {
					return nil
				}
			}
		} else if errors.Is(err, errGone) {
			return err
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("page not loaded after %s", loadTimeout)
		}
	}
}

// sample reads the page's streams every sampleInterval until Stop or the
// browser's end.
func (s *Session) sample(d *devtools) {
	ticker := time.NewTicker(sampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.exited:
			s.gone = true
			return
		case <-ticker.C:
		}
		s.cpu.scan(s.pid())
		ctx, cancel := context.WithTimeout(s.ctx, evaluateTimeout)
		v, err := d.evaluate(ctx, collectExpression)
		cancel()
		now := time.Now()
		if err != nil {
			// Stop cancels the context, which also closes the connection:
			// that is the end of the run, not the browser's.
			if s.ctx.Err() != nil {
				return
			}
			if errors.Is(err, errGone) || s.processGone() {
				s.gone = true
				return
			}
			s.logger.Debug("Browser sample failed", slog.String("error", err.Error()))
			continue
		}
		var sample struct {
			Streams []streamStat `json:"streams"`
		}
		if err := json.Unmarshal([]byte(v), &sample); err != nil {
			s.logger.Debug("Browser sample not read", slog.String("error", err.Error()))
			continue
		}
		s.acc.add(now.Sub(s.loadedAt).Seconds(), sample.Streams)
		s.lastAt = now
	}
}

func (s *Session) displayURL() string {
	if s.opts.DisplayURL != "" {
		return s.opts.DisplayURL
	}
	return s.opts.URL
}

// Stop ends the browser and returns its result; later calls return the
// same result.
func (s *Session) Stop() *scenario.BrowserResult {
	s.stopOnce.Do(func() {
		s.cancel()
		<-s.done
		stopped := time.Now()
		if pid := s.pid(); pid != 0 {
			s.cpu.scan(pid)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
			select {
			case <-s.exited:
			case <-time.After(5 * time.Second):
				s.logger.Warn("Browser did not exit after SIGKILL", slog.Int("pid", pid))
			}
		}
		if s.profile != "" {
			if err := os.RemoveAll(s.profile); err != nil {
				s.logger.Warn("Browser profile not removed", slog.String("path", s.profile), slog.String("error", err.Error()))
			}
		}
		s.result = s.build(stopped)
		if st := s.cmd; st != nil && st.ProcessState != nil {
			s.logger.Debug("Browser CPU", slog.Float64("group_seconds", s.cpu.seconds()),
				slog.Float64("own_process_seconds", (st.ProcessState.UserTime()+st.ProcessState.SystemTime()).Seconds()))
		}
	})
	return s.result
}

func (s *Session) build(stopped time.Time) *scenario.BrowserResult {
	r := &scenario.BrowserResult{
		URL:            s.displayURL(),
		Window:         strconv.Itoa(s.opts.Window[0]) + "x" + strconv.Itoa(s.opts.Window[1]),
		BrowserVersion: s.version,
	}
	switch {
	case s.reason != "":
		r.Status, r.Reason = scenario.BrowserUnavailable, s.reason
		return r
	case s.gone && s.acc.decodedAny():
		r.Status, r.Reason = scenario.BrowserPartial, scenario.BrowserExited
		s.logger.Warn("Browser ended before the run", slog.String("reason", scenario.BrowserExited))
	case s.gone:
		r.Status, r.Reason = scenario.BrowserUnavailable, scenario.BrowserExited
		s.logger.Warn("Browser not measured", slog.String("reason", scenario.BrowserExited))
		return r
	case !s.acc.decodedAny():
		r.Status, r.Reason = scenario.BrowserUnavailable, scenario.NoVideoReceived
		s.logger.Warn("Browser not measured", slog.String("reason", scenario.NoVideoReceived))
		return r
	default:
		r.Status = scenario.BrowserMeasured
	}
	s.acc.summarize(r, s.opts.ExpectedHeight)
	if !s.lastAt.IsZero() {
		r.Period = &scenario.Period{Start: s.loadedAt.UTC(), End: s.lastAt.UTC()}
	}
	if wall := stopped.Sub(s.started).Seconds(); wall > 0 {
		cpu := s.cpu.seconds() / wall
		r.CPUThreadsAverage = &cpu
	}
	return r
}
