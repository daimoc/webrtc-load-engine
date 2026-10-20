package mediasoup

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/media"
)

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
// and it received packets of each kind.
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

// waitConnected waits until the fake's side of every transport of id is
// connected: ICE and DTLS are up, which is what makes the engine's peer
// established (the lifecycle itself is the engine's, not exercised here).
func waitConnected(t *testing.T, fake *fakeMediasoup, id string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		p := fake.peer(id)
		ok := p != nil
		if ok {
			p.mu.Lock()
			for _, ft := range []*fakeTransport{p.recv, p.send} {
				if ft != nil && ft.pc.ConnectionState() != webrtc.PeerConnectionStateConnected {
					ok = false
				}
			}
			ok = ok && p.recv != nil
			p.mu.Unlock()
		}
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: transports not connected", id)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func newTestAdapter(t *testing.T, fake *fakeMediasoup, extra map[string]any, logger *slog.Logger) *Adapter {
	t.Helper()
	m := fake.config("load010")
	for k, v := range extra {
		m[k] = v
	}
	cfg, err := ParseConfig(m)
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		logger = quietLogger
	}
	return NewAdapter(logger, cfg)
}

// connect joins b through a capture wrapper and returns the wrapper.
func connect(t *testing.T, fake *fakeMediasoup, a *Adapter, b *bot) *capture {
	t.Helper()
	c := &capture{SignalingHandler: b, s: fake, id: b.ID()}
	if err := a.Connect(context.Background(), b.ID(), c); err != nil {
		t.Fatalf("Connect(%s): %v", b.ID(), err)
	}
	t.Cleanup(func() { a.Disconnect(b.ID()) })
	return c
}

func TestAdapter_BotsProduceAndConsume(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	if !a.PublishSession() {
		t.Fatal("PublishSession() = false")
	}
	scenarioID := uuid.NewString()
	var bots []*bot
	for _, id := range []string{"bot-1", "bot-2", "bot-3"} {
		b := newBot(t, scenarioID, id, audioVideo, 0)
		connect(t, fake, a, b)
		bots = append(bots, b)
	}
	sendMedia(t, scenarioID, bots...)
	for _, b := range bots {
		waitConnected(t, fake, b.ID())
		waitOffered(t, b, 2)
	}

	p := fake.peer("bot-1")
	p.mu.Lock()
	order := slices.Clone(p.order)
	connected := p.connected
	p.mu.Unlock()
	want := []string{"getRouterRtpCapabilities", "createWebRtcTransport", "connectWebRtcTransport", "createWebRtcTransport", "join", "connectWebRtcTransport", "produce", "produce"}
	if !slices.Equal(order, want) {
		t.Errorf("request order = %v, want %v (research R4)", order, want)
	}
	// Pion answers the ICE-lite receive offer as DTLS server; the publish
	// answer makes the bot the client.
	if connected["consumer"].Role != "server" || connected["producer"].Role != "client" {
		t.Errorf("connect roles: consumer %q, producer %q", connected["consumer"].Role, connected["producer"].Role)
	}
	if n := fake.early.Load(); n != 0 {
		t.Errorf("%d consumers accepted before their section was negotiated", n)
	}
	// Each bot declined the demo's data consumer, and stays connected.
	time.Sleep(200 * time.Millisecond)
	for _, b := range bots {
		waitConnected(t, fake, b.ID())
		select {
		case err := <-b.lost:
			t.Errorf("%s lost: %v", b.ID(), err)
		default:
		}
	}
	select {
	case origin := <-fake.origins:
		if !strings.HasPrefix(origin, "http://127.0.0.1:") {
			t.Errorf("Origin = %q, want http://<url host>", origin)
		}
	default:
		t.Error("no Origin recorded")
	}
}

func TestAdapter_FirstBotAloneIsEstablished(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	scenarioID := uuid.NewString()
	b := newBot(t, scenarioID, "alone", audioVideo, 0)
	connect(t, fake, a, b)
	waitConnected(t, fake, "alone") // Research R3: the receiving session opens at join
	l := newBot(t, scenarioID, "listener-alone", nil, 0)
	other := newFakeMediasoup(t) // An empty room for a receive-only bot
	connect(t, other, newTestAdapter(t, other, nil, nil), l)
	waitConnected(t, other, "listener-alone")
}

func TestAdapter_ReceiveOnlyBot(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	scenarioID := uuid.NewString()
	sender := newBot(t, scenarioID, "sender", audioVideo, 0)
	connect(t, fake, a, sender)
	listener := newBot(t, scenarioID, "listener", nil, 0)
	connect(t, fake, a, listener)
	sendMedia(t, scenarioID, sender)
	waitOffered(t, listener, 1)
	p := fake.peer("listener")
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.send != nil || len(p.producers) != 0 {
		t.Error("the receive-only bot created a producer transport or producers")
	}
}

