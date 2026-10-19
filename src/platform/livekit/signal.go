package livekit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"nhooyr.io/websocket"

	"webrtc-load-engine/src/platform"
)

// protocolVersion is the client protocol announced in the join URL: the
// pinned server (v1.13.10) answers it with the dual-PC model (008 T002).
const protocolVersion = "17"

// Keepalive defaults when the JoinResponse gives none (research R8).
const (
	defaultPingInterval = 5 * time.Second
	defaultPingTimeout  = 15 * time.Second
)

const (
	readLimit    = 8 << 20 // Join responses and offers grow with the room
	writeTimeout = 5 * time.Second
	leaveTimeout = 2 * time.Second
)

// errUnauthorized is a join the server refused for its token.
var errUnauthorized = errors.New("unauthorized")

// session is one bot's signaling connection (data-model "Bot Session"). Its
// read loop handles the server's messages in order; writes are serialized.
type session struct {
	peerID string
	h      platform.SignalingHandler
	logger *slog.Logger
	conn   *websocket.Conn

	ctx    context.Context // Ends when the session closes
	cancel context.CancelFunc
	done   chan struct{} // Closed when the read loop has returned

	writeMu   sync.Mutex
	lastHeard atomic.Int64 // Unix nanoseconds of the last frame read
	leaving   atomic.Bool  // Set by Disconnect: a close is not a loss
	ready     atomic.Bool  // Connect returned: losses go to OnSignalingLost
	lostOnce  sync.Once
	onLost    func(error) // After ready: forget the session, tell the peer

	// Join progress, read by Connect.
	joined      chan *joinResponse          // The JoinResponse
	published   chan trackPublishedResponse // TrackPublishedResponses
	pubAnswer   chan sessionDescription     // Answer to the publish offer
	subAnswered chan struct{}               // First subscriber offer answered
	failed      chan error                  // Fatal error before ready

	// Read loop only.
	subs        *subscriptions
	subStarted  bool            // First subscriber offer applied (OnOffer used)
	refusedLogs map[string]bool // Tracks whose refused subscription was logged
	pingEvery   time.Duration   // From the JoinResponse
	pingTimeout time.Duration
}

// dialURL is the join URL of research R1 (the token travels only here).
func dialURL(base, token string) string {
	q := url.Values{
		"access_token":   {token},
		"auto_subscribe": {"0"},
		"protocol":       {protocolVersion},
		"sdk":            {"go"},
		"version":        {"webrtc-load-engine"},
		"os":             {runtime.GOOS},
	}
	return base + "?" + q.Encode()
}

// redactURL hides the access token of a join URL, for logs.
func redactURL(u string) string {
	base, query, ok := strings.Cut(u, "?")
	if !ok {
		return u
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return base + "?[redacted]"
	}
	if q.Has("access_token") {
		q.Set("access_token", "[redacted]")
	}
	return base + "?" + q.Encode()
}

// dial opens the WebSocket. A refused token is errUnauthorized.
func dial(ctx context.Context, rawURL string) (*websocket.Conn, error) {
	conn, resp, err := websocket.Dial(ctx, rawURL, nil)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return nil, fmt.Errorf("%w (HTTP %d)", errUnauthorized, resp.StatusCode)
		}
		// The error text may quote the URL: never the token.
		return nil, errors.New(strings.ReplaceAll(err.Error(), rawURL, redactURL(rawURL)))
	}
	conn.SetReadLimit(readLimit)
	return conn, nil
}

func newSession(peerID string, h platform.SignalingHandler, logger *slog.Logger, conn *websocket.Conn) *session {
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{
		peerID:      peerID,
		h:           h,
		logger:      logger,
		conn:        conn,
		ctx:         ctx,
		cancel:      cancel,
		done:        make(chan struct{}),
		joined:      make(chan *joinResponse, 1),
		published:   make(chan trackPublishedResponse, 8),
		pubAnswer:   make(chan sessionDescription, 1),
		subAnswered: make(chan struct{}),
		failed:      make(chan error, 1),
		refusedLogs: map[string]bool{},
		pingEvery:   defaultPingInterval,
		pingTimeout: defaultPingTimeout,
	}
	s.lastHeard.Store(time.Now().UnixNano())
	return s
}

