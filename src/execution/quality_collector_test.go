package execution

import (
	"bytes"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"

	"webrtc-load-engine/src/media"
)

// histogram returns the sample count and sum of a histogram series.
func histogram(t *testing.T, o prometheus.Observer) (uint64, float64) {
	t.Helper()
	var m dto.Metric
	if err := o.(prometheus.Histogram).Write(&m); err != nil {
		t.Fatal(err)
	}
	return m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
}

// syncBuffer is a log destination safe for concurrent writes.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func qualityPeer(now time.Time) *fakeMediaPeer {
	return &fakeMediaPeer{id: "a", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"video": {
			Packets: 100, Offered: 2, Streams: 2, LastReceived: now,
			Video: []media.VideoQualitySnapshot{
				{Frames: 60, Freezes: 1, FreezeTime: time.Second, Height: 360, HeightTime: map[int]time.Duration{360: 2 * time.Second}, PeriodIntervals: 60, PeriodSpan: 2 * time.Second},
				{Frames: 30, Height: 180, HeightTime: map[int]time.Duration{180: time.Second}, Switches: 1, PeriodIntervals: 30, PeriodSpan: 2 * time.Second},
			},
			KeyframeRequestsSent: 2, KeyframeRequestsReceived: 1,
			RoundTrips: []time.Duration{40 * time.Millisecond},
		},
		"audio": {
			Packets: 100, Offered: 1, Streams: 1, LastReceived: now,
			Audio: []media.AudioQualitySnapshot{{Frame: 20 * time.Millisecond, Concealed: 1, Expected: 100, PeriodConcealed: 1, PeriodExpected: 100}},
		},
	}}
}

