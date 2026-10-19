//go:build live

// Package live holds tests that run against a real platform deployment. They
// are excluded from the default build; run them explicitly, for example:
//
//	JITSI_ROOM_URL=https://meet.example.org/MyRoom go test -tags live -v ./tests/live/
//
// Deployments that require authentication (meet.jit.si answers SASL with
// "token required") also need a JWT accepted by the deployment, either as the
// jwt query parameter of JITSI_ROOM_URL or in JITSI_TOKEN; it is passed as the token query parameter of the WebSocket URL, as the
// Jitsi web client does. JITSI_XMPP_DOMAIN, JITSI_WS_URL and JITSI_MUC_DOMAIN
// override the derived endpoints. JITSI_LOG_XMPP=1 logs the stanzas.
//
// JITSI_MEDIA (receive_only, audio or audio_video; default receive_only) sets
// what the bots send; when they send, the media metrics must show it.
// JITSI_MEDIA_FILE is a media file the bots send instead of the built-in
// content (transcoded with ffmpeg at the start).
// JITSI_MAX_RECEIVED caps the remote streams each bot receives per kind.
// JITSI_NO_BRIDGE_CHANNEL=1 leaves the bridge channel closed (no video
// requested, no audio subscription), as before feature 005.
// The quality summary of the run (feature 006) is logged at the end; with
// media, the audio quality must meet 006 SC-001 (concealed p95 under 1%,
// median MOS at least 4.0), and with JITSI_EXPECT_VIDEO also the video
// quality (median frame rate 27–33, height 360, no freezes during the hold).
// JITSI_SIMULCAST (360 or 720) makes the bots send three video layers up to
// that height; JITSI_REQUESTED_HEIGHTS ("360:0.5,90:0.5") sets the heights
// they ask for, by shares in launch order. With either and
// JITSI_EXPECT_VIDEO, every bot must receive its height at least 95% of
// the time (006 SC-005).
// JITSI_METRICS_ADDR (e.g. 127.0.0.1:9100) serves /metrics and
// /debug/pprof/ during the run.
// JITSI_SERVER_CONTAINERS (comma-separated, e.g.
// "docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1") names the local
// containers whose CPU and memory the engine samples, every
// JITSI_SERVER_INTERVAL seconds (default 5); the summary's server and
// load_machine sections (feature 007) are logged with it.
//
// JITSI_PARTICIPANTS (default 1) bots join over JITSI_RAMP_UP seconds
// (default 0) and must all hold an established transport for JITSI_HOLD
// (default 10s). Jicofo only starts a conference with at least two
// participants by default, so an otherwise empty room needs two bots. Only
// run more than one bot against a deployment you operate, e.g. the local
// docker-jitsi-meet:
//
//	JITSI_ROOM_URL=http://localhost:8000/loadtest JITSI_XMPP_DOMAIN=meet.jitsi \
//	JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
//	JITSI_PARTICIPANTS=10 go test -tags live -count=1 -v ./tests/live/
package live

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/platform/jitsi"
	"webrtc-load-engine/src/scenario"
)

