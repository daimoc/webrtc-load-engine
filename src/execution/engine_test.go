package execution

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

// MockPlatformAdapter is a mock implementation of the PlatformAdapter interface for testing.
type MockPlatformAdapter struct {
	connectCount atomic.Int32 // Peers connect concurrently
	t            *testing.T
	connectDelay time.Duration // Simulate connection delay
}

var _ platform.PlatformAdapter = (*MockPlatformAdapter)(nil)

func (m *MockPlatformAdapter) Connect(_ context.Context, peerID string, _ platform.SignalingHandler) error {
	m.t.Logf("MockPlatformAdapter: Connecting peer %s", peerID)
	time.Sleep(m.connectDelay) // Simulate connection time
	m.connectCount.Add(1)
	return nil
}

func (m *MockPlatformAdapter) Disconnect(peerID string) error {
	m.t.Logf("MockPlatformAdapter: Disconnecting peer %s", peerID)
	return nil
}

func TestEngine_StartStop(t *testing.T) {
	// Setup a dummy logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Define a test scenario
	testScenarioID := "test-scenario-123"
	testPlan := scenario.TestPlan{
		Participants:    10,
		RampUpSeconds:   1,
		DurationSeconds: 2,
	}

	// Create a mock adapter
	mockAdapter := &MockPlatformAdapter{t: t, connectDelay: 5 * time.Millisecond} // Add connect delay

	// Channel to signal when onComplete is called
	onCompleteCalled := make(chan struct{})
	var finalStatus string

	onComplete := func(scenarioID string, status string) {
		if scenarioID != testScenarioID {
			t.Errorf("Expected scenarioID %s, got %s", testScenarioID, scenarioID)
		}
		finalStatus = status
		close(onCompleteCalled)
	}

	engine := NewEngine(testScenarioID, testPlan, mockAdapter, logger, onComplete)

	// Start the engine in a goroutine
	go engine.Start()

	// Give it some time to ramp up and run
	time.Sleep(time.Duration(testPlan.RampUpSeconds+testPlan.DurationSeconds+1) * time.Second)

	// Wait for the onComplete callback to be triggered
	select {
	case <-onCompleteCalled:
		t.Log("onComplete callback received.")
	case <-time.After(5 * time.Second):
		t.Fatal("onComplete callback not received within timeout")
	}

	// Assertions
	if got := int(mockAdapter.connectCount.Load()); got != testPlan.Participants {
		t.Errorf("Expected %d peers to connect, got %d", testPlan.Participants, got)
	}

	if finalStatus != "completed" {
		t.Errorf("Expected final status 'completed', got %s", finalStatus)
	}

	engine.peersMutex.RLock()
	for _, peer := range engine.peers {
		if peer.State() != PeerStateDisconnected {
			t.Errorf("Expected peer %s to be disconnected, but got state %s", peer.ID(), peer.State())
		}
	}
	engine.peersMutex.RUnlock()
}

func TestEngine_StopDuringRampUp(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	testScenarioID := "test-scenario-456"
	testPlan := scenario.TestPlan{
		Participants:    100, // Large number to ensure we stop mid-ramp-up
		RampUpSeconds:   10,
		DurationSeconds: 10,
	}

	mockAdapter := &MockPlatformAdapter{t: t, connectDelay: 5 * time.Millisecond} // Add connect delay

	onCompleteCalled := make(chan struct{})
	var finalStatus string

	onComplete := func(scenarioID string, status string) {
		finalStatus = status
		close(onCompleteCalled)
	}

	engine := NewEngine(testScenarioID, testPlan, mockAdapter, logger, onComplete)

	go engine.Start()

	// Stop the engine early during ramp-up
	time.Sleep(500 * time.Millisecond) // Wait a bit for some peers to connect
	engine.Stop()

	// Wait for the onComplete callback to be triggered
	select {
	case <-onCompleteCalled:
		t.Log("onComplete callback received after early stop.")
	case <-time.After(5 * time.Second):
		t.Fatal("onComplete callback not received after early stop within timeout")
	}

	if finalStatus != "stopped" { // Stopped on request before the duration elapsed
		t.Errorf("Expected final status 'stopped', got %s", finalStatus)
	}

	engine.peersMutex.RLock()
	for _, peer := range engine.peers {
		if peer.State() != PeerStateDisconnected {
			t.Errorf("Expected peer %s to be disconnected, but got state %s", peer.ID(), peer.State())
		}
	}
	engine.peersMutex.RUnlock()
}

