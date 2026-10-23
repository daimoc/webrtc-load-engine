package janus

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/media"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// lockedBuffer is a log sink written from the adapter's goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// bot is a real engine peer whose signaling losses are recorded instead of
// failing it.
type bot struct {
	*execution.Peer
	lost chan error
}

func (b *bot) OnSignalingLost(err error) { b.lost <- err }

var audioVideo = []string{media.KindAudio, media.KindVideo}

func newBot(t *testing.T, scenarioID, id string, kinds []string, limit int) *bot {
	t.Helper()
	p, err := execution.NewPeer(id, scenarioID, execution.PeerMedia{Kinds: kinds, ReceiveLimit: limit, PublishSession: true}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	return &bot{Peer: p, lost: make(chan error, 4)}
}

// sendMedia runs the scenario's sources into the bots' tracks until the
// test ends.
func sendMedia(t *testing.T, scenarioID string, bots ...*bot) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	for _, kind := range audioVideo {
		c, err := media.LoadContent(kind)
		if err != nil {
			t.Fatal(err)
		}
		src := media.NewSource(scenarioID, c)
		for _, b := range bots {
			if tr := b.LocalTracks()[kind]; tr != nil {
				src.Subscribe(tr)
			}
		}
		go src.Run(ctx)
	}
}

