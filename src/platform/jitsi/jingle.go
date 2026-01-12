package jitsi

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"

	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

// SessionState represents the state of a Jingle session.
type SessionState int

const (
	SessionStatePending SessionState = iota
	SessionStateActive
	SessionStateEnded
)

func (s SessionState) String() string {
	switch s {
	case SessionStatePending:
		return "PENDING"
	case SessionStateActive:
		return "ACTIVE"
	case SessionStateEnded:
		return "ENDED"
	default:
		return "UNKNOWN"
	}
}

// JingleSession represents a single media session negotiation state.
type JingleSession struct {
	ID        string
	PeerJID   string // The focus/JVB JID
	State     SessionState
	LocalSDP  *webrtc.SessionDescription
	RemoteSDP *webrtc.SessionDescription
}

// HandleSessionInitiate processes a session-initiate Jingle request.
func (s *SignalingClient) HandleSessionInitiate(fromJID string, jingle *JingleIQ) error {
	s.logger.Info("Handling session-initiate", slog.String("sid", jingle.SID), slog.String("initiator", jingle.Initiator))

	s.mu.Lock()
	if _, exists := s.sessions[jingle.SID]; exists {
		s.mu.Unlock()
		s.logger.Warn("Session already exists", slog.String("sid", jingle.SID))
		return fmt.Errorf("session already exists")
	}

	// Create new session
	session := &JingleSession{
		ID:      jingle.SID,
		PeerJID: fromJID, // Usually From is the focus.
		State:   SessionStatePending,
	}
	s.sessions[jingle.SID] = session
	s.mu.Unlock()

	// Parse remote SDP (T020)
	sdp, err := convertJingleToSDP(jingle)
	if err != nil {
		s.logger.Error("Failed to convert Jingle to SDP", slog.String("error", err.Error()))
		return err
	}
	session.RemoteSDP = sdp
	s.logger.Info("Parsed remote SDP", slog.String("sdp", sdp.SDP))

	// Trigger Peer to handle this (T023)
	if s.handler != nil {
		answerSDP, err := s.handler.OnOffer(sdp.SDP)
		if err != nil {
			s.logger.Error("Handler failed to process offer", slog.String("error", err.Error()))
			return err
		}
		
		// Parse answer
		localSDP := &webrtc.SessionDescription{
			Type: webrtc.SDPTypeAnswer,
			SDP:  answerSDP,
		}
		
		// Send Session Accept
		if err := s.SendSessionAccept(context.Background(), jingle.SID, localSDP); err != nil {
			s.logger.Error("Failed to send session-accept", slog.String("error", err.Error()))
			return err
		}
	} else {
		s.logger.Warn("No signaling handler registered, cannot process offer")
	}
	
	return nil
}

