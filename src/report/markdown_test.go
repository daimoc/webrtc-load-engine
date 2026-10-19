package report

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// loadSummary reads a recorded quality summary from testdata.
func loadSummary(t *testing.T, name string) *scenario.QualitySummary {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var q scenario.QualitySummary
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	return &q
}

func markdownRun(t *testing.T, q *scenario.QualitySummary, status, errText string) (Run, string) {
	t.Helper()
	sc := testScenario()
	sc.Name = "ten bots"
	sc.TestPlan.RampUpSeconds = 5
	r := testResult()
	r.Status, r.Error, r.Quality = status, errText, q
	run := NewRun(sc, r)
	return run, string(Markdown(run))
}

// row returns the value cell of the key-figures row named label.
func row(t *testing.T, md, label string) string {
	t.Helper()
	for _, line := range strings.Split(md, "\n") {
		if p := "| " + label + " | "; strings.HasPrefix(line, p) {
			return strings.TrimSuffix(strings.TrimPrefix(line, p), " |")
		}
	}
	t.Fatalf("no key-figures row %q in:\n%s", label, md)
	return ""
}

func TestMarkdownSections(t *testing.T) {
	_, md := markdownRun(t, loadSummary(t, "full.json"), "completed", "")
	if !strings.HasPrefix(md, "# Load test report: ten bots\n") {
		t.Errorf("title: %q", md[:min(60, len(md))])
	}
	last := -1
	for _, h := range []string{"## Key figures", "## Audio", "## Video", "## Server", "## Load machine", "## Worst bots", "## Test plan"} {
		i := strings.Index(md, "\n"+h+"\n")
		if i < 0 || i < last {
			t.Fatalf("section %q missing or out of order:\n%s", h, md)
		}
		last = i
	}
	if !strings.HasSuffix(strings.TrimSpace(md), fmt.Sprintf("Engine %s (%s)", EngineVersion(), runtime.Version())) {
		t.Errorf("footer: %q", md[max(0, len(md)-80):])
	}
}

// The key figures equal the JSON report's values, rounded for display
// (SC-003).
func TestMarkdownKeyFigures(t *testing.T) {
	q := loadSummary(t, "full.json")
	run, md := markdownRun(t, q, "completed", "")
	checks := map[string]string{
		"Platform":                    "jitsi",
		"Participants":                fmt.Sprintf("10 (bots measured: %d)", q.Bots),
		"Test duration":               fmt.Sprintf("60 s (ramp-up 5 s); run %.3f s", run.DurationSeconds),
		"Outcome":                     "completed",
		"Audio MOS":                   fmt.Sprintf("median %.2f, worst bot %.2f", q.Audio.MOS.P50, q.Worst.LowestMOS[0].Value),
		"Audio concealed":             fmt.Sprintf("p95 %.2f %%", q.Audio.ConcealedRatio.P95*100),
		"Video freezes":               fmt.Sprintf("%d in total; freeze time p95 %.3f s", q.Video.FreezesTotal, q.Video.FreezeSeconds.P95),
		"Video frame rate":            fmt.Sprintf("%.1f / %.1f / %.1f fps (p50 / p95 / p99)", q.Video.FrameRate.P50, q.Video.FrameRate.P95, q.Video.FrameRate.P99),
		"Video resolution":            "360: 100.00 %",
		"Latency p50 (audio / video)": fmt.Sprintf("%.1f ms / %.1f ms", q.Audio.Latency.P50*1000, q.Video.Latency.P50*1000),
		"Server CPU (total)":          fmt.Sprintf("avg %.2f threads, peak %.2f threads", q.Server.Total.CPUThreads.Average, q.Server.Total.CPUThreads.Peak),
		"Server memory (total)":       fmt.Sprintf("peak %.1f MiB", float64(q.Server.Total.MemoryBytes.Peak)/(1<<20)),
		"Engine CPU / memory": fmt.Sprintf("avg %.2f threads, peak %.2f threads · peak %.1f MiB · %d hardware threads",
			q.LoadMachine.Engine.CPUThreads.Average, q.LoadMachine.Engine.CPUThreads.Peak, float64(q.LoadMachine.Engine.MemoryBytes.Peak)/(1<<20), q.LoadMachine.HardwareThreads),
	}
	for label, want := range checks {
		if got := row(t, md, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	for _, c := range q.Server.Containers {
		if !strings.Contains(md, "| "+c.Name+" | "+c.Status+" |") {
			t.Errorf("no server row for %s", c.Name)
		}
	}
	if !strings.Contains(md, q.Worst.LowestMOS[0].PeerID) {
		t.Error("worst bot's peer ID missing")
	}
	if !strings.Contains(md, "### Most freeze seconds\n\nNone.") {
		t.Errorf("empty ranking not shown as None.:\n%s", md)
	}
}

func TestMarkdownNotMeasured(t *testing.T) {
	_, md := markdownRun(t, loadSummary(t, "receive_only.json"), "completed", "")
	for label, want := range map[string]string{
		"Audio MOS":                   "not measured: no bot received audio",
		"Video freezes":               "not measured: no bot received video",
		"Video frame rate":            "not measured: no bot received video",
		"Latency p50 (audio / video)": "not measured: no bot received audio / not measured: no bot received video",
		"Server CPU (total)":          "not measured: not_requested",
	} {
		if got := row(t, md, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
	if !strings.Contains(md, "Server load not requested.") {
		t.Error("server section does not say it was not requested")
	}
	if strings.Contains(md, "| 0.00") || strings.Contains(md, "median 0.00") {
		t.Errorf("zeros shown for measures that do not apply:\n%s", md)
	}
}

func TestMarkdownFailedRun(t *testing.T) {
	_, md := markdownRun(t, nil, "failed", "open clip.mp4: no such file")
	if !strings.Contains(md, "\n> Error: open clip.mp4: no such file\n") {
		t.Errorf("no error line:\n%s", md)
	}
	if got := row(t, md, "Audio MOS"); got != "not measured: the run failed before measuring" {
		t.Errorf("Audio MOS = %q", got)
	}
	if strings.Contains(md, "| 0") {
		t.Errorf("zeros in a failed run's report:\n%s", md)
	}
}

func TestMarkdownTestPlanRedacted(t *testing.T) {
	_, md := markdownRun(t, loadSummary(t, "full.json"), "completed", "")
	if !strings.Contains(md, `"jwt_app_secret": "[redacted]"`) {
		t.Errorf("test plan block:\n%s", md)
	}
}

func TestMarkdownTimes(t *testing.T) {
	run, md := markdownRun(t, loadSummary(t, "full.json"), "completed", "")
	want := fmt.Sprintf("jitsi · run %s · %s → %s (UTC) · completed", run.RunID,
		run.StartedAt.Format(time.DateTime), run.EndedAt.Format(time.DateTime))
	if !strings.Contains(md, "\n"+want+"\n") {
		t.Errorf("subtitle missing %q:\n%s", want, md[:200])
	}
}