// waitOffered waits until b's current offer has want streams of each kind
// (the dummy's section is not one) and it received packets of each kind.
func waitOffered(t *testing.T, b *bot, want int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		snap := b.MediaSnapshot(time.Now())
		a, v := snap[media.KindAudio], snap[media.KindVideo]
		if a.Offered == want && v.Offered == want && (want == 0 || a.Packets > 0 && v.Packets > 0) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: offered audio %d/video %d (%d/%d packets), want %d each", b.ID(), a.Offered, v.Offered, a.Packets, v.Packets, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitConnected waits until the fake's side of every PeerConnection of id
// is connected: ICE and DTLS are up, which is what makes the engine's peer
// established (the lifecycle itself is the engine's, not exercised here).
func waitConnected(t *testing.T, fake *fakeJanus, id string) {
	t.Helper()
	fake.subConnected(t, id)
	deadline := time.Now().Add(10 * time.Second)
	for {
		p := fake.publisherHandle(id)
		if p == nil {
			t.Fatalf("%s: no publisher handle", id)
		}
		p.pcMu.Lock()
		pc := p.pc
		p.pcMu.Unlock()
		if pc == nil || pc.ConnectionState() == webrtc.PeerConnectionStateConnected {
			return // pc nil: a receive-only bot publishes nothing
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: publisher PC %s", id, pc.ConnectionState())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func newTestAdapter(t *testing.T, fake *fakeJanus, room uint64, extra map[string]any, logger *slog.Logger) *Adapter {
	t.Helper()
	m := fake.config(room)
	for k, v := range extra {
		m[k] = v
	}
	cfg, err := ParseConfig(m)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Participants = 10
	if logger == nil {
		logger = quietLogger
	}
	return NewAdapter(logger, cfg)
}

// connect joins b through a capture wrapper and returns the wrapper.
func connect(t *testing.T, a *Adapter, b *bot) *capture {
	t.Helper()
	c := &capture{SignalingHandler: b}
	if err := a.Connect(context.Background(), b.ID(), c); err != nil {
		t.Fatalf("Connect(%s): %v", b.ID(), err)
	}
	t.Cleanup(func() { a.Disconnect(b.ID()) })
	return c
}

func noLoss(t *testing.T, bots ...*bot) {
	t.Helper()
	for _, b := range bots {
		select {
		case err := <-b.lost:
			t.Errorf("%s lost: %v", b.ID(), err)
		default:
		}
	}
}

func TestAdapter_BotsPublishAndSubscribe(t *testing.T) {
	fake := newFakeJanus(t)
	a := newTestAdapter(t, fake, 1011, nil, nil)
	if !a.PublishSession() {
		t.Fatal("PublishSession() = false")
	}
	scenarioID := uuid.NewString()
	bots := make([]*bot, 3)
	var wg sync.WaitGroup
	for i, id := range []string{"bot-1", "bot-2", "bot-3"} {
		bots[i] = newBot(t, scenarioID, id, audioVideo, 0)
		wg.Add(1)
		go func() { // Together: the room-creation race (FR-004)
			defer wg.Done()
			connect(t, a, bots[i])
		}()
	}
	wg.Wait()
	sendMedia(t, scenarioID, bots...)
	for _, b := range bots {
		waitConnected(t, fake, b.ID())
		waitOffered(t, b, 2)
	}

	fake.mu.Lock()
	created := slices.Clone(fake.created)
	fake.mu.Unlock()
	if len(created) != 3 {
		t.Fatalf("%d room creates, want one per bot", len(created))
	}
	for _, c := range created {
		if num(c["room"]) != 1011 || num(c["publishers"]) != 10 || c["dummy_publisher"] != true || c["permanent"] != false ||
			c["audiocodec"] != "opus" || c["videocodec"] != "vp8" || c["admin_key"] != nil || c["pin"] != nil {
			t.Errorf("room create = %v (research R4)", c)
		}
	}
	got := fake.requestsOf("bot-1")
	want := []string{"create", "attach", "create", "join:publisher", "publish", "attach", "join:subscriber", "start"}
	if len(got) < len(want) || !slices.Equal(got[:len(want)], want) {
		t.Errorf("requests = %v, want %v first (research R4)", got, want)
	}
	fake.mu.Lock()
	updating := fake.updating
	fake.mu.Unlock()
	if updating != 0 {
		t.Errorf("%d updates sent while an offer was unanswered (research R6)", updating)
	}
	select {
	case p := <-fake.subprotocols:
		if p != subprotocol {
			t.Errorf("subprotocol = %q", p)
		}
	default:
		t.Error("no handshake recorded")
	}
	time.Sleep(200 * time.Millisecond)
	noLoss(t, bots...)
}

func TestAdapter_FirstBotAloneIsEstablished(t *testing.T) {
	fake := newFakeJanus(t)
	a := newTestAdapter(t, fake, 1, nil, nil)
	scenarioID := uuid.NewString()
	b := newBot(t, scenarioID, "alone", audioVideo, 0)
	connect(t, a, b)
	waitConnected(t, fake, "alone") // Research R3: the dummy subscription opens at join
	waitOffered(t, b, 0)            // The dummy's section is not an offered stream
	l := newBot(t, scenarioID, "listener-alone", nil, 0)
	connect(t, newTestAdapter(t, fake, 2, nil, nil), l) // An empty room for a receive-only bot
	fake.subConnected(t, "listener-alone")
}

func TestAdapter_ReceiveOnlyBot(t *testing.T) {
	fake := newFakeJanus(t)
	a := newTestAdapter(t, fake, 1, nil, nil)
	scenarioID := uuid.NewString()
	sender := newBot(t, scenarioID, "sender", audioVideo, 0)
	connect(t, a, sender)
	listener := newBot(t, scenarioID, "listener", nil, 0)
	connect(t, a, listener)
	sendMedia(t, scenarioID, sender)
	waitOffered(t, listener, 1)
	if slices.Contains(fake.requestsOf("listener"), "publish") {
		t.Error("the receive-only bot published")
	}
	if h := fake.publisherHandle("listener"); h.pc != nil {
		t.Error("the receive-only bot has a publisher PeerConnection")
	}
}

func TestAdapter_ReceiveCapAndPromotion(t *testing.T) {
	fake := newFakeJanus(t)
	a := newTestAdapter(t, fake, 1, nil, nil)
	scenarioID := uuid.NewString()
	var senders []*bot
	for _, id := range []string{"s1", "s2", "s3"} {
		s := newBot(t, scenarioID, id, audioVideo, 0)
		connect(t, a, s)
		senders = append(senders, s)
	}
	sendMedia(t, scenarioID, senders...)
	x := newBot(t, scenarioID, "x", nil, 2)
	connect(t, a, x)
	waitOffered(t, x, 2)
	if got := fake.subscribed("x"); !slices.Equal(got, []string{"s1/audio", "s1/video", "s2/audio", "s2/video"}) {
		t.Fatalf("x subscribes to %v, want s1's and s2's (earliest)", got)
	}

	a.Disconnect("s1")
	deadline := time.Now().Add(10 * time.Second)
	for !slices.Equal(fake.subscribed("x"), []string{"s2/audio", "s2/video", "s3/audio", "s3/video"}) {
		if time.Now().After(deadline) {
			t.Fatalf("after s1 left, x subscribes to %v, want s2's and s3's", fake.subscribed("x"))
		}
		time.Sleep(50 * time.Millisecond)
	}
	// One update both drops s1's streams and adds s3's.
	ups := fake.updatesOf("x")
	last := ups[len(ups)-1]
	if len(last.Unsubscribe) != 2 || len(last.Subscribe) != 2 {
		t.Errorf("promotion update = %+v, want 2 unsubscribed and 2 subscribed", last)
	}
	waitOffered(t, x, 2)
	before := x.MediaSnapshot(time.Now())
	time.Sleep(500 * time.Millisecond)
	after := x.MediaSnapshot(time.Now())
	if after[media.KindVideo].Packets <= before[media.KindVideo].Packets || after[media.KindAudio].Packets <= before[media.KindAudio].Packets {
		t.Error("x stopped receiving after the promotion")
	}
	fake.mu.Lock()
	updating := fake.updating
	fake.mu.Unlock()
	if updating != 0 {
		t.Errorf("%d updates sent while an offer was unanswered", updating)
	}
	noLoss(t, x)
}

func TestAdapter_KeepaliveHoldsSession(t *testing.T) {
	fake := newFakeJanus(t)
	fake.sessionTimeout = 300 * time.Millisecond
	a := newTestAdapter(t, fake, 1, nil, nil)
	a.keepalive = 100 * time.Millisecond
	b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
	connect(t, a, b)
	time.Sleep(time.Second)
	noLoss(t, b)
	if fake.publisherHandle("bot-1") == nil {
		t.Error("the session expired despite keepalives")
	}
}

func TestAdapter_DisconnectStopsSDPWork(t *testing.T) {
	fake := newFakeJanus(t)
	a := newTestAdapter(t, fake, 1, nil, nil)
	scenarioID := uuid.NewString()
	b := newBot(t, scenarioID, "bot-1", audioVideo, 0)
	c := connect(t, a, b)
	if err := a.Disconnect("bot-1"); err != nil {
		t.Fatal(err)
	}
	c.closed.Store(true)
	if fake.destroys.Load() != 1 {
		t.Errorf("%d destroy requests, want 1", fake.destroys.Load())
	}
	if err := a.Disconnect("bot-1"); err != nil {
		t.Errorf("second Disconnect: %v", err)
	}
	if err := a.Disconnect("unknown"); err != nil {
		t.Errorf("Disconnect(unknown): %v", err)
	}
	other := newBot(t, scenarioID, "bot-2", audioVideo, 0)
	connect(t, a, other)
	time.Sleep(300 * time.Millisecond)
	if n := c.late.Load(); n != 0 {
		t.Errorf("%d SDP calls after Disconnect", n)
	}
	noLoss(t, b)
	deadline := time.Now().Add(5 * time.Second)
	for fake.publisherHandle("bot-1") != nil {
		if time.Now().After(deadline) {
			t.Fatal("the server still has bot-1 after Disconnect")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAdapter_JoinFailures(t *testing.T) {
	t.Run("publish rejected", func(t *testing.T) {
		fake := newFakeJanus(t)
		fake.failRequest("publish", 432, "Maximum number of active publishers (1) already reached")
		a := newTestAdapter(t, fake, 1, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), &capture{SignalingHandler: b})
		if err == nil || !strings.Contains(err.Error(), "publish: 432 Maximum number of active publishers") {
			t.Fatalf("err = %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for fake.publisherHandle("bot-1") != nil {
			if time.Now().After(deadline) {
				t.Fatal("the session stayed open after a failed join")
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err := a.Disconnect("bot-1"); err != nil {
			t.Errorf("Disconnect after a failed join: %v", err)
		}
	})
	t.Run("handshake refused", func(t *testing.T) {
		fake := newFakeJanus(t)
		fake.refuseStatus.Store(403)
		a := newTestAdapter(t, fake, 1, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), b)
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("err = %v, want the HTTP status", err)
		}
	})
	t.Run("room without dummy publisher", func(t *testing.T) {
		fake := newFakeJanus(t)
		fake.prepareRoom(7, 10, false)
		a := newTestAdapter(t, fake, 7, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), b)
		if err == nil || !strings.Contains(err.Error(), "room 7 has no dummy publisher") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("wrong api secret", func(t *testing.T) {
		fake := newFakeJanus(t)
		fake.apiSecret = "right"
		a := newTestAdapter(t, fake, 1, map[string]any{"api_secret": "wrong"}, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), b)
		if err == nil || !strings.Contains(err.Error(), "create: 403 Unauthorized") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestAdapter_SignalingLost(t *testing.T) {
	for name, cause := range map[string]func(fake *fakeJanus, a *Adapter){
		"websocket closed":  func(fake *fakeJanus, a *Adapter) { fake.kick("bot-1") },
		"subscriber hangup": func(fake *fakeJanus, a *Adapter) { fake.hangup("bot-1") },
		"session timeout": func(fake *fakeJanus, a *Adapter) {
			fake.mu.Lock()
			fake.sessionTimeout = 200 * time.Millisecond
			fake.mu.Unlock()
		},
	} {
		t.Run(name, func(t *testing.T) {
			fake := newFakeJanus(t)
			a := newTestAdapter(t, fake, 1, nil, nil)
			b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
			connect(t, a, b)
			cause(fake, a)
			select {
			case <-b.lost:
			case <-time.After(10 * time.Second):
				t.Fatal("loss not reported")
			}
			time.Sleep(300 * time.Millisecond)
			select {
			case err := <-b.lost:
				t.Errorf("loss reported twice (%v)", err)
			default:
			}
			if err := a.Disconnect("bot-1"); err != nil {
				t.Errorf("Disconnect after a loss: %v", err)
			}
		})
	}
}

func TestAdapter_SecretsAndAdminKey(t *testing.T) {
	fake := newFakeJanus(t)
	fake.apiSecret, fake.adminKey = "apisecret-value", "adminkey-value"
	var logs lockedBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := newTestAdapter(t, fake, 1, map[string]any{
		"api_secret": "apisecret-value", "token": "token-value", "pin": "pin-value", "admin_key": "adminkey-value",
	}, logger)
	a.cfg.Participants = 4
	scenarioID := uuid.NewString()
	b := newBot(t, scenarioID, "bot-1", audioVideo, 0)
	connect(t, a, b)
	fake.mu.Lock()
	created := fake.created[0]
	fake.mu.Unlock()
	if created["admin_key"] != "adminkey-value" || created["pin"] != "pin-value" || num(created["publishers"]) != 4 {
		t.Errorf("room create = %v", created)
	}
	for _, raw := range fake.framesOf("bot-1") {
		if !strings.Contains(raw, `"apisecret":"apisecret-value"`) || !strings.Contains(raw, `"token":"token-value"`) {
			t.Errorf("frame without apisecret or token: %s", raw)
		}
		if strings.Contains(raw, `"request":"join"`) && !strings.Contains(raw, `"pin":"pin-value"`) {
			t.Errorf("join without pin: %s", raw)
		}
	}
	a.Disconnect("bot-1")
	for _, v := range []string{"apisecret-value", "token-value", "pin-value", "adminkey-value"} {
		if strings.Contains(logs.String(), v) {
			t.Errorf("logs contain %q:\n%s", v, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "has_api_secret=true") {
		t.Errorf("logs lack the config:\n%s", logs.String())
	}
}

func TestStripDummy_RealOffer(t *testing.T) {
	b, err := os.ReadFile("testdata/golden/updated_offer.sdp")
	if err != nil {
		t.Fatal(err)
	}
	offer := string(b)
	out := stripDummy(offer, "0")
	sections := strings.Split(out, "\r\nm=")
	if len(sections) != 4 {
		t.Fatalf("%d sections", len(sections))
	}
	for _, l := range []string{"a=ssrc:2043142101", "a=msid:janus janus0"} {
		if strings.Contains(sections[1], l) {
			t.Errorf("dummy section keeps %q", l)
		}
	}
	// The other sections and the rest of the dummy's are untouched.
	for _, l := range []string{"a=ssrc:2175825213 cname:janus", "a=msid:janus janus1", "a=ssrc:1652509142 cname:janus", "a=msid:janus janus2"} {
		if !strings.Contains(out, l) {
			t.Errorf("lost %q", l)
		}
	}
	if !strings.Contains(sections[1], "a=mid:0") || !strings.Contains(sections[1], "a=sendonly") || !strings.Contains(sections[1], "a=rtpmap:111 opus/48000/2") {
		t.Errorf("dummy section damaged:\n%s", sections[1])
	}
	if got := strings.Count(offer, "\r\n") - strings.Count(out, "\r\n"); got != 2 {
		t.Errorf("%d lines removed, want 2", got)
	}
	if stripDummy(offer, "") != offer || stripDummy(offer, "9") != offer {
		t.Error("unknown mid changed the offer")
	}
}
