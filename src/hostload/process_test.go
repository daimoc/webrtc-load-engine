package hostload

import (
	"runtime"
	"testing"
	"time"
)

func TestProcessSample_CPU(t *testing.T) {
	before, _, err := processSample()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(50 * time.Millisecond)
	x := 0
	for time.Now().Before(deadline) {
		x++
	}
	after, _, err := processSample()
	if err != nil {
		t.Fatal(err)
	}
	used := time.Duration(after - before)
	// The busy loop's 50 ms, plus whatever the test runtime does meanwhile.
	if used < 40*time.Millisecond || used > 500*time.Millisecond {
		t.Errorf("CPU time over a 50 ms busy loop: %v (x=%d)", used, x)
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
