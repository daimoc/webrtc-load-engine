#!/usr/bin/env bash
# Generates the media content embedded by src/media. Run from the repository
# root; the output files are committed. Needs Docker (ffmpeg and the SVC
# encoders run from images, so they do not have to be installed).
#
#   scripts/gen-media.sh          VP9, AV1 and H.264 single-stream and simulcast
#                                 content at the VP8 content's quality, and
#                                 src/media/assets/content.json (specs/014 R2, R3)
#   scripts/gen-media.sh h264     only the H.264 files and their entries
#   scripts/gen-media.sh svc      VP9 and AV1 scalable (L3T3_KEY) content (specs/014 R4)
#   scripts/gen-media.sh base     the VP8 content and the audio (specs/004 R1,
#                                 specs/006 R6). Not run by default: the VP8
#                                 files are the quality reference and must not
#                                 change (specs/014 FR-011).
set -euo pipefail

FFMPEG_IMAGE=jrottenberg/ffmpeg:7.1-ubuntu
SVC_IMAGE=webrtc-load-engine/svc-encoders
OUT=src/media/assets
MANIFEST=$OUT/content.json
WORK=$(mktemp -d "${TMPDIR:-/tmp}/gen-media.XXXXXX")
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$OUT"

# The VMAF model of libvmaf 3.0.0 (the image's libvmaf has no built-in one).
VMAF_MODEL_URL=https://raw.githubusercontent.com/Netflix/vmaf/v3.0.0/model/vmaf_v0.6.1.json
VMAF_MODEL_SHA256=5950d61fa1f861bd45d8149d80539ed9f3376cfc2495b8f0fa8e9f57cb131ee3
# Equal quality: a codec's VMAF within this much of the VP8 content's
# (specs/014 SC-009).
VMAF_TOLERANCE=1.0
SVC_SEARCH_TOLERANCE=0.5
MAX_ENCODES=10

# Runs ffmpeg with the repository at /w and the work folder at /t.
ffmpeg() {
	docker run --rm --user "$(id -u):$(id -g)" -v "$PWD":/w -v "$WORK":/t -w /w "$FFMPEG_IMAGE" -hide_banner -loglevel error -y "$@"
}

# The source pattern: 30 fps, 10 s.
SRC="testsrc2=rate=30:duration=10"

size_of() { # height -> WxH
	case $1 in 90) echo 160x90 ;; 180) echo 320x180 ;; 360) echo 640x360 ;; 720) echo 1280x720 ;; *) echo "bad height $1" >&2; exit 1 ;; esac
}

# VP8, 30 fps, CBR, a keyframe every 30 frames, 10 s of a moving test
# pattern. No alt-ref frames, so every IVF frame is shown and frames can be
# paced at a fixed 1/30 s. All layers come from the same pattern with
# keyframes at the same frames, so they can be sent in lockstep as
# simulcast layers (specs/006-media-quality/research.md R6).
vp8() { # size kbit/s output
	ffmpeg -f lavfi -i "testsrc2=size=$1:rate=30:duration=10" \
		-c:v libvpx -pix_fmt yuv420p -b:v "$2k" -minrate "$2k" -maxrate "$2k" -bufsize "$(($2 / 2))k" \
		-g 30 -keyint_min 30 -auto-alt-ref 0 -lag-in-frames 0 -deadline good -cpu-used 4 \
		-map_metadata -1 -fflags +bitexact -f ivf "$OUT/$3"
}

base() {
	vp8 640x360 500 video.ivf  # The single stream (004) and the 360 layer
	vp8 160x90 60 video_90.ivf # Simulcast layers (006)
	vp8 320x180 150 video_180.ivf
	vp8 1280x720 1500 video_720.ivf
	# Opus, 48 kHz mono, 20 ms frames, 32 kbit/s, 10 s of a 440 Hz tone with
	# a beep every second (never silent). One packet per Ogg page, as Pion's
	# oggreader returns one page at a time.
	ffmpeg -f lavfi -i "sine=frequency=440:beep_factor=4:sample_rate=48000:duration=10" \
		-ac 1 -c:a libopus -b:a 32k -frame_duration 20 -application audio -page_duration 20000 \
		-map_metadata -1 -fflags +bitexact "$OUT/audio.ogg"
}

