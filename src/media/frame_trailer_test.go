package media

import (
	"bytes"
	"testing"

	"github.com/pion/rtp/codecs"
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

// Each codec carries the trailer where its decoders skip it (specs/014
// data-model "Frame trailer carrier"), and reads it back.
func TestTrailerCarrier_RoundTrip(t *testing.T) {
	tag := scenarioTag("carrier")
	other := scenarioTag("someone else")
	for _, c := range VideoCodecs() {
		for _, n := range []uint32{1, 0x000001, 0x00ffff, 0x123456, frameNumberMask} {
			payloads := addTrailer(c, [][]byte{{0x10, 1, 2}, {0x10, 3, 4}}, tag, n)
			last := payloads[len(payloads)-1]
			got, ok := readWireTrailer(c, last, tag)
			if !ok || got != n {
				t.Fatalf("%s frame %#x: read %#x, %v", c, n, got, ok)
			}
			if _, ok := readWireTrailer(c, last, other); ok {
				t.Fatalf("%s: another scenario's tag read as ours", c)
			}
			if _, ok := readTrailer(c, payloads[0], nil, tag); ok {
				t.Fatalf("%s: a packet without a trailer read as one", c)
			}
			switch c {
			case CodecVP9:
				// RTP padding (RFC 3550 5.1): the trailer, then the padding
				// length; the frame data before it is unchanged, so libvpx
				// never sees the trailer (014 T051).
				if !trailerIsPadding(c) || last[len(last)-1] != frameTrailerSize+1 || !bytes.Equal(last[:3], []byte{0x10, 3, 4}) {
					t.Fatalf("VP9 last payload % x", last)
				}
				if _, ok := readTrailer(c, last, nil, tag); ok {
					t.Fatal("VP9: a trailer read from frame data without padding")
				}
			case CodecH264:
				if len(payloads) != 3 || nalType(last) != nalSEI {
					t.Fatalf("H.264: the trailer is not its own SEI packet: %d packets, type %d", len(payloads), nalType(last))
				}
				if bytes.Contains(last, []byte{0, 0, 0}) || bytes.Contains(last, []byte{0, 0, 1}) || bytes.Contains(last, []byte{0, 0, 2}) {
					t.Fatalf("H.264 SEI % x without emulation prevention", last)
				}
			}
		}
	}
}

// AV1's trailer is a padding OBU, the last element of the last packet:
// Pion's depacketizer sees it as an OBU element of type OBU_PADDING.
func TestTrailerCarrier_AV1Padding(t *testing.T) {
	c := loadSingle(t, CodecAV1)
	payloads := (&av1Payloader{}).Payload(1000, c.Frames[1].Data)
	payloads = addTrailer(CodecAV1, payloads, scenarioTag("x"), 7)
	var pkt codecs.AV1Packet
	if _, err := pkt.Unmarshal(payloads[len(payloads)-1]); err != nil {
		t.Fatal(err)
	}
	last := pkt.OBUElements[len(pkt.OBUElements)-1]
	if int(last[0]>>3)&0x0f != obuPadding || len(last) != 7 {
		t.Fatalf("last element % x, want a 7-byte padding OBU", last)
	}
}

// What the trailer costs in each codec's packets.
func TestTrailerRoom(t *testing.T) {
	for c, want := range map[VideoCodec]int{CodecVP8: 6, CodecVP9: 7, CodecAV1: 8, CodecH264: 0} {
		if got := trailerRoom(c); got != want {
			t.Errorf("%s: %d bytes, want %d", c, got, want)
		}
	}
}

// readWireTrailer reads the trailer of a sent payload as it goes on the
// wire: for codecs carrying it as RTP padding, the payload ends with the
// padding.
func readWireTrailer(c VideoCodec, wire []byte, tag [3]byte) (uint32, bool) {
	if trailerIsPadding(c) && len(wire) > 0 && int(wire[len(wire)-1]) <= len(wire) {
		k := len(wire) - int(wire[len(wire)-1])
		return readTrailer(c, wire[:k], wire[k:], tag)
	}
	return readTrailer(c, wire, nil, tag)
}
