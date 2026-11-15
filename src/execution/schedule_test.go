package execution

import (
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

func ip(v int) *int { return &v }

// testNetwork is a plan of 4 bots with one group of 2: a downlink cap from
// 10 s to 20 s, then a delay from 30 s to the end of a 60 s run.
func testNetwork() *scenario.NetworkConfig {
	return &scenario.NetworkConfig{Groups: []scenario.BotGroup{{Name: "slow", Count: 2, Schedule: []scenario.ScheduleStep{
		{StartSeconds: 10, DurationSeconds: ip(10), Downlink: &scenario.LinkCondition{RateKbps: 300, QueueMs: 200}},
		{StartSeconds: 30, Uplink: &scenario.LinkCondition{RateKbps: 400, QueueMs: 200}},
	}}}}
}

func TestSchedule_StepsApplied(t *testing.T) {
	t0 := time.Unix(1000, 0)
	s := newSchedule(testNetwork(), 4, 60, 7)
	s.begin(t0)
	if s.shaperFor(0) != nil || s.shaperFor(1) != nil {
		t.Fatal("bots in no group got a shaper")
	}
	a, b := s.shaperFor(2), s.shaperFor(3)
	if a == nil || b == nil || a == b {
		t.Fatal("each bot of the group needs its own shaper")
	}
	if a.down == nil || a.up == nil {
		t.Fatal("the group uses both directions: both links expected")
	}
	at := func(sec float64) time.Time { return t0.Add(time.Duration(sec * float64(time.Second))) }

	s.advance(at(9.9))
	if a.down.cond != nil {
		t.Error("step 0 in force before 10 s")
	}
	s.advance(at(10.05))
	if a.down.cond == nil || a.down.cond.RateKbps != 300 || a.down.step != 0 {
		t.Errorf("step 0 not in force at 10 s: %+v", a.down.cond)
	}
	if a.up.cond != nil {
		t.Error("uplink conditioned by a downlink step")
	}
	s.advance(at(20.05))
	if a.down.cond != nil {
		t.Error("step 0 still in force after its end")
	}
	s.advance(at(30.1))
	if a.up.cond == nil || a.up.step != 1 || b.up.cond == nil {
		t.Error("step 1 not in force at 30 s")
	}
	s.advance(at(60))
	steps := s.applied()
	if len(steps[0]) != 2 {
		t.Fatalf("applied %v", steps)
	}
	st := steps[0][0]
	if st.StartedAtSeconds == nil || *st.StartedAtSeconds < 10 || *st.StartedAtSeconds > 10.1 || *st.EndedAtSeconds < 20 || *st.EndedAtSeconds > 20.1 {
		t.Errorf("step 0 times %v–%v, want about 10–20 s", st.StartedAtSeconds, st.EndedAtSeconds)
	}
	if e := steps[0][1].EndedAtSeconds; e == nil || *e != 60 {
		t.Errorf("open step ended at %v, want the end of the run (60)", e)
	}
}

func TestSchedule_LateJoinGetsStepInForce(t *testing.T) {
	t0 := time.Unix(1000, 0)
	s := newSchedule(testNetwork(), 4, 60, 7)
	s.begin(t0)
	s.advance(t0.Add(15 * time.Second))
	// The shaper exists from the start; a bot launched now has it in force.
	if sh := s.shaperFor(3); sh.down.cond == nil || sh.down.cond.RateKbps != 300 {
		t.Error("a bot launched during a step does not have it in force")
	}
}

func TestSchedule_DuringStartUp(t *testing.T) {
	t0 := time.Unix(1000, 0)
	s := newSchedule(testNetwork(), 4, 60, 7) // settle 10 s by default
	s.begin(t0)
	s.joined(t0.Add(5 * time.Second))
	s.advance(t0.Add(10 * time.Second))
	s.advance(t0.Add(20 * time.Second))
	s.advance(t0.Add(30 * time.Second))
	steps := s.applied()[0]
	if !steps[0].DuringStartUp {
		t.Error("a step at 10 s, 5 s after the last join, is not flagged")
	}
	if steps[1].DuringStartUp {
		t.Error("a step at 30 s, 25 s after the last join, is flagged")
	}
}

func TestSchedule_StepNotReached(t *testing.T) {
	t0 := time.Unix(1000, 0)
	s := newSchedule(testNetwork(), 4, 60, 7)
	s.begin(t0)
	s.advance(t0.Add(15 * time.Second))
	s.finish(t0.Add(16 * time.Second)) // Stopped early
	steps := s.applied()[0]
	if steps[0].EndedAtSeconds == nil || *steps[0].EndedAtSeconds != 16 {
		t.Errorf("step cut by the end of the run ended at %v, want 16", steps[0].EndedAtSeconds)
	}
	if steps[1].StartedAtSeconds != nil {
		t.Error("a step after the end of the run has a start")
	}
}

func TestSchedule_Seeds(t *testing.T) {
	a := newSchedule(testNetwork(), 4, 60, 7)
	b := newSchedule(testNetwork(), 4, 60, 7)
	if a.seedFor(2, 0) != b.seedFor(2, 0) || a.seedFor(2, 0) == a.seedFor(3, 0) || a.seedFor(2, 0) == a.seedFor(2, 1) {
		t.Error("seeds must follow the run seed, the launch index and the direction")
	}
}
