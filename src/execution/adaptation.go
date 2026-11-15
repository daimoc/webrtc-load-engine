package execution

import (
	"time"

	"webrtc-load-engine/src/scenario"
)

// The network section of a run's result (specs/016-network-conditions,
// contracts/report-network.md): what each step did to each constrained bot,
// measured by its links, and the quality of each group.

// buildNetwork builds the network result at the end of a run. ids are the
// peer IDs by launch index ("" for a bot never launched), requested their
// requested heights; collector, when not nil, gives the groups' quality.
// estimates are the server's estimates toward each constrained bot, nil
// when the adapter cannot read them.
func buildNetwork(s *schedule, ids []string, requested []int, collector *mediaCollector, now time.Time, seed uint64, seedSource string, senders map[string]*scenario.SenderResult, estimates map[string][]scenario.EstimateSample) *scenario.NetworkResult {
	applied := s.applied()
	n := &scenario.NetworkResult{
		Seed: seed, SeedSource: seedSource, SettleSeconds: s.cfg.Settle(),
		TimelineResolutionSeconds: int(mediaCollectInterval / time.Second),
		Groups:                    []scenario.GroupResult{},
		Bots:                      []scenario.BotNetwork{},
	}
	if n.TimelineResolutionSeconds < 1 {
		n.TimelineResolutionSeconds = 1
	}
	constrained := map[string]bool{}
	for gi, g := range s.groups {
		gr := scenario.GroupResult{Name: g.group.Name, Bots: []string{}, Steps: applied[gi]}
		members := map[string]bool{}
		for _, i := range g.bots {
			if i >= len(ids) || ids[i] == "" {
				continue
			}
			gr.Bots = append(gr.Bots, ids[i])
			members[ids[i]] = true
			constrained[ids[i]] = true
			var tl [][]timelineSample
			var counts []countSample
			if collector != nil && collector.bots[ids[i]] != nil {
				tl, counts = collector.bots[ids[i]].timeline, collector.bots[ids[i]].videoCounts
			}
			bn := botNetwork(s, g, applied[gi], i, ids[i], requested[i], tl, counts, now)
			if g.group.HasUplink() {
				if r := senders[ids[i]]; r != nil {
					bn.Sender = r
				} else {
					bn.Sender = &scenario.SenderResult{Reason: scenario.ReceiveOnly}
				}
			}
			if estimates != nil {
				bn.ServerEstimate = &scenario.ServerEstimate{Kbps: estimates[ids[i]]}
				if len(bn.ServerEstimate.Kbps) == 0 {
					bn.ServerEstimate.Reason = scenario.NotApplied
				}
			}
			n.Bots = append(n.Bots, bn)
		}
		if collector != nil {
			gr.Quality = collector.summaryOf(now, func(id string) bool { return members[id] })
		}
		n.Groups = append(n.Groups, gr)
	}
	if collector != nil {
		n.UnconstrainedQuality = collector.summaryOf(now, func(id string) bool { return !constrained[id] })
	}
	return n
}

// botNetwork is one constrained bot's result.
func botNetwork(s *schedule, g *groupRun, steps []scenario.AppliedStep, index int, id string, requested int, timeline [][]timelineSample, counts []countSample, now time.Time) scenario.BotNetwork {
	sh := s.shaperFor(index)
	down, up := sh.down.counters(), sh.up.counters()
	b := scenario.BotNetwork{Bot: id, Group: g.group.Name, RequestedHeight: requested, Steps: []scenario.BotStep{},
		ServerEstimate: &scenario.ServerEstimate{Reason: scenario.NotGivenByServer}}
	for i, st := range g.group.Schedule {
		seconds := stepSeconds(steps[i])
		bs := scenario.BotStep{Index: i}
		if st.Downlink != nil {
			m := directionMeasure(down[i], seconds, st.Downlink)
			bs.Downlink = &m
			if a := steps[i]; a.StartedAtSeconds != nil {
				start := s.start.Add(time.Duration(*a.StartedAtSeconds * float64(time.Second)))
				var end *time.Time
				if st.DurationSeconds != nil && a.EndedAtSeconds != nil {
					e := s.start.Add(time.Duration(*a.EndedAtSeconds * float64(time.Second)))
					end = &e
				}
				stepEnd := now
				if a.EndedAtSeconds != nil {
					stepEnd = s.start.Add(time.Duration(*a.EndedAtSeconds * float64(time.Second)))
				}
				bs.MissingAfterRetransmissionShare = missingShare(counts, start, stepEnd)
				r := reactionRecovery(timeline, requested, start, end, now)
				bs.ReactionSeconds, bs.RecoverySeconds, bs.Recovered, bs.Reason = r.ReactionSeconds, r.RecoverySeconds, r.Recovered, r.Reason
			}
		}
		if st.Uplink != nil {
			m := directionMeasure(up[i], seconds, st.Uplink)
			bs.Uplink = &m
		}
		b.Steps = append(b.Steps, bs)
	}
	b.Sender = &scenario.SenderResult{Reason: scenario.NoUplinkCondition} // Replaced for an upload condition
	return b
}

// stepSeconds is how long a step was in force; 0 if it never started.
func stepSeconds(a scenario.AppliedStep) float64 {
	if a.StartedAtSeconds == nil || a.EndedAtSeconds == nil {
		return 0
	}
	return *a.EndedAtSeconds - *a.StartedAtSeconds
}

