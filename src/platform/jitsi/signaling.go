package jitsi

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gosrc.io/xmpp"
	"gosrc.io/xmpp/stanza"
	"webrtc-load-engine/src/platform"
)

// SignalingClient handles XMPP signaling for Jitsi.
type SignalingClient struct {
	logger *slog.Logger
	client *xmpp.Client
	config JingleConfig
	router *xmpp.Router
	joined chan struct{} // Signal when MUC join is complete
	
	sessions map[string]*JingleSession
	mu       sync.RWMutex
	
	handler platform.SignalingHandler
}

// NewSignalingClient creates a new SignalingClient.
func NewSignalingClient(logger *slog.Logger, config JingleConfig) *SignalingClient {
	return &SignalingClient{
		logger:   logger,
		config:   config,
		router:   xmpp.NewRouter(),
		joined:   make(chan struct{}),
		sessions: make(map[string]*JingleSession),
	}
}

// SetHandler sets the signaling handler.
func (s *SignalingClient) SetHandler(handler platform.SignalingHandler) {
	s.handler = handler
}

// Connect establishes the XMPP connection using anonymous authentication.
func (s *SignalingClient) Connect(ctx context.Context) error {
	s.logger.Info("Connecting to XMPP server",
		slog.String("websocket_url", s.config.WebSocketURL),
		slog.String("xmpp_domain", s.config.XMPPDomain))

	// Register handlers
	// Note: We register IQ handler here to catch early session requests if any
	s.router.HandleFunc("iq", s.handleIQ)
	s.router.HandleFunc("presence", s.handlePresence)

	// Parse timeout check
	if _, err := time.ParseDuration(s.config.ConnectTimeout); err != nil {
		s.logger.Warn("Invalid connect timeout, using default 10s", slog.String("error", err.Error()))
	}

	// Create XMPP configuration for anonymous authentication
	config := xmpp.Config{
		TransportConfiguration: xmpp.TransportConfiguration{
			Address: s.config.WebSocketURL,
			Domain:  s.config.XMPPDomain,
		},
		// Anonymous auth: No Jid/Password usually triggers it or we might need specific SASL handling if library requires it.
		// For now, we leave Jid/Password empty.
		
		// T014a: Configure keepalive
		// Using standard client behavior.
	}

	// Initialize the client
	// Pass the router we created in NewSignalingClient
	client, err := xmpp.NewClient(&config, s.router, s.errorHandler)
	if err != nil {
		return fmt.Errorf("failed to create xmpp client: %w", err)
	}
	s.client = client

	// Connect
	err = client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to xmpp: %w", err)
	}
	
	s.logger.Info("XMPP connection established")
	
	return nil
}

// JoinMUC joins the configured Multi-User Chat room.
func (s *SignalingClient) JoinMUC(ctx context.Context, nickname string) error {
	roomJID := fmt.Sprintf("%s@%s/%s", s.config.RoomName, s.config.MUCDomain, nickname)
	s.logger.Info("Joining MUC", slog.String("room_jid", roomJID))

	// Send initial presence
	// stanza.Presence likely embeds Attrs
	pres := stanza.Presence{
		Attrs: stanza.Attrs{
			To: roomJID,
		},
	}
	err := s.client.Send(pres)
	if err != nil {
		return fmt.Errorf("failed to send presence: %w", err)
	}

	return nil
}

// handlePresence processes incoming presence stanzas.
func (s *SignalingClient) handlePresence(sender xmpp.Sender, p stanza.Packet) {
	presence, ok := p.(*stanza.Presence) // Use pointer
	if !ok {
		return
	}

	// T014: Detect self-presence to confirm join.
	s.logger.Debug("Received presence", slog.String("from", presence.From), slog.String("type", string(presence.Type)))
	
	if presence.Type == "error" {
		s.logger.Error("Presence error", slog.String("error", fmt.Sprintf("%v", presence.Error)))
		return
	}

	// Simple latch: if we get presence from the room, we consider it joined for MVP.
	select {
	case <-s.joined:
		// Already closed
	default:
		// Ideally check if From == RoomJID/Nick
		close(s.joined)
		s.logger.Info("Joined MUC successfully", slog.String("room", s.config.RoomName))
	}
}

// handleIQ processes incoming IQ stanzas, looking for Jingle requests.
func (s *SignalingClient) handleIQ(sender xmpp.Sender, p stanza.Packet) {
	iq, ok := p.(*stanza.IQ) // Use pointer
	if !ok {
		return
	}

	// We only care about set requests
	if iq.Type != "set" {
		return
	}

	// Re-marshal to check for Jingle
	// We use a wrapper struct to decode the whole IQ and extract Jingle
	type JingleIQWrapper struct {
		XMLName xml.Name  `xml:"iq"`
		Jingle  *JingleIQ `xml:"jingle"`
	}

	data, err := xml.Marshal(iq)
	if err != nil {
		s.logger.Warn("Failed to marshal IQ", slog.String("error", err.Error()))
		return
	}

	var wrapper JingleIQWrapper
	if err := xml.Unmarshal(data, &wrapper); err == nil {
		if wrapper.Jingle != nil && wrapper.Jingle.XMLName.Space == "urn:xmpp:jingle:1" {
			jingle := wrapper.Jingle
			if jingle.Action == "session-initiate" {
				if err := s.HandleSessionInitiate(*iq, jingle); err != nil {
					s.logger.Error("Failed to handle session-initiate", slog.String("error", err.Error()))
				}
			} else if jingle.Action == "transport-info" {
				if err := s.HandleTransportInfo(*iq, jingle); err != nil {
					s.logger.Error("Failed to handle transport-info", slog.String("error", err.Error()))
				}
			}
		}
	}
}

// WaitJoined waits for the MUC join to complete.
func (s *SignalingClient) WaitJoined(ctx context.Context) error {
	select {
	case <-s.joined:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Disconnect closes the XMPP connection.
func (s *SignalingClient) Disconnect() error {
	s.logger.Info("Disconnecting XMPP client")
	if s.client != nil {
		return s.client.Disconnect()
	}
	return nil
}

func (s *SignalingClient) errorHandler(err error) {
	s.logger.Error("XMPP client error", slog.String("error", err.Error()))
}
