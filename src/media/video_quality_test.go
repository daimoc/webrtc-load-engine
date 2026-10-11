package media

import (
	"testing"
	"time"
)

// videoFeed sends frames of 3 packets to a VideoQuality at 30 fps.
type videoFeed struct {
	q   *VideoQuality
	seq uint16
	ts  uint32
	now time.Time
}

func newVideoFeed(stall time.Duration) *videoFeed {
	return &videoFeed{q: NewVideoQuality(stall), seq: 100, ts: 9000, now: time.Unix(1000, 0)}
}

// frame sends one frame of the given height (0: interframe) after gap.
func (f *videoFeed) frame(gap time.Duration, height int) {
	f.now = f.now.Add(gap)
	f.ts += 3000
	first := vp8Info{frameStart: true}
	if height > 0 {
		first = vp8Info{frameStart: true, keyframe: true, width: height * 16 / 9, height: height}
	}
	f.q.Update(f.seq, f.ts, first, f.now)
	f.q.Update(f.seq+1, f.ts, vp8Info{}, f.now.Add(time.Millisecond))
	f.q.Update(f.seq+2, f.ts, vp8Info{}, f.now.Add(2*time.Millisecond))
	f.seq += 3
}

// frames sends n interframes 33 ms apart.
func (f *videoFeed) frames(n int) {
	for i := 0; i < n; i++ {
		f.frame(33*time.Millisecond, 0)
	}
}

const frameGap = 33 * time.Millisecond

func TestVideoQuality_CountsFramesOnce(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frame(0, 360)
	f.frames(29)
	// Duplicate of the last frame's first packet and a reordered packet of
	// an earlier frame: no new frame.
	f.q.Update(f.seq-3, f.ts, vp8Info{frameStart: true}, f.now.Add(5*time.Millisecond))
	f.q.Update(f.seq-10, f.ts-6000, vp8Info{frameStart: true}, f.now.Add(6*time.Millisecond))
	s := f.q.Snapshot(false)
	if s.Frames != 30 {
		t.Fatalf("frames %d, want 30", s.Frames)
	}
	// A frame whose first packet is lost starts at its first received
	// packet (new timestamp).
	f.now = f.now.Add(frameGap)
	f.ts += 3000
	f.q.Update(f.seq+1, f.ts, vp8Info{}, f.now)
	if s := f.q.Snapshot(false); s.Frames != 31 {
		t.Fatalf("frames %d after a frame without its first packet, want 31", s.Frames)
	}
}

func TestVideoQuality_Freezes(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frame(0, 360)
	f.frames(30)
	f.frame(120*time.Millisecond, 0) // Below 150 ms: not a freeze
	if s := f.q.Snapshot(false); s.Freezes != 0 {
		t.Fatalf("120 ms gap counted as a freeze")
	}
	f.frames(30)

	// A 1 s gap is counted when the next frame arrives.
	f.now = f.now.Add(time.Second)
	if s := f.q.Snapshot(false); s.Freezes != 0 {
		t.Fatal("freeze counted before the stream resumed")
	}
	f.now = f.now.Add(-time.Second)
	f.frame(time.Second, 0)
	s := f.q.Snapshot(false)
	if s.Freezes != 1 || s.FreezeTime != time.Second {
		t.Fatalf("got %d freezes, %s; want 1, 1s", s.Freezes, s.FreezeTime)
	}

	// The usual interval is not moved by the freeze: a 200 ms gap after
	// 30 more frames is a freeze again.
	f.frames(30)
	f.frame(200*time.Millisecond, 0)
	if s := f.q.Snapshot(false); s.Freezes != 2 {
		t.Fatalf("freezes %d after a 200 ms gap, want 2", s.Freezes)
	}

	// A gap longer than the stall timeout counts as a freeze of the stall
	// timeout.
	f.frames(30)
	f.frame(30*time.Second, 0)
	if s := f.q.Snapshot(false); s.Freezes != 3 || s.FreezeTime != time.Second+200*time.Millisecond+10*time.Second {
		t.Fatalf("got %d freezes, %s after a 30 s gap", s.Freezes, s.FreezeTime)
	}
}

func TestVideoQuality_StreamEndIsNotAFreeze(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frame(0, 360)
	f.frames(30)
	f.now = f.now.Add(time.Minute)
	if s := f.q.Snapshot(false); s.Freezes != 0 || s.FreezeTime != 0 {
		t.Fatalf("stream end counted: %+v", s)
	}
}

