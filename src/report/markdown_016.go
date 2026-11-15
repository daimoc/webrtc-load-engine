package report

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"webrtc-load-engine/src/scenario"
)

// Report additions of 016 (specs/016-network-conditions/contracts/report-network.md):
// the network conditions applied, as measured, and each group's quality.

func writeNetwork(b *strings.Builder, n *scenario.NetworkResult) {
	b.WriteString("\n## Network conditions\n\n")
	source := "test plan"
	if n.SeedSource == scenario.SeedDrawn {
		source = "drawn by the engine"
	}
	fmt.Fprintf(b, "Seed %d (%s), settling time %d s, timeline resolution %d s. Measures are the links' own counts, not the plan's values.\n\n",
		n.Seed, source, n.SettleSeconds, n.TimelineResolutionSeconds)
	b.WriteString("| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |\n|---|---|---|---|---|---|---|---|---|\n")
	for _, g := range n.Groups {
		for _, st := range g.Steps {
			for _, dir := range []string{"down", "up"} {
				c := st.Planned.Downlink
				if dir == "up" {
					c = st.Planned.Uplink
				}
				if c == nil {
					continue
				}
				fmt.Fprintf(b, "| %s | %d | %d | %s: %s | %s | %s |\n", g.Name, len(g.Bots), st.Index, dir, conditionText(c), stepTimes(st),
					groupMeasure(n.Bots, g.Name, st.Index, dir))
			}
		}
	}
	writeReactions(b, n)
	writeSenders(b, n)
	writeServerEstimates(b, n)
	b.WriteString("\n| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |\n|---|---|---|---|---|\n")
	for _, g := range n.Groups {
		writeGroupQuality(b, fmt.Sprintf("%s (%s)", g.Name, plural(len(g.Bots), "bot")), g.Quality)
	}
	if u := n.UnconstrainedQuality; u != nil && u.Bots > 0 {
		writeGroupQuality(b, fmt.Sprintf("no condition (%s)", plural(u.Bots, "bot")), u)
	}
}

// conditionText describes a planned condition.
func conditionText(c *scenario.LinkCondition) string {
	var parts []string
	if c.RateKbps > 0 {
		parts = append(parts, fmt.Sprintf("%d kbit/s, queue %d ms", c.RateKbps, c.QueueMs))
	}
	if c.DelayMs > 0 {
		d := fmt.Sprintf("delay %d ms", c.DelayMs)
		if c.JitterMs > 0 {
			d = fmt.Sprintf("delay %d ± %d ms", c.DelayMs, c.JitterMs)
		}
		parts = append(parts, d)
	}
	if c.Loss != nil {
		l := "loss " + pct(c.Loss.Share)
		if c.Loss.MeanBurst > 1 {
			l += fmt.Sprintf(" in bursts of %g", c.Loss.MeanBurst)
		}
		parts = append(parts, l)
	}
	return strings.Join(parts, ", ")
}

func stepTimes(st scenario.AppliedStep) string {
	if st.StartedAtSeconds == nil {
		return "not started"
	}
	end := "end"
	if st.EndedAtSeconds != nil {
		end = fmt.Sprintf("%.1f", *st.EndedAtSeconds)
	}
	s := fmt.Sprintf("%.1f–%s s", *st.StartedAtSeconds, end)
	if st.DuringStartUp {
		s += " (during start-up)"
	}
	return s
}

