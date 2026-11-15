package execution

import (
	"math"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// TestDirectionMeasure_FromCounters: the report's figures come from what
// the link counted, never from the plan (FR-006).
func TestDirectionMeasure_FromCounters(t *testing.T) {
	c := stepCounters{Offered: 1000, Delivered: 900, DroppedCap: 100, OfferedBytes: 1000 * 1000, Delivered8: 900 * 1000,
		DelaySum: 900 * 150 * time.Millisecond, DelayMax: 201 * time.Millisecond, DeliveredPerSecond: map[int]uint64{}}
	// 20 s at 45 kB/s (360 kbit/s), with one 10 s stretch at 50 kB/s.
	for s := 0; s < 20; s++ {
		c.DeliveredPerSecond[s] = 45000
	}
	for s := 5; s < 15; s++ {
		c.DeliveredPerSecond[s] = 50000
	}
	m := directionMeasure(c, 20, &scenario.LinkCondition{RateKbps: 400})
	if m.OfferedPackets != 1000 || m.DeliveredPackets != 900 {
		t.Errorf("packets %d/%d", m.OfferedPackets, m.DeliveredPackets)
	}
	if math.Abs(m.DeliveredKbps.Mean-380) > 0.5 { // 950 kB delivered in the 20 s
		t.Errorf("mean %.1f kbit/s, want 380 (950 kB over 20 s)", m.DeliveredKbps.Mean)
	}
	if math.Abs(m.DeliveredKbps.Max10s-400) > 0.5 {
		t.Errorf("max over 10 s %.1f kbit/s, want 400", m.DeliveredKbps.Max10s)
	}
	if m.DroppedByCapShare == nil || math.Abs(*m.DroppedByCapShare-0.1) > 1e-9 {
		t.Errorf("dropped by cap %v, want 0.1", m.DroppedByCapShare)
	}
	if m.DroppedByLossShare == nil || *m.DroppedByLossShare != 0 || m.MeanBurst != nil {
		t.Errorf("no loss planned: share %v, burst %v; want 0 and none", m.DroppedByLossShare, m.MeanBurst)
	}
	if math.Abs(m.AddedDelayMs.Mean-150) > 0.01 || m.AddedDelayMs.Max != 201 {
		t.Errorf("delay %+v", m.AddedDelayMs)
	}
}

func TestDirectionMeasure_NothingOffered(t *testing.T) {
	m := directionMeasure(stepCounters{}, 20, &scenario.LinkCondition{RateKbps: 400})
	if m.Reason != scenario.NotApplied || m.DroppedByCapShare != nil {
		t.Errorf("nothing offered: %+v, want reason not_applied and no shares", m)
	}
}

func TestBuildNetwork(t *testing.T) {
	t0 := time.Unix(1000, 0)
	s := newSchedule(testNetwork(), 4, 60, 7)
	s.begin(t0)
	s.joined(t0.Add(time.Second))
	s.advance(t0.Add(10 * time.Second))
	// Bot 2 sends through step 0; bot 3 never connects.
	for i := 0; i < 50; i++ {
		s.shaperFor(2).down.admit(t0.Add(10*time.Second+time.Duration(i)*100*time.Millisecond), 1000)
	}
	s.advance(t0.Add(20 * time.Second))
	s.finish(t0.Add(25 * time.Second))
	ids := []string{"p0", "p1", "p2", "p3"}
	n := buildNetwork(s, ids, []int{720, 720, 720, 360}, nil, t0.Add(25*time.Second), 7, scenario.SeedFromTestPlan,
		map[string]*scenario.SenderResult{"p2": {Adapts: true, Layers: []scenario.LayerChange{{AtSeconds: 0, Heights: []int{180, 360, 720}}}}}, nil)
	if n.Seed != 7 || n.SeedSource != scenario.SeedFromTestPlan || n.SettleSeconds != 10 || n.TimelineResolutionSeconds != int(mediaCollectInterval/time.Second) {
		t.Errorf("header %+v", n)
	}
	if len(n.Groups) != 1 || n.Groups[0].Name != "slow" || len(n.Groups[0].Bots) != 2 || n.Groups[0].Bots[0] != "p2" {
		t.Fatalf("groups %+v", n.Groups)
	}
	if len(n.Bots) != 2 {
		t.Fatalf("bots %+v", n.Bots)
	}
	b2, b3 := n.Bots[0], n.Bots[1]
	if b2.Bot != "p2" || b2.Group != "slow" || b2.RequestedHeight != 720 || b3.RequestedHeight != 360 {
		t.Errorf("bots %+v %+v", b2, b3)
	}
	if d := b2.Steps[0].Downlink; d == nil || d.OfferedPackets != 50 || d.Reason != "" {
		t.Errorf("bot 2 step 0 downlink %+v", d)
	}
	if d := b3.Steps[0].Downlink; d == nil || d.Reason != scenario.NotApplied {
		t.Errorf("bot 3 never sent: %+v, want not_applied", d)
	}
	if b2.Steps[0].Uplink != nil {
		t.Error("a downlink step has an uplink measure")
	}
	// Step 1 (uplink from 30 s) never started.
	if u := b2.Steps[1].Uplink; u == nil || u.Reason != scenario.NotApplied {
		t.Errorf("step never started: %+v", u)
	}
	if b2.Sender == nil || !b2.Sender.Adapts || b2.ServerEstimate == nil || b2.ServerEstimate.Reason != scenario.NotGivenByServer {
		t.Errorf("sender %+v, estimate %+v", b2.Sender, b2.ServerEstimate)
	}
	if b3.Sender == nil || b3.Sender.Reason != scenario.ReceiveOnly {
		t.Errorf("a group bot with an upload step and no sender: %+v, want receive_only", b3.Sender)
	}
}

// timeline builds one stream's samples every 2 s from t0: heights[i] at
// t0 + 2i s, frozen where froze[i].
func timeline(t0 time.Time, heights []int, froze map[int]bool) []timelineSample {
	var out []timelineSample
	for i, h := range heights {
		out = append(out, timelineSample{at: t0.Add(time.Duration(2*i) * time.Second), height: h, froze: froze[i]})
	}
	return out
}

func TestReactionRecovery(t *testing.T) {
	t0 := time.Unix(1000, 0)
	start, end := t0.Add(10*time.Second), t0.Add(30*time.Second)
	// Samples at 0, 2, ... 48 s. 720 until 12 s, 180 from 14 s, 720 again from 36 s.
	var hs []int
	for i := 0; i < 25; i++ {
		switch at := 2 * i; {
		case at <= 12:
			hs = append(hs, 720)
		case at < 36:
			hs = append(hs, 180)
		default:
			hs = append(hs, 720)
		}
	}
	streams := [][]timelineSample{timeline(t0, hs, nil), timeline(t0, hs, nil)}
	r := reactionRecovery(streams, 720, start, &end, t0.Add(48*time.Second))
	if r.Reason != "" || r.ReactionSeconds == nil || *r.ReactionSeconds != 4 {
		t.Errorf("reaction %v (%s), want 4 s (first lower sample at 14 s)", r.ReactionSeconds, r.Reason)
	}
	if r.RecoverySeconds == nil || *r.RecoverySeconds != 6 || r.Recovered == nil || !*r.Recovered {
		t.Errorf("recovery %v, want 6 s (720 held from 36 s)", r.RecoverySeconds)
	}

	// A freeze counts as a reaction; a freeze after the end delays the
	// recovery until 5 s without one.
	froze := map[int]bool{6: true, 16: true} // 12 s and 32 s
	flat := make([]int, 25)
	for i := range flat {
		flat[i] = 720
	}
	r = reactionRecovery([][]timelineSample{timeline(t0, flat, froze)}, 720, start, &end, t0.Add(48*time.Second))
	if *r.ReactionSeconds != 2 {
		t.Errorf("reaction %v, want 2 s (freeze at 12 s)", *r.ReactionSeconds)
	}
	if *r.RecoverySeconds != 4 { // From 34 s: 34, 36, 38 s without a freeze
		t.Errorf("recovery %v, want 4 s", *r.RecoverySeconds)
	}

	// Never back at 720 before the end of the run.
	low := append([]int(nil), hs[:18]...)
	for len(low) < 25 {
		low = append(low, 180)
	}
	r = reactionRecovery([][]timelineSample{timeline(t0, low, nil)}, 720, start, &end, t0.Add(48*time.Second))
	if r.Recovered == nil || *r.Recovered || r.RecoverySeconds != nil || r.Reason != scenario.NotRecoveredBeforeEnd {
		t.Errorf("not recovered: %+v", r)
	}

	// No stream at its height when the step began.
	r = reactionRecovery([][]timelineSample{timeline(t0, low[13:], nil)}, 720, start, &end, t0.Add(48*time.Second))
	if r.Reason != scenario.NotAtRequestedHeight || r.ReactionSeconds != nil {
		t.Errorf("not at its height: %+v", r)
	}

	// No video at all.
	r = reactionRecovery(nil, 720, start, &end, t0.Add(48*time.Second))
	if r.Reason != scenario.NoVideo {
		t.Errorf("no video: %+v", r)
	}

	// An open step has a reaction and no recovery.
	r = reactionRecovery(streams, 720, start, nil, t0.Add(48*time.Second))
	if r.ReactionSeconds == nil || r.RecoverySeconds != nil || r.Recovered != nil {
		t.Errorf("open step: %+v", r)
	}

	// Never lowered and never frozen.
	r = reactionRecovery([][]timelineSample{timeline(t0, flat, nil)}, 720, start, &end, t0.Add(48*time.Second))
	if r.ReactionSeconds != nil || r.Reason != scenario.NotLowered {
		t.Errorf("never lowered: %+v", r)
	}
}

func TestMissingShare(t *testing.T) {
	t0 := time.Unix(1000, 0)
	counts := []countSample{
		{at: t0, packets: 1000, lost: 0},
		{at: t0.Add(10 * time.Second), packets: 2000, lost: 10},
		{at: t0.Add(20 * time.Second), packets: 2900, lost: 110},
		{at: t0.Add(30 * time.Second), packets: 3900, lost: 110},
	}
	// From 10 s to 20 s: 900 received, 100 more missing.
	got := missingShare(counts, t0.Add(10*time.Second), t0.Add(20*time.Second))
	if got == nil || math.Abs(*got-0.1) > 1e-9 {
		t.Errorf("missing share %v, want 0.1", got)
	}
	if missingShare(nil, t0, t0.Add(time.Second)) != nil {
		t.Error("a share without video")
	}
}
