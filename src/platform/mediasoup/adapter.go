package mediasoup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"webrtc-load-engine/src/platform"
)

// connectTimeout bounds a bot's join (a variable for tests).
var connectTimeout = 20 * time.Second

// disconnectTimeout bounds the wait for the bot's SDP goroutine in
// Disconnect.
const disconnectTimeout = 2 * time.Second

// botHeaderExtensions are the router header extensions the bots announce
// in join (research R5).
var botHeaderExtensions = []string{
	"urn:ietf:params:rtp-hdrext:sdes:mid",
	"http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time",
	"http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01",
	"urn:ietf:params:rtp-hdrext:ssrc-audio-level",
}

// Adapter joins bots to a mediasoup-demo room (research R1–R7). It
// implements platform.PlatformAdapter and platform.PublishSessionAdapter.
type Adapter struct {
	logger *slog.Logger
	cfg    Config

	mu          sync.Mutex
	sessions    map[string]*session // By peer ID
	versionOnce sync.Once
	unknownLogs sync.Map // Server request methods already logged as unsupported
}

var (
	_ platform.PlatformAdapter       = (*Adapter)(nil)
	_ platform.PublishSessionAdapter = (*Adapter)(nil)
)

// NewAdapter returns an adapter for the mediasoup-demo server and room of
// cfg.
func NewAdapter(logger *slog.Logger, cfg Config) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{logger: logger, cfg: cfg, sessions: map[string]*session{}}
}

// PublishSession implements platform.PublishSessionAdapter: mediasoup
// receives a bot's media on a transport of its own, the session the bot
// offers.
func (a *Adapter) PublishSession() bool { return true }

// dialURL is the protoo URL of research R1 (the secret travels only here).
func dialURL(cfg Config, peerID string) string {
	q := url.Values{"roomId": {cfg.Room}, "peerId": {peerID}}
	if cfg.UnlimitedRoomSecret != "" {
		q.Set("unlimitedRoomSecret", cfg.UnlimitedRoomSecret)
	}
	if cfg.UsePipeTransports {
		q.Set("usePipeTransports", "true")
	}
	return cfg.URL + "/?" + q.Encode()
}

// session is one bot's signaling connection and receive state (data-model
// "Bot session"). Everything that touches the bot's SDP runs on its worker
// goroutine, in order.
type session struct {
	a      *Adapter
	peerID string
	h      platform.SignalingHandler
	logger *slog.Logger
	conn   *protooConn

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

	// Worker only.
	routerCaps RtpCapabilities
	recvT      TransportInfo
	sendT      TransportInfo
	recv       *recvOffer
	cap        *capState
}

func newSession(a *Adapter, peerID string, h platform.SignalingHandler, logger *slog.Logger, conn *protooConn) *session {
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
		s.logger.Warn("mediasoup signaling lost", slog.String("error", err.Error()))
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

	u := dialURL(a.cfg, peerID)
	logger.Info("mediasoup adapter: connecting peer", slog.String("url", redactQuery(u)), slog.Any("config", a.cfg))
	ws, err := dialProtoo(ctx, u, a.cfg.Origin, a.cfg.InsecureTLS)
	if err != nil {
		return fmt.Errorf("mediasoup join failed: %w", err)
	}
	conn := newProtooConn(ws, logger)
	s := newSession(a, peerID, h, logger, conn)
	conn.onRequest = s.onRequest
	conn.onNotification = s.onNotification
	s.onLost = func(err error) {
		a.forget(peerID, s)
		h.OnSignalingLost(err)
	}
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
			s.close()
		}
		return fmt.Errorf("mediasoup join failed: %w", err)
	}
	s.ready.Store(true)
	// A loss between the last join step and ready took the pre-ready path.
	select {
	case err := <-s.failed:
		if a.forget(peerID, s) {
			s.close()
		}
		return fmt.Errorf("mediasoup join failed: %w", err)
	default:
	}
	logger.Info("mediasoup adapter: peer joined")
	return nil
}

// dropTrickle drops local candidates: mediasoup is ICE-lite and learns the
// bot's address from its checks (research R2).
func dropTrickle(string, string) {}

