package media

import (
	"testing"
	"time"
)

// audioFeed sends 20 ms Opus packets (960 ticks at 48 kHz) to an
// AudioQuality. Packet i is due at start + i × 20 ms on the fastest path.
type audioFeed struct {
	q     *AudioQuality
	start time.Time
	seq0  uint16
	ts0   uint32
}

func newAudioFeed() *audioFeed {
	return &audioFeed{q: NewAudioQuality(48000, 20*time.Millisecond), start: time.Unix(1000, 0), seq0: 65500, ts0: 4294900000}
}

// packet sends packet i with the given extra delay over the fastest path.
func (f *audioFeed) packet(i int, delay time.Duration) {
	f.q.Update(f.seq0+uint16(i), f.ts0+uint32(i)*960, f.start.Add(time.Duration(i)*20*time.Millisecond+delay))
}

func TestAudioQuality_Steady(t *testing.T) {
	f := newAudioFeed()
	for i := 0; i < 100; i++ { // Crosses the sequence and timestamp wrap
		f.packet(i, time.Duration(i%3)*time.Millisecond)
	}
	s := f.q.Snapshot(false)
	if s.Concealed != 0 || s.Expected != 100 {
		t.Fatalf("got %+v, want 0 concealed of 100", s)
	}
}

func TestAudioQuality_MissingCountedOnceAtDeadline(t *testing.T) {
	f := newAudioFeed()
	for i := 0; i < 10; i++ {
		f.packet(i, 0)
	}
	f.packet(11, 0) // 10 missing; its deadline is 10×20+60 ms
	if s := f.q.Snapshot(false); s.Concealed != 0 {
		t.Fatal("missing packet concealed before its deadline")
	}
	for i := 12; i < 20; i++ {
		f.packet(i, 0)
	}
	if s := f.q.Snapshot(false); s.Concealed != 1 || s.Expected != 20 {
		t.Fatalf("got %+v, want 1 concealed of 20", s)
	}
	f.packet(10, 300*time.Millisecond) // Arrives late: already counted
	if s := f.q.Snapshot(false); s.Concealed != 1 {
		t.Fatalf("late packet filling a gap counted twice: %d", s.Concealed)
	}
}

func TestAudioQuality_LateAndReordered(t *testing.T) {
	f := newAudioFeed()
	for i := 0; i < 10; i++ {
		f.packet(i, 0)
	}
	f.packet(10, 100*time.Millisecond) // In order but 100 ms late (> 60 ms)
	f.packet(12, 0)
	f.packet(11, 30*time.Millisecond) // Reordered within 60 ms: heard
	for i := 13; i < 30; i++ {
		f.packet(i, 0)
	}
	if s := f.q.Snapshot(false); s.Concealed != 1 || s.Expected != 30 {
		t.Fatalf("got %+v, want 1 concealed of 30", s)
	}
	f.packet(5, 0) // Duplicate
	if s := f.q.Snapshot(false); s.Concealed != 1 {
		t.Fatalf("duplicate counted: %+v", s)
	}
}

func TestAudioQuality_BaseIsFastestPath(t *testing.T) {
	f := newAudioFeed()
	// The first packets are 50 ms slow, then the path gets faster: the
	// base moves down and the slow start is not concealed.
	for i := 0; i < 5; i++ {
		f.packet(i, 50*time.Millisecond)
	}
	for i := 5; i < 20; i++ {
		f.packet(i, 0)
	}
	if s := f.q.Snapshot(false); s.Concealed != 0 {
		t.Fatalf("concealed %d", s.Concealed)
	}
}

func TestAudioQuality_Restart(t *testing.T) {
	f := newAudioFeed()
	for i := 0; i < 10; i++ {
		f.packet(i, 0)
	}
	// The sender restarts: sequence jumps by more than 1,000 and the new
	// stream arrives 500 ms "later" in media time terms. Nothing is
	// concealed and the expected count continues.
	f.seq0 += 5000
	f.ts0 += 48000 * 20
	f.start = f.start.Add(500 * time.Millisecond)
	for i := 10; i < 20; i++ {
		f.packet(i, 0)
	}
	if s := f.q.Snapshot(false); s.Concealed != 0 || s.Expected != 20 {
		t.Fatalf("got %+v, want 0 concealed of 20", s)
	}
}

func TestAudioQuality_Period(t *testing.T) {
	f := newAudioFeed()
	for i := 0; i < 50; i++ {
		if i != 20 {
			f.packet(i, 0)
		}
	}
	s := f.q.Snapshot(true)
	if s.PeriodConcealed != 1 || s.PeriodExpected != 50 {
		t.Fatalf("first period %+v", s)
	}
	for i := 50; i < 100; i++ {
		f.packet(i, 0)
	}
	s = f.q.Snapshot(true)
	if s.PeriodConcealed != 0 || s.PeriodExpected != 50 || s.Concealed != 1 || s.Expected != 100 {
		t.Fatalf("second period %+v", s)
	}
}

func TestAudioQuality_Latency(t *testing.T) {
	st := NewSendTimes()
	f := newAudioFeed()
	f.q.SetLatency(st, Latency.WithLabelValues("audio-latency-test", KindAudio))
	for i := 0; i < 20; i++ {
		if i != 5 { // Packet 5 has no known send time
			st.Record(f.ts0+uint32(i)*960, f.start.Add(time.Duration(i)*20*time.Millisecond-30*time.Millisecond))
		}
		if i != 10 {
			f.packet(i, 0)
		}
	}
	f.packet(10, 10*time.Millisecond) // Reordered: still measured
	f.packet(3, 0)                    // Duplicate: not measured again
	s := f.q.Snapshot(false)
	if s.Latency.N != 19 {
		t.Fatalf("latency samples %d, want 19", s.Latency.N)
	}
	if s.LatencyLookups != 20 {
		t.Errorf("latency lookups %d, want 20 (every packet once, found or not)", s.LatencyLookups)
	}
	if q, _ := s.Latency.Quantile(0.5); q != 30500*time.Microsecond {
		t.Fatalf("median %s, want 30.5ms", q)
	}
}
