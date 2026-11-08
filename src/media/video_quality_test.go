package media

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/prometheus/client_golang/prometheus"
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

// trailerFeed sends frames of 3 packets, the last with a frame trailer, as a
// server that rewrites timestamps would forward them.
type trailerFeed struct {
	t   *testing.T
	q   *VideoQuality
	st  *SendTimes
	tag [3]byte
	seq uint16
	ts  uint32
	now time.Time
}

func newTrailerFeed(t *testing.T) *trailerFeed {
	tag := [3]byte{0xa1, 0xb2, 0xc3}
	f := &trailerFeed{t: t, q: NewVideoQuality(10 * time.Second), st: NewVideoSendTimes(tag), tag: tag, seq: 100, ts: 3001, now: time.Unix(1000, 0)}
	f.q.SetLatency(f.st, Latency.WithLabelValues("video-latency-test", KindVideo))
	return f
}

func vp8Packet(first bool) []byte {
	if first {
		return []byte{0x10, 0x01, 7, 7, 7, 7, 7, 7, 7}
	}
	return []byte{0x00, 0x01, 7, 7, 7, 7, 7, 7, 7}
}

// start sends a frame's first packets, gap after the previous frame.
func (f *trailerFeed) start(gap time.Duration) {
	f.now = f.now.Add(gap)
	f.ts += 3000
	f.q.UpdatePacket(f.seq, f.ts, vp8Packet(true), f.now)
	f.q.UpdatePacket(f.seq+1, f.ts, vp8Packet(false), f.now.Add(time.Millisecond))
	f.seq += 2
}

// finish sends the frame's last packet with the trailer of frame n.
func (f *trailerFeed) finish(n uint32) {
	f.q.UpdatePacket(f.seq, f.ts, appendFrameTrailer(vp8Packet(false), f.tag, n), f.now.Add(2*time.Millisecond))
	f.seq++
}

// frame sends frame n, sent sendAge before its first packet arrives.
func (f *trailerFeed) frame(n uint32, sendAge time.Duration) {
	f.start(frameGap)
	f.st.Record(n, f.now.Add(-sendAge))
	f.finish(n)
}

func (f *trailerFeed) samples() uint64 { return f.q.Snapshot(false).Latency.N }

// Latency runs to the arrival of the frame's first packet, though the
// trailer comes in its last one.
func TestVideoQuality_Latency(t *testing.T) {
	f := newTrailerFeed(t)
	for n := uint32(1); n <= 10; n++ {
		f.frame(n, 40*time.Millisecond)
	}
	s := f.q.Snapshot(false)
	if s.Latency.N != 10 {
		t.Fatalf("latency samples %d, want 10", s.Latency.N)
	}
	if q, _ := s.Latency.Quantile(0.5); q != 40500*time.Microsecond {
		t.Fatalf("median %s, want 40.5ms", q)
	}
}

// Frames with no sample still count as looked up: the summary's reason
// depends on it.
func TestVideoQuality_LatencyLookupsEveryFrame(t *testing.T) {
	f := newTrailerFeed(t)
	f.frame(1, 40*time.Millisecond)
	f.start(frameGap)
	f.finish(2) // Unknown: another sender, or forgotten
	f.start(frameGap)
	f.st.Record(3, f.now.Add(time.Second)) // "Arrives" before it was sent
	f.finish(3)
	f.start(frameGap) // Last packet lost
	s := f.q.Snapshot(false)
	if s.Latency.N != 1 {
		t.Errorf("latency samples %d, want 1", s.Latency.N)
	}
	if s.LatencyLookups != 4 {
		t.Errorf("latency lookups %d, want 4 (every frame, found or not)", s.LatencyLookups)
	}
}

func TestVideoQuality_LatencyCountsRetransmissionOnce(t *testing.T) {
	f := newTrailerFeed(t)
	f.frame(1, 40*time.Millisecond)
	f.seq-- // The last packet again
	f.finish(1)
	if n := f.samples(); n != 1 {
		t.Fatalf("latency samples %d, want 1", n)
	}
}

