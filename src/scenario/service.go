package scenario

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
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
	StartExecution(scenarioID string, testPlan TestPlan, adapter platform.PlatformAdapter, onComplete func(scenarioID string, r RunResult)) (ExecutionEngine, error)
}

// Errors of GetQuality.
var (
	ErrScenarioNotFound = errors.New("scenario not found")
	ErrNoQualitySummary = errors.New("no quality summary yet: the scenario has not run, or is running")
)

// Service provides scenario-related operations.
type Service struct {
	scenarios          map[string]*Scenario
	mu                 sync.RWMutex
	supportedPlatforms map[string]bool
	logger             *slog.Logger

	runningEngines   map[string]ExecutionEngine // Map to hold running engines (using the interface)
	engineMu         sync.Mutex                 // Mutex for runningEngines
	executionStarter ExecutionStarter
	catalog          PlatformCatalog // The starter's, if it has one

	reporter Reporter       // Writes each run's report files (009); nil: none
	pending  sync.WaitGroup // Runs whose completion (and report) is not done
}

// shutdownReportWait bounds how long Shutdown waits for the reports of the
// runs it stops.
const shutdownReportWait = 10 * time.Second

// SetReporter sets what writes the report files of each run; nil writes
// none.
func (s *Service) SetReporter(r Reporter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reporter = r
}

// NewService creates a new scenario service.
func NewService(logger *slog.Logger, starter ExecutionStarter) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	supported := map[string]bool{"jitsi": true}
	catalog, _ := starter.(PlatformCatalog)
	if catalog != nil {
		supported = map[string]bool{}
		for _, name := range catalog.Platforms() {
			supported[name] = true
		}
	}
	return &Service{
		scenarios:          make(map[string]*Scenario),
		supportedPlatforms: supported,
		logger:             logger,
		runningEngines:     make(map[string]ExecutionEngine),
		executionStarter:   starter,
		catalog:            catalog,
	}
}

// secretKeys names the secret platform_config keys of platform name.
func (s *Service) secretKeys(name string) []string {
	if s.catalog == nil {
		return nil
	}
	return s.catalog.SecretKeys(name)
}

// redacted returns a snapshot of sc with its secrets hidden.
func (s *Service) redacted(sc *Scenario) *Scenario {
	c := sc.snapshot()
	c.TestPlan = RedactTestPlan(c.TestPlan, s.secretKeys(sc.Platform))
	return c
}

