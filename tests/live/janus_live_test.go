//go:build live

package live

// TestJanusLive_JoinAndEstablishMedia runs the execution engine against a
// Janus VideoRoom (feature 011), as the other platforms' live tests do, with
// the same checks and the same quality summary:
//
//	JANUS_URL=ws://localhost:8188 JANUS_ROOM=1011 JANUS_PARTICIPANTS=10 \
//	JANUS_MEDIA=audio_video go test -tags live -count=1 -v -run Janus ./tests/live/
//
// JANUS_URL and JANUS_ROOM (a number, default 1011) are the platform_config;
// JANUS_API_SECRET, JANUS_TOKEN, JANUS_PIN and JANUS_ADMIN_KEY set its
// secrets and JANUS_INSECURE_TLS=1 its insecure_tls. JANUS_PARTICIPANTS
// (default 2) bots join over JANUS_RAMP_UP seconds (default 0), send
// JANUS_MEDIA (receive_only, audio or audio_video; default audio_video),
// receive at most JANUS_MAX_RECEIVED streams per kind (0: all) and must all
// hold an established transport for JANUS_HOLD (default 10s).
// JANUS_SERVER_CONTAINERS (e.g. janus) and JANUS_SERVER_INTERVAL name the
// containers whose load the summary reports (feature 007).
// JANUS_METRICS_ADDR (e.g. 127.0.0.1:9100) serves /metrics and
// /debug/pprof/ during the run. JANUS_LEAVE_ONE=1 adds a bot outside the
// engine that joins first and leaves halfway through the hold: the bots must
// replace its streams by waiting ones (receive cap, 011 FR-006).
//
// Janus rooms keep the size they were created with: use a new JANUS_ROOM
// when JANUS_PARTICIPANTS grows (research R4).

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/platform/catalog"
	"webrtc-load-engine/src/platform/janus"
	"webrtc-load-engine/src/scenario"
)