// groupMeasure is the measured figures of one step and direction over the
// group's bots: mean of their delivered rates, highest 10-second window,
// shares over all their datagrams, mean and highest added delay.
func groupMeasure(bots []scenario.BotNetwork, group string, step int, dir string) string {
	var n int
	var mean, max10, delay, delayMax float64
	var offered, capDrops, lossDrops float64
	var burstSum float64
	var burstN int
	for _, bot := range bots {
		if bot.Group != group || step >= len(bot.Steps) {
			continue
		}
		m := bot.Steps[step].Downlink
		if dir == "up" {
			m = bot.Steps[step].Uplink
		}
		if m == nil || m.Reason != "" {
			continue
		}
		n++
		mean += m.DeliveredKbps.Mean
		max10 = max(max10, m.DeliveredKbps.Max10s)
		delay += m.AddedDelayMs.Mean
		delayMax = max(delayMax, m.AddedDelayMs.Max)
		offered += float64(m.OfferedPackets)
		if m.DroppedByCapShare != nil {
			capDrops += *m.DroppedByCapShare * float64(m.OfferedPackets)
		}
		if m.DroppedByLossShare != nil {
			lossDrops += *m.DroppedByLossShare * float64(m.OfferedPackets)
		}
		if m.MeanBurst != nil {
			burstSum += *m.MeanBurst
			burstN++
		}
	}
	if n == 0 || offered == 0 {
		return "not applied | | |"
	}
	loss := pct(lossDrops / offered)
	if burstN > 0 {
		loss += fmt.Sprintf(", bursts of %.1f", burstSum/float64(burstN))
	}
	return fmt.Sprintf("%.0f kbit/s, %.0f at most over 10 s | %s | %s | %.0f ms, %.0f at most",
		mean/float64(n), max10, pct(capDrops/offered), loss, delay/float64(n), delayMax)
}

func writeGroupQuality(b *strings.Builder, label string, q *scenario.QualitySummary) {
	if q == nil {
		fmt.Fprintf(b, "| %s | not measured | | | |\n", label)
		return
	}
	heights, requested, mos := "none", "", ""
	if len(q.Video.HeightShare) > 0 {
		heights = heightShares(q.Video.HeightShare)
	}
	if q.Video.RequestedHeightShare != nil {
		requested = pct(q.Video.RequestedHeightShare.P50)
	}
	if q.Audio.MOS != nil {
		mos = fmt.Sprintf("%.2f", q.Audio.MOS.P50)
	}
	fmt.Fprintf(b, "| %s | %s | %s | %d | %s |\n", label, heights, requested, q.Video.FreezesTotal, mos)
}