// join runs research R4 steps 2–6.
func (s *session) join(ctx context.Context) error {
	kinds := s.h.LocalMediaKinds()
	var rc routerCapsResponse
	if err := s.conn.Request(ctx, "getRouterRtpCapabilities", nil, &rc); err != nil {
		return err
	}
	routerCaps := rc.RouterRtpCapabilities
	for _, kind := range kinds {
		want := wantedCodec[kind].mime
		if !slices.ContainsFunc(routerCaps.Codecs, func(c RtpCodecCapability) bool { return strings.EqualFold(c.MimeType, want) }) {
			return fmt.Errorf("the router has no %s codec", want)
		}
	}

	var recvT TransportInfo
	if err := s.conn.Request(ctx, "createWebRtcTransport", createTransportRequest{AppData: map[string]string{"direction": "consumer"}}, &recvT); err != nil {
		return err
	}
	// Open the receiving session now (research R3), so the bot is
	// established whoever else is in the room.
	err := s.do(ctx, func() error {
		s.routerCaps, s.recvT = routerCaps, recvT
		s.recv = newRecvOffer(recvT)
		s.cap = newCapState(s.h.ReceiveLimit())
		offer, err := s.recv.SDP()
		if err != nil {
			return err
		}
		answer, err := s.h.OnOffer(offer, dropTrickle)
		if err != nil {
			return fmt.Errorf("receive offer: %w", err)
		}
		return s.connectTransport(ctx, recvT.TransportID, answer)
	})
	if err != nil {
		return err
	}

	var sendT TransportInfo
	if len(kinds) > 0 {
		if err := s.conn.Request(ctx, "createWebRtcTransport", createTransportRequest{AppData: map[string]string{"direction": "producer"}}, &sendT); err != nil {
			return err
		}
	}
	var jr joinResponse
	join := joinRequest{DisplayName: s.peerID, Device: device{Flag: "unknown", Name: "webrtc-load-engine"}, RtpCapabilities: botCaps(routerCaps)}
	if err := s.conn.Request(ctx, "join", join, &jr); err != nil {
		return err
	}
	s.logger.Debug("mediasoup join response", slog.Int("peers", len(jr.Peers)))
	if len(kinds) == 0 {
		return nil
	}
	return s.do(ctx, func() error {
		s.sendT = sendT
		return s.publish(ctx)
	})
}

// botCaps are the RTP capabilities the bot announces (research R5): the
// router's Opus, VP8 and VP8's RTX, with the router's payload types, and
// the header extensions of botHeaderExtensions.
func botCaps(router RtpCapabilities) RtpCapabilities {
	caps := RtpCapabilities{Codecs: []RtpCodecCapability{}, HeaderExtensions: []RtpHeaderExtension{}}
	vp8 := -1
	for _, c := range router.Codecs {
		switch strings.ToLower(c.MimeType) {
		case "audio/opus":
			caps.Codecs = append(caps.Codecs, c)
		case "video/vp8":
			caps.Codecs = append(caps.Codecs, c)
			if vp8 < 0 {
				vp8 = c.PreferredPayloadType
			}
		}
	}
	for _, c := range router.Codecs {
		if strings.EqualFold(c.MimeType, "video/rtx") && aptOf(c.Parameters) == vp8 {
			caps.Codecs = append(caps.Codecs, c)
		}
	}
	for _, h := range router.HeaderExtensions {
		if slices.Contains(botHeaderExtensions, h.URI) {
			caps.HeaderExtensions = append(caps.HeaderExtensions, h)
		}
	}
	return caps
}

