package platform

// SignalingHandler defines the callback interface for signaling events.
type SignalingHandler interface {
	OnOffer(sdp string) (string, error)
	OnICECandidate(candidate string) error
}

// PlatformAdapter defines the interface for a platform-specific signaling client.
type PlatformAdapter interface {
	Connect(peerID string, handler SignalingHandler) error
	Disconnect(peerID string) error
}