// writeReactions gives, for each step that conditions the download, how
// fast the server lowered the group's bots and gave their height back.
func writeReactions(b *strings.Builder, n *scenario.NetworkResult) {
	var rows, details []string
	for _, g := range n.Groups {
		for _, st := range g.Steps {
			if st.Planned.Downlink == nil || st.StartedAtSeconds == nil {
				continue
			}
			var reactions, recoveries, missing []float64
			var bots, recovered, ended int
			for _, bot := range n.Bots {
				if bot.Group != g.Name || st.Index >= len(bot.Steps) {
					continue
				}
				bs := bot.Steps[st.Index]
				bots++
				if bs.MissingAfterRetransmissionShare != nil {
					missing = append(missing, *bs.MissingAfterRetransmissionShare)
				}
				line := fmt.Sprintf("- %s, step %d: ", bot.Bot, st.Index)
				if bs.ReactionSeconds != nil {
					reactions = append(reactions, *bs.ReactionSeconds)
					line += fmt.Sprintf("reaction %.1f s", *bs.ReactionSeconds)
				} else {
					line += "no reaction"
				}
				if bs.Recovered != nil {
					ended++
					if *bs.Recovered && bs.RecoverySeconds != nil {
						recovered++
						recoveries = append(recoveries, *bs.RecoverySeconds)
						line += fmt.Sprintf(", recovery %.1f s", *bs.RecoverySeconds)
					} else {
						line += ", not recovered"
					}
				}
				if bs.Reason != "" {
					line += " (" + bs.Reason + ")"
				}
				details = append(details, line)
			}
			recovery := "step open to the end"
			if ended > 0 {
				recovery = fmt.Sprintf("%d of %d | %s", recovered, bots, medianMax(recoveries))
			} else {
				recovery += " |"
			}
			still := ""
			if len(missing) > 0 {
				sum := 0.0
				for _, v := range missing {
					sum += v
				}
				still = pct(sum / float64(len(missing)))
			}
			rows = append(rows, fmt.Sprintf("| %s | %d | %d of %d | %s | %s | %s |", g.Name, st.Index, len(reactions), bots, medianMax(reactions), recovery, still))
		}
	}
	if len(rows) == 0 {
		return
	}
	fmt.Fprintf(b, "\nReaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution %d s.\n\n", n.TimelineResolutionSeconds)
	b.WriteString("| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |\n|---|---|---|---|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
	sort.Strings(details)
	b.WriteString("\n<details><summary>Per bot</summary>\n\n")
	for _, d := range details {
		b.WriteString(d + "\n")
	}
	b.WriteString("\n</details>\n")
}

// medianMax is "median, longest" of values in seconds, or empty.
func medianMax(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	v := append([]float64(nil), values...)
	sort.Float64s(v)
	return fmt.Sprintf("%.1f s, %.1f s", v[(len(v)-1)/2], v[len(v)-1])
}

// maxLayerChanges is how many layer changes per sender the Markdown lists;
// the JSON has them all.
const maxLayerChanges = 12

// writeSenders lists, for bots whose upload has a condition, the layers
// they sent over time.
func writeSenders(b *strings.Builder, n *scenario.NetworkResult) {
	var lines []string
	for _, bot := range n.Bots {
		s := bot.Sender
		if s == nil || s.Reason == scenario.NoUplinkCondition {
			continue
		}
		if !s.Adapts {
			lines = append(lines, fmt.Sprintf("- %s: does not adapt (%s)", bot.Bot, s.Reason))
			continue
		}
		var parts []string
		for i, c := range s.Layers {
			if i == maxLayerChanges {
				parts = append(parts, fmt.Sprintf("… %d more", len(s.Layers)-i))
				break
			}
			parts = append(parts, fmt.Sprintf("%s from %.1f s", joinInts(c.Heights), c.AtSeconds))
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", bot.Bot, strings.Join(parts, "; ")))
	}
	if len(lines) == 0 {
		return
	}
	b.WriteString("\nSenders under an upload condition, layers sent over time:\n\n")
	for _, l := range lines {
		b.WriteString(l + "\n")
	}
}

func joinInts(v []int) string {
	s := make([]string, len(v))
	for i, x := range v {
		s[i] = fmt.Sprint(x)
	}
	return strings.Join(s, "+")
}

// writeServerEstimates gives, where the server's signaling gives it, the
// server's estimate toward each group's bots during each step: the median
// of their samples in the step.
func writeServerEstimates(b *strings.Builder, n *scenario.NetworkResult) {
	var rows []string
	for _, g := range n.Groups {
		for _, st := range g.Steps {
			if st.StartedAtSeconds == nil {
				continue
			}
			end := math.Inf(1)
			if st.EndedAtSeconds != nil {
				end = *st.EndedAtSeconds
			}
			var in []float64
			given := false
			for _, bot := range n.Bots {
				if bot.Group != g.Name || bot.ServerEstimate == nil || bot.ServerEstimate.Reason == scenario.NotGivenByServer {
					continue
				}
				given = true
				for _, e := range bot.ServerEstimate.Kbps {
					if e.AtSeconds >= *st.StartedAtSeconds && e.AtSeconds <= end {
						in = append(in, float64(e.Kbps))
					}
				}
			}
			if !given {
				continue
			}
			v := "no sample"
			if len(in) > 0 {
				sort.Float64s(in)
				v = fmt.Sprintf("%.0f kbit/s (%d samples, %.0f to %.0f)", in[(len(in)-1)/2], len(in), in[0], in[len(in)-1])
			}
			rows = append(rows, fmt.Sprintf("| %s | %d | %s |", g.Name, st.Index, v))
		}
	}
	if len(rows) == 0 {
		b.WriteString("\nServer's estimate toward the bots: not given by this server's signaling.\n")
		return
	}
	b.WriteString("\n| Group | Step | Server's estimate toward the group's bots (median) |\n|---|---|---|\n")
	for _, r := range rows {
		b.WriteString(r + "\n")
	}
}
