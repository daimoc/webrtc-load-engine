package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/platform/catalog"
	"webrtc-load-engine/src/platform/jitsi"
	"webrtc-load-engine/src/report"
	"webrtc-load-engine/src/scenario"
)

// jitsi-connect joins a Jitsi room with --participants bots (one by default),
// negotiates the Jingle session with the bridge and stays in the room until
// interrupted or until --duration elapses. With --media the bots also send
// audio (and video), from --media-file when set. It runs the same execution engine as the HTTP API.
func main() {
	// Parse flags
	xmppDomain := flag.String("domain", "meet.jit.si", "XMPP Domain")
	mucDomain := flag.String("muc", "", "MUC Domain (default conference.<domain>)")
	roomName := flag.String("room", "testroom-"+uuid.New().String()[:8], "Room Name")
	wsURL := flag.String("ws", "", "WebSocket URL (default wss://<domain>/xmpp-websocket)")
	focusJID := flag.String("focus", "", "Focus component JID (default focus.<domain>)")
	duration := flag.Int("duration", 3600, "Seconds to stay in the room")
	participants := flag.Int("participants", 1, "Number of bots")
	mediaProfile := flag.String("media", scenario.MediaProfileReceiveOnly, "What each bot sends: receive_only, audio or audio_video")
	maxReceived := flag.Int("max-received", 0, "Remote streams each bot receives per media kind (0: no cap)")
	mediaFile := flag.String("media-file", "", "Media file the bots send instead of the built-in content (needs ffmpeg)")
	debug := flag.Bool("debug", false, "Enable debug logging")
	logXMPP := flag.Bool("log-xmpp", false, "Log all XMPP traffic to stdout")
	jwtToken := flag.String("jwt", os.Getenv("JITSI_JWT"), "JWT shared by all bots, for token-authenticated Jitsi (default $JITSI_JWT)")
	jwtAppID := flag.String("jwt-app-id", "", "App id (iss) to sign one JWT per bot, with --jwt-app-secret")
	jwtAppSecret := flag.String("jwt-app-secret", os.Getenv("JITSI_APP_SECRET"), "App secret (HS256) to sign one JWT per bot (default $JITSI_APP_SECRET)")
	jwtModerator := flag.Bool("jwt-moderator", false, "Mark the signed bot tokens as moderator")
	serverContainers := flag.String("server-containers", "", "Comma-separated local Docker containers of the media server to sample for CPU and memory (e.g. docker-jitsi-meet-jvb-1)")
	serverInterval := flag.Int("server-interval", 0, "Seconds between server samples (default 5)")
	reportsDir := flag.String("reports-dir", "", "Folder of the run reports (default $"+report.EnvDir+", else ./reports)")

	flag.Parse()

	// Setup logger
	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	logger.Info("Starting Jitsi Connect Test",
		slog.String("domain", *xmppDomain),
		slog.String("room", *roomName))

	adapter := jitsi.NewAdapter(logger)
	config := map[string]interface{}{
		"xmpp_domain":   *xmppDomain,
		"muc_domain":    *mucDomain,
		"room_name":     *roomName,
		"websocket_url": *wsURL,
		"focus_jid":     *focusJID,
		"debug":         *logXMPP,
		"jwt_moderator": *jwtModerator,
	}
	// The secret may come from the environment; only use it when signing.
	if *jwtAppID != "" {
		config["jwt_app_id"] = *jwtAppID
		config["jwt_app_secret"] = *jwtAppSecret
	} else if *jwtToken != "" {
		config["jwt"] = *jwtToken
	}
	if err := adapter.ParseConfig(config); err != nil {
		logger.Error("Failed to parse config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	var monitoring *scenario.ServerMonitoringConfig
	if *serverContainers != "" {
		monitoring = &scenario.ServerMonitoringConfig{Containers: strings.Split(*serverContainers, ","), IntervalSeconds: *serverInterval}
	}

	// NewScenario validates the plan and applies the media defaults.
	sc, err := scenario.NewScenario("jitsi-connect", "jitsi", scenario.TestPlan{
		Participants:     *participants,
		DurationSeconds:  *duration,
		Media:            &scenario.MediaConfig{Profile: *mediaProfile, MaxReceivedStreams: *maxReceived, SourceFile: *mediaFile},
		ServerMonitoring: monitoring,
	})
	if err != nil {
		logger.Error("Invalid flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	sc.ID = "cli-" + uuid.New().String()[:8]
	done := make(chan scenario.RunResult, 1)
	engine := execution.NewEngine(sc.ID, sc.TestPlan, adapter, logger, func(_ string, r scenario.RunResult) {
		done <- r
	})
	go engine.Start()

	logger.Info("Bots launched. Press Ctrl+C to exit.", slog.Int("participants", *participants), slog.String("media", *mediaProfile))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	var result scenario.RunResult
	select {
	case <-stop:
		logger.Info("Disconnecting...")
		engine.Stop()
		result = <-done
	case result = <-done:
		logger.Info("Run finished", slog.String("status", result.Status))
	}
	printLoad(result.Quality)
	writeReport(logger, *reportsDir, sc, config, result)
	logger.Info("Exited.")
}

// writeReport writes the run's report files (feature 009); the report's
// test plan has the adapter's config, its secrets hidden. The writer logs
// where the files went, or why not.
func writeReport(logger *slog.Logger, reportsDir string, sc *scenario.Scenario, config map[string]any, r scenario.RunResult) {
	dir, err := report.ResolveDir(reportsDir)
	if err != nil {
		logger.Warn("Run report not written", slog.String("error", err.Error()))
		return
	}
	reported := *sc
	reported.TestPlan.PlatformConfig = config
	reported.TestPlan = scenario.RedactTestPlan(reported.TestPlan, catalog.New(logger).SecretKeys("jitsi"))
	report.Writer{Dir: dir, Logger: logger}.WriteReport(&reported, r)
}

// printLoad prints the server and load-machine sections of the run's
// summary (feature 007).
func printLoad(q *scenario.QualitySummary) {
	if q == nil {
		return
	}
	raw, err := json.MarshalIndent(struct {
		Server      scenario.ServerLoad  `json:"server"`
		LoadMachine scenario.LoadMachine `json:"load_machine"`
	}{q.Server, q.LoadMachine}, "", "  ")
	if err != nil {
		return
	}
	fmt.Println(string(raw))
}
