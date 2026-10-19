package livekit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"nhooyr.io/websocket"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/scenario"
)

var quietLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

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

// waitStreams waits until b receives want streams of each kind, with
// packets.
func waitStreams(t *testing.T, b *bot, want int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		snap := b.MediaSnapshot(time.Now())
		a, v := snap[media.KindAudio], snap[media.KindVideo]
		if a.Streams == want && v.Streams == want && (want == 0 || a.Packets > 0 && v.Packets > 0) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: received audio %d/video %d streams (%d/%d packets), want %d each", b.ID(), a.Streams, v.Streams, a.Packets, v.Packets, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// waitLive waits until the fake forwards every track of identity (media
// flowing), and returns them.
func waitLive(t *testing.T, fake *fakeLiveKit, identity string, n int) []trackInfo {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if p := fake.participant(identity); p != nil {
			if tracks := p.info().Tracks; len(tracks) == n {
				return tracks
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: tracks not live", identity)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitOffered waits until b's last applied offer has want streams of each
// kind, with packets on each kind.
func waitOffered(t *testing.T, b *bot, want int) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		snap := b.MediaSnapshot(time.Now())
		a, v := snap[media.KindAudio], snap[media.KindVideo]
		if a.Offered == want && v.Offered == want && a.Packets > 0 && v.Packets > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: offered audio %d/video %d, want %d each", b.ID(), a.Offered, v.Offered, want)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func newTestAdapter(t *testing.T, fake *fakeLiveKit, logger *slog.Logger) *Adapter {
	t.Helper()
	cfg, err := ParseConfig(fake.config("load008"))
	if err != nil {
		t.Fatal(err)
	}
	if logger == nil {
		logger = quietLogger
	}
	return NewAdapter(logger, cfg)
}

func connect(t *testing.T, a *Adapter, b *bot) {
	t.Helper()
	if err := a.Connect(context.Background(), b.ID(), b); err != nil {
		t.Fatalf("Connect(%s): %v", b.ID(), err)
	}
	t.Cleanup(func() { a.Disconnect(b.ID()) })
}

func TestAdapter_BotsPublishAndReceive(t *testing.T) {
	fake := newFakeLiveKit(t)
	a := newTestAdapter(t, fake, nil)
	if !a.PublishSession() {
		t.Fatal("PublishSession() = false")
	}
	scenarioID := uuid.NewString()
	var bots []*bot
	for i, id := range []string{"bot-1", "bot-2", "bot-3"} {
		b := newBot(t, scenarioID, id, audioVideo, 0)
		connect(t, a, b)
		// Connect returns once the bot has published (FR-003).
		if n := fake.offers.Load(); int(n) != i+1 {
			t.Fatalf("after %s joined: %d publish offers, want %d", id, n, i+1)
		}
		bots = append(bots, b)
	}
	sendMedia(t, scenarioID, bots...)
	for _, b := range bots {
		waitStreams(t, b, 2)
	}

	for _, b := range bots {
		if err := a.Disconnect(b.ID()); err != nil {
			t.Fatal(err)
		}
	}
	for range bots {
		select {
		case <-fake.leaves:
		case <-time.After(5 * time.Second):
			t.Fatal("a bot left without a LeaveRequest")
		}
	}
	for _, b := range bots {
		select {
		case err := <-b.lost:
			t.Errorf("%s: Disconnect reported a loss: %v", b.ID(), err)
		default:
		}
	}
}

func TestAdapter_ReceiveOnlyBot(t *testing.T) {
	fake := newFakeLiveKit(t)
	a := newTestAdapter(t, fake, nil)
	scenarioID := uuid.NewString()
	sender := newBot(t, scenarioID, "sender", audioVideo, 0)
	connect(t, a, sender)
	listener := newBot(t, scenarioID, "listener", nil, 0)
	connect(t, a, listener)
	if n := fake.offers.Load(); n != 1 {
		t.Errorf("publish offers = %d, want 1 (the receive-only bot publishes nothing)", n)
	}
	sendMedia(t, scenarioID, sender)
	waitStreams(t, listener, 1)
}

func TestAdapter_ReceiveCapAndReplacement(t *testing.T) {
	fake := newFakeLiveKit(t)
	a := newTestAdapter(t, fake, nil)
	scenarioID := uuid.NewString()
	s1 := newBot(t, scenarioID, "s1", audioVideo, 0)
	s2 := newBot(t, scenarioID, "s2", audioVideo, 0)
	connect(t, a, s1)
	connect(t, a, s2)
	sendMedia(t, scenarioID, s1, s2)
	waitLive(t, fake, "s1", 2)
	waitLive(t, fake, "s2", 2)

	x := newBot(t, scenarioID, "x", nil, 1)
	connect(t, a, x)
	waitStreams(t, x, 1)
	owners := func() []string {
		var ids []string
		for _, sid := range fake.participant("x").subscribedTo() {
			if tr := fake.track(sid); tr != nil {
				ids = append(ids, tr.owner.identity)
			}
		}
		slices.Sort(ids)
		return ids
	}
	if got := owners(); !slices.Equal(got, []string{"s1", "s1"}) {
		t.Fatalf("x subscribed to tracks of %v, want s1's audio and video (earliest)", got)
	}

	a.Disconnect("s1")
	deadline := time.Now().Add(10 * time.Second)
	for !slices.Equal(owners(), []string{"s2", "s2"}) {
		if time.Now().After(deadline) {
			t.Fatalf("after s1 left, x subscribed to tracks of %v, want s2's", owners())
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Streams counts every stream received: s1's ended ones and s2's.
	waitOffered(t, x, 1)
}

func TestAdapter_WrongSecret(t *testing.T) {
	fake := newFakeLiveKit(t)
	var logs lockedBuffer
	cfg := fake.config("load008")
	cfg["api_secret"] = "wrong-secret-value"
	c, err := ParseConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	a := NewAdapter(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug})), c)
	b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
	err = a.Connect(context.Background(), b.ID(), b)
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("Connect err = %v, want unauthorized", err)
	}
	if fake.refused.Load() != 1 {
		t.Errorf("refused joins = %d, want 1", fake.refused.Load())
	}
	tok, _ := tokenFor(c, b.ID(), time.Now())
	sig := tok[strings.LastIndex(tok, ".")+1:]
	for _, secret := range []string{"wrong-secret-value", sig} {
		if strings.Contains(logs.String(), secret) || strings.Contains(err.Error(), secret) {
			t.Errorf("logs or error contain a secret (%q)", secret)
		}
	}
	if !strings.Contains(logs.String(), "access_token=%5Bredacted%5D") {
		t.Errorf("join URL not logged redacted: %s", logs.String())
	}
}

func TestAdapter_SignalingLosses(t *testing.T) {
	for name, cause := range map[string]func(*fakeLiveKit){
		"no pongs":     func(f *fakeLiveKit) { f.dropPongs.Store(true) },
		"server leave": func(f *fakeLiveKit) { f.sendLeave("bot-1") },
		"socket close": func(f *fakeLiveKit) { f.kick("bot-1") },
	} {
		t.Run(name, func(t *testing.T) {
			fake := newFakeLiveKit(t)
			a := newTestAdapter(t, fake, nil)
			b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)
			connect(t, a, b)
			cause(fake)
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

func TestAdapter_DisconnectIsSafe(t *testing.T) {
	fake := newFakeLiveKit(t)
	a := newTestAdapter(t, fake, nil)
	if err := a.Disconnect("unknown"); err != nil {
		t.Errorf("Disconnect(unknown) = %v", err)
	}
	b := newBot(t, uuid.NewString(), "bot-1", []string{media.KindAudio}, 0)
	connect(t, a, b)
	if err := a.Disconnect("bot-1"); err != nil {
		t.Fatal(err)
	}
	if err := a.Disconnect("bot-1"); err != nil {
		t.Errorf("second Disconnect = %v", err)
	}
	select {
	case id := <-fake.leaves:
		if id != "bot-1" {
			t.Errorf("leave from %s", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no LeaveRequest")
	}
}

func TestAdapter_SubscriptionRefused(t *testing.T) {
	fake := newFakeLiveKit(t)
	a := newTestAdapter(t, fake, nil)
	scenarioID := uuid.NewString()
	s1 := newBot(t, scenarioID, "s1", audioVideo, 0)
	connect(t, a, s1)
	sendMedia(t, scenarioID, s1)
	var videoSID string
	for _, tr := range waitLive(t, fake, "s1", 2) {
		if tr.Type == trackVideo {
			videoSID = tr.SID
		}
	}
	fake.refuseSub.Store(videoSID)
	x := newBot(t, scenarioID, "x", nil, 0)
	connect(t, a, x)
	deadline := time.Now().Add(10 * time.Second)
	for {
		snap := x.MediaSnapshot(time.Now())
		if snap[media.KindAudio].Streams == 1 && snap[media.KindAudio].Packets > 0 {
			if snap[media.KindVideo].Streams != 0 {
				t.Errorf("refused video received anyway")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("audio not received: %+v", snap)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestAdapter_CancelledJoinLeavesNothing(t *testing.T) {
	// A server that accepts the WebSocket and never answers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		for {
			if _, _, err := c.Read(r.Context()); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	cfg, err := ParseConfig(map[string]any{"url": srv.URL, "room": "r", "api_key": "k", "api_secret": "s"})
	if err != nil {
		t.Fatal(err)
	}
	a := NewAdapter(quietLogger, cfg)
	b := newBot(t, uuid.NewString(), "bot-1", audioVideo, 0)

	before := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := a.Connect(ctx, b.ID(), b); err == nil {
		t.Fatal("Connect succeeded without a JoinResponse")
	}
	a.mu.Lock()
	n := len(a.sessions)
	a.mu.Unlock()
	if n != 0 {
		t.Errorf("%d sessions left after a failed join", n)
	}
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines: %d before the join, %d after", before, runtime.NumGoroutine())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestOfferTracks(t *testing.T) {
	sdp := "v=0\r\nm=audio 9 UDP/TLS/RTP/SAVPF 111\r\na=mid:0\r\na=msid:peer-1 audio\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\na=mid:1\r\na=msid:peer-1 video\r\nm=application 9 UDP/DTLS/SCTP webrtc-datachannel\r\na=msid:x y\r\n"
	got := offerTracks(sdp)
	want := []localTrack{{kind: "audio", id: "audio"}, {kind: "video", id: "video"}}
	if !slices.Equal(got, want) {
		t.Errorf("offerTracks = %+v, want %+v", got, want)
	}
}

func TestRedactURL(t *testing.T) {
	u := dialURL("ws://h:7880/rtc", "tok.en.sig")
	if r := redactURL(u); strings.Contains(r, "tok.en.sig") || !strings.Contains(r, "auto_subscribe=0") || !strings.Contains(r, "protocol=17") {
		t.Errorf("redactURL(%s) = %s", u, r)
	}
}

// jsonPaths returns the dotted key paths of a JSON object.
func jsonPaths(prefix string, v any, out map[string]bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	for k, child := range m {
		out[prefix+k] = true
		jsonPaths(prefix+k+".", child, out)
	}
}

// The real engine through the LiveKit adapter: the quality summary has the
// sections and fields a Jitsi run has (US2, SC-004).
func TestEngine_LiveKitQualitySummary(t *testing.T) {
	fake := newFakeLiveKit(t)
	cfg, err := ParseConfig(fake.config("load008"))
	if err != nil {
		t.Fatal(err)
	}
	plan := scenario.TestPlan{Participants: 3, DurationSeconds: 6, Media: &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo}}
	sc, err := scenario.NewScenario("lk", "livekit", plan)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *scenario.QualitySummary, 1)
	engine := execution.NewEngine("lk-engine-"+uuid.NewString(), sc.TestPlan, NewAdapter(quietLogger, cfg), quietLogger,
		func(_ string, r scenario.RunResult) { q := r.Quality; done <- q })
	go engine.Start()
	var q *scenario.QualitySummary
	select {
	case q = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("scenario did not complete")
	}
	if q == nil || q.Bots != 3 {
		t.Fatalf("summary %+v", q)
	}
	if !q.Video.Available || q.Video.Bots != 3 || q.Video.FrameRate == nil || q.Video.FrameRate.P50 < 25 {
		t.Errorf("video %+v", q.Video)
	}
	if !q.Audio.Available || q.Audio.Bots != 3 || q.Audio.MOS == nil || q.Audio.MOS.P50 < 4 {
		t.Errorf("audio %+v", q.Audio)
	}
	// The fake forwards RTP unchanged, so latency is measured on both kinds.
	if !q.Audio.LatencyAvailable || !q.Video.LatencyAvailable {
		t.Errorf("latency: audio %v (%q), video %v (%q)", q.Audio.LatencyAvailable, q.Audio.LatencyReason, q.Video.LatencyAvailable, q.Video.LatencyReason)
	}
	raw, _ := json.Marshal(q)
	var decoded any
	json.Unmarshal(raw, &decoded)
	got := map[string]bool{}
	jsonPaths("", decoded, got)
	// The keys a Jitsi run of the same plan has (006 and 007 contracts).
	for _, k := range strings.Fields(`scenario_id generated_at bots
		video.available video.bots video.latency_available video.latency_seconds.p50 video.frame_rate.p50
		video.freeze_seconds.p50 video.freezes_total video.resolution_switches_total
		video.keyframe_requests_sent_total video.keyframe_requests_received_total video.height_share
		audio.available audio.bots audio.latency_available audio.latency_seconds.p50 audio.concealed_ratio.p50 audio.mos.p50
		worst.lowest_mos worst.most_freeze_seconds worst.highest_latency_p95_seconds worst.lowest_requested_height_share
		server.available server.reason load_machine.engine.cpu_threads.average load_machine.hardware_threads`) {
		if !got[k] {
			t.Errorf("summary has no %s", k)
		}
	}
}
