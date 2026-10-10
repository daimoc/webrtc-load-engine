package scenario

import (
	"fmt"
	"time"
)

// Scenario represents a load test configuration.
type Scenario struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Platform  string    `json:"platform"`
	TestPlan  TestPlan  `json:"test_plan"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// TestPlan defines the load test parameters.
type TestPlan struct {
	Participants    int `json:"participants"`
	RampUpSeconds   int `json:"ramp_up_seconds"`
	DurationSeconds int `json:"duration_seconds"`
	// PlatformConfig holds adapter-specific settings; its schema is owned by the
	// platform adapter (see specs/003-xmpp-signaling/contracts/configuration.yaml for Jitsi).
	PlatformConfig map[string]any `json:"platform_config,omitempty"`
	// Media says what each bot sends (platform-independent). NewScenario sets
	// it, with defaults, so it is never nil on a created scenario.
	Media *MediaConfig `json:"media,omitempty"`
}

// Media profiles (specs/004-synthetic-media/contracts/test-plan-media.yaml).
const (
	MediaProfileReceiveOnly = "receive_only"
	MediaProfileAudio       = "audio"
	MediaProfileAudioVideo  = "audio_video"
)

// defaultStallTimeoutSeconds is used when stall_timeout_seconds is not set.
const defaultStallTimeoutSeconds = 10

// MediaConfig is the media part of a test plan.
type MediaConfig struct {
	Profile             string `json:"profile,omitempty"`
	StallTimeoutSeconds int    `json:"stall_timeout_seconds,omitempty"`
	// MaxReceivedStreams caps the remote streams each bot receives per
	// media kind; 0 means no cap.
	MaxReceivedStreams int `json:"max_received_streams,omitempty"`
}

// Kinds returns the media kinds each bot sends, in a stable order; nil for
// receive-only (or a nil config).
func (m *MediaConfig) Kinds() []string {
	if m == nil {
		return nil
	}
	switch m.Profile {
	case MediaProfileAudio:
		return []string{"audio"}
	case MediaProfileAudioVideo:
		return []string{"audio", "video"}
	}
	return nil
}

// withDefaults validates m and returns a copy with defaults applied. A nil
// config means receive-only.
func (m *MediaConfig) withDefaults() (*MediaConfig, error) {
	c := MediaConfig{}
	if m != nil {
		c = *m
	}
	switch c.Profile {
	case "":
		c.Profile = MediaProfileReceiveOnly
	case MediaProfileReceiveOnly, MediaProfileAudio, MediaProfileAudioVideo:
	default:
		return nil, fmt.Errorf("media.profile must be one of %s, %s, %s, got %q",
			MediaProfileReceiveOnly, MediaProfileAudio, MediaProfileAudioVideo, c.Profile)
	}
	switch {
	case c.StallTimeoutSeconds == 0:
		c.StallTimeoutSeconds = defaultStallTimeoutSeconds
	case c.StallTimeoutSeconds < 0:
		return nil, fmt.Errorf("media.stall_timeout_seconds must be greater than 0")
	}
	if c.MaxReceivedStreams < 0 {
		return nil, fmt.Errorf("media.max_received_streams cannot be negative")
	}
	return &c, nil
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
	media, err := testPlan.Media.withDefaults()
	if err != nil {
		return nil, err
	}
	testPlan.Media = media

	return &Scenario{
		Name:      name,
		Platform:  platform,
		TestPlan:  testPlan,
		Status:    "created", // Default status
		CreatedAt: time.Now(),
	}, nil
}

// snapshot returns a copy of the scenario that callers may read without the
// service lock. TestPlan.PlatformConfig and TestPlan.Media are shared: they
// are never modified after creation.
func (s *Scenario) snapshot() *Scenario {
	c := *s
	return &c
}
