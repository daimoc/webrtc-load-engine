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
