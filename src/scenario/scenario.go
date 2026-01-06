package scenario

import (
	"fmt"
	"time"
)

// Scenario represents a load test configuration.
type Scenario struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Platform string    `json:"platform"`
	TestPlan TestPlan  `json:"test_plan"`
	Status   string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TestPlan defines the load test parameters.
type TestPlan struct {
	Participants   int `json:"participants"`
	RampUpSeconds  int `json:"ramp_up_seconds"`
	DurationSeconds int `json:"duration_seconds"`
}

// NewScenario creates a new Scenario instance with default status and creation timestamp.
func NewScenario(name, platform string, testPlan TestPlan) (*Scenario, error) {
	if name == "" {
		return nil, fmt.Errorf("scenario name cannot be empty")
	}
	if platform == "" {
		return nil, fmt.Errorf("platform cannot be empty")
	}
	if testPlan.Participants <= 0 {
		return nil, fmt.Errorf("participants must be greater than 0")
	}
	if testPlan.DurationSeconds <= 0 {
		return nil, fmt.Errorf("duration_seconds must be greater than 0")
	}
	if testPlan.RampUpSeconds < 0 {
		return nil, fmt.Errorf("ramp_up_seconds cannot be negative")
	}

	return &Scenario{
		Name:     name,
		Platform: platform,
		TestPlan: testPlan,
		Status:   "created", // Default status
		CreatedAt: time.Now(),
	}, nil
}
