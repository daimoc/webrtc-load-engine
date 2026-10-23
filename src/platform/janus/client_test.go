package janus

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// scriptServer is a WebSocket server speaking janus-protocol whose replies
// a test scripts: handle is called with each frame the client sends and
// the server's connection.
type scriptServer struct {
	*httptest.Server
	mu     sync.Mutex
	frames []map[string]any
	conn   *websocket.Conn
}

func newScriptServer(t *testing.T, handle func(s *scriptServer, f map[string]any)) *scriptServer {
	t.Helper()
	s := &scriptServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"janus-protocol"}})
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conn = c
		s.mu.Unlock()
		for {
			_, b, err := c.Read(context.Background())
			if err != nil {
				return
			}
			var f map[string]any
			json.Unmarshal(b, &f)
			s.mu.Lock()
			s.frames = append(s.frames, f)
			s.mu.Unlock()
			handle(s, f)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *scriptServer) send(m any) {
	b, _ := json.Marshal(m)
	s.mu.Lock()
	c := s.conn
	s.mu.Unlock()
	c.Write(context.Background(), websocket.MessageText, b)
}

func (s *scriptServer) sent() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.frames...)
}

func (s *scriptServer) wsURL() string { return "ws" + strings.TrimPrefix(s.URL, "http") }

func dialTest(t *testing.T, s *scriptServer, apiSecret, token string) *janusConn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, err := dialJanus(ctx, s.wsURL(), false)
	if err != nil {
		t.Fatal(err)
	}
	c := newJanusConn(ws, slog.New(slog.NewTextHandler(io.Discard, nil)), apiSecret, token)
	go c.readLoop()
	t.Cleanup(c.Close)
	return c
}

func TestClient_CoreRequestAndSession(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		switch f["janus"] {
		case "create":
			s.send(map[string]any{"janus": "success", "transaction": f["transaction"], "data": map[string]any{"id": uint64(7342466929253127)}})
		case "attach":
			s.send(map[string]any{"janus": "success", "transaction": f["transaction"], "session_id": f["session_id"], "data": map[string]any{"id": 11}})
		}
	})
	c := dialTest(t, s, "", "")
	if err := c.CreateSession(context.Background()); err != nil || c.SessionID() != 7342466929253127 {
		t.Fatalf("create: %v, session %d", err, c.SessionID())
	}
	h, err := c.Attach(context.Background())
	if err != nil || h != 11 {
		t.Fatalf("attach = %d, %v", h, err)
	}
	f := s.sent()
	if f[0]["session_id"] != nil {
		t.Errorf("create carried session_id: %v", f[0])
	}
	if f[1]["plugin"] != "janus.plugin.videoroom" || f[1]["session_id"].(float64) != 7342466929253127 {
		t.Errorf("attach frame = %v", f[1])
	}
	if f[0]["transaction"] == f[1]["transaction"] || f[0]["transaction"] == "" {
		t.Errorf("transactions not unique: %v %v", f[0]["transaction"], f[1]["transaction"])
	}
}

func TestClient_OutOfOrderReplies(t *testing.T) {
	var mu sync.Mutex
	var held []map[string]any
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		mu.Lock()
		held = append(held, f)
		if len(held) == 2 {
			for i := 1; i >= 0; i-- {
				s.send(map[string]any{"janus": "success", "transaction": held[i]["transaction"], "data": map[string]any{"id": i + 1}})
			}
		}
		mu.Unlock()
	})
	c := dialTest(t, s, "", "")
	var wg sync.WaitGroup
	ids := make([]uint64, 2)
	for i := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m, err := c.Request(context.Background(), map[string]any{"janus": "attach", "plugin": pluginName, "n": i})
			if err != nil {
				t.Error(err)
				return
			}
			ids[i] = m.Data.ID
		}()
	}
	wg.Wait()
	// Each request got the reply carrying its own transaction.
	sent := s.sent()
	for i, f := range sent {
		n := int(f["n"].(float64))
		if ids[n] != uint64(i+1) {
			t.Errorf("request %d got id %d, want %d", n, ids[n], i+1)
		}
	}
}

