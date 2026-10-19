package report

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"webrtc-load-engine/src/scenario"
)

// notMeasuredFailed is the reason given for every measure of a run that
// failed before the media collector started.
const notMeasuredFailed = "the run failed before measuring"

// Markdown renders run as the readable report
// (contracts/report-markdown.md): key figures first, then the details. It
// reads the same values as the JSON report, rounded for display only.
func Markdown(run Run) []byte {
	var b strings.Builder
	q := run.Quality
	fmt.Fprintf(&b, "# Load test report: %s\n\n", run.Scenario.Name)
	fmt.Fprintf(&b, "%s · run %s · %s → %s (UTC) · %s\n", run.Scenario.Platform, run.RunID,
		run.StartedAt.UTC().Format(time.DateTime), run.EndedAt.UTC().Format(time.DateTime), run.Outcome.Status)
	if run.Outcome.Error != "" {
		fmt.Fprintf(&b, "\n> Error: %s\n", run.Outcome.Error)
	}

	writeKeyFigures(&b, run)
	if q == nil {
		for _, h := range []string{"Audio", "Video", "Server", "Load machine", "Worst bots"} {
			fmt.Fprintf(&b, "\n## %s\n\nNot measured: %s.\n", h, notMeasuredFailed)
		}
	} else {
		writeAudio(&b, q.Audio)
		writeVideo(&b, q.Video)
		writeServer(&b, q.Server)
		writeLoadMachine(&b, q.LoadMachine)
		writeWorst(&b, q.Worst)
	}

	plan, _ := json.MarshalIndent(run.TestPlan, "", "  ")
	fmt.Fprintf(&b, "\n## Test plan\n\n```json\n%s\n```\n", plan)
	fmt.Fprintf(&b, "\nEngine %s (%s)\n", run.Engine.Version, run.Engine.GoVersion)
	return []byte(b.String())
}

func writeKeyFigures(b *strings.Builder, run Run) {
	tp, q := run.TestPlan, run.Quality
	rows := [][2]string{
		{"Platform", run.Scenario.Platform},
		{"Participants", strconv.Itoa(tp.Participants)},
		{"Test duration", fmt.Sprintf("%d s (ramp-up %d s); run %.3f s", tp.DurationSeconds, tp.RampUpSeconds, run.DurationSeconds)},
		{"Outcome", run.Outcome.Status},
	}
	if q == nil {
		for _, label := range []string{"Audio MOS", "Audio concealed", "Video freezes", "Video frame rate", "Video resolution",
			"Latency p50 (audio / video)", "Server CPU (total)", "Server memory (total)", "Engine CPU / memory"} {
			rows = append(rows, [2]string{label, notMeasured(notMeasuredFailed)})
		}
	} else {
		rows[1][1] = fmt.Sprintf("%d (bots measured: %d)", tp.Participants, q.Bots)
		a, v := q.Audio, q.Video
		mos, concealed := notMeasured(a.Reason), notMeasured(a.Reason)
		if a.Available {
			mos, concealed = "not measured", "not measured"
			if a.MOS != nil {
				mos = fmt.Sprintf("median %.2f", a.MOS.P50)
				if len(q.Worst.LowestMOS) > 0 {
					mos += fmt.Sprintf(", worst bot %.2f", q.Worst.LowestMOS[0].Value)
				}
			}
			if a.ConcealedRatio != nil {
				concealed = "p95 " + pct(a.ConcealedRatio.P95)
			}
		}
		freezes, frameRate, resolution := notMeasured(v.Reason), notMeasured(v.Reason), notMeasured(v.Reason)
		if v.Available {
			freezes = fmt.Sprintf("%d in total", v.FreezesTotal)
			if v.FreezeSeconds != nil {
				freezes += fmt.Sprintf("; freeze time p95 %.3f s", v.FreezeSeconds.P95)
			}
			frameRate = "not measured"
			if v.FrameRate != nil {
				frameRate = fmt.Sprintf("%.1f / %.1f / %.1f fps (p50 / p95 / p99)", v.FrameRate.P50, v.FrameRate.P95, v.FrameRate.P99)
			}
			resolution = heightShares(v.HeightShare)
		}
		rows = append(rows,
			[2]string{"Audio MOS", mos},
			[2]string{"Audio concealed", concealed},
			[2]string{"Video freezes", freezes},
			[2]string{"Video frame rate", frameRate},
			[2]string{"Video resolution", resolution},
			[2]string{"Latency p50 (audio / video)", latencyP50(a.KindQuality) + " / " + latencyP50(v.KindQuality)},
		)
		cpu, mem := notMeasured(q.Server.Reason), notMeasured(q.Server.Reason)
		if t := q.Server.Total; q.Server.Available && t != nil {
			cpu = fmt.Sprintf("avg %s, peak %s", threads(t.CPUThreads.Average), threads(t.CPUThreads.Peak))
			mem = "peak " + mib(t.MemoryBytes.Peak)
		}
		lm := q.LoadMachine
		rows = append(rows,
			[2]string{"Server CPU (total)", cpu},
			[2]string{"Server memory (total)", mem},
			[2]string{"Engine CPU / memory", fmt.Sprintf("avg %s, peak %s · peak %s · %d hardware threads",
				threads(lm.Engine.CPUThreads.Average), threads(lm.Engine.CPUThreads.Peak), mib(lm.Engine.MemoryBytes.Peak), lm.HardwareThreads)},
		)
	}
	b.WriteString("\n## Key figures\n\n| | |\n|---|---|\n")
	for _, r := range rows {
		fmt.Fprintf(b, "| %s | %s |\n", r[0], r[1])
	}
}