vmaf_model() {
	local cache=${XDG_CACHE_HOME:-$HOME/.cache}/webrtc-load-engine
	mkdir -p "$cache"
	if [ ! -f "$cache/vmaf_v0.6.1.json" ]; then
		curl -sfL "$VMAF_MODEL_URL" -o "$cache/vmaf_v0.6.1.json.part"
		mv "$cache/vmaf_v0.6.1.json.part" "$cache/vmaf_v0.6.1.json"
	fi
	echo "$VMAF_MODEL_SHA256  $cache/vmaf_v0.6.1.json" | sha256sum -c --quiet -
	cp "$cache/vmaf_v0.6.1.json" "$WORK/vmaf.json"
}

# vmaf FILE HEIGHT: the VMAF of an encoded file against the pattern at its
# size (default model, mean over frames).
vmaf() {
	docker run --rm --user "$(id -u):$(id -g)" -v "$PWD":/w -v "$WORK":/t -w /w "$FFMPEG_IMAGE" -hide_banner -loglevel info \
		-i "$1" -f lavfi -i "$SRC:size=$(size_of "$2")" \
		-lavfi "[0:v]setpts=PTS-STARTPTS[d];[1:v]setpts=PTS-STARTPTS[r];[d][r]libvmaf=model=path=/t/vmaf.json:n_threads=8" \
		-f null - 2>&1 | sed -n 's/.*VMAF score: \([0-9.]*\).*/\1/p'
}

# encode CODEC HEIGHT KBPS OUTPUT: one single-stream file. Every frame is
# shown (no alt-ref, no lag, no B-frames) and keyframes fall every 30
# frames, at the same frames as the VP8 content's.
encode() {
	local codec=$1 h=$2 k=$3 out=$4 size
	size=$(size_of "$h")
	local common=(-f lavfi -i "$SRC:size=$size" -pix_fmt yuv420p
		-b:v "${k}k" -minrate "${k}k" -maxrate "${k}k" -bufsize "$((k / 2))k"
		-g 30 -keyint_min 30 -force_key_frames "expr:eq(mod(n,30),0)" -map_metadata -1 -fflags +bitexact)
	case $codec in
	vp9)
		ffmpeg "${common[@]}" -c:v libvpx-vp9 -profile:v 0 -auto-alt-ref 0 -lag-in-frames 0 \
			-deadline good -cpu-used 4 -row-mt 1 -f ivf "$out" ;;
	av1)
		ffmpeg "${common[@]}" -c:v libaom-av1 -usage realtime -cpu-used 8 -lag-in-frames 0 \
			-row-mt 1 -f ivf "$out" ;;
	h264)
		# Constrained Baseline (42e01f in SDP), SPS and PPS before every IDR,
		# an access unit delimiter per frame, Annex B. One slice per frame,
		# as browsers send at these sizes: -tune zerolatency turns on
		# sliced threads (one slice per thread), which tripled the packets
		# per frame (specs/014 T063).
		ffmpeg "${common[@]}" -c:v libx264 -profile:v baseline -level 3.1 -preset medium -tune zerolatency \
			-bf 0 -sc_threshold 0 -x264-params repeat-headers=1:aud=1:sliced-threads=0:slices=1 -f h264 "$out" ;;
	esac
}

ext_of() { [ "$1" = h264 ] && echo h264 || echo ivf; }

# within A B: |A - B| <= tolerance
within() { awk -v a="$1" -v b="$2" -v t="${3:-$VMAF_TOLERANCE}" 'BEGIN { d = a - b; if (d < 0) d = -d; exit !(d <= t) }'; }
below() { awk -v a="$1" -v b="$2" 'BEGIN { exit !(a < b) }'; }

