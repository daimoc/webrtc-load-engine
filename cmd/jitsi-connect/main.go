package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/uuid"
	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/platform/jitsi"
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
	}
	if err := adapter.ParseConfig(config); err != nil {
		logger.Error("Failed to parse config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// NewScenario validates the plan and applies the media defaults.
	sc, err := scenario.NewScenario("jitsi-connect", "jitsi", scenario.TestPlan{
		Participants:    *participants,
		DurationSeconds: *duration,
		Media:           &scenario.MediaConfig{Profile: *mediaProfile, MaxReceivedStreams: *maxReceived, SourceFile: *mediaFile},
	})
	if err != nil {
		logger.Error("Invalid flags", slog.String("error", err.Error()))
		os.Exit(1)
	}

	done := make(chan string, 1)
	engine := execution.NewEngine("cli-"+uuid.New().String()[:8], sc.TestPlan, adapter, logger, func(_, status string, _ *scenario.QualitySummary) {
		done <- status
	})
	go engine.Start()

	logger.Info("Bots launched. Press Ctrl+C to exit.", slog.Int("participants", *participants), slog.String("media", *mediaProfile))

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		logger.Info("Disconnecting...")
		engine.Stop()
		<-done
	case status := <-done:
		logger.Info("Run finished", slog.String("status", status))
	}
	logger.Info("Exited.")
}
