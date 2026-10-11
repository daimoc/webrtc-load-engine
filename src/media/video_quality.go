package media

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Freezes (research R2): a gap between frames longer than
// max(freezeFactor × the usual interval, minFreeze).
const (
	freezeFactor = 3
	minFreeze    = 150 * time.Millisecond
	// keyframeRequestInterval bounds the keyframe requests per stream (R8).
	keyframeRequestInterval = time.Second
)

// VideoQuality measures one received VP8 stream from its packet headers,
// without decoding (FR-007): frames, freezes, the height of the last
// keyframe, height switches and the need for a keyframe. Update is called
// by the stream's read loop, Snapshot by the collector.
type VideoQuality struct {
	stallTimeout time.Duration
	sendTimes    *SendTimes          // Nil: no latency
	observer     prometheus.Observer // Latency samples, seconds

	mu         sync.Mutex
	started    bool
	highestSeq uint16
	missing    int // Sequence numbers skipped since the last frame start and not received since
	lastTS     uint32
	lastFrame  time.Time
	interval   time.Duration // Usual frame interval (EWMA 1/16 of non-freeze intervals)

	frames     uint64
	freezes    uint64
	freezeTime time.Duration
	height     int
	heightTime map[int]time.Duration
	switches   uint64

	periodStart     time.Time // Frame before the period's first interval
	periodIntervals int

	needKeyframe bool
	lastRequest  time.Time

	latency LatencyBuckets
}

// VideoQualitySnapshot is a point-in-time copy of a VideoQuality. Counters
// are cumulative; Period values cover the time since the last reset.
type VideoQualitySnapshot struct {
	Frames     uint64
	Freezes    uint64
	FreezeTime time.Duration
	Height     int // Of the last keyframe; 0 before the first
	HeightTime map[int]time.Duration
	Switches   uint64
	LastFrame  time.Time

	PeriodIntervals int           // Frame intervals that ended in the period
	PeriodSpan      time.Duration // Their total duration

	Latency LatencyBuckets // Send-to-receive time of the frames found in the send times
}

// FrameRate returns the period's frames per second, or 0 without an
// interval.
func (s VideoQualitySnapshot) FrameRate() float64 {
	if s.PeriodIntervals == 0 || s.PeriodSpan <= 0 {
		return 0
	}
	return float64(s.PeriodIntervals) / s.PeriodSpan.Seconds()
}

// NewVideoQuality creates the measures of a stream; freezes are capped at
// stallTimeout (a longer silence is a stall, 004).
func NewVideoQuality(stallTimeout time.Duration) *VideoQuality {
	return &VideoQuality{stallTimeout: stallTimeout, heightTime: map[int]time.Duration{}}
}

// UpdatePacket records a received packet from its VP8 payload, and reports
// whether a keyframe request should be sent now (see NeedKeyframe).
func (q *VideoQuality) UpdatePacket(seq uint16, ts uint32, payload []byte, arrival time.Time) (requestKeyframe bool) {
	info, ok := parseVP8(payload)
	if !ok {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	q.updateLocked(seq, ts, info, arrival)
	return q.needKeyframeLocked(arrival)
}

// SetLatency makes the stream measure the latency of its frames from the
// scenario's send times, observing each sample (seconds) into o. Call it
// before the first Update.
func (q *VideoQuality) SetLatency(st *SendTimes, o prometheus.Observer) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sendTimes, q.observer = st, o
}

// Update records a received packet with its parsed VP8 header.
func (q *VideoQuality) Update(seq uint16, ts uint32, info vp8Info, arrival time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.updateLocked(seq, ts, info, arrival)
}

func (q *VideoQuality) updateLocked(seq uint16, ts uint32, info vp8Info, arrival time.Time) {
	if !q.started {
		q.started = true
		q.highestSeq = seq
		q.newFrame(ts, info, arrival)
		return
	}
	delta := seq - q.highestSeq
	if delta == 0 || delta >= 1<<15 {
		// Duplicate or older packet: a retransmission may fill a gap.
		if delta != 0 && q.missing > 0 {
			q.missing--
		}
		return
	}
	q.missing += int(delta) - 1
	q.highestSeq = seq
	if info.frameStart || ts != q.lastTS {
		q.newFrame(ts, info, arrival)
	}
}

func (q *VideoQuality) newFrame(ts uint32, info vp8Info, arrival time.Time) {
	if q.frames > 0 {
		gap := arrival.Sub(q.lastFrame)
		counted := gap
		if counted > q.stallTimeout {
			counted = q.stallTimeout
		}
		threshold := freezeFactor * q.interval
		if threshold < minFreeze {
			threshold = minFreeze
		}
		switch {
		case gap > threshold:
			q.freezes++
			q.freezeTime += counted
		case q.interval == 0:
			q.interval = gap
		default:
			q.interval += (gap - q.interval) / 16
		}
		if q.height > 0 {
			q.heightTime[q.height] += counted
		}
		if q.periodStart.IsZero() {
			q.periodStart = q.lastFrame
		}
		q.periodIntervals++
	}
	q.frames++
	q.lastTS = ts
	q.lastFrame = arrival
	if q.sendTimes != nil {
		if at, ok := q.sendTimes.Lookup(ts); ok {
			if d := arrival.Sub(at); d >= 0 {
				q.latency.Add(d)
				q.observer.Observe(d.Seconds())
			}
		}
	}

	switch {
	case info.keyframe:
		q.needKeyframe = false
		if info.height > 0 {
			if q.height > 0 && info.height != q.height {
				q.switches++
			}
			q.height = info.height
		}
	case q.frames == 1 || q.missing > 0:
		q.needKeyframe = true
	}
	q.missing = 0
}

// NeedKeyframe reports whether a keyframe request should be sent now: the
// stream cannot be decoded until the next keyframe, and no request was
// sent in the last second (research R8).
func (q *VideoQuality) NeedKeyframe(now time.Time) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.needKeyframeLocked(now)
}

func (q *VideoQuality) needKeyframeLocked(now time.Time) bool {
	if !q.needKeyframe || now.Sub(q.lastRequest) < keyframeRequestInterval {
		return false
	}
	q.lastRequest = now
	return true
}

// Snapshot returns the current values; resetPeriod starts a new period.
func (q *VideoQuality) Snapshot(resetPeriod bool) VideoQualitySnapshot {
	q.mu.Lock()
	defer q.mu.Unlock()
	s := VideoQualitySnapshot{
		Frames:          q.frames,
		Freezes:         q.freezes,
		FreezeTime:      q.freezeTime,
		Height:          q.height,
		HeightTime:      make(map[int]time.Duration, len(q.heightTime)),
		Switches:        q.switches,
		LastFrame:       q.lastFrame,
		PeriodIntervals: q.periodIntervals,
		Latency:         q.latency,
	}
	for h, d := range q.heightTime {
		s.HeightTime[h] = d
	}
	if q.periodIntervals > 0 {
		s.PeriodSpan = q.lastFrame.Sub(q.periodStart)
	}
	if resetPeriod {
		q.periodStart = time.Time{}
		q.periodIntervals = 0
	}
	return s
}
