package janus

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// requestTimeout bounds a Janus request, ack and event included (a
// variable for tests).
var requestTimeout = 15 * time.Second

const (
	pluginName   = "janus.plugin.videoroom"
	subprotocol  = "janus-protocol"
	readLimit    = 8 << 20 // SDP offers grow with the subscription; be generous
	writeTimeout = 5 * time.Second
)

// dialError is a WebSocket handshake the server refused.
type dialError struct {
	Status int
	Body   string
}

func (e *dialError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("handshake refused (HTTP %d)", e.Status)
	}
	return fmt.Sprintf("handshake refused (HTTP %d: %s)", e.Status, e.Body)
}

// dialJanus opens the Janus WebSocket of research R1, with subprotocol
// janus-protocol. A refused handshake is a *dialError.
func dialJanus(ctx context.Context, rawURL string, insecureTLS bool) (*websocket.Conn, error) {
	opts := &websocket.DialOptions{Subprotocols: []string{subprotocol}}
	if insecureTLS {
		opts.HTTPClient = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}
	}
	conn, resp, err := websocket.Dial(ctx, rawURL, opts)
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			de := &dialError{Status: resp.StatusCode}
			if resp.Body != nil {
				b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
				de.Body = strings.TrimSpace(string(b))
			}
			return nil, de
		}
		return nil, err
	}
	conn.SetReadLimit(readLimit)
	return conn, nil
}

// janusError is a core error (janus: error) or a VideoRoom error
// (error_code) answering a request.
type janusError struct {
	Code   int
	Reason string
}

func (e *janusError) Error() string { return fmt.Sprintf("%d %s", e.Code, e.Reason) }

// errorCode returns the Janus code of err, or 0.
func errorCode(err error) int {
	var je *janusError
	if errors.As(err, &je) {
		return je.Code
	}
	return 0
}

// janusConn is a Janus client over one WebSocket, for one session
// (research R1). Replies are matched by transaction; other messages are
// handed, from the read loop, to the handler of their sender handle, or to
// the session handler (timeout). Handlers must not block.
type janusConn struct {
	conn      *websocket.Conn
	logger    *slog.Logger
	apiSecret string
	token     string
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{} // Closed when the read loop has returned

	writeMu   sync.Mutex
	mu        sync.Mutex
	sessionID uint64
	pending   map[string]chan *Message
	handlers  map[uint64]func(*Message)
	onSession func(*Message)
	closed    bool
	err       error // Why the read loop ended
}

func newJanusConn(conn *websocket.Conn, logger *slog.Logger, apiSecret, token string) *janusConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &janusConn{
		conn: conn, logger: logger, apiSecret: apiSecret, token: token,
		ctx: ctx, cancel: cancel, done: make(chan struct{}),
		pending: map[string]chan *Message{}, handlers: map[uint64]func(*Message){},
	}
}

// SessionID is the Janus session, once CreateSession succeeded.
func (c *janusConn) SessionID() uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionID
}

func (c *janusConn) setSessionID(id uint64) {
	c.mu.Lock()
	c.sessionID = id
	c.mu.Unlock()
}

// OnHandle routes the unsolicited messages of a handle to h.
func (c *janusConn) OnHandle(handle uint64, h func(*Message)) {
	c.mu.Lock()
	c.handlers[handle] = h
	c.mu.Unlock()
}

// OnSession routes the unsolicited messages without a known handle
// (timeout) to h.
func (c *janusConn) OnSession(h func(*Message)) {
	c.mu.Lock()
	c.onSession = h
	c.mu.Unlock()
}

// CreateSession creates the connection's Janus session.
func (c *janusConn) CreateSession(ctx context.Context) error {
	m, err := c.Request(ctx, map[string]any{"janus": "create"})
	if err != nil {
		return err
	}
	if m.Data == nil || m.Data.ID == 0 {
		return errors.New("create: no session ID in the reply")
	}
	c.setSessionID(m.Data.ID)
	return nil
}

// Attach attaches a VideoRoom handle to the session.
func (c *janusConn) Attach(ctx context.Context) (uint64, error) {
	m, err := c.Request(ctx, map[string]any{"janus": "attach", "plugin": pluginName})
	if err != nil {
		return 0, err
	}
	if m.Data == nil || m.Data.ID == 0 {
		return 0, errors.New("attach: no handle ID in the reply")
	}
	return m.Data.ID, nil
}

