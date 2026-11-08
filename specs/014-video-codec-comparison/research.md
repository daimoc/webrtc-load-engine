# Research: Video Codec Comparison (014)

Decisions for [spec.md](spec.md), from the current code (Pion webrtc v3.3.6,
pion/rtp v1.8.7, the four adapters of 003, 008, 010 and 011, the media package
of 004, 006 and 012) and from what the servers in `tests/platforms` run. Facts
marked **verify** are not checked on a running server yet; the first live task
(R17) checks them before any code depends on them.

## What exists today

- Bots send VP8 only. The content is pre-encoded IVF made by
  `scripts/gen-media.sh` with the `jrottenberg/ffmpeg:7.1-ubuntu` image
  (004 R1): 640x360 at 500 kbit/s, plus 90, 180 and 720 layers for simulcast
  (006 R6), 30 fps, a keyframe every 30 frames, 10 s.
- The receive side is VP8-specific: `src/execution/peer.go` measures video only
  when the track is VP8, and `src/media/video_quality.go` reads frame starts,
  keyframes and sizes from the VP8 payload descriptor (`src/media/vp8.go`).
- The 012 frame trailer is 6 bytes appended to the last RTP payload of each
  frame, read back as the last 6 bytes of a packet.
- Simulcast and requested heights work on Jitsi only; the LiveKit, mediasoup
  and Janus configs refuse both (`src/platform/*/config.go`).
- Pion registers VP8, VP9 (profile 0 and 2), AV1 and several H.264 profiles by
  default (`RegisterDefaultCodecs`). pion/rtp v1.8.7 has payloaders for VP9
  (no layers, no scalability structure), H.264 and AV1 (one OBU per call, no
  dependency descriptor). Nothing in Pion writes or reads the dependency
  descriptor RTP header extension.
- `TrackLocalStaticRTP` rewrites SSRC and payload type per binding, not header
  extension IDs, which each server negotiates on its own.
- mediasoup-demo runs mediasoup 3.30.1 with our `tests/platforms/mediasoup/config.mjs`:
  VP8, VP9 `profile-id=2` and two H.264 profiles; no AV1, no VP9 profile 0.
- The ffmpeg image has libvpx (VP8), libvpx-vp9, libaom-av1, libsvtav1,
  libx264 and the libvmaf, ssim and psnr filters. ffmpeg is not installed on
  the workstation.

## R1. The codec in the test plan

- **Decision**: a new field `media.video_codec`: `vp8`, `vp9`, `av1` or `h264`,
  default `vp8` ([contract](contracts/test-plan-codec.md)). Creation refuses
  any other value, naming the four, and refuses the field with a profile other
  than `audio_video`. The existing `media.simulcast` keeps its meaning, "send
  layered video", for every codec: simulcast for VP8 and H.264, scalable
  layers (SVC) for VP9 and AV1, with the same `max_height` (360 or 720) and the
  same heights (90/180/360 or 180/360/720). `requested_heights` and
  `max_received_streams` keep their meaning.
- **Rationale**: FR-001, FR-002, FR-013, FR-014. One knob for layers keeps
  existing test plans valid (SC-007) and makes a codec run differ from the VP8
  run by one field.
- **Alternatives considered**: a separate `svc` field (two ways to ask for
  layers, and an invalid combination per codec); a codec per bot (out of
  scope: one codec per room).

## R2. Single-stream and simulcast content

- **Decision**: `scripts/gen-media.sh` gains one function per codec, all from
  the same `testsrc2` pattern, 30 fps, 10 s, a keyframe every 30 frames, every
  frame shown (no hidden or alt-ref frames), so frames pace at a fixed 1/30 s
  and layers stay in lockstep as VP8's do:
  - VP9: `libvpx-vp9`, profile 0, CBR, `-auto-alt-ref 0 -lag-in-frames 0`,
    IVF.
  - AV1: `libaom-av1` realtime usage, CBR, `-lag-in-frames 0`, IVF (temporal
    units of OBUs).
  - H.264: `libx264`, Constrained Baseline (`42e01f`), no B-frames, no scene
    cut, SPS and PPS before every IDR, an access unit delimiter per frame,
    Annex B (`.h264`).
  The VP8 files do not change (FR-011). Each file is parsed once at startup:
  Pion's `ivfreader` for VP9 and AV1, `h264reader` grouped into access units
  at each delimiter for H.264.
- **Rationale**: FR-005, FR-006 (no encoding at run time, constitution IV).
  SPS/PPS with every IDR lets a bot joining late decode within 1 s, as VP8's
  keyframe period does. A delimiter makes access units explicit instead of
  guessed from slice headers.