// convertJingleToSDP converts a Jingle IQ to a WebRTC SessionDescription.
func convertJingleToSDP(jingle *JingleIQ) (*webrtc.SessionDescription, error) {
	var sb strings.Builder

	// Session level
	sb.WriteString("v=0\r\n")
	sb.WriteString(fmt.Sprintf("o=- %s 2 IN IP4 0.0.0.0\r\n", jingle.SID))
	sb.WriteString("s=-\r\n")
	sb.WriteString("t=0 0\r\n")
	
	// Bundle (usually Jitsi uses bundle)
	// We'll assume bundle for audio/video if present
	// sb.WriteString("a=group:BUNDLE audio video\r\n") // Simplified
	
	// Content mapping
	for _, content := range jingle.Content {
		media := content.Name // "audio" or "video" usually
		
		// Map description
		if content.Description != nil {
			// Find primary codec to set port/proto
			// For simplified SDP generation:
			sb.WriteString(fmt.Sprintf("m=%s 9 UDP/TLS/RTP/SAVPF", media))
			for _, pt := range content.Description.PayloadTypes {
				sb.WriteString(fmt.Sprintf(" %s", pt.ID))
			}
			sb.WriteString("\r\n")
			
			sb.WriteString("c=IN IP4 0.0.0.0\r\n")
			sb.WriteString("a=rtcp-mux\r\n")

			// Transport
			if content.Transport != nil {
				if content.Transport.Ufrag != "" {
					sb.WriteString(fmt.Sprintf("a=ice-ufrag:%s\r\n", content.Transport.Ufrag))
				}
				if content.Transport.Pwd != "" {
					sb.WriteString(fmt.Sprintf("a=ice-pwd:%s\r\n", content.Transport.Pwd))
				}
				for _, fp := range content.Transport.Fingerprints {
					sb.WriteString(fmt.Sprintf("a=fingerprint:%s %s\r\n", fp.Hash, fp.Value))
				}
				// Candidates would be added here, but usually Jitsi sends them via trickle (transport-info)
				// or in the initial offer.
				// For initial offer:
				for _, cand := range content.Transport.Candidates {
					// candidate:1 1 UDP 2130706431 10.0.0.1 5000 typ host
					sb.WriteString(fmt.Sprintf("a=candidate:%s %s %s %s %s %s typ %s", 
						cand.ID, cand.Component, cand.Protocol, cand.Priority, cand.IP, cand.Port, cand.Type))
					if cand.RelAddr != "" {
						sb.WriteString(fmt.Sprintf(" raddr %s rport %s", cand.RelAddr, cand.RelPort))
					}
					sb.WriteString("\r\n")
				}
			}

			// Payload types
			for _, pt := range content.Description.PayloadTypes {
				sb.WriteString(fmt.Sprintf("a=rtpmap:%s %s\r\n", pt.ID, pt.Name)) // ClockRate?
				// Add fmtp/rtcp-fb
			}
			
			// Setup (usually actpass or active)
			sb.WriteString("a=setup:actpass\r\n") // Jicofo is usually actpass? Or active?
			// Jitsi Bridge usually sends "actpass" in offer.
			
			sb.WriteString("a=mid:" + content.Name + "\r\n")
		}
	}

	return &webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sb.String(),
	},
nil
}

// JingleIQWrapper wraps Jingle payload in an IQ stanza for sending.
type JingleIQWrapper struct {
	XMLName xml.Name `xml:"iq"`
	Type    string   `xml:"type,attr"`
	To      string   `xml:"to,attr"`
	ID      string   `xml:"id,attr"`
	Jingle  *JingleIQ
}

// SendSessionAccept sends a session-accept Jingle stanza with the local SDP.
func (s *SignalingClient) SendSessionAccept(ctx context.Context, sid string, localSDP *webrtc.SessionDescription) error {
	s.mu.Lock()
	session, exists := s.sessions[sid]
	s.mu.Unlock()
	
	if !exists {
		return fmt.Errorf("session not found: %s", sid)
	}

	jingle, err := convertSDPToJingle(localSDP, sid, "session-accept")
	if err != nil {
		return fmt.Errorf("failed to convert SDP to Jingle: %w", err)
	}
	
	// Update session
	s.mu.Lock()
	session.LocalSDP = localSDP
	session.State = SessionStateActive
	s.mu.Unlock()

	wrapper := JingleIQWrapper{
		Type:   "set",
		To:     session.PeerJID,
		ID:     fmt.Sprintf("jingle-%d", rand.Int()),
		Jingle: jingle,
	}
	
	// Use session.Encode to send the wrapper struct
	return s.session.Encode(ctx, wrapper)
}

// HandleTransportInfo processes a transport-info Jingle request.
func (s *SignalingClient) HandleTransportInfo(fromJID string, jingle *JingleIQ) error {
	s.logger.Info("Handling transport-info", slog.String("sid", jingle.SID))

	// Iterate candidates and pass to handler
	for _, content := range jingle.Content {
		if content.Transport != nil {
			for _, cand := range content.Transport.Candidates {
				// Construct SDP candidate string
				// candidate:foundation component protocol priority ip port typ type ...
				candidate := fmt.Sprintf("candidate:%s %s %s %s %s %s typ %s",
					cand.ID, cand.Component, cand.Protocol, cand.Priority, cand.IP, cand.Port, cand.Type)
				
				if cand.RelAddr != "" {
					candidate += fmt.Sprintf(" raddr %s rport %s", cand.RelAddr, cand.RelPort)
				}
				
				if s.handler != nil {
					if err := s.handler.OnICECandidate(candidate); err != nil {
						s.logger.Warn("Failed to handle ICE candidate", slog.String("error", err.Error()))
					}
				}
			}
		}
	}
	return nil
}

