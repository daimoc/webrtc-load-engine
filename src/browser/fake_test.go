package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// A fake Chrome for the tests: the test binary run again with
// fakeModeEnv set. It reads --user-data-dir, serves a DevTools endpoint on
// a loopback port, writes DevToolsActivePort as Chrome does, and answers
// the commands the session sends. No real browser is needed.

const (
	fakeModeEnv = "WLE_FAKE_CHROME"      // ok, exit, silent, error-page, no-video, die
	fakeArgsEnv = "WLE_FAKE_CHROME_ARGS" // File the fake writes its arguments to
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(fakeModeEnv); mode != "" {
		runFakeChrome(mode)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runFakeChrome(mode string) {
	if f := os.Getenv(fakeArgsEnv); f != "" {
		_ = os.WriteFile(f, []byte(strings.Join(os.Args[1:], "\n")), 0o600)
	}
	switch mode {
	case "exit":
		os.Exit(3)
	case "silent":
		time.Sleep(time.Hour)
	}
	var profile string
	for _, a := range os.Args[1:] {
		if v, ok := strings.CutPrefix(a, "--user-data-dir="); ok {
			profile = v
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(4)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	page := &fakePage{mode: mode}
	mux := http.NewServeMux()
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"Browser":"FakeChrome/1.0"}`)
	})
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `[{"type":"page","id":"P1","webSocketDebuggerUrl":"ws://127.0.0.1:%d/devtools/page/P1"}]`, port)
	})
	mux.HandleFunc("/devtools/page/P1", page.serve)
	go func() { _ = http.Serve(ln, mux) }()
	_ = os.WriteFile(filepath.Join(profile, "DevToolsActivePort"), []byte(fmt.Sprintf("%d\n/devtools/browser/fake\n", port)), 0o600)
	time.Sleep(time.Hour)
}

type fakePage struct {
	mode      string
	evaluated atomic.Int64
}

func (p *fakePage) serve(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	ctx := context.Background()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var cmd struct {
			ID     int64           `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(data, &cmd) != nil {
			continue
		}
		result := p.answer(cmd.Method, cmd.Params)
		reply, _ := json.Marshal(map[string]any{"id": cmd.ID, "result": result})
		if err := c.Write(ctx, websocket.MessageText, reply); err != nil {
			return
		}
	}
}

// answer gives the fake's result for a command.
func (p *fakePage) answer(method string, params json.RawMessage) any {
	switch method {
	case "Page.navigate":
		return map[string]any{"frameId": "F"}
	case "Runtime.evaluate":
		var e struct {
			Expression string `json:"expression"`
		}
		_ = json.Unmarshal(params, &e)
		if !strings.Contains(e.Expression, "__wleCollect") {
			if p.mode == "error-page" {
				return evalValue(`{"href":"chrome-error://chromewebdata/","ready":"complete"}`)
			}
			return evalValue(`{"href":"https://h/r","ready":"complete"}`)
		}
		n := p.evaluated.Add(1)
		if p.mode == "die" && n > 3 {
			os.Exit(5)
		}
		if p.mode == "no-video" {
			return evalValue(`{"streams":[{"id":"probe","fd":0,"fc":0,"tfd":0,"b":1000}]}`)
		}
		// Two streams at 360 for 2 samples, then one at 720.
		fd := 30 * n
		h := 360
		if n > 2 {
			h = 720
		}
		return evalValue(fmt.Sprintf(`{"streams":[{"id":"a","h":%d,"fd":%d,"fc":0,"tfd":0,"b":%d},{"id":"b","h":360,"fd":%d,"fc":1,"tfd":0.5,"b":%d}]}`,
			h, fd, 62500*n, fd, 62500*n))
	}
	return map[string]any{}
}

func evalValue(v string) any {
	return map[string]any{"result": map[string]any{"type": "string", "value": v}}
}

// fakeOptions gives Options running the fake in mode.
func fakeOptions(t *testing.T, mode string) Options {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(fakeModeEnv, mode)
	return Options{URL: "https://h/r", Window: [2]int{1280, 720}, Executable: exe, ExpectedHeight: 720}
}
