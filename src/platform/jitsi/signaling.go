package jitsi

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"mellium.im/sasl"
	"mellium.im/xmlstream"
	"mellium.im/xmpp"
	"mellium.im/xmpp/jid"
	"mellium.im/xmpp/mux"
	"mellium.im/xmpp/stanza"
	xmppws "mellium.im/xmpp/websocket"
	ws "nhooyr.io/websocket"
	"webrtc-load-engine/src/platform"
)

// SignalingClient handles XMPP signaling for Jitsi.
type SignalingClient struct {
	logger *slog.Logger
	session *xmpp.Session
	config JingleConfig
	joined chan struct{} // Signal when MUC join is complete
	
sessions map[string]*JingleSession
	mu       sync.RWMutex
	
handler platform.SignalingHandler
	wsConn  *ws.Conn
}

// NewSignalingClient creates a new SignalingClient.
func NewSignalingClient(logger *slog.Logger, config JingleConfig) *SignalingClient {
	return &SignalingClient{
		logger:   logger,
		config:   config,
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

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second) 
	defer cancel()

	wsConn, _, err := ws.Dial(dialCtx, s.config.WebSocketURL, &ws.DialOptions{
		Subprotocols: []string{"xmpp"},
	})
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %w", err)
	}
	s.wsConn = wsConn

	nc := ws.NetConn(ctx, wsConn, ws.MessageText)

	j, err := jid.Parse(s.config.XMPPDomain)
	if err != nil {
		nc.Close()
		return fmt.Errorf("failed to parse jid: %w", err)
	}

	// Establish XMPP Session using generic NewSession but with WebSocket Negotiator
	// This allows us to customize StreamConfig for debugging.
	session, err := xmpp.NewSession(
		ctx,
		j.Domain(), 
		j,
		nc,
		xmpp.Secure,
		xmppws.Negotiator(func(sess *xmpp.Session, cfg *xmpp.StreamConfig) xmpp.StreamConfig {
			var c xmpp.StreamConfig
			if cfg != nil {
				c = *cfg
			}
			c.Features = []xmpp.StreamFeature{
				xmpp.SASL("", "", sasl.Anonymous),
				xmpp.BindResource(),
			}
			if s.config.LogXMPP {
				var lastDirection int32 = 0 // 0=init, 1=in, 2=out
				c.TeeIn = &colorWriter{w: os.Stdout, color: ansiRed, last: &lastDirection, dir: 1}   // Recv -> Red
				c.TeeOut = &colorWriter{w: os.Stdout, color: ansiGreen, last: &lastDirection, dir: 2} // Send -> Green
			}
			return c
		}),
	)
	if err != nil {
		nc.Close()
		return fmt.Errorf("failed to create xmpp session: %w", err)
	}
	
s.session = session
	s.logger.Info("XMPP connection established", slog.String("jid", session.LocalAddr().String()))

	go s.serve()

	return nil
}

// serve handles incoming XML stream
func (s *SignalingClient) serve() {
	// mux.New takes stanza namespace (stanza.NSClient is "jabber:client")
	m := mux.New(
		stanza.NSClient,
		mux.PresenceFunc(stanza.AvailablePresence, xml.Name{}, s.handlePresence),
		mux.PresenceFunc(stanza.ErrorPresence, xml.Name{}, s.handlePresence),
		// Match Jingle IQs (set, urn:xmpp:jingle:1 jingle)
		mux.IQFunc(stanza.SetIQ, xml.Name{Space: "urn:xmpp:jingle:1", Local: "jingle"}, s.handleIQ),
	)

	if err := s.session.Serve(m); err != nil {
		// Serve returns error when session closes, which is expected on disconnect
		if err != io.EOF {
			s.logger.Error("Session serve error", slog.String("error", err.Error()))
		}
	}
}