// A last packet that arrives after the next frame started cannot borrow
// that frame's arrival.
func TestVideoQuality_LatencyIgnoresReorderedLastPacket(t *testing.T) {
	f := newTrailerFeed(t)
	f.frame(1, 40*time.Millisecond)
	f.start(frameGap)
	f.st.Record(2, f.now.Add(-40*time.Millisecond))
	oldTS, oldSeq := f.ts, f.seq
	f.start(frameGap)
	f.q.UpdatePacket(oldSeq, oldTS, appendFrameTrailer(vp8Packet(false), f.tag, 2), f.now.Add(2*time.Millisecond))
	if n := f.samples(); n != 1 {
		t.Fatalf("latency samples %d, want 1", n)
	}
}

func TestVideoQuality_LatencyIgnoresWrongTag(t *testing.T) {
	f := newTrailerFeed(t)
	f.st.Record(1, f.now)
	f.tag[2]++
	f.frame(1, 40*time.Millisecond)
	if n := f.samples(); n != 0 {
		t.Fatalf("latency samples %d, want 0", n)
	}
}

// Frame numbers forgotten by the send times give no sample.
func TestVideoQuality_LatencyIgnoresForgottenFrame(t *testing.T) {
	f := newTrailerFeed(t)
	for n := uint32(1); n <= sendTimesSize+1; n++ {
		f.st.Record(n, f.now)
	}
	f.start(frameGap)
	f.finish(1)
	if n := f.samples(); n != 0 {
		t.Fatalf("latency samples %d, want 0", n)
	}
}

// The picture sent again in a later content loop matches only its own frame
// number, never the earlier send time.
func TestVideoQuality_LatencyAcrossContentLoop(t *testing.T) {
	f := newTrailerFeed(t)
	f.start(frameGap)
	f.st.Record(1, f.now.Add(-10*time.Second)) // The same picture, one loop ago
	f.st.Record(301, f.now.Add(-40*time.Millisecond))
	f.finish(301)
	s := f.q.Snapshot(false)
	if q, _ := s.Latency.Quantile(0.5); s.Latency.N != 1 || q != 40500*time.Microsecond {
		t.Fatalf("%d samples, median %s, want 1 at 40.5ms", s.Latency.N, q)
	}
}

// 012 FR-004: at a simulcast layer switch timestamps jump, while frame
// numbers go on: both layers give samples, with no outlier at the switch.
func TestVideoQuality_LatencyAcrossLayerSwitch(t *testing.T) {
	f := newTrailerFeed(t)
	for n := uint32(1); n <= 10; n++ {
		f.frame(n, 40*time.Millisecond)
	}
	f.ts += 1 << 20
	f.seq += 500
	for n := uint32(11); n <= 20; n++ {
		f.frame(n, 40*time.Millisecond)
	}
	s := f.q.Snapshot(false)
	if s.Latency.N != 20 {
		t.Fatalf("latency samples %d, want 20", s.Latency.N)
	}
	if s.Latency.Max != 40*time.Millisecond {
		t.Fatalf("largest sample %s, want 40ms", s.Latency.Max)
	}
}

type countingObserver struct{ n int }

func (o *countingObserver) Observe(float64) { o.n++ }

// A frame that seems to arrive before it was sent reaches neither the
// buckets nor the Prometheus histogram.
func TestVideoQuality_LatencyIgnoresFrameBeforeItsSendTime(t *testing.T) {
	f := newTrailerFeed(t)
	o := &countingObserver{}
	f.q.SetLatency(f.st, o)
	f.frame(1, -time.Second)
	if n := f.samples(); n != 0 || o.n != 0 {
		t.Fatalf("%d samples, %d observed, want 0", n, o.n)
	}
}