- **Alternatives considered**: SVT-AV1 (no realtime one-pass CBR as predictable
  as libaom's at these sizes); High profile H.264 (fewer servers and browsers
  accept it, spec Assumptions).

## R3. Equal picture quality

- **Decision**: VMAF (libvmaf, default model) of each encoding against the
  `testsrc2` frames it was made from, at the same size. The reference is the
  VMAF of today's VP8 file at each height (90, 180, 360, 720). For every other
  codec and height, `gen-media.sh` bisects the CBR bitrate until the VMAF is
  within **1.0** of the VP8 reference. The result is written to
  `src/media/assets/content.json`: codec, arrangement, height, bitrate, VMAF,
  reference VMAF. It is embedded, so reports and the validation report give
  each codec's bitrate (US4, SC-009).
- **Rationale**: the user chose equal quality, each codec at its own bitrate
  (spec Clarifications). VMAF is the score servers and codec comparisons
  commonly use; one score for all codecs keeps the rule simple. 1.0 is below
  the difference viewers notice and well within what bisection reaches in 6–8
  encodes.
- **Limit**: `testsrc2` is synthetic. Codecs' relative efficiency on it is not
  that of camera video; the validation report says so. Changing the source
  would change the VP8 files, against FR-011.
- **Alternatives considered**: PSNR or SSIM (weaker match to perceived
  quality across codecs); equal bitrate (rejected by the user).

## R4. Scalable (SVC) content for VP9 and AV1

- **Decision**: three spatial layers and three temporal layers, `L3T3_KEY`:
  spatial layers depend on each other on keyframes only, the mode Chrome uses
  for VP9 and AV1 SVC with Jitsi and LiveKit. Heights 90/180/360 or 180/360/720
  as simulcast. ffmpeg cannot write spatial layers, so the content comes from
  the encoders' own examples, built pinned in `scripts/svc-encoders/Dockerfile`:
  libvpx `vp9_spatial_svc_encoder` and libaom `svc_encoder_rtc`, fed raw I420
  frames of `testsrc2` from ffmpeg. Each spatial layer's bitrate is bisected to
  the VMAF of the VP8 simulcast layer of that height (R3). At startup, VP9
  superframes are split into layer frames by their superframe index, AV1
  temporal units into layer frames by the OBU extension header's spatial ID.
- **Rationale**: FR-013, constitution IV (pre-encoded). `L3T3_KEY` is what
  servers expect to switch layers on, so the comparison measures their real
  layer work.
- **Verify** (R17): the exact command lines and output layout of both example
  encoders, and that the result has every frame shown at 30 fps.
- **Alternatives considered**: encoding SVC at run time (constitution IV);
  simulcast for VP9 and AV1 (rejected by the user's choice; it would not test
  SVC forwarding); full SVC (`L3T3`, inter-layer prediction on every frame:
  Chrome does not send it, and servers forward it worse).

## R5. Packetization per codec

- **Decision**: packetization stays once per frame per source, shared by all
  bots (004 R2). Per codec:
  - **VP8**: unchanged.
  - **VP9**: an own payloader (RFC 9628): flexible mode, 15-bit picture ID,
    layer indices (TID, U, SID, D), reference differences, B and E per layer
    frame, the scalability structure with each layer's width and height on
    keyframes, the marker on the last packet of the picture. Single-stream VP9
    is the same writer with one layer. Pion's payloader has no layers.
  - **AV1**: an own payloader (AV1 RTP specification): OBUs without size
    fields, temporal delimiters dropped, the aggregation header's N bit on a
    new coded video sequence, one layer frame per packet group. Pion's sends
    one OBU per call and no descriptor.
  - **H.264**: Pion's `H264Payloader` (STAP-A for SPS/PPS, FU-A), packetization
    mode 1, per access unit.
- **Rationale**: FR-003, FR-013. Servers forward VP9 and AV1 layers only from
  these fields.

## R6. Dependency descriptor

- **Decision**: a hand-written writer and reader of the dependency descriptor
  header extension (`https://aomediacodec.github.io/av1-rtp-spec/#dependency-descriptor-rtp-header-extension`),
  as 008 hand-wrote LiveKit's protobuf: fixed template structures for one
  layer and for `L3T3_KEY`, written once; per packet only the start and end of
  frame bits, template ID and frame number change. The structure, with each
  layer's resolution, goes on the first packet of every keyframe. Sent with
  every AV1 packet, and with VP9 SVC packets where the server negotiates it.
  The extension is registered for video on both PeerConnections.
- **Rationale**: Jitsi's bridge, LiveKit and Janus read AV1 frames and layers
  from it; LiveKit also reads VP9 SVC from it (**verify** each, R17). No new
  module (no Pion support exists).