// Message sends a VideoRoom request on a handle, with an optional jsep,
// and returns the plugin's reply: the success of a synchronous request, or
// the event that follows the ack of an asynchronous one. A VideoRoom
// error_code is an error naming the request.
func (c *janusConn) Message(ctx context.Context, handle uint64, body map[string]any, jsep *JSEP) (*Message, error) {
	req := map[string]any{"janus": "message", "handle_id": handle, "body": body}
	if jsep != nil {
		req["jsep"] = jsep
	}
	m, err := c.Request(ctx, req)
	if err != nil {
		return nil, err
	}
	if e := m.Event(); e != nil && e.ErrorCode != 0 {
		return m, fmt.Errorf("%s: %w", requestName(req), &janusError{Code: e.ErrorCode, Reason: e.Error})
	}
	return m, nil
}

// requestName names a request in errors: the VideoRoom request, or the
// core verb.
func requestName(req map[string]any) string {
	if body, ok := req["body"].(map[string]any); ok {
		if r, ok := body["request"].(string); ok {
			return r
		}
	}
	s, _ := req["janus"].(string)
	return s
}

// Request sends a core request (a map with "janus" and its fields) and
// waits for its reply: success or error, or, for "message", the event
// after the ack. Transaction, session and secrets are added here.
func (c *janusConn) Request(ctx context.Context, req map[string]any) (*Message, error) {
	name := requestName(req)
	tx := strconv.FormatUint(rand.Uint64(), 36)
	ch := make(chan *Message, 2) // ack, then the event
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("%s: connection closed", name)
	}
	c.pending[tx] = ch
	if c.sessionID != 0 && req["janus"] != "create" {
		req["session_id"] = c.sessionID
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, tx)
		c.mu.Unlock()
	}()
	req["transaction"] = tx
	if c.apiSecret != "" {
		req["apisecret"] = c.apiSecret
	}
	if c.token != "" {
		req["token"] = c.token
	}
	if err := c.write(req); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	timer := time.NewTimer(requestTimeout)
	defer timer.Stop()
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return nil, fmt.Errorf("%s: connection closed", name)
			}
			switch {
			case m.Janus == "error":
				je := &janusError{}
				if m.Error != nil {
					je.Code, je.Reason = m.Error.Code, m.Error.Reason
				}
				return nil, fmt.Errorf("%s: %w", name, je)
			case m.Janus == "ack" && req["janus"] == "message":
				continue // The event follows
			}
			return m, nil
		case <-timer.C:
			return nil, fmt.Errorf("%s: timeout after %s", name, requestTimeout)
		case <-ctx.Done():
			return nil, fmt.Errorf("%s: %w", name, ctx.Err())
		}
	}
}

func (c *janusConn) write(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(c.ctx, writeTimeout)
	defer cancel()
	return c.conn.Write(ctx, websocket.MessageText, b)
}

// StartKeepalive sends a keepalive every interval until the connection
// closes (research R7: Janus drops a session silent for 60 s).
func (c *janusConn) StartKeepalive(interval time.Duration) {
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-c.ctx.Done():
				return
			case <-t.C:
			}
			if _, err := c.Request(c.ctx, map[string]any{"janus": "keepalive"}); err != nil && c.ctx.Err() == nil {
				c.logger.Debug("Janus keepalive failed", slog.String("error", err.Error()))
			}
		}
	}()
}

// readLoop reads and dispatches messages until the connection closes;
// then it fails the pending requests and closes Done.
func (c *janusConn) readLoop() {
	defer close(c.done)
	for {
		_, b, err := c.conn.Read(c.ctx)
		if err != nil {
			c.shutdown(err)
			return
		}
		var m Message
		if err := json.Unmarshal(b, &m); err != nil {
			c.logger.Warn("Undecodable Janus message", slog.String("error", err.Error()))
			continue
		}
		c.mu.Lock()
		ch := c.pending[m.Transaction]
		h := c.handlers[m.Sender]
		if m.Sender == 0 {
			h = c.onSession
		}
		c.mu.Unlock()
		switch {
		case m.Transaction != "" && ch != nil:
			select {
			case ch <- &m:
			default: // A third reply to one request: not one Janus sends
			}
		case h != nil:
			h(&m)
		default:
			c.logger.Debug("Unrouted Janus message", slog.String("janus", m.Janus), slog.Uint64("sender", m.Sender))
		}
	}
}

// shutdown records why the connection ended and fails pending requests.
func (c *janusConn) shutdown(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	c.err = err
	for tx, ch := range c.pending {
		close(ch)
		delete(c.pending, tx)
	}
}

// Close closes the connection; pending requests fail and the keepalive
// stops.
func (c *janusConn) Close() {
	c.cancel()
	c.conn.Close(websocket.StatusNormalClosure, "")
	c.shutdown(errors.New("closed"))
}

// Done is closed when the read loop has returned.
func (c *janusConn) Done() <-chan struct{} { return c.done }

// Err is why the read loop ended (after Done).
func (c *janusConn) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}
