package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/execution" // Import execution package
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

// IntegrationMockPlatformAdapter for integration tests.
type IntegrationMockPlatformAdapter struct {
	connectCount int
	t            *testing.T
	connectDelay time.Duration // Simulate connection delay
	disconnectWG sync.WaitGroup // To wait for disconnects
}

func (m *IntegrationMockPlatformAdapter) Connect(peerID string) error {
	m.t.Logf("IntegrationMockPlatformAdapter: Connecting peer %s", peerID)
	time.Sleep(m.connectDelay) // Simulate connection time
	m.connectCount++
	return nil
}

func (m *IntegrationMockPlatformAdapter) Disconnect(peerID string) error {
	m.t.Logf("IntegrationMockPlatformAdapter: Disconnecting peer %s", peerID)
	// m.disconnectWG.Done() // Removed waitgroup logic for simplicity in this pass, unless we explicitly wait for disconnects
	return nil
}

// TestEngineStarter is an implementation of scenario.ExecutionStarter for tests.
type TestEngineStarter struct {
	logger *slog.Logger
	adapter platform.PlatformAdapter
}

// NewTestEngineStarter creates a new TestEngineStarter.
func NewTestEngineStarter(logger *slog.Logger, adapter platform.PlatformAdapter) *TestEngineStarter {
	return &TestEngineStarter{logger: logger, adapter: adapter}
}

// StartExecution creates and returns a new execution.Engine.
func (es *TestEngineStarter) StartExecution(scenarioID string, testPlan scenario.TestPlan, adapter platform.PlatformAdapter, onComplete func(scenarioID string, status string)) scenario.ExecutionEngine {
	// Use the injected mock adapter
	return execution.NewEngine(scenarioID, testPlan, es.adapter, es.logger, onComplete)
}

// NoOpMetrics is a dummy implementation of handlers.Metrics that does nothing.
type NoOpMetrics struct{}

func (m *NoOpMetrics) IncScenarioCreationRequestsTotal(_ string) {}
func (m *NoOpMetrics) ObserveScenarioCreationDurationSeconds(_ float64) {}
func (m *NoOpMetrics) IncScenarioExecutionRequestsTotal(_ string) {}

func TestScenarioExecutionIntegration(t *testing.T) {
	// Setup a dummy logger for the main application components
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Global metrics are not ideal for integration tests, but for now, we'll use no-ops
	// For a real scenario, you'd want to expose and assert on these metrics
	noOpMetrics := &NoOpMetrics{}

	// Create a mock adapter for the test
	mockAdapter := &IntegrationMockPlatformAdapter{t: t}
	
	testStarter := NewTestEngineStarter(logger, mockAdapter)
	scenarioService := scenario.NewService(logger, testStarter)
	scenarioHandler := handlers.NewScenarioHandler(scenarioService, logger, noOpMetrics)

	// Create a test HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/scenarios", scenarioHandler.CreateScenario)

	// Register the execute endpoint
	executeScenarioRegex := regexp.MustCompile(`/api/v1/scenarios/(?P<id>[^/]+)/_execute`)
	getScenarioRegex := regexp.MustCompile(`/api/v1/scenarios/(?P<id>[^/]+)$`)

	mux.HandleFunc("/api/v1/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if executeScenarioRegex.MatchString(r.URL.Path) {
				scenarioHandler.ExecuteScenario(w, r)
				return
			}
		} else if r.Method == http.MethodGet {
			if getScenarioRegex.MatchString(r.URL.Path) {
				scenarioHandler.GetScenario(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 1. Create a scenario
	createReqBody := handlers.ScenarioRequest{
		Name:     "Integration Test Scenario",
		Platform: "jitsi",
		TestPlan: scenario.TestPlan{Participants: 2, RampUpSeconds: 1, DurationSeconds: 2},
	}
	body, _ := json.Marshal(createReqBody)
	resp, err := http.Post(ts.URL+"/api/v1/scenarios", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create scenario: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	var createResp handlers.ScenarioResponse
	json.NewDecoder(resp.Body).Decode(&createResp)
	scenarioID := createResp.ID
	resp.Body.Close()

	if scenarioID == "" {
		t.Fatal("Scenario ID not returned")
	}
	t.Logf("Created scenario with ID: %s", scenarioID)

	// 2. Execute the scenario
	execResp, err := http.Post(fmt.Sprintf("%s/api/v1/scenarios/%s/_execute", ts.URL, scenarioID), "application/json", nil)
	if err != nil {
		t.Fatalf("Failed to execute scenario: %v", err)
	}
	if execResp.StatusCode != http.StatusAccepted {
		var errResp handlers.ErrorResponse
		json.NewDecoder(execResp.Body).Decode(&errResp)
		t.Fatalf("Expected status %d, got %d. Error: %s", http.StatusAccepted, execResp.StatusCode, errResp.Error)
	}
	execResp.Body.Close()
	t.Logf("Executed scenario with ID: %s", scenarioID)

	// 3. Poll for status change to "completed"
	timeout := time.After(10 * time.Second)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	var currentStatus string
	for {
		select {
		case <-timeout:
			t.Fatalf("Scenario did not complete within timeout. Last status: %s", currentStatus)
		case <-tick.C:
			statusResp, err := http.Get(fmt.Sprintf("%s/api/v1/scenarios/%s", ts.URL, scenarioID))
			if err != nil {
				t.Fatalf("Failed to get scenario status: %v", err)
			}
			var getResp handlers.ScenarioResponse
			json.NewDecoder(statusResp.Body).Decode(&getResp)
			currentStatus = getResp.Status
			statusResp.Body.Close()

			t.Logf("Polling scenario %s, current status: %s", scenarioID, currentStatus)

			if currentStatus == "completed" {
				t.Logf("Scenario %s completed successfully.", scenarioID)
				return
			}
			if currentStatus == "failed" {
				t.Fatalf("Scenario %s failed unexpectedly.", scenarioID)
			}
		}
	}
}