- **Alternatives considered**: a third-party Go implementation (new module,
  against the project's rule of no new module without need).

## R7. A video track that rewrites extension IDs

- **Decision**: video uses a new `TrackLocal` in `src/execution` instead of
  `TrackLocalStaticRTP`. At `Bind` it records the binding's SSRC, payload type,
  codec and the IDs of the extensions it writes (dependency descriptor); per
  packet it rewrites those. If the negotiated codec is not the content's codec,
  `Bind` fails and the bot fails with `codec_mismatch` (R11).
- **Rationale**: each server negotiates its own extension IDs (R6), and the
  bind is where the sent codec becomes known (FR-004).

## R8. Measures on receipt, per codec

- **Decision**: `VideoQuality` takes a codec-neutral frame description (frame
  start and end, keyframe, width and height when known, spatial and temporal
  IDs), produced per packet by one small parser per codec:
  - VP8: today's parser.
  - VP9: payload descriptor; keyframe = not P, spatial ID 0, frame start; size
    from the scalability structure or the keyframe's uncompressed header.
  - AV1: dependency descriptor (start/end, template → spatial and temporal IDs,
    resolutions from the structure); for one layer, the sequence header's size.
  - H.264: NAL type (IDR = keyframe), size from the SPS (Exp-Golomb read of the
    few fields needed).
  The height of a received SVC picture is that of its highest spatial layer;
  a drop of upper layers is a layer change, not a freeze (spec Edge Cases).
  `peer.go` measures video for any of the four codecs.
- **Rationale**: FR-007, FR-015, SC-003. Headers only, no decoding
  (constitution IV).

## R9. Frame trailer per codec

- **Decision**: the 012 trailer (3-byte tag, 3-byte frame number) is carried
  in each layer frame, so a receiver of the lowest layer still measures
  latency. Its carrier depends on the codec:
  - **VP8**: unchanged, appended to the frame data.
  - **VP9**: the RTP padding of the layer frame's last packet (RFC 3550 5.1:
    the 6 bytes, then the padding length 7, padding bit set). *Revised in
    T051 with the user*: first appended to the layer frame data with a
    `0x00` after it, which ffmpeg's VP9 decoder accepts but libvpx, the
    browsers' decoder, does not: after a frame it skips only zero bytes and
    reads anything else as the next frame ("Invalid frame marker"), in a
    single stream as in the layer frames of an SVC picture. Receivers strip
    padding before decoding; each server must forward it (checked per
    server, else VP9 latency is reported as not identified).
  - **AV1**: its own `OBU_PADDING` element, the last of the last packet of the
    layer frame; decoders drop padding OBUs. The trailer stays the packet's
    last 6 bytes.
  - **H.264**: an SEI NAL (user data unregistered, the project's own UUID)
    after the access unit's slices, in its own last packet, with emulation
    prevention; read by parsing the SEI.
  The reader becomes per codec; VP8 and AV1 keep "last 6 bytes".
- **Rationale**: FR-008. Raw bytes after H.264 slice data can form a start code
  and split the NAL in the receiver; VP9's superframe check reads the last
  byte. Codec-defined padding or SEI is what decoders are required to skip.
- **Verify**: SC-004's browser check (R16) before any other task depends on
  it.

## R10. Each platform's codec choice

- **Jitsi**: jicofo offers the codecs it has enabled in its session-initiate;
  the bot's answer accepts them, and it sends the plan's codec. The bot's
  presence announces its codec first in `jitsi_participant_codecList`, as
  lib-jitsi-meet does (**verify** whether jicofo or the bridge need it). VP9
  and AV1 SVC are one SSRC with RTX, no SIM group; requested heights go
  through `ReceiverVideoConstraints` as today. `tests/platforms/docker-jitsi-meet`
  enables the four codecs in jicofo (`ENABLE_CODEC_VP9`, `ENABLE_CODEC_AV1`,
  `ENABLE_CODEC_H264`; **verify** defaults).
- **LiveKit**: `AddTrackRequest` gains the codec (`simulcast_codecs`) and its
  layers (`layers`, one `VideoLayer` per height); SVC is signalled by the
  codec and the encoding's scalability mode. Simulcast uses RIDs. Requested
  heights go through `UpdateTrackSettings` (width, height). Field numbers from
  livekit/protocol v1.52.1, as 008. **Verify** that the server's default
  enabled codecs include VP9, AV1 and H.264.
- **mediasoup**: `config.mjs` adds VP9 `profile-id=0` and AV1 to the router's
  codecs. The adapter produces with the plan's codec: three encodings with RIDs
  for simulcast, one encoding with `scalabilityMode: L3T3_KEY` for SVC; the
  bot's RTP capabilities list the four codecs and the dependency descriptor.
  Requested heights through the demo's `setConsumerPreferredLayers`.
  **Verify** AV1 and the descriptor on 3.30.1.
- **Janus**: the room is created with `videocodec` set to the plan's codec
  (`vp9_profile: "0"`, `h264_profile: "42e01f"`), and the publish request names
  it. Simulcast by RID; SVC for VP9 and AV1. Requested heights by `configure`
  on the subscriber handle: `substream` for simulcast, `spatial_layer` for SVC.
- **Rationale**: FR-003, FR-014, FR-017. Each server's own normal mechanism,
  inside its adapter (constitution III).
- **Scope note**: simulcast and requested heights on LiveKit, mediasoup and
  Janus are new for every codec, VP8 included. That is the bulk of US3, and
  the reason US3 is planned after US1 and US2.

## R11. Codec checks

- **Decision**: a bot fails with reason `codec_mismatch` when the codec it
  binds to send (R7) or a track it receives (`onTrack`) is not the plan's
  codec, naming the codec found. A server that forwards the codec but not its
  layers is detected at the end from what bots received (no layer above the
  lowest in a layered run) and reported as `layers_not_forwarded`, naming the
  codec and the server (FR-016).
- **Rationale**: FR-004, SC-002: a run measuring the wrong codec fails rather
  than passing.

## R12. The report

- **Decision**: the quality summary and both report files gain the video codec
  sent, the arrangement (`single`, `simulcast`, `svc`), the content bitrate
  per height from `content.json`, and per bot the codec received
  ([contract](contracts/report-codec.md)). Existing fields keep their names.
- **Rationale**: FR-009, US4.

## R13. Media source files

- **Decision**: a `source_file` is transcoded at scenario start into the run's
  codec with the per-codec settings of R2, at the bitrates `content.json` gives
  for each height. With SVC (VP9 or AV1 with `simulcast`), creation refuses
  `source_file`: ffmpeg cannot write spatial layers, and the SVC encoders are
  build-time tools, not run-time dependencies.
- **Rationale**: FR-010 for single stream and simulcast. The SVC refusal is a
  narrowing of FR-010, recorded in the plan's Complexity Tracking.

## R14. Engine CPU

- **Decision**: per-packet work stays shared (R5); the dependency descriptor is
  written once per packet per layer and only its ID byte changes per binding
  (R7). Benchmarks per codec next to `source_bench_test.go` and
  `quality_bench_test.go`; a 100-bot run per codec against VP8 for FR-012.
- **Rationale**: constitution IV, FR-012, SC-006.

## R15. Binary size

- **Decision**: accept the growth. Today's content is 2.9 MB. Three codecs at
  four heights, plus two SVC sets per VP9 and AV1, add roughly 6–10 MB at
  VP8-equivalent quality (VP9 and AV1 smaller, H.264 larger).
- **Rationale**: single static binary (constitution) without files beside it.
- **Alternatives considered**: loading content from disk (breaks the single
  binary); fewer heights (breaks layer parity with VP8).

## R16. Browser decoding gate

- **Decision**: as 012's T011, the first implementation check after the
  content exists: a browser on the local Janus or LiveKit server shows each
  codec, single stream and SVC, with the trailer, without artefacts, in Chrome
  and Firefox. If a codec's carrier (R9) breaks decoding, stop and revise R9
  for that codec before building on it.
- **Rationale**: SC-004; 012 showed this is where a design fails.

## R17. Facts to verify first

One live task before the code depends on them:

1. Which codecs jicofo offers in the local stack, and the
   `ENABLE_CODEC_*` defaults.
2. LiveKit's default enabled codecs, and whether it needs the dependency
   descriptor for AV1 and VP9 SVC.
3. mediasoup 3.30.1 with AV1 and the descriptor.
4. Janus v1.4.3 with AV1 SVC and H.264 simulcast.
5. The SVC example encoders' command lines and output (R4).

A server that cannot do something is recorded and reported (FR-017), not
worked around in its code.

## R18. Validation

- **Decision**: single-stream runs first (US1, US2): 10 bots, 1 minute, each
  codec on each server, local then on the 013 remote machine for the CPU
  comparison, with two VP8 runs the same day for run-to-run variation
  (SC-005). Then layered runs with requested heights (US3, SC-008). Results
  in `validation-report.md`, one table per server (US4).
- **Rationale**: server CPU is compared only within one set of runs on one
  machine (project rule); the remote machine is the one where the laptop does
  not distort it (013).
