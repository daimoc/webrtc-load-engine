//go:build live

package live

import (
	"io"
	"log/slog"
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
