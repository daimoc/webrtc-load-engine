package report

import (
	"fmt"
	"strings"
	"testing"

	"webrtc-load-engine/src/scenario"
)

// Report additions of 015 (specs/015-bandwidth-estimate-parity/contracts/report.md).

func markdown015(t *testing.T, tp func(*scenario.TestPlan), q func(*scenario.QualitySummary), b *scenario.BrowserResult) string {
	t.Helper()
	sc := testScenario()
	if tp != nil {
		tp(&sc.TestPlan)
	}
	r := testResult()
	r.Quality = loadSummary(t, "full.json")
	if q != nil {
		q(r.Quality)
	}
	r.Browser = b
	return string(Markdown(NewRun(sc, r)))
}

func TestMarkdownServerSettings(t *testing.T) {
	cases := []struct {
		settings *scenario.ServerSettingsConfig
		want     string
	}{
		{nil, "not known to the run"},
		{&scenario.ServerSettingsConfig{Source: scenario.ServerSettingsDeployment}, "none changed by the deployment"},
		{&scenario.ServerSettingsConfig{Source: scenario.ServerSettingsDeployment, Values: map[string]string{"JVB_CC_TRUST_BWE": "false"}},
			"JVB_CC_TRUST_BWE=false (stated by the deployment)"},
		{&scenario.ServerSettingsConfig{Source: scenario.ServerSettingsOperator, Values: map[string]string{"B": "2", "A": "1"}},
			"A=1, B=2 (stated by the operator)"},
	}
	for _, tc := range cases {
		md := markdown015(t, func(tp *scenario.TestPlan) { tp.ServerSettings = tc.settings }, nil, nil)
		if got := row(t, md, "Server settings"); got != tc.want {
			t.Errorf("want %q, got %q", tc.want, got)
		}
		if strings.Contains(row(t, md, "Server settings"), "default") {
			t.Errorf("the row says defaults: %q", row(t, md, "Server settings"))
		}
	}
}

func reception(flagged int) func(*scenario.QualitySummary) {
	return func(q *scenario.QualitySummary) {
		r := scenario.ReceptionReports{Available: true, Bots: 2,
			AcknowledgedShare: &scenario.Percentiles{P50: 1, P95: 1, P99: 0.97},
			LateShare:         &scenario.Percentiles{P50: 0, P95: 0.004, P99: 0.03},
			StampLagMaxMs:     &scenario.Percentiles{P50: 0.4, P95: 2.1, P99: 38},
			Limits:            scenario.ReceptionLimits{AcknowledgedShareMin: 0.99, LateShareMax: 0.01, LateIntervalMs: 200},
			PerBot: []scenario.BotReception{
				{PeerID: "bot-1", PacketsArrived: 100, PacketsAcknowledged: 100, AcknowledgedShare: 1, FeedbacksSent: 10},
			}}
		for i := 0; i < flagged; i++ {
			id := fmt.Sprintf("bot-f%d", i)
			r.Flagged = append(r.Flagged, id)
			r.PerBot = append(r.PerBot, scenario.BotReception{PeerID: id, PacketsArrived: 100, PacketsAcknowledged: 97,
				AcknowledgedShare: 0.97, FeedbacksSent: 10, IntervalMs: scenario.MeanMax{Mean: 100.3, Max: 412}, LateShare: 0.03,
				StampLagMs: scenario.MeanMax{Mean: 0.6, Max: 38}})
		}
		q.ReceptionReports = r
	}
}

func TestMarkdownReceptionReports(t *testing.T) {
	md := markdown015(t, nil, reception(0), nil)
	if got := row(t, md, "Reception reports"); got != "acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged" {
		t.Errorf("row %q", got)
	}
	if strings.Contains(md, "Warning:") {
		t.Error("warning without a flagged bot")
	}
	sections := []string{"## Video", "## Reception reports", "## Server"}
	last := -1
	for _, h := range sections {
		i := strings.Index(md, "\n"+h+"\n")
		if i < last {
			t.Fatalf("%s out of order (%d < %d)", h, i, last)
		}
		last = i
	}
	if !strings.Contains(md, "| Acknowledged share | 100.00 % | 100.00 % | 97.00 % |") || !strings.Contains(md, "0.99") {
		t.Errorf("percentiles or limits missing:\n%s", md)
	}
}

func TestMarkdownReceptionFlagged(t *testing.T) {
	md := markdown015(t, nil, reception(1), nil)
	if got := row(t, md, "Reception reports"); got != "acknowledged 100.00 % (lowest bot 97.00 %) · 1 bot flagged" {
		t.Errorf("row %q", got)
	}
	video := md[strings.Index(md, "\n## Video\n"):strings.Index(md, "\n## Reception reports\n")]
	warning := "> Warning: 1 bot reported its reception incompletely or late (see Reception\n> reports). Layered results of this run can reflect the bots."
	if !strings.Contains(video, warning) {
		t.Errorf("no warning in the Video section:\n%s", video)
	}
	if !strings.Contains(md, "| bot-f0 | 97.00 % | 3.00 % | 412.0 ms | 38.0 ms |") {
		t.Errorf("flagged bot row missing:\n%s", md)
	}
}