func writeAudio(b *strings.Builder, a scenario.AudioQuality) {
	b.WriteString("\n## Audio\n\n")
	if !a.Available {
		fmt.Fprintf(b, "Not measured: %s.\n", a.Reason)
		return
	}
	fmt.Fprintf(b, "Bots receiving audio: %d.\n\n", a.Bots)
	writePercentiles(b, []pctRow{
		{"MOS", a.MOS, func(v float64) string { return fmt.Sprintf("%.2f", v) }},
		{"Concealed", a.ConcealedRatio, pct},
		{"Latency", a.Latency, ms},
	})
	writeLatencyReason(b, a.KindQuality)
}

func writeVideo(b *strings.Builder, v scenario.VideoQuality) {
	b.WriteString("\n## Video\n\n")
	if !v.Available {
		fmt.Fprintf(b, "Not measured: %s.\n", v.Reason)
		return
	}
	fmt.Fprintf(b, "Bots receiving video: %d.\n\n", v.Bots)
	writePercentiles(b, []pctRow{
		{"Frame rate", v.FrameRate, func(f float64) string { return fmt.Sprintf("%.1f fps", f) }},
		{"Freeze time", v.FreezeSeconds, func(f float64) string { return fmt.Sprintf("%.3f s", f) }},
		{"Requested-height share", v.RequestedHeightShare, pct},
		{"Latency", v.Latency, ms},
	})
	writeLatencyReason(b, v.KindQuality)
	fmt.Fprintf(b, "\n- Freezes: %d\n- Resolution switches: %d\n- Keyframe requests sent: %d, received: %d\n",
		v.FreezesTotal, v.ResolutionSwitchesTotal, v.KeyframeRequestsSentTotal, v.KeyframeRequestsReceivedTotal)
	if len(v.HeightShare) > 0 {
		b.WriteString("\n| Height | Share of video time |\n|---|---|\n")
		for _, h := range sortedHeights(v.HeightShare) {
			fmt.Fprintf(b, "| %s | %s |\n", h, pct(v.HeightShare[h]))
		}
	}
}

// pctRow is one measure of a percentile table.
type pctRow struct {
	label  string
	p      *scenario.Percentiles
	format func(float64) string
}

