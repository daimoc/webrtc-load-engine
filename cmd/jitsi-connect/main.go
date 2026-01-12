package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/platform/jitsi"
)

// DummyHandler implements platform.SignalingHandler for testing.
type DummyHandler struct {
	logger *slog.Logger
}

// Ensure DummyHandler implements platform.SignalingHandler
var _ platform.SignalingHandler = &DummyHandler{}

func (h *DummyHandler) OnOffer(sdp string) (string, error) {
	h.logger.Info("Received Offer", slog.String("sdp_preview", sdp[:min(len(sdp), 50)]+"..."))
	// In a real scenario, we would generate an Answer here.
	return "", nil
}

func (h *DummyHandler) OnICECandidate(candidate string) error {
	h.logger.Info("Received ICE Candidate", slog.String("candidate", candidate))
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func main() {
	// Parse flags
	xmppDomain := flag.String("domain", "meet.jit.si", "XMPP Domain")
	mucDomain := flag.String("muc", "conference.meet.jit.si", "MUC Domain")
	roomName := flag.String("room", "testroom-" + uuid.New().String()[:8], "Room Name")
	wsURL := flag.String("ws", "wss://meet.jit.si/xmpp-websocket", "WebSocket URL")
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
		slog.String("room", *roomName),
		slog.String("ws", *wsURL))

	// Create Adapter
	adapter := jitsi.NewAdapter(logger)

	// Configure Adapter
	config := map[string]interface{}{
		"xmpp_domain":   *xmppDomain,
		"muc_domain":    *mucDomain,
		"room_name":     *roomName,
		"websocket_url": *wsURL,
		"debug":         *debug,
		"log_xmpp":      *logXMPP,
	}

	// The Adapter interface doesn't expose ParseConfig directly in the interface definition 
	// (it's on the concrete struct or via some other mechanism in the real app usually),
	// but looking at src/platform/jitsi/adapter.go, ParseConfig IS a method on *Adapter.
	jitsiAdapter, ok := adapter.(*jitsi.Adapter)
	if !ok {
		logger.Error("Failed to type assert adapter to *jitsi.Adapter")
		os.Exit(1)
	}

	if err := jitsiAdapter.ParseConfig(config); err != nil {
		logger.Error("Failed to parse config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Connect
	peerID := "user-" + uuid.New().String()[:8]
	handler := &DummyHandler{logger: logger}

	if err := adapter.Connect(peerID, handler); err != nil {
		logger.Error("Failed to connect", slog.String("error", err.Error()))
		
		// Check for common error
		if strings.Contains(err.Error(), "no matching authentication") && strings.Contains(err.Error(), "ANONYMOUS") {
			logger.Warn("NOTE: The connection failed because the server requires SASL ANONYMOUS authentication, but the current XMPP library configuration is attempting PLAIN authentication. The gosrc.io/xmpp library v0.5.1 does not natively support ANONYMOUS auth easily.")
		}
		
		os.Exit(1)
	}

	logger.Info("Connected! Press Ctrl+C to exit.")

	// Wait for interrupt
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("Disconnecting...")
	
	// Disconnect
	if err := adapter.Disconnect(peerID); err != nil {
		logger.Error("Failed to disconnect", slog.String("error", err.Error()))
	}
	
	logger.Info("Exited.")
}