ENTRIES=()
entry() { # codec arrangement max_height height kbps vmaf vmaf_ref file
	local mh=""
	[ "$3" != 0 ] && mh="\"max_height\": $3, "
	ENTRIES+=("  {\"codec\": \"$1\", \"arrangement\": \"$2\", ${mh}\"height\": $4, \"bitrate_kbps\": $5, \"vmaf\": $6, \"vmaf_reference\": $7, \"file\": \"$8\"}")
}

write_manifest() {
	local keep=$1 # entries of the other mode, kept from the existing manifest
	{
		echo "["
		local all=()
		if [ -n "$keep" ]; then mapfile -t all <<<"$keep"; fi
		all+=("${ENTRIES[@]}")
		local n=${#all[@]} i
		for ((i = 0; i < n; i++)); do
			if ((i + 1 < n)); then echo "${all[i]},"; else echo "${all[i]}"; fi
		done
		echo "]"
	} >"$MANIFEST"
}

# kept_entries ARRANGEMENT: the existing manifest's lines of that
# arrangement (one entry per line), without their trailing comma.
kept_entries() {
	[ -f "$MANIFEST" ] || return 0
	grep "\"arrangement\": \"$1\"" "$MANIFEST" | sed 's/,$//' || true
}

declare -A VP8_FILE=([90]=video_90.ivf [180]=video_180.ivf [360]=video.ivf [720]=video_720.ivf)
declare -A VP8_KBPS=([90]=60 [180]=150 [360]=500 [720]=1500)
declare -A VP8_VMAF

reference() {
	local h v
	for h in 90 180 360 720; do
		v=$(vmaf "$OUT/${VP8_FILE[$h]}" "$h")
		[ -n "$v" ] || { echo "no VMAF for the VP8 $h file" >&2; exit 1; }
		VP8_VMAF[$h]=$v
		echo "vp8 $h: ${VP8_KBPS[$h]} kbit/s, VMAF $v (reference)"
	done
}

# match CODEC HEIGHT: bisects the CBR bitrate until the VMAF is within the
# tolerance of the VP8 file of that height, then writes the file.
match() {
	local codec=$1 h=$2 ref=${VP8_VMAF[$2]} ext name lo hi k v n=0
	ext=$(ext_of "$codec")
	name="${codec}_${h}.${ext}"
	lo=$((VP8_KBPS[$h] / 5))
	hi=$((VP8_KBPS[$h] * 3))
	k=${VP8_KBPS[$h]}
	while :; do
		n=$((n + 1))
		encode "$codec" "$h" "$k" "/t/try.$ext"
		v=$(vmaf "/t/try.$ext" "$h")
		[ -n "$v" ] || { echo "$codec $h: no VMAF for the encoded file" >&2; exit 1; }
		echo "  $codec $h: $k kbit/s -> VMAF $v (target $ref)"
		if within "$v" "$ref"; then break; fi
		if ((n >= MAX_ENCODES)); then
			echo "$codec $h: no bitrate within $VMAF_TOLERANCE of VMAF $ref after $n encodes" >&2
			exit 1
		fi
		if below "$v" "$ref"; then lo=$k; else hi=$k; fi
		k=$(((lo + hi) / 2))
		if ((k == lo || k == hi)); then
			echo "$codec $h: bitrate range closed at $k kbit/s, VMAF $v, target $ref" >&2
			exit 1
		fi
	done
	cp "$WORK/try.$ext" "$OUT/$name"
	echo "$codec $h: $k kbit/s, VMAF $v -> $name"
	entry "$codec" single 0 "$h" "$k" "$v" "$ref" "$name"
}

codecs() {
	vmaf_model
	reference
	local h c
	for h in 90 180 360 720; do
		entry vp8 single 0 "$h" "${VP8_KBPS[$h]}" "${VP8_VMAF[$h]}" "${VP8_VMAF[$h]}" "${VP8_FILE[$h]}"
	done
	for c in vp9 av1 h264; do
		for h in 90 180 360 720; do
			match "$c" "$h"
		done
	done
	write_manifest "$(kept_entries svc)"
}

# h264: only the H.264 files and their manifest entries; the other entries
# are kept as they are.
h264_only() {
	vmaf_model
	reference
	local h keep=""
	for h in 90 180 360 720; do
		match h264 "$h"
	done
	if [ -f "$MANIFEST" ]; then
		keep=$(grep '"codec"' "$MANIFEST" | grep -v '"codec": "h264"' | sed 's/,$//' || true)
	fi
	write_manifest "$keep"
}

# --- Scalable content (specs/014 R4): VP9 and AV1 L3T3_KEY ---

# Runs a program of the SVC encoder image with the work folder at /t.
svc_run() {
	docker run --rm --user "$(id -u):$(id -g)" -v "$WORK":/t -w /t "$SVC_IMAGE" "$@"
}

svc_heights() { # max height -> the three layer heights, low to high
	case $1 in 360) echo "90 180 360" ;; 720) echo "180 360 720" ;; *) echo "bad max height $1" >&2; exit 1 ;; esac
}

