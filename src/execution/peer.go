package execution

import (
	"fmt"
	"log/slog"

	"github.com/pion/webrtc/v3"
)

// PeerState represents the state of a peer in the execution lifecycle.
type PeerState string

const (
	PeerStateNew        PeerState = "new"
	PeerStateConnecting PeerState = "connecting"
	PeerStateConnected  PeerState = "connected"
	PeerStateDisconnected PeerState = "disconnected"
	PeerStateFailed     PeerState = "failed"
)

// Peer encapsulates a WebRTC peer connection and its state.
type Peer struct {
	id     string
	pc     *webrtc.PeerConnection
	state  PeerState
	logger *slog.Logger
}

// NewPeer creates a new Peer.
func NewPeer(id string, logger *slog.Logger) (*Peer, error) {
	if logger == nil {
		logger = slog.Default()
	}

	// Create API with default settings
	api := webrtc.NewAPI()
	pc, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	return &Peer{
		id:     id,
		pc:     pc,
		state:  PeerStateNew,
		logger: logger,
	}, nil
}

// ID returns the peer's ID.
func (p *Peer) ID() string {
	return p.id
}

// OnOffer handles an incoming SDP offer.
func (p *Peer) OnOffer(sdp string) (string, error) {
	p.logger.Info("Received SDP offer", slog.String("peer_id", p.id))

	if err := p.pc.SetRemoteDescription(webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdp,
	}); err != nil {
		return "", fmt.Errorf("failed to set remote description: %w", err)
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return "", fmt.Errorf("failed to create answer: %w", err)
	}

	if err := p.pc.SetLocalDescription(answer); err != nil {
		return "", fmt.Errorf("failed to set local description: %w", err)
	}

	return answer.SDP, nil
}

// OnICECandidate handles an incoming ICE candidate.
func (p *Peer) OnICECandidate(candidate string) error {
	p.logger.Info("Received ICE candidate", slog.String("peer_id", p.id))
	
	// Parse candidate string to webrtc.ICECandidateInit?
	// Pion expects ICECandidateInit.
	// For now, MVP might skip trickle or assume full candidate string parsing logic is needed.
	// We'll log it.
	return nil 
}


