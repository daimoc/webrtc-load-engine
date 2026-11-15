package scenario

import (
	"encoding/json"
	"time"
)

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
	// Browser is the browser's result when the plan asked for one (015);
	// nil otherwise.
	Browser *BrowserResult
	// Network is the network section when the plan has one (016); nil
	// otherwise.
	Network *NetworkResult
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

// The browser's result in a run (015,
// specs/015-bandwidth-estimate-parity/contracts/report.md).

// Browser statuses.
const (
	BrowserMeasured    = "measured"
	BrowserPartial     = "partial" // The browser ended before the run; measures up to then
	BrowserUnavailable = "unavailable"
)

// Reasons the browser is not measured.
const (
	BrowserNotFound    = "browser_not_found"
	BrowserStartFailed = "browser_start_failed"
	PageLoadFailed     = "page_load_failed"
	NoVideoReceived    = "no_video_received"
	BrowserExited      = "browser_exited"
	// ExpectedHeightNotStated: the plan states no expected height.
	ExpectedHeightNotStated = "not_stated"
)

// BrowserSample is one second of the browser's reception.
type BrowserSample struct {
	T       float64 `json:"t"`       // Seconds since the page's load
	Heights []int   `json:"heights"` // Heights of the streams that decoded a frame in this second
	Kbps    float64 `json:"kbps"`    // Video received in this second
}

// BrowserResult is what the browser measured itself during a run.
type BrowserResult struct {
	Status         string  `json:"status"`
	Reason         string  `json:"reason,omitempty"`
	URL            string  `json:"url"`
	Window         string  `json:"window"`
	BrowserVersion string  `json:"browser_version,omitempty"`
	Period         *Period `json:"period,omitempty"`
	VideoStreams   int     `json:"video_streams"`
	// HeightShare is height → share of decoded stream-seconds.
	HeightShare             map[string]float64 `json:"height_share"`
	ExpectedHeight          int                `json:"expected_height,omitempty"`
	ExpectedHeightShare     *float64           `json:"expected_height_share,omitempty"`
	ExpectedHeightReason    string             `json:"expected_height_reason,omitempty"`
	SecondsToExpectedHeight *float64           `json:"seconds_to_expected_height,omitempty"`
	ReceivedBitrateKbps     MeanPeakKbps       `json:"received_bitrate_kbps"`
	FreezesTotal            int64              `json:"freezes_total"`
	FreezeSeconds           float64            `json:"freeze_seconds"`
	// FreezeSource is always "browser": the browser's own freeze rule.
	FreezeSource      string          `json:"freeze_source"`
	CPUThreadsAverage *float64        `json:"cpu_threads_average,omitempty"`
	Samples           []BrowserSample `json:"samples"`
}

// MeanPeakKbps is a bitrate's average and its peak over one second.
type MeanPeakKbps struct {
	Average float64 `json:"average"`
	Peak    float64 `json:"peak"`
}

// MarshalJSON gives only status, reason, url and window when the browser
// was not measured.
func (b BrowserResult) MarshalJSON() ([]byte, error) {
	if b.Status == BrowserUnavailable {
		return json.Marshal(struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
			URL    string `json:"url"`
			Window string `json:"window"`
		}{b.Status, b.Reason, b.URL, b.Window})
	}
	type plain BrowserResult
	p := plain(b)
	if p.HeightShare == nil {
		p.HeightShare = map[string]float64{}
	}
	if p.Samples == nil {
		p.Samples = []BrowserSample{}
	}
	return json.Marshal(p)
}
