package janus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"webrtc-load-engine/src/platform"
)

// connectTimeout bounds a bot's join (a variable for tests).
var connectTimeout = 20 * time.Second

const (
	// defaultKeepalive keeps a session alive: Janus drops one silent for
	// 60 s (research R7).
	defaultKeepalive = 25 * time.Second
	// disconnectTimeout bounds destroy and the wait for the bot's worker
	// in Disconnect.
	disconnectTimeout = 2 * time.Second
)

// Adapter joins bots to a Janus VideoRoom (research R1–R7). It implements
// platform.PlatformAdapter and platform.PublishSessionAdapter.
type Adapter struct {
	logger    *slog.Logger
	cfg       Config
	keepalive time.Duration // Tests lower it

	mu       sync.Mutex
	sessions map[string]*session // By peer ID
}

var (
	_ platform.PlatformAdapter       = (*Adapter)(nil)
	_ platform.PublishSessionAdapter = (*Adapter)(nil)
)

// NewAdapter returns an adapter for the Janus server and room of cfg;
// cfg.Participants sizes the room the bots create.
func NewAdapter(logger *slog.Logger, cfg Config) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{logger: logger, cfg: cfg, keepalive: defaultKeepalive, sessions: map[string]*session{}}
}

// Config returns the adapter's config. It holds the secrets; its String
// and LogValue hide them.
func (a *Adapter) Config() Config { return a.cfg }

// PublishSession implements platform.PublishSessionAdapter: Janus receives
// a bot's media on its publisher handle's PeerConnection, the session the
// bot offers.
func (a *Adapter) PublishSession() bool { return true }

// session is one bot's Janus session and receive state (data-model "Bot
// session"). Everything that touches the bot's subscription runs on its
// worker goroutine, in order.
type session struct {
	a      *Adapter
	peerID string
	h      platform.SignalingHandler
	logger *slog.Logger
	conn   *janusConn

	queueMu sync.Mutex
	queue   []func()
	wake    chan struct{} // Signals queued work
	stop    chan struct{} // Closed by close
	stopped sync.Once
	worked  chan struct{} // Closed when the worker has returned

	leaving  atomic.Bool // Set by Disconnect: a close is not a loss
	ready    atomic.Bool // Connect returned: losses go to OnSignalingLost
	lostOnce sync.Once
	failed   chan error  // Fatal error before ready
	onLost   func(error) // After ready: forget the session, tell the peer

	pub, sub      uint64 // Publisher and subscriber handles
	participantID uint64

	// Worker only.
	cap        *receiveCap
	dummyMID   string      // The dummy's section in the subscription
	subReady   bool        // The subscription's first offer is answered
	offerOut   bool        // Janus queued an update: its offer is coming
	subscribed []streamKey // What the subscription carries, dummy aside
}

func newSession(a *Adapter, peerID string, h platform.SignalingHandler, logger *slog.Logger, conn *janusConn) *session {
	return &session{
		a: a, peerID: peerID, h: h, logger: logger, conn: conn,
		wake: make(chan struct{}, 1), stop: make(chan struct{}), worked: make(chan struct{}),
		failed: make(chan error, 1),
	}
}

// enqueue adds work for the worker.
func (s *session) enqueue(f func()) {
	s.queueMu.Lock()
	s.queue = append(s.queue, f)
	s.queueMu.Unlock()
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// worker runs queued work in order until the session stops.
func (s *session) worker() {
	defer close(s.worked)
	for {
		s.queueMu.Lock()
		var f func()
		if len(s.queue) > 0 {
			f = s.queue[0]
			s.queue = s.queue[1:]
		}
		s.queueMu.Unlock()
		if f == nil {
			select {
			case <-s.wake:
				continue
			case <-s.stop:
				return
			}
		}
		select {
		case <-s.stop:
			return
		default:
		}
		f()
	}
}

// do runs f on the worker and waits for its result.
func (s *session) do(ctx context.Context, f func() error) error {
	res := make(chan error, 1)
	s.enqueue(func() { res <- f() })
	select {
	case err := <-res:
		return err
	case err := <-s.failed:
		return err
	case <-s.stop:
		return errors.New("session closed")
	case <-ctx.Done():
		return ctx.Err()
	}
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
		s.logger.Warn("Janus signaling lost", slog.String("error", err.Error()))
		s.close()
		if s.onLost != nil {
			s.onLost(err)
		}
	})
}

// watch reports the connection's end as a loss.
func (s *session) watch() {
	<-s.conn.Done()
	select {
	case <-s.stop:
		return // Closed by the adapter
	default:
	}
	err := s.conn.Err()
	if err == nil {
		err = errors.New("closed")
	}
	s.lost(fmt.Errorf("signaling connection closed: %w", err))
}

