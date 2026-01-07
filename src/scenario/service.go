package scenario

import (
	"fmt"
	"log/slog"
	"sync"
	"webrtc-load-engine/src/platform"

	"github.com/google/uuid"
)

// ExecutionEngine defines the interface that the scenario service uses to interact with the execution engine.
type ExecutionEngine interface {
	Start()
	Stop()
}

// ExecutionStarter defines the interface for starting a scenario execution.
type ExecutionStarter interface {
	StartExecution(scenarioID string, testPlan TestPlan, adapter platform.PlatformAdapter, onComplete func(scenarioID string, status string)) ExecutionEngine
}

// Service provides scenario-related operations.
type Service struct {
	scenarios map[string]*Scenario
	mu        sync.RWMutex
	supportedPlatforms map[string]bool
	logger    *slog.Logger

	runningEngines map[string]ExecutionEngine // Map to hold running engines (using the interface)
	engineMu       sync.Mutex                 // Mutex for runningEngines
	executionStarter ExecutionStarter
}

// NewService creates a new scenario service.
func NewService(logger *slog.Logger, starter ExecutionStarter) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		scenarios: make(map[string]*Scenario),
		supportedPlatforms: map[string]bool{
			"jitsi": true,
		},
		logger: logger,
		runningEngines: make(map[string]ExecutionEngine),
		executionStarter: starter,
	}
}

// CreateScenario creates and stores a new scenario.
func (s *Service) CreateScenario(name, platform string, testPlan TestPlan) (*Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Attempting to create scenario",
		slog.String("name", name),
		slog.String("platform", platform),
		slog.Any("test_plan", testPlan))

	if !s.supportedPlatforms[platform] {
		s.logger.Warn("Unsupported platform adapter", slog.String("platform", platform))
		return nil, fmt.Errorf("unsupported platform adapter: %s", platform)
	}

	scenario, err := NewScenario(name, platform, testPlan)
	if err != nil {
		s.logger.Error("Failed to create new scenario instance", slog.String("error", err.Error()))
		return nil, err
	}

	scenario.ID = fmt.Sprintf("scenario-%s", uuid.New().String())
	s.scenarios[scenario.ID] = scenario
	s.logger.Info("Scenario created successfully", slog.String("scenario_id", scenario.ID))
	return scenario, nil
}

// ExecuteScenario retrieves a scenario by ID and starts its execution.
func (s *Service) ExecuteScenario(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sc, ok := s.scenarios[id]
	if !ok {
		return fmt.Errorf("scenario not found: %s", id)
	}

	if sc.Status == "running" {
		return fmt.Errorf("scenario is already running: %s", id)
	}

	if sc.Status != "created" && sc.Status != "completed" && sc.Status != "failed" {
		return fmt.Errorf("scenario cannot be executed in current status '%s'", sc.Status)
	}

	sc.Status = "running"
	s.logger.Info("Scenario status updated to running", slog.String("scenario_id", sc.ID))

	// Create a platform adapter (currently hardcoded to Jitsi placeholder)
	// TODO: Dynamically select adapter based on sc.Platform
	// adapter := jitsi.NewAdapter(s.logger) // Removed direct import

	onComplete := func(scenarioID string, status string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if completedSc, ok := s.scenarios[scenarioID]; ok {
			completedSc.Status = status
			s.logger.Info("Scenario execution completed",
				slog.String("scenario_id", scenarioID),
				slog.String("final_status", status))
		}
		// Remove engine from runningEngines map
		s.engineMu.Lock()
		delete(s.runningEngines, scenarioID)
		s.engineMu.Unlock()
	}

	// Delegate engine creation and start to the starter
	// The adapter will also need to be created by the starter to avoid import cycles here.
	engine := s.executionStarter.StartExecution(sc.ID, sc.TestPlan, nil, onComplete) // Pass nil adapter for now.
	
	s.engineMu.Lock()
	s.runningEngines[sc.ID] = engine
	s.engineMu.Unlock()

	go engine.Start()

	return nil
}

// GetScenario retrieves a scenario by ID.
func (s *Service) GetScenario(id string) (*Scenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sc, ok := s.scenarios[id]
	if !ok {
		return nil, fmt.Errorf("scenario not found: %s", id)
	}
	return sc, nil
}

// Shutdown stops all running scenario engines.
func (s *Service) Shutdown() {
	s.engineMu.Lock()
	defer s.engineMu.Unlock()

	s.logger.Info("Initiating service shutdown. Stopping all running scenario engines.")

	for id, engine := range s.runningEngines {
		s.logger.Info("Stopping running scenario engine", slog.String("scenario_id", id))
		engine.Stop()
		delete(s.runningEngines, id)
	}
	s.logger.Info("All running scenario engines stopped.")
}