# svc_bitrates K0 K1 K2: the encoders' per-layer targets, spatial × temporal.
# Each spatial layer's own (not cumulative over spatial layers); within a
# spatial layer cumulative over temporal layers, T0 60% and T0+T1 80%.
svc_bitrates() {
	local out="" k
	for k in "$@"; do out+="$((k * 6 / 10)),$((k * 8 / 10)),$k,"; done
	echo "${out%,}"
}

# svc_encode CODEC MAX K0 K1 K2: /t/svc.ivf from the raw frames /t/in.yuv,
# 3 spatial and 3 temporal layers (0-2-1-2), inter-layer prediction on
# keyframes only, CBR. A keyframe every 20 pictures, not 30 as the other
# content: the encoders do not restart the temporal pattern at a keyframe,
# so with 30 the keyframe of picture 30 would be in temporal layer 1, which
# a server thinning temporal layers drops. 20 is a multiple of 4 and
# divides the 300 pictures.
svc_encode() {
	local codec=$1 max=$2 w h total=$(($3 + $4 + $5)) bl
	IFS=x read -r w h <<<"$(size_of "$max")"
	bl=$(svc_bitrates "$3" "$4" "$5")
	case $codec in
	vp9)
		svc_run vp9_spatial_svc_encoder -f 300 -w "$w" -h "$h" -t 1/30 -sl 3 -tl 3 -tlm 3 -k 20 \
			-r 1/4,1/2,1/1 -b "$total" -bl "$bl" --rc-end-usage=1 --lag-in-frames=0 -sp 7 \
			--inter-layer-pred=2 in.yuv -o svc.ivf >/dev/null ;;
	av1)
		# Mode 10 is KSVC (L3T3_KEY); the input's length sets the frame count.
		svc_run svc_encoder_rtc -w "$w" -h "$h" -t 1/30 -sl 3 -tl 3 -lm 10 -k 20 \
			-b "$total" -bl "$bl" -sp 9 -o svc.ivf in.yuv >/dev/null
		rm -f "$WORK"/svc.ivf_*.av1 ;; # The operating points' files (not used)
	esac
}

# A layer's reference pictures: what the SVC encoder codes in that layer.
# The encoders scale the top frames down themselves, each with its own
# filter (no ffmpeg scaler matches libaom's quarter size from 1280x720), and
# the pattern's text keeps its pixel size, so the pattern drawn at the
# layer's size is a different picture. svc_pictures takes the lower layers'
# pictures from a near-lossless encode; the top layer is the pattern.
# ref_input MAX HEIGHT: ffmpeg input options of a layer's reference.
ref_input() {
	if [ "$1" = "$2" ]; then
		echo "-f lavfi -i $SRC:size=$(size_of "$1")"
	else
		echo "-f rawvideo -pix_fmt yuv420p -s $(size_of "$2") -r 30 -i /t/ref_$2.yuv"
	fi
}