// JoinMUC joins the configured Multi-User Chat room.
func (s *SignalingClient) JoinMUC(ctx context.Context, nickname string) error {
	roomJIDStr := fmt.Sprintf("%s@%s/%s", s.config.RoomName, s.config.MUCDomain, nickname)
	// For sending presence, we can use session.Send with an xmlstream.Reader
	
	return s.session.Send(ctx, xmlstream.Wrap(
		xmlstream.ReaderFunc(func() (xml.Token, error) {
			return nil, io.EOF
		}),
		xml.StartElement{
			Name: xml.Name{Local: "presence"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "to"}, Value: roomJIDStr},
				{Name: xml.Name{Space: "http://jabber.org/protocol/muc", Local: "x"}}, // Minimal MUC x
			},
		},
	))
}

// handlePresence processes incoming presence stanzas.
func (s *SignalingClient) handlePresence(p stanza.Presence, t xmlstream.TokenReadEncoder) error {
	// p contains the header info (From, To, Type)
	// t contains the body (children)
	
	// T014: Detect self-presence to confirm join.
	s.logger.Debug("Received presence", slog.String("from", p.From.String()), slog.String("type", string(p.Type)))
	
	if p.Type == stanza.ErrorPresence {
		s.logger.Error("Presence error", slog.String("from", p.From.String()))
		xmlstream.Copy(xmlstream.Discard(), t)
		return nil
	}

	// Simple latch: if we get presence from the room, we consider it joined.
	// We could verify it matches our nick.
	select {
	case <-s.joined:
	default:
		close(s.joined)
		s.logger.Info("Joined MUC successfully")
	}
	
	// Discard remaining tokens in this stanza to advance stream
	_, err := xmlstream.Copy(xmlstream.Discard(), t)
	return err
}

// handleIQ processes incoming IQ stanzas.
func (s *SignalingClient) handleIQ(iq stanza.IQ, t xmlstream.TokenReadEncoder, start *xml.StartElement) error {
	// mux.IQFunc passes:
	// iq: Header
	// t: Reader for payload
	// start: The start element of the payload (e.g. <jingle ...>)
	
	// Decode Jingle payload
	var jingle JingleIQ
	
	// Use DecodeElement to decode using the start element we matched
	d := xml.NewTokenDecoder(t)
	if err := d.DecodeElement(&jingle, start); err != nil {
		s.logger.Warn("Failed to decode element", slog.String("error", err.Error()))
		return nil
	}

	if jingle.Action == "session-initiate" {
		if err := s.HandleSessionInitiate(iq.From.String(), &jingle); err != nil {
			s.logger.Error("Failed to handle session-initiate", slog.String("error", err.Error()))
		}
	} else if jingle.Action == "transport-info" {
		if err := s.HandleTransportInfo(iq.From.String(), &jingle); err != nil {
			s.logger.Error("Failed to handle transport-info", slog.String("error", err.Error()))
		}
	}

	return nil
}

// Disconnect closes the session.
func (s *SignalingClient) Disconnect() error {
	s.logger.Info("Disconnecting XMPP client")
	if s.session != nil {
		return s.session.Close()
	}
	if s.wsConn != nil {
		return s.wsConn.Close(ws.StatusNormalClosure, "disconnecting")
	}
	return nil
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

func (s *SignalingClient) errorHandler(err error) {
	s.logger.Error("XMPP client error", slog.String("error", err.Error()))
}

// --- Debug helpers ---

const (
	ansiRed   = "\033[31m"
	ansiGreen = "\033[32m"
	ansiReset = "\033[0m"
)

type colorWriter struct {
	w     io.Writer
	color string
	last  *int32 // Shared state: 0=init, 1=in, 2=out
	dir   int32  // This writer's direction: 1=in, 2=out
}

func (cw *colorWriter) Write(p []byte) (n int, err error) {
	// Check if direction changed
	if atomic.SwapInt32(cw.last, cw.dir) != cw.dir {
		cw.w.Write([]byte("\n"))
	}

	cw.w.Write([]byte(cw.color))
	n, err = cw.w.Write(p)
	cw.w.Write([]byte(ansiReset))
	return n, err
}
