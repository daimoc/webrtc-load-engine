# Validation Report: Video Codec Comparison (014)

## Baseline (T001)

- Commit: `fd6e0b49ba59b4872200b7ab2bc92a55d3a1291d` (013 committed; 014 spec, plan and tasks uncommitted).
- `go vet ./... && go test -race ./...`: green.
- `go build ./src`: 29,627,365 bytes (R15 baseline).

## Server facts (R17)

Checked 2026-11-07 against the images the local setups run (T002), from
their shipped configuration and the source at the pinned tag.

1. **Jitsi (jicofo `stable-11248`, the version `sfu-remote` and the local
   `.env` use)**: `reference.conf` in `jicofo-common-1.0-1205.jar` enables
   all four codecs by default: AV1 pt 41 (rtx 42), VP8 pt 100 (rtx 96), VP9
   pt 101 (rtx 97), H.264 pt 107 (rtx 99), and the
   `av1-dependency-descriptor` extension with id 11. The image's
   `/defaults/jicofo.conf` overrides a codec only when `ENABLE_CODEC_*` is
   set; neither `env.example` nor the local `.env` sets one. **No change to
   `tests/platforms/docker-jitsi-meet` is needed (T032 has nothing to do).**
   The H.264 profile jicofo offers is read from a live session-initiate in
   T028.
