package jitsi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	// "gosrc.io/xmpp" // Not directly used here anymore, used in signaling.go
	//"webrtc-load-engine/src/execution" // Removed to break import cycle
	"webrtc-load-engine/src/platform"
)

// Adapter implements the PlatformAdapter interface for Jitsi.
type Adapter struct {
	logger *slog.Logger
	config JingleConfig
	
	clients map[string]*SignalingClient
	mu      sync.Mutex
}

// JingleConfig holds the configuration for the Jitsi adapter.
// It matches the schema defined in specs/003-xmpp-signaling/contracts/configuration.yaml.
type JingleConfig struct {
	XMPPDomain     string `json:"xmpp_domain"`
	MUCDomain      string `json:"muc_domain"`
	RoomName       string `json:"room_name"`
	WebSocketURL   string `json:"websocket_url"`
	ConnectTimeout string `json:"connect_timeout"`
	Debug          bool   `json:"debug"`
}

// NewAdapter creates a new Jitsi adapter.
func NewAdapter(logger *slog.Logger) platform.PlatformAdapter {
	if logger == nil {
		logger = slog.Default()
	}
	return &Adapter{
		logger:  logger,
		clients: make(map[string]*SignalingClient),
	}
}

// ParseConfig validates and extracts the platform configuration.
func (a *Adapter) ParseConfig(config map[string]interface{}) error {
	// Convert map to JSON bytes then unmarshal into struct
	// This is a common way to handle generic map decoding in Go
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := json.Unmarshal(data, &a.config); err != nil {
		return fmt.Errorf("failed to parse jitsi config: %w", err)
	}

	// Validate required fields
	if a.config.XMPPDomain == "" {
		return fmt.Errorf("xmpp_domain is required")
	}
	if a.config.RoomName == "" {
		return fmt.Errorf("room_name is required")
	}

	// Set defaults
	if a.config.MUCDomain == "" {
		a.config.MUCDomain = fmt.Sprintf("conference.%s", a.config.XMPPDomain)
	}
	if a.config.WebSocketURL == "" {
		a.config.WebSocketURL = fmt.Sprintf("wss://%s/xmpp-websocket", a.config.XMPPDomain)
	}
	if a.config.ConnectTimeout == "" {
		a.config.ConnectTimeout = "10s"
	}

	a.logger.Info("Jitsi configuration parsed successfully",
		slog.String("xmpp_domain", a.config.XMPPDomain),
		slog.String("room", a.config.RoomName),
		slog.String("muc_domain", a.config.MUCDomain),
		slog.String("websocket_url", a.config.WebSocketURL))

	return nil
}

// Connect simulates connecting a peer to a Jitsi meeting.
func (a *Adapter) Connect(peerID string, handler platform.SignalingHandler) error {
	a.logger.Info("Jitsi adapter: Connecting peer", slog.String("peer_id", peerID))

	client := NewSignalingClient(a.logger, a.config)
	client.SetHandler(handler)
	
	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second) // TODO: Use config timeout
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect XMPP: %w", err)
	}

	if err := client.JoinMUC(ctx, peerID); err != nil {
		// Cleanup if join fails
		// client.Close() // TODO: Implement Close
		return fmt.Errorf("failed to join MUC: %w", err)
	}

	// Store client
	a.mu.Lock()
	a.clients[peerID] = client
	a.mu.Unlock()
	
	// Wait for Join to complete (optional but good for 'Connected' state)
	// We run this in background or block? Adapter.Connect is expected to block until connected?
	// Usually Connect returns when 'Connected'.
	if err := client.WaitJoined(ctx); err != nil {
		return fmt.Errorf("timed out waiting for MUC join: %w", err)
	}

	return nil
}

// Disconnect simulates disconnecting a peer from a Jitsi meeting.
func (a *Adapter) Disconnect(peerID string) error {
	a.logger.Info("Jitsi adapter: Disconnecting peer", slog.String("peer_id", peerID))
	
	a.mu.Lock()
	client, ok := a.clients[peerID]
	delete(a.clients, peerID)
	a.mu.Unlock()

	if !ok {
		return nil // Already disconnected
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	client.TerminateSessions(ctx)
	
	if err := client.Disconnect(); err != nil {
		a.logger.Error("Failed to disconnect XMPP client", slog.String("error", err.Error()))
		return err
	}
	
	return nil
}

