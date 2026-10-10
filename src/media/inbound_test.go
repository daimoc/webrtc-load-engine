package media

import (
	"math"
	"testing"
	"time"
)

func feed(s *InboundStats, start time.Time, seqs []uint16) {
	for i, seq := range seqs {
		s.Update(seq, uint32(seq)*960, 100, start.Add(time.Duration(i)*20*time.Millisecond))
	}
}

func TestInboundStats_LossFromSequenceGaps(t *testing.T) {
	start := time.Unix(0, 0)
	cases := map[string]struct {
		seqs []uint16
		lost int64
	}{
		"in order":        {[]uint16{1, 2, 3, 4, 5}, 0},
		"gap of 3":        {[]uint16{1, 2, 6, 7}, 3},
		"reordered":       {[]uint16{1, 3, 2, 4}, 0},
		"duplicate":       {[]uint16{1, 2, 2, 3}, 0},
		"wrap-around":     {[]uint16{65534, 65535, 0, 1}, 0},
		"gap across wrap": {[]uint16{65534, 1, 2}, 2},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			s := NewInboundStats(48000)
			feed(s, start, tc.seqs)
			snap := s.Snapshot()
			if snap.Lost != tc.lost {
				t.Errorf("expected %d lost, got %d", tc.lost, snap.Lost)
			}
			if snap.Packets != uint64(len(tc.seqs)) || snap.Bytes != uint64(100*len(tc.seqs)) {
				t.Errorf("expected %d packets / %d bytes, got %d / %d", len(tc.seqs), 100*len(tc.seqs), snap.Packets, snap.Bytes)
			}
		})
	}
}

func TestInboundStats_Jitter(t *testing.T) {
	// Packets sent every 20 ms (960 ticks at 48 kHz) and arriving exactly on
	// time: no jitter.
	s := NewInboundStats(48000)
	feed(s, time.Unix(0, 0), []uint16{1, 2, 3, 4, 5, 6})
	if j := s.Snapshot().Jitter; j != 0 {
		t.Fatalf("expected no jitter, got %v", j)
	}

	// Arrivals alternate 10 ms early / late: |D| = 20 ms after the first
	// pair, and J converges to it (RFC 3550 A.8: J += (|D| - J) / 16).
	s = NewInboundStats(48000)
	base := time.Unix(0, 0)
	var want float64
	for i := 0; i < 200; i++ {
		offset := 10 * time.Millisecond
		if i%2 == 1 {
			offset = -offset
		}
		s.Update(uint16(i), uint32(i)*960, 100, base.Add(time.Duration(i)*20*time.Millisecond+offset))
		if i > 0 {
			want += (0.020 - want) / 16
		}
	}
	if got := s.Snapshot().Jitter; math.Abs(got-want) > 1e-6 {
		t.Fatalf("expected jitter %.6f s, got %.6f s", want, got)
	}
}

func TestReportRoundTrip(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	sent := now.Add(-150 * time.Millisecond)
	lsr := compactNTP(sent)
	dlsr := uint32(65536 / 10) // The remote held the report 100 ms
	rtt, ok := ReportRoundTrip(now, lsr, dlsr)
	if !ok {
		t.Fatal("expected a round-trip sample")
	}
	if d := rtt - 50*time.Millisecond; d < -time.Millisecond || d > time.Millisecond {
		t.Fatalf("expected ~50 ms, got %s", rtt)
	}
	if _, ok := ReportRoundTrip(now, 0, dlsr); ok {
		t.Fatal("LSR 0 (no sender report received yet) must give no sample")
	}
}
