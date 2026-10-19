package scenario

import "time"

// RunResult is what the engine hands back when a run ends
// (specs/009-run-report-files/data-model.md).
type RunResult struct {
	// Status is "completed", "stopped" or "failed".
	Status string
	// Error is set when Status is "failed"; empty otherwise.
	Error string
	// StartedAt and EndedAt are UTC; EndedAt is when the summary is final.
	StartedAt time.Time
	EndedAt   time.Time
	// Quality is nil when the run failed before the media collector started.
	Quality *QualitySummary
}

// ReportFile is one report file of a run: exactly one of Path and Error is
// set.
type ReportFile struct {
	Path  string `json:"path,omitempty"`  // Absolute path of the written file
	Error string `json:"error,omitempty"` // Why the file was not written
}

// ReportFiles are the report files of a run
// (specs/009-run-report-files/contracts/api-scenario-report.yaml).
type ReportFiles struct {
	RunID    string     `json:"run_id"`
	JSON     ReportFile `json:"json"`
	Markdown ReportFile `json:"markdown"`
}

// Reporter writes the report files of a run. sc is the scenario's redacted
// snapshot (secret platform_config values hidden). Failures are returned in
// ReportFiles, never as an error: they do not change the run.
type Reporter interface {
	WriteReport(sc *Scenario, r RunResult) ReportFiles
}
