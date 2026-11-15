package execution

import (
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"webrtc-load-engine/src/scenario"
)

// simulate offers n datagrams of size bytes at offeredKbps to a link with
// condition c, on a simulated clock, and returns the delivery times of
// those not dropped.
func simulate(t *testing.T, c *scenario.LinkCondition, offeredKbps, size int, seconds float64) (*linkShaper, []time.Time) {
	t.Helper()
	l := newLinkShaper(1)
	start := time.Unix(1000, 0)
	l.setCondition(0, c, start)
	interval := time.Duration(float64(time.Second) * float64(size*8) / float64(offeredKbps*1000))
	var dues []time.Time
	for now := start; now.Sub(start).Seconds() < seconds; now = now.Add(interval) {
		due, ok := l.admit(now, size)
		if ok {
			dues = append(dues, due)
		}
	}
	return l, dues
}

func TestShaper_RateCap(t *testing.T) {
	c := &scenario.LinkCondition{RateKbps: 500, QueueMs: 200}
	l, dues := simulate(t, c, 1000, 1172, 20)
	// Delivered bytes per 10 s window, with the 28 header bytes counted.
	start := time.Unix(1000, 0)
	perSecond := map[int]int{}
	for _, d := range dues {
		perSecond[int(d.Sub(start).Seconds())] += 1172 + udpIPHeaderBytes
	}
	for w := 0; w+10 <= 20; w++ {
		bytes := 0
		for s := w; s < w+10; s++ {
			bytes += perSecond[s]
		}
		kbps := float64(bytes*8) / 10 / 1000
		if kbps > 500*1.10 {
			t.Errorf("window %d–%d s: %.0f kbit/s, more than 10%% over the 500 kbit/s cap", w, w+10, kbps)
		}
		if w >= 1 && kbps < 500*0.90 {
			t.Errorf("window %d–%d s: %.0f kbit/s, less than 90%% of the cap with twice as much offered", w, w+10, kbps)
		}
	}
	got := l.counters()[0]
	if got.DroppedCap == 0 || got.Offered != got.Delivered+got.DroppedCap {
		t.Errorf("counters %+v: want drops by the cap and offered = delivered + dropped", got)
	}
}

func TestShaper_QueueBound(t *testing.T) {
	c := &scenario.LinkCondition{RateKbps: 500, QueueMs: 200}
	l := newLinkShaper(1)
	now := time.Unix(1000, 0)
	l.setCondition(0, c, now)
	// A burst at one instant: what is admitted never waits more than the
	// queue's 200 ms plus one datagram's serialization.
	var last time.Time
	for i := 0; i < 100; i++ {
		if due, ok := l.admit(now, 1200); ok {
			last = due
		}
	}
	one := time.Duration(float64(time.Second) * float64((1200+udpIPHeaderBytes)*8) / 500000)
	if wait := last.Sub(now); wait > 200*time.Millisecond+one {
		t.Errorf("longest wait %v, more than the 200 ms queue", wait)
	}
	if l.counters()[0].DroppedCap == 0 {
		t.Error("a 100-datagram burst at 500 kbit/s was not cut by the queue")
	}
}

func TestShaper_PassThrough(t *testing.T) {
	l := newLinkShaper(1)
	now := time.Unix(1000, 0)
	if due, ok := l.admit(now, 1000); !ok || !due.Equal(now) {
		t.Errorf("no condition: due %v ok %v, want now", due, ok)
	}
	if len(l.counters()) != 0 {
		t.Error("counted with no condition in force")
	}
	l.setCondition(0, &scenario.LinkCondition{RateKbps: 100}, now)
	l.setCondition(-1, nil, now)
	if due, ok := l.admit(now.Add(time.Second), 1000); !ok || !due.Equal(now.Add(time.Second)) {
		t.Errorf("condition removed: due %v ok %v", due, ok)
	}
}

func TestShaper_CountersPerStep(t *testing.T) {
	l := newLinkShaper(1)
	now := time.Unix(1000, 0)
	l.setCondition(0, &scenario.LinkCondition{RateKbps: 1000}, now)
	l.admit(now, 100)
	l.setCondition(1, &scenario.LinkCondition{RateKbps: 2000}, now)
	l.admit(now, 100)
	l.admit(now, 100)
	c := l.counters()
	if c[0].Offered != 1 || c[1].Offered != 2 {
		t.Errorf("per step offered: %d, %d; want 1, 2", c[0].Offered, c[1].Offered)
	}
}