// writePercentiles writes a p50/p95/p99 table; rows without percentiles
// are left out.
func writePercentiles(b *strings.Builder, rows []pctRow) {
	b.WriteString("| | p50 | p95 | p99 |\n|---|---|---|---|\n")
	for _, r := range rows {
		if r.p == nil {
			continue
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", r.label, r.format(r.p.P50), r.format(r.p.P95), r.format(r.p.P99))
	}
}

func writeLatencyReason(b *strings.Builder, k scenario.KindQuality) {
	if !k.LatencyAvailable {
		fmt.Fprintf(b, "\nLatency not measured: %s.\n", latencyReason(k))
	}
}

func writeServer(b *strings.Builder, s scenario.ServerLoad) {
	b.WriteString("\n## Server\n\n")
	if s.Reason == scenario.ReasonNotRequested {
		b.WriteString("Server load not requested.\n")
		return
	}
	if !s.Available {
		fmt.Fprintf(b, "Not measured: %s.\n\n", s.Reason)
	}
	if s.Period != nil {
		fmt.Fprintf(b, "Sampled every %d s from %s to %s (UTC).\n\n", s.IntervalSeconds,
			s.Period.Start.UTC().Format(time.DateTime), s.Period.End.UTC().Format(time.DateTime))
	}
	if len(s.Containers) == 0 {
		return
	}
	b.WriteString("| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |\n|---|---|---|---|---|---|---|---|\n")
	for _, c := range s.Containers {
		status := c.Status
		if c.Reason != "" {
			status += " (" + c.Reason + ")"
		}
		if c.Status == scenario.LoadStatusUnavailable {
			fmt.Fprintf(b, "| %s | %s | %d / %d | – | – | – | – | – |\n", c.Name, status, c.Samples, c.Missed)
			continue
		}
		fmt.Fprintf(b, "| %s | %s | %s |\n", c.Name, status, loadCells(c.LoadFigures))
	}
	if t := s.Total; t != nil {
		fmt.Fprintf(b, "| **total** | %d measured | %s |\n", t.ContainersMeasured, loadCells(t.LoadFigures))
	}
	for _, c := range s.Containers {
		if len(c.SharedWith) > 0 {
			fmt.Fprintf(b, "\n%s was also sampled by %s during this run: its load is not this run's alone.\n",
				c.Name, strings.Join(c.SharedWith, ", "))
		}
	}
}

// loadCells are the samples, CPU and memory cells of a server table row.
func loadCells(f scenario.LoadFigures) string {
	return fmt.Sprintf("%d / %d | %s | %s | %s | %s | %s", f.Samples, f.Missed,
		threads(f.CPUThreads.First), threads(f.CPUThreads.Average), threads(f.CPUThreads.Peak),
		mib(f.MemoryBytes.First), mib(f.MemoryBytes.Peak))
}

func writeLoadMachine(b *strings.Builder, lm scenario.LoadMachine) {
	b.WriteString("\n## Load machine\n\n")
	e := lm.Engine
	if e.Samples == 0 {
		fmt.Fprintf(b, "Not measured: %s.\n", scenario.ReasonNoSamples)
		return
	}
	b.WriteString("| | Avg CPU | Peak CPU | Peak memory | Samples / missed |\n|---|---|---|---|---|\n")
	fmt.Fprintf(b, "| Engine | %s | %s | %s | %d / %d |\n", threads(e.CPUThreads.Average), threads(e.CPUThreads.Peak), mib(e.MemoryBytes.Peak), e.Samples, e.Missed)
	fmt.Fprintf(b, "\nHardware threads: %d (usable by the engine: %d).\n", lm.HardwareThreads, lm.UsableThreads)
	if lm.Period != nil {
		fmt.Fprintf(b, "Period: %s to %s (UTC).\n", lm.Period.Start.UTC().Format(time.DateTime), lm.Period.End.UTC().Format(time.DateTime))
	}
}

func writeWorst(b *strings.Builder, w scenario.WorstBots) {
	b.WriteString("\n## Worst bots\n")
	for _, r := range []struct {
		title  string
		bots   []scenario.BotValue
		format func(float64) string
	}{
		{"Lowest MOS", w.LowestMOS, func(v float64) string { return fmt.Sprintf("%.2f", v) }},
		{"Most freeze seconds", w.MostFreezeSeconds, func(v float64) string { return fmt.Sprintf("%.3f s", v) }},
		{"Highest latency p95", w.HighestLatencyP95Seconds, ms},
		{"Lowest requested-height share", w.LowestRequestedHeight, pct},
	} {
		fmt.Fprintf(b, "\n### %s\n\n", r.title)
		if len(r.bots) == 0 {
			b.WriteString("None.\n")
			continue
		}
		b.WriteString("| Peer | Value |\n|---|---|\n")
		for _, bot := range r.bots {
			fmt.Fprintf(b, "| %s | %s |\n", bot.PeerID, r.format(bot.Value))
		}
	}
}

// latencyP50 is a kind's median latency, or why it is missing.
func latencyP50(k scenario.KindQuality) string {
	if !k.Available {
		return notMeasured(k.Reason)
	}
	if !k.LatencyAvailable || k.Latency == nil {
		return notMeasured(latencyReason(k))
	}
	return ms(k.Latency.P50)
}

func latencyReason(k scenario.KindQuality) string {
	if k.LatencyReason != "" {
		return k.LatencyReason
	}
	return scenario.LatencyNotMeasured
}

// heightShares lists the share of video time per height, highest first.
func heightShares(shares map[string]float64) string {
	if len(shares) == 0 {
		return "not measured"
	}
	var parts []string
	for _, h := range sortedHeights(shares) {
		parts = append(parts, h+": "+pct(shares[h]))
	}
	return strings.Join(parts, ", ")
}

// sortedHeights are the heights of shares, highest first.
func sortedHeights(shares map[string]float64) []string {
	heights := make([]string, 0, len(shares))
	for h := range shares {
		heights = append(heights, h)
	}
	slices.SortFunc(heights, func(a, b string) int {
		x, _ := strconv.Atoi(a)
		y, _ := strconv.Atoi(b)
		return y - x
	})
	return heights
}

func notMeasured(reason string) string { return "not measured: " + reason }
func threads(v float64) string         { return fmt.Sprintf("%.2f threads", v) }
func mib(v int64) string               { return fmt.Sprintf("%.1f MiB", float64(v)/(1<<20)) }
func pct(v float64) string             { return fmt.Sprintf("%.2f %%", v*100) }
func ms(v float64) string              { return fmt.Sprintf("%.1f ms", v*1000) }
