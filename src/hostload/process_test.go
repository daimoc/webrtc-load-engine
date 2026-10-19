package hostload

import (
	"runtime"
	"testing"
	"time"
)

// The counter advances with CPU work, in nanoseconds. Busy work is timed by
// the counter itself, not by the wall clock: on a loaded machine (the whole
// suite in parallel) a 50 ms wall-clock loop gets less than 50 ms of CPU.
func TestProcessSample_CPU(t *testing.T) {
	before, _, err := processSample()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	x := 0
	for {
		for i := 0; i < 1e5; i++ {
			x++
		}
		now, _, err := processSample()
		if err != nil {
			t.Fatal(err)
		}
		if time.Duration(now-before) >= 50*time.Millisecond {
			break
		}
		if time.Since(start) > 5*time.Second {
			t.Fatalf("CPU counter advanced %v in 5 s of busy work (x=%d)", time.Duration(now-before), x)
		}
	}
	// 50 ms of CPU take at least 50 ms divided by the threads that ran:
	// a counter in the wrong unit would be off by 1000.
	if wall := time.Since(start); wall < 50*time.Millisecond/time.Duration(runtime.NumCPU()) {
		t.Errorf("50 ms of CPU counted in %v of wall time", wall)
	}
}

func TestProcessSample_RSS(t *testing.T) {
	_, before, err := processSample()
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64<<20)
	for i := range buf {
		buf[i] = byte(i)
	}
	_, after, err := processSample()
	if err != nil {
		t.Fatal(err)
	}
	if after-before < 48<<20 {
		t.Errorf("RSS grew by %d bytes after touching 64 MB", after-before)
	}
	runtime.KeepAlive(buf)
}

func TestParseCPUList(t *testing.T) {
	for in, want := range map[string]int{
		"0-15\n":   16,
		"0":        1,
		"0-3,8-11": 8,
		"0,2,4-5":  4,
		"":         0,
		"garbage":  0,
	} {
		if got := parseCPUList(in); got != want {
			t.Errorf("parseCPUList(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestThreads(t *testing.T) {
	if h := hardwareThreads(); h < 1 {
		t.Errorf("hardware threads %d", h)
	}
	if u := usableThreads(); u != runtime.NumCPU() || u > hardwareThreads() {
		t.Errorf("usable threads %d (NumCPU %d, hardware %d)", u, runtime.NumCPU(), hardwareThreads())
	}
}
