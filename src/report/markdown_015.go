package report

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"webrtc-load-engine/src/scenario"
)

// Markdown parts of 015 (specs/015-bandwidth-estimate-parity/contracts/report.md):
// server settings, reception reports and the browser.

// maxFlaggedRows is the number of flagged bots the Markdown lists; the JSON
// file has them all.
const maxFlaggedRows = 10

// serverSettingsRow words what the run knows of the server's settings. It
// never says "defaults": a run cannot know them (US4 scenario 3).
func serverSettingsRow(s *scenario.ServerSettingsConfig) string {
	switch {
	case s == nil:
		return "not known to the run"
	case len(s.Values) == 0:
		return "none changed by the deployment"
	}
	var pairs []string
	for _, k := range slices.Sorted(maps.Keys(s.Values)) {
		pairs = append(pairs, k+"="+s.Values[k])
	}
	return fmt.Sprintf("%s (stated by the %s)", strings.Join(pairs, ", "), s.Source)
}

func receptionRow(r scenario.ReceptionReports) string {
	if !r.Available {
		return notMeasured(r.Reason)
	}
	median, lowest := 0.0, 1.0
	if r.AcknowledgedShare != nil {
		median = r.AcknowledgedShare.P50
	}
	for _, b := range r.PerBot {
		lowest = min(lowest, b.AcknowledgedShare)
	}
	return fmt.Sprintf("acknowledged %s (lowest bot %s) · %s flagged", pct(median), pct(lowest), plural(len(r.Flagged), "bot"))
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// writeReceptionWarning warns, in the Video section, that layered results
// can reflect the bots (FR-006).
func writeReceptionWarning(b *strings.Builder, r scenario.ReceptionReports) {
	if !r.Available || len(r.Flagged) == 0 {
		return
	}
	their := "its"
	if len(r.Flagged) > 1 {
		their = "their"
	}
	fmt.Fprintf(b, "> Warning: %s reported %s reception incompletely or late (see Reception\n> reports). Layered results of this run can reflect the bots.\n\n",
		plural(len(r.Flagged), "bot"), their)
}

func writeReception(b *strings.Builder, r scenario.ReceptionReports) {
	b.WriteString("\n## Reception reports\n\n")
	if !r.Available {
		fmt.Fprintf(b, "Not measured: %s.\n", r.Reason)
		return
	}
	fmt.Fprintf(b, "How completely and how regularly the bots reported their reception to the server (%s measured).\n\n", plural(r.Bots, "bot"))
	writePercentiles(b, []pctRow{
		{"Acknowledged share", r.AcknowledgedShare, pct},
		{"Late feedback share", r.LateShare, pct},
		{"Longest stamp lag", r.StampLagMaxMs, func(v float64) string { return fmt.Sprintf("%.1f ms", v) }},
	})
	l := r.Limits
	fmt.Fprintf(b, "\nA bot is flagged when it acknowledged under %g of its arriving packets, or when over %s of its feedback intervals were longer than %d ms.\n",
		l.AcknowledgedShareMin, pct(l.LateShareMax), l.LateIntervalMs)
	if len(r.Flagged) == 0 {
		b.WriteString("\nNo bot flagged.\n")
		return
	}
	byID := map[string]scenario.BotReception{}
	for _, p := range r.PerBot {
		byID[p.PeerID] = p
	}
	b.WriteString("\n| Bot | Acknowledged | Late feedback | Longest interval | Longest stamp lag |\n|---|---|---|---|---|\n")
	for i, id := range r.Flagged {
		if i == maxFlaggedRows {
			fmt.Fprintf(b, "\nand %d more (all in the JSON report).\n", len(r.Flagged)-maxFlaggedRows)
			break
		}
		p := byID[id]
		fmt.Fprintf(b, "| %s | %s | %s | %.1f ms | %.1f ms |\n", id, pct(p.AcknowledgedShare), pct(p.LateShare), p.IntervalMs.Max, p.StampLagMs.Max)
	}
}

func browserRow(r *scenario.BrowserResult) string {
	if r.Status == scenario.BrowserUnavailable {
		return notMeasured(r.Reason)
	}
	var parts []string
	if r.ExpectedHeightShare != nil {
		parts = append(parts, fmt.Sprintf("%d lines %s of the time", r.ExpectedHeight, pct(*r.ExpectedHeightShare)))
		if r.SecondsToExpectedHeight != nil {
			parts = append(parts, fmt.Sprintf("reached after %.1f s", *r.SecondsToExpectedHeight))
		} else {
			parts = append(parts, "not reached")
		}
	} else {
		parts = append(parts, heightShares(r.HeightShare), "no expected height stated")
	}
	parts = append(parts, plural(int(r.FreezesTotal), "freeze"))
	row := strings.Join(parts, " · ")
	if r.Status == scenario.BrowserPartial {
		row = fmt.Sprintf("partial (%s): %s", r.Reason, row)
	}
	return row
}

func writeBrowserLoad(b *strings.Builder, r *scenario.BrowserResult) {
	if r == nil || r.CPUThreadsAverage == nil {
		return
	}
	fmt.Fprintf(b, "\nA browser ran on this machine during the run: %.2f threads on average, not counted in the engine's figure.\n", *r.CPUThreadsAverage)
}

func writeBrowser(b *strings.Builder, r *scenario.BrowserResult) {
	b.WriteString("\n## Browser\n\n")
	fmt.Fprintf(b, "Address %s, window %s.\n\n", r.URL, r.Window)
	if r.Status == scenario.BrowserUnavailable {
		fmt.Fprintf(b, "Not measured: %s.\n", r.Reason)
		return
	}
	if r.Status == scenario.BrowserPartial {
		fmt.Fprintf(b, "Partial: %s; the measures stop there.\n\n", r.Reason)
	}
	if r.BrowserVersion != "" {
		fmt.Fprintf(b, "- Browser: %s\n", r.BrowserVersion)
	}
	fmt.Fprintf(b, "- Video streams decoding at once: at most %d\n", r.VideoStreams)
	switch {
	case r.ExpectedHeightShare != nil && r.SecondsToExpectedHeight != nil:
		fmt.Fprintf(b, "- At %d lines: %s of the time, first after %.1f s\n", r.ExpectedHeight, pct(*r.ExpectedHeightShare), *r.SecondsToExpectedHeight)
	case r.ExpectedHeightShare != nil:
		fmt.Fprintf(b, "- At %d lines: %s of the time, never reached\n", r.ExpectedHeight, pct(*r.ExpectedHeightShare))
	default:
		b.WriteString("- Expected height: not stated\n")
	}
	fmt.Fprintf(b, "- Received video: %.0f kbit/s on average, peak %.0f kbit/s over one second\n", r.ReceivedBitrateKbps.Average, r.ReceivedBitrateKbps.Peak)
	fmt.Fprintf(b, "- Freezes: %d, %.3f s (as counted by the browser)\n", r.FreezesTotal, r.FreezeSeconds)
	if r.CPUThreadsAverage != nil {
		fmt.Fprintf(b, "- CPU: %s on average\n", threads(*r.CPUThreadsAverage))
	}
	if len(r.HeightShare) > 0 {
		b.WriteString("\n| Height | Share of decoded stream time |\n|---|---|\n")
		for _, h := range sortedHeights(r.HeightShare) {
			fmt.Fprintf(b, "| %s | %s |\n", h, pct(r.HeightShare[h]))
		}
	}
}