func TestAdapter_ReceiveCapAndPromotion(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	scenarioID := uuid.NewString()
	var senders []*bot
	for _, id := range []string{"s1", "s2", "s3"} {
		s := newBot(t, scenarioID, id, audioVideo, 0)
		connect(t, fake, a, s)
		senders = append(senders, s)
	}
	sendMedia(t, scenarioID, senders...)
	x := newBot(t, scenarioID, "x", nil, 2)
	connect(t, fake, a, x)
	waitOffered(t, x, 2)
	if got := fake.activeFrom("x"); !slices.Equal(got, []string{"s1/audio", "s1/video", "s2/audio", "s2/video"}) {
		t.Fatalf("x consumes %v, want s1's and s2's (earliest)", got)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(fake.declined("x")) != 2 {
		if time.Now().After(deadline) {
			t.Fatalf("x declined %v, want s3's audio and video", fake.declined("x"))
		}
		time.Sleep(20 * time.Millisecond)
	}

	a.Disconnect("s1")
	deadline = time.Now().Add(10 * time.Second)
	for !slices.Equal(fake.activeFrom("x"), []string{"s2/audio", "s2/video", "s3/audio", "s3/video"}) {
		if time.Now().After(deadline) {
			t.Fatalf("after s1 left, x consumes %v, want s2's and s3's", fake.activeFrom("x"))
		}
		time.Sleep(50 * time.Millisecond)
	}
	resumed, declined := fake.resumed("x"), fake.declined("x")
	slices.Sort(resumed)
	slices.Sort(declined)
	if !slices.Equal(resumed, declined) {
		t.Errorf("resumed %v, want the declined %v", resumed, declined)
	}
	waitOffered(t, x, 2)
	before := x.MediaSnapshot(time.Now())
	time.Sleep(500 * time.Millisecond)
	after := x.MediaSnapshot(time.Now())
	if after[media.KindVideo].Packets <= before[media.KindVideo].Packets || after[media.KindAudio].Packets <= before[media.KindAudio].Packets {
		t.Error("x stopped receiving after the promotion")
	}
	if n := fake.early.Load(); n != 0 {
		t.Errorf("%d consumers accepted or resumed before their section was negotiated", n)
	}
}

func TestAdapter_DisconnectStopsSDPWork(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	scenarioID := uuid.NewString()
	b := newBot(t, scenarioID, "bot-1", audioVideo, 0)
	c := connect(t, fake, a, b)
	if err := a.Disconnect("bot-1"); err != nil {
		t.Fatal(err)
	}
	c.closed.Store(true)
	if err := a.Disconnect("bot-1"); err != nil {
		t.Errorf("second Disconnect: %v", err)
	}
	if err := a.Disconnect("unknown"); err != nil {
		t.Errorf("Disconnect(unknown): %v", err)
	}
	// Another bot producing must not reach the disconnected one.
	other := newBot(t, scenarioID, "bot-2", audioVideo, 0)
	connect(t, fake, a, other)
	time.Sleep(300 * time.Millisecond)
	if n := c.late.Load(); n != 0 {
		t.Errorf("%d SDP calls after Disconnect", n)
	}
	select {
	case err := <-b.lost:
		t.Errorf("Disconnect reported a loss: %v", err)
	default:
	}
	deadline := time.Now().Add(5 * time.Second)
	for fake.peer("bot-1") != nil {
		if time.Now().After(deadline) {
			t.Fatal("the server still has bot-1 after Disconnect")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAdapter_JoinFailures(t *testing.T) {
	t.Run("join rejected", func(t *testing.T) {
		fake := newFakeMediasoup(t)
		fake.failMethod.Store("join")
		a := newTestAdapter(t, fake, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), &capture{SignalingHandler: b, s: fake, id: b.ID()})
		if err == nil || !strings.Contains(err.Error(), "join") || !strings.Contains(err.Error(), "fake failure of join") {
			t.Fatalf("err = %v", err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for fake.peer("bot-1") != nil {
			if time.Now().After(deadline) {
				t.Fatal("the WebSocket stayed open after a failed join")
			}
			time.Sleep(20 * time.Millisecond)
		}
		if err := a.Disconnect("bot-1"); err != nil {
			t.Errorf("Disconnect after a failed join: %v", err)
		}
	})
	t.Run("handshake refused", func(t *testing.T) {
		fake := newFakeMediasoup(t)
		fake.refuseStatus.Store(403)
		a := newTestAdapter(t, fake, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), b)
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("err = %v, want the HTTP status", err)
		}
	})
	t.Run("router without VP8", func(t *testing.T) {
		fake := newFakeMediasoup(t)
		fake.caps.Codecs = slices.DeleteFunc(slices.Clone(fake.caps.Codecs), func(c RtpCodecCapability) bool { return c.MimeType == "video/VP8" })
		a := newTestAdapter(t, fake, nil, nil)
		b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
		err := a.Connect(context.Background(), b.ID(), &capture{SignalingHandler: b, s: fake, id: b.ID()})
		if err == nil || !strings.Contains(err.Error(), "VP8") {
			t.Fatalf("err = %v, want one naming VP8", err)
		}
	})
}

func TestAdapter_SignalingLost(t *testing.T) {
	fake := newFakeMediasoup(t)
	a := newTestAdapter(t, fake, nil, nil)
	b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
	connect(t, fake, a, b)
	fake.kick("bot-1")
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
}

func TestAdapter_URLAndSecret(t *testing.T) {
	fake := newFakeMediasoup(t)
	var logs lockedBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	a := newTestAdapter(t, fake, map[string]any{"unlimited_room_secret": "s3cret-room-key", "use_pipe_transports": true}, logger)
	b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
	connect(t, fake, a, b)
	select {
	case u := <-fake.urls:
		for _, want := range []string{"roomId=load010", "peerId=bot-1", "unlimitedRoomSecret=s3cret-room-key", "usePipeTransports=true"} {
			if !strings.Contains(u, want) {
				t.Errorf("handshake URL %q lacks %q", u, want)
			}
		}
	default:
		t.Fatal("no handshake recorded")
	}
	a.Disconnect("bot-1")
	if strings.Contains(logs.String(), "s3cret-room-key") {
		t.Errorf("logs contain the secret:\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "has_unlimited_room_secret=true") {
		t.Errorf("logs lack the config:\n%s", logs.String())
	}
}