// directionMeasure turns one direction's counters over a step of seconds
// into the report's figures; plan says which drops can occur.
func directionMeasure(c stepCounters, seconds float64, plan *scenario.LinkCondition) scenario.DirectionMeasure {
	if c.Offered == 0 || seconds <= 0 {
		return scenario.DirectionMeasure{Reason: scenario.NotApplied}
	}
	m := scenario.DirectionMeasure{OfferedPackets: c.Offered, DeliveredPackets: c.Delivered}
	whole := max(1, int(seconds)) // Whole seconds of the step, as the 10 s windows count them
	var inStep uint64
	for sec, bytes := range c.DeliveredPerSecond {
		if sec < whole {
			inStep += bytes
		}
	}
	m.DeliveredKbps.Mean = round1(float64(inStep*8) / float64(whole) / 1000)
	m.DeliveredKbps.Max10s = round1(maxWindowKbps(c.DeliveredPerSecond, whole))
	capShare := float64(c.DroppedCap) / float64(c.Offered)
	lossShare := float64(c.DroppedLoss) / float64(c.Offered)
	m.DroppedByCapShare, m.DroppedByLossShare = &capShare, &lossShare
	if plan.Loss != nil && c.Bursts > 0 {
		burst := float64(c.DroppedLoss) / float64(c.Bursts)
		m.MeanBurst = &burst
	}
	if c.Delivered > 0 {
		m.AddedDelayMs = scenario.MeanMax{Mean: round1(ms(c.DelaySum) / float64(c.Delivered)), Max: round1(ms(c.DelayMax))}
	}
	return m
}

// maxWindowKbps is the highest rate over 10 consecutive seconds of a step
// of seconds (the whole step when shorter).
func maxWindowKbps(perSecond map[int]uint64, seconds int) float64 {
	window := min(10, max(1, seconds))
	best := 0.0
	for start := 0; start+window <= max(seconds, window); start++ {
		var bytes uint64
		for s := start; s < start+window; s++ {
			bytes += perSecond[s]
		}
		best = max(best, float64(bytes*8)/float64(window)/1000)
	}
	return best
}

func round1(v float64) float64 {
	return float64(int64(v*10+0.5)) / 10
}

// recoveryHold is how long a bot must receive its requested height without
// a freeze to count as recovered (spec US2).
const recoveryHold = 5 * time.Second

// reactionResult is a step's reaction and recovery for one bot.
type reactionResult struct {
	ReactionSeconds, RecoverySeconds *float64
	Recovered                        *bool
	Reason                           string
}

// reactionRecovery measures, on a bot's received video streams, how long
// the server took to react to a step starting at start (a lower height or
// a freeze on a stream that was at the requested height), and, when the
// step ends (end not nil), how long until every such stream was back at
// that height for recoveryHold without a freeze (research R8). runEnd is
// when the measures stop.
func reactionRecovery(streams [][]timelineSample, requested int, start time.Time, end *time.Time, runEnd time.Time) reactionResult {
	if len(streams) == 0 {
		return reactionResult{Reason: scenario.NoVideo}
	}
	var watched [][]timelineSample
	for _, s := range streams {
		var last *timelineSample
		for i := range s {
			if s[i].at.After(start) {
				break
			}
			last = &s[i]
		}
		if last != nil && last.height == requested {
			watched = append(watched, s)
		}
	}
	if len(watched) == 0 {
		return reactionResult{Reason: scenario.NotAtRequestedHeight}
	}
	stepEnd := runEnd
	if end != nil {
		stepEnd = *end
	}
	var r reactionResult
	var first time.Time
	for _, s := range watched {
		for _, x := range s {
			if !x.at.After(start) || x.at.After(stepEnd) {
				continue
			}
			if x.height < requested || x.froze {
				if first.IsZero() || x.at.Before(first) {
					first = x.at
				}
				break
			}
		}
	}
	if first.IsZero() {
		r.Reason = scenario.NotLowered
	} else {
		v := first.Sub(start).Seconds()
		r.ReactionSeconds = &v
	}
	if end == nil {
		return r
	}
	// Candidate times: the collections from the end of the step on.
	for _, x := range watched[0] {
		if x.at.Before(*end) {
			continue
		}
		if heldFrom(watched, requested, x.at) {
			v := x.at.Sub(*end).Seconds()
			yes := true
			r.RecoverySeconds, r.Recovered = &v, &yes
			return r
		}
	}
	no := false
	r.Recovered = &no
	if r.Reason == "" {
		r.Reason = scenario.NotRecoveredBeforeEnd
	}
	return r
}

// heldFrom reports whether every stream is at requested, with no freeze,
// in all its samples from t to t+recoveryHold, and has samples that far.
func heldFrom(streams [][]timelineSample, requested int, t time.Time) bool {
	until := t.Add(recoveryHold)
	for _, s := range streams {
		covered := false
		for _, x := range s {
			if x.at.Before(t) || x.at.After(until) {
				continue
			}
			if x.height != requested || x.froze {
				return false
			}
			if !x.at.Before(until.Add(-mediaCollectInterval)) {
				covered = true
			}
		}
		if !covered {
			return false
		}
	}
	return true
}

// missingShare is the share of the video a bot should have received over
// [start, end] that was still missing after retransmissions (FR-009),
// from the collections nearest those times; nil without video then.
func missingShare(counts []countSample, start, end time.Time) *float64 {
	var from, to *countSample
	for i := range counts {
		c := &counts[i]
		if !c.at.After(start) {
			from = c
		}
		if !c.at.After(end) {
			to = c
		}
	}
	if from == nil && len(counts) > 0 && !counts[0].at.After(end) {
		from = &counts[0]
	}
	if from == nil || to == nil || to == from {
		return nil
	}
	packets := float64(to.packets) - float64(from.packets)
	lost := float64(to.lost - from.lost)
	if packets+lost <= 0 {
		return nil
	}
	v := max(0, lost) / (packets + max(0, lost))
	return &v
}
