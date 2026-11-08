package media

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/pion/rtp/codecs"
)

func loadSingle(t *testing.T, c VideoCodec) *Content {
	t.Helper()
	v, err := LoadVideo(c, ArrangementSingle, 0)
	if err != nil {
		t.Fatal(err)
	}
	return v[0]
}

func TestVP9Payloader_SingleLayer(t *testing.T) {
	c := loadSingle(t, CodecVP9)
	p := newVP9Payloader(c.Width, c.Height)
	const mtu = 1194
	var lastPID uint16
	for i, f := range c.Frames[:61] {
		payloads := p.Payload(mtu, f.Data)
		if len(payloads) == 0 {
			t.Fatalf("frame %d: no packets", i)
		}
		var data []byte
		for j, b := range payloads {
			if len(b) > mtu {
				t.Fatalf("frame %d packet %d: %d bytes > MTU %d", i, j, len(b), mtu)
			}
			var pkt codecs.VP9Packet
			if _, err := pkt.Unmarshal(b); err != nil {
				t.Fatalf("frame %d packet %d: %v", i, j, err)
			}
			key := i%30 == 0
			if !pkt.I || !pkt.F || pkt.L || pkt.P == key || pkt.B != (j == 0) || pkt.E != (j == len(payloads)-1) {
				t.Fatalf("frame %d packet %d: I=%v F=%v L=%v P=%v B=%v E=%v", i, j, pkt.I, pkt.F, pkt.L, pkt.P, pkt.B, pkt.E)
			}
			if !key && (len(pkt.PDiff) != 1 || pkt.PDiff[0] != 1) {
				t.Fatalf("frame %d: P_DIFF %v, want [1]", i, pkt.PDiff)
			}
			if pkt.V != (key && j == 0) {
				t.Fatalf("frame %d packet %d: V=%v", i, j, pkt.V)
			}
			if pkt.V && (pkt.NS != 0 || !pkt.Y || pkt.Width[0] != 640 || pkt.Height[0] != 360) {
				t.Fatalf("frame %d: scalability structure NS=%d Y=%v %v×%v", i, pkt.NS, pkt.Y, pkt.Width, pkt.Height)
			}
			if j == 0 && i > 0 && pkt.PictureID != (lastPID+1)&0x7fff {
				t.Fatalf("frame %d: picture ID %d after %d", i, pkt.PictureID, lastPID)
			}
			lastPID = pkt.PictureID
			data = append(data, pkt.Payload...)
		}
		if !bytes.Equal(data, f.Data) {
			t.Fatalf("frame %d: packets do not rebuild the frame", i)
		}
	}
}

func loadSVC(t *testing.T, c VideoCodec, maxHeight int) *Content {
	t.Helper()
	v, err := LoadVideo(c, ArrangementSVC, maxHeight)
	if err != nil {
		t.Fatal(err)
	}
	return v[0]
}

// svcPDiff is the picture distance to the reference of a layer frame of
// temporal layer tid in the 0-2-1-2 pattern: a T0 frame refers to the T0
// four pictures back, T1 to the T0 two back, T2 to the picture before.
var svcPDiff = [3]uint8{4, 2, 1}

func TestVP9Payloader_Layers(t *testing.T) {
	c := loadSVC(t, CodecVP9, 720)
	widths, heights := []int{320, 640, 1280}, []int{180, 360, 720}
	p := newVP9SVCPayloader(widths, heights)
	const mtu = 1194
	var lastPID uint16
	for i, f := range c.Frames[:41] {
		key := i%20 == 0
		layers := p.PayloadLayers(mtu, f.Layers)
		if len(layers) != 3 {
			t.Fatalf("picture %d: %d layer frames", i, len(layers))
		}
		for s, payloads := range layers {
			var data []byte
			for j, b := range payloads {
				if len(b) > mtu {
					t.Fatalf("picture %d layer %d packet %d: %d bytes", i, s, j, len(b))
				}
				var pkt codecs.VP9Packet
				if _, err := pkt.Unmarshal(b); err != nil {
					t.Fatalf("picture %d layer %d packet %d: %v", i, s, j, err)
				}
				name := func() string { return "picture " + itoa(i) + " layer " + itoa(s) + " packet " + itoa(j) }
				if !pkt.I || !pkt.F || !pkt.L || pkt.B != (j == 0) || pkt.E != (j == len(payloads)-1) {
					t.Fatalf("%s: I=%v F=%v L=%v B=%v E=%v", name(), pkt.I, pkt.F, pkt.L, pkt.B, pkt.E)
				}
				if int(pkt.SID) != s || int(pkt.TID) != f.Layers[s].TemporalID || !pkt.U {
					t.Fatalf("%s: SID %d TID %d U %v", name(), pkt.SID, pkt.TID, pkt.U)
				}
				// L3T3_KEY: on a keyframe picture the upper layers predict
				// from the layer below and nothing earlier; otherwise each
				// layer only from its own earlier frames.
				if pkt.D != (key && s > 0) || pkt.P == key {
					t.Fatalf("%s: D=%v P=%v", name(), pkt.D, pkt.P)
				}
				if !key && (len(pkt.PDiff) != 1 || pkt.PDiff[0] != svcPDiff[f.Layers[s].TemporalID]) {
					t.Fatalf("%s: P_DIFF %v", name(), pkt.PDiff)
				}
				if pkt.Z != (!key || s == 2) {
					t.Fatalf("%s: Z=%v", name(), pkt.Z)
				}
				if pkt.V != (key && s == 0 && j == 0) {
					t.Fatalf("%s: V=%v", name(), pkt.V)
				}
				if pkt.V && (pkt.NS != 2 || !pkt.Y || !equalInts(pkt.Width, widths) || !equalInts(pkt.Height, heights)) {
					t.Fatalf("%s: scalability structure NS=%d %v×%v", name(), pkt.NS, pkt.Width, pkt.Height)
				}
				if i > 0 && s == 0 && j == 0 && pkt.PictureID != (lastPID+1)&0x7fff {
					t.Fatalf("%s: picture ID %d after %d", name(), pkt.PictureID, lastPID)
				}
				if s > 0 && pkt.PictureID != lastPID {
					t.Fatalf("%s: picture ID %d, the picture's is %d", name(), pkt.PictureID, lastPID)
				}
				lastPID = pkt.PictureID
				data = append(data, pkt.Payload...)
			}
			if !bytes.Equal(data, f.Layers[s].Data) {
				t.Fatalf("picture %d layer %d: packets do not rebuild the layer frame", i, s)
			}
		}
	}
}

func equalInts[T ~uint16 | ~int](a []T, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if int(a[i]) != b[i] {
			return false
		}
	}
	return true
}

func itoa(i int) string { return strconv.Itoa(i) }