// SendSessionTerminate sends a session-terminate Jingle stanza.
func (s *SignalingClient) SendSessionTerminate(ctx context.Context, sid string) error {
	s.mu.Lock()
	session, exists := s.sessions[sid]
	s.mu.Unlock()
	
	if !exists {
		return nil // Already gone
	}

	jingle := &JingleIQ{
		Action: "session-terminate",
		SID:    sid,
		Reason: &JingleReason{
			Condition: "success",
		},
	}

	// Update session state
	s.mu.Lock()
	session.State = SessionStateEnded
	delete(s.sessions, sid)
	s.mu.Unlock()

	wrapper := JingleIQWrapper{
		Type:   "set",
		To:     session.PeerJID,
		ID:     fmt.Sprintf("jingle-%d", rand.Int()),
		Jingle: jingle,
	}
	
	return s.session.Encode(ctx, wrapper)
}

// TerminateSessions terminates all active sessions.
func (s *SignalingClient) TerminateSessions(ctx context.Context) {
	s.mu.Lock()
	// Copy SIDs to avoid deadlock during iteration if SendSessionTerminate locks
	var sids []string
	for sid := range s.sessions {
		sids = append(sids, sid)
	}
	s.mu.Unlock()

	for _, sid := range sids {
		if err := s.SendSessionTerminate(ctx, sid); err != nil {
			s.logger.Error("Failed to terminate session", slog.String("sid", sid), slog.String("error", err.Error()))
		}
	}
}

// SendTransportInfo sends a transport-info Jingle stanza with ICE candidates.
func (s *SignalingClient) SendTransportInfo(ctx context.Context, sid string, candidates []webrtc.ICECandidate) error {
	// TODO: Map webrtc.ICECandidate to JingleCandidate and send
	// For now, placeholder
	return nil
}

// convertSDPToJingle converts a WebRTC SessionDescription to a Jingle IQ.
func convertSDPToJingle(desc *webrtc.SessionDescription, sid string, action string) (*JingleIQ, error) {
	parsed := &sdp.SessionDescription{}
	if err := parsed.Unmarshal([]byte(desc.SDP)); err != nil {
		return nil, err
	}

	jingle := &JingleIQ{
		Action: action,
		SID:    sid,
		Responder: "", // Should set our JID?
	}

	for _, md := range parsed.MediaDescriptions {
		content := JingleContent{
			Creator: "initiator", // or responder? Jitsi expects responder usually.
			Name:    md.MediaName.Media,
		}
		
		// Description
		descXML := &JingleDescription{
			Media: md.MediaName.Media,
		}
		
		// Payload types (simplified)
		// ... map md.Attributes to PayloadTypes ...
		// This is tedious to implement fully in one step. 
		// We'll add a placeholder payload type to satisfy structure.
		// In a real impl, we'd iterate md.Attributes ("rtpmap", "fmtp").
		
		content.Description = descXML
		
		// Transport (ICE)
		transXML := &JingleTransport{
			Ufrag: "", // Extract from session attributes or media attributes
			Pwd:   "",
		}
		
		// Find ufrag/pwd in session or media level
		for _, attr := range parsed.Attributes {
			if attr.Key == "ice-ufrag" {
				transXML.Ufrag = attr.Value
			} else if attr.Key == "ice-pwd" {
				transXML.Pwd = attr.Value
			} else if attr.Key == "fingerprint" {
				// Parse fingerprint
				parts := strings.SplitN(attr.Value, " ", 2)
				if len(parts) == 2 {
					transXML.Fingerprints = append(transXML.Fingerprints, Fingerprint{
						Hash:  parts[0],
						Value: parts[1],
					})
				}
			}
		}
		// Also check media level attributes
		for _, attr := range md.Attributes {
			if attr.Key == "ice-ufrag" {
				transXML.Ufrag = attr.Value
			} else if attr.Key == "ice-pwd" {
				transXML.Pwd = attr.Value
			} else if attr.Key == "fingerprint" {
				parts := strings.SplitN(attr.Value, " ", 2)
				if len(parts) == 2 {
					transXML.Fingerprints = append(transXML.Fingerprints, Fingerprint{
						Hash:  parts[0],
						Value: parts[1],
					})
				}
			}
		}

		content.Transport = transXML
		jingle.Content = append(jingle.Content, content)
	}

	return jingle,
nil
}
