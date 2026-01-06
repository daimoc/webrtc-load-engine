package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/scenario"
)

// NoOpLogger is a dummy slog.Handler that does nothing.
type NoOpLogger struct{}

func (n *NoOpLogger) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (n *NoOpLogger) Handle(_ context.Context, _ slog.Record) error { return nil }
func (n *NoOpLogger) WithAttrs(_ []slog.Attr) slog.Handler         { return n }
func (n *NoOpLogger) WithGroup(_ string) slog.Handler             { return n }

// NoOpMetrics is a dummy implementation of handlers.Metrics that does nothing.
type NoOpMetrics struct{}

func (m *NoOpMetrics) IncScenarioCreationRequestsTotal(_ string) {}
func (m *NoOpMetrics) ObserveScenarioCreationDurationSeconds(_ float64) {}

func TestCreateScenarioIntegration(t *testing.T) {
	// Initialize logger and metrics for tests
	testLogger := slog.New(&NoOpLogger{})
	testMetrics := &NoOpMetrics{}

	// Initialize service and handler with dummy logger and metrics
	scenarioService := scenario.NewService(testLogger)
	scenarioHandler := handlers.NewScenarioHandler(scenarioService, testLogger, testMetrics)

	// Create a test HTTP server
	router := http.NewServeMux()
	router.HandleFunc("/api/v1/scenarios", scenarioHandler.CreateScenario)
	ts := httptest.NewServer(router)
	defer ts.Close()

	tests := []struct {
		name         string
		requestBody  handlers.ScenarioRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "Successful Scenario Creation",
			requestBody: handlers.ScenarioRequest{
				Name:     "Integration Test Scenario",
				Platform: "jitsi",
				TestPlan: scenario.TestPlan{Participants: 10, RampUpSeconds: 5, DurationSeconds: 60},
			},
			expectedStatus: http.StatusCreated,
			expectedError:  "",
		},
		{
			name: "Invalid Platform",
			requestBody: handlers.ScenarioRequest{
				Name:     "Invalid Platform Scenario",
				Platform: "unsupported",
				TestPlan: scenario.TestPlan{Participants: 10, RampUpSeconds: 5, DurationSeconds: 60},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "unsupported platform adapter: unsupported",
		},
		{
			name: "Invalid Test Plan - Zero Participants",
			requestBody: handlers.ScenarioRequest{
				Name:     "Zero Participants Scenario",
				Platform: "jitsi",
				TestPlan: scenario.TestPlan{Participants: 0, RampUpSeconds: 5, DurationSeconds: 60},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "participants must be greater than 0",
		},
		{
			name: "Invalid Request Body - Missing Name",
			requestBody: handlers.ScenarioRequest{
				Platform: "jitsi",
				TestPlan: scenario.TestPlan{Participants: 10, RampUpSeconds: 5, DurationSeconds: 60},
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "scenario name cannot be empty", // This error comes from the NewScenario constructor
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.requestBody)
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/scenarios", bytes.NewBuffer(body))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedError != "" {
				var errorResponse handlers.ErrorResponse
				if err := json.NewDecoder(resp.Body).Decode(&errorResponse); err != nil {
					t.Fatalf("Failed to decode error response: %v", err)
				}
				if errorResponse.Error != tt.expectedError {
					t.Errorf("Expected error message %q, got %q", tt.expectedError, errorResponse.Error)
				}
			} else {
				var scenarioResponse handlers.ScenarioResponse
				if err := json.NewDecoder(resp.Body).Decode(&scenarioResponse); err != nil {
					t.Fatalf("Failed to decode success response: %v", err)
				}
				if scenarioResponse.ID == "" {
					t.Errorf("Expected scenario ID in response, got empty")
				}
				if scenarioResponse.Status != "created" {
					t.Errorf("Expected scenario status 'created', got %q", scenarioResponse.Status)
				}
			}
		})
	}
}