2. **LiveKit v1.13.10**: `pkg/config/config.go` default `EnabledCodecs`
   includes VP8, H.264, VP9, AV1 (and H.265); `livekit.yaml` does not
   override it. `pkg/sfu/forwarder.go`: VP9 SVC is forwarded with the
   dependency descriptor if negotiated, otherwise by the VP9 payload
   descriptor; **AV1 SVC needs the dependency descriptor** (without it the
   forwarder falls back to the simulcast selector, "nothing tells the
   temporal layers apart"). H.264 uses the simulcast selector.
3. **mediasoup 3.30.1** (`supportedRtpCapabilities` in the demo image):
   video codecs VP8, VP9, H.264 (`level-asymmetry-allowed=1`) and **AV1**;
   the dependency descriptor (preferred id 7) is **`recvonly`**: mediasoup
   reads it from producers (`RTP/Codecs/AV1.cpp` parses AV1 layers only from
   it, no descriptor means no AV1 layer information) but does not send it to
   consumers. So receiving bots cannot read AV1 layers from a descriptor on
   mediasoup; the AV1 receive parser must fall back to the OBU headers.
   `config.mjs` today has VP8, VP9 `profile-id=2`, H.264 `4d0032`/`42e01f`:
   VP9 profile 0 and AV1 must be added (T030).
4. **Janus v1.4.3** (`src/plugins/janus_videoroom.c`): rooms take
   `videocodec` (a list), `vp9_profile`, `h264_profile`; a publisher
   stream is SVC only when the `publish`/`configure` request carries
   `"svc": [{"mindex": n}]` and the codec is VP9 or AV1 (line 13054);
   VP9 layers come from the payload descriptor (`janus_vp9_parse_svc`),
   **AV1 layers from the dependency descriptor** (`dd_len > 0`, line 8940).
   Simulcast is by RID or SSRC for any codec, H.264 included. Subscribers
   choose with `substream`/`temporal` or `spatial_layer`/`temporal_layer`.

Consequence for the plan: AV1 must always carry the dependency descriptor
(research R6 already says so); the AV1 receive parser cannot rely on it
being forwarded (mediasoup), so it reads frame start and keyframes from the
aggregation header and OBUs when the descriptor is absent (T037).

### SVC encoders (T003, R17.5)

`scripts/svc-encoders/Dockerfile`: Ubuntu 24.04, libvpx `v1.15.0` (all
examples built: the single example target has no make rule) and libaom
`v3.12.0` (`svc_encoder_rtc` target). Tried on 60 raw I420 frames of
`testsrc2` at 640x360, 30 fps:

```bash
vp9_spatial_svc_encoder -f 60 -w 640 -h 360 -t 1/30 -sl 3 -tl 3 -tlm 3 -k 30 \
  -r 1/4,1/2,1/1 -b 600 --rc-end-usage=1 --lag-in-frames=0 -sp 7 --inter-layer-pred=2 in.yuv -o vp9svc.ivf
svc_encoder_rtc -w 640 -h 360 -t 1/30 -sl 3 -tl 3 -lm 10 -k 30 -b 600 \
  -bl 60,80,100,90,120,150,210,280,350 -sp 9 -o out in.yuv
```

- **VP9**: one IVF (`VP90`, 640x360, timebase 1/30) of 60 superframes,
  each with a superframe index of 3 layer frames (160x90, 320x180,
  640x360); the spatial layer 0 frame is a keyframe at frames 0 and 30
  only, the upper layers' frames on those superframes are inter frames
  (inter-layer prediction on keyframes: `L3T3_KEY`); every layer frame has
  `show_frame=1`. `--inter-layer-pred=2` is "key frames only";
  `-tlm 3` is the 0-2-1-2 temporal pattern (no temporal ID in the VP9
  bitstream: picture i has T0, T2, T1, T2 for i mod 4 = 0, 1, 2, 3).
- **AV1**: `-lm 10` is libaom's KSVC mode ("only keep the inter-layer
  reference for superframes whose base is key"), i.e. `L3T3_KEY`; mode 9
  would be full `L3T3`. `-bl` is required (9 cumulative values, spatial ×
  temporal; the top temporal value of each spatial layer sums to `-b`).
  `--frames` is listed in the usage but not parsed by 3.12.0 (it is
  refused), so the input file decides the length. Output `out` is an IVF
  (`AV01`) of 180 frames, 3 per timestamp (one per spatial layer, low to
  high), each with an OBU extension header carrying temporal and spatial
  IDs; the first of each timestamp starts with a temporal delimiter; key
  frames and sequence headers at timestamps 0 and 30 on spatial layer 0
  only; every frame shown; temporal IDs 0, 2, 1, 2. `out_0.av1` …
  `out_8.av1` are the operating points (not used).
- Both need the raw frames at the top size; each scales down internally
  (`-r` for VP9; libaom's mode 10 uses 1/4, 1/2, 1).

## SC-001: every codec on every server

### Jitsi, first check (T028)

Local `stable-11248`. jicofo's session-initiate offers, for video, AV1 41,
VP8 100, VP9 101 and H.264 107 (one `profile-level-id` parameter of value
`42e01f;level-asymmetry-allowed=1;packetization-mode=1;`), each with RTX,
and the dependency descriptor as extension 11. The bots' answer accepts all
four; the video track binds the plan's codec. Its trailing `;` made the
bots' session-accept carry a parameter without a name; fixed in
`jingle_sdp.go` (skipped), with a test. 2 bots, 12 s, per codec with
`jitsi-connect --video-codec`: `received` is the plan's codec for both bots
in VP9, AV1 and H.264, no mismatch. Nothing in presence is needed (the
bridge forwards by payload type); `tests/platforms/docker-jitsi-meet` is
unchanged (T032: nothing to do).

### LiveKit, first check (T029)

Local v1.13.10, 2 bots, 8 s per codec. The bots' `AddTrackRequest` names a
non-VP8 codec in `simulcast_codecs` (field 10; golden generated with
livekit/protocol v1.52.1) and their publish offer lists the plan's codec
first (all transceivers, VP8 left in Pion's order). `received` is the plan's
codec for both bots in VP8, VP9, AV1 and H.264; no mismatch.

### mediasoup, first check (T030)

`config.mjs` gains VP9 `profile-id: 0` and AV1 (VP8, VP9 profile 2 and the
two H.264 entries kept). The bots announce only the router's entry for the
plan's codec (VP9 profile 0, H.264 `42e01f` mode 1) and its RTX, produce
it, and fail with `codec_mismatch` naming the router's video codecs when it
has none (test with the 010 router: VP9 profile 2 only, no AV1).

A bug of the bots found here, present before 014 but harmless with VP8:
mediasoup's first receive offer has only the data channel section, and
while it is the only one, Pion takes any packet of an unknown SSRC
(mediasoup's bandwidth probe) for that section's and adds a `sendrecv`
video transceiver sending a sample track in its first video codec, VP8.
The next video section reused it: in VP8 runs it answered `sendrecv` with
a dummy source; in VP9, AV1 and H.264 runs binding VP8 failed and the
consumer was refused (one bot of two got no video). The bots now stop such
transceivers on their receiving session before each renegotiation
(`dropPionSenders`, with a test). After the fix, 2 bots, 6 s, twice per
codec: both bots receive the plan's codec in all four.

H.264 note (found writing T030): Pion lists H.264 `42001f` (Baseline)
before `42e01f` (Constrained Baseline); the bots now bind `42e01f` itself
first, the profile the content is and the servers list.

### Media source file (T036)

A `source_file` is transcoded into the run's codec with `gen-media.sh`'s
encoder settings, at `content.json`'s bitrate for each height. Tested with
ffmpeg from the `jrottenberg/ffmpeg:7.1-ubuntu` image (a scratch wrapper
first in `PATH`; ffmpeg is not installed on the workstation): VP9, AV1 and
H.264 at 180 and 360 lines, keyframes every 30 frames. Live: `jitsi-connect
--video-codec h264 --media-file clip.mkv`, 2 bots on the local Jitsi,
completed, both bots received H.264.

### Single stream, 10 bots, 1 minute (T035, T044)

2026-11-08, workstation (16 hardware threads), one server running at a
time, `audio_video`, 10 bots, 60 s hold, live tests built from the working
tree. Every run passed its live test's checks. In every run all 10 bots
received the plan's codec (`received` = {codec: 10}), with no mismatch, at
the 360-line height, with no resolution switch. The table gives the median
received frame rate, the freezes over the whole run, the server's mean CPU
(threads, the containers sampled every 5 s), and the engine's.

| Server | Codec | Frame rate | Freezes | Server CPU | Engine CPU |
|---|---|---|---|---|---|
| Janus 1.4.3 | VP8 | 30.0 | 0 | 0.93 | 0.72 |
| | VP9 | 30.0 | 0 | 0.76 | 0.60 |
| | AV1 | 30.0 | 0 | 0.95 | 0.74 |
| | H.264 | 30.0 | 0 | 1.66 | 1.26 |
| LiveKit 1.13.10 | VP8 | 30.0 | 0 | 0.66 | 0.77 |
| | VP9 | 30.0 | 0 | 0.55 | 0.62 |
| | AV1 | 30.0 | 0 | 0.70 | 0.81 |
| | H.264 | 30.0 | 0 | 1.08 | 1.36 |
| mediasoup 3.30.1 | VP8 | 30.0 | 0 | 0.27 | 0.80 |
| | VP9 | 30.0 | 0 | 0.22 | 0.67 |
| | AV1 | 30.0 | 0 | 0.28 | 0.81 |
| | H.264 | 30.0 | 0 | 0.41 | 1.11 |
| Jitsi stable-11248 (bridge + jicofo) | VP8 | 30.0 | 0 | 1.17 + 0.01 | 0.54 |
| | VP9 | 30.0 | 0 | 0.90 + 0.00 | 0.50 |
| | AV1 | 30.0 | 0 | 1.09 + 0.01 | 0.58 |
| | H.264 | 30.0 | 0 | 1.55 + 0.01 | 0.87 |

These are local checks of the path, not the comparison (US4, remote
machine): the server and the bots share the workstation. They already show
H.264 costing every server more than the other codecs. Part of that is the
content: H.264 is the highest bitrate at 360 lines (500 kbit/s, with VP8),
and it has one more packet per frame (the trailer SEI). The engine's CPU
follows the same order.

**Jitsi needs its bandwidth estimate off locally too.** With Jitsi's default
(the local setup), VP8 and AV1 failed on freezes (41 and 21 over the run,
10 s each, no loss: the bridge stopped forwarding some streams), VP9 and
H.264 passed. The build before 014 (fd6e0b4) fails the same way in VP8 (20
freezes), and a second VP8 run of the 014 build gave 7: the bridge's
allocation at 10 bots, not the codec. The four runs in the table are with
`JVB_CC_TRUST_BWE=false` (a scratch compose override, as the remote overlay
of 013 does); the repository's local setup still keeps the default.

**AV1 on Jitsi**: the bridge logs "Could not parse AV1 Dependency
Descriptor ... No external dependency structure specified for non-first
packet" for each stream's packets in the first two seconds only (398 lines
in one run): packets that left before the bridge had a keyframe with the
structure. The bots send the structure on every keyframe, as browsers do.

## SC-002: codec named, mismatch fails

Every report of the 16 runs names the codec sent and received ("Video
codec" row, `quality.video.codec`).

Mismatch (T035): AV1 removed from `config.mjs`, mediasoup restarted, one
run asking for AV1 with 10 bots. All 10 bots fail at stage `media` with
`codec_mismatch: sending negotiated VP8, VP9, H264, plan asks for av1`. The
report lists the 10 bots (`direction` sent, `codec` vp8; Markdown "Codec
mismatch": "The figures are not av1 figures"), `received` is empty, and no
video figure is given ("no bot received video"). `config.mjs` restored
after. The report's outcome is `stopped`, not `failed`: the live test stops
the engine itself once bots fail, as it does for every live failure since
009.

Refusals: a plan with `video_codec` and profile `receive_only` is refused
before any bot joins (a first Jitsi attempt without `JITSI_MEDIA`: "test
plan: media.video_codec needs profile audio_video, got \"receive_only\"").
An unknown codec is refused naming the four (unit test).

## SC-003: same measures for every codec

In the 16 runs, every report gives frame rate, freezes, resolution and
video latency, in every codec. Audio latency is given everywhere but on
Janus, which gives `frame_not_identified` in every codec, VP8 included, as
before 014. Median MOS 4.37 in all 16.

## SC-004: browser decoding

### Single stream, without the frame trailer (T025)

2026-11-08, local Janus v1.4.3, one room per codec created by the bots
(`videocodec` vp8, vp9 with `vp9_profile` 0, av1, h264 with `h264_profile`
42e01f), 2 bots each, joined with the Janus VideoRoom demo page. The 012
trailer is off for VP9, AV1 and H.264 at this stage (VP8 keeps it).

| Codec | Chrome | Firefox |
|---|---|---|
| VP8 (control) | moving pattern | moving pattern |
| VP9 | moving pattern | moving pattern |
| AV1 | moving pattern | moving pattern |
| H.264 | moving pattern | moving pattern |

Checked by the user. A first attempt saw empty rooms: the bots' 15-minute
hold had ended before the browsers joined (Janus logs); the second run held
for 60 minutes.

### Single stream, with the frame trailer (T043)

2026-11-08. The bots held four rooms on the local Janus for 60 minutes
with the trailer on in every codec. The user went on without reporting a
per-browser result, so the check that stands is a decoder one:

- 10 s of the source's packets per codec (301 frames), depacketized as a
  receiver does, written one frame per packet, decoded by ffmpeg 7.1 with
  `-err_detect explode`: native `vp9`, `libdav1d`, native `h264` (the
  decoder Chrome uses for H.264);
- every frame decodes in all three codecs, and the decoded pictures are
  identical, frame for frame (framemd5), to those of the same frames with
  the trailer removed.

**VP9 corrected by T051**: this check passed VP9 wrongly. ffmpeg's VP9
decoder ignores bytes after a frame; libvpx, which Chrome and Firefox use,
does not (`vpxdec`: "Invalid frame marker", single stream and SVC alike).
The browsers were never shown single-stream VP9 with the trailer (the user
reported no per-browser result). VP9's trailer is now RTP padding (research
R9 revised); see T051.

H.264: ffmpeg logs "Late SEI is not implemented" once per frame and
ignores the SEI. An SEI after the picture's slices is outside the order
H.264 sets for an access unit (7.4.1.2.3), so a strict decoder could refuse
it; none of those tried does. Not seen in a browser with the trailer: see
"Not met or not checked".

### Layered, with the trailer (T051)

2026-11-08, local Janus v1.4.3, 2 bots per room sending layers up to 720:
VP8 and H.264 simulcast, VP9 and AV1 SVC (`L3T3_KEY`), viewed with the
VideoRoom demo in Chrome and Firefox, switching layers.

| Room | Chrome and Firefox |
|---|---|
| VP8 simulcast (control) | moving pattern |
| H.264 simulcast | moving pattern |
| VP9 SVC | **not working** |
| AV1 SVC | moving pattern |

Cause (reproduced with libvpx's `vpxdec`): the VP9 trailer after each layer
frame. The browser hands the decoder the picture's layer frames back to
back, and libvpx reads the trailer as the start of another frame; the same
holds for a single VP9 stream. With the user's choice (option 1 of 3), the
VP9 trailer moved to the RTP padding of the layer frame's last packet,
which receivers strip before decoding. Through Janus the bots still measure
VP9 latency (the server forwards the padding). Second check, 2026-11-08
about 13:10, with the padding carrier (VP9 SVC in room 9514, single VP9
stream in room 9513): reported OK by the user.

| Room (second check) | Chrome and Firefox |
|---|---|
| VP9 SVC | OK (user) |
| VP9 single stream | OK (user) |

Third check, 2026-11-08 about 21:20, after the H.264 content was
re-encoded with one slice per frame (SC-006, T063): H.264 single stream in
room 9515 and H.264 simulcast up to 720 in room 9516, 2 bots each, local
Janus, Chrome and Firefox: reported OK by the user.

| Room (third check) | Chrome and Firefox |
|---|---|
| H.264 single stream, one slice per frame | OK (user) |
| H.264 simulcast, one slice per frame | OK (user) |

## SC-005: codecs side by side per server

### Remote machine, 10 bots (T060, T061)

2026-11-08, 18:59 to 19:47 (local time), `fetis@192.168.1.95` (AMD PRO
A10-8770E, 4 cores, 18 GiB; workstation 192.168.1.65, both at 1000 Mb/s),
one server at a time with `sfu-remote deploy` and its settings file
(`DOCKER_HOST=ssh://`, server load sampled every 5 s), `sfu-remote remove`
after each server. Every run: 10 bots, 1 minute hold, `audio_video`.
Single-stream runs: no receive cap (9 streams per kind per bot), in the
order VP8, VP9, AV1, H.264, VP8 again. Layered runs: as T059 (layers up to
720, receive cap 4, requested heights `180:0.5,720:0.5`). Jitsi with the
remote overlay (`JVB_CC_TRUST_BWE=false`) and a 10 s ramp-up; the others
without ramp-up. Server CPU is in threads (1.00 = one core) over the run,
memory the peak of the server's containers. MOS is audio, p50 over bots.
Latency p50 is video (`frame_not_identified`: VP9 on LiveKit and Jitsi, see
SC-008). "Requested-height share" is the lowest bot's share of video time
at the height it asked for (layered runs). Report files: the session's
scratch folder `t060/reports` (not kept in the repository).

**Content is synthetic**: `testsrc2` (research R3 limit). The bitrates are
per codec at equal VMAF on it; a real camera picture would give other
ratios, so a codec's server cost here is its cost at these bitrates.

#### janus

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.76 | 0.79 | 16 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp9 | vp9 | single | 360: 200 | 0.79 | 0.82 | 17 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| av1 | av1 | single | 360: 400 | 0.82 | 0.85 | 18 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 0.99 | 1.00 | 19 | 4.37 | 0 | 30.0 | 3.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.82 | 0.85 | 19 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp8-layered | vp8 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.74 | 0.79 | 21 | 4.37 | 0 | 30.0 | 2.5 | 1.000 | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1350 | 0.93 | 0.96 | 21 | 4.37 | 0 | 30.0 | 2.5 | 1.000 | PASS |
| vp9-layered | vp9 | svc | 180: 168, 360: 750, 720: 2062 | 0.80 | 0.83 | 21 | 4.37 | 0 | 30.0 | 2.5 | 1.000 | PASS |
| av1-layered | av1 | svc | 180: 150, 360: 625, 720: 1875 | 0.84 | 0.89 | 21 | 4.37 | 0 | 30.0 | 2.5 | 0.991 | PASS |

#### livekit

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.86 | 0.88 | 93 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp9 | vp9 | single | 360: 200 | 0.69 | 0.76 | 122 | 4.37 | 0 | 30.0 | frame_not_identified | – | FAIL |
| av1 | av1 | single | 360: 400 | 0.92 | 0.94 | 124 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 1.17 | 1.20 | 122 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.87 | 0.88 | 124 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp8-layered | vp8 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.74 | 0.77 | 122 | 4.37 | 0 | 30.0 | 2.5 | 0.989 | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1350 | 1.06 | 1.09 | 128 | 4.37 | 0 | 30.0 | 2.5 | 0.993 | PASS |
| vp9-layered | vp9 | svc | 180: 168, 360: 750, 720: 2062 | 0.93 | 0.94 | 133 | 4.37 | 0 | 30.0 | frame_not_identified | 1.000 | FAIL |
| av1-layered | av1 | svc | 180: 150, 360: 625, 720: 1875 | 0.90 | 0.95 | 126 | 4.37 | 0 | 30.0 | 2.5 | 1.000 | PASS |

#### mediasoup

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.49 | 0.54 | 64 | 4.37 | 0 | 30.0 | 5.5 | – | PASS |
| vp9 | vp9 | single | 360: 200 | 0.42 | 0.46 | 66 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| av1 | av1 | single | 360: 400 | 0.48 | 0.51 | 71 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 0.51 | 0.54 | 87 | 4.37 | 0 | 30.0 | 5.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.48 | 0.49 | 89 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| vp8-layered | vp8 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.43 | 0.48 | 90 | 4.37 | 1 | 30.0 | 3.5 | 0.550 | FAIL |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1350 | 0.50 | 0.54 | 98 | 4.37 | 0 | 30.0 | 5.5 | 0.701 | FAIL |
| vp9-layered | vp9 | svc | 180: 168, 360: 750, 720: 2062 | 0.47 | 0.47 | 100 | 4.37 | 1 | 30.0 | 3.5 | 0.710 | FAIL |
| av1-layered | av1 | svc | 180: 150, 360: 625, 720: 1875 | 0.47 | 0.48 | 105 | 4.37 | 3 | 29.6 | 3.5 | 0.853 | FAIL |

#### jitsi

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 1.53 | 3.60 | 794 | 4.33 | 31 | 30.0 | 4.5 | – | PASS |
| vp9 | vp9 | single | 360: 200 | 1.12 | 1.37 | 872 | 4.37 | 0 | 30.0 | frame_not_identified | – | FAIL |
| av1 | av1 | single | 360: 400 | 1.23 | 1.81 | 1120 | 4.36 | 0 | 30.0 | 4.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 1.40 | 1.57 | 1115 | 4.37 | 0 | 30.0 | 5.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 1.09 | 1.29 | 1144 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| vp8-layered | vp8 | simulcast | 180: 150, 360: 500, 720: 1500 | 1.03 | 1.31 | 1124 | 4.37 | 0 | 30.0 | 4.5 | 1.000 | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1350 | 2.68 | 3.03 | 1564 | 4.36 | 0 | 30.0 | 12.5 | layers_not_forwarded | FAIL |
| vp9-layered | vp9 | svc | 180: 168, 360: 750, 720: 2062 | 1.08 | 1.59 | 1485 | 4.37 | 0 | 30.0 | frame_not_identified | 1.000 | FAIL |
| av1-layered | av1 | svc | 180: 150, 360: 625, 720: 1875 | 1.05 | 1.33 | 1491 | 4.37 | 0 | 30.0 | 3.5 | 1.000 | PASS |


**Server CPU against VP8** (single stream; "within" means the difference
from VP8's mean is no larger than the two VP8 runs' difference):

| Server | VP8 runs | VP9 | AV1 | H.264 (5 slices, superseded) |
|---|---|---|---|---|
| Janus | 0.76, 0.82 (0.06) | 0.79 within | 0.82 within | 0.99, **+0.20** |
| LiveKit | 0.86, 0.87 (0.01) | 0.69, **−0.18** | 0.92, **+0.06** | 1.17, **+0.31** |
| mediasoup | 0.49, 0.48 (0.01) | 0.42, **−0.07** | 0.48 within | 0.51, +0.02 (just above) |
| Jitsi | 1.53, 1.09 (0.44) | 1.12 within | 1.23 within | 1.40 within |

- H.264 here is the first H.264 content, 5 slices per frame: superseded by
  the rerun below (one slice per frame).
- VP9's content is 200 kbit/s against VP8's 500, so less traffic: LiveKit
  and mediasoup spend less on it; Janus does not differ.
- Jitsi's two VP8 runs differ by 0.44: the first run after `deploy` is on
  a cold bridge (CPU peak 3.60, 31 freezes during the ramp-up, none in the
  hold). No codec stands out beyond that, so on Jitsi this run set cannot
  tell the codecs apart; a warm-up run before the first VP8 run would be
  needed (condition that differs between runs, US4 scenario 2).
- Memory grows across each server's runs (Jitsi 794 to 1491 MB, LiveKit 93
  to 133 MB): the order of the runs, not the codec.
- Layered: Jitsi H.264 simulcast (5-slice content) cost 2.68 threads
  against 1.03 for VP8 simulcast; with one slice per frame it is 1.27 (rerun
  below), so the slices' packets were most of it. mediasoup's bots
  asking for 720 stay below it (0.55 to 0.85, as T059), with 1 to 3 freezes.


### H.264 rerun, one slice per frame (T060, T061)

The H.264 content was re-encoded with one slice per frame (SC-006, T063;
`scripts/gen-media.sh h264`; 720 layer now 1500 kbit/s, 90 layer 78) and
checked in browsers (SC-004, third check). 2026-11-08, 21:05 to 21:45, same
machine and settings as above, per server: VP8, H.264, VP8 single stream,
then H.264 layered as T059; `deploy` and `remove` around each server. The
local workstation also ran 4 bots in two Janus rooms (the browser check)
during the Janus and part of the LiveKit runs.

Video packets per second per stream: VP8 66, H.264 96 (one frame's data
packets plus the trailer's SEI in its own packet: about one packet more
per frame), against about 190 with the 5-slice content (T063: 1914 per bot for 10 streams).


#### janus

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.79 | 0.84 | 16 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 0.85 | 0.89 | 18 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.81 | 0.84 | 19 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.76 | 0.81 | 20 | 4.37 | 0 | 30.0 | 2.5 | 1.000 | PASS |

#### livekit

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.86 | 0.88 | 91 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 0.97 | 0.98 | 123 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.86 | 0.89 | 120 | 4.37 | 0 | 30.0 | 2.5 | – | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.79 | 0.81 | 111 | 4.37 | 0 | 30.0 | 2.5 | 0.993 | PASS |

#### mediasoup

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 0.49 | 0.55 | 64 | 4.37 | 0 | 30.0 | 5.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 0.49 | 0.51 | 73 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 0.49 | 0.50 | 77 | 4.37 | 0 | 30.0 | 5.5 | – | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1500 | 0.47 | 0.49 | 83 | 4.37 | 0 | 30.0 | 4.5 | 0.588 | FAIL |

#### jitsi

| Run | Codec | Arrangement | Content kbit/s | Server CPU avg (threads) | Server CPU peak | Server memory peak (MB) | MOS p50 | Freezes | Frame rate p50 | Latency p50 (ms) | Requested-height share, lowest bot | Result |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| vp8a | vp8 | single | 360: 500 | 1.53 | 3.49 | 794 | 4.33 | 42 | 30.0 | 4.5 | – | PASS |
| h264 | h264 | single | 360: 500 | 1.22 | 1.62 | 845 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| vp8b | vp8 | single | 360: 500 | 1.12 | 1.40 | 861 | 4.37 | 0 | 30.0 | 4.5 | – | PASS |
| h264-layered | h264 | simulcast | 180: 150, 360: 500, 720: 1500 | 1.27 | 1.77 | 958 | 4.37 | 0 | 30.0 | 6.5 | layers_not_forwarded | FAIL |

**Server CPU against VP8, one slice per frame** (single stream):

| Server | VP8 runs | H.264 | Against VP8's mean |
|---|---|---|---|
| Janus | 0.79, 0.81 (0.02) | 0.85 | **+0.05** |
| LiveKit | 0.86, 0.86 (0.00) | 0.97 | **+0.11** |
| mediasoup | 0.49, 0.49 (0.00) | 0.49 | within |
| Jitsi | 1.53, 1.12 (0.41; first run on a cold bridge, 42 freezes in the ramp-up) | 1.22 | within |

H.264 still costs Janus (+6%) and LiveKit (+13%) more than VP8 at the same
bitrate, much less than with 5 slices (+25%, +36%); it sends 45% more
packets, the trailer's SEI packet being one per frame, so part of the
remaining cost is the measurement's own. mediasoup does not differ; Jitsi
cannot tell (cold first run, as above). H.264 layered: Janus and LiveKit
forward the requested heights (1.000, 0.993); mediasoup's bots asking for
720 get it 0.59 of the time (as the other codecs); Jitsi forwards only the
lowest layer (`layers_not_forwarded`).

## SC-006: engine CPU per codec

### Benchmarks (T062)

2026-11-08, workstation (i5-13600H), `go test ./src/media/ -bench
'SourcePacketize|QualityVideoPacket' -count 3`, mean of 3. Packetize: one
picture, every layer of it (up to 720), trailer and dependency descriptor
included, per source (once per frame for all bots). Quality: one received
packet (highest simulcast layer, or every SVC layer), latency lookup
included, per bot and stream.

| Codec | Packetize single | Packetize layered | Quality single | Quality layered |
|---|---|---|---|---|
| VP8 | 1.0 µs, 8 allocs | 3.4 µs (simulcast), 27 allocs | 69 ns, 0 allocs | 57 ns |
| VP9 | 0.6 µs, 7 allocs | 7.2 µs (SVC), 88 allocs | 87 ns | 50 ns |
| AV1 | 1.5 µs, 22 allocs | 10.1 µs (SVC), 97 allocs | 115 ns (7 B/op average) | 104 ns (20 B/op average) |
| H.264 | 1.9 µs, 21 allocs (5 slices); 2.9 µs, 14 allocs (1 slice) | 7.3 µs, 76 allocs (5 slices); 13.1 µs, 47 allocs (1 slice, 720 layer at 1500 kbit/s) | 57 ns; 69 ns (1 slice) | 52 ns; 58 ns (1 slice) |

Every codec stays under research R10's 200 ns per received packet; per
frame, packetizing costs at most 10 µs of a 33 ms frame interval, once per
source. The figures vary by up to 50% between runs on this laptop (VP8
single quality 102 to 168 ns before the benchmark fix below), so only the
order of magnitude is meant.

The VP8 quality path was 52 ns and 0 allocations at fd6e0b4 (before 014)
and is 68 ns now: +16 ns per packet from the codec-neutral parser, about
0.01% of a core per bot at 5,400 received video packets per second.
`BenchmarkQualityVideoPacket` at first showed 1 allocation (80 B) per
packet: the benchmark called `UpdatePacket`, whose own header escapes into
the parser; the bots' receive loop reuses one header per stream
(`UpdateRTP`, `execution.Peer.onTrack`), so the benchmarks now do the same
and show 0 allocations for VP8, VP9 and H.264.


### Engine CPU at 100 bots (T063)

2026-11-08, local LiveKit 1.13.10 restarted before each run, 100 bots
`audio_video`, single stream, receive cap 10, 20 s ramp-up, 2 minute hold
(146 s per run), in the order VP8, VP9, AV1, H.264, VP8. Engine CPU is the
engine's own `load_machine` average over the run, per bot.

| Run | Engine CPU (threads) | Per bot | Against VP8 | Video packets/s received per bot | Engine memory peak | Freezes | MOS p50 |
|---|---|---|---|---|---|---|---|
| VP8 | 4.35 | 0.0435 | | 656 | 1024 MB | 0 | 4.36 |
| VP9 | 3.49 | 0.0349 | −20% | 320 | 964 MB | 0 | 4.36 |
| AV1 | 4.41 | 0.0441 | +1.7% (within 10%) | 633 | 989 MB | 0 | 4.35 |
| H.264 | 5.58 | 0.0558 | **+28.6%** | 1914 | 1020 MB | 0 | 4.36 |
| VP8 | 4.33 | 0.0433 | (VP8 runs 0.5% apart) | 667 | 1027 MB | 0 | 4.35 |

(VP9's run fails the live test only on latency, `frame_not_identified` on
LiveKit.) The engine's CPU follows the packets it receives, not the codec:
VP9's content is 200 kbit/s (half the packets of VP8), H.264's has three
times VP8's packets at the same 500 kbit/s. **Cause: the H.264 content has
5 slices per frame** (1450 slices for 300 frames in `h264_360.h264`, about
376 bytes each): `-tune zerolatency` turns on x264's sliced threads, one
slice per thread, in `scripts/gen-media.sh`. Pion's payloader sends each
slice as its own packet (it drops the access unit delimiters and groups
SPS/PPS with the next unit), and the trailer SEI adds one: about 6 packets
per frame against VP8's 2.2. Browsers' H.264 encoders send one slice per
frame at this size, so these H.264 figures overstate H.264's cost by its
packet count. With the user's choice, the H.264 content was re-encoded with
one slice per frame (`sliced-threads=0`, `scripts/gen-media.sh h264`) and
the H.264 runs redone (below, and SC-005's rerun).

Rerun, 2026-11-08 after 21:45, same plan, the new H.264 content, in the
order VP8, H.264, VP8:

| Run | Engine CPU (threads) | Per bot | Against VP8 | Video packets/s received per bot | Engine memory peak | Freezes | MOS p50 |
|---|---|---|---|---|---|---|---|
| VP8 | 4.37 | 0.0437 | | 656 | 1015 MB | 0 | 4.35 |
| H.264, 1 slice | 4.92 | 0.0492 | **+12.1%** | 958 | 1029 MB | 0 | 4.35 |
| VP8 | 4.40 | 0.0440 | (VP8 runs 0.7% apart) | 667 | 1016 MB | 0 | 4.35 |

**SC-006**: AV1 is within 10% of VP8 (+1.7%). VP9 (−20%) and H.264
(+12.1%) are not, and the difference is measured and stated: the engine's
CPU follows the packets it receives. VP9's content is 200 kbit/s (half
VP8's packets); H.264 sends 45% more packets than VP8 at the same 500
kbit/s, one of them per frame being the trailer's SEI, a cost of the
measurement itself.

## SC-007: VP8 unchanged

### Existing VP8 live tests (T064)

2026-11-08, local servers, the live tests without `*_VIDEO_CODEC`, as 012
ran them (T019, SC-001, SC-002): 10 bots, `audio_video`, 60 s hold; Jitsi
with simulcast 720, cap 4, default heights, 5 s ramp-up and the
repository's local setup (Jitsi's default bandwidth estimate, no
override). Reports name `vp8`, `single` (Jitsi `simulcast`).

| Server | 012 (latency audio / video, freezes, fps, MOS) | 014 | Result |
|---|---|---|---|
| Janus | – / 1.5 ms, 0, 30.0, 4.37 | – / 1.5 ms, 0, 30.0, 4.37 | PASS |
| LiveKit | 0.5 / 1.5 ms, 0, 30.0, 4.37 | 0.5 / 1.5 ms, 0, 30.0, 4.37 | PASS |
| mediasoup | 1.5 / 2.5 ms, 0, 30.0, 4.37 | 1.5 / 2.5 ms, 0, 30.0, 4.37 | PASS |
| Jitsi | 0.5 / 1.5 ms, 0, 720: 100% | 0.5 / 1.5 ms, 0, 30.0, 4.37; 720: 54%, then 100% | FAIL, then PASS |

The first Jitsi run failed on requested heights: five bots received 360
instead of 720 for most of the hold (1.4% to 43% of the time at 720). The
build before 014 (`fd6e0b4`), right after with the same plan, failed the
same way (one bot at 1.4%), and a second 014 run passed with every bot at
720 the whole time: Jitsi's default bandwidth estimate on this machine, as
in T035, not 014.

Engine CPU at 100 bots on LiveKit, VP8 (T063 against 012 T021, same plan
but a 2 minute hold instead of 10): 4.35 and 4.33 threads against 4.28 and
4.30 with 012, +1.2%. The VP8 quality path per packet: 68 ns against 52 ns
before 014 (SC-006 benchmarks).

**Met**, within run-to-run variation.

## SC-008: requested heights with layers

### First checks, 4 bots (T052–T055)

2026-11-08, local servers, 4 bots, 30 s hold, layers up to 720
(`*_SIMULCAST=720`), requested heights `180:0.5,720:0.5`; share of each
bot's received video time at the height it asked for (lowest bot). The
full runs (10 bots, 1 minute, receive cap 4) are T059.

| Server | VP8 simulcast | H.264 simulcast | VP9 SVC | AV1 SVC |
|---|---|---|---|---|
| Janus 1.4.3 | 1.000 | 1.000 | 1.000 | 0.979 |
| LiveKit 1.13.10 | 0.974 | 0.973 | 1.000 (latency not identified) | 1.000 |
| mediasoup 3.30.1 | 0.000–0.982 (7 runs) | 0.949 | 0.876 | 0.827 |
| Jitsi stable-11248 | 1.000 | 0.000 (only 180 forwarded) | 0.983 (latency not identified) | 1.000 |

LiveKit: the bot announces its layers in `AddTrackRequest` (`layers`, and
the codec's `layers` with `video_layer_mode`), one per height with
`content.json`'s bitrate; simulcast layers carry the RIDs the bot sends
(`l`, `m`, `h`). Requested heights go through `UpdateTrackSettings` (width,
height) after each subscription. Departure from T052: livekit/protocol
v1.52.1 has no field for the scalability mode, so none is sent; the server
reads the SVC structure from the dependency descriptor.

mediasoup: the bot produces three encodings with the offer's RIDs and
SSRCs for simulcast, one with `scalabilityMode: "L3T3_KEY"` and the
dependency descriptor for SVC; requested heights go through the demo's
`setConsumerPreferredLayers` notification (spatial layer from the height,
temporal layer left to the server). Every codec arrives layered, and every
bot asking for 180 receives it 100% of the time. Bots asking for 720 wait
for mediasoup's estimate of their receiving session to grow: it starts at
the demo's `initialAvailableOutgoingBitrate` (1 Mbit/s), and while it is
low the server forwards a lower layer. In most runs this takes 1 to 5 s of
the 30 s hold (0.83 to 0.98), but in VP8 runs one bot in a few stayed
lower for 6 s to the whole hold (0.000, 0.081, 0.838). With every bot
asking for 720 the same happens, so it is not the preferred-layer request.
The server's log of one such run: three receiving sessions jump from
1.08 to 6.5 Mbit/s after 1.3 s, the fourth stays at 1.36 Mbit/s until
7 s. The bots acknowledge every arriving packet, the server's probes
included (transport feedback recorded at the SRTP buffer, 005 R9). Not
checked against a browser, so a bot-side cause is not ruled out. VP9
latency is measured: mediasoup keeps the RTP padding.

**VP9 latency on LiveKit lost with the padding carrier.** LiveKit forwards
the RTP payload without its padding, so since T051 the bots find no VP9
marker: latency is `frame_not_identified` for VP9, SVC and single stream
alike (a 3-bot single VP9 run confirms it). The T044 VP9 run on LiveKit,
with the marker after the frame data, measured latency. Janus keeps the
padding.

Jitsi (T055, with `JVB_CC_TRUST_BWE=false` as in the T035 table): the
bot's own sources come from its answer, so nothing changed in the adapter.
An SVC stream is one source with its RTX source (FID) and no SIM group;
H.264 simulcast is a SIM group of three, as VP8; every source is
`<nick>-v0`, `videoType` camera (tests in `sources_test.go` and
`json_sources_test.go`). Requested heights go through
`ReceiverVideoConstraints`, unchanged. Results:

- VP8 simulcast, VP9 SVC and AV1 SVC arrive layered at the requested
  heights.
- **H.264 simulcast: the bridge forwards only the 180 layer**, to the bots
  asking for 720 too (0 switches). The bridge's media library
  (`jitsi-media-transform-2.3-326`) has packet parsers for VP8, VP9 and
  AV1 (dependency descriptor) and none for H.264, so it cannot find H.264
  keyframes to switch layers on. This is the case `layers_not_forwarded`
  (T057–T058) reports.
- **VP9 latency is lost on Jitsi too**: `frame_not_identified` for VP9
  SVC and for a 3-bot single VP9 run; the bridge does not forward the RTP
  padding. With the padding carrier, VP9 latency is measured on Janus and
  mediasoup only.

### Full runs, 10 bots (T059)

2026-11-08, local servers one at a time, 10 bots, 1 minute hold, layers
up to 720, receive cap 4, requested heights `180:0.5,720:0.5` (Jitsi with
`JVB_CC_TRUST_BWE=false` and a 5 s ramp-up, as in the T035 table). Share of
received video time at the requested height, lowest bot; 0 freezes in
every run. Reports and logs in the session's scratch folder, not kept.

| Server | VP8 simulcast | H.264 simulcast | VP9 SVC | AV1 SVC |
|---|---|---|---|---|
| Janus 1.4.3 | 1.000 | 1.000 | 0.990 | 0.991 |
| Jitsi stable-11248 | 1.000 | `layers_not_forwarded` | 1.000 (latency not identified) | 1.000 |
| LiveKit 1.13.10 | 0.985 | 0.985 | 1.000 (latency not identified) | 1.000 |
| mediasoup 3.30.1 | 0.526 | 0.996 | 0.606 | 0.861 |

- H.264 here is the 5-slice content; the remote rerun with one slice per
  frame (SC-005) gives the same layer results (Janus 1.000, LiveKit 0.993,
  mediasoup 0.59, Jitsi `layers_not_forwarded`).
- Jitsi H.264: the bridge forwards only the 180 layer (no H.264 parser in
  the bridge, see T055); the run sets `layers_not_forwarded` and the
  Markdown report reads "The server forwarded only the lowest layer:
  layered result not available. Codec h264 (simulcast) on jitsi."
- VP9 on Jitsi and LiveKit fails only on latency (`frame_not_identified`):
  layers are forwarded.
- mediasoup: the bots asking for 720 spend part of the minute on 360 or
  180 (38 to 50 layer switches per run, H.264 aside), not only the first
  seconds as in the 30 s runs: the server's per-session estimate keeps
  falling back with 10 bots each receiving 4 streams. The bots asking for
  180 get it. Same open question as T053 (server ramp-up or bot feedback).

## SC-009: equal picture quality

### Single-stream and simulcast content (T015, T016)

`scripts/gen-media.sh` (no argument), 2026-11-07, `jrottenberg/ffmpeg:7.1-ubuntu`,
libvmaf 3.0.0 with `vmaf_v0.6.1.json` (the image has no built-in model; the
script fetches it from Netflix/vmaf tag v3.0.0 and checks its SHA-256). VMAF
is the mean over the 300 frames against `testsrc2` at the file's size.
Bisection stops at the first bitrate within 1.0 of the VP8 reference.

| Height | VP8 kbit/s (VMAF) | VP9 kbit/s (VMAF) | AV1 kbit/s (VMAF) | H.264 kbit/s (VMAF) |
|---|---|---|---|---|
| 90 | 60 (70.92) | 39 (71.20) | 48 (70.48) | 78 (70.84) |
| 180 | 150 (82.05) | 60 (82.41) | 90 (82.53) | 150 (81.98) |
| 360 | 500 (91.74) | 200 (91.12) | 400 (92.60) | 500 (92.69) |
| 720 | 1500 (90.39) | 600 (90.38) | 900 (89.67) | 1350 (90.56) |

Every entry is within 1.0 of its reference. Checks of the files:
`sha256sum` of the four VP8 files is unchanged; every file has 300 frames
with keyframes at frames 0, 30, …, 270; VP9: no superframes, every frame
shown; AV1: each temporal unit starts with a temporal delimiter, a sequence
header at every keyframe, every frame shown; H.264: 300 access units split
at the delimiters, every keyframe access unit is SPS, PPS (an SEI on the
first), then IDR slices. x264 with `-tune zerolatency` cuts frames into
several slices (1 at 90 lines, up to 13 at 720): valid, and the payloader
takes NAL units one by one.

Limit (research R3): `testsrc2` is synthetic. On it VP9 needs 40–60% of
VP8's bitrate and AV1 60–80%, which says little about camera video; the
comparison is of server cost at equal quality on this content.

Binary size (R15): 35,275,959 bytes with the new files embedded, +5.6 MB
(the files total 5,601,721 bytes).

### Scalable content (T045)

`scripts/gen-media.sh svc`, 2026-11-08: libvpx `vp9_spatial_svc_encoder`
and libaom `svc_encoder_rtc` (T003 command lines), `L3T3_KEY`, 300
pictures, CBR, each layer's temporal targets 60/80/100% of its bitrate.
Each spatial layer is decoded alone (`vpxdec --svc-decode-layer`; ffmpeg's
libdav1d with operating point 6, 3 or 0) and its VMAF must be within 1.0 of
VP8's on the same pictures at the VP8 layer's bitrate (search within 0.5,
lowest layer first; the final file checked at 1.0 for every layer).

| Codec, top | Layer | Bitrate (kbit/s) | VMAF | VP8 reference (its bitrate) |
|---|---|---|---|---|
| VP9 up to 360 | 90 | 48 | 89.08 | 89.18 (60) |
| | 180 | 168 | 92.90 | 92.49 (150) |
| | 360 | 562 | 91.35 | 91.74 (500) |
| AV1 up to 360 | 90 | 42 | 87.46 | 87.74 (60) |
| | 180 | 135 | 92.95 | 92.97 (150) |
| | 360 | 562 | 91.47 | 91.74 (500) |
| VP9 up to 720 | 180 | 168 | 94.69 | 94.49 (150) |
| | 360 | 750 | 94.17 | 94.56 (500) |
| | 720 | 2062 | 90.70 | 90.39 (1500) |
| AV1 up to 720 | 180 | 150 | 92.49 | 92.38 (150) |
| | 360 | 625 | 94.22 | 94.43 (500) |
| | 720 | 1875 | 90.24 | 90.39 (1500) |

Three departures from T045 as written:

- **Reference pictures.** The encoders scale the top frames down
  themselves, and `testsrc2` keeps its text at a fixed pixel size, so a
  lower layer is not the VP8 file of its height (that pattern drawn at the
  layer's size): VMAF against it is meaningless (2.5). The lower layers'
  references are the encoder's own downscaled pictures, decoded from an
  encode at 20 times the bitrates (ffmpeg's bilinear scaler matched libvpx,
  99.9 on a near-lossless layer, and libaom at 640x360, but not libaom's
  quarter size from 1280x720: that layer stopped at VMAF 93.9 against a
  reference of 94.9 whatever the bitrate), and the
  VP8 reference is VP8 coding those same pictures at the VP8 layer's
  bitrate. The top layer is the pattern itself, so its reference is the VP8
  file's (91.74 at 360, 90.39 at 720).
- **Keyframes every 20 pictures, not 30.** Neither encoder restarts its
  0-2-1-2 temporal pattern at a keyframe: with 30, libaom's keyframe of
  picture 30 is in temporal layer 1 (seen in its OBU headers), which a
  server thinning temporal layers drops. 20 is a multiple of 4 and divides
  the 300 pictures; a receiver waits at most 0.67 s for a picture, against
  1 s for the other content. So the SVC keyframes do not fall at the VP8
  content's.
- **Layers are not independent.** Raising an upper layer's bitrate moves a
  lower layer's VMAF by about 0.1 (inter-layer prediction on keyframes, rate
  control), hence the narrower search.

**These bitrates are not comparable to the single-stream ones.** At the
same VMAF, scalable VP9 needs 562 kbit/s at 360 lines where single-stream
VP9 needs 200, and 2062 at 720 where it needs 600 (AV1: 562 vs 400, 1875 vs
900). The SVC example encoders run their real-time modes at high speeds
(`-sp 7` and `-sp 9`, their own defaults for RTC), the single-stream files
come from ffmpeg at slower settings (libvpx `good`, `cpu-used 4`); a
browser's SVC encoder is closer to the former. The servers' cost per codec
with layers (US4) is therefore at these bitrates, and is compared with the
other arrangements only with that said.

Binary size: 44,487,674 bytes with the four SVC files (9.0 MB), +14.9 MB
over the 013 baseline in all.

## Not met or not checked

- SC-004 (browsers): checked in Chrome and Firefox by the user for every
  layered stream (T051), VP9 single (second check) and H.264 single and
  simulcast after the re-encode (third check). VP8 and AV1 single streams
  with the trailer were not viewed in a browser (their layered streams
  were; ffmpeg decoders accept them). The H.264 trailer SEI comes after
  the picture's slices, which H.264 does not allow; the browsers and
  decoders tried ignore it.
- SC-005 on Jitsi: the first VP8 run after `deploy` is on a cold bridge
  (CPU 1.53 against 1.09–1.12, freezes during the ramp-up), so the two VP8
  runs differ by 0.41–0.44 thread and no codec can be told apart on Jitsi;
  a warm-up run before the first VP8 run would be needed. On the other
  servers H.264 costs more than VP8 (Janus +6%, LiveKit +13% with one
  slice per frame), VP9 less on LiveKit and mediasoup (lower bitrate).
- SC-006: VP9 −20% and H.264 +12.1% engine CPU per bot against VP8 at 100
  bots, beyond 10%; measured and stated (packets received; H.264's
  trailer SEI is one packet per frame).
- SC-008 on mediasoup (T053, T059, T060): bots asking for 720 lose time to
  the server's bandwidth estimate: under 95% in 30 s runs; at 10 bots for
  1 minute locally 0.53 (VP8), 0.61 (VP9), 0.86 (AV1), H.264 0.996; on the
  remote machine 0.55 (VP8), 0.71 (VP9), 0.85 (AV1), H.264 0.70 and 0.59.
  Cause between mediasoup's ramp-up and the bots' feedback not settled
  (not compared with a browser). **015**: the cause is a mediasoup 3.30.1
  worker bug (probe packets share one buffer across transports, so new
  transports make the others' probes fail SRTP); with a 20 s ramp-up every
  720 bot stayed at 0.986 or more
  (specs/015-bandwidth-estimate-parity/validation-report.md, SC-004).
- SC-008 for H.264 on Jitsi (T055, T059, T060): the bridge forwards only
  the lowest H.264 simulcast layer (no H.264 parser in
  `jitsi-media-transform`); reported as `layers_not_forwarded`.
- VP9 latency on LiveKit (T052) and Jitsi (T055): `frame_not_identified`
  since the marker moved to the RTP padding (T051); both drop padding when
  forwarding. Measured on Janus and mediasoup.
- Local Jitsi at 10 bots (T035, T044, T064): with the repository's local
  setup (Jitsi's default bandwidth estimate), some VP8 and AV1 runs fail
  on freezes or hold bots below their requested height, on the build
  before 014 too; the 014 local tables are with `JVB_CC_TRUST_BWE=false`
  (a scratch override). The local setup is unchanged; the reports do not
  record the setting. **015**: verdict "the bots" (the bots did not
  acknowledge the copies the bridge sends); corrected: no suspended stream
  in VP8 at 10 bots with the default settings; AV1 still suspends some bots
  at start-up with ramp-up 0; reports now state the server's settings
  (specs/015-bandwidth-estimate-parity/validation-report.md, SC-003, SC-005).
- The 16 single-stream VP9 runs of T044 measured VP9 latency with the
  earlier carrier (marker after the frame data); not redone with the
  padding carrier.
- `gofmt -l .` lists `src/api/handlers/scenario_handler.go`, `src/main.go`,
  `tests/integration/api/scenario_api_test.go` and
  `tests/integration/execution/scenario_execution_test.go`, all unchanged
  since before 014 (`fd6e0b4`); left as they are.
