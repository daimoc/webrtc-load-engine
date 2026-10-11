package media

import (
	"sync"
	"time"
)

// sendTimesSize is the number of frames a SendTimes remembers: about 10 s
// of audio and 17 s of video (research R5).
const sendTimesSize = 512

// SendTimes records when the scenario's source started writing each frame,
// by RTP timestamp, so that receivers can tell how long a frame took to
// reach them (research R5): every bot sends the same frames with the same
// timestamps at the same time. One per scenario and kind.
type SendTimes struct {
	mu    sync.RWMutex
	at    map[uint32]time.Time
	order [sendTimesSize]uint32 // Timestamps in recording order, to forget the oldest
	n     int                   // Frames recorded
}

// NewSendTimes creates an empty registry.
func NewSendTimes() *SendTimes {
	return &SendTimes{at: make(map[uint32]time.Time, sendTimesSize)}
}

// Record records that the frame with RTP timestamp ts was sent at at.
func (s *SendTimes) Record(ts uint32, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	slot := s.n % sendTimesSize
	if s.n >= sendTimesSize {
		delete(s.at, s.order[slot])
	}
	s.order[slot] = ts
	s.at[ts] = at
	s.n++
}

// Lookup returns the send time of the frame with timestamp ts, or false
// when it is not one of the recorded frames (never a guess).
func (s *SendTimes) Lookup(ts uint32) (time.Time, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	at, ok := s.at[ts]
	return at, ok
}

// Latency buckets of a stream: 1 ms wide up to latencyFine, then 10 ms wide
// up to latencyMax, then one overflow bucket.
const (
	latencyFine       = 250 * time.Millisecond
	latencyMax        = 2 * time.Second
	latencyFineBucket = time.Millisecond
	latencyWideBucket = 10 * time.Millisecond
	latencyBuckets    = int(latencyFine/latencyFineBucket) + int((latencyMax-latencyFine)/latencyWideBucket) + 1
)

// LatencyBuckets counts latency samples finely enough for a median within
// 1 ms below 250 ms (SC-004). The zero value is empty.
type LatencyBuckets struct {
	Counts [latencyBuckets]uint32
	N      uint64
	Max    time.Duration
}

func latencyBucket(d time.Duration) int {
	if d < latencyFine {
		return int(d / latencyFineBucket)
	}
	if d < latencyMax {
		return int(latencyFine/latencyFineBucket) + int((d-latencyFine)/latencyWideBucket)
	}
	return latencyBuckets - 1
}

// Add counts one sample; negative samples are ignored.
func (b *LatencyBuckets) Add(d time.Duration) {
	if d < 0 {
		return
	}
	b.Counts[latencyBucket(d)]++
	b.N++
	if d > b.Max {
		b.Max = d
	}
}

// Merge adds the samples of o.
func (b *LatencyBuckets) Merge(o *LatencyBuckets) {
	if o.N == 0 {
		return
	}
	for i, c := range o.Counts {
		b.Counts[i] += c
	}
	b.N += o.N
	if o.Max > b.Max {
		b.Max = o.Max
	}
}

// Quantile returns the q-quantile (0 < q ≤ 1) as the middle of its bucket
// (the largest sample for the overflow bucket), or false without samples.
func (b *LatencyBuckets) Quantile(q float64) (time.Duration, bool) {
	if b.N == 0 {
		return 0, false
	}
	rank := uint64(q*float64(b.N) + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > b.N {
		rank = b.N
	}
	var seen uint64
	for i, c := range b.Counts {
		seen += uint64(c)
		if seen < rank {
			continue
		}
		switch {
		case i == latencyBuckets-1:
			return b.Max, true
		case i < int(latencyFine/latencyFineBucket):
			return time.Duration(i)*latencyFineBucket + latencyFineBucket/2, true
		default:
			j := i - int(latencyFine/latencyFineBucket)
			return latencyFine + time.Duration(j)*latencyWideBucket + latencyWideBucket/2, true
		}
	}
	return b.Max, true
}