// TestShaper_Sockets sends through a shaped link between two local UDP
// sockets in real time.
func TestShaper_Sockets(t *testing.T) {
	if testing.Short() {
		t.Skip("real time")
	}
	rx, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer rx.Close()
	tx, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Close()
	l := newLinkShaper(1)
	l.setCondition(0, &scenario.LinkCondition{RateKbps: 400, QueueMs: 200}, time.Now())
	defer l.close()

	var mu sync.Mutex
	received := 0
	go func() {
		buf := make([]byte, 2000)
		for {
			n, _, err := rx.ReadFrom(buf)
			if err != nil {
				return
			}
			mu.Lock()
			received += n + udpIPHeaderBytes
			mu.Unlock()
		}
	}()
	payload := make([]byte, 972) // 1000 bytes on the wire
	start := time.Now()
	for time.Since(start) < 3*time.Second { // 800 kbit/s offered
		pkt := append([]byte(nil), payload...)
		l.offer(pkt, func(p []byte) { _, _ = tx.WriteTo(p, rx.LocalAddr()) })
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	kbps := float64(received*8) / time.Since(start).Seconds() / 1000
	mu.Unlock()
	if kbps > 400*1.10 || kbps < 400*0.75 {
		t.Errorf("received %.0f kbit/s through a 400 kbit/s cap", kbps)
	}
}

func TestShaper_DelayAndJitter(t *testing.T) {
	l := newLinkShaper(7)
	start := time.Unix(1000, 0)
	l.setCondition(0, &scenario.LinkCondition{DelayMs: 40, JitterMs: 10}, start)
	var prev time.Time
	var sum time.Duration
	n := 0
	lo, hi := time.Hour, time.Duration(0)
	for i := 0; i < 5000; i++ {
		now := start.Add(time.Duration(i) * 20 * time.Millisecond) // Far apart: jitter shows as is
		due, ok := l.admit(now, 200)
		if !ok {
			t.Fatal("dropped without loss or cap")
		}
		if due.Before(prev) {
			t.Fatal("reordered")
		}
		prev = due
		d := due.Sub(now)
		sum += d
		lo, hi = min(lo, d), max(hi, d)
		n++
	}
	if mean := sum / time.Duration(n); mean < 35*time.Millisecond || mean > 45*time.Millisecond {
		t.Errorf("mean added delay %v, want 40 ms within 5", mean)
	}
	if lo < 30*time.Millisecond || hi > 50*time.Millisecond || hi-lo < 15*time.Millisecond {
		t.Errorf("delays from %v to %v, want spread over 30–50 ms", lo, hi)
	}
	// Packets close together keep their order: a later one never leaves first.
	l2 := newLinkShaper(7)
	l2.setCondition(0, &scenario.LinkCondition{DelayMs: 40, JitterMs: 20}, start)
	prev = time.Time{}
	for i := 0; i < 1000; i++ {
		due, _ := l2.admit(start.Add(time.Duration(i)*time.Millisecond), 200)
		if due.Before(prev) {
			t.Fatal("reordered")
		}
		prev = due
	}
}

// lossRun offers n datagrams under share and burst with seed, and returns
// the counters.
func lossRun(seed uint64, n int, share, burst float64) stepCounters {
	l := newLinkShaper(seed)
	start := time.Unix(1000, 0)
	l.setCondition(0, &scenario.LinkCondition{Loss: &scenario.Loss{Share: share, MeanBurst: burst}}, start)
	for i := 0; i < n; i++ {
		l.admit(start.Add(time.Duration(i)*time.Millisecond), 100)
	}
	return l.counters()[0]
}

func TestShaper_RandomLoss(t *testing.T) {
	c := lossRun(3, 50000, 0.03, 1)
	share := float64(c.DroppedLoss) / float64(c.Offered)
	if math.Abs(share-0.03) > 0.01 {
		t.Errorf("dropped %.4f, want 0.03 within 1 point", share)
	}
	if c.Delivered+c.DroppedLoss != c.Offered {
		t.Errorf("counters %+v", c)
	}
}

func TestShaper_BurstLoss(t *testing.T) {
	c := lossRun(3, 50000, 0.05, 4)
	share := float64(c.DroppedLoss) / float64(c.Offered)
	if math.Abs(share-0.05) > 0.01 {
		t.Errorf("dropped %.4f, want 0.05 within 1 point", share)
	}
	if mean := float64(c.DroppedLoss) / float64(c.Bursts); math.Abs(mean-4) > 0.6 {
		t.Errorf("mean burst %.2f, want 4 within 15%%", mean)
	}
}

func TestShaper_SeedRepeats(t *testing.T) {
	a, b := lossRun(11, 60000, 0.03, 2), lossRun(11, 61000, 0.03, 2)
	sa, sb := float64(a.DroppedLoss)/float64(a.Offered), float64(b.DroppedLoss)/float64(b.Offered)
	if math.Abs(sa-sb) > 0.005 {
		t.Errorf("same seed: %.4f and %.4f, want within 0.5 point (SC-005)", sa, sb)
	}
	if x, y := lossRun(11, 1000, 0.1, 1), lossRun(11, 1000, 0.1, 1); x.DroppedLoss != y.DroppedLoss {
		t.Error("the same seed and datagrams gave different drops")
	}
	if x, y := lossRun(11, 5000, 0.1, 1), lossRun(12, 5000, 0.1, 1); x.DroppedLoss == y.DroppedLoss {
		t.Log("two seeds gave the same count (possible, unlikely)")
	}
}

// Dense packets (a video frame's burst) keep the mean added delay within
// 5 ms of the plan (SC-001): jitter is held for jitterHold, so packets of
// one burst share a delay and do not wait behind each other.
func TestShaper_JitterOnBursts(t *testing.T) {
	l := newLinkShaper(9)
	start := time.Unix(1000, 0)
	l.setCondition(0, &scenario.LinkCondition{DelayMs: 40, JitterMs: 10}, start)
	var sum time.Duration
	n := 0
	for i := 0; i < 20000; i++ {
		now := start.Add(time.Duration(i) * 500 * time.Microsecond) // 2000 packets/s
		due, _ := l.admit(now, 1000)
		sum += due.Sub(now)
		n++
	}
	if mean := sum / time.Duration(n); mean < 35*time.Millisecond || mean > 45*time.Millisecond {
		t.Errorf("mean added delay %v on dense packets, want 40 ms within 5", mean)
	}
}
