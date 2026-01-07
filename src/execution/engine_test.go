package execution

import (
	"log/slog"
	"os"
	"testing"
	"time"

	//"webrtc-load-engine/src/platform" // Removed unused import
	"webrtc-load-engine/src/scenario"
)

// MockPlatformAdapter is a mock implementation of the PlatformAdapter interface for testing.
type MockPlatformAdapter struct {
	connectCount int
	t            *testing.T
	connectDelay time.Duration // Simulate connection delay
}

func (m *MockPlatformAdapter) Connect(peerID string) error {
	m.t.Logf("MockPlatformAdapter: Connecting peer %s", peerID)
	time.Sleep(m.connectDelay) // Simulate connection time
	m.connectCount++
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
	if mockAdapter.connectCount != testPlan.Participants {
		t.Errorf("Expected %d peers to connect, got %d", testPlan.Participants, mockAdapter.connectCount)
	}

	if finalStatus != "completed" {
		t.Errorf("Expected final status 'completed', got %s", finalStatus)
	}

	engine.peersMutex.RLock()
	for _, peer := range engine.peers {
		if peer.state != PeerStateDisconnected {
			t.Errorf("Expected peer %s to be disconnected, but got state %s", peer.ID(), peer.state)
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

	if finalStatus != "failed" { // Should be failed because it was stopped prematurely
		t.Errorf("Expected final status 'failed', got %s", finalStatus)
	}

	engine.peersMutex.RLock()
	for _, peer := range engine.peers {
		if peer.state != PeerStateDisconnected {
			t.Errorf("Expected peer %s to be disconnected, but got state %s", peer.ID(), peer.state)
		}
	}
	engine.peersMutex.RUnlock()
}