func TestClient_PluginMessages(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		tx := f["transaction"]
		body, _ := f["body"].(map[string]any)
		switch body["request"] {
		case "create": // Synchronous: success with plugindata.
			s.send(map[string]any{"janus": "success", "transaction": tx, "sender": f["handle_id"],
				"plugindata": map[string]any{"plugin": pluginName, "data": map[string]any{"videoroom": "event", "error_code": 427, "error": "Room 1 already exists"}}})
		case "join": // Asynchronous: ack, then the event.
			s.send(map[string]any{"janus": "ack", "transaction": tx})
			s.send(map[string]any{"janus": "event", "transaction": tx, "sender": f["handle_id"],
				"plugindata": map[string]any{"plugin": pluginName, "data": map[string]any{"videoroom": "joined", "id": 5, "private_id": 6}}})
		case "publish":
			s.send(map[string]any{"janus": "ack", "transaction": tx})
			s.send(map[string]any{"janus": "event", "transaction": tx, "sender": f["handle_id"],
				"plugindata": map[string]any{"plugin": pluginName, "data": map[string]any{"videoroom": "event", "error_code": 432, "error": "Maximum number of active publishers (1) already reached"}}})
		case "configure": // Core error.
			s.send(map[string]any{"janus": "error", "transaction": tx, "error": map[string]any{"code": 458, "reason": "No such session 1"}})
		case "start":
			s.send(map[string]any{"janus": "ack", "transaction": tx})
			s.send(map[string]any{"janus": "event", "transaction": tx, "sender": f["handle_id"], "jsep": map[string]any{"type": "offer", "sdp": "v=0"},
				"plugindata": map[string]any{"plugin": pluginName, "data": map[string]any{"videoroom": "event", "started": "ok"}}})
		}
	})
	c := dialTest(t, s, "", "")
	ctx := context.Background()

	_, err := c.Message(ctx, 9, map[string]any{"request": "create", "room": 1}, nil)
	var je *janusError
	if !errors.As(err, &je) || je.Code != 427 || !strings.Contains(err.Error(), "create: 427 Room 1 already exists") {
		t.Errorf("create err = %v", err)
	}
	m, err := c.Message(ctx, 9, map[string]any{"request": "join"}, nil)
	if err != nil || m.Event().VideoRoom != "joined" || m.Event().ID != 5 {
		t.Errorf("join = %+v, %v", m, err)
	}
	_, err = c.Message(ctx, 9, map[string]any{"request": "publish"}, &JSEP{Type: "offer", SDP: "v=0"})
	if !errors.As(err, &je) || je.Code != 432 || !strings.HasPrefix(err.Error(), "publish: 432 ") {
		t.Errorf("publish err = %v", err)
	}
	_, err = c.Message(ctx, 9, map[string]any{"request": "configure"}, nil)
	if !errors.As(err, &je) || je.Code != 458 || err.Error() != "configure: 458 No such session 1" {
		t.Errorf("configure err = %v", err)
	}
	m, err = c.Message(ctx, 9, map[string]any{"request": "start"}, &JSEP{Type: "answer", SDP: "v=0 answer"})
	if err != nil || m.JSEP == nil || m.Event().Started != "ok" {
		t.Errorf("start = %+v, %v", m, err)
	}
	for _, f := range s.sent() {
		if f["janus"] != "message" || f["handle_id"].(float64) != 9 {
			t.Errorf("frame = %v", f)
		}
	}
	if j := s.sent()[2]["jsep"].(map[string]any); j["type"] != "offer" || j["sdp"] != "v=0" {
		t.Errorf("publish jsep = %v", j)
	}
}