# svc_pictures CODEC MAX: /t/ref_<height>.yuv for the two lower layers,
# decoded from an encode at 20 times the VP8 layers' bitrates.
svc_pictures() {
	local codec=$1 max=$2 heights s h
	read -r -a heights <<<"$(svc_heights "$max")"
	svc_encode "$codec" "$max" $((VP8_KBPS[${heights[0]}] * 20)) $((VP8_KBPS[${heights[1]}] * 20)) $((VP8_KBPS[${heights[2]}] * 20))
	for s in 0 1; do
		h=${heights[s]}
		svc_decode "$codec" "$s" "ref_$h.yuv"
	done
}

# svc_decode CODEC LAYER FILE: decodes one spatial layer of /t/svc.ivf alone
# into raw I420 frames /t/FILE.
svc_decode() {
	case $1 in
	vp9) svc_run vpxdec --svc-decode-layer="$2" --i420 -o "$3" svc.ivf >/dev/null 2>&1 ;;
	av1)
		# libdav1d's operating points: 0 is every layer, 3 the two lower, 6
		# the lowest; it outputs the highest spatial layer of each.
		docker run --rm --user "$(id -u):$(id -g)" -v "$WORK":/t "$FFMPEG_IMAGE" -hide_banner -loglevel error -y \
			-c:v libdav1d -oppoint $(((2 - $2) * 3)) -i /t/svc.ivf -f rawvideo -pix_fmt yuv420p "/t/$3" ;;
	esac
}

# ref_vmaf MAX HEIGHT INPUT...: the VMAF of a decoded input (ffmpeg input
# options, files under /t) against the layer's reference pictures.
ref_vmaf() {
	local max=$1 h=$2 ref
	shift 2
	read -r -a ref <<<"$(ref_input "$max" "$h")"
	docker run --rm --user "$(id -u):$(id -g)" -v "$WORK":/t "$FFMPEG_IMAGE" -hide_banner -loglevel info \
		"$@" "${ref[@]}" \
		-lavfi "[0:v]setpts=PTS-STARTPTS[d];[1:v]setpts=PTS-STARTPTS[r];[d][r]libvmaf=model=path=/t/vmaf.json:n_threads=8" \
		-f null - 2>&1 | sed -n 's/.*VMAF score: \([0-9.]*\).*/\1/p'
}

# svc_reference CODEC MAX: per layer height, the VMAF VP8 reaches on the
# layer's reference pictures at the VP8 simulcast layer's bitrate, with the
# VP8 content's settings. Each SVC layer must be within the tolerance of it.
declare -A SVC_REF
svc_reference() {
	local codec=$1 max=$2 h k v ref
	for h in $(svc_heights "$max"); do
		k=${VP8_KBPS[$h]}
		read -r -a ref <<<"$(ref_input "$max" "$h")"
		ffmpeg "${ref[@]}" \
			-c:v libvpx -pix_fmt yuv420p -b:v "${k}k" -minrate "${k}k" -maxrate "${k}k" -bufsize "$((k / 2))k" \
			-g 30 -keyint_min 30 -auto-alt-ref 0 -lag-in-frames 0 -deadline good -cpu-used 4 \
			-f ivf /t/ref.ivf
		v=$(ref_vmaf "$max" "$h" -i /t/ref.ivf)
		[ -n "$v" ] || { echo "no VMAF for the VP8 reference of the $h layer up to $max" >&2; exit 1; }
		SVC_REF[$max.$h]=$v
		echo "vp8 $h of $max ($codec's pictures): $k kbit/s, VMAF $v (reference)"
	done
	rm -f "$WORK/ref.ivf"
}

# svc_vmaf CODEC MAX LAYER: the VMAF of one spatial layer of /t/svc.ivf
# decoded alone at its size.
svc_vmaf() {
	local codec=$1 max=$2 h v
	h=$(svc_heights "$max" | cut -d' ' -f$(($3 + 1)))
	svc_decode "$codec" "$3" layer.yuv
	v=$(ref_vmaf "$max" "$h" -f rawvideo -pix_fmt yuv420p -s "$(size_of "$h")" -r 30 -i /t/layer.yuv)
	rm -f "$WORK/layer.yuv"
	echo "$v"
}

