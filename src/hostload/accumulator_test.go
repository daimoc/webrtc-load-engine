package hostload

import (
	"math"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

var t0 = time.Unix(1000, 0)

// at returns a sample taken s seconds after t0 with cpu seconds of CPU
// time used so far and mem bytes of memory.
func at(s, cpu float64, mem int64) sample {
	return sample{at: t0.Add(time.Duration(s * float64(time.Second))), cpuNS: uint64(cpu * 1e9), memBytes: mem}
}

func failed(reason string) sample { return sample{err: reason} }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestAccumulator_Rates(t *testing.T) {
	var a accumulator
	if _, ok := a.add(at(0, 10, 100)); ok {
		t.Error("first sample gave an interval")
	}
	if r, ok := a.add(at(5, 15, 300)); !ok || !near(r, 1) {
		t.Errorf("interval 1: %v %v, want 1 thread", r, ok)
	}
	if r, _ := a.add(at(10, 30, 200)); !near(r, 3) {
		t.Errorf("interval 2: %v, want 3 threads", r)
	}
	a.add(at(20, 30, 150)) // Idle for 10 s
	f := a.figures()
	want := scenario.LoadFigures{
		// 20 s of CPU over 20 s of wall time, not the mean of 1, 3 and 0.
		CPUThreads:  scenario.CPUThreads{Average: 1, Peak: 3, First: 1},
		MemoryBytes: scenario.MemoryBytes{Peak: 300, First: 100},
		Samples:     4,
	}
	if f != want {
		t.Errorf("figures %+v, want %+v", f, want)
	}
}

func TestAccumulator_FailedSample(t *testing.T) {
	var a accumulator
	a.add(at(0, 0, 10))
	a.add(failed(scenario.ReasonNotRunning))
	// The next interval is measured from the last good sample.
	if r, ok := a.add(at(10, 20, 10)); !ok || !near(r, 2) {
		t.Errorf("after a miss: %v %v, want 2 threads over 10 s", r, ok)
	}
	f := a.figures()
	if f.Samples != 2 || f.Missed != 1 || !near(f.CPUThreads.Average, 2) {
		t.Errorf("figures %+v", f)
	}
	if a.reason != scenario.ReasonNotRunning {
		t.Errorf("reason %q", a.reason)
	}
	a.miss() // Skipped tick: counted, reason kept
	if f := a.figures(); f.Missed != 2 || a.reason != scenario.ReasonNotRunning {
		t.Errorf("after skip: %+v %q", f, a.reason)
	}
}

func TestAccumulator_Restart(t *testing.T) {
	var a accumulator
	a.add(at(0, 100, 1))
	a.add(at(10, 110, 1)) // 1 thread
	// Restarted: the counter starts again from 0. The interval is missed.
	if _, ok := a.add(at(20, 2, 1)); ok {
		t.Error("restart interval reported")
	}
	if r, _ := a.add(at(30, 32, 1)); !near(r, 3) {
		t.Errorf("after restart: %v, want 3", r)
	}
	f := a.figures()
	// Segments: 10 s CPU over 10 s, then 30 s CPU over 10 s: 40 / 20.
	if !near(f.CPUThreads.Average, 2) || f.Missed != 1 || f.Samples != 4 || !near(f.CPUThreads.Peak, 3) {
		t.Errorf("figures %+v", f)
	}
}

func TestAccumulator_FewSamples(t *testing.T) {
	var none accumulator
	if f := none.figures(); f != (scenario.LoadFigures{}) {
		t.Errorf("no sample: %+v", f)
	}
	var one accumulator
	one.add(at(0, 5, 42))
	if f := one.figures(); f.Samples != 1 || f.CPUThreads != (scenario.CPUThreads{}) || f.MemoryBytes.Peak != 42 {
		t.Errorf("one sample: %+v", f)
	}
}

func TestAccumulator_NotClamped(t *testing.T) {
	var a accumulator
	a.add(at(0, 0, 0))
	if r, _ := a.add(at(1, 40, 0)); !near(r, 40) {
		t.Errorf("40 threads busy: %v", r)
	}
}