// Every codec's stream is measured as VP8's (specs/014 FR-007): frames,
// height, freezes, no keyframe request on a clean stream.
func TestVideoQuality_EveryCodec(t *testing.T) {
	for _, c := range VideoCodecs() {
		ddID := uint8(0)
		if c == CodecAV1 {
			ddID = SourceDDExtID
		}
		frames := sourcePacketsFor(t, c, 2*time.Second)
		q := NewVideoQualityFor(c, ddID, 10*time.Second)
		at := time.Unix(1000, 0)
		requests := 0
		for i, f := range frames {
			// 500 ms without frames 20 to 34: one freeze. They are not
			// lost (same sequence numbers skipped), only late.
			if i >= 20 && i < 35 {
				continue
			}
			at = time.Unix(1000, 0).Add(time.Duration(i) * time.Second / 30)
			for j := range f {
				if q.UpdateRTP(&f[j].Header, f[j].Payload, at) {
					requests++
				}
			}
		}
		s := q.Snapshot(false)
		want := uint64(len(frames) - 15)
		if s.Frames != want {
			t.Errorf("%s: %d frames, want %d", c, s.Frames, want)
		}
		if s.Height != 360 {
			t.Errorf("%s: height %d, want 360", c, s.Height)
		}
		if s.Freezes != 1 {
			t.Errorf("%s: %d freezes, want 1", c, s.Freezes)
		}
		// Frames 20-34 were skipped with their sequence numbers: they count
		// as lost, so one keyframe request (then frame 30's keyframe... was
		// skipped too) is expected; never more than one per second.
		if requests > 2 {
			t.Errorf("%s: %d keyframe requests", c, requests)
		}
	}
}

// Latency comes from each codec's trailer carrier (specs/014 FR-007, R9).
func TestVideoQuality_LatencyEveryCodec(t *testing.T) {
	for _, c := range VideoCodecs() {
		w := &recorder{}
		s := runSource(t, "latency-"+string(c), loadSingle(t, c), time.Second, nil, w)
		packets, at := w.snapshot()
		ddID := uint8(0)
		if c == CodecAV1 {
			ddID = SourceDDExtID
		}
		q := NewVideoQualityFor(c, ddID, 10*time.Second)
		q.SetLatency(s.SendTimes(), prometheus.NewHistogram(prometheus.HistogramOpts{Name: "x"}))
		for i := range packets {
			q.UpdateRTP(&packets[i].Header, packets[i].Payload, at[i].Add(20*time.Millisecond))
		}
		snap := q.Snapshot(false)
		if snap.Latency.N < 25 {
			t.Errorf("%s: %d latency samples over %d frames", c, snap.Latency.N, snap.Frames)
			continue
		}
		if p50, _ := snap.Latency.Quantile(0.5); p50 < 15*time.Millisecond || p50 > 25*time.Millisecond {
			t.Errorf("%s: median latency %s, want 20 ms", c, p50)
		}
	}
}

// spatialLayer returns the spatial layer of a scalable source packet, read
// with the codecs' own fields: VP9's payload descriptor, AV1's descriptor.
func spatialLayer(t *testing.T, c VideoCodec, p *rtp.Packet, structure **DDStructure) int {
	t.Helper()
	if c == CodecVP9 {
		var pkt codecs.VP9Packet
		if _, err := pkt.Unmarshal(p.Payload); err != nil {
			t.Fatal(err)
		}
		return int(pkt.SID)
	}
	d, err := ParseDD(p.Header.GetExtension(SourceDDExtID), *structure)
	if err != nil {
		t.Fatal(err)
	}
	if d.Structure != nil {
		*structure = d.Structure
	}
	return d.Layer(*structure).SpatialID
}