# svc_match CODEC MAX: bisects each spatial layer's bitrate, lowest first
# with the lower ones fixed, until its VMAF is within SVC_SEARCH_TOLERANCE
# of VP8's on the same pictures (svc_pictures, svc_reference); then checks every layer of
# the final file against the full tolerance and writes the file and its
# entries. The layers are not quite independent (a lower layer's VMAF moves
# by about 0.1 when an upper layer's bitrate changes), hence the narrower
# search.
svc_match() {
	local codec=$1 max=$2 heights ks s h ref lo hi k v n name
	read -r -a heights <<<"$(svc_heights "$max")"
	ks=("${VP8_KBPS[${heights[0]}]}" "${VP8_KBPS[${heights[1]}]}" "${VP8_KBPS[${heights[2]}]}")
	for s in 0 1 2; do
		h=${heights[s]} ref=${SVC_REF[$max.${heights[s]}]} n=0
		lo=$((VP8_KBPS[$h] / 5)) hi=$((VP8_KBPS[$h] * 3)) k=${ks[s]}
		while :; do
			n=$((n + 1)) ks[s]=$k
			svc_encode "$codec" "$max" "${ks[@]}"
			v=$(svc_vmaf "$codec" "$max" "$s")
			[ -n "$v" ] || { echo "$codec svc $max layer $h: no VMAF" >&2; exit 1; }
			echo "  $codec svc $max layer $h: $k kbit/s -> VMAF $v (target $ref)"
			if within "$v" "$ref" "$SVC_SEARCH_TOLERANCE"; then break; fi
			if ((n >= MAX_ENCODES)); then
				echo "$codec svc $max layer $h: no bitrate within $SVC_SEARCH_TOLERANCE of VMAF $ref after $n encodes" >&2
				exit 1
			fi
			if below "$v" "$ref"; then lo=$k; else hi=$k; fi
			k=$(((lo + hi) / 2))
			if ((k == lo || k == hi)); then
				echo "$codec svc $max layer $h: bitrate range closed at $k kbit/s, VMAF $v, target $ref" >&2
				exit 1
			fi
		done
	done
	# The last encode has every layer's bitrate; check each layer again on it.
	name="${codec}_svc_${max}.ivf"
	for s in 0 1 2; do
		h=${heights[s]} ref=${SVC_REF[$max.${heights[s]}]}
		v=$(svc_vmaf "$codec" "$max" "$s")
		within "$v" "$ref" || { echo "$codec svc $max layer $h: VMAF $v, target $ref, on the final file" >&2; exit 1; }
		echo "$codec svc $max layer $h: ${ks[s]} kbit/s, VMAF $v"
		entry "$codec" svc "$max" "$h" "${ks[s]}" "$v" "$ref" "$name"
	done
	cp "$WORK/svc.ivf" "$OUT/$name"
	echo "-> $name"
}

svc() {
	docker build -q -t "$SVC_IMAGE" scripts/svc-encoders >/dev/null
	vmaf_model
	local max c
	for max in 360 720; do
		# Raw I420 frames at the top size: both encoders scale down themselves.
		ffmpeg -f lavfi -i "$SRC:size=$(size_of "$max")" -pix_fmt yuv420p -f rawvideo /t/in.yuv
		for c in vp9 av1; do
			svc_pictures "$c" "$max"
			svc_reference "$c" "$max"
			svc_match "$c" "$max"
			rm -f "$WORK"/ref_*.yuv
		done
	done
	rm -f "$WORK/in.yuv"
	write_manifest "$(kept_entries single)"
}

case ${1:-codecs} in
base) base ;;
codecs) codecs ;;
h264) h264_only ;;
svc) svc ;;
*) echo "usage: $0 [codecs|h264|svc|base]" >&2; exit 2 ;;
esac
ls -l "$OUT"
