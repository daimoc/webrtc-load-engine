package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// needFFmpeg skips the test when ffmpeg is not installed.
func needFFmpeg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath(FFmpeg); err != nil {
		t.Skipf("ffmpeg not found: %v", err)
	}
}

// makeClip writes a 3 s 4:3 clip with a test pattern and a tone, in a
// container and codecs the engine does not send (H.264 and AAC would need an
// encoder build option, so MPEG-4 Part 2 and MP2 in Matroska).
func makeClip(t *testing.T, audio bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clip.mkv")
	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y",
		"-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=3"}
	if audio {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100:duration=3", "-c:a", "mp2")
	}
	args = append(args, "-c:v", "mpeg4", path)
	if out, err := exec.Command(FFmpeg, args...).CombinedOutput(); err != nil {
		t.Fatalf("making clip: %v: %s", err, out)
	}
	return path
}

func TestLoadFile_Layers(t *testing.T) {
	needFFmpeg(t)
	path := makeClip(t, true)
	heights := []int{90, 180, 360}
	c, err := LoadFile(context.Background(), path, true, heights)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Video) != len(heights) {
		t.Fatalf("%d video layers, want %d", len(c.Video), len(heights))
	}
	for i, v := range c.Video {
		if v.Codec.MimeType != "video/VP8" {
			t.Errorf("layer %d codec %s", i, v.Codec.MimeType)
		}
		// 25 fps in, 30 fps out: 90 frames of 1/30 s for 3 s.
		if n := len(v.Frames); n < 88 || n > 92 {
			t.Errorf("layer %d: %d frames, want about 90", i, n)
		}
		if d := v.Frames[0].Duration; d < 33*time.Millisecond || d > 34*time.Millisecond {
			t.Errorf("layer %d: frame duration %s, want 1/30 s", i, d)
		}
		if !isVP8Keyframe(v.Frames[0].Data) || !isVP8Keyframe(v.Frames[30].Data) || isVP8Keyframe(v.Frames[15].Data) {
			t.Errorf("layer %d: keyframes not every 30 frames", i)
		}
		// 4:3 picture padded to 16:9.
		if w, h := keyframeSize(v.Frames[0].Data); h != heights[i] || w != heights[i]*16/9 {
			t.Errorf("layer %d: %dx%d, want height %d in 16:9", i, w, h, heights[i])
		}
	}
	if c.Audio == nil || c.Audio.Codec.MimeType != "audio/opus" {
		t.Fatalf("audio %+v", c.Audio)
	}
	if d := c.Audio.LoopDuration; d < 2900*time.Millisecond || d > 3100*time.Millisecond {
		t.Errorf("audio lasts %s, want about 3 s", d)
	}

	// Loaded again: kept for the process.
	again, err := LoadFile(context.Background(), path, true, heights)
	if err != nil || again != c {
		t.Errorf("second load not reused: %v", err)
	}
}

func TestLoadFile_Errors(t *testing.T) {
	if _, err := LoadFile(context.Background(), filepath.Join(t.TempDir(), "none.mp4"), true, nil); err == nil {
		t.Error("missing file: no error")
	}
	if _, err := LoadFile(context.Background(), t.TempDir(), true, nil); err == nil {
		t.Error("directory: no error")
	}

	needFFmpeg(t)
	path := makeClip(t, false)
	if _, err := LoadFile(context.Background(), path, false, []int{480}); err == nil || !strings.Contains(err.Error(), "480") {
		t.Errorf("unknown height: %v", err)
	}
	// No audio stream when audio is asked for: ffmpeg's error is reported.
	if _, err := LoadFile(context.Background(), path, true, []int{360}); err == nil || !strings.Contains(err.Error(), "ffmpeg failed") {
		t.Errorf("no audio stream: %v", err)
	}
	if c, err := LoadFile(context.Background(), path, false, []int{360}); err != nil || c.Audio != nil || len(c.Video) != 1 {
		t.Errorf("video only: %+v, %v", c, err)
	}
}

func TestLoadFile_NoFFmpeg(t *testing.T) {
	defer func(old string) { FFmpeg = old }(FFmpeg)
	FFmpeg = filepath.Join(t.TempDir(), "no-ffmpeg")
	path := filepath.Join(t.TempDir(), "clip.mp4")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(context.Background(), path, true, nil); err == nil || !strings.Contains(err.Error(), "needs ffmpeg") {
		t.Errorf("no ffmpeg: %v", err)
	}
}
