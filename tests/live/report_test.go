//go:build live

package live

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"testing"

	"webrtc-load-engine/src/platform/catalog"
	"webrtc-load-engine/src/report"
	"webrtc-load-engine/src/scenario"
)

// writeReport writes the run's report files (feature 009) into
// $REPORTS_DIR, else the repository's reports folder: go test runs in
// tests/live, so the default is found from the module root. config is the
// adapter's config, reported with its secrets hidden.
func writeReport(t *testing.T, sc *scenario.Scenario, id string, config map[string]any, r scenario.RunResult) {
	t.Helper()
	dir := os.Getenv(report.EnvDir)
	if dir == "" {
		root, err := report.ModuleRootDir(".")
		if err != nil {
			t.Errorf("run report: %v", err)
			return
		}
		dir = filepath.Join(root, "reports")
	}
	dir, err := report.ResolveDir(dir)
	if err != nil {
		t.Errorf("run report: %v", err)
		return
	}
	reported := *sc
	reported.ID = id
	reported.TestPlan.PlatformConfig = config
	reported.TestPlan = scenario.RedactTestPlan(reported.TestPlan, catalog.New(nil).SecretKeys(sc.Platform))
	files := report.Writer{Dir: dir, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}.WriteReport(&reported, r)
	for kind, f := range map[string]scenario.ReportFile{"json": files.JSON, "markdown": files.Markdown} {
		switch {
		case f.Path != "":
			t.Logf("run report (%s): %s", kind, f.Path)
		case f.Error != "":
			t.Errorf("run report (%s) not written: %s", kind, f.Error)
		}
	}
}

// browserFrom reads <prefix>_BROWSER_URL, _BROWSER_HEIGHT, _BROWSER_WINDOW
// and _BROWSER_INSECURE_TLS=1 (015): a headless browser beside the bots,
// opening that address; nil without <prefix>_BROWSER_URL.
func browserFrom(t *testing.T, prefix string) *scenario.BrowserConfig {
	t.Helper()
	u := os.Getenv(prefix + "_BROWSER_URL")
	if u == "" {
		return nil
	}
	return &scenario.BrowserConfig{
		URL:            u,
		ExpectedHeight: envInt(t, prefix+"_BROWSER_HEIGHT", 0),
		Window:         os.Getenv(prefix + "_BROWSER_WINDOW"),
		InsecureTLS:    os.Getenv(prefix+"_BROWSER_INSECURE_TLS") == "1",
	}
}

// serverSettingsFrom reads <prefix>_SERVER_SETTINGS, which sfu-remote deploy
// writes to its settings file (015): present, even empty, it is the
// deployment's statement; absent, the settings are not known to the run.
func serverSettingsFrom(t *testing.T, prefix string) *scenario.ServerSettingsConfig {
	t.Helper()
	v, ok := os.LookupEnv(prefix + "_SERVER_SETTINGS")
	if !ok {
		return nil
	}
	values, err := scenario.ParseServerSettings(v)
	if err != nil {
		t.Fatalf("%s_SERVER_SETTINGS: %v", prefix, err)
	}
	return &scenario.ServerSettingsConfig{Source: scenario.ServerSettingsDeployment, Values: values}
}

// networkFrom reads <prefix>_NETWORK, a JSON file holding the plan's network
// section (016); nil without it.
func networkFrom(t *testing.T, prefix string) *scenario.NetworkConfig {
	t.Helper()
	path := os.Getenv(prefix + "_NETWORK")
	if path == "" {
		return nil
	}
	n, err := scenario.ReadNetworkFile(path)
	if err != nil {
		t.Fatalf("%s_NETWORK: %v", prefix, err)
	}
	return n
}

// logNetwork logs each group's steps and quality, if the plan had a network
// section.
func logNetwork(t *testing.T, n *scenario.NetworkResult) {
	t.Helper()
	if n == nil {
		return
	}
	t.Logf("network: seed %d (%s)", n.Seed, n.SeedSource)
	for _, g := range n.Groups {
		for _, st := range g.Steps {
			t.Logf("  group %s step %d: started %v ended %v during start-up %v", g.Name, st.Index, deref(st.StartedAtSeconds), deref(st.EndedAtSeconds), st.DuringStartUp)
		}
		if q := g.Quality; q != nil && q.Video.RequestedHeightShare != nil {
			t.Logf("  group %s: %d bots, heights %v, requested height share p50 %.3f, freezes %d", g.Name, len(g.Bots), q.Video.HeightShare, q.Video.RequestedHeightShare.P50, q.Video.FreezesTotal)
		}
	}
	if q := n.UnconstrainedQuality; q != nil && q.Video.RequestedHeightShare != nil {
		t.Logf("  no condition: %d bots, heights %v, requested height share p50 %.3f, freezes %d", q.Bots, q.Video.HeightShare, q.Video.RequestedHeightShare.P50, q.Video.FreezesTotal)
	}
}

// conditionedHold returns holdFreezes, or NaN (not checked) when the plan
// put bots under network conditions: the counter covers every bot, and the
// constrained bots' freezes are a result, reported, not a failure (016).
// The other checks then apply to the bots in no group (unconstrained).
func conditionedHold(t *testing.T, n *scenario.NetworkResult, holdFreezes float64) float64 {
	t.Helper()
	if n == nil {
		return holdFreezes
	}
	t.Logf("network conditions: %.0f freezes during the hold, reported, not checked", holdFreezes)
	return math.NaN()
}

// unconstrained returns the summary the requested-height check applies to:
// the bots in no group when the plan has network conditions (016).
func unconstrained(q *scenario.QualitySummary, n *scenario.NetworkResult) *scenario.QualitySummary {
	if n == nil || n.UnconstrainedQuality == nil {
		return q
	}
	u := *n.UnconstrainedQuality
	u.Video.Codec = q.Video.Codec // A run's result, not a group's
	return &u
}

func deref(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}

// logBrowser logs the browser's result, if the plan asked for one. A
// browser that was not measured does not fail the test (FR-019).
func logBrowser(t *testing.T, b *scenario.BrowserResult) {
	t.Helper()
	if b == nil {
		return
	}
	if b.Status == scenario.BrowserUnavailable {
		t.Logf("browser: not measured (%s)", b.Reason)
		return
	}
	line := fmt.Sprintf("browser: %s, %d streams, heights %v, %.0f kbit/s, %d freezes", b.Status, b.VideoStreams, b.HeightShare,
		b.ReceivedBitrateKbps.Average, b.FreezesTotal)
	if b.ExpectedHeightShare != nil {
		line += fmt.Sprintf("; %.3f of the time at %d", *b.ExpectedHeightShare, b.ExpectedHeight)
		if b.SecondsToExpectedHeight != nil {
			line += fmt.Sprintf(", first after %.1f s", *b.SecondsToExpectedHeight)
		}
	}
	if b.CPUThreadsAverage != nil {
		line += fmt.Sprintf("; CPU %.2f threads", *b.CPUThreadsAverage)
	}
	t.Log(line)
}
