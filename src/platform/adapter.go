package platform

// PlatformAdapter defines the interface for a platform-specific signaling client.
type PlatformAdapter interface {
	Connect(peerID string) error
	Disconnect(peerID string) error
}