// close stops the worker and closes the connection.
func (s *session) close() {
	s.stopped.Do(func() { close(s.stop) })
	s.conn.Close()
}

// Connect joins peerID to the room and blocks until it can send and
// receive (FR-003): the join sequence of research R4. On error, everything
// is closed.
func (a *Adapter) Connect(ctx context.Context, peerID string, h platform.SignalingHandler) error {
	logger := a.logger.With(slog.String("peer_id", peerID))
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	logger.Info("Janus adapter: connecting peer", slog.Any("config", a.cfg))
	ws, err := dialJanus(ctx, a.cfg.URL, a.cfg.InsecureTLS)
	if err != nil {
		return fmt.Errorf("janus join failed: %w", err)
	}
	conn := newJanusConn(ws, logger, a.cfg.APISecret, a.cfg.Token)
	s := newSession(a, peerID, h, logger, conn)
	// The cap exists before any room event can reach the worker: another
	// bot may publish between this bot's joined and its next step.
	s.cap = newReceiveCap(h.ReceiveLimit(), 0)
	s.onLost = func(err error) {
		a.forget(peerID, s)
		h.OnSignalingLost(err)
	}
	conn.OnSession(s.onSessionMessage)
	a.mu.Lock()
	if _, exists := a.sessions[peerID]; exists {
		a.mu.Unlock()
		conn.Close()
		return fmt.Errorf("peer %s is already connected", peerID)
	}
	a.sessions[peerID] = s
	a.mu.Unlock()
	go conn.readLoop()
	go s.watch()
	go s.worker()

	if err := s.join(ctx); err != nil {
		if a.forget(peerID, s) {
			s.leaving.Store(true)
			s.destroy()
			s.close()
		}
		return fmt.Errorf("janus join failed: %w", err)
	}
	s.ready.Store(true)
	// A loss between the last join step and ready took the pre-ready path.
	select {
	case err := <-s.failed:
		if a.forget(peerID, s) {
			s.close()
		}
		return fmt.Errorf("janus join failed: %w", err)
	default:
	}
	logger.Info("Janus adapter: peer joined", slog.Uint64("session_id", conn.SessionID()), slog.Uint64("participant_id", s.participantID))
	return nil
}

// withPIN adds the room PIN to a VideoRoom request when configured.
func (s *session) withPIN(body map[string]any) map[string]any {
	if s.a.cfg.PIN != "" {
		body["pin"] = s.a.cfg.PIN
	}
	return body
}

// join runs research R4 steps 1–6.
func (s *session) join(ctx context.Context) error {
	cfg := s.a.cfg
	if err := s.conn.CreateSession(ctx); err != nil {
		return err
	}
	s.conn.StartKeepalive(s.a.keepalive)
	pub, err := s.conn.Attach(ctx)
	if err != nil {
		return err
	}
	s.pub = pub
	s.conn.OnHandle(pub, s.onPublisherMessage)

	// Step 2: the room, sized for the plan's bots; the dummy takes no
	// publisher place (T002). "Exists" (427) is success: the bots race.
	create := map[string]any{
		"request": "create", "room": cfg.Room, "permanent": false, "publishers": max(cfg.Participants, 1),
		"audiocodec": "opus", "videocodec": "vp8", "bitrate": 0,
		"dummy_publisher": true, "dummy_streams": []map[string]string{{"codec": "opus"}},
	}
	if cfg.AdminKey != "" {
		create["admin_key"] = cfg.AdminKey
	}
	if _, err := s.conn.Message(ctx, pub, s.withPIN(create), nil); err != nil && errorCode(err) != 427 {
		return err
	}

	// Step 3: join as a participant.
	m, err := s.conn.Message(ctx, pub, s.withPIN(map[string]any{
		"request": "join", "ptype": "publisher", "room": cfg.Room, "display": s.peerID,
	}), nil)
	if err != nil {
		return err
	}
	joined := m.Event()
	if joined.VideoRoom != "joined" {
		return fmt.Errorf("join: unexpected reply %q", joined.VideoRoom)
	}
	s.participantID = joined.ID
	i := slices.IndexFunc(joined.Publishers, func(p Publisher) bool { return p.Dummy })
	if i < 0 {
		return fmt.Errorf("room %d has no dummy publisher: create it with dummy_publisher=true (research R3)", cfg.Room)
	}
	dummy := joined.Publishers[i].ID
	err = s.do(ctx, func() error {
		s.cap.dummy = dummy
		s.cap.announce(joined.Publishers)
		return nil
	})
	if err != nil {
		return err
	}

	// Step 4: publish, for bots that send.
	if len(s.h.LocalMediaKinds()) > 0 {
		offer, err := s.h.CreatePublishOffer(s.trickle(pub))
		if err != nil {
			return fmt.Errorf("publish offer: %w", err)
		}
		m, err := s.conn.Message(ctx, pub, map[string]any{"request": "publish", "audiocodec": "opus", "videocodec": "vp8"}, &JSEP{Type: "offer", SDP: offer})
		if err != nil {
			return err
		}
		if m.JSEP == nil || m.JSEP.Type != "answer" {
			return errors.New("publish: no answer in the reply")
		}
		if err := s.h.ApplyPublishAnswer(m.JSEP.SDP); err != nil {
			return fmt.Errorf("publish answer: %w", err)
		}
	}

	// Step 5: the subscription, opened on the dummy (research R3).
	sub, err := s.conn.Attach(ctx)
	if err != nil {
		return err
	}
	s.sub = sub
	s.conn.OnHandle(sub, s.onSubscriberMessage)
	m, err = s.conn.Message(ctx, sub, s.withPIN(map[string]any{
		"request": "join", "ptype": "subscriber", "room": cfg.Room, "private_id": joined.PrivateID,
		"use_msid": false, "autoupdate": false, "streams": []streamRef{{Feed: dummy}},
	}), nil)
	if err != nil {
		return err
	}
	if m.JSEP == nil || m.JSEP.Type != "offer" {
		return errors.New("join: no subscription offer in the reply")
	}
	attached := m.Event()
	// Step 6: answer, then subscribe to the publishers already there.
	return s.do(ctx, func() error {
		for _, st := range attached.Streams {
			if st.FeedID == dummy {
				s.dummyMID = st.MID
			}
		}
		answer, err := s.h.OnOffer(stripDummy(m.JSEP.SDP, s.dummyMID), s.trickle(sub))
		if err != nil {
			return fmt.Errorf("subscription offer: %w", err)
		}
		if _, err := s.conn.Message(ctx, sub, map[string]any{"request": "start"}, &JSEP{Type: "answer", SDP: answer}); err != nil {
			return err
		}
		s.subReady = true
		s.sync()
		return nil
	})
}

