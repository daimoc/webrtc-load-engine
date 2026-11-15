package report

import (
	"strings"
	"testing"

	"webrtc-load-engine/src/scenario"
)

// Report additions of 016 (specs/016-network-conditions/contracts/report-network.md).

func networkResult() *scenario.NetworkResult {
	f := func(v float64) *float64 { return &v }
	dur := 20
	q := &scenario.QualitySummary{Bots: 3}
	q.Video.Available, q.Video.Bots, q.Video.FreezesTotal = true, 3, 4
	q.Video.HeightShare = map[string]float64{"360": 0.8, "720": 0.2}
	q.Video.RequestedHeightShare = &scenario.Percentiles{P50: 0.2}
	q.Audio.Available, q.Audio.MOS = true, &scenario.Percentiles{P50: 4.1}
	u := &scenario.QualitySummary{Bots: 7}
	u.Video.Available, u.Video.Bots = true, 7
	u.Video.HeightShare = map[string]float64{"720": 1}
	u.Video.RequestedHeightShare = &scenario.Percentiles{P50: 1}
	measure := func(kbps float64) *scenario.DirectionMeasure {
		return &scenario.DirectionMeasure{OfferedPackets: 100, DeliveredPackets: 90, DeliveredKbps: scenario.KbpsMean{Mean: kbps, Max10s: kbps + 5},
			DroppedByCapShare: f(0.1), DroppedByLossShare: f(0), AddedDelayMs: scenario.MeanMax{Mean: 150, Max: 200}}
	}
	return &scenario.NetworkResult{Seed: 42, SeedSource: scenario.SeedFromTestPlan, SettleSeconds: 10, TimelineResolutionSeconds: 2,
		Groups: []scenario.GroupResult{{Name: "slow", Bots: []string{"b8", "b9", "b10"}, Quality: q, Steps: []scenario.AppliedStep{
			{Index: 0, Planned: scenario.ScheduleStep{StartSeconds: 20, DurationSeconds: &dur, Downlink: &scenario.LinkCondition{RateKbps: 500, QueueMs: 200}},
				StartedAtSeconds: f(20), EndedAtSeconds: f(40), DuringStartUp: true},
			{Index: 1, Planned: scenario.ScheduleStep{StartSeconds: 50, Uplink: &scenario.LinkCondition{DelayMs: 40, JitterMs: 10, Loss: &scenario.Loss{Share: 0.03, MeanBurst: 2}}}},
		}}},
		UnconstrainedQuality: u,
		Bots: []scenario.BotNetwork{
			{Bot: "b8", Group: "slow", Steps: []scenario.BotStep{{Index: 0, Downlink: measure(490)}, {Index: 1, Uplink: &scenario.DirectionMeasure{Reason: scenario.NotApplied}}}},
			{Bot: "b9", Group: "slow", Steps: []scenario.BotStep{{Index: 0, Downlink: measure(500)}, {Index: 1, Uplink: &scenario.DirectionMeasure{Reason: scenario.NotApplied}}}},
			{Bot: "b10", Group: "slow", Steps: []scenario.BotStep{{Index: 0, Downlink: &scenario.DirectionMeasure{Reason: scenario.NotApplied}}, {Index: 1, Uplink: &scenario.DirectionMeasure{Reason: scenario.NotApplied}}}},
		}}
}

func markdown016(t *testing.T, n *scenario.NetworkResult) string {
	t.Helper()
	r := testResult()
	r.Quality = loadSummary(t, "full.json")
	r.Network = n
	return string(Markdown(NewRun(testScenario(), r)))
}

func TestMarkdownNetwork(t *testing.T) {
	md := markdown016(t, networkResult())
	if !strings.Contains(md, "\n## Network conditions\n") {
		t.Fatalf("no network section:\n%s", md)
	}
	for _, want := range []string{
		"Seed 42 (test plan)",
		"settling time 10 s",
		"| slow | 3 | 0 | down: 500 kbit/s, queue 200 ms | 20.0–40.0 s (during start-up) | 495 kbit/s, 505 at most over 10 s | 10.00 % | 0.00 % | 150 ms, 200 at most |",
		"| slow | 3 | 1 | up: delay 40 ± 10 ms, loss 3.00 % in bursts of 2 | not started |",
		"| slow (3 bots) | 720: 20.00 %, 360: 80.00 % | 20.00 % | 4 | 4.10 |",
		"| no condition (7 bots) | 720: 100.00 % | 100.00 % | 0 |  |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, section(md, "Network conditions"))
		}
	}
}

