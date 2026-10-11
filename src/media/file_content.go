package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// FFmpeg is the command that transcodes media files (LoadFile). It is looked
// up in PATH unless it holds a path.
var FFmpeg = "ffmpeg"

// FileContent is a media file transcoded for the bots, with the parameters
// of the built-in content (scripts/gen-media.sh): VP8 at 30 fps with a
// keyframe every 30 frames, and Opus 48 kHz mono in 20 ms packets. It is
// sent and measured like the built-in content.
type FileContent struct {
	Audio *Content   // nil when audio was not asked for
	Video []*Content // One layer per height asked for, low to high; nil when none
}

// Video bitrates by height in kbit/s, as for the built-in content.
var videoBitrates = map[int]int{90: 60, 180: 150, 360: 500, 720: 1500}

var (
	filesMu sync.Mutex
	files   = map[string]*FileContent{} // By fileKey
)

// LoadFile transcodes the media file at path with ffmpeg: its first audio
// stream when audio is true, and its first video stream once per height in
// heights (low to high; 90, 180, 360 or 720 lines, 16:9, the picture scaled
// to fit and padded). The result is kept for the process while the file's
// size and modification time stay the same, so a scenario run again does not
// transcode again.
func LoadFile(ctx context.Context, path string, audio bool, heights []int) (*FileContent, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("media file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("media file %s is not a regular file", path)
	}
	for _, h := range heights {
		if videoBitrates[h] == 0 {
			return nil, fmt.Errorf("media file: no video height %d (90, 180, 360 or 720)", h)
		}
	}
	key := fmt.Sprintf("%s|%d|%d|%t|%v", path, info.Size(), info.ModTime().UnixNano(), audio, heights)

	filesMu.Lock()
	defer filesMu.Unlock()
	if c := files[key]; c != nil {
		return c, nil
	}
	c, err := transcodeFile(ctx, path, audio, heights)
	if err != nil {
		return nil, err
	}
	files[key] = c
	return c, nil
}

// transcodeFile runs one ffmpeg for every output, in a temporary directory
// removed once the outputs are read.
func transcodeFile(ctx context.Context, path string, audio bool, heights []int) (*FileContent, error) {
	dir, err := os.MkdirTemp("", "webrtc-load-media-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y", "-i", path}
	for _, h := range heights {
		w := h * 16 / 9
		args = append(args,
			"-map", "0:v:0", "-an", "-sn", "-dn",
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30", w, h, w, h),
			"-c:v", "libvpx", "-pix_fmt", "yuv420p",
			"-b:v", fmt.Sprintf("%dk", videoBitrates[h]), "-minrate", fmt.Sprintf("%dk", videoBitrates[h]),
			"-maxrate", fmt.Sprintf("%dk", videoBitrates[h]), "-bufsize", fmt.Sprintf("%dk", videoBitrates[h]/2),
			// Keyframes at the same frames in every layer, no alt-ref
			// frames (see scripts/gen-media.sh).
			"-g", "30", "-keyint_min", "30", "-auto-alt-ref", "0", "-lag-in-frames", "0",
			"-deadline", "good", "-cpu-used", "4",
			"-map_metadata", "-1", "-f", "ivf", videoFileName(dir, h))
	}
	if audio {
		args = append(args,
			"-map", "0:a:0", "-vn", "-sn", "-dn",
			"-ac", "1", "-ar", "48000", "-c:a", "libopus", "-b:a", "32k",
			"-frame_duration", "20", "-application", "audio", "-page_duration", "20000",
			"-map_metadata", "-1", "-f", "ogg", filepath.Join(dir, "audio.ogg"))
	}

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, FFmpeg, args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) && ctx.Err() == nil { // Not started
			return nil, fmt.Errorf("media file: transcoding needs ffmpeg (%s): %w", FFmpeg, err)
		}
		return nil, fmt.Errorf("media file: ffmpeg failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	out := &FileContent{}
	for _, h := range heights {
		data, err := os.ReadFile(videoFileName(dir, h))
		if err != nil {
			return nil, err
		}
		c, err := parseIVF(data)
		if err != nil {
			return nil, fmt.Errorf("media file, %d-line video: %w", h, err)
		}
		out.Video = append(out.Video, c)
	}
	if len(out.Video) > 1 {
		if err := checkAligned(out.Video); err != nil {
			return nil, fmt.Errorf("media file: %w", err)
		}
	}
	if audio {
		data, err := os.ReadFile(filepath.Join(dir, "audio.ogg"))
		if err != nil {
			return nil, err
		}
		if out.Audio, err = parseOgg(data); err != nil {
			return nil, fmt.Errorf("media file: %w", err)
		}
	}
	return out, nil
}

func videoFileName(dir string, height int) string {
	return filepath.Join(dir, fmt.Sprintf("video_%d.ivf", height))
}
