package media

import (
	"sync"
	"time"
)

// InboundStats accounts for one received RTP stream: packets, bytes, losses
// from sequence gaps (RFC 3550 A.1) and interarrival jitter (RFC 3550 A.8).
// Update is called by the stream's read loop, Snapshot by the collector.
type InboundStats struct {
	clockRate float64

	mu          sync.Mutex
	packets     uint64
	bytes       uint64
	started     bool
	baseSeq     uint16
	maxSeq      uint16
	cycles      uint64
	lastArrival time.Time
	lastTS      uint32
	jitter      float64 // In RTP timestamp units
}

// InboundSnapshot is a point-in-time copy of an InboundStats.
type InboundSnapshot struct {
	Packets     uint64
	Bytes       uint64
	Lost        int64   // Expected minus received, never negative
	Jitter      float64 // Seconds
	LastArrival time.Time
}

// NewInboundStats creates the stats of a stream with the given RTP clock rate.
func NewInboundStats(clockRate uint32) *InboundStats {
	return &InboundStats{clockRate: float64(clockRate)}
}

// Update records a received packet.
func (s *InboundStats) Update(seq uint16, timestamp uint32, payloadLen int, arrival time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.packets++
	s.bytes += uint64(payloadLen)
	if !s.started {
		s.started = true
		s.baseSeq, s.maxSeq = seq, seq
		s.lastArrival, s.lastTS = arrival, timestamp
		return
	}

	// Sequence: a forward step of less than half the space is in order
	// (possibly after a gap); anything else is a reordered or duplicate
	// packet and does not move the highest sequence number.
	if delta := seq - s.maxSeq; delta != 0 && delta < 1<<15 {
		if seq < s.maxSeq {
			s.cycles++
		}
		s.maxSeq = seq
	}

	// Jitter: D = (Rj - Ri) - (Sj - Si) in timestamp units.
	arrivalTicks := arrival.Sub(s.lastArrival).Seconds() * s.clockRate
	d := arrivalTicks - float64(int32(timestamp-s.lastTS))
	if d < 0 {
		d = -d
	}
	s.jitter += (d - s.jitter) / 16
	s.lastArrival, s.lastTS = arrival, timestamp
}

// Snapshot returns the current values.
func (s *InboundStats) Snapshot() InboundSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := InboundSnapshot{Packets: s.packets, Bytes: s.bytes, LastArrival: s.lastArrival}
	if s.started {
		expected := int64(s.cycles<<16) + int64(s.maxSeq) - int64(s.baseSeq) + 1
		if lost := expected - int64(s.packets); lost > 0 {
			snap.Lost = lost
		}
		snap.Jitter = s.jitter / s.clockRate
	}
	return snap
}