// send writes one request.
func (s *session) send(r signalRequest) error {
	b, err := encodeSignalRequest(r)
	if err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	ctx, cancel := context.WithTimeout(s.ctx, writeTimeout)
	defer cancel()
	return s.conn.Write(ctx, websocket.MessageBinary, b)
}

// lost ends the session on a signaling failure: before Connect returns it
// fails the join, after it is reported once to the peer. A loss during
// Disconnect is not one.
func (s *session) lost(err error) {
	if s.leaving.Load() {
		return
	}
	s.lostOnce.Do(func() {
		if !s.ready.Load() {
			select {
			case s.failed <- err:
			default:
			}
			return
		}
		s.logger.Warn("LiveKit signaling lost", slog.String("error", err.Error()))
		s.close()
		if s.onLost != nil {
			s.onLost(err)
		}
	})
}

// close stops the session's goroutines and closes the connection.
func (s *session) close() {
	s.cancel()
	s.conn.Close(websocket.StatusNormalClosure, "")
}

// leave sends a LeaveRequest and closes (Disconnect).
func (s *session) leave() {
	s.leaving.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), leaveTimeout)
	defer cancel()
	if b, err := encodeSignalRequest(signalRequest{Leave: &leaveRequest{Reason: reasonClientInitiated}}); err == nil {
		s.writeMu.Lock()
		_ = s.conn.Write(ctx, websocket.MessageBinary, b)
		s.writeMu.Unlock()
	}
	s.close()
	select {
	case <-s.done:
	case <-ctx.Done():
	}
}

// readLoop reads and handles the server's messages until the connection
// closes.
func (s *session) readLoop() {
	defer close(s.done)
	for {
		typ, data, err := s.conn.Read(s.ctx)
		if err != nil {
			if s.ctx.Err() == nil {
				s.lost(fmt.Errorf("signaling connection closed: %w", err))
			}
			return
		}
		s.lastHeard.Store(time.Now().UnixNano())
		if typ != websocket.MessageBinary {
			continue
		}
		r, err := decodeSignalResponse(data)
		if err != nil {
			s.logger.Warn("Undecodable LiveKit message", slog.String("error", err.Error()))
			continue
		}
		s.handle(r)
	}
}

// handle dispatches one server message (contracts/signaling-messages.md).
func (s *session) handle(r signalResponse) {
	switch {
	case r.Join != nil:
		j := r.Join
		if j.PingInterval > 0 {
			s.pingEvery = time.Duration(j.PingInterval) * time.Second
		}
		if j.PingTimeout > 0 {
			s.pingTimeout = time.Duration(j.PingTimeout) * time.Second
		}
		s.subs = newSubscriptions(s.h.ReceiveLimit(), j.Participant.SID)
		select {
		case s.joined <- j:
		default:
		}
		s.subscribe(s.subs.participants(j.OtherParticipants))
	case r.Offer != nil:
		s.onSubscriberOffer(*r.Offer)
	case r.Answer != nil:
		select {
		case s.pubAnswer <- *r.Answer:
		default:
			s.logger.Warn("Unexpected LiveKit publish answer")
		}
	case r.Trickle != nil:
		s.onTrickle(*r.Trickle)
	case r.Update != nil:
		if s.subs != nil {
			s.subscribe(s.subs.participants(r.Update.Participants))
		}
	case r.TrackPublished != nil:
		select {
		case s.published <- *r.TrackPublished:
		default:
		}
	case r.Leave != nil:
		s.lost(fmt.Errorf("server sent leave (reason %d)", r.Leave.Reason))
	case r.SubscriptionResponse != nil:
		sid := r.SubscriptionResponse.TrackSID
		if !s.refusedLogs[sid] {
			s.refusedLogs[sid] = true
			s.logger.Warn("LiveKit refused a subscription", slog.String("track_sid", sid), slog.Int("error", int(r.SubscriptionResponse.Err)))
		}
		if s.subs != nil {
			s.subscribe(s.subs.drop(sid))
		}
	}
}