func TestMarkdownReceptionManyFlagged(t *testing.T) {
	md := markdown015(t, nil, reception(12), nil)
	if !strings.Contains(md, "| bot-f9 |") || strings.Contains(md, "| bot-f10 |") || !strings.Contains(md, "and 2 more") {
		t.Errorf("want 10 bots then \"and 2 more\":\n%s", md)
	}
	if !strings.Contains(md, "> Warning: 12 bots reported") {
		t.Error("plural warning missing")
	}
}

func TestMarkdownReceptionNotMeasured(t *testing.T) {
	for _, reason := range []string{scenario.ReceptionNoVideo, scenario.ReceptionNotNegotiated, scenario.ReceptionNotCounted} {
		md := markdown015(t, nil, func(q *scenario.QualitySummary) { q.ReceptionReports = scenario.ReceptionReports{Reason: reason} }, nil)
		if got := row(t, md, "Reception reports"); got != "not measured: "+reason {
			t.Errorf("%s: row %q", reason, got)
		}
		section := md[strings.Index(md, "\n## Reception reports\n"):]
		if !strings.Contains(section, "Not measured: "+reason+".") {
			t.Errorf("%s: section %q", reason, section[:80])
		}
	}
}

func measuredBrowser() *scenario.BrowserResult {
	share, secs, cpu := 0.94, 3.0, 0.82
	return &scenario.BrowserResult{Status: scenario.BrowserMeasured, URL: "https://h/r", Window: "1280x720",
		BrowserVersion: "Chrome/156", VideoStreams: 4, HeightShare: map[string]float64{"720": 0.94, "180": 0.06},
		ExpectedHeight: 720, ExpectedHeightShare: &share, SecondsToExpectedHeight: &secs,
		ReceivedBitrateKbps: scenario.MeanPeakKbps{Average: 5120, Peak: 6300}, FreezeSource: "browser",
		CPUThreadsAverage: &cpu}
}

func TestMarkdownBrowser(t *testing.T) {
	md := markdown015(t, nil, nil, measuredBrowser())
	if got := row(t, md, "Browser"); got != "720 lines 94.00 % of the time · reached after 3.0 s · 0 freezes" {
		t.Errorf("row %q", got)
	}
	lm, br := strings.Index(md, "\n## Load machine\n"), strings.Index(md, "\n## Browser\n")
	if br < 0 || br < lm || br > strings.Index(md, "\n## Worst bots\n") {
		t.Fatalf("Browser section not between Load machine and Worst bots:\n%s", md)
	}
	if !strings.Contains(md[lm:br], "A browser ran on this machine during the run: 0.82 threads on average, not counted in the engine's figure.") {
		t.Errorf("load machine line missing:\n%s", md[lm:br])
	}
	section := md[br:]
	for _, want := range []string{"https://h/r", "1280x720", "Chrome/156", "| 720 | 94.00 % |", "5120 kbit/s", "as counted by the browser"} {
		if !strings.Contains(section, want) {
			t.Errorf("section lacks %q:\n%s", want, section)
		}
	}
}

func TestMarkdownBrowserVariants(t *testing.T) {
	never := measuredBrowser()
	never.SecondsToExpectedHeight = nil
	zero := 0.0
	never.ExpectedHeightShare = &zero
	if got := row(t, markdown015(t, nil, nil, never), "Browser"); got != "720 lines 0.00 % of the time · not reached · 0 freezes" {
		t.Errorf("never reached: %q", got)
	}
	noHeight := measuredBrowser()
	noHeight.ExpectedHeight, noHeight.ExpectedHeightShare, noHeight.SecondsToExpectedHeight = 0, nil, nil
	noHeight.ExpectedHeightReason = scenario.ExpectedHeightNotStated
	if got := row(t, markdown015(t, nil, nil, noHeight), "Browser"); got != "720: 94.00 %, 180: 6.00 % · no expected height stated · 0 freezes" {
		t.Errorf("no expected height: %q", got)
	}
	partial := measuredBrowser()
	partial.Status, partial.Reason = scenario.BrowserPartial, scenario.BrowserExited
	if got := row(t, markdown015(t, nil, nil, partial), "Browser"); !strings.HasPrefix(got, "partial (browser_exited): 720 lines") {
		t.Errorf("partial: %q", got)
	}
	for _, reason := range []string{scenario.BrowserNotFound, scenario.BrowserStartFailed, scenario.PageLoadFailed, scenario.NoVideoReceived, scenario.BrowserExited} {
		md := markdown015(t, nil, nil, &scenario.BrowserResult{Status: scenario.BrowserUnavailable, Reason: reason, URL: "https://h/r", Window: "1280x720"})
		if got := row(t, md, "Browser"); got != "not measured: "+reason {
			t.Errorf("%s: row %q", reason, got)
		}
		if strings.Contains(md, "A browser ran on this machine") {
			t.Errorf("%s: load machine line without a measured browser", reason)
		}
		if !strings.Contains(md[strings.Index(md, "\n## Browser\n"):], "Not measured: "+reason+".") {
			t.Errorf("%s: section without the reason", reason)
		}
	}
}

func TestMarkdownNoBrowser(t *testing.T) {
	md := markdown015(t, nil, nil, nil)
	if strings.Contains(md, "| Browser |") || strings.Contains(md, "\n## Browser\n") || strings.Contains(md, "A browser ran") {
		t.Errorf("browser parts in a run without one:\n%s", md)
	}
}
