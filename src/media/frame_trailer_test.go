package media

import (
	"bytes"
	"testing"
)

func TestFrameTrailer_Layout(t *testing.T) {
	tag := [3]byte{0xa1, 0xb2, 0xc3}
	got := appendFrameTrailer([]byte{1, 2}, tag, 258)
	want := []byte{1, 2, 0xa1, 0xb2, 0xc3, 0x00, 0x01, 0x02}
	if !bytes.Equal(got, want) {
		t.Fatalf("got % x, want % x", got, want)
	}
}

func TestFrameTrailer_KeepsTwentyFourBits(t *testing.T) {
	tag := [3]byte{1, 2, 3}
	if n, ok := readFrameTrailer(appendFrameTrailer(nil, tag, 1<<24+5), tag); !ok || n != 5 {
		t.Fatalf("got %d, %v, want 5, true", n, ok)
	}
}

func TestFrameTrailer_Read(t *testing.T) {
	tag := [3]byte{0xa1, 0xb2, 0xc3}
	p := appendFrameTrailer([]byte{9, 9, 9}, tag, 258)
	if n, ok := readFrameTrailer(p, tag); !ok || n != 258 {
		t.Fatalf("got %d, %v, want 258, true", n, ok)
	}
	if _, ok := readFrameTrailer(p, [3]byte{0xa1, 0xb2, 0xc4}); ok {
		t.Error("matched a wrong tag")
	}
	if _, ok := readFrameTrailer(p[3:len(p)-1], tag); ok {
		t.Error("matched a payload under 6 bytes")
	}
	if _, ok := readFrameTrailer(nil, tag); ok {
		t.Error("matched an empty payload")
	}
}

func TestScenarioTag(t *testing.T) {
	a := "0b1f4c52-3a53-4d16-8a56-5d2f9c0f4a11"
	b := "7c9e6679-7425-40de-944b-e07fc1f90ae7"
	if scenarioTag(a) != scenarioTag(a) {
		t.Error("tag changes for one ID")
	}
	if scenarioTag(a) == scenarioTag(b) {
		t.Error("two IDs share a tag")
	}
	// 32-bit FNV-1a of "a" is 0xe40c292c.
	if got, want := scenarioTag("a"), [3]byte{0xe4, 0x0c, 0x29}; got != want {
		t.Errorf("tag % x, want % x", got, want)
	}
}

func TestNewerFrame(t *testing.T) {
	for _, c := range []struct {
		n, last uint32
		want    bool
	}{
		{2, 1, true},
		{1, 2, false},
		{5, 5, false},
		{0, 16777215, true},
		{16777215, 0, false},
	} {
		if got := newerFrame(c.n, c.last); got != c.want {
			t.Errorf("newerFrame(%d, %d) = %v, want %v", c.n, c.last, got, c.want)
		}
	}
}