func TestMediaCollector_QualityMetrics(t *testing.T) {
	id := "collect-quality-" + uuid.NewString()
	logs := &syncBuffer{}
	c := newMediaCollector(id, 10*time.Second, slog.New(slog.NewTextHandler(logs, nil)))
	now := time.Unix(1000, 0)
	a := qualityPeer(now)
	c.collect(now, []mediaPeer{a})

	if n, sum := histogram(t, media.FrameRate.WithLabelValues(id)); n != 2 || sum != 45 {
		t.Errorf("frame rate: %d samples, sum %v; want 2, 45", n, sum)
	}
	if n, sum := histogram(t, media.FrameHeight.WithLabelValues(id)); n != 2 || sum != 540 {
		t.Errorf("frame height: %d samples, sum %v; want 2, 540", n, sum)
	}
	// MOS with 1% concealed and half the 40 ms round trip + 80 ms.
	if n, sum := histogram(t, media.AudioMOS.WithLabelValues(id)); n != 1 || math.Abs(sum-media.MOS(1, 100*time.Millisecond)) > 1e-9 {
		t.Errorf("MOS: %d samples, sum %v", n, sum)
	}
	counters := map[string]prometheus.Collector{
		"freezes":     media.Freezes.WithLabelValues(id),
		"freeze s":    media.FreezeSeconds.WithLabelValues(id),
		"360 s":       media.HeightSeconds.WithLabelValues(id, "360"),
		"180 s":       media.HeightSeconds.WithLabelValues(id, "180"),
		"switches":    media.ResolutionSwitches.WithLabelValues(id),
		"kf sent":     media.KeyframeRequestsSent.WithLabelValues(id),
		"kf received": media.KeyframeRequestsReceived.WithLabelValues(id),
		"concealed s": media.AudioConcealedSeconds.WithLabelValues(id),
		"expected s":  media.AudioExpectedSeconds.WithLabelValues(id),
	}
	check := func(want map[string]float64) {
		t.Helper()
		for k, w := range want {
			if got := testutil.ToFloat64(counters[k]); math.Abs(got-w) > 1e-9 {
				t.Errorf("%s: %v, want %v", k, got, w)
			}
		}
	}
	check(map[string]float64{"freezes": 1, "freeze s": 1, "360 s": 2, "180 s": 1, "switches": 1,
		"kf sent": 2, "kf received": 1, "concealed s": 0.02, "expected s": 2})

	// Cumulative values grow: only the deltas are added.
	v := a.snap["video"]
	v.Video = []media.VideoQualitySnapshot{
		{Frames: 120, Freezes: 2, FreezeTime: 1500 * time.Millisecond, Height: 360, HeightTime: map[int]time.Duration{360: 4 * time.Second}, PeriodIntervals: 60, PeriodSpan: 2 * time.Second},
		{Frames: 60, Height: 180, HeightTime: map[int]time.Duration{180: 3 * time.Second}, Switches: 1, PeriodIntervals: 30, PeriodSpan: 2 * time.Second},
	}
	v.KeyframeRequestsSent = 3
	a.snap["video"] = v
	c.collect(now.Add(2*time.Second), []mediaPeer{a})
	check(map[string]float64{"freezes": 2, "freeze s": 1.5, "360 s": 4, "180 s": 3, "switches": 1, "kf sent": 3})

	// The two freezes of the stream within a minute are logged once.
	if n := strings.Count(logs.String(), "Video freeze"); n != 1 {
		t.Errorf("freeze logged %d times, want 1:\n%s", n, logs.String())
	}
	if !strings.Contains(logs.String(), "peer_id=a") || !strings.Contains(logs.String(), "scenario_id="+id) {
		t.Errorf("freeze log without identifiers:\n%s", logs.String())
	}

	// The peer leaves: its record stays in the summary.
	a.state = PeerStateDisconnected
	c.collect(now.Add(4*time.Second), []mediaPeer{a})
	s := c.summary(now.Add(4 * time.Second))
	if s.Bots != 1 || !s.Video.Available || s.Video.FreezesTotal != 2 || s.Audio.MOS == nil {
		t.Fatalf("summary after the peer left: %+v", s)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// summaryCollector returns a collector that has seen n bots receiving audio
// and video; bot i has MOS falling and freezes growing with i.
func summaryCollector(t *testing.T, n int) *mediaCollector {
	t.Helper()
	c := newMediaCollector("summary-"+uuid.NewString(), 10*time.Second, quietLogger)
	now := time.Unix(1000, 0)
	var peers []mediaPeer
	for i := 0; i < n; i++ {
		peers = append(peers, &fakeMediaPeer{id: fmt.Sprintf("bot-%02d", i), state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
			"video": {Packets: 10, Offered: 1, Streams: 1, LastReceived: now, Video: []media.VideoQualitySnapshot{{
				Frames: 60, Freezes: uint64(i), FreezeTime: time.Duration(i) * 100 * time.Millisecond,
				Height: 360, HeightTime: map[int]time.Duration{360: 3 * time.Second, 180: time.Second},
				PeriodIntervals: 60 - i, PeriodSpan: 2 * time.Second,
			}}, KeyframeRequestsSent: 1},
			"audio": {Packets: 10, Offered: 1, Streams: 1, LastReceived: now, Audio: []media.AudioQualitySnapshot{{
				Frame: 20 * time.Millisecond, Concealed: uint64(i), Expected: 100, PeriodConcealed: uint64(i), PeriodExpected: 100,
			}}},
		}})
	}
	c.collect(now, peers)
	return c
}

func TestMediaCollector_Summary(t *testing.T) {
	c := summaryCollector(t, 12)
	now := time.Unix(2000, 0)
	s := c.summary(now)
	if s.ScenarioID != c.scenarioID || !s.GeneratedAt.Equal(now) || s.Bots != 12 {
		t.Fatalf("header: %+v", s)
	}
	v := s.Video
	if !v.Available || v.Bots != 12 || v.FreezesTotal != 66 || v.KeyframeRequestsSentTotal != 12 {
		t.Fatalf("video: %+v", v)
	}
	// Frame rates 30, 29.5, ... 24.5 (nearest rank).
	if !near(v.FrameRate.P50, 27) || !near(v.FrameRate.P95, 30) || !near(v.FrameRate.P99, 30) {
		t.Errorf("frame rate %+v", *v.FrameRate)
	}
	if !near(v.FreezeSeconds.P50, 0.5) || !near(v.FreezeSeconds.P99, 1.1) {
		t.Errorf("freeze seconds %+v", *v.FreezeSeconds)
	}
	if !near(v.HeightShare["360"], 0.75) || !near(v.HeightShare["180"], 0.25) {
		t.Errorf("height share %v", v.HeightShare)
	}
	if v.LatencyAvailable || v.Latency != nil {
		t.Errorf("latency without measures: %+v", v.KindQuality)
	}
	a := s.Audio
	if !a.Available || a.Bots != 12 || !near(a.ConcealedRatio.P50, 0.05) || !near(a.ConcealedRatio.P99, 0.11) {
		t.Fatalf("audio: %+v %+v", a, a.ConcealedRatio)
	}
	if a.MOS.P50 >= 4.41 || a.MOS.P50 <= 3 {
		t.Errorf("MOS %+v", *a.MOS)
	}

	if n := len(s.Worst.LowestMOS); n != 10 {
		t.Fatalf("lowest MOS: %d bots, want 10", n)
	}
	if s.Worst.LowestMOS[0].PeerID != "bot-11" || s.Worst.LowestMOS[9].PeerID != "bot-02" {
		t.Errorf("lowest MOS order: %+v", s.Worst.LowestMOS)
	}
	if f := s.Worst.MostFreezeSeconds; len(f) != 10 || f[0].PeerID != "bot-11" || math.Abs(f[0].Value-1.1) > 1e-9 {
		t.Errorf("most freeze seconds: %+v", f)
	}
	// Every bot asks for 360 and spends 3 s of 4 at 360.
	if s.Video.RequestedHeightShare == nil || !near(s.Video.RequestedHeightShare.P50, 0.75) || len(s.Worst.LowestRequestedHeight) != 10 {
		t.Errorf("requested height share %+v, worst %+v", s.Video.RequestedHeightShare, s.Worst.LowestRequestedHeight)
	}
	if len(s.Worst.HighestLatencyP95Seconds) != 0 {
		t.Errorf("latency ranking without latency: %+v", s.Worst.HighestLatencyP95Seconds)
	}
}

func TestMediaCollector_SummaryWithoutVideo(t *testing.T) {
	c := newMediaCollector("summary-audio-"+uuid.NewString(), 10*time.Second, quietLogger)
	now := time.Unix(1000, 0)
	c.collect(now, []mediaPeer{&fakeMediaPeer{id: "a", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"audio": {Packets: 10, Offered: 1, Streams: 1, LastReceived: now, Audio: []media.AudioQualitySnapshot{{Frame: 20 * time.Millisecond, Expected: 100, PeriodExpected: 100}}},
	}}})
	s := c.summary(now)
	if s.Video.Available || s.Video.Reason != "no bot received video" || s.Video.FrameRate != nil {
		t.Errorf("video: %+v", s.Video)
	}
	if !s.Audio.Available || s.Audio.Bots != 1 {
		t.Errorf("audio: %+v", s.Audio)
	}

	empty := newMediaCollector("summary-empty-"+uuid.NewString(), 10*time.Second, quietLogger).summary(now)
	if empty.Bots != 0 || empty.Audio.Available || empty.Audio.Reason != "no bot received audio" {
		t.Errorf("empty: %+v", empty)
	}
}

