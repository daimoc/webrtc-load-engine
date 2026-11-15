package execution

import (
	"strings"
	"testing"
	"time"

	"github.com/pion/sdp/v3"

	"webrtc-load-engine/src/scenario"
)

// fakeLink is the estimator's view: an estimate and whether it sees
// congestion.
type fakeLink struct {
	estimate  int
	congested bool
}

func newTestAdapter(layers int) (*senderAdapter, *fakeLink, *[]int, time.Time) {
	link := &fakeLink{estimate: 2_000_000}
	var applied []int
	t0 := time.Unix(1000, 0)
	a := newSenderAdapter([]int{180, 360, 720}[3-layers:], func() (int, bool, bool) { return link.estimate, link.congested, true },
		func(n int) { applied = append(applied, n) }, t0)
	// Past the warm-up, on a clean link.
	for t := t0; t.Before(t0.Add(senderWarmUp)); t = t.Add(senderTick) {
		a.tick(t)
	}
	return a, link, &applied, t0.Add(senderWarmUp)
}

func run(a *senderAdapter, from time.Time, d time.Duration) time.Time {
	end := from.Add(d)
	for t := from; t.Before(end); t = t.Add(senderTick) {
		a.tick(t)
	}
	return end
}

func TestSenderAdapter_DropsAfterOneSecond(t *testing.T) {
	a, link, applied, t0 := newTestAdapter(3)
	now := run(a, t0, 5*time.Second)
	if len(*applied) != 0 {
		t.Fatalf("dropped on a clean link: %v", *applied)
	}
	link.congested = true
	now = run(a, now, 900*time.Millisecond)
	if len(*applied) != 0 {
		t.Fatal("dropped before 1 s of congestion")
	}
	now = run(a, now, 200*time.Millisecond)
	if len(*applied) != 1 || (*applied)[0] != 2 {
		t.Fatalf("after 1 s of congestion: %v, want one drop to 2 layers", *applied)
	}
	// Still congested: down to the lowest, never below.
	run(a, now, 10*time.Second)
	if last := (*applied)[len(*applied)-1]; last != 1 {
		t.Errorf("layers %v, want down to 1", *applied)
	}
	for _, n := range *applied {
		if n < 1 {
			t.Fatal("dropped the lowest layer")
		}
	}
}

func TestSenderAdapter_RestoreBackoff(t *testing.T) {
	a, link, applied, t0 := newTestAdapter(3)
	link.congested = true
	now := run(a, t0, 1100*time.Millisecond) // Dropped to 2 at about 1 s
	dropAt := now
	link.congested = false
	// First try 2 s after the drop.
	now = run(a, now, 1800*time.Millisecond)
	if (*applied)[len(*applied)-1] != 2 {
		t.Fatal("tried before 2 s")
	}
	now = run(a, now, 400*time.Millisecond)
	if (*applied)[len(*applied)-1] != 3 {
		t.Fatalf("no try 2 s after the drop: %v", *applied)
	}
	// The try fails: congestion within 3 s drops it again, and the next
	// try waits 4 s.
	link.congested = true
	now = run(a, now, 1100*time.Millisecond)
	if (*applied)[len(*applied)-1] != 2 {
		t.Fatalf("failed try not dropped: %v", *applied)
	}
	link.congested = false
	failedAt := now
	now = run(a, now, 3800*time.Millisecond)
	if (*applied)[len(*applied)-1] != 2 {
		t.Fatalf("retried before 4 s: %v", *applied)
	}
	now = run(a, now, 400*time.Millisecond)
	if (*applied)[len(*applied)-1] != 3 {
		t.Fatalf("no retry 4 s after a failed try: %v", *applied)
	}
	// Held 3 s: a success; the wait goes back to 2 s.
	now = run(a, now, 3500*time.Millisecond)
	if a.backoff != senderBackoffStart {
		t.Errorf("backoff %v after a success, want %v", a.backoff, senderBackoffStart)
	}
	_, _ = dropAt, failedAt
	r := a.result(t0)
	if !r.Adapts || len(r.Layers) < 4 || r.Layers[0].AtSeconds != -5 || len(r.Layers[0].Heights) != 3 {
		t.Errorf("result %+v", r)
	}
	if len(r.EstimateKbps) < 10 {
		t.Errorf("%d estimate samples, want one per second", len(r.EstimateKbps))
	}
}

// Every try meets congestion (the link has room for 2 layers): the wait
// doubles up to its cap. No try is made while congestion is under way.
func TestSenderAdapter_BackoffCapped(t *testing.T) {
	a, link, applied, t0 := newTestAdapter(3)
	link.congested = true
	now := run(a, t0, 1100*time.Millisecond)
	for i := 0; i < 600; i++ {
		link.congested = (*applied)[len(*applied)-1] == 3
		a.tick(now)
		now = now.Add(senderTick)
	}
	if a.backoff != senderBackoffMax {
		t.Errorf("backoff %v after failing tries, want capped at %v", a.backoff, senderBackoffMax)
	}
}

func TestSenderAdapter_SingleStream(t *testing.T) {
	a, link, applied, t0 := newTestAdapter(1)
	link.congested = true
	run(a, t0, 10*time.Second)
	if len(*applied) != 0 {
		t.Errorf("a single stream changed: %v", *applied)
	}
	if r := a.result(t0); r.Adapts || r.Reason != scenario.CannotAdapt {
		t.Errorf("single stream result %+v, want cannot_adapt", r)
	}
}

// Only a peer that adapts offers transport-wide sequence numbers and the
// transport-cc feedback on its publish session; the others send as before
// 016 (FR-018, T037).
func TestPeer_SenderEstimationOnlyWhenAdapting(t *testing.T) {
	for _, adapt := range []bool{false, true} {
		p, err := NewPeer("peer-up", "sc", PeerMedia{Kinds: []string{"audio", "video"}, PublishSession: true, adaptSending: adapt}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		offer, err := p.pub.CreateOffer(nil)
		if err != nil {
			t.Fatal(err)
		}
		hasExt := strings.Contains(offer.SDP, sdp.TransportCCURI)
		hasFB := strings.Contains(offer.SDP, " transport-cc")
		if hasExt != adapt || hasFB != adapt {
			t.Errorf("adapt %v: transport-cc extension %v, feedback %v", adapt, hasExt, hasFB)
		}
		if (p.sender != nil) != adapt {
			t.Errorf("adapt %v: sender %v", adapt, p.sender)
		}
		if adapt {
			if r := p.senderResult(time.Now()); r == nil || r.Adapts || r.Reason != scenario.CannotAdapt {
				t.Errorf("single-stream adapting peer: %+v, want cannot_adapt", r)
			}
		}
		p.Close()
	}
}

// No layer is dropped in the warm-up, however congested the estimator says
// the link is.
func TestSenderAdapter_WarmUp(t *testing.T) {
	var applied []int
	t0 := time.Unix(1000, 0)
	a := newSenderAdapter([]int{180, 360, 720}, func() (int, bool, bool) { return 100000, true, true },
		func(n int) { applied = append(applied, n) }, t0)
	run(a, t0, senderWarmUp-200*time.Millisecond)
	if len(applied) != 0 {
		t.Fatalf("dropped in the warm-up: %v", applied)
	}
	run(a, t0.Add(senderWarmUp), 1200*time.Millisecond)
	if len(applied) != 1 {
		t.Errorf("after the warm-up: %v, want one drop", applied)
	}
}