func TestVideoQuality_HeightAndSwitches(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frames(3) // Before the first keyframe: height 0
	if s := f.q.Snapshot(false); s.Height != 0 {
		t.Fatalf("height %d before a keyframe", s.Height)
	}
	f.frame(frameGap, 360)
	f.frames(30)
	f.frame(frameGap, 180)
	f.frames(30)
	f.frame(frameGap, 180) // Same height: no switch
	f.frame(frameGap, 360)
	s := f.q.Snapshot(false)
	if s.Height != 360 || s.Switches != 2 {
		t.Fatalf("height %d, switches %d; want 360, 2", s.Height, s.Switches)
	}
	if got := s.HeightTime[360]; got != 31*frameGap {
		t.Fatalf("time at 360: %s, want %s", got, 31*frameGap)
	}
	if got := s.HeightTime[180]; got != 32*frameGap {
		t.Fatalf("time at 180: %s, want %s", got, 32*frameGap)
	}
}

func TestVideoQuality_NeedKeyframe(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frame(0, 0) // First frame is not a keyframe
	if !f.q.NeedKeyframe(f.now) {
		t.Fatal("no keyframe request for a stream starting on an interframe")
	}
	f.frames(5)
	if f.q.NeedKeyframe(f.now) {
		t.Fatal("second request within 1 s")
	}
	f.frames(30)
	if !f.q.NeedKeyframe(f.now) {
		t.Fatal("no new request after 1 s without a keyframe")
	}
	f.frame(frameGap, 360)
	f.now = f.now.Add(2 * time.Second)
	if f.q.NeedKeyframe(f.now) {
		t.Fatal("request after a keyframe")
	}

	// A lost packet not repaired before the next frame.
	f.frames(30)
	f.seq++ // Lose one packet
	f.frames(1)
	if !f.q.NeedKeyframe(f.now) {
		t.Fatal("no request after an unrepaired gap")
	}
	f.frame(frameGap, 360)

	// A lost packet repaired (retransmitted) before the next frame.
	f.frames(30)
	f.now = f.now.Add(frameGap)
	f.ts += 3000
	f.q.Update(f.seq, f.ts, vp8Info{frameStart: true}, f.now)
	f.q.Update(f.seq+2, f.ts, vp8Info{}, f.now.Add(time.Millisecond))
	f.q.Update(f.seq+1, f.ts, vp8Info{}, f.now.Add(5*time.Millisecond)) // Retransmission
	f.seq += 3
	f.frames(1)
	f.now = f.now.Add(2 * time.Second)
	if f.q.NeedKeyframe(f.now) {
		t.Fatal("request after a repaired gap")
	}
}

func TestVideoQuality_PeriodFrameRate(t *testing.T) {
	f := newVideoFeed(10 * time.Second)
	f.frame(0, 360)
	f.frames(60)
	s := f.q.Snapshot(true)
	if s.PeriodIntervals != 60 || s.PeriodSpan != 60*frameGap {
		t.Fatalf("period: %d intervals over %s", s.PeriodIntervals, s.PeriodSpan)
	}
	if s := f.q.Snapshot(false); s.PeriodIntervals != 0 || s.Frames != 61 {
		t.Fatalf("after reset: %+v", s)
	}
	// The next period starts at the last frame of the previous one.
	f.frames(10)
	if s := f.q.Snapshot(true); s.PeriodIntervals != 10 || s.PeriodSpan != 10*frameGap {
		t.Fatalf("second period: %d intervals over %s", s.PeriodIntervals, s.PeriodSpan)
	}
}

func TestVideoQuality_Latency(t *testing.T) {
	st := NewSendTimes()
	obs := Latency.WithLabelValues("video-latency-test", KindVideo)
	f := newVideoFeed(10 * time.Second)
	f.q.SetLatency(st, obs)
	// Frames 1 to 10 were sent 40 ms before they arrive; frame 11 is
	// unknown (another sender) and frame 12 "arrives" before it was sent.
	for i := 1; i <= 12; i++ {
		ts := f.ts + 3000
		at := f.now.Add(frameGap - 40*time.Millisecond)
		switch i {
		case 11:
		case 12:
			st.Record(ts, f.now.Add(time.Second))
		default:
			st.Record(ts, at)
		}
		f.frame(frameGap, 0)
	}
	s := f.q.Snapshot(false)
	if s.Latency.N != 10 {
		t.Fatalf("latency samples %d, want 10", s.Latency.N)
	}
	if q, _ := s.Latency.Quantile(0.5); q != 40500*time.Microsecond {
		t.Fatalf("median %s, want 40.5ms", q)
	}
}