// latencyOf returns buckets with n samples of d.
func latencyOf(d time.Duration, n int) media.LatencyBuckets {
	var b media.LatencyBuckets
	for i := 0; i < n; i++ {
		b.Add(d)
	}
	return b
}

func TestMediaCollector_Latency(t *testing.T) {
	id := "collect-latency-" + uuid.NewString()
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	now := time.Unix(1000, 0)
	var peers []mediaPeer
	for i := 0; i < 3; i++ {
		d := time.Duration(10*(i+1)) * time.Millisecond // 10, 20, 30 ms
		peers = append(peers, &fakeMediaPeer{id: fmt.Sprintf("bot-%d", i), state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
			"video": {Packets: 10, Offered: 1, Streams: 1, LastReceived: now, Video: []media.VideoQualitySnapshot{{
				Frames: 60, Height: 360, PeriodIntervals: 59, PeriodSpan: 2 * time.Second, Latency: latencyOf(d+5*time.Millisecond, 60),
			}}},
			"audio": {Packets: 10, Offered: 1, Streams: 1, LastReceived: now, RoundTrips: []time.Duration{400 * time.Millisecond},
				Audio: []media.AudioQualitySnapshot{{Frame: 20 * time.Millisecond, Expected: 100, PeriodExpected: 100, Latency: latencyOf(d, 100)}}},
		}})
	}
	c.collect(now, peers)

	// The score uses the stream's median latency (10.5 ms + 80 ms for bot 0),
	// not half the 400 ms round trip.
	if n, sum := histogram(t, media.AudioMOS.WithLabelValues(id)); n != 3 ||
		math.Abs(sum-(media.MOS(0, 90500*time.Microsecond)+media.MOS(0, 100500*time.Microsecond)+media.MOS(0, 110500*time.Microsecond))) > 1e-9 {
		t.Errorf("MOS: %d samples, sum %v", n, sum)
	}

	s := c.summary(now)
	if !s.Audio.LatencyAvailable || s.Audio.Latency == nil || !near(s.Audio.Latency.P50, 0.0205) || !near(s.Audio.Latency.P99, 0.0305) {
		t.Errorf("audio latency %+v", s.Audio.KindQuality)
	}
	if !s.Video.LatencyAvailable || s.Video.Latency == nil || !near(s.Video.Latency.P50, 0.0255) {
		t.Errorf("video latency %+v", s.Video.KindQuality)
	}
	w := s.Worst.HighestLatencyP95Seconds
	if len(w) != 3 || w[0].PeerID != "bot-2" || !near(w[0].Value, 0.0355) {
		t.Errorf("highest latency p95: %+v", w)
	}
}
