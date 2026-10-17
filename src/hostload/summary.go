package hostload

import (
	"math"

	"webrtc-load-engine/src/scenario"
)

// Summary returns the server section of the run's summary (FR-006). Call
// it after Run has returned.
func (s *Sampler) Summary() scenario.ServerLoad {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.containers) == 0 {
		return scenario.ServerLoad{Reason: scenario.ReasonNotRequested}
	}
	out := scenario.ServerLoad{IntervalSeconds: int(s.interval.Seconds())}
	if !s.startAt.IsZero() {
		out.Period = &scenario.Period{Start: s.startAt, End: s.end}
	}
	total := &scenario.ServerTotal{}
	reasons := map[string]bool{}
	for i, name := range s.containers {
		a := &s.acc[i]
		c := scenario.ContainerLoad{Name: name, LoadFigures: rounded(a.figures()), SharedWith: s.sharedWith[name]}
		switch {
		case a.samples == 0:
			c.Status = scenario.LoadStatusUnavailable
			c.Reason = a.reason
			if c.Reason == "" {
				c.Reason = scenario.ReasonNoSamples
			}
			reasons[c.Reason] = true
		case a.missed > 0:
			c.Status = scenario.LoadStatusPartial
			c.Reason = a.reason
		default:
			c.Status = scenario.LoadStatusMeasured
		}
		if a.samples > 0 {
			total.ContainersMeasured++
			total.CPUThreads.Average += a.average()
			total.CPUThreads.First += a.firstCPU
			total.MemoryBytes.First += a.firstMem
			total.Missed += a.missed
		}
		out.Containers = append(out.Containers, c)
	}
	if total.ContainersMeasured == 0 {
		// Nothing measured: one reason when all containers share it.
		if len(reasons) == 1 {
			for r := range reasons {
				out.Reason = r
			}
		}
		return out
	}
	out.Available = true
	total.CPUThreads.Peak = s.total.peakCPU
	total.MemoryBytes.Peak = s.total.peakMem
	total.Samples = s.total.samples
	total.LoadFigures = rounded(total.LoadFigures)
	out.Total = total
	return out
}

// rounded rounds CPU figures to 3 decimals.
func rounded(f scenario.LoadFigures) scenario.LoadFigures {
	r := func(v float64) float64 { return math.Round(v*1000) / 1000 }
	f.CPUThreads = scenario.CPUThreads{Average: r(f.CPUThreads.Average), Peak: r(f.CPUThreads.Peak), First: r(f.CPUThreads.First)}
	return f
}

// LoadMachine returns the load-machine section of the run's summary: the
// engine's figures over the same ticks as the server section. Call it after
// Run has returned.
func (s *Sampler) LoadMachine() scenario.LoadMachine {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := scenario.LoadMachine{
		Engine:          rounded(s.proc.figures()),
		HardwareThreads: hardwareThreads(),
		UsableThreads:   usableThreads(),
	}
	if !s.startAt.IsZero() {
		out.Period = &scenario.Period{Start: s.startAt, End: s.end}
	}
	return out
}
