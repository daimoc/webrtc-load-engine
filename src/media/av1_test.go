package media

import (
	"bytes"
	"testing"

	"github.com/pion/rtp/codecs"
)

// av1Reassemble depacketizes a frame's packets with Pion's AV1 depacketizer
// and returns its OBUs in the low overhead format.
func av1Reassemble(t *testing.T, payloads [][]byte) []byte {
	t.Helper()
	var (
		out []byte
		buf []byte // An OBU element continued across packets
	)
	for i, b := range payloads {
		var pkt codecs.AV1Packet // Pion's keeps its elements once parsed
		if _, err := pkt.Unmarshal(b); err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}
		for k, e := range pkt.OBUElements {
			if k == 0 && pkt.Z {
				buf = append(buf, e...)
			} else {
				buf = append([]byte(nil), e...)
			}
			if k == len(pkt.OBUElements)-1 && pkt.Y {
				continue // Continues in the next packet
			}
			if buf[0]&obuHasSizeField != 0 {
				t.Fatalf("packet %d element %d: OBU sent with its size field", i, k)
			}
			obus, err := splitOBUs(buf) // One OBU without a size field
			if err != nil || len(obus) != 1 {
				t.Fatalf("packet %d element %d: %v", i, k, err)
			}
			out = append(out, joinOBUs(obus)...)
			buf = nil
		}
	}
	return out
}

func TestAV1Payloader_RoundTrip(t *testing.T) {
	c := loadSingle(t, CodecAV1)
	p := &av1Payloader{}
	const mtu = 1194
	for i, f := range c.Frames[:61] {
		payloads := p.Payload(mtu, f.Data)
		for j, b := range payloads {
			if len(b) > mtu {
				t.Fatalf("frame %d packet %d: %d bytes", i, j, len(b))
			}
			var pkt codecs.AV1Packet
			if _, err := pkt.Unmarshal(b); err != nil {
				t.Fatal(err)
			}
			if pkt.N != (i%30 == 0 && j == 0) {
				t.Fatalf("frame %d packet %d: N=%v", i, j, pkt.N)
			}
			if j == 0 && pkt.Z {
				t.Fatalf("frame %d: first packet continues an OBU", i)
			}
			if j == len(payloads)-1 && pkt.Y {
				t.Fatalf("frame %d: last packet continues in the next", i)
			}
		}
		if got := av1Reassemble(t, payloads); !bytes.Equal(got, f.Data) {
			t.Fatalf("frame %d: reassembled %d bytes, want %d", i, len(got), len(f.Data))
		}
	}
}

func TestAV1Payloader_Fragments(t *testing.T) {
	big := joinOBUs([]obu{{header: []byte{obuFrame << 3}, payload: bytes.Repeat([]byte{0xab}, 5000)}})
	payloads := (&av1Payloader{}).Payload(1000, big)
	if len(payloads) < 6 {
		t.Fatalf("%d packets for 5000 bytes at MTU 1000", len(payloads))
	}
	for j, b := range payloads {
		var pkt codecs.AV1Packet
		if _, err := pkt.Unmarshal(b); err != nil {
			t.Fatal(err)
		}
		if pkt.Z != (j > 0) || pkt.Y != (j < len(payloads)-1) {
			t.Fatalf("packet %d: Z=%v Y=%v", j, pkt.Z, pkt.Y)
		}
	}
	if got := av1Reassemble(t, payloads); !bytes.Equal(got, big) {
		t.Fatal("fragments do not rebuild the OBU")
	}
}