// blockingAdapter blocks Connect until the engine cancels it.
type blockingAdapter struct {
	disconnects atomic.Int32
}

func (b *blockingAdapter) Connect(ctx context.Context, _ string, _ platform.SignalingHandler) error {
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingAdapter) Disconnect(string) error {
	b.disconnects.Add(1)
	return nil
}

func TestEngine_StopCancelsInFlightConnects(t *testing.T) {
	adapter := &blockingAdapter{}
	done := make(chan string, 1)
	plan := scenario.TestPlan{Participants: 3, DurationSeconds: 60}
	engine := NewEngine("test-scenario-789", plan, adapter, quietLogger, func(_ string, status string) { done <- status })

	go engine.Start()
	time.Sleep(100 * time.Millisecond) // All peers are stuck connecting

	stopped := make(chan struct{})
	go func() {
		engine.Stop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not cancel the in-flight connects")
	}
	engine.Stop() // Idempotent

	if status := <-done; status != "stopped" {
		t.Errorf("expected status stopped, got %s", status)
	}
	if got := adapter.disconnects.Load(); got != 3 {
		t.Errorf("expected 3 disconnects, got %d", got)
	}
	engine.peersMutex.RLock()
	defer engine.peersMutex.RUnlock()
	for _, p := range engine.peers {
		if p.State() != PeerStateDisconnected {
			t.Errorf("peer %s: expected disconnected, got %s", p.ID(), p.State())
		}
	}
}

// mediaServerAdapter stands in for a platform whose media server offers an
// audio, a video and a data section to each peer on Connect, and records the
// media kinds it receives from each peer.
type mediaServerAdapter struct {
	t        *testing.T
	mu       sync.Mutex
	received map[string]map[string]bool // peer ID -> kinds received
}

func (a *mediaServerAdapter) Connect(_ context.Context, peerID string, h platform.SignalingHandler) error {
	server := newOfferer(a.t)
	server.OnTrack(func(tr *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		if _, _, err := tr.ReadRTP(); err != nil {
			return
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.received[peerID] == nil {
			a.received[peerID] = map[string]bool{}
		}
		a.received[peerID][tr.Kind().String()] = true
	})
	offer, err := server.CreateOffer(nil)
	if err != nil {
		return err
	}
	gathered := webrtc.GatheringCompletePromise(server)
	if err := server.SetLocalDescription(offer); err != nil {
		return err
	}
	<-gathered
	answer, err := h.OnOffer(server.LocalDescription().SDP, func(string, string) {})
	if err != nil {
		return err
	}
	return server.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer})
}

func (a *mediaServerAdapter) Disconnect(string) error { return nil }

func (a *mediaServerAdapter) receivedAll(peers int, kinds ...string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.received) < peers {
		return false
	}
	for _, got := range a.received {
		for _, k := range kinds {
			if !got[k] {
				return false
			}
		}
	}
	return true
}

func TestEngine_SendsMediaOnceEstablished(t *testing.T) {
	adapter := &mediaServerAdapter{t: t, received: map[string]map[string]bool{}}
	plan := scenario.TestPlan{Participants: 2, DurationSeconds: 60, Media: &scenario.MediaConfig{Profile: scenario.MediaProfileAudioVideo}}
	done := make(chan string, 1)
	id := "engine-media-" + uuid.NewString()
	engine := NewEngine(id, plan, adapter, quietLogger, func(_, status string) { done <- status })
	go engine.Start()

	deadline := time.Now().Add(15 * time.Second)
	for !adapter.receivedAll(2, "audio", "video") {
		if time.Now().After(deadline) {
			t.Fatalf("media server did not receive audio and video from both peers: %v", adapter.received)
		}
		time.Sleep(20 * time.Millisecond)
	}

	engine.Stop()
	if status := <-done; status != "stopped" {
		t.Fatalf("expected stopped, got %s", status)
	}
	for _, kind := range []string{"audio", "video"} {
		if n := testutil.ToFloat64(media.PacketsSent.WithLabelValues(id, kind)); n == 0 {
			t.Errorf("expected %s packets counted as sent", kind)
		}
	}
	sent := testutil.ToFloat64(media.PacketsSent.WithLabelValues(id, "video"))
	time.Sleep(200 * time.Millisecond)
	if after := testutil.ToFloat64(media.PacketsSent.WithLabelValues(id, "video")); after != sent {
		t.Errorf("media still sent after stop: %v -> %v", sent, after)
	}
}
