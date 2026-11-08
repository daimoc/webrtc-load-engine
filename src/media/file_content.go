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
	return LoadFileAs(ctx, path, audio, heights, CodecVP8)
}

// LoadFileAs is LoadFile with the video in codec c, encoded as
// scripts/gen-media.sh encodes the built-in content of that codec, at its
// bitrate for each height (content.json; specs/014 FR-010). Scalable layers
// cannot be made from a file: VP9 and AV1 files are single-stream or
// simulcast layers.
func LoadFileAs(ctx context.Context, path string, audio bool, heights []int, c VideoCodec) (*FileContent, error) {
	if c == "" {
		c = CodecVP8
	}
	if _, err := ParseVideoCodec(string(c)); err != nil {
		return nil, err
	}
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
	key := fmt.Sprintf("%s|%d|%d|%t|%v|%s", path, info.Size(), info.ModTime().UnixNano(), audio, heights, c)

	filesMu.Lock()
	defer filesMu.Unlock()
	if c := files[key]; c != nil {
		return c, nil
	}
	fc, err := transcodeFile(ctx, path, audio, heights, c)
	if err != nil {
		return nil, err
	}
	files[key] = fc
	return fc, nil
}

// fileBitrate is a file's video bitrate in kbit/s at a height: the built-in
// content's in that codec.
func fileBitrate(c VideoCodec, height int) int {
	if m, err := loadManifest(); err == nil {
		if e, err := m.lookup(c, ArrangementSingle, height); err == nil {
			return e.BitrateKbps
		}
	}
	return videoBitrates[height]
}

// encoderArgs are the ffmpeg encoder options of codec c at k kbit/s:
// scripts/gen-media.sh's. Every frame is shown, keyframes every 30 frames.
func encoderArgs(c VideoCodec, k int) []string {
	rate := []string{"-pix_fmt", "yuv420p", "-b:v", fmt.Sprintf("%dk", k), "-minrate", fmt.Sprintf("%dk", k),
		"-maxrate", fmt.Sprintf("%dk", k), "-bufsize", fmt.Sprintf("%dk", k/2),
		"-g", "30", "-keyint_min", "30"}
	switch c {
	case CodecVP9:
		return append(append([]string{"-c:v", "libvpx-vp9", "-profile:v", "0"}, rate...),
			"-force_key_frames", "expr:eq(mod(n,30),0)", "-auto-alt-ref", "0", "-lag-in-frames", "0", "-deadline", "good", "-cpu-used", "4", "-row-mt", "1")
	case CodecAV1:
		return append(append([]string{"-c:v", "libaom-av1", "-usage", "realtime", "-cpu-used", "8"}, rate...),
			"-force_key_frames", "expr:eq(mod(n,30),0)", "-lag-in-frames", "0", "-row-mt", "1")
	case CodecH264:
		return append(append([]string{"-c:v", "libx264", "-profile:v", "baseline", "-level", "3.1", "-preset", "medium", "-tune", "zerolatency"}, rate...),
			"-force_key_frames", "expr:eq(mod(n,30),0)", "-bf", "0", "-sc_threshold", "0", "-x264-params", "repeat-headers=1:aud=1")
	default:
		// Keyframes at the same frames in every layer, no alt-ref frames
		// (see scripts/gen-media.sh).
		return append(append([]string{"-c:v", "libvpx"}, rate...),
			"-auto-alt-ref", "0", "-lag-in-frames", "0", "-deadline", "good", "-cpu-used", "4")
	}
}

// transcodeFile runs one ffmpeg for every output, in a temporary directory
// removed once the outputs are read.
func transcodeFile(ctx context.Context, path string, audio bool, heights []int, c VideoCodec) (*FileContent, error) {
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
			"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2,pad=%d:%d:(ow-iw)/2:(oh-ih)/2,setsar=1,fps=30", w, h, w, h))
		args = append(args, encoderArgs(c, fileBitrate(c, h))...)
		format := "ivf"
		if c == CodecH264 {
			format = "h264"
		}
		args = append(args, "-map_metadata", "-1", "-f", format, videoFileName(dir, h))
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
		var v *Content
		if c == CodecH264 {
			v, err = parseH264(data, h)
		} else {
			v, err = parseIVFAs(c, data)
		}
		if err != nil {
			return nil, fmt.Errorf("media file, %d-line video: %w", h, err)
		}
		v.Heights = []int{h}
		out.Video = append(out.Video, v)
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
