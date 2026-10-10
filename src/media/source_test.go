package media

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakeClock jumps to each deadline at once and stops the source (cancels
// its context) at the first deadline after limit. onSleep runs before each
// jump.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	limit   time.Time
	cancel  context.CancelFunc
	onSleep func(due time.Time)
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) SleepUntil(ctx context.Context, t time.Time) bool {
	if c.onSleep != nil {
		c.onSleep(t)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if t.After(c.limit) {
		c.cancel()
		return false
	}
	if t.After(c.now) {
		c.now = t
	}
	return ctx.Err() == nil
}

// recorder is a TrackWriter that keeps copies of the packets and the
// (fake) time each was written at.
type recorder struct {
	mu      sync.Mutex
	clock   *fakeClock
	packets []rtp.Packet
	at      []time.Time
	err     error
}

func (r *recorder) WriteRTP(p *rtp.Packet) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	c := *p
	c.Payload = append([]byte(nil), p.Payload...)
	r.packets = append(r.packets, c)
	r.at = append(r.at, r.clock.Now())
	return nil
}

func (r *recorder) snapshot() ([]rtp.Packet, []time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]rtp.Packet(nil), r.packets...), append([]time.Time(nil), r.at...)
}

// runSource runs a source over the content for the given (fake) duration
// with the given writers subscribed.
func runSource(t *testing.T, scenarioID string, c *Content, d time.Duration, onSleep func(*Source, time.Time), writers ...*recorder) *Source {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	start := time.Unix(1000, 0)
	clock := &fakeClock{now: start, limit: start.Add(d), cancel: cancel}
	s := NewSource(scenarioID, c)
	s.clock = clock
	if onSleep != nil {
		clock.onSleep = func(due time.Time) { onSleep(s, due) }
	}
	for _, w := range writers {
		w.clock = clock
		s.Subscribe(w)
	}
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("source did not stop")
	}
	return s
}

// testContent is a 3-frame, 100 ms loop of tiny frames, with a frame
// larger than the MTU to get several packets.
func testContent() *Content {
	big := make([]byte, 3000)
	for i := range big {
		big[i] = byte(i)
	}
	return &Content{
		Kind:         KindVideo,
		Frames:       []Frame{{Data: []byte{0x10, 1, 2}, Duration: 30 * time.Millisecond, Samples: 2700}, {Data: big, Duration: 30 * time.Millisecond, Samples: 2700}, {Data: []byte{0x11, 3}, Duration: 40 * time.Millisecond, Samples: 3600}},
		LoopDuration: 100 * time.Millisecond,
	}
}

func TestSource_SchedulesFramesAcrossLoops(t *testing.T) {
	w := &recorder{}
	runSource(t, "sched", testContent(), 250*time.Millisecond, nil, w)
	packets, at := w.snapshot()

	// Frame start times: 0, 30, 60 ms, then +100 ms per loop, up to 250 ms.
	var frameTimes []time.Duration
	var timestamps []uint32
	for i, p := range packets {
		if i == 0 || packets[i-1].Marker {
			frameTimes = append(frameTimes, at[i].Sub(time.Unix(1000, 0)))
			timestamps = append(timestamps, p.Timestamp)
		}
	}
	wantTimes := []time.Duration{0, 30, 60, 100, 130, 160, 200, 230}
	if len(frameTimes) != len(wantTimes) {
		t.Fatalf("expected %d frames, got %d (%v)", len(wantTimes), len(frameTimes), frameTimes)
	}
	for i := range wantTimes {
		if frameTimes[i] != wantTimes[i]*time.Millisecond {
			t.Errorf("frame %d written at %s, want %s", i, frameTimes[i], wantTimes[i]*time.Millisecond)
		}
	}
	// Timestamps advance by each frame's samples, across the loop boundary.
	wantTS := []uint32{0, 2700, 5400, 9000, 11700, 14400, 18000, 20700}
	for i := range wantTS {
		if timestamps[i]-timestamps[0] != wantTS[i] {
			t.Errorf("frame %d timestamp +%d, want +%d", i, timestamps[i]-timestamps[0], wantTS[i])
		}
	}
	for i := 1; i < len(packets); i++ {
		if packets[i].SequenceNumber != packets[i-1].SequenceNumber+1 {
			t.Fatalf("sequence gap at packet %d: %d after %d", i, packets[i].SequenceNumber, packets[i-1].SequenceNumber)
		}
	}
	// The large frame is split into several packets, the last one marked.
	if len(packets) <= len(frameTimes) {
		t.Fatalf("expected the 3000-byte frame to span several packets, got %d packets for %d frames", len(packets), len(frameTimes))
	}
}

func TestSource_IdenticalPacketsForEveryTrack(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	id := "fanout-" + t.Name() + time.Now().Format("150405.000000000")
	runSource(t, id, testContent(), 150*time.Millisecond, nil, a, b)
	pa, _ := a.snapshot()
	pb, _ := b.snapshot()
	if len(pa) == 0 || len(pa) != len(pb) {
		t.Fatalf("expected the same number of packets, got %d and %d", len(pa), len(pb))
	}
	for i := range pa {
		if pa[i].SequenceNumber != pb[i].SequenceNumber || pa[i].Timestamp != pb[i].Timestamp || string(pa[i].Payload) != string(pb[i].Payload) {
			t.Fatalf("packet %d differs between tracks", i)
		}
	}
	if got := testutil.ToFloat64(PacketsSent.WithLabelValues(id, KindVideo)); int(got) != 2*len(pa) {
		t.Errorf("expected %d packets counted as sent, got %v", 2*len(pa), got)
	}
}

func TestSource_UnsubscribeStopsWrites(t *testing.T) {
	a, b := &recorder{}, &recorder{}
	unsubscribeAt := time.Unix(1000, 0).Add(100 * time.Millisecond)
	runSource(t, "unsub", testContent(), 250*time.Millisecond, func(s *Source, due time.Time) {
		if due.Equal(unsubscribeAt) {
			s.Unsubscribe(b)
		}
	}, a, b)
	_, atA := a.snapshot()
	_, atB := b.snapshot()
	if len(atB) == 0 || len(atB) >= len(atA) {
		t.Fatalf("expected b to get fewer packets than a, got %d and %d", len(atB), len(atA))
	}
	for _, at := range atB {
		if !at.Before(unsubscribeAt) {
			t.Fatalf("b written at %s after unsubscribing", at)
		}
	}
}

func TestSource_WriterErrorDoesNotStopOthers(t *testing.T) {
	failing := &recorder{err: errors.New("closed")}
	ok := &recorder{}
	runSource(t, "errs", testContent(), 150*time.Millisecond, nil, failing, ok)
	if p, _ := ok.snapshot(); len(p) == 0 {
		t.Fatal("healthy track got nothing")
	}
}

func TestSource_RunStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewSource("cancel", testContent())
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}