func TestClient_Timeout(t *testing.T) {
	defer func(d time.Duration) { requestTimeout = d }(requestTimeout)
	requestTimeout = 100 * time.Millisecond
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		s.send(map[string]any{"janus": "ack", "transaction": f["transaction"]}) // Never the event
	})
	c := dialTest(t, s, "", "")
	_, err := c.Message(context.Background(), 1, map[string]any{"request": "join"}, nil)
	if err == nil || err.Error() != "join: timeout after 100ms" {
		t.Errorf("err = %v", err)
	}
}

func TestClient_UnsolicitedRouting(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {})
	c := dialTest(t, s, "", "")
	got := make(chan string, 4)
	c.OnHandle(21, func(m *Message) { got <- "pub:" + m.Janus })
	c.OnHandle(22, func(m *Message) { got <- "sub:" + m.Janus })
	c.OnSession(func(m *Message) { got <- "session:" + m.Janus })
	c.setSessionID(5) // As if create had run
	s.send(map[string]any{"janus": "event", "sender": 21, "plugindata": map[string]any{"plugin": pluginName, "data": map[string]any{"videoroom": "event", "unpublished": 3}}})
	s.send(map[string]any{"janus": "hangup", "sender": 22, "reason": "Close PC"})
	s.send(map[string]any{"janus": "event", "sender": 99}) // Unknown handle: dropped
	s.send(map[string]any{"janus": "timeout", "session_id": 5})
	for _, want := range []string{"pub:event", "sub:hangup", "session:timeout"} {
		select {
		case g := <-got:
			if g != want {
				t.Errorf("got %s, want %s", g, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("no %s", want)
		}
	}
}

func TestClient_SecretsOnEveryFrameAndKeepalive(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		s.send(map[string]any{"janus": "ack", "transaction": f["transaction"], "session_id": f["session_id"]})
	})
	c := dialTest(t, s, "as3cret", "tok3n")
	c.setSessionID(5)
	c.StartKeepalive(30 * time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	frames := s.sent()
	if len(frames) < 3 {
		t.Fatalf("%d keepalives in 200 ms at 30 ms", len(frames))
	}
	for _, f := range frames {
		if f["janus"] != "keepalive" || f["apisecret"] != "as3cret" || f["token"] != "tok3n" || f["session_id"].(float64) != 5 {
			t.Errorf("frame = %v", f)
		}
	}
	c.Close()
	n := len(s.sent())
	time.Sleep(100 * time.Millisecond)
	if len(s.sent()) != n {
		t.Error("keepalive went on after Close")
	}
}

func TestClient_CloseFailsPending(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {})
	c := dialTest(t, s, "", "")
	errc := make(chan error, 1)
	go func() {
		_, err := c.Message(context.Background(), 1, map[string]any{"request": "join"}, nil)
		errc <- err
	}()
	time.Sleep(50 * time.Millisecond)
	c.Close()
	if err := <-errc; err == nil || err.Error() != "join: connection closed" {
		t.Errorf("err = %v", err)
	}
	if _, err := c.Request(context.Background(), map[string]any{"janus": "keepalive"}); err == nil || err.Error() != "keepalive: connection closed" {
		t.Errorf("after close: %v", err)
	}
}

func TestClient_ServerCloses(t *testing.T) {
	s := newScriptServer(t, func(s *scriptServer, f map[string]any) {
		s.conn.Close(websocket.StatusGoingAway, "bye")
	})
	c := dialTest(t, s, "", "")
	if _, err := c.Request(context.Background(), map[string]any{"janus": "create"}); err == nil {
		t.Error("request succeeded")
	}
	select {
	case <-c.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("read loop did not end")
	}
	if c.Err() == nil {
		t.Error("no error after the server closed")
	}
}

func TestDial_RefusedHandshake(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()
	_, err := dialJanus(context.Background(), "ws"+strings.TrimPrefix(srv.URL, "http"), false)
	var de *dialError
	if !errors.As(err, &de) || de.Status != 403 || err.Error() != "handshake refused (HTTP 403: forbidden)" {
		t.Errorf("err = %v", err)
	}
}
