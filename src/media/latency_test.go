package media

import (
	"sync"
	"testing"
	"time"
)

func TestSendTimes_RecordAndLookup(t *testing.T) {
	s := NewSendTimes()
	t0 := time.Unix(1000, 0)
	ts := uint32(4294967000)
	s.Record(ts, t0)
	s.Record(ts+960, t0.Add(20*time.Millisecond)) // Wraps around
	if at, ok := s.Lookup(ts); !ok || !at.Equal(t0) {
		t.Fatalf("lookup: %v %v", at, ok)
	}
	if at, ok := s.Lookup(ts + 960); !ok || !at.Equal(t0.Add(20*time.Millisecond)) {
		t.Fatalf("lookup after wrap: %v %v", at, ok)
	}
	if _, ok := s.Lookup(12345); ok {
		t.Fatal("unknown timestamp found")
	}
}

func TestSendTimes_ForgetsOldest(t *testing.T) {
	s := NewSendTimes()
	t0 := time.Unix(1000, 0)
	for i := 0; i < sendTimesSize+1; i++ {
		s.Record(uint32(i)*3000, t0.Add(time.Duration(i)*33*time.Millisecond))
	}
	if _, ok := s.Lookup(0); ok {
		t.Fatal("first frame still known after 513 records")
	}
	if _, ok := s.Lookup(3000); !ok {
		t.Fatal("second frame forgotten")
	}
	if _, ok := s.Lookup(sendTimesSize * 3000); !ok {
		t.Fatal("last frame unknown")
	}
}

func TestSendTimes_Concurrent(t *testing.T) {
	s := NewSendTimes()
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 2000; i++ {
				s.Lookup(uint32(i) * 960)
			}
		}()
	}
	for i := 0; i < 2000; i++ {
		s.Record(uint32(i)*960, time.Now())
	}
	wg.Wait()
}

func TestLatencyBuckets(t *testing.T) {
	var b LatencyBuckets
	if _, ok := b.Quantile(0.5); ok {
		t.Fatal("quantile of nothing")
	}
	for i := 0; i < 90; i++ {
		b.Add(50*time.Millisecond + time.Duration(i%3)*100*time.Microsecond)
	}
	for i := 0; i < 10; i++ {
		b.Add(300 * time.Millisecond)
	}
	b.Add(-time.Millisecond) // Ignored
	if b.N != 100 {
		t.Fatalf("N %d", b.N)
	}
	if q, _ := b.Quantile(0.5); q != 50500*time.Microsecond {
		t.Fatalf("median %s, want 50.5ms", q)
	}
	if q, _ := b.Quantile(0.95); q != 305*time.Millisecond {
		t.Fatalf("p95 %s, want 305ms", q)
	}

	var o LatencyBuckets
	o.Add(5 * time.Second) // Overflow
	b.Merge(&o)
	if q, _ := b.Quantile(1); q != 5*time.Second || b.N != 101 {
		t.Fatalf("max after merge %s, N %d", q, b.N)
	}
}
