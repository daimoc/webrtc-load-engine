#!/usr/bin/env bash
# Generates the media content embedded by src/media (see
# specs/004-synthetic-media/research.md R1). Run once from the repository
# root; the output files are committed. Needs Docker (ffmpeg runs from its
# image, so it does not have to be installed).
set -euo pipefail

FFMPEG_IMAGE=jrottenberg/ffmpeg:7.1-ubuntu
OUT=src/media/assets
mkdir -p "$OUT"

ffmpeg() {
	docker run --rm --user "$(id -u):$(id -g)" -v "$PWD":/w -w /w "$FFMPEG_IMAGE" -hide_banner -loglevel error -y "$@"
}

# VP8, 640x360, 30 fps, ~500 kbit/s CBR, a keyframe every 30 frames, 10 s of
# a moving test pattern. No alt-ref frames, so every IVF frame is shown and
# frames can be paced at a fixed 1/30 s.
ffmpeg -f lavfi -i testsrc2=size=640x360:rate=30:duration=10 \
	-c:v libvpx -pix_fmt yuv420p -b:v 500k -minrate 500k -maxrate 500k -bufsize 250k \
	-g 30 -keyint_min 30 -auto-alt-ref 0 -lag-in-frames 0 -deadline good -cpu-used 4 \
	-map_metadata -1 -fflags +bitexact -f ivf "$OUT/video.ivf"

# Opus, 48 kHz mono, 20 ms frames, 32 kbit/s, 10 s of a 440 Hz tone with a
# beep every second (never silent). One packet per Ogg page, as Pion's
# oggreader returns one page at a time.
ffmpeg -f lavfi -i "sine=frequency=440:beep_factor=4:sample_rate=48000:duration=10" \
	-ac 1 -c:a libopus -b:a 32k -frame_duration 20 -application audio -page_duration 20000 \
	-map_metadata -1 -fflags +bitexact "$OUT/audio.ogg"

ls -l "$OUT"
