package mediasoup

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// protooPeer is the server side of a test connection: it reads frames and
// lets the test answer or send.
type protooPeer struct {
	conn   *websocket.Conn
	frames chan map[string]any
}

func (p *protooPeer) send(t *testing.T, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := p.conn.Write(context.Background(), websocket.MessageText, b); err != nil {
		t.Errorf("server write: %v", err)
	}
}

func (p *protooPeer) next(t *testing.T) map[string]any {
	t.Helper()
	select {
	case f := <-p.frames:
		return f
	case <-time.After(5 * time.Second):
		t.Fatal("no frame from the client")
		return nil
	}
}

// protooPair starts a protoo-speaking test server and dials it.
func protooPair(t *testing.T) (*protooConn, *protooPeer) {
	t.Helper()
	peers := make(chan *protooPeer, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"protoo"}, InsecureSkipVerify: true})
		if err != nil {
			return
		}
		if c.Subprotocol() != "protoo" {
			c.Close(websocket.StatusPolicyViolation, "protoo subprotocol required")
			return
		}
		p := &protooPeer{conn: c, frames: make(chan map[string]any, 16)}
		peers <- p
		for {
			_, b, err := c.Read(context.Background())
			if err != nil {
				close(p.frames)
				return
			}
			var f map[string]any
			json.Unmarshal(b, &f)
			p.frames <- f
		}
	}))
	t.Cleanup(srv.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ws, err := dialProtoo(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/?roomId=r&peerId=p", "http://localhost", false)
	if err != nil {
		t.Fatal(err)
	}
	pc := newProtooConn(ws, quietLogger)
	t.Cleanup(pc.Close)
	return pc, <-peers
}

func TestProtoo_RequestsAnsweredOutOfOrder(t *testing.T) {
	pc, srv := protooPair(t)
	go pc.readLoop()
	var wg sync.WaitGroup
	results := make([]string, 2)
	for i, method := range []string{"first", "second"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out struct{ Name string }
			if err := pc.Request(context.Background(), method, map[string]any{"i": i}, &out); err != nil {
				t.Errorf("%s: %v", method, err)
			}
			results[i] = out.Name
		}()
	}
	a, b := srv.next(t), srv.next(t)
	if a["request"] != true || a["id"] == b["id"] {
		t.Fatalf("frames %v %v", a, b)
	}
	for _, f := range []map[string]any{b, a} {
		srv.send(t, map[string]any{"response": true, "id": f["id"], "ok": true, "data": map[string]any{"Name": f["method"]}})
	}
	wg.Wait()
	if results[0] != "first" || results[1] != "second" {
		t.Errorf("results = %v", results)
	}
}

func TestProtoo_ErrorResponse(t *testing.T) {
	pc, srv := protooPair(t)
	go pc.readLoop()
	errc := make(chan error, 1)
	go func() { errc <- pc.Request(context.Background(), "join", nil, nil) }()
	f := srv.next(t)
	// The server omits "ok" on errors (010 T002).
	srv.send(t, map[string]any{"response": true, "id": f["id"], "errorCode": 500, "errorReason": "Peer already joined"})
	err := <-errc
	if err == nil || !strings.Contains(err.Error(), "join") || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "Peer already joined") {
		t.Errorf("err = %v", err)
	}
}

func TestProtoo_Timeout(t *testing.T) {
	old := requestTimeout
	requestTimeout = 100 * time.Millisecond
	t.Cleanup(func() { requestTimeout = old })
	pc, srv := protooPair(t)
	go pc.readLoop()
	err := pc.Request(context.Background(), "slow", nil, nil)
	srv.next(t)
	if err == nil || !strings.Contains(err.Error(), "slow") || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("err = %v", err)
	}
}

