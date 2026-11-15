package execution

import (
	"log/slog"
	"sync"
	"time"

	"webrtc-load-engine/src/scenario"
)

// schedule applies a test plan's network groups to the bots' links
// (specs/016-network-conditions, research R5): each constrained bot gets a
// shaper at launch, and each group's steps are put in force and removed at
// their times, counted from the run's first launch.
type schedule struct {
	cfg        *scenario.NetworkConfig
	runSeconds int
	seed       uint64
	names      []string // Group name by launch index
	logger     *slog.Logger

	mu       sync.Mutex
	start    time.Time
	lastJoin time.Time
	groups   []*groupRun
	shapers  []*shaper // By launch index; nil for bots in no group
	finished bool
}

// groupRun is one group's state during a run.
type groupRun struct {
	group   scenario.BotGroup
	current int          // Step in force; -1: none
	starts  []*time.Time // Actual start of each step
	ends    []*time.Time
	bots    []int // Launch indexes
}

func newSchedule(cfg *scenario.NetworkConfig, participants, runSeconds int, seed uint64) *schedule {
	s := &schedule{cfg: cfg, runSeconds: runSeconds, seed: seed, names: cfg.Assign(participants), logger: slog.Default(),
		shapers: make([]*shaper, participants)}
	byName := map[string]*groupRun{}
	for _, g := range cfg.Groups {
		gr := &groupRun{group: g, current: -1, starts: make([]*time.Time, len(g.Schedule)), ends: make([]*time.Time, len(g.Schedule))}
		s.groups = append(s.groups, gr)
		byName[g.Name] = gr
	}
	for i, name := range s.names {
		gr := byName[name]
		if gr == nil {
			continue
		}
		gr.bots = append(gr.bots, i)
		sh := &shaper{}
		var down, up bool
		for _, st := range gr.group.Schedule {
			down = down || st.Downlink != nil
			up = up || st.Uplink != nil
		}
		if down {
			sh.down = newLinkShaper(s.seedFor(i, 0))
		}
		if up {
			sh.up = newLinkShaper(s.seedFor(i, 1))
		}
		s.shapers[i] = sh
	}
	return s
}

// seedFor is the seed of one bot's link in one direction (0 down, 1 up):
// the run's seed mixed with the launch index and the direction (R4).
func (s *schedule) seedFor(index, direction int) uint64 {
	z := s.seed + uint64(2*index+direction+1)*0x9E3779B97F4A7C15
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// shaperFor returns the shaper of the bot launched at index, nil for a bot
// in no group.
func (s *schedule) shaperFor(index int) *shaper {
	if s == nil || index < 0 || index >= len(s.shapers) {
		return nil
	}
	return s.shapers[index]
}

// groupOf returns the group name of the bot launched at index.
func (s *schedule) groupOf(index int) string {
	if s == nil || index < 0 || index >= len(s.names) {
		return ""
	}
	return s.names[index]
}

// begin marks the run's first launch.
func (s *schedule) begin(now time.Time) {
	s.mu.Lock()
	s.start = now
	s.mu.Unlock()
	s.advance(now)
}

// joined records a bot's join, for the start-up flag (FR-011).
func (s *schedule) joined(now time.Time) {
	s.mu.Lock()
	if now.After(s.lastJoin) {
		s.lastJoin = now
	}
	s.mu.Unlock()
}

// advance puts in force, or removes, every step due by now.
func (s *schedule) advance(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.start.IsZero() {
		return
	}
	elapsed := now.Sub(s.start)
	for _, g := range s.groups {
		want := -1
		for i, st := range g.group.Schedule {
			if elapsed >= time.Duration(st.StartSeconds)*time.Second && elapsed < time.Duration(st.End(s.runSeconds))*time.Second {
				want = i
				break
			}
		}
		if want == g.current {
			continue
		}
		t := now
		if g.current >= 0 {
			g.ends[g.current] = &t
		}
		g.current = want
		var down, up *scenario.LinkCondition
		if want >= 0 {
			g.starts[want] = &t
			down, up = g.group.Schedule[want].Downlink, g.group.Schedule[want].Uplink
		}
		for _, i := range g.bots {
			sh := s.shapers[i]
			sh.down.setCondition(stepOrNone(want, down), down, now)
			sh.up.setCondition(stepOrNone(want, up), up, now)
		}
		s.logger.Info("Network condition changed", slog.String("group", g.group.Name), slog.Int("step", want),
			slog.Any("downlink", down), slog.Any("uplink", up))
	}
}

func stepOrNone(step int, c *scenario.LinkCondition) int {
	if c == nil {
		return -1
	}
	return step
}

// run advances the schedule every 100 ms until stop is closed.
func (s *schedule) run(stop <-chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			s.advance(now)
		}
	}
}

// finish ends the step in force at now, the end of the run.
func (s *schedule) finish(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	for _, g := range s.groups {
		if g.current >= 0 {
			t := now
			g.ends[g.current] = &t
		}
	}
}

// close stops every link.
func (s *schedule) close() {
	if s == nil {
		return
	}
	for _, sh := range s.shapers {
		sh.close()
	}
}

// applied returns each group's steps as they ran, by group.
func (s *schedule) applied() [][]scenario.AppliedStep {
	s.mu.Lock()
	defer s.mu.Unlock()
	settle := time.Duration(s.cfg.Settle()) * time.Second
	out := make([][]scenario.AppliedStep, len(s.groups))
	for gi, g := range s.groups {
		for i, st := range g.group.Schedule {
			a := scenario.AppliedStep{Index: i, Planned: st}
			if t := g.starts[i]; t != nil {
				v := round3(t.Sub(s.start).Seconds())
				a.StartedAtSeconds = &v
				a.DuringStartUp = s.lastJoin.IsZero() || t.Before(s.lastJoin.Add(settle))
			}
			if t := g.ends[i]; t != nil {
				v := round3(t.Sub(s.start).Seconds())
				a.EndedAtSeconds = &v
			}
			out[gi] = append(out[gi], a)
		}
	}
	return out
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}