func TestMarkdownNoNetwork(t *testing.T) {
	if md := markdown016(t, nil); strings.Contains(md, "Network conditions") {
		t.Error("network section without network")
	}
}

// section returns the Markdown section named heading.
func section(md, heading string) string {
	_, rest, ok := strings.Cut(md, "\n## "+heading+"\n")
	if !ok {
		return ""
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

func TestMarkdownReactionRecovery(t *testing.T) {
	n := networkResult()
	f := func(v float64) *float64 { return &v }
	yes, no := true, false
	n.Bots[0].Steps[0].ReactionSeconds, n.Bots[0].Steps[0].RecoverySeconds, n.Bots[0].Steps[0].Recovered = f(2), f(6), &yes
	n.Bots[1].Steps[0].ReactionSeconds, n.Bots[1].Steps[0].RecoverySeconds, n.Bots[1].Steps[0].Recovered = f(4), f(10), &yes
	n.Bots[2].Steps[0].ReactionSeconds, n.Bots[2].Steps[0].Recovered, n.Bots[2].Steps[0].Reason = f(8), &no, scenario.NotRecoveredBeforeEnd
	n.Bots[0].Steps[0].MissingAfterRetransmissionShare, n.Bots[1].Steps[0].MissingAfterRetransmissionShare = f(0.01), f(0.02)
	md := markdown016(t, n)
	for _, want := range []string{
		"| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |",
		"| slow | 0 | 3 of 3 | 4.0 s, 8.0 s | 2 of 3 | 6.0 s, 10.0 s | 1.50 % |",
		"- b10, step 0: reaction 8.0 s, not recovered (not_recovered_before_end)",
		"- b8, step 0: reaction 2.0 s, recovery 6.0 s",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, section(md, "Network conditions"))
		}
	}
}

func TestMarkdownSenders(t *testing.T) {
	n := networkResult()
	n.Bots[0].Sender = &scenario.SenderResult{Adapts: true, Layers: []scenario.LayerChange{
		{AtSeconds: 0, Heights: []int{180, 360, 720}}, {AtSeconds: 33.1, Heights: []int{180, 360}}}}
	n.Bots[1].Sender = &scenario.SenderResult{Reason: scenario.NoFeedback}
	n.Bots[2].Sender = &scenario.SenderResult{Reason: scenario.NoUplinkCondition}
	md := markdown016(t, n)
	for _, want := range []string{
		"- b8: 180+360+720 from 0.0 s; 180+360 from 33.1 s",
		"- b9: does not adapt (no_feedback)",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, section(md, "Network conditions"))
		}
	}
	if strings.Contains(md, "- b10:") {
		t.Error("a bot with no upload condition is listed as a sender")
	}
}

func TestMarkdownServerEstimate(t *testing.T) {
	n := networkResult()
	for i := range n.Bots {
		n.Bots[i].ServerEstimate = &scenario.ServerEstimate{Reason: scenario.NotGivenByServer}
	}
	if md := markdown016(t, n); !strings.Contains(md, "Server's estimate toward the bots: not given by this server's signaling.") {
		t.Error("no line for a server that gives no estimate")
	}
	n.Bots[0].ServerEstimate = &scenario.ServerEstimate{Kbps: []scenario.EstimateSample{{AtSeconds: 10, Kbps: 3000}, {AtSeconds: 21, Kbps: 600}, {AtSeconds: 25, Kbps: 500}, {AtSeconds: 30, Kbps: 700}}}
	md := markdown016(t, n)
	if want := "| slow | 0 | 600 kbit/s (3 samples, 500 to 700) |"; !strings.Contains(md, want) {
		t.Errorf("missing %q in:\n%s", want, section(md, "Network conditions"))
	}
}
