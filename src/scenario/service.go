package scenario

import (
	"fmt"
	"log/slog" // Import slog
	"sync"
	// "time" // Removed unused import

	"github.com/google/uuid"
)

// Service provides scenario-related operations.
type Service struct {
	scenarios map[string]*Scenario
	mu        sync.RWMutex
	supportedPlatforms map[string]bool
	logger    *slog.Logger // Add logger field
}

// NewService creates a new scenario service.
func NewService(logger *slog.Logger) *Service { // Accept logger as argument
	if logger == nil {
		logger = slog.Default() // Use default logger if none provided
	}
	return &Service{
		scenarios: make(map[string]*Scenario),
		supportedPlatforms: map[string]bool{
			"jitsi": true, // Example supported platform
		},
		logger: logger,
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