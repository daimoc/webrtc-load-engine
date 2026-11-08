package media

import (
	"testing"
	"time"

	"github.com/pion/rtp"
)

// sourcePackets runs the codec's single-stream content through a Source
// for a bit over one second and returns the packets, split into frames.
func sourcePackets(t *testing.T, c VideoCodec) [][]rtp.Packet {
	t.Helper()
	return sourcePacketsFor(t, c, 1100*time.Millisecond)
}

func sourcePacketsFor(t *testing.T, c VideoCodec, d time.Duration) [][]rtp.Packet {
	t.Helper()
	w := &recorder{}
	runSource(t, "parse-"+t.Name(), loadSingle(t, c), d, nil, w)
	packets, _ := w.snapshot()
	frames := splitFrames(packets)
	if len(frames) < 33 {
		t.Fatalf("%s: %d frames", c, len(frames))
	}
	return frames
}

// Each codec's parser finds the frame starts, keyframes and the size, from
// the packets the bots send (research R8).
func TestFrameParser_EveryCodec(t *testing.T) {
	for _, c := range VideoCodecs() {
		for _, withDD := range []bool{true, false} {
			if withDD && c != CodecAV1 {
				continue
			}
			ddID := uint8(0)
			if withDD {
				ddID = SourceDDExtID
			}
			frames := sourcePackets(t, c)
			p := NewFrameParser(c, ddID)
			for i, f := range frames[:33] {
				key := i%30 == 0
				for j := range f {
					d, ok := p.Parse(&f[j].Header, f[j].Payload)
					if !ok {
						t.Fatalf("%s dd=%v frame %d packet %d: not parsed", c, withDD, i, j)
					}
					first := j == 0
					if first && d.Keyframe != key {
						t.Fatalf("%s dd=%v frame %d: keyframe %v", c, withDD, i, d.Keyframe)
					}
					if !first && d.Keyframe {
						t.Fatalf("%s dd=%v frame %d packet %d: keyframe on a later packet", c, withDD, i, j)
					}
					// AV1 without the descriptor has nothing marking a
					// frame's start: the timestamp change does.
					if (c != CodecAV1 || withDD) && d.Start != first {
						t.Fatalf("%s dd=%v frame %d packet %d: start %v", c, withDD, i, j, d.Start)
					}
					if first && key && (d.Width != 640 || d.Height != 360) {
						t.Fatalf("%s dd=%v frame %d: size %dx%d, want 640x360", c, withDD, i, d.Width, d.Height)
					}
					if d.SpatialID != 0 || d.TemporalID != 0 {
						t.Fatalf("%s frame %d: layer %d/%d", c, i, d.SpatialID, d.TemporalID)
					}
				}
			}
		}
	}
}

// The VP8 parser gives what parseVP8 gives (VP8 results unchanged, FR-011).
func TestFrameParser_VP8IsParseVP8(t *testing.T) {
	p := NewFrameParser(CodecVP8, 0)
	for _, f := range sourcePackets(t, CodecVP8) {
		for _, pkt := range f {
			want, wok := parseVP8(pkt.Payload)
			got, ok := p.Parse(&pkt.Header, pkt.Payload)
			if ok != wok || got.Start != want.frameStart || got.Keyframe != want.keyframe || got.Width != want.width || got.Height != want.height {
				t.Fatalf("got %+v %v, parseVP8 %+v %v", got, ok, want, wok)
			}
		}
	}
}

func TestH264SPSSize(t *testing.T) {
	for _, h := range []int{90, 180, 360, 720} {
		layers, err := LoadVideo(CodecH264, ArrangementSimulcast, map[int]int{90: 360, 180: 360, 360: 720, 720: 720}[h])
		if err != nil {
			t.Fatal(err)
		}
		var c *Content
		for _, l := range layers {
			if l.Heights[0] == h {
				c = l
			}
		}
		if c == nil {
			continue
		}
		for _, n := range splitNALs(c.Frames[0].Data) {
			if nalType(n) == nalSPS {
				w, gh := h264SPSSize(n)
				if gh != h || w != h*16/9 {
					t.Errorf("%d lines: SPS says %dx%d", h, w, gh)
				}
			}
		}
	}
}
