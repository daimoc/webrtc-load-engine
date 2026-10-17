package hostload

import (
	"time"

	"webrtc-load-engine/src/scenario"
)

// sample is one reading of a target (a container or the engine's process).
type sample struct {
	at       time.Time // When the answer arrived
	cpuNS    uint64    // Cumulative CPU time
	memBytes int64
	err      string // scenario.Reason* when the reading failed
}

// accumulator folds a target's samples as they come (research R3,
// data-model "Target Accumulator"). The CPU average is the CPU time over the
// wall time between the first and last samples, so missed intervals weigh
// right; a counter that goes down (restart) closes a segment.
type accumulator struct {
	segStart, last sample
	inSegment      bool
	closedCPU      uint64        // CPU time of closed segments
	closedWall     time.Duration // Wall time of closed segments

	peakCPU, firstCPU float64
	haveInterval      bool
	firstMem, peakMem int64
	samples, missed   int
	reason            string // Last failure reason
}

// add folds s in and returns the CPU in threads over the interval since
// the last good sample, if there is one.
func (a *accumulator) add(s sample) (threads float64, ok bool) {
	if s.err != "" {
		a.missed++
		a.reason = s.err
		return 0, false
	}
	a.samples++
	if a.samples == 1 {
		a.firstMem = s.memBytes
	}
	a.peakMem = max(a.peakMem, s.memBytes)
	if !a.inSegment {
		a.segStart, a.last, a.inSegment = s, s, true
		return 0, false
	}
	if s.cpuNS < a.last.cpuNS { // Restarted
		a.closeSegment()
		a.segStart, a.last, a.inSegment = s, s, true
		a.missed++
		return 0, false
	}
	wall := s.at.Sub(a.last.at)
	if wall <= 0 {
		return 0, false
	}
	threads = float64(s.cpuNS-a.last.cpuNS) / float64(wall.Nanoseconds())
	a.last = s
	if !a.haveInterval {
		a.firstCPU, a.haveInterval = threads, true
	}
	a.peakCPU = max(a.peakCPU, threads)
	return threads, true
}

// miss counts an interval without a sample (a skipped tick).
func (a *accumulator) miss() { a.missed++ }

func (a *accumulator) closeSegment() {
	a.closedCPU += a.last.cpuNS - a.segStart.cpuNS
	a.closedWall += a.last.at.Sub(a.segStart.at)
}

// average is the CPU in threads over all segments; 0 without an interval.
func (a *accumulator) average() float64 {
	cpu, wall := a.closedCPU, a.closedWall
	if a.inSegment {
		cpu += a.last.cpuNS - a.segStart.cpuNS
		wall += a.last.at.Sub(a.segStart.at)
	}
	if wall <= 0 {
		return 0
	}
	return float64(cpu) / float64(wall.Nanoseconds())
}

func (a *accumulator) figures() scenario.LoadFigures {
	return scenario.LoadFigures{
		CPUThreads:  scenario.CPUThreads{Average: a.average(), Peak: a.peakCPU, First: a.firstCPU},
		MemoryBytes: scenario.MemoryBytes{Peak: a.peakMem, First: a.firstMem},
		Samples:     a.samples,
		Missed:      a.missed,
	}
}
