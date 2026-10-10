package platform

import "context"

// TrickleFunc delivers a local ICE candidate ("candidate:..." SDP attribute
// value, without the "a=" prefix) for the media section identified by mid to
// the remote side.
type TrickleFunc func(mid, candidate string)

// DataChannel is a data channel opened on the media session.
type DataChannel interface {
	// SendText sends a text message. It fails while the channel is not
	// open; the caller retries on its next OnOpen.
	SendText(msg string) error
}

// DataChannelHandler receives a data channel's events. Methods are called
// from the media stack's goroutines, one at a time per channel, and must not
// block.
type DataChannelHandler interface {
	OnOpen()               // Open, or open again
	OnMessage(text string) // Text message (binary messages are dropped)
	OnClose()
}

// SignalingHandler defines the callback interface for signaling events.
// It is implemented by the media side (execution.Peer) and driven by a
// PlatformAdapter, so that no platform-specific type crosses the boundary.
type SignalingHandler interface {
	// OnOffer applies a remote SDP offer and returns the SDP answer. Local
	// candidates gathered before the answer is returned are embedded in it;
	// later ones are passed to trickle.
	OnOffer(sdp string, trickle TrickleFunc) (string, error)
	// OnICECandidate applies a remote ICE candidate for the media section mid.
	OnICECandidate(mid, candidate string) error
	// OpenDataChannel opens a data channel on the negotiated session (after
	// OnOffer), e.g. the channel a platform's media server expects for its
	// control messages. Events go to h; the returned channel sends.
	OpenDataChannel(label, protocol string, h DataChannelHandler) (DataChannel, error)
	// LocalMediaKinds returns the media kinds ("audio", "video") the peer
	// sends, in a stable order. Adapters call it before joining so that they
	// can announce the sources (for example in presence). Empty for a
	// receive-only peer.
	LocalMediaKinds() []string
	// Renegotiate applies an updated remote SDP offer on an already
	// negotiated session (remote sources added or removed) and returns the
	// SDP answer. It does not trickle candidates or record negotiation
	// metrics. Adapters whose platform expects no reply may drop the answer.
	Renegotiate(sdp string) (string, error)
	// ReceiveLimit returns the maximum number of remote streams the peer
	// receives per media kind (0: no limit). Adapters subscribe to (or
	// offer) at most that many and replace an ended one by a waiting one.
	ReceiveLimit() int
	// OnSignalingLost reports that the signaling connection was lost after
	// Connect returned (server gone, kicked from the room, keepalive timeout).
	OnSignalingLost(err error)
}

// PlatformAdapter defines the interface for a platform-specific signaling client.
type PlatformAdapter interface {
	// Connect joins peerID to the platform and blocks until it is joined, the
	// join fails, or ctx is cancelled. On error no resources are left behind.
	Connect(ctx context.Context, peerID string, handler SignalingHandler) error
	// Disconnect tears down everything Connect created for peerID. It is safe
	// to call for unknown or already-disconnected peers.
	Disconnect(peerID string) error
}
