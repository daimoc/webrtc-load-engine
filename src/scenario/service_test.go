package scenario

import (
	"context" // Add this import
	"log/slog" // Add this import
	"testing"
	"time"

	// No explicit import of "webrtc-load-engine/src/scenario" needed,
	// as this is a test file for the "scenario" package.
)

// NoOpLogger is a dummy slog.Handler that does nothing.
type NoOpLogger struct{}

func (n *NoOpLogger) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (n *NoOpLogger) Handle(_ context.Context, _ slog.Record) error { return nil }
func (n *NoOpLogger) WithAttrs(_ []slog.Attr) slog.Handler         { return n }
func (n *NoOpLogger) WithGroup(_ string) slog.Handler             { return n }

func TestNewScenario(t *testing.T) {
	tests := []struct {
		name        string
		scenarioName string
		platform    string
		testPlan    TestPlan // Use directly
		expectError bool
		expectedIDPrefix string
	}{
		{
			name:        "Happy Path - Valid Scenario",
			scenarioName: "My Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: false,
			expectedIDPrefix: "scenario-",
		},
		{
			name:        "Invalid Scenario Name - Empty",
			scenarioName: "",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
		{
			name:        "Invalid Platform - Empty",
			scenarioName: "My Load Test",
			platform:    "",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
		{
			name:        "Invalid Test Plan - Zero Participants",
			scenarioName: "My Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 0, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
		{
			name:        "Invalid Test Plan - Negative Participants",
			scenarioName: "My Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: -10, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
		{
			name:        "Invalid Test Plan - Zero Duration",
			scenarioName: "My Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 0}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
		{
			name:        "Invalid Test Plan - Negative RampUpSeconds",
			scenarioName: "My Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: -10, DurationSeconds: 300}, // Use directly
			expectError: true,
			expectedIDPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := NewScenario(tt.scenarioName, tt.platform, tt.testPlan) // Use directly
			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if s.Name != tt.scenarioName {
					t.Errorf("expected name %q, got %q", tt.scenarioName, s.Name)
				}
				if s.Platform != tt.platform {
					t.Errorf("expected platform %q, got %q", tt.platform, s.Platform)
				}
				if s.TestPlan != tt.testPlan {
					t.Errorf("expected test plan %v, got %v", tt.testPlan, s.TestPlan)
				}
				if s.Status != "created" {
					t.Errorf("expected status 'created', got %q", s.Status)
				}
				if time.Since(s.CreatedAt) > time.Second {
					t.Errorf("expected CreatedAt to be recent, got %v", s.CreatedAt)
				}
			}
		})
	}
}

func TestService_CreateScenario(t *testing.T) {
	testLogger := slog.New(&NoOpLogger{}) // Create a dummy logger
	service := NewService(testLogger)     // Pass the dummy logger

	tests := []struct {
		name        string
		scenarioName string
		platform    string
		testPlan    TestPlan // Use directly
		expectError bool
		errorMessage string
	}{
		{
			name:        "Happy Path - Valid Scenario",
			scenarioName: "My First Load Test",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError: false,
		},
		{
			name:        "Unsupported Platform",
			scenarioName: "Another Load Test",
			platform:    "unsupported",
			testPlan:    TestPlan{Participants: 50, RampUpSeconds: 30, DurationSeconds: 180}, // Use directly
			expectError: true,
			errorMessage: "unsupported platform adapter: unsupported",
		},
		{
			name:        "Invalid Scenario - Empty Name",
			scenarioName: "",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 10, RampUpSeconds: 10, DurationSeconds: 60}, // Use directly
			expectError: true,
			errorMessage: "scenario name cannot be empty",
		},
		{
			name:        "Invalid Scenario - Zero Participants",
			scenarioName: "Invalid Participants",
			platform:    "jitsi",
			testPlan:    TestPlan{Participants: 0, RampUpSeconds: 10, DurationSeconds: 60}, // Use directly
			expectError: true,
			errorMessage: "participants must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := service.CreateScenario(tt.scenarioName, tt.platform, tt.testPlan)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected an error but got none")
				}
				if err != nil && err.Error() != tt.errorMessage {
					t.Errorf("expected error message %q, got %q", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if s == nil {
					t.Fatal("expected a scenario but got nil")
				}
				if s.Name != tt.scenarioName {
					t.Errorf("expected name %q, got %q", tt.scenarioName, s.Name)
				}
				if s.Platform != tt.platform {
					t.Errorf("expected platform %q, got %q", tt.platform, s.Platform)
				}
				if s.ID == "" {
					t.Errorf("expected scenario ID to be generated, but it's empty")
				}
				if s.Status != "created" {
					t.Errorf("expected scenario status to be 'created', got %q", s.Status)
				}
			}
		})
	}
}