// trickle sends the bot's candidates for a handle's PeerConnection.
func (s *session) trickle(handle uint64) platform.TrickleFunc {
	return func(mid, candidate string) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()
			_, err := s.conn.Request(ctx, map[string]any{"janus": "trickle", "handle_id": handle,
				"candidate": Candidate{SDPMid: mid, Candidate: candidate}})
			if err != nil && !s.leaving.Load() {
				s.logger.Debug("Janus trickle failed", slog.String("error", err.Error()))
			}
		}()
	}
}

// sync sends the update that brings the subscription to what the receive
// cap wants, unless an offer is pending (research R6). Worker only.
func (s *session) sync() {
	if !s.subReady || s.offerOut {
		return
	}
	want := s.cap.active()
	u := diff(s.subscribed, want)
	if u.empty() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	m, err := s.conn.Message(ctx, s.sub, u.body(), nil)
	if err != nil {
		s.logger.Warn("Janus subscription update failed", slog.String("error", err.Error()))
		return
	}
	s.subscribed = want
	switch e := m.Event(); {
	case e.VideoRoom == "updating":
		s.offerOut = true // The offer comes unsolicited after the current one
	case m.JSEP != nil:
		s.answer(m.JSEP.SDP)
	}
}

// answer applies a subscription offer and sends its answer. Worker only.
func (s *session) answer(offer string) {
	answer, err := s.h.Renegotiate(stripDummy(offer, s.dummyMID))
	if err != nil {
		s.logger.Warn("Failed to renegotiate the Janus subscription", slog.String("error", err.Error()))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if _, err := s.conn.Message(ctx, s.sub, map[string]any{"request": "start"}, &JSEP{Type: "answer", SDP: answer}); err != nil {
		s.logger.Warn("Janus start failed", slog.String("error", err.Error()))
	}
}

// handleGone handles the events every handle gets: hangup and detached end
// the bot (or fail its join), a trickled candidate goes to the peer, and
// transport news is logged. It reports whether m was one of them.
func (s *session) handleGone(m *Message) bool {
	switch m.Janus {
	case "hangup", "detached":
		reason := m.Reason
		if reason == "" {
			reason = m.Janus
		}
		s.lost(fmt.Errorf("janus %s: %s", m.Janus, reason))
		return true
	case "trickle":
		if c := m.Candidate; c != nil && c.Candidate != "" {
			var err error
			if m.Sender == s.pub {
				err = s.h.OnPublishICECandidate(c.SDPMid, c.Candidate)
			} else {
				err = s.h.OnICECandidate(c.SDPMid, c.Candidate)
			}
			if err != nil {
				s.logger.Debug("Janus candidate rejected", slog.String("error", err.Error()))
			}
		}
		return true
	case "webrtcup", "media", "slowlink":
		s.logger.Debug("Janus "+m.Janus, slog.Uint64("handle", m.Sender), slog.String("mid", m.MID))
		return true
	}
	return false
}

// onPublisherMessage handles the publisher handle's events: room
// publishers come and go, the bot is kicked or the room destroyed. From
// the read loop: work goes to the worker.
func (s *session) onPublisherMessage(m *Message) {
	if s.handleGone(m) {
		return
	}
	e := m.Event()
	if e == nil {
		return
	}
	switch {
	case e.VideoRoom == "destroyed":
		s.lost(errors.New("janus: the room was destroyed"))
	case e.Kicked != 0 && e.Kicked == s.participantID:
		s.lost(errors.New("janus: kicked from the room"))
	case len(e.Publishers) > 0:
		pubs := e.Publishers
		s.enqueue(func() {
			s.cap.announce(pubs)
			s.sync()
		})
	case e.Unpublished != nil && !e.Unpublished.OK:
		s.feedGone(e.Unpublished.ID)
	case e.Leaving != nil && !e.Leaving.OK:
		s.feedGone(e.Leaving.ID)
	}
}

func (s *session) feedGone(feed uint64) {
	s.enqueue(func() {
		s.cap.gone(feed)
		s.sync()
	})
}

// onSubscriberMessage handles the subscriber handle's unsolicited events:
// the offer of an update Janus postponed (T002).
func (s *session) onSubscriberMessage(m *Message) {
	if s.handleGone(m) {
		return
	}
	e := m.Event()
	if e == nil {
		return
	}
	if e.VideoRoom == "destroyed" {
		s.lost(errors.New("janus: the room was destroyed"))
		return
	}
	if e.VideoRoom == "updated" && m.JSEP != nil {
		offer := m.JSEP.SDP
		s.enqueue(func() {
			s.answer(offer)
			s.offerOut = false
			s.sync()
		})
	}
}

// onSessionMessage handles messages without a handle: the session timeout.
func (s *session) onSessionMessage(m *Message) {
	if m.Janus == "timeout" {
		s.lost(errors.New("janus: session timeout"))
	}
}

// stripDummy removes the SSRC and msid lines of the dummy's section (by
// mid) from a subscription offer, so that the peer does not count that
// silent section as an offered stream (research R3).
func stripDummy(offer, mid string) string {
	if mid == "" {
		return offer
	}
	lines := strings.Split(offer, "\r\n")
	out := make([]string, 0, len(lines))
	start := 0 // First line of the current section
	flush := func(end int) {
		section := lines[start:end]
		if start > 0 && slices.Contains(section, "a=mid:"+mid) {
			section = slices.DeleteFunc(slices.Clone(section), func(l string) bool {
				return strings.HasPrefix(l, "a=ssrc:") || strings.HasPrefix(l, "a=ssrc-group:") || strings.HasPrefix(l, "a=msid:")
			})
		}
		out = append(out, section...)
	}
	for i, l := range lines {
		if strings.HasPrefix(l, "m=") {
			flush(i)
			start = i
		}
	}
	flush(len(lines))
	return strings.Join(out, "\r\n")
}

// destroy ends the Janus session, best effort: Janus detaches both handles
// and the bot leaves the room.
func (s *session) destroy() {
	if s.conn.SessionID() == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), disconnectTimeout)
	defer cancel()
	if _, err := s.conn.Request(ctx, map[string]any{"janus": "destroy"}); err != nil {
		s.logger.Debug("Janus destroy failed", slog.String("error", err.Error()))
	}
}

// forget removes s from the registry if it is still registered for
// peerID, and reports whether it did.
func (a *Adapter) forget(peerID string, s *session) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sessions[peerID] != s {
		return false
	}
	delete(a.sessions, peerID)
	return true
}

// Disconnect destroys the bot's session and closes its connection. No SDP
// call runs after it returns. It is safe for unknown or already
// disconnected peers.
func (a *Adapter) Disconnect(peerID string) error {
	a.mu.Lock()
	s, ok := a.sessions[peerID]
	delete(a.sessions, peerID)
	a.mu.Unlock()
	if !ok {
		return nil
	}
	a.logger.Info("Janus adapter: disconnecting peer", slog.String("peer_id", peerID))
	s.leaving.Store(true)
	s.stopped.Do(func() { close(s.stop) })
	select {
	case <-s.worked:
	case <-time.After(disconnectTimeout):
		s.logger.Warn("Janus adapter: SDP work still running after Disconnect")
	}
	s.destroy()
	s.close()
	return nil
}