func TestJanusLive_JoinAndEstablishMedia(t *testing.T) {
	if os.Getenv("JANUS_URL") == "" {
		t.Skip("JANUS_URL not set")
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if addr := os.Getenv("JANUS_METRICS_ADDR"); addr != "" {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		srv := &http.Server{Addr: addr, Handler: mux}
		go srv.ListenAndServe()
		defer srv.Close()
	}
	participants := envInt(t, "JANUS_PARTICIPANTS", 2)
	rampUp := envInt(t, "JANUS_RAMP_UP", 0)
	hold, err := time.ParseDuration(envOr("JANUS_HOLD", "10s"))
	if err != nil {
		t.Fatalf("JANUS_HOLD: %v", err)
	}
	config := map[string]any{
		"url":  os.Getenv("JANUS_URL"),
		"room": envOr("JANUS_ROOM", "1011"),
	}
	for env, key := range map[string]string{"JANUS_API_SECRET": "api_secret", "JANUS_TOKEN": "token", "JANUS_PIN": "pin", "JANUS_ADMIN_KEY": "admin_key"} {
		if v := os.Getenv(env); v != "" {
			config[key] = v
		}
	}
	if os.Getenv("JANUS_INSECURE_TLS") != "" {
		config["insecure_tls"] = true
	}
	var monitoring *scenario.ServerMonitoringConfig
	if v := os.Getenv("JANUS_SERVER_CONTAINERS"); v != "" {
		monitoring = &scenario.ServerMonitoringConfig{Containers: strings.Split(v, ","), IntervalSeconds: envInt(t, "JANUS_SERVER_INTERVAL", 0)}
	}

	// The plan goes through the same creation-time rules and adapter as the
	// HTTP API.
	cat := catalog.New(logger)
	sc, err := scenario.NewScenario("live-janus", "janus", scenario.TestPlan{
		Participants: participants, RampUpSeconds: rampUp, DurationSeconds: 3600,
		PlatformConfig: config,
		Media: &scenario.MediaConfig{
			Profile:            envOr("JANUS_MEDIA", scenario.MediaProfileAudioVideo),
			MaxReceivedStreams: envInt(t, "JANUS_MAX_RECEIVED", 0),
		},
		ServerMonitoring: monitoring,
	})
	if err != nil {
		t.Fatalf("test plan: %v", err)
	}
	plan := sc.TestPlan
	if err := cat.ValidateTestPlan("janus", plan); err != nil {
		t.Fatalf("test plan rejected: %v", err)
	}
	adapter, err := cat.NewAdapter("janus", plan)
	if err != nil {
		t.Fatalf("adapter: %v", err)
	}
	kinds := plan.Media.Kinds()

	// The extra bot joins before the engine's, so every bot subscribes to
	// it first (earliest), then loses it halfway.
	leaveOne := os.Getenv("JANUS_LEAVE_ONE") != ""
	var extraLeave func()
	if leaveOne {
		extraLeave = joinExtraJanusBot(t, logger, config, participants+1)
	}

	scenarioID := "live-" + uuid.New().String()[:8]
	done := make(chan string, 1)
	var result scenario.RunResult
	engine := execution.NewEngine(scenarioID, plan, adapter, logger, func(_ string, r scenario.RunResult) {
		result = r
		done <- r.Status
	})
	checkVideo := participants > 1 && slices.Contains(kinds, "video")
	holdFreezes := -1.0
	start := time.Now()
	go engine.Start()
	defer func() {
		engine.Stop()
		stopped := time.Now()
		if status := <-done; status != "stopped" {
			t.Errorf("expected the run to end stopped, got %s", status)
		}
		writeReport(t, sc, scenarioID, config, result)
		quality := result.Quality
		t.Logf("bots left in %s", time.Since(stopped).Round(time.Millisecond))
		if time.Since(stopped) > 10*time.Second {
			t.Errorf("bots took %s to leave, want under 10 s (SC-001)", time.Since(stopped).Round(time.Second))
		}
		qualityChecks(t, quality, participants > 1 && len(kinds) > 0, checkVideo, true, holdFreezes)
		if n := peers(t, scenarioID, execution.PeerStateDisconnected); int(n) != participants {
			t.Errorf("expected %d bots to end disconnected, got %v", participants, n)
		}
	}()

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
	t.Logf("%d bots established with Janus in %s", participants, time.Since(start).Round(time.Millisecond))

	others := participants - 1
	if leaveOne {
		others++
	}
	want := others
	if limit := plan.Media.MaxReceivedStreams; limit > 0 && limit < want {
		want = limit
	}
	if checkVideo {
		waitVideoSettled(t, scenarioID, participants, want, false)
	}

	recvBefore := map[string]float64{}
	for _, kind := range []string{"audio", "video"} {
		recvBefore[kind] = counter(t, "webrtc_media_packets_received_total", scenarioID, kind)
	}
	freezesBefore := counter(t, "webrtc_media_freezes_total", scenarioID, "")
	holdStart := time.Now()
	halfway := holdStart.Add(hold / 2)
	for end := time.Now().Add(hold); time.Now().Before(end); time.Sleep(time.Second) {
		if n := int(peers(t, scenarioID, execution.PeerStateEstablished)); n != participants {
			t.Fatalf("%d/%d bots still established after %s", n, participants, time.Since(start).Round(time.Second))
		}
		if extraLeave != nil && time.Now().After(halfway) {
			extraLeave()
			extraLeave = nil
			t.Log("the extra bot left")
		}
	}
	t.Logf("all %d bots held their transport for %s", participants, hold)
	time.Sleep(2500 * time.Millisecond) // Media metrics are collected every 2 s
	window := time.Since(holdStart).Seconds()
	holdFreezes = counter(t, "webrtc_media_freezes_total", scenarioID, "") - freezesBefore

	if len(kinds) > 0 && participants > 1 {
		if leaveOne {
			want = min(want, participants-1) // The extra bot is gone
		}
		for _, kind := range []string{"audio", "video"} {
			streams := counter(t, "webrtc_media_remote_streams", scenarioID, kind)
			recv := counter(t, "webrtc_media_packets_received_total", scenarioID, kind) - recvBefore[kind]
			t.Logf("%s: %.1f remote streams offered per bot, %.0f packets/s received per bot", kind, streams/float64(participants), recv/window/float64(participants))
			if int(streams) != participants*want {
				t.Errorf("%s: %.1f remote streams per bot, want %d", kind, streams/float64(participants), want)
			}
			if recv == 0 {
				t.Errorf("bots received no %s", kind)
			}
		}
	}
}

// joinExtraJanusBot joins a sending bot outside the engine and returns the
// function that makes it leave.
func joinExtraJanusBot(t *testing.T, logger *slog.Logger, config map[string]any, roomSize int) func() {
	t.Helper()
	cfg, err := janus.ParseConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Participants = roomSize // It joins first: the room it creates holds the engine's bots too
	a := janus.NewAdapter(logger, cfg)
	scenarioID := "live-extra-" + uuid.New().String()[:8]
	p, err := execution.NewPeer("extra-bot", scenarioID, execution.PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}, PublishSession: true}, logger)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := a.Connect(ctx, p.ID(), p); err != nil {
		cancel()
		t.Fatalf("extra bot: %v", err)
	}
	for _, kind := range []string{media.KindAudio, media.KindVideo} {
		c, err := media.LoadContent(kind)
		if err != nil {
			t.Fatal(err)
		}
		src := media.NewSource(scenarioID, c)
		src.Subscribe(p.LocalTracks()[kind])
		go src.Run(ctx)
	}
	time.Sleep(2 * time.Second) // Its media flows before the engine's bots join
	left := false
	leave := func() {
		if left {
			return
		}
		left = true
		a.Disconnect(p.ID())
		cancel()
		p.Close()
	}
	t.Cleanup(leave)
	return leave
}
