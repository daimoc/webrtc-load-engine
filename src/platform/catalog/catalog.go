// Package catalog maps platform names to their adapters (008 research R10):
// how a test plan is checked at creation, which platform_config keys are
// secret, and how a run's adapter is built. The scenario service reaches it
// through the execution starter (scenario.PlatformCatalog), without
// importing any adapter.
package catalog

import (
	"fmt"
	"log/slog"
	"slices"

	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/platform/jitsi"
	"webrtc-load-engine/src/platform/livekit"
	"webrtc-load-engine/src/scenario"
)

// Entry describes one platform.
type Entry struct {
	Name string
	// Validate checks a test plan at creation; nil accepts every plan.
	Validate func(scenario.TestPlan) error
	// SecretKeys are the platform_config keys hidden in answers and logs.
	SecretKeys []string
	// NewAdapter builds the adapter of a run from its platform_config.
	NewAdapter func(logger *slog.Logger, config map[string]any) (platform.PlatformAdapter, error)
}

// Catalog is the set of platforms the engine runs. It implements
// scenario.PlatformCatalog.
type Catalog struct {
	logger  *slog.Logger
	entries []Entry
}

var _ scenario.PlatformCatalog = (*Catalog)(nil)

// New returns the catalog of the platforms built into the engine.
func New(logger *slog.Logger) *Catalog {
	if logger == nil {
		logger = slog.Default()
	}
	return &Catalog{logger: logger, entries: []Entry{jitsiEntry(), livekitEntry()}}
}

// jitsiEntry keeps Jitsi's behaviour: its config is parsed when the run
// starts, not at creation, so plans created before their config is known
// still create.
func jitsiEntry() Entry {
	return Entry{
		Name:       "jitsi",
		SecretKeys: []string{"jwt_app_secret", "jwt"},
		NewAdapter: func(logger *slog.Logger, config map[string]any) (platform.PlatformAdapter, error) {
			a := jitsi.NewAdapter(logger)
			if err := a.ParseConfig(config); err != nil {
				return nil, err
			}
			return a, nil
		},
	}
}

// livekitEntry checks LiveKit plans at creation (008 FR-002, FR-008).
func livekitEntry() Entry {
	return Entry{
		Name:       "livekit",
		Validate:   livekit.Validate,
		SecretKeys: []string{"api_secret", "token"},
		NewAdapter: func(logger *slog.Logger, config map[string]any) (platform.PlatformAdapter, error) {
			cfg, err := livekit.ParseConfig(config)
			if err != nil {
				return nil, err
			}
			return livekit.NewAdapter(logger, cfg), nil
		},
	}
}

func (c *Catalog) entry(name string) (Entry, bool) {
	i := slices.IndexFunc(c.entries, func(e Entry) bool { return e.Name == name })
	if i < 0 {
		return Entry{}, false
	}
	return c.entries[i], true
}

// Platforms implements scenario.PlatformCatalog.
func (c *Catalog) Platforms() []string {
	names := make([]string, len(c.entries))
	for i, e := range c.entries {
		names[i] = e.Name
	}
	return names
}

// ValidateTestPlan implements scenario.PlatformCatalog.
func (c *Catalog) ValidateTestPlan(name string, tp scenario.TestPlan) error {
	e, ok := c.entry(name)
	if !ok {
		return fmt.Errorf("unsupported platform adapter: %s", name)
	}
	if e.Validate == nil {
		return nil
	}
	return e.Validate(tp)
}

// SecretKeys implements scenario.PlatformCatalog.
func (c *Catalog) SecretKeys(name string) []string {
	e, _ := c.entry(name)
	return e.SecretKeys
}

// NewAdapter implements scenario.PlatformCatalog.
func (c *Catalog) NewAdapter(name string, tp scenario.TestPlan) (platform.PlatformAdapter, error) {
	e, ok := c.entry(name)
	if !ok {
		return nil, fmt.Errorf("unsupported platform adapter: %s", name)
	}
	return e.NewAdapter(c.logger, tp.PlatformConfig)
}
