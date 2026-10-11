package media

import (
	"testing"

	"github.com/pion/rtp/codecs"
)

// keyframe360 is the start of a VP8 keyframe of 640x360: frame tag (key,
// shown), start code, width and height (14 bits, little endian).
var keyframe360 = []byte{0x50, 0x2a, 0x01, 0x9d, 0x01, 0x2a, 0x80, 0x02, 0x68, 0x01, 0xaa}

func TestParseVP8_Descriptors(t *testing.T) {
	cases := []struct {
		name       string
		descriptor []byte
		start      bool
	}{
		{"minimal, start", []byte{0x10}, true},
		{"minimal, continuation", []byte{0x00}, false},
		{"start of partition 1", []byte{0x11}, false},
		{"7-bit picture ID", []byte{0x90, 0x80, 0x05}, true},
		{"15-bit picture ID", []byte{0x90, 0x80, 0x81, 0x23}, true},
		{"picture ID, TL0PICIDX, TID/KEYIDX", []byte{0x90, 0xf0, 0x81, 0x23, 0x07, 0x40}, true},
		{"TL0PICIDX only", []byte{0x90, 0x40, 0x07}, true},
		{"KEYIDX only", []byte{0x90, 0x10, 0x01}, true},
		{"extension without fields", []byte{0x90, 0x00}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload := append(append([]byte(nil), c.descriptor...), keyframe360...)
			info, ok := parseVP8(payload)
			if !ok {
				t.Fatal("not parsed")
			}
			if info.frameStart != c.start {
				t.Fatalf("frameStart %v, want %v", info.frameStart, c.start)
			}
			if !c.start {
				if info.keyframe || info.height != 0 {
					t.Fatalf("frame header read outside a frame start: %+v", info)
				}
				return
			}
			if !info.keyframe || info.width != 640 || info.height != 360 {
				t.Fatalf("got %+v, want keyframe 640x360", info)
			}
		})
	}
}

func TestParseVP8_FrameHeaders(t *testing.T) {
	// Interframe: bit 0 of the frame tag is 1.
	info, ok := parseVP8([]byte{0x10, 0x31, 0x02, 0x00})
	if !ok || !info.frameStart || info.keyframe || info.height != 0 {
		t.Fatalf("interframe: %+v ok=%v", info, ok)
	}
	// Scale bits (top 2 bits of each dimension) are masked.
	scaled := append([]byte{0x10}, keyframe360...)
	scaled[8] |= 0xc0
	scaled[10] |= 0x80
	if info, _ := parseVP8(scaled); info.width != 640 || info.height != 360 {
		t.Fatalf("scaled keyframe: %+v", info)
	}
	// Keyframe without a valid start code: a keyframe of unknown size.
	bad := append([]byte{0x10}, keyframe360...)
	bad[4] = 0
	if info, ok := parseVP8(bad); !ok || !info.keyframe || info.width != 0 || info.height != 0 {
		t.Fatalf("bad start code: %+v ok=%v", info, ok)
	}
	// Keyframe too short for the size.
	if info, ok := parseVP8([]byte{0x10, 0x50, 0x2a, 0x01}); !ok || !info.keyframe || info.height != 0 {
		t.Fatalf("short keyframe: %+v ok=%v", info, ok)
	}
}

func TestParseVP8_Truncated(t *testing.T) {
	for _, p := range [][]byte{nil, {}, {0x90}, {0x90, 0x80}, {0x90, 0x80, 0x81}, {0x90, 0x40}, {0x90, 0x20}, {0x10}} {
		if _, ok := parseVP8(p); ok {
			t.Errorf("payload %x parsed", p)
		}
	}
}

func TestParseVP8_PionPackets(t *testing.T) {
	c, err := LoadContent(KindVideo)
	if err != nil {
		t.Fatal(err)
	}
	payloader := &codecs.VP8Payloader{EnablePictureID: true}
	for frame := 0; frame < 2; frame++ {
		payloads := payloader.Payload(300, c.Frames[frame].Data)
		if len(payloads) < 2 {
			t.Fatalf("frame %d: expected several packets, got %d", frame, len(payloads))
		}
		for i, p := range payloads {
			info, ok := parseVP8(p)
			if !ok {
				t.Fatalf("frame %d packet %d: not parsed", frame, i)
			}
			if info.frameStart != (i == 0) {
				t.Fatalf("frame %d packet %d: frameStart %v", frame, i, info.frameStart)
			}
			if i == 0 && frame == 0 && (!info.keyframe || info.width != 640 || info.height != 360) {
				t.Fatalf("first frame: %+v, want keyframe 640x360", info)
			}
			if i == 0 && frame == 1 && info.keyframe {
				t.Fatal("second frame parsed as a keyframe")
			}
		}
	}
}
