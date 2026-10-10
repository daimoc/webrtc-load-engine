package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"webrtc-load-engine/src/platform"
)

// NoOpLogger is a dummy slog.Handler that does nothing.
type NoOpLogger struct{}

func (n *NoOpLogger) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (n *NoOpLogger) Handle(_ context.Context, _ slog.Record) error { return nil }
func (n *NoOpLogger) WithAttrs(_ []slog.Attr) slog.Handler          { return n }
func (n *NoOpLogger) WithGroup(_ string) slog.Handler               { return n }

func TestNewScenario(t *testing.T) {
	tests := []struct {
		name             string
		scenarioName     string
		platform         string
		testPlan         TestPlan // Use directly
		expectError      bool
		expectedIDPrefix string
	}{
		{
			name:             "Happy Path - Valid Scenario",
			scenarioName:     "My Load Test",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:      false,
			expectedIDPrefix: "scenario-",
		},
		{
			name:             "Invalid Scenario Name - Empty",
			scenarioName:     "",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:      true,
			expectedIDPrefix: "",
		},
		{
			name:             "Invalid Platform - Empty",
			scenarioName:     "My Load Test",
			platform:         "",
			testPlan:         TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:      true,
			expectedIDPrefix: "",
		},
		{
			name:             "Invalid Test Plan - Zero Participants",
			scenarioName:     "My Load Test",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: 0, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:      true,
			expectedIDPrefix: "",
		},
		{
			name:             "Invalid Test Plan - Negative Participants",
			scenarioName:     "My Load Test",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: -10, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:      true,
			expectedIDPrefix: "",
		},
		{
			name:             "Invalid Test Plan - Zero Duration",
			scenarioName:     "My Load Test",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 0}, // Use directly
			expectError:      true,
			expectedIDPrefix: "",
		},
		{
			name:             "Invalid Test Plan - Negative RampUpSeconds",
			scenarioName:     "My Load Test",
			platform:         "jitsi",
			testPlan:         TestPlan{Participants: 100, RampUpSeconds: -10, DurationSeconds: 300}, // Use directly
			expectError:      true,
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
				want := tt.testPlan
				want.Media = &MediaConfig{Profile: MediaProfileReceiveOnly, StallTimeoutSeconds: defaultStallTimeoutSeconds}
				if !reflect.DeepEqual(s.TestPlan, want) {
					t.Errorf("expected test plan %+v, got %+v", want, s.TestPlan)
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
	testLogger := slog.New(&NoOpLogger{})  // Create a dummy logger
	service := NewService(testLogger, nil) // Pass the dummy logger and nil starter

	tests := []struct {
		name         string
		scenarioName string
		platform     string
		testPlan     TestPlan // Use directly
		expectError  bool
		errorMessage string
	}{
		{
			name:         "Happy Path - Valid Scenario",
			scenarioName: "My First Load Test",
			platform:     "jitsi",
			testPlan:     TestPlan{Participants: 100, RampUpSeconds: 60, DurationSeconds: 300}, // Use directly
			expectError:  false,
		},
		{
			name:         "Unsupported Platform",
			scenarioName: "Another Load Test",
			platform:     "unsupported",
			testPlan:     TestPlan{Participants: 50, RampUpSeconds: 30, DurationSeconds: 180}, // Use directly
			expectError:  true,
			errorMessage: "unsupported platform adapter: unsupported",
		},
		{
			name:         "Invalid Scenario - Empty Name",
			scenarioName: "",
			platform:     "jitsi",
			testPlan:     TestPlan{Participants: 10, RampUpSeconds: 10, DurationSeconds: 60}, // Use directly
			expectError:  true,
			errorMessage: "scenario name cannot be empty",
		},
		{
			name:         "Invalid Scenario - Zero Participants",
			scenarioName: "Invalid Participants",
			platform:     "jitsi",
			testPlan:     TestPlan{Participants: 0, RampUpSeconds: 10, DurationSeconds: 60}, // Use directly
			expectError:  true,
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

func TestTestPlan_PlatformConfigJSONRoundTrip(t *testing.T) {
	in := `{"participants":2,"ramp_up_seconds":0,"duration_seconds":5,"platform_config":{"xmpp_domain":"meet.example.org","room_name":"load1","debug":true}}`

	var tp TestPlan
	if err := json.Unmarshal([]byte(in), &tp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := tp.PlatformConfig["xmpp_domain"]; got != "meet.example.org" {
		t.Errorf("expected xmpp_domain to be decoded, got %v", got)
	}
	if got := tp.PlatformConfig["debug"]; got != true {
		t.Errorf("expected debug to be decoded, got %v", got)
	}

	out, err := json.Marshal(tp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"platform_config":{`) {
		t.Errorf("expected platform_config in JSON output, got %s", out)
	}
}

// failingStarter is an ExecutionStarter that always refuses to start.
type failingStarter struct{ err error }

func (f *failingStarter) StartExecution(_ string, _ TestPlan, _ platform.PlatformAdapter, _ func(string, string)) (ExecutionEngine, error) {
	return nil, f.err
}

func TestService_ExecuteScenario_StarterErrorResetsStatus(t *testing.T) {
	starterErr := errors.New("xmpp_domain is required")
	service := NewService(slog.New(&NoOpLogger{}), &failingStarter{err: starterErr})

	sc, err := service.CreateScenario("cfg-less", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = service.ExecuteScenario(sc.ID)
	if !errors.Is(err, starterErr) {
		t.Fatalf("expected starter error to be returned, got %v", err)
	}

	got, _ := service.GetScenario(sc.ID)
	if got.Status != "created" {
		t.Errorf("expected status to be reset to 'created' after failed start, got %q", got.Status)
	}
	if len(service.runningEngines) != 0 {
		t.Errorf("expected no running engine to be registered, got %d", len(service.runningEngines))
	}
}

// completingStarter starts engines that complete as soon as they start.
type completingStarter struct{}

type completingEngine struct {
	id         string
	onComplete func(string, string)
}

func (e *completingEngine) Start() { e.onComplete(e.id, "completed") }
func (e *completingEngine) Stop()  {}

func (completingStarter) StartExecution(id string, _ TestPlan, _ platform.PlatformAdapter, onComplete func(string, string)) (ExecutionEngine, error) {
	return &completingEngine{id: id, onComplete: onComplete}, nil
}

// Callers read the returned scenario without the service lock (the API
// encodes it) while the engine updates the status: they must get a snapshot.
func TestService_ScenarioReadsDoNotRaceWithExecution(t *testing.T) {
	service := NewService(slog.New(&NoOpLogger{}), completingStarter{})
	created, err := service.CreateScenario("race", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 50; i++ {
			if err := service.ExecuteScenario(created.ID); err != nil && !strings.Contains(err.Error(), "already running") {
				t.Errorf("execute: %v", err)
			}
		}
	}()
	for i := 0; i < 200; i++ {
		got, err := service.GetScenario(created.ID)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		_ = got.Status
		_ = created.Status
	}
	<-done

	// The last run's engine starts asynchronously (go engine.Start()).
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := service.GetScenario(created.ID)
		if got.Status == "completed" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected the last run to have completed, got %q", got.Status)
		}
		time.Sleep(time.Millisecond)
	}
}

// stoppingStarter starts engines that report being stopped on request.
type stoppingStarter struct{}

func (stoppingStarter) StartExecution(id string, _ TestPlan, _ platform.PlatformAdapter, onComplete func(string, string)) (ExecutionEngine, error) {
	return &stoppedEngine{id: id, onComplete: onComplete}, nil
}

type stoppedEngine struct {
	id         string
	onComplete func(string, string)
}

func (e *stoppedEngine) Start() { e.onComplete(e.id, "stopped") }
func (e *stoppedEngine) Stop()  {}

func TestService_StoppedScenarioCanBeExecutedAgain(t *testing.T) {
	service := NewService(slog.New(&NoOpLogger{}), stoppingStarter{})
	sc, err := service.CreateScenario("stop", "jitsi", TestPlan{Participants: 1, DurationSeconds: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := service.ExecuteScenario(sc.ID); err != nil {
		t.Fatalf("execute: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, _ := service.GetScenario(sc.ID)
		if got.Status == "stopped" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected status stopped, got %q", got.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := service.ExecuteScenario(sc.ID); err != nil {
		t.Errorf("a stopped scenario must be executable again, got %v", err)
	}
}