// subscribe asks for the given tracks.
func (s *session) subscribe(sids []string) {
	if len(sids) == 0 {
		return
	}
	if err := s.send(signalRequest{Subscription: &updateSubscription{TrackSIDs: sids, Subscribe: true}}); err != nil {
		s.logger.Warn("Failed to send LiveKit subscription", slog.String("error", err.Error()))
		return
	}
	s.logger.Debug("Subscribed to LiveKit tracks", slog.Any("track_sids", sids))
}

// onSubscriberOffer answers the server's offer for the session it sends
// on: the first with OnOffer, the next ones with Renegotiate (research R6).
func (s *session) onSubscriberOffer(offer sessionDescription) {
	var answer string
	var err error
	first := !s.subStarted
	if first {
		answer, err = s.h.OnOffer(offer.SDP, s.trickle(targetSubscriber))
	} else {
		answer, err = s.h.Renegotiate(offer.SDP)
	}
	if err != nil {
		if first {
			s.lost(fmt.Errorf("subscriber offer: %w", err))
		} else {
			s.logger.Warn("Failed to apply LiveKit subscriber offer", slog.String("error", err.Error()))
		}
		return
	}
	s.subStarted = true
	if err := s.send(signalRequest{Answer: &sessionDescription{Type: "answer", SDP: answer, ID: offer.ID}}); err != nil {
		s.lost(fmt.Errorf("send subscriber answer: %w", err))
		return
	}
	if first {
		close(s.subAnswered)
	}
}

// candidateInit is the JSON of an RTCIceCandidateInit, as LiveKit carries
// candidates.
type candidateInit struct {
	Candidate string  `json:"candidate"`
	SDPMid    *string `json:"sdpMid,omitempty"`
}

// trickle returns the TrickleFunc of the session with target.
func (s *session) trickle(target int32) platform.TrickleFunc {
	return func(mid, candidate string) {
		c := candidateInit{Candidate: candidate}
		if mid != "" {
			c.SDPMid = &mid
		}
		b, err := json.Marshal(c)
		if err != nil {
			return
		}
		if err := s.send(signalRequest{Trickle: &trickleRequest{CandidateInit: string(b), Target: target}}); err != nil && s.ctx.Err() == nil {
			s.logger.Debug("Failed to trickle a LiveKit candidate", slog.String("error", err.Error()))
		}
	}
}

// onTrickle applies a server candidate to its session.
func (s *session) onTrickle(t trickleRequest) {
	var c candidateInit
	if err := json.Unmarshal([]byte(t.CandidateInit), &c); err != nil || c.Candidate == "" {
		return // End of candidates, or nothing usable
	}
	mid := ""
	if c.SDPMid != nil {
		mid = *c.SDPMid
	}
	var err error
	if t.Target == targetPublisher {
		err = s.h.OnPublishICECandidate(mid, c.Candidate)
	} else {
		err = s.h.OnICECandidate(mid, c.Candidate)
	}
	if err != nil {
		s.logger.Debug("Failed to apply a LiveKit candidate", slog.String("error", err.Error()))
	}
}

// keepalive pings every pingEvery and reports the session lost when the
// server has been silent for pingTimeout (research R8).
func (s *session) keepalive(every, timeout time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case now := <-ticker.C:
			if silent := now.Sub(time.Unix(0, s.lastHeard.Load())); silent > timeout {
				s.lost(fmt.Errorf("no message from the server for %s", silent.Round(time.Millisecond)))
				return
			}
			if err := s.send(signalRequest{PingReq: &ping{Timestamp: now.UnixMilli()}}); err != nil && s.ctx.Err() == nil {
				s.logger.Debug("Failed to send LiveKit ping", slog.String("error", err.Error()))
			}
		}
	}
}