// TestJitsiLive_JoinAndEstablishMedia runs the execution engine with
// JITSI_PARTICIPANTS bots in JITSI_ROOM_URL and checks, through the webrtc_peers metric, that the
// Jingle negotiation with the bridge reaches an established ICE+DTLS
// transport that holds, and that the bots then leave cleanly.
func TestJitsiLive_JoinAndEstablishMedia(t *testing.T) {
	raw := os.Getenv("JITSI_ROOM_URL")
	if raw == "" {
		t.Skip("JITSI_ROOM_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || strings.Trim(u.Path, "/") == "" {
		t.Fatalf("JITSI_ROOM_URL must look like https://<domain>/[<path>/]<room>, got %q", raw)
	}
	domain := envOr("JITSI_XMPP_DOMAIN", u.Hostname())
	// The room is the last path segment (the web app may live under a path,
	// e.g. /meet/). Jitsi web clients lowercase it in the MUC JID.
	room := strings.ToLower(path.Base(u.Path))

	// Defaults follow a standard single-tenant deployment; multi-tenant ones
	// set JITSI_WS_URL and JITSI_MUC_DOMAIN from their config.js.
	wsURL := envOr("JITSI_WS_URL", "wss://"+u.Host+"/xmpp-websocket") + "?room=" + url.QueryEscape(room)
	mucDomain := envOr("JITSI_MUC_DOMAIN", "conference."+domain)
	token := os.Getenv("JITSI_TOKEN")
	if token == "" {
		token = u.Query().Get("jwt") // Room links of token-auth deployments carry it
	}
	if token != "" {
		wsURL += "&token=" + url.QueryEscape(token)
	}

	if addr := os.Getenv("JITSI_METRICS_ADDR"); addr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		srv := &http.Server{Addr: addr, Handler: mux}
		go srv.ListenAndServe()
		defer srv.Close()
	}

	participants := envInt(t, "JITSI_PARTICIPANTS", 1)
	rampUp := envInt(t, "JITSI_RAMP_UP", 0)
	hold, err := time.ParseDuration(envOr("JITSI_HOLD", "10s"))
	if err != nil {
		t.Fatalf("JITSI_HOLD: %v", err)
	}

	level := slog.LevelInfo
	if os.Getenv("JITSI_LOG_XMPP") != "" {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	noBridgeChannel := os.Getenv("JITSI_NO_BRIDGE_CHANNEL") != ""
	adapter := jitsi.NewAdapter(logger)
	config := map[string]any{
		"xmpp_domain":       domain,
		"muc_domain":        mucDomain,
		"room_name":         room,
		"websocket_url":     wsURL,
		"connect_timeout":   "20s",
		"debug":             os.Getenv("JITSI_LOG_XMPP") != "",
		"no_bridge_channel": noBridgeChannel,
	}
	if err := adapter.ParseConfig(config); err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Run the real engine, as the HTTP API and jitsi-connect do.
	scenarioID := "live-" + uuid.New().String()[:8]
	sc, err := scenario.NewScenario("live", "jitsi", scenario.TestPlan{
		Participants: participants, RampUpSeconds: rampUp, DurationSeconds: 3600,
		Media: &scenario.MediaConfig{
			Profile:            envOr("JITSI_MEDIA", scenario.MediaProfileReceiveOnly),
			MaxReceivedStreams: envInt(t, "JITSI_MAX_RECEIVED", 0),
			Simulcast:          simulcastConfig(t),
			RequestedHeights:   requestedHeights(t),
			SourceFile:         os.Getenv("JITSI_MEDIA_FILE"),
		},
		ServerMonitoring: serverMonitoring(t),
	})
	if err != nil {
		t.Fatalf("test plan: %v", err)
	}
	plan := sc.TestPlan
	kinds := plan.Media.Kinds()
	done := make(chan string, 1)
	var result scenario.RunResult
	engine := execution.NewEngine(scenarioID, plan, adapter, logger, func(_ string, r scenario.RunResult) {
		result = r
		done <- r.Status
	})
	// checkQuality is set once the run's expectations are known; it checks
	// the quality summary after the run.
	checkQuality := func(*scenario.QualitySummary) {}
	start := time.Now()
	go engine.Start()
	defer func() {
		engine.Stop()
		if status := <-done; status != "stopped" {
			t.Errorf("expected the run to end stopped, got %s", status)
		}
		writeReport(t, sc, scenarioID, config, result)
		checkQuality(result.Quality)
		if n := peers(t, scenarioID, execution.PeerStateDisconnected); int(n) != participants {
			t.Errorf("expected %d bots to end disconnected, got %v", participants, n)
		}
	}()

	// Each bot joins, Jicofo sends session-initiate, the peer answers and the
	// ICE+DTLS transport to the bridge must come up.
	deadline := start.Add(time.Duration(rampUp)*time.Second + 60*time.Second)
	for {
		established := int(peers(t, scenarioID, execution.PeerStateEstablished))
		if established == participants {
			break
		}
		if failed := peers(t, scenarioID, execution.PeerStateFailed); failed > 0 {
			t.Fatalf("%v bots failed (%d established)", failed, established)
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d/%d bots established after %s", established, participants, time.Since(start).Round(time.Second))
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Logf("%d bots established with the bridge in %s", participants, time.Since(start).Round(time.Millisecond))

	// Each bot opens the bridge channel, which carries its receive
	// preferences (feature 005).
	expectVideo := os.Getenv("JITSI_EXPECT_VIDEO") != ""
	if noBridgeChannel && expectVideo {
		t.Fatal("JITSI_EXPECT_VIDEO needs the bridge channel: unset JITSI_NO_BRIDGE_CHANNEL")
	}
	for until := time.Now().Add(10 * time.Second); !noBridgeChannel; time.Sleep(200 * time.Millisecond) {
		open := counter(t, "webrtc_data_channels_open", scenarioID, "")
		if int(open) == participants || time.Now().After(until) {
			t.Logf("%.0f/%d bots with an open bridge channel", open, participants)
			if expectVideo && int(open) != participants {
				t.Errorf("expected every bot to open the bridge channel, got %.0f/%d", open, participants)
			}
			break
		}
	}

	// Bots receive the video of `want` others: at least the other bots
	// (more if others are in the room), up to the receive cap.
	want := participants - 1
	if limit := plan.Media.MaxReceivedStreams; limit > 0 && limit < want {
		want = limit
	}
	checkVideo := expectVideo && participants > 1 && slices.Contains(kinds, "video")
	holdFreezes := -1.0 // Video freezes counted during the hold
	checkQuality = func(q *scenario.QualitySummary) {
		single := plan.Media.Simulcast == nil && plan.Media.RequestedHeights == nil
		qualityChecks(t, q, participants > 1 && len(kinds) > 0, checkVideo, single, holdFreezes)
		if checkVideo && !single {
			requestedHeightChecks(t, q)
		}
	}

	// The bridge raises its bandwidth estimate for a few seconds after the
	// bots join, so video starts below the requested volume. Wait for it to
	// settle before the measured window.
	if checkVideo {
		waitVideoSettled(t, scenarioID, participants, want, plan.Media.Simulcast != nil)
	}

	// Hold, checking that no transport drops.
	sentBefore := map[string]float64{}
	recvBefore := map[string]float64{}
	for _, kind := range kinds {
		sentBefore[kind] = counter(t, "webrtc_media_packets_sent_total", scenarioID, kind)
	}
	for _, kind := range []string{"audio", "video"} {
		recvBefore[kind] = counter(t, "webrtc_media_packets_received_total", scenarioID, kind)
	}
	var videoSentBefore float64
	if slices.Contains(kinds, "video") {
		videoSentBefore = sentBefore["video"]
	}
	freezesBefore := counter(t, "webrtc_media_freezes_total", scenarioID, "")
	holdStart := time.Now()
	for end := time.Now().Add(hold); time.Now().Before(end); time.Sleep(time.Second) {
		if n := int(peers(t, scenarioID, execution.PeerStateEstablished)); n != participants {
			t.Fatalf("%d/%d bots still established after %s", n, participants, time.Since(start).Round(time.Second))
		}
	}
	t.Logf("all %d bots held their transport for %s", participants, hold)

	// Sending bots kept sending during the hold.
	for _, kind := range kinds {
		sent := counter(t, "webrtc_media_packets_sent_total", scenarioID, kind) - sentBefore[kind]
		t.Logf("%s: %.0f packets sent during the hold (%.0f/s per bot)", kind, sent, sent/hold.Seconds()/float64(participants))
		if sent == 0 {
			t.Errorf("no %s packets sent during the hold", kind)
		}
	}

	// What the bots received (media metrics, updated every 2 s).
	time.Sleep(2500 * time.Millisecond)
	window := time.Since(holdStart).Seconds()
	holdFreezes = counter(t, "webrtc_media_freezes_total", scenarioID, "") - freezesBefore
	for _, kind := range []string{"audio", "video"} {
		recv := counter(t, "webrtc_media_packets_received_total", scenarioID, kind) - recvBefore[kind]
		lost := counter(t, "webrtc_media_packets_lost_total", scenarioID, kind)
		streams := counter(t, "webrtc_media_remote_streams", scenarioID, kind)
		stalls := counter(t, "webrtc_media_stalls_total", scenarioID, kind)
		t.Logf("%s: %.1f remote streams offered per bot, %.0f packets received during the hold (%.0f/s per bot), %.0f lost, %.0f stalls",
			kind, streams/float64(participants), recv, recv/window/float64(participants), lost, stalls)
		if stalls > 0 {
			t.Errorf("%s: %.0f stalls", kind, stalls)
		}
	}
	// Traffic from the server beyond the media the bots keep (005 SC-003):
	// close to the RTP/SRTP/RTCP overhead when the server sends only the
	// received streams.
	transport := counter(t, "webrtc_transport_bytes_received_total", scenarioID, "")
	kept := counter(t, "webrtc_media_bytes_received_total", scenarioID, "audio") + counter(t, "webrtc_media_bytes_received_total", scenarioID, "video")
	if kept > 0 {
		t.Logf("transport: %.1f MB received by the bots over the run, %.1f MB of media payload kept, ratio %.2f",
			transport/1e6, kept/1e6, transport/kept)
	}

	// Every bot is offered the other bots' audio, and the bridge forwards
	// audio without receiver constraints. Video is only forwarded once a
	// receiver sends its constraints over the bridge channel, which not every
	// deployment allows (JITSI_EXPECT_VIDEO=1 to require it).
	if participants > 1 && len(kinds) > 0 {
		streams := counter(t, "webrtc_media_remote_streams", scenarioID, "audio")
		if int(streams) < participants*want {
			t.Errorf("expected at least %d remote audio streams per bot, got %.1f", want, streams/float64(participants))
		}
		if limit := plan.Media.MaxReceivedStreams; limit > 0 && int(streams) > participants*limit {
			t.Errorf("expected at most %d remote audio streams per bot (cap), got %.1f", limit, streams/float64(participants))
		}
		if counter(t, "webrtc_media_packets_received_total", scenarioID, "audio")-recvBefore["audio"] == 0 {
			t.Error("bots received no audio")
		}
		if expectVideo {
			recv := counter(t, "webrtc_media_packets_received_total", scenarioID, "video") - recvBefore["video"]
			if recv == 0 {
				t.Error("bots received no video")
			}
			// Each bot receives the video of `want` others: within 10% of
			// what one bot sends (SC-001 of 005). With simulcast a bot
			// receives one of the layers sent, by the height it asked
			// for: the quality checks cover it.
			if checkVideo && plan.Media.Simulcast == nil {
				sent := counter(t, "webrtc_media_packets_sent_total", scenarioID, "video") - videoSentBefore
				sentPerBot := sent / float64(participants) / window
				recvPerBot := recv / float64(participants) / window
				ratio := recvPerBot / (float64(want) * sentPerBot)
				t.Logf("video: %.0f packets/s received per bot, %.2f of %d × %.0f/s sent", recvPerBot, ratio, want, sentPerBot)
				if ratio < 0.9 || ratio > 1.1 {
					t.Errorf("video received per bot is %.2f of the expected volume, want within 10%%", ratio)
				}
			}
		}
	}
}

// qualityChecks logs the quality summary of the run and checks it (006
// SC-001): audio when the bots receive audio, video when checkVideo.
//
// Freezes are checked over the hold (holdFreezes), after video settled: when
// many bots join at once, the bridge forwards the first keyframe of some
// streams and holds them until its bandwidth estimate allows them, which
// the summary counts as freezes of the start-up.
//
// single: the bots send and ask for one 360-line stream, so all received
// video must be at 360.
func qualityChecks(t *testing.T, q *scenario.QualitySummary, checkAudio, checkVideo, single bool, holdFreezes float64) {
	t.Helper()
	if q == nil {
		t.Error("no quality summary")
		return
	}
	raw, _ := json.MarshalIndent(q, "", "  ")
	t.Logf("quality summary:\n%s", raw)
	if checkAudio {
		a := q.Audio
		switch {
		case !a.Available || a.ConcealedRatio == nil || a.MOS == nil:
			t.Errorf("no audio quality: %+v", a)
		default:
			if a.ConcealedRatio.P95 >= 0.01 {
				t.Errorf("audio concealed ratio p95 %.4f, want under 1%%", a.ConcealedRatio.P95)
			}
			if a.MOS.P50 < 4.0 {
				t.Errorf("median MOS %.2f, want at least 4.0", a.MOS.P50)
			}
		}
	}
	if checkVideo {
		v := q.Video
		switch {
		case !v.Available || v.FrameRate == nil:
			t.Errorf("no video quality: %+v", v)
		default:
			if v.FrameRate.P50 < 27 || v.FrameRate.P50 > 33 {
				t.Errorf("median frame rate %.1f, want 27–33", v.FrameRate.P50)
			}
			if single && v.HeightShare["360"] < 0.99 {
				t.Errorf("height 360 for %.1f%% of the received time, want 100%%", 100*v.HeightShare["360"])
			}
			t.Logf("video freezes: %.0f during the hold, %d over the whole run", holdFreezes, v.FreezesTotal)
			if holdFreezes != 0 {
				t.Errorf("%.0f freezes during the hold, want none", holdFreezes)
			}
		}
	}
}

// requestedHeightChecks checks that every bot received the video height it
// asked for at least 95% of its received video time (006 SC-005).
func requestedHeightChecks(t *testing.T, q *scenario.QualitySummary) {
	t.Helper()
	v := q.Video
	if v.RequestedHeightShare == nil {
		t.Error("no requested height share in the summary")
		return
	}
	t.Logf("received video time per height %v, %d switches; share at the requested height p50 %.3f",
		v.HeightShare, v.ResolutionSwitchesTotal, v.RequestedHeightShare.P50)
	for _, b := range q.Worst.LowestRequestedHeight {
		t.Logf("  %s: %.3f of its video at the height it asked for", b.PeerID, b.Value)
		if b.Value < 0.95 {
			t.Errorf("%s received the height it asked for %.1f%% of the time, want at least 95%%", b.PeerID, 100*b.Value)
		}
	}
}

// simulcastConfig returns the simulcast setting from JITSI_SIMULCAST (360
// or 720: the highest layer), nil when unset.
func simulcastConfig(t *testing.T) *scenario.SimulcastConfig {
	t.Helper()
	v := os.Getenv("JITSI_SIMULCAST")
	if v == "" {
		return nil
	}
	h, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("JITSI_SIMULCAST must be 360 or 720, got %q", v)
	}
	return &scenario.SimulcastConfig{MaxHeight: h}
}

// serverMonitoring returns the server containers of JITSI_SERVER_CONTAINERS
// and JITSI_SERVER_INTERVAL, nil when unset.
func serverMonitoring(t *testing.T) *scenario.ServerMonitoringConfig {
	t.Helper()
	v := os.Getenv("JITSI_SERVER_CONTAINERS")
	if v == "" {
		return nil
	}
	return &scenario.ServerMonitoringConfig{
		Containers:      strings.Split(v, ","),
		IntervalSeconds: envInt(t, "JITSI_SERVER_INTERVAL", 0),
	}
}

// requestedHeights parses JITSI_REQUESTED_HEIGHTS ("height:share,..."),
// nil when unset.
func requestedHeights(t *testing.T) []scenario.RequestedHeight {
	t.Helper()
	v := os.Getenv("JITSI_REQUESTED_HEIGHTS")
	if v == "" {
		return nil
	}
	var out []scenario.RequestedHeight
	for _, part := range strings.Split(v, ",") {
		hs, ss, ok := strings.Cut(part, ":")
		h, err1 := strconv.Atoi(hs)
		s, err2 := strconv.ParseFloat(ss, 64)
		if !ok || err1 != nil || err2 != nil {
			t.Fatalf("JITSI_REQUESTED_HEIGHTS: expected height:share,..., got %q", v)
		}
		out = append(out, scenario.RequestedHeight{Height: h, Share: s})
	}
	return out
}

// waitVideoSettled waits, up to 30 s, until the video the bots receive over
// one collection period (2 s) is at least 90% of `want` times what one bot
// sends, and logs how long it took. With simulcast, where each bot receives
// only some of the layers sent, it waits until the received volume is
// within 5% of the previous period's instead. It does not fail: the
// measured window that follows does.
func waitVideoSettled(t *testing.T, scenarioID string, participants, want int, simulcast bool) {
	t.Helper()
	const step = 2 * time.Second
	start := time.Now()
	sample := func() (sent, recv float64) {
		return counter(t, "webrtc_media_packets_sent_total", scenarioID, "video"),
			counter(t, "webrtc_media_packets_received_total", scenarioID, "video")
	}
	sent, recv := sample()
	ratio, prev := 0.0, 0.0
	for time.Since(start) < 30*time.Second {
		time.Sleep(step)
		s, r := sample()
		settled := false
		if simulcast {
			if prev > 0 {
				ratio = (r - recv) / prev
				settled = ratio >= 0.95 && ratio <= 1.05
			}
			prev = r - recv
		} else if s > sent {
			ratio = (r - recv) / (float64(want) * (s - sent))
			settled = ratio >= 0.9
		}
		sent, recv = s, r
		if settled {
			t.Logf("video settled at %.2f of the expected volume after %s", ratio, time.Since(start).Round(time.Second))
			return
		}
	}
	t.Logf("video not settled after %s (%.2f of the expected volume)", time.Since(start).Round(time.Second), ratio)
}

func envInt(t *testing.T, key string, fallback int) int {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		t.Fatalf("%s must be a non-negative integer, got %q", key, v)
	}
	return n
}

// peers reads the webrtc_peers gauge for a scenario and state.
func peers(t *testing.T, scenarioID string, state execution.PeerState) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, f := range families {
		if f.GetName() != "webrtc_peers" {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["scenario_id"] == scenarioID && labels["state"] == string(state) {
				return m.GetGauge().GetValue()
			}
		}
	}
	return 0
}

// counter reads a counter (or gauge) with labels scenario_id and kind.
func counter(t *testing.T, name, scenarioID, kind string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["scenario_id"] == scenarioID && labels["kind"] == kind {
				if c := m.GetCounter(); c != nil {
					return c.GetValue()
				}
				return m.GetGauge().GetValue()
			}
		}
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