// On a scalable stream the picture's height is its highest spatial layer's
// (014 T050): 720 with every layer; when the server stops forwarding the
// top layer the height drops to 360, one switch, no freeze, the frame rate
// unchanged; latency is measured once per picture from its first trailer.
func TestVideoQuality_SVC(t *testing.T) {
	for _, c := range []VideoCodec{CodecVP9, CodecAV1} {
		t.Run(string(c), func(t *testing.T) {
			w := &recorder{}
			s := runSource(t, "svc-quality-"+t.Name(), loadSVC(t, c, 720), 4*time.Second, nil, w)
			packets, at := w.snapshot()
			q := NewVideoQualityFor(c, SourceDDExtID, 10*time.Second)
			q.SetLatency(s.SendTimes(), prometheus.NewHistogram(prometheus.HistogramOpts{Name: "x"}))
			var structure *DDStructure
			half := at[0].Add(2 * time.Second)
			var mid VideoQualitySnapshot
			for i := range packets {
				p := &packets[i]
				sid := spatialLayer(t, c, p, &structure)
				if !at[i].Before(half) {
					if mid.Frames == 0 {
						mid = q.Snapshot(false)
					}
					if sid == 2 {
						continue // The server forwards up to the 360 layer from here
					}
				}
				q.UpdateRTP(&p.Header, p.Payload, at[i].Add(20*time.Millisecond))
			}
			end := q.Snapshot(false)
			if mid.Height != 720 || mid.Switches != 0 {
				t.Fatalf("every layer: height %d, %d switches", mid.Height, mid.Switches)
			}
			if mid.Frames < 58 || mid.Frames > 61 {
				t.Fatalf("every layer: %d pictures in 2 s, want 60 (one per picture, not per layer frame)", mid.Frames)
			}
			if end.Height != 360 || end.Switches != 1 || end.Freezes != 0 {
				t.Fatalf("top layer dropped: height %d, %d switches, %d freezes", end.Height, end.Switches, end.Freezes)
			}
			if end.Frames < 118 || end.Frames > 121 || end.HeightTime[720] < 1900*time.Millisecond || end.HeightTime[360] < 1900*time.Millisecond {
				t.Fatalf("%d pictures, time per height %v", end.Frames, end.HeightTime)
			}
			if end.Latency.N < 110 || end.Latency.N > end.Frames {
				t.Fatalf("%d latency samples over %d pictures", end.Latency.N, end.Frames)
			}
			if p50, _ := end.Latency.Quantile(0.5); p50 < 15*time.Millisecond || p50 > 25*time.Millisecond {
				t.Fatalf("median latency %s, want 20 ms", p50)
			}
		})
	}
}

// Janus does not forward the dependency descriptor to subscribers: an AV1
// receiver without it takes each layer frame's spatial layer from its OBU
// extension headers and the layer's height from the run's layer heights
// (every bot sends the same content).
func TestVideoQuality_SVCWithoutDescriptor(t *testing.T) {
	w := &recorder{}
	runSource(t, "svc-nodd-"+t.Name(), loadSVC(t, CodecAV1, 720), 4*time.Second, nil, w)
	packets, at := w.snapshot()
	q := NewVideoQualityFor(CodecAV1, 0, 10*time.Second) // Not negotiated
	q.SetLayerHeights([]int{180, 360, 720})
	var structure *DDStructure
	half := at[0].Add(2 * time.Second)
	var mid VideoQualitySnapshot
	for i := range packets {
		p := packets[i]
		sid := spatialLayer(t, CodecAV1, &p, &structure)
		p.Header.Extension, p.Header.Extensions = false, nil
		if !at[i].Before(half) {
			if mid.Frames == 0 {
				mid = q.Snapshot(false)
			}
			if sid > 0 {
				continue // The server forwards the lowest layer from here
			}
		}
		q.UpdateRTP(&p.Header, p.Payload, at[i].Add(20*time.Millisecond))
	}
	end := q.Snapshot(false)
	if mid.Height != 720 || mid.Switches != 0 || mid.Frames < 58 || mid.Frames > 61 {
		t.Fatalf("every layer: height %d, %d switches, %d pictures", mid.Height, mid.Switches, mid.Frames)
	}
	if end.Height != 180 || end.Switches != 1 || end.Freezes != 0 {
		t.Fatalf("lowest layer only: height %d, %d switches, %d freezes", end.Height, end.Switches, end.Freezes)
	}
}
