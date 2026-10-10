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
// JITSI_MAX_RECEIVED caps the remote streams each bot receives per kind.
// JITSI_METRICS_ADDR (e.g. 127.0.0.1:9100) serves /metrics and
// /debug/pprof/ during the run.
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
	adapter := jitsi.NewAdapter(logger)
	if err := adapter.ParseConfig(map[string]interface{}{
		"xmpp_domain":     domain,
		"muc_domain":      mucDomain,
		"room_name":       room,
		"websocket_url":   wsURL,
		"connect_timeout": "20s",
		"debug":           os.Getenv("JITSI_LOG_XMPP") != "",
	}); err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}

	// Run the real engine, as the HTTP API and jitsi-connect do.
	scenarioID := "live-" + uuid.New().String()[:8]
	sc, err := scenario.NewScenario("live", "jitsi", scenario.TestPlan{
		Participants: participants, RampUpSeconds: rampUp, DurationSeconds: 3600,
		Media: &scenario.MediaConfig{
			Profile:            envOr("JITSI_MEDIA", scenario.MediaProfileReceiveOnly),
			MaxReceivedStreams: envInt(t, "JITSI_MAX_RECEIVED", 0),
		},
	})
	if err != nil {
		t.Fatalf("test plan: %v", err)
	}
	plan := sc.TestPlan
	kinds := plan.Media.Kinds()
	done := make(chan string, 1)
	engine := execution.NewEngine(scenarioID, plan, adapter, logger, func(_ string, status string) { done <- status })
	start := time.Now()
	go engine.Start()
	defer func() {
		engine.Stop()
		if status := <-done; status != "stopped" {
			t.Errorf("expected the run to end stopped, got %s", status)
		}
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
	for until := time.Now().Add(10 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		open := counter(t, "webrtc_data_channels_open", scenarioID, "")
		if int(open) == participants || time.Now().After(until) {
			t.Logf("%.0f/%d bots with an open bridge channel", open, participants)
			if expectVideo && int(open) != participants {
				t.Errorf("expected every bot to open the bridge channel, got %.0f/%d", open, participants)
			}
			break
		}
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
		// At least the other bots (more if others are in the room), up to
		// the receive cap.
		want := participants - 1
		if limit := plan.Media.MaxReceivedStreams; limit > 0 && limit < want {
			want = limit
		}
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
			// what one bot sends (SC-001 of 005).
			if slices.Contains(kinds, "video") {
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
