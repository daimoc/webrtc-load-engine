package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// A minimal Chrome DevTools protocol client (research R1): commands with
// increasing ids over one page target's WebSocket, answers matched by id.
// Events are not read: the session polls instead.

// errGone is returned once the browser's connection is closed.
var errGone = errors.New("browser connection closed")

type devtools struct {
	conn *websocket.Conn
	base string // http://127.0.0.1:<port>

	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan reply // Guarded by mu
	closed  bool                 // Guarded by mu
	done    chan struct{}
}

type reply struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// readActivePort reads the DevTools port Chrome writes into its profile
// folder; ok is false until the file is complete.
func readActivePort(profile string) (port int, ok bool) {
	raw, err := os.ReadFile(filepath.Join(profile, "DevToolsActivePort"))
	if err != nil {
		return 0, false
	}
	first, _, found := strings.Cut(string(raw), "\n")
	if !found {
		return 0, false
	}
	port, err = strconv.Atoi(strings.TrimSpace(first))
	return port, err == nil && port > 0
}

// connectDevtools connects to the first page target of the browser on port.
func connectDevtools(ctx context.Context, port int) (*devtools, error) {
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	var targets []struct {
		Type string `json:"type"`
		URL  string `json:"webSocketDebuggerUrl"`
	}
	if err := getJSON(ctx, base+"/json/list", &targets); err != nil {
		return nil, err
	}
	var wsURL string
	for _, t := range targets {
		if t.Type == "page" && t.URL != "" {
			wsURL = t.URL
			break
		}
	}
	if wsURL == "" {
		return nil, errors.New("no page target")
	}
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(16 << 20) // getStats of many streams
	d := &devtools{conn: conn, base: base, pending: map[int64]chan reply{}, done: make(chan struct{})}
	go d.read()
	return d, nil
}

// version returns the browser's product, e.g. "Chrome/156.0.8078.79".
func (d *devtools) version(ctx context.Context) string {
	var v struct {
		Browser string `json:"Browser"`
	}
	if getJSON(ctx, d.base+"/json/version", &v) != nil {
		return ""
	}
	return v.Browser
}

func (d *devtools) read() {
	defer d.shut()
	for {
		_, data, err := d.conn.Read(context.Background())
		if err != nil {
			return
		}
		var msg struct {
			ID int64 `json:"id"`
			reply
		}
		if json.Unmarshal(data, &msg) != nil || msg.ID == 0 {
			continue // An event
		}
		d.mu.Lock()
		ch := d.pending[msg.ID]
		delete(d.pending, msg.ID)
		d.mu.Unlock()
		if ch != nil {
			ch <- msg.reply
		}
	}
}

func (d *devtools) shut() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.done)
	}
}

// call sends one command and waits for its answer.
func (d *devtools) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return nil, errGone
	}
	d.nextID++
	id := d.nextID
	ch := make(chan reply, 1)
	d.pending[id] = ch
	d.mu.Unlock()
	msg, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	if err := d.conn.Write(ctx, websocket.MessageText, msg); err != nil {
		return nil, errGone
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, r.Error.Message)
		}
		return r.Result, nil
	case <-d.done:
		return nil, errGone
	case <-ctx.Done():
		d.mu.Lock()
		delete(d.pending, id)
		d.mu.Unlock()
		return nil, ctx.Err()
	}
}

// evaluate runs expression in the page, awaiting a promise, and returns its
// string value.
func (d *devtools) evaluate(ctx context.Context, expression string) (string, error) {
	raw, err := d.call(ctx, "Runtime.evaluate", map[string]any{
		"expression": expression, "awaitPromise": true, "returnByValue": true,
	})
	if err != nil {
		return "", err
	}
	var r struct {
		Result struct {
			Value any `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	if r.ExceptionDetails != nil {
		return "", fmt.Errorf("evaluation failed: %s", r.ExceptionDetails.Text)
	}
	s, ok := r.Result.Value.(string)
	if !ok {
		return "", fmt.Errorf("evaluation gave %T, not a string", r.Result.Value)
	}
	return s, nil
}

func (d *devtools) close() {
	_ = d.conn.Close(websocket.StatusNormalClosure, "")
	d.shut()
}

func getJSON(ctx context.Context, url string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
