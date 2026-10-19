package media

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	// playoutDelay is the fixed delay of the modelled receiver (research R3).
	playoutDelay = 60 * time.Millisecond
	// restartSeqJump and restartTSJump detect a sender restart: the stream
	// is then measured as a new one.
	restartSeqJump = 1000
	restartTSJump  = 10 * time.Second
	// pendingWindow bounds the missing frames tracked; older ones are
	// concealed for good.
	pendingWindow = 64
)

// pendingFrame is a frame skipped by the sequence numbers, waiting for its
// packet or its deadline.
type pendingFrame struct {
	seq       int64 // Extended sequence number
	deadline  time.Time
	concealed bool // Counted at its deadline; a late arrival is not counted again
}

// AudioQuality measures the audio a listener would not hear in time on one
// received stream (research R3): a receiver plays each frame at
// base + media time + playoutDelay, where base is the fastest
// arrival − media time seen; a frame is concealed when its packet is
// missing at that deadline or arrives after it, once.
type AudioQuality struct {
	clockRate float64
	frame     time.Duration
	sendTimes *SendTimes          // Nil: no latency
	observer  prometheus.Observer // Latency samples, seconds

	mu        sync.Mutex
	started   bool
	firstSeq  int64 // Extended
	highest   int64 // Extended highest sequence number
	firstTS   uint32
	highestTS uint32
	epoch     time.Time     // First arrival; times below are relative to it
	base      time.Duration // min(arrival − media time)
	pending   []pendingFrame

	concealed     uint64
	expectedPrev  uint64 // Frames expected by previous runs of a restarted stream
	periodBase    uint64 // Expected at the period start
	periodConceal uint64

	latency LatencyBuckets
	lookups uint64 // Packets looked up in sendTimes
}

// AudioQualitySnapshot is a point-in-time copy of an AudioQuality, in
// frames.
type AudioQualitySnapshot struct {
	Frame           time.Duration
	Concealed       uint64
	Expected        uint64
	PeriodConcealed uint64
	PeriodExpected  uint64

	Latency        LatencyBuckets // Send-to-receive time of the packets found in the send times
	LatencyLookups uint64         // Packets looked up in the send times, found or not
}

// NewAudioQuality creates the measures of a stream with the given RTP clock
// rate and frame duration.
func NewAudioQuality(clockRate uint32, frame time.Duration) *AudioQuality {
	return &AudioQuality{clockRate: float64(clockRate), frame: frame, pending: make([]pendingFrame, 0, pendingWindow)}
}

// SetLatency makes the stream measure the latency of its packets from the
// scenario's send times, observing each sample (seconds) into o. Call it
// before the first Update.
func (q *AudioQuality) SetLatency(st *SendTimes, o prometheus.Observer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sendTimes, q.observer = st, o
}

// measureLatency counts the latency of a received packet, if its send time
// is known.
func (q *AudioQuality) measureLatency(ts uint32, arrival time.Time) {
	if q.sendTimes == nil {
		return
	}
	q.lookups++
	if at, ok := q.sendTimes.Lookup(ts); ok {
		if d := arrival.Sub(at); d >= 0 {
			q.latency.Add(d)
			q.observer.Observe(d.Seconds())
		}
	}
}

// Update records a received packet.
func (q *AudioQuality) Update(seq uint16, ts uint32, arrival time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.started {
		d := int64(int16(seq - uint16(q.highest)))
		dts := time.Duration(float64(int32(ts-q.highestTS)) / q.clockRate * float64(time.Second))
		if d > restartSeqJump || d < -restartSeqJump || dts > restartTSJump || dts < -restartTSJump {
			q.restart()
		}
	}
	if !q.started {
		q.started = true
		q.firstSeq, q.highest = int64(seq), int64(seq)
		q.firstTS, q.highestTS = ts, ts
		if q.epoch.IsZero() {
			q.epoch = arrival
		}
		q.base = arrival.Sub(q.epoch) - q.mediaTime(ts)
		q.measureLatency(ts, arrival)
		q.checkPending(arrival)
		return
	}

	ext := q.highest + int64(int16(seq-uint16(q.highest)))
	mt := q.mediaTime(ts)
	if b := arrival.Sub(q.epoch) - mt; b < q.base {
		q.base = b
	}
	deadline := q.epoch.Add(q.base + mt + playoutDelay)
	switch {
	case ext > q.highest:
		for s := q.highest + 1; s < ext; s++ {
			missingMT := mt - time.Duration(ext-s)*q.frame
			q.addPending(pendingFrame{seq: s, deadline: q.epoch.Add(q.base + missingMT + playoutDelay)})
		}
		q.highest, q.highestTS = ext, ts
		q.measureLatency(ts, arrival)
		if arrival.After(deadline) {
			q.conceal()
		}
	default:
		// An older packet: counted if it fills a gap late and the gap was
		// not concealed yet; a duplicate is ignored.
		for i, p := range q.pending {
			if p.seq != ext {
				continue
			}
			if !p.concealed && arrival.After(p.deadline) {
				q.conceal()
			}
			q.measureLatency(ts, arrival)
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			break
		}
	}
	q.checkPending(arrival)
}

// mediaTime is the time of ts from the stream's first timestamp.
func (q *AudioQuality) mediaTime(ts uint32) time.Duration {
	return time.Duration(float64(int32(ts-q.firstTS)) / q.clockRate * float64(time.Second))
}

func (q *AudioQuality) addPending(p pendingFrame) {
	if len(q.pending) == pendingWindow {
		if !q.pending[0].concealed {
			q.conceal()
		}
		q.pending = append(q.pending[:0], q.pending[1:]...)
	}
	q.pending = append(q.pending, p)
}

// checkPending conceals the missing frames whose deadline has passed, and
// forgets concealed ones too old to arrive.
func (q *AudioQuality) checkPending(now time.Time) {
	n := 0
	for _, p := range q.pending {
		if !p.concealed && now.After(p.deadline) {
			p.concealed = true
			q.conceal()
		}
		if p.concealed && q.highest-p.seq > pendingWindow {
			continue
		}
		q.pending[n] = p
		n++
	}
	q.pending = q.pending[:n]
}

func (q *AudioQuality) conceal() {
	q.concealed++
	q.periodConceal++
}

// restart ends the current run of the stream; the next packet starts a new
// one.
func (q *AudioQuality) restart() {
	q.expectedPrev += uint64(q.highest - q.firstSeq + 1)
	q.started = false
	q.pending = q.pending[:0]
}

func (q *AudioQuality) expected() uint64 {
	e := q.expectedPrev
	if q.started {
		e += uint64(q.highest - q.firstSeq + 1)
	}
	return e
}

// Snapshot returns the current values; resetPeriod starts a new period.
func (q *AudioQuality) Snapshot(resetPeriod bool) AudioQualitySnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.expected()
	s := AudioQualitySnapshot{
		Frame:           q.frame,
		Concealed:       q.concealed,
		Expected:        e,
		PeriodConcealed: q.periodConceal,
		PeriodExpected:  e - q.periodBase,
		Latency:         q.latency,
		LatencyLookups:  q.lookups,
	}
	if resetPeriod {
		q.periodBase = e
		q.periodConceal = 0
	}
	return s
}