func TestProtoo_ServerRequests(t *testing.T) {
	pc, srv := protooPair(t)
	pc.onRequest = func(method string, data json.RawMessage, reply replyFunc) {
		switch method {
		case "newConsumer":
			go reply(nil, nil) // Answered later, from another goroutine
		case "newDataConsumer":
			reply(nil, &requestError{Code: 403, Reason: "data channels not used"})
		default:
			reply(nil, &requestError{Code: 405, Reason: "unsupported"})
		}
	}
	notes := make(chan string, 1)
	pc.onNotification = func(method string, data json.RawMessage) { notes <- method + string(data) }
	go pc.readLoop()

	srv.send(t, map[string]any{"request": true, "id": 7, "method": "newConsumer", "data": map[string]any{}})
	if f := srv.next(t); f["response"] != true || f["id"] != float64(7) || f["ok"] != true {
		t.Errorf("accept = %v", f)
	}
	srv.send(t, map[string]any{"request": true, "id": 8, "method": "newDataConsumer", "data": map[string]any{}})
	if f := srv.next(t); f["ok"] != false || f["errorCode"] != float64(403) || f["errorReason"] != "data channels not used" {
		t.Errorf("decline = %v", f)
	}
	srv.send(t, map[string]any{"request": true, "id": 9, "method": "other", "data": map[string]any{}})
	if f := srv.next(t); f["errorCode"] != float64(405) {
		t.Errorf("unknown = %v", f)
	}
	srv.send(t, map[string]any{"notification": true, "method": "consumerClosed", "data": map[string]any{"consumerId": "c1"}})
	select {
	case n := <-notes:
		if n != `consumerClosed{"consumerId":"c1"}` {
			t.Errorf("notification = %s", n)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("notification not dispatched")
	}
	if err := pc.Notify("resumeConsumer", consumerRef{ConsumerID: "c2"}); err != nil {
		t.Fatal(err)
	}
	if f := srv.next(t); f["notification"] != true || f["method"] != "resumeConsumer" || f["data"].(map[string]any)["consumerId"] != "c2" {
		t.Errorf("notify = %v", f)
	}
}

func TestProtoo_NoHandlerAnswers405(t *testing.T) {
	pc, srv := protooPair(t)
	go pc.readLoop()
	srv.send(t, map[string]any{"request": true, "id": 1, "method": "newConsumer", "data": map[string]any{}})
	if f := srv.next(t); f["errorCode"] != float64(405) {
		t.Errorf("answer = %v", f)
	}
}

func TestProtoo_ClosePendingAndDone(t *testing.T) {
	pc, srv := protooPair(t)
	go pc.readLoop()
	errc := make(chan error, 1)
	go func() { errc <- pc.Request(context.Background(), "join", nil, nil) }()
	srv.next(t)
	srv.conn.Close(websocket.StatusGoingAway, "bye")
	select {
	case err := <-errc:
		if err == nil || !strings.Contains(err.Error(), "connection closed") {
			t.Errorf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pending request not failed")
	}
	select {
	case <-pc.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Done not closed")
	}
	if pc.Err() == nil {
		t.Error("Err() = nil after a remote close")
	}
	if err := pc.Request(context.Background(), "late", nil, nil); err == nil {
		t.Error("request after close succeeded")
	}
}

func TestProtoo_DialStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "http://good" || !strings.Contains(r.Header.Get("Sec-WebSocket-Protocol"), "protoo") {
			http.Error(w, "GO AWAY", http.StatusForbidden)
			return
		}
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"protoo"}, InsecureSkipVerify: true})
		if err == nil {
			c.Close(websocket.StatusNormalClosure, "")
		}
	}))
	defer srv.Close()
	u := "ws" + strings.TrimPrefix(srv.URL, "http") + "/?roomId=r&peerId=p&unlimitedRoomSecret=s3cret"
	_, err := dialProtoo(context.Background(), u, "http://bad", false)
	var de *dialError
	if !errors.As(err, &de) || de.Status != http.StatusForbidden {
		t.Fatalf("err = %v, want a dialError with 403", err)
	}
	if strings.Contains(err.Error(), "s3cret") {
		t.Errorf("error quotes the secret: %v", err)
	}
	ws, err := dialProtoo(context.Background(), u, "http://good", false)
	if err != nil {
		t.Fatal(err)
	}
	ws.Close(websocket.StatusNormalClosure, "")
}
