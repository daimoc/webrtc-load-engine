package media

import (
	"testing"
	"time"
)

func TestContent_Video(t *testing.T) {
	c, err := LoadContent(KindVideo)
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if c.Codec.MimeType != "video/VP8" || c.Codec.ClockRate != 90000 {
		t.Fatalf("unexpected codec %+v", c.Codec)
	}
	if n := len(c.Frames); n < 299 || n > 301 {
		t.Fatalf("expected 300 ±1 frames, got %d", n)
	}
	if !isVP8Keyframe(c.Frames[0].Data) {
		t.Fatal("first frame is not a keyframe")
	}
	if c.LoopDuration < 9900*time.Millisecond || c.LoopDuration > 10100*time.Millisecond {
		t.Fatalf("expected a 10 s loop, got %s", c.LoopDuration)
	}
	if c.Frames[1].Samples != 3000 {
		t.Fatalf("expected 3000 RTP ticks per frame at 30 fps, got %d", c.Frames[1].Samples)
	}
	if c.Width != 640 || c.Height != 360 {
		t.Fatalf("expected 640×360 from the IVF header, got %d×%d", c.Width, c.Height)
	}
}

func TestContent_Audio(t *testing.T) {
	c, err := LoadContent(KindAudio)
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if c.Codec.MimeType != "audio/opus" || c.Codec.ClockRate != 48000 || c.Codec.Channels != 2 {
		t.Fatalf("unexpected codec %+v", c.Codec)
	}
	if n := len(c.Frames); n < 499 || n > 501 {
		t.Fatalf("expected 500 ±1 packets, got %d", n)
	}
	for i, f := range c.Frames {
		if f.Duration != 20*time.Millisecond || f.Samples != 960 || len(f.Data) == 0 {
			t.Fatalf("packet %d: expected 20 ms / 960 samples, got %s / %d (%d bytes)", i, f.Duration, f.Samples, len(f.Data))
		}
	}
}

func TestContent_UnknownKind(t *testing.T) {
	if _, err := LoadContent("screen"); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}

func TestLoadLayers(t *testing.T) {
	for maxHeight, want := range map[int][]int{360: {90, 180, 360}, 720: {180, 360, 720}} {
		layers, err := LoadLayers(maxHeight)
		if err != nil {
			t.Fatalf("LoadLayers(%d): %v", maxHeight, err)
		}
		if len(layers) != 3 {
			t.Fatalf("LoadLayers(%d): %d layers", maxHeight, len(layers))
		}
		for i, c := range layers {
			_, h := keyframeSize(c.Frames[0].Data)
			if h != want[i] {
				t.Errorf("LoadLayers(%d) layer %d: height %d, want %d", maxHeight, i, h, want[i])
			}
			for j, f := range c.Frames {
				if f.Samples != layers[0].Frames[j].Samples {
					t.Fatalf("layer %d frame %d: %d ticks, base %d", i, j, f.Samples, layers[0].Frames[j].Samples)
				}
				if key := isVP8Keyframe(f.Data); key != (j%30 == 0) {
					t.Fatalf("layer %d frame %d: keyframe %v", i, j, key)
				}
			}
		}
	}
	if _, err := LoadLayers(480); err == nil {
		t.Fatal("LoadLayers(480) succeeded")
	}
}