// CreateScenario creates and stores a new scenario.
func (s *Service) CreateScenario(name, platform string, testPlan TestPlan) (*Scenario, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	logged := RedactTestPlan(testPlan, s.secretKeys(platform))
	s.logger.Info("Attempting to create scenario",
		slog.String("name", name),
		slog.String("platform", platform),
		slog.Any("test_plan", logged))

	if !s.supportedPlatforms[platform] {
		s.logger.Warn("Unsupported platform adapter", slog.String("platform", platform))
		return nil, fmt.Errorf("unsupported platform adapter: %s", platform)
	}

	scenario, err := NewScenario(name, platform, testPlan)
	if err != nil {
		s.logger.Error("Failed to create new scenario instance", slog.String("error", err.Error()))
		return nil, err
	}
	if s.catalog != nil {
		if err := s.catalog.ValidateTestPlan(platform, scenario.TestPlan); err != nil {
			s.logger.Warn("Test plan rejected by its platform", slog.String("platform", platform), slog.String("error", err.Error()))
			return nil, err
		}
	}

	scenario.ID = fmt.Sprintf("scenario-%s", uuid.New().String())
	s.scenarios[scenario.ID] = scenario
	s.logger.Info("Scenario created successfully", slog.String("scenario_id", scenario.ID))
	return s.redacted(scenario), nil
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

	if sc.Status != "created" && sc.Status != "completed" && sc.Status != "stopped" && sc.Status != "failed" {
		return fmt.Errorf("scenario cannot be executed in current status '%s'", sc.Status)
	}

	run := sc.runs + 1
	onComplete := func(scenarioID string, r RunResult) {
		defer s.pending.Done()
		s.mu.Lock()
		var snapshot *Scenario
		reporter := s.reporter
		if completedSc, ok := s.scenarios[scenarioID]; ok {
			completedSc.Status = r.Status
			completedSc.quality = r.Quality
			snapshot = s.redacted(completedSc)
			s.logger.Info("Scenario execution completed",
				slog.String("scenario_id", scenarioID),
				slog.String("final_status", r.Status))
		}
		s.mu.Unlock()
		// Remove engine from runningEngines map
		s.engineMu.Lock()
		delete(s.runningEngines, scenarioID)
		s.engineMu.Unlock()

		// The report is written without the lock, so a slow disk does not
		// block readers; it is dropped if a newer run has started since.
		if reporter == nil || snapshot == nil {
			return
		}
		files := reporter.WriteReport(snapshot, r)
		s.mu.Lock()
		defer s.mu.Unlock()
		if sc, ok := s.scenarios[scenarioID]; ok && sc.runs == run {
			sc.report = &files
		}
	}

	// The catalog builds the adapter for the scenario's platform (the
	// platform packages are not imported here: import cycle). Without a
	// catalog, a nil adapter lets the starter pick one from
	// sc.TestPlan.PlatformConfig.
	var adapter platform.PlatformAdapter
	if s.catalog != nil {
		a, err := s.catalog.NewAdapter(sc.Platform, sc.TestPlan)
		if err != nil {
			s.logger.Error("Failed to build platform adapter", slog.String("scenario_id", sc.ID), slog.String("error", err.Error()))
			return fmt.Errorf("failed to start scenario %s: invalid platform_config: %w", sc.ID, err)
		}
		adapter = a
	}
	engine, err := s.executionStarter.StartExecution(sc.ID, sc.TestPlan, adapter, onComplete)
	if err != nil {
		s.logger.Error("Failed to start scenario execution", slog.String("scenario_id", sc.ID), slog.String("error", err.Error()))
		return fmt.Errorf("failed to start scenario %s: %w", sc.ID, err)
	}

	sc.Status = "running"
	sc.quality = nil
	sc.report = nil
	sc.runs = run
	s.pending.Add(1)
	s.logger.Info("Scenario status updated to running", slog.String("scenario_id", sc.ID))

	s.engineMu.Lock()
	s.runningEngines[sc.ID] = engine
	s.engineMu.Unlock()

	go engine.Start()

	return nil
}

// GetScenario retrieves a scenario by ID. It returns a snapshot: the stored
// scenario keeps being updated by its execution.
func (s *Service) GetScenario(id string) (*Scenario, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sc, ok := s.scenarios[id]
	if !ok {
		return nil, fmt.Errorf("scenario not found: %s", id)
	}
	return s.redacted(sc), nil
}

// GetQuality returns the quality summary of the scenario's last run, with
// the scenario's status. It fails with ErrScenarioNotFound, or with
// ErrNoQualitySummary while the scenario has not run or is running.
func (s *Service) GetQuality(id string) (*QualitySummary, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sc, ok := s.scenarios[id]
	if !ok {
		return nil, "", fmt.Errorf("%w: %s", ErrScenarioNotFound, id)
	}
	if sc.Status == "running" || sc.quality == nil {
		return nil, sc.Status, ErrNoQualitySummary
	}
	return sc.quality, sc.Status, nil
}

// Shutdown stops all running scenario engines and waits, up to
// shutdownReportWait, for their runs' reports.
func (s *Service) Shutdown() {
	s.engineMu.Lock()
	s.logger.Info("Initiating service shutdown. Stopping all running scenario engines.")
	for id, engine := range s.runningEngines {
		s.logger.Info("Stopping running scenario engine", slog.String("scenario_id", id))
		engine.Stop()
		delete(s.runningEngines, id)
	}
	s.engineMu.Unlock()
	s.logger.Info("All running scenario engines stopped.")

	done := make(chan struct{})
	go func() {
		s.pending.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownReportWait):
		s.logger.Warn("Shutdown did not wait for every run report", slog.Duration("waited", shutdownReportWait))
	}
}
