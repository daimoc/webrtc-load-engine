package scenario

import (
	"fmt"
	"math"
	"os"
	"slices"
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

	// quality is the summary of the last run, set when it ends (served on
	// its own endpoint, not with the scenario).
	quality *QualitySummary
	// report is where the last run's report files went (009); nil before
	// the first run's report is written and while a run is running.
	report *ReportFiles
	// runs counts the runs started, so a late report is not stored on a
	// newer run.
	runs int
}

// Report returns the report files of the scenario's last run, or nil.
func (s *Scenario) Report() *ReportFiles { return s.report }

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
	// ServerMonitoring names the media server's containers whose CPU and
	// memory the engine samples during the run (platform-independent,
	// specs/007-server-load-report/contracts/test-plan-server.yaml); nil:
	// none.
	ServerMonitoring *ServerMonitoringConfig `json:"server_monitoring,omitempty"`
}

// Limits of the server_monitoring section.
const (
	DefaultServerIntervalSeconds = 5
	maxServerContainers          = 16
	maxServerIntervalSeconds     = 60
)

// ServerMonitoringConfig is the server_monitoring part of a test plan.
type ServerMonitoringConfig struct {
	// Containers are the container names, as `docker ps` lists them.
	Containers []string `json:"containers"`
	// IntervalSeconds is the sampling interval (default 5).
	IntervalSeconds int `json:"interval_seconds,omitempty"`
}

// withDefaults validates m and returns a copy with defaults applied; nil
// stays nil. Docker is not contacted: a wrong name is reported in the run's
// summary, never as an error here.
func (m *ServerMonitoringConfig) withDefaults() (*ServerMonitoringConfig, error) {
	if m == nil {
		return nil, nil
	}
	c := *m
	if len(c.Containers) < 1 || len(c.Containers) > maxServerContainers {
		return nil, fmt.Errorf("server_monitoring.containers must list 1 to %d containers", maxServerContainers)
	}
	seen := map[string]bool{}
	for i, name := range c.Containers {
		if name == "" {
			return nil, fmt.Errorf("server_monitoring.containers: empty name at index %d", i)
		}
		if seen[name] {
			return nil, fmt.Errorf("server_monitoring.containers: duplicate name %q", name)
		}
		seen[name] = true
	}
	c.Containers = slices.Clone(c.Containers)
	switch {
	case c.IntervalSeconds == 0:
		c.IntervalSeconds = DefaultServerIntervalSeconds
	case c.IntervalSeconds < 1 || c.IntervalSeconds > maxServerIntervalSeconds:
		return nil, fmt.Errorf("server_monitoring.interval_seconds must be between 1 and %d, got %d", maxServerIntervalSeconds, c.IntervalSeconds)
	}
	return &c, nil
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
	// Simulcast sends each bot's video as three resolutions at once
	// (audio_video only); nil sends one 640x360 stream.
	Simulcast *SimulcastConfig `json:"simulcast,omitempty"`
	// RequestedHeights sets the video height each bot asks the media
	// server for, as shares of the bots in launch order; nil: the highest
	// height sent.
	RequestedHeights []RequestedHeight `json:"requested_heights,omitempty"`
	// SourceFile is the path, on the engine's host, of a media file the
	// bots send instead of the built-in content: its first audio stream
	// and, for audio_video, its first video stream. The engine transcodes
	// it with ffmpeg when the scenario starts. Empty: built-in content.
	SourceFile string `json:"source_file,omitempty"`
}

// SimulcastConfig is the media.simulcast part of a test plan
// (specs/006-media-quality/contracts/test-plan-quality.yaml).
type SimulcastConfig struct {
	// MaxHeight is the highest layer: 360 (layers 90, 180, 360; the
	// default) or 720 (180, 360, 720).
	MaxHeight int `json:"max_height,omitempty"`
}

// RequestedHeight gives a share of the bots the video height they ask for.
type RequestedHeight struct {
	Height int     `json:"height"`
	Share  float64 `json:"share"`
}

// defaultVideoHeight is the height sent without simulcast (004 content).
const defaultVideoHeight = 360

// HeightFor returns the video height bot i of n asks for: the first
// share × n bots get the first height, and so on; by default the highest
// height sent.
func (m *MediaConfig) HeightFor(i, n int) int {
	if m == nil {
		return defaultVideoHeight
	}
	if len(m.RequestedHeights) == 0 || n <= 0 {
		if m.Simulcast != nil && m.Simulcast.MaxHeight > 0 {
			return m.Simulcast.MaxHeight
		}
		return defaultVideoHeight
	}
	cumulative := 0.0
	for _, r := range m.RequestedHeights {
		cumulative += r.Share
		if float64(i) < math.Round(cumulative*float64(n)) {
			return r.Height
		}
	}
	return m.RequestedHeights[len(m.RequestedHeights)-1].Height
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
	if c.Simulcast != nil {
		if c.Profile != MediaProfileAudioVideo {
			return nil, fmt.Errorf("media.simulcast needs profile %s, got %q", MediaProfileAudioVideo, c.Profile)
		}
		s := *c.Simulcast
		switch s.MaxHeight {
		case 0:
			s.MaxHeight = defaultVideoHeight
		case 360, 720:
		default:
			return nil, fmt.Errorf("media.simulcast.max_height must be 360 or 720, got %d", s.MaxHeight)
		}
		c.Simulcast = &s
	}
	if c.RequestedHeights != nil {
		if len(c.RequestedHeights) == 0 {
			return nil, fmt.Errorf("media.requested_heights cannot be empty")
		}
		sum := 0.0
		for _, r := range c.RequestedHeights {
			if r.Height < 1 {
				return nil, fmt.Errorf("media.requested_heights: height must be at least 1, got %d", r.Height)
			}
			if r.Share <= 0 || r.Share > 1 {
				return nil, fmt.Errorf("media.requested_heights: share must be in (0, 1], got %g", r.Share)
			}
			sum += r.Share
		}
		if math.Abs(sum-1) > 0.001 {
			return nil, fmt.Errorf("media.requested_heights: shares must sum to 1, got %g", sum)
		}
	}
	if c.SourceFile != "" {
		if c.Profile == MediaProfileReceiveOnly {
			return nil, fmt.Errorf("media.source_file needs profile %s or %s", MediaProfileAudio, MediaProfileAudioVideo)
		}
		info, err := os.Stat(c.SourceFile)
		if err != nil {
			return nil, fmt.Errorf("media.source_file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("media.source_file: %s is not a regular file", c.SourceFile)
		}
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
	monitoring, err := testPlan.ServerMonitoring.withDefaults()
	if err != nil {
		return nil, err
	}
	testPlan.ServerMonitoring = monitoring

	return &Scenario{
		Name:      name,
		Platform:  platform,
		TestPlan:  testPlan,
		Status:    "created", // Default status
		CreatedAt: time.Now(),
	}, nil
}

// snapshot returns a copy of the scenario that callers may read without the
// service lock. TestPlan.PlatformConfig, TestPlan.Media and
// TestPlan.ServerMonitoring are shared: they
// are never modified after creation.
func (s *Scenario) snapshot() *Scenario {
	c := *s
	return &c
}
