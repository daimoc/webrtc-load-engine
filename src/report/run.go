// Package report writes the report files of a run: a JSON document and a
// Markdown report (specs/009-run-report-files). It knows no platform: the
// scenario it is given is already redacted.
package report

import (
	"math"
	"runtime"
	"runtime/debug"
	"time"

	"webrtc-load-engine/src/scenario"
)

// FormatVersion is the version of the JSON report's layout.
const FormatVersion = 1

// Run is one run's report (contracts/run-report.schema.yaml).
type Run struct {
	FormatVersion   int                      `json:"format_version"`
	RunID           string                   `json:"run_id"`
	Scenario        Identity                 `json:"scenario"`
	TestPlan        scenario.TestPlan        `json:"test_plan"`
	StartedAt       time.Time                `json:"started_at"`
	EndedAt         time.Time                `json:"ended_at"`
	DurationSeconds float64                  `json:"duration_seconds"`
	Outcome         Outcome                  `json:"outcome"`
	Engine          Engine                   `json:"engine"`
	Quality         *scenario.QualitySummary `json:"quality"`
}

// Identity names the scenario of a run.
type Identity struct {
	ID       string `json:"id"`
	Name     string `json:"name"` // Exactly as given
	Platform string `json:"platform"`
}

// Outcome is how a run ended.
type Outcome struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Engine says which engine build ran.
type Engine struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
}

// NewRun builds the report of a run from sc, whose test plan must already
// be redacted, and the engine's result.
func NewRun(sc *scenario.Scenario, r scenario.RunResult) Run {
	return Run{
		FormatVersion:   FormatVersion,
		RunID:           newRunID(),
		Scenario:        Identity{ID: sc.ID, Name: sc.Name, Platform: sc.Platform},
		TestPlan:        sc.TestPlan,
		StartedAt:       r.StartedAt.UTC(),
		EndedAt:         r.EndedAt.UTC(),
		DurationSeconds: math.Round(r.EndedAt.Sub(r.StartedAt).Seconds()*1000) / 1000,
		Outcome:         Outcome{Status: r.Status, Error: r.Error},
		Engine:          Engine{Version: EngineVersion(), GoVersion: runtime.Version()},
		Quality:         r.Quality,
	}
}

// EngineVersion is the VCS revision the binary was built from (12
// characters, "+dirty" for a modified tree), else the main module's
// version, else "devel" (research R7: go run and go test stamp nothing).
func EngineVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	var revision string
	var modified bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision != "" {
		if len(revision) > 12 {
			revision = revision[:12]
		}
		if modified {
			revision += "+dirty"
		}
		return revision
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	return "devel"
}
