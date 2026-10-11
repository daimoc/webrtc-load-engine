package execution

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"webrtc-load-engine/src/media"
)

type fakeMediaPeer struct {
	id        string
	state     PeerState
	snap      map[string]KindMediaSnapshot
	transport uint64 // Transport bytes received
	height    int    // Requested video height (0: 360)
}

func (f *fakeMediaPeer) TransportBytesReceived() uint64 { return f.transport }
func (f *fakeMediaPeer) RequestedVideoHeight() int {
	if f.height == 0 {
		return 360
	}
	return f.height
}

func (f *fakeMediaPeer) ID() string                                           { return f.id }
func (f *fakeMediaPeer) State() PeerState                                     { return f.state }
func (f *fakeMediaPeer) MediaSnapshot(time.Time) map[string]KindMediaSnapshot { return f.snap }

func TestMediaCollector_AddsDeltasAndSetsRemoteStreams(t *testing.T) {
	id := "collect-deltas-" + uuid.NewString()
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	now := time.Unix(1000, 0)
	a := &fakeMediaPeer{id: "a", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"audio": {Packets: 10, Bytes: 1000, Lost: 1, Offered: 2, Streams: 2, LastReceived: now, Jitters: []float64{0.001, 0.002}, RoundTrips: []time.Duration{30 * time.Millisecond}},
	}}
	b := &fakeMediaPeer{id: "b", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"audio": {Packets: 5, Bytes: 500, Offered: 2, Streams: 2, LastReceived: now},
	}}
	idle := &fakeMediaPeer{id: "c", state: PeerStateConnecting, snap: map[string]KindMediaSnapshot{
		"audio": {Packets: 1000, Offered: 7},
	}}
	c.collect(now, []mediaPeer{a, b, idle})
	if got := testutil.ToFloat64(media.PacketsReceived.WithLabelValues(id, "audio")); got != 15 {
		t.Fatalf("expected 15 packets, got %v", got)
	}
	if got := testutil.ToFloat64(media.RemoteStreams.WithLabelValues(id, "audio")); got != 4 {
		t.Fatalf("expected 4 remote streams (non-established peers excluded), got %v", got)
	}

	a.snap["audio"] = KindMediaSnapshot{Packets: 25, Bytes: 2500, Lost: 3, Offered: 1, LastReceived: now}
	c.collect(now.Add(2*time.Second), []mediaPeer{a, b, idle})
	checks := map[string]float64{
		"packets": testutil.ToFloat64(media.PacketsReceived.WithLabelValues(id, "audio")),
		"bytes":   testutil.ToFloat64(media.BytesReceived.WithLabelValues(id, "audio")),
		"lost":    testutil.ToFloat64(media.PacketsLost.WithLabelValues(id, "audio")),
		"streams": testutil.ToFloat64(media.RemoteStreams.WithLabelValues(id, "audio")),
	}
	want := map[string]float64{"packets": 30, "bytes": 3000, "lost": 3, "streams": 3}
	for k, v := range want {
		if checks[k] != v {
			t.Errorf("%s: expected %v, got %v", k, v, checks[k])
		}
	}
	if n := testutil.CollectAndCount(media.Jitter); n == 0 {
		t.Error("expected jitter samples")
	}

	c.finish()
	if got := testutil.ToFloat64(media.RemoteStreams.WithLabelValues(id, "audio")); got != 0 {
		t.Errorf("expected remote streams reset at the end, got %v", got)
	}
}

func TestMediaCollector_CountsStallsOnce(t *testing.T) {
	id := "collect-stalls-" + uuid.NewString()
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	t0 := time.Unix(2000, 0)
	p := &fakeMediaPeer{id: "p", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"video": {Packets: 100, Offered: 1, Streams: 1, LastReceived: t0},
	}}
	stalls := func() float64 { return testutil.ToFloat64(media.Stalls.WithLabelValues(id, "video")) }

	c.collect(t0.Add(5*time.Second), []mediaPeer{p})
	if stalls() != 0 {
		t.Fatal("no stall before the timeout")
	}
	c.collect(t0.Add(11*time.Second), []mediaPeer{p})
	c.collect(t0.Add(13*time.Second), []mediaPeer{p})
	if stalls() != 1 {
		t.Fatalf("expected one stall counted once, got %v", stalls())
	}
	p.snap["video"] = KindMediaSnapshot{Packets: 120, Offered: 1, Streams: 1, LastReceived: t0.Add(14 * time.Second)}
	c.collect(t0.Add(15*time.Second), []mediaPeer{p})
	c.collect(t0.Add(30*time.Second), []mediaPeer{p})
	if stalls() != 2 {
		t.Fatalf("expected a second stall after media resumed then stopped, got %v", stalls())
	}
	if p.State() != PeerStateEstablished {
		t.Fatal("a stall must not change the lifecycle")
	}
}

func TestMediaCollector_NoStallWithoutRemoteSourcesOrMedia(t *testing.T) {
	id := "collect-nostall-" + uuid.NewString()
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	t0 := time.Unix(3000, 0)
	never := &fakeMediaPeer{id: "never", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"video": {Offered: 3}, // Offered but nothing ever received (e.g. the bridge forwards no video)
	}}
	alone := &fakeMediaPeer{id: "alone", state: PeerStateEstablished, snap: map[string]KindMediaSnapshot{
		"audio": {Packets: 50, Offered: 0, LastReceived: t0}, // The last remote sender left
	}}
	c.collect(t0.Add(time.Minute), []mediaPeer{never, alone})
	for _, kind := range []string{"audio", "video"} {
		if n := testutil.ToFloat64(media.Stalls.WithLabelValues(id, kind)); n != 0 {
			t.Errorf("%s: expected no stall, got %v", kind, n)
		}
	}
}

func TestMediaCollector_AddsTransportBytes(t *testing.T) {
	id := "collect-transport-" + uuid.NewString()
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	now := time.Unix(1000, 0)
	read := func() float64 { return testutil.ToFloat64(transportBytesReceived.WithLabelValues(id)) }

	a := &fakeMediaPeer{id: "a", state: PeerStateEstablished, transport: 1000}
	idle := &fakeMediaPeer{id: "b", state: PeerStateConnecting, transport: 50000}
	c.collect(now, []mediaPeer{a, idle})
	if got := read(); got != 1000 {
		t.Fatalf("expected 1000 bytes, got %v", got)
	}
	a.transport = 3500
	c.collect(now.Add(2*time.Second), []mediaPeer{a, idle})
	if got := read(); got != 3500 {
		t.Fatalf("expected 1000 + 2500 bytes, got %v", got)
	}
	// No longer established: not read again.
	a.state, a.transport = PeerStateDisconnected, 9000
	c.collect(now.Add(4*time.Second), []mediaPeer{a, idle})
	if got := read(); got != 3500 {
		t.Errorf("expected a peer that left not to be read, got %v", got)
	}
}