func aptOf(p map[string]any) int {
	switch v := p["apt"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return -2
}

// connectTransport sends connectWebRtcTransport with the bot's DTLS
// parameters, read from its own SDP.
func (s *session) connectTransport(ctx context.Context, transportID, localSDP string) error {
	fp, err := localFingerprint(localSDP)
	if err != nil {
		return err
	}
	role, err := localDtlsRole(localSDP)
	if err != nil {
		return err
	}
	return s.conn.Request(ctx, "connectWebRtcTransport", connectTransportRequest{
		TransportID:    transportID,
		DtlsParameters: DtlsParameters{Role: role, Fingerprints: []DtlsFingerprint{fp}},
	}, nil)
}

// publish produces the bot's tracks (research R4 step 6). Worker only.
func (s *session) publish(ctx context.Context) error {
	offer, err := s.h.CreatePublishOffer(dropTrickle)
	if err != nil {
		return fmt.Errorf("publish offer: %w", err)
	}
	specs, err := producerParams(offer, s.routerCaps)
	if err != nil {
		return err
	}
	if err := s.connectTransport(ctx, s.sendT.TransportID, offer); err != nil {
		return err
	}
	for _, spec := range specs {
		source := "mic"
		if spec.Kind == "video" {
			source = "webcam"
		}
		var pr produceResponse
		if err := s.conn.Request(ctx, "produce", produceRequest{
			TransportID: s.sendT.TransportID, Kind: spec.Kind, RtpParameters: spec.RtpParameters,
			AppData: map[string]string{"source": source},
		}, &pr); err != nil {
			return err
		}
		s.logger.Debug("mediasoup producer created", slog.String("kind", spec.Kind), slog.String("producer_id", pr.ProducerID))
	}
	answer, err := publishAnswer(offer, s.sendT, specs)
	if err != nil {
		return err
	}
	if err := s.h.ApplyPublishAnswer(answer); err != nil {
		return fmt.Errorf("publish answer: %w", err)
	}
	return nil
}

// onRequest handles the server's requests (contracts/signaling-messages.md);
// it runs on the read loop and hands SDP work to the worker.
func (s *session) onRequest(method string, data json.RawMessage, reply replyFunc) {
	switch method {
	case "newConsumer":
		var nc NewConsumerRequest
		if err := json.Unmarshal(data, &nc); err != nil {
			reply(nil, &requestError{Code: 400, Reason: "bad newConsumer"})
			return
		}
		if nc.Kind != "audio" && nc.Kind != "video" {
			reply(nil, &requestError{Code: 403, Reason: "kind not received"})
			return
		}
		s.enqueue(func() { s.onNewConsumer(nc, reply) })
	case "newDataConsumer":
		reply(nil, &requestError{Code: 403, Reason: "data channels not used"})
	default:
		if _, logged := s.a.unknownLogs.LoadOrStore(method, true); !logged {
			s.logger.Info("Unsupported mediasoup request", slog.String("method", method))
		}
		reply(nil, &requestError{Code: 405, Reason: "unsupported request"})
	}
}

// onNotification handles the server's notifications.
func (s *session) onNotification(method string, data json.RawMessage) {
	switch method {
	case "consumerClosed":
		var ref consumerRef
		if json.Unmarshal(data, &ref) != nil || ref.ConsumerID == "" {
			return
		}
		s.enqueue(func() {
			if s.cap != nil {
				s.apply(s.cap.closed(ref.ConsumerID))
			}
		})
	case "peerClosed":
		var ref peerRef
		if json.Unmarshal(data, &ref) == nil {
			s.logger.Debug("mediasoup peer closed", slog.String("peer", ref.PeerID))
		}
	case "mediasoupVersion":
		var v versionNotification
		if json.Unmarshal(data, &v) == nil {
			s.a.versionOnce.Do(func() { s.a.logger.Info("mediasoup server version", slog.String("version", v.Version)) })
		}
	}
}

// onNewConsumer applies the receive cap to a newConsumer (research R6).
// Worker only.
func (s *session) onNewConsumer(nc NewConsumerRequest, reply replyFunc) {
	if s.cap == nil {
		reply(nil, &requestError{Code: 503, Reason: "not ready"})
		return
	}
	act := s.cap.offer(nc)
	if act.kind == actionDecline {
		reply(nil, &requestError{Code: 403, Reason: "receive cap"})
		return
	}
	s.recv.add(nc)
	if err := s.renegotiate(); err != nil {
		s.logger.Warn("Failed to add a mediasoup consumer", slog.String("consumer_id", nc.ConsumerID), slog.String("error", err.Error()))
		reply(nil, &requestError{Code: 500, Reason: "renegotiation failed"})
		s.recv.deactivate(nc.ConsumerID)
		s.apply(s.cap.closed(nc.ConsumerID))
		return
	}
	reply(nil, nil)
}

// apply carries out the cap's actions after a close: sections changed,
// one renegotiation, then the promoted consumers resumed. Worker only.
func (s *session) apply(acts []capAction) {
	if len(acts) == 0 {
		return
	}
	var promoted []string
	for _, act := range acts {
		switch act.kind {
		case actionDeactivate:
			s.recv.deactivate(act.consumerID)
		case actionPromote:
			s.recv.add(act.consumer)
			promoted = append(promoted, act.consumer.ConsumerID)
		}
	}
	if err := s.renegotiate(); err != nil {
		s.logger.Warn("Failed to renegotiate the mediasoup receiving session", slog.String("error", err.Error()))
		return
	}
	for _, id := range promoted {
		if err := s.conn.Notify("resumeConsumer", consumerRef{ConsumerID: id}); err != nil {
			s.logger.Debug("Failed to resume a mediasoup consumer", slog.String("consumer_id", id), slog.String("error", err.Error()))
		}
	}
}

// renegotiate applies the current receive offer. Worker only.
func (s *session) renegotiate() error {
	offer, err := s.recv.SDP()
	if err != nil {
		return err
	}
	_, err = s.h.Renegotiate(offer)
	return err
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

// Disconnect closes the bot's connection; the server then closes its
// transports. No SDP call runs after it returns. It is safe for unknown or
// already disconnected peers.
func (a *Adapter) Disconnect(peerID string) error {
	a.mu.Lock()
	s, ok := a.sessions[peerID]
	delete(a.sessions, peerID)
	a.mu.Unlock()
	if !ok {
		return nil
	}
	a.logger.Info("mediasoup adapter: disconnecting peer", slog.String("peer_id", peerID))
	s.leaving.Store(true)
	s.close()
	select {
	case <-s.worked:
	case <-time.After(disconnectTimeout):
		s.logger.Warn("mediasoup adapter: SDP work still running after Disconnect")
	}
	return nil
}
