package mediasoup

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
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// requestTimeout bounds a protoo request (a variable for tests).
var requestTimeout = 15 * time.Second

const (
	readLimit    = 8 << 20 // Router capabilities and consumer parameters stay small; be generous
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

// dialProtoo opens the protoo WebSocket of research R1: subprotocol protoo,
// the Origin header the demo checks. A refused handshake is a *dialError;
// no error quotes the URL's query (it may carry the unlimited-room secret).
func dialProtoo(ctx context.Context, rawURL, origin string, insecureTLS bool) (*websocket.Conn, error) {
	opts := &websocket.DialOptions{
		Subprotocols: []string{"protoo"},
		HTTPHeader:   http.Header{"Origin": {origin}},
	}
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
		return nil, errors.New(strings.ReplaceAll(err.Error(), rawURL, redactQuery(rawURL)))
	}
	conn.SetReadLimit(readLimit)
	return conn, nil
}

// redactQuery drops a URL's query, for logs and errors.
func redactQuery(u string) string {
	base, _, _ := strings.Cut(u, "?")
	return base
}

// requestError is a protoo error response.
type requestError struct {
	Code   int
	Reason string
}

func (e *requestError) Error() string { return fmt.Sprintf("%d %s", e.Code, e.Reason) }

// replyFunc answers a server request: data on success, or an error. It is
// called exactly once, from any goroutine.
type replyFunc func(data any, err *requestError)

// frame is any protoo message.
type frame struct {
	Request      bool            `json:"request,omitempty"`
	Response     bool            `json:"response,omitempty"`
	Notification bool            `json:"notification,omitempty"`
	ID           *int64          `json:"id,omitempty"`
	Method       string          `json:"method,omitempty"`
	OK           *bool           `json:"ok,omitempty"`
	Data         json.RawMessage `json:"data,omitempty"`
	ErrorCode    int             `json:"errorCode,omitempty"`
	ErrorReason  string          `json:"errorReason,omitempty"`
}

// protooConn is a protoo client over one WebSocket
// (contracts/signaling-messages.md "Framing"). Server requests and
// notifications are handed to onRequest and onNotification from the read
// loop, which they must not block.
type protooConn struct {
	conn   *websocket.Conn
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{} // Closed when the read loop has returned

	onRequest      func(method string, data json.RawMessage, reply replyFunc)
	onNotification func(method string, data json.RawMessage)

	writeMu sync.Mutex
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan frame
	closed  bool
	err     error // Why the read loop ended
}

func newProtooConn(conn *websocket.Conn, logger *slog.Logger) *protooConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &protooConn{
		conn: conn, logger: logger, ctx: ctx, cancel: cancel, done: make(chan struct{}),
		nextID:  rand.Int64N(1_000_000) + 1,
		pending: map[int64]chan frame{},
	}
}

func (p *protooConn) write(f frame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(p.ctx, writeTimeout)
	defer cancel()
	return p.conn.Write(ctx, websocket.MessageText, b)
}

func marshalData(data any) (json.RawMessage, error) {
	if data == nil {
		return json.RawMessage(`{}`), nil
	}
	return json.Marshal(data)
}

// Request sends a request and decodes the response's data into out (if
// not nil). An error response, a timeout and a closed connection are
// errors naming the method.
func (p *protooConn) Request(ctx context.Context, method string, data, out any) error {
	raw, err := marshalData(data)
	if err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	ch := make(chan frame, 1)
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return fmt.Errorf("%s: connection closed", method)
	}
	id := p.nextID
	p.nextID++
	p.pending[id] = ch
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
	}()

	if err := p.write(frame{Request: true, ID: &id, Method: method, Data: raw}); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	timer := time.NewTimer(requestTimeout)
	defer timer.Stop()
	select {
	case resp, ok := <-ch:
		if !ok {
			return fmt.Errorf("%s: connection closed", method)
		}
		if resp.OK == nil || !*resp.OK {
			return fmt.Errorf("%s: %w", method, &requestError{Code: resp.ErrorCode, Reason: resp.ErrorReason})
		}
		if out != nil && len(resp.Data) > 0 {
			if err := json.Unmarshal(resp.Data, out); err != nil {
				return fmt.Errorf("%s: response: %w", method, err)
			}
		}
		return nil
	case <-timer.C:
		return fmt.Errorf("%s: timeout after %s", method, requestTimeout)
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	}
}

// Notify sends a notification.
func (p *protooConn) Notify(method string, data any) error {
	raw, err := marshalData(data)
	if err != nil {
		return err
	}
	return p.write(frame{Notification: true, Method: method, Data: raw})
}

// reply sends the response to the server request id.
func (p *protooConn) reply(id int64, method string, data any, rerr *requestError) {
	f := frame{Response: true, ID: &id}
	if rerr != nil {
		ok := false
		f.OK, f.ErrorCode, f.ErrorReason = &ok, rerr.Code, rerr.Reason
	} else {
		raw, err := marshalData(data)
		if err != nil {
			raw = json.RawMessage(`{}`)
		}
		ok := true
		f.OK, f.Data = &ok, raw
	}
	if err := p.write(f); err != nil && p.ctx.Err() == nil {
		p.logger.Debug("Failed to answer a mediasoup request", slog.String("method", method), slog.String("error", err.Error()))
	}
}

// readLoop reads and dispatches frames until the connection closes; then
// it fails the pending requests and closes Done.
func (p *protooConn) readLoop() {
	defer close(p.done)
	for {
		_, b, err := p.conn.Read(p.ctx)
		if err != nil {
			p.shutdown(err)
			return
		}
		var f frame
		if err := json.Unmarshal(b, &f); err != nil {
			p.logger.Warn("Undecodable mediasoup message", slog.String("error", err.Error()))
			continue
		}
		switch {
		case f.Response && f.ID != nil:
			p.mu.Lock()
			ch := p.pending[*f.ID]
			p.mu.Unlock()
			if ch != nil {
				ch <- f
			}
		case f.Request && f.ID != nil:
			id, method := *f.ID, f.Method
			var once sync.Once
			reply := func(data any, rerr *requestError) {
				once.Do(func() { p.reply(id, method, data, rerr) })
			}
			if p.onRequest == nil {
				reply(nil, &requestError{Code: 405, Reason: "unsupported request"})
				continue
			}
			p.onRequest(method, f.Data, reply)
		case f.Notification:
			if p.onNotification != nil {
				p.onNotification(f.Method, f.Data)
			}
		}
	}
}

// shutdown records why the connection ended and fails pending requests.
func (p *protooConn) shutdown(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed = true
	p.err = err
	for id, ch := range p.pending {
		close(ch)
		delete(p.pending, id)
	}
}

// Close closes the connection; pending requests fail.
func (p *protooConn) Close() {
	p.cancel()
	p.conn.Close(websocket.StatusNormalClosure, "")
	p.shutdown(errors.New("closed"))
}

// Done is closed when the read loop has returned.
func (p *protooConn) Done() <-chan struct{} { return p.done }

// Err is why the read loop ended (after Done).
func (p *protooConn) Err() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}
