# Implementation Plan: Video Codec Comparison

**Branch**: `014-video-codec-comparison` (work continues on `rework`) | **Date**: 2026-11-07 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/014-video-codec-comparison/spec.md`

## Summary

A test plan names the video codec the bots send: `media.video_codec`, one of
`vp8` (default), `vp9`, `av1` and `h264` (R1). Everything else in a run stays
as it is, so a codec's run differs from the VP8 run by that one field, and the
reports read side by side show what the codec changes for the server.

**Content** stays pre-encoded (constitution IV). `scripts/gen-media.sh` makes
each codec from the same test pattern, at each codec's bitrate for the same
picture quality as today's VP8 files, judged by VMAF within 1.0 (R2, R3). The
VP8 files do not change. The bitrates go into an embedded manifest, so each
report says what it cost.

**Sending** keeps one packetization per frame per source, shared by all bots.
VP9 and AV1 get their own payloaders, and AV1 the dependency descriptor that
servers read its frames and layers from; Pion has neither (R5, R6). A new
video track rewrites the descriptor's ID per server and fails the bot when the
negotiated codec is not the plan's (R7, R11).

**Measures** become codec-neutral: one small header parser per codec feeds the
existing freeze, frame rate, resolution and latency code (R8). The 012 frame
trailer gets a carrier each codec's decoders skip: SEI for H.264, a padding
OBU for AV1, a guard byte for VP9 (R9). A browser gate checks this before
anything builds on it (R16).

**Layers** (US3) are the largest part. `media.simulcast` now means layered
video in every codec: simulcast for VP8 and H.264, `L3T3_KEY` SVC for VP9 and
AV1, made with the encoders' own example programs built in Docker (R4).
Simulcast and requested heights exist only on Jitsi today, so LiveKit,
mediasoup and Janus gain them for every codec, VP8 included (R10).

**Several server facts are not yet checked** (R17): what jicofo offers, which
codecs LiveKit enables, mediasoup's AV1 support, Janus's AV1 SVC, and the
example encoders' output. The first live task checks them; a server that
cannot do something is reported, not patched.

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: `pion/webrtc/v3` v3.3.6 (custom `TrackLocal`), `pion/rtp` v1.8.7 (H.264 payloader, VP9 and AV1 packet types), `pion/webrtc/v3/pkg/media/ivfreader` and `h264reader`; hand-written VP9 and AV1 payloaders and dependency descriptor. No new module. Build-time only: the `jrottenberg/ffmpeg:7.1-ubuntu` image (libvpx-vp9, libaom-av1, libx264, libvmaf), and libvpx and libaom example encoders built in `scripts/svc-encoders/Dockerfile`

**Storage**: embedded content files and `src/media/assets/content.json`; reports as 009

**Testing**: `go test -race ./...`: payloaders and parsers round-trip against Pion's packet types and the real content; descriptor writer against its reader and spec examples; adapter tests with the existing fake servers; report golden files. Live: `-tags live` tests with `<PLATFORM>_VIDEO_CODEC`, local servers, then the 013 remote machine. Browser gate (R16) by hand, Chrome and Firefox

**Target Platform**: Linux

**Project Type**: Go engine and CLI

**Performance Goals**: engine CPU per bot within 10% of VP8 for each codec at 100 bots, or measured and documented (FR-012)

**Constraints**: no run-time encoding (FR-006); VP8 results unchanged (FR-011); one codec per room; servers' code unchanged, configuration only (spec Assumptions)

**Scale/Scope**: 4 codecs × 4 servers × 3 arrangements; content grows the binary by about 6–10 MB (R15)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status |
|---|---|
| I. Infrastructure-first | Pass. Bots stay synthetic. The browser is a one-off check that the bots' media is valid, not a client of the engine. |
| II. Media engine independence | Pass. Payloaders, parsers, descriptor and trailer carriers live in `src/media`; the codec reaches it as a value, and no platform concept enters it. |
| III. Pluggable signaling | Pass. Each server's codec choice, simulcast and layer requests stay inside its adapter (R10). |
| IV. CPU efficiency | Pass, with a check. Content is pre-encoded, packetized once per frame and shared; per binding only header fields change (R7, R14). FR-012 measures it at 100 bots. |
| V. Deterministic lifecycle | Pass. `codec_mismatch` is a failure reason of the existing failed state; no new state. |
| Go, Pion, single static binary | Pass. No new module; the content stays embedded (R15). |

Re-checked after Phase 1 design: unchanged. The design adds no run-time
encoder, no module, and nothing platform-specific in `src/media`.

## Project Structure

### Documentation (this feature)

```text
specs/014-video-codec-comparison/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── test-plan-codec.md
│   └── report-codec.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
scripts/
├── gen-media.sh                 # + VP9, AV1, H.264 per height; VMAF bisection; content.json; "svc" mode
└── svc-encoders/Dockerfile      # new: libvpx and libaom example encoders, pinned

src/media/
├── assets/                      # + vp9_*, av1_*, h264_* files, svc files, content.json
├── content.go                   # codec-aware loading; IVF any codec, Annex B access units, SVC layer split
├── codec.go                     # new: codec values, arrangement, per-codec payloader and parser selection
├── vp9.go                       # new: payloader (flexible, layers, SS) and receive parser
├── av1.go                       # new: payloader, OBU split, receive parser
├── h264.go                      # new: access units, SPS size, SEI trailer
├── dependency_descriptor.go     # new: writer and reader, fixed templates
├── frame_trailer.go             # per-codec carriers
├── source.go                    # layer frames, per-layer trailer
└── video_quality.go             # codec-neutral frame description

src/execution/
├── video_track.go               # new: TrackLocal rewriting SSRC, PT, extension IDs; codec check at Bind
├── peer.go                      # video measured for every codec; received codec checked; layered tracks per arrangement
└── pion.go                      # dependency descriptor extension registered

src/scenario/                    # media.video_codec field and validation; codec in the quality summary
src/report/                      # codec fields, JSON and Markdown

src/platform/
├── jitsi/                       # codec in presence; SVC sources (one SSRC + RTX)
├── livekit/                     # AddTrackRequest codecs and layers; UpdateTrackSettings; simulcast and SVC accepted
├── mediasoup/                   # produce per codec and arrangement; preferred layers; simulcast and SVC accepted
└── janus/                       # room and publish codec; simulcast and SVC; configure substream/spatial_layer

cmd/jitsi-connect/               # --video-codec
tests/live/                      # <PLATFORM>_VIDEO_CODEC (and SIMULCAST, REQUESTED_HEIGHTS where new)
tests/platforms/
├── mediasoup/config.mjs         # + VP9 profile-id=0, AV1
└── docker-jitsi-meet/           # jicofo codecs enabled if not by default (R17)
```

**Structure Decision**: the existing single Go project. Codec knowledge goes
into `src/media` (send and receive), the binding into `src/execution`, and
each server's way of choosing codecs and layers into its adapter. No new
package outside them.

## Delivery order

1. **US1** (P1), single stream, all four servers: field, content, payloaders,
   descriptor, video track, codec checks, report fields. Gate: R17 facts, then
   the browser check (R16) as soon as the content and payloaders exist.
2. **US2** (P2): per-codec parsers and trailer carriers, so every measure works
   for every codec.
3. **US3** (P3): SVC content; simulcast and requested heights on LiveKit,
   mediasoup and Janus (VP8 first, then the other codecs); SVC on the four
   servers.
4. **US4** (P4): the runs on the remote machine and the validation report's
   tables.

US1 and US2 together already answer the user's question for single streams.

## Complexity Tracking

| Deviation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| FR-010 narrowed: `source_file` refused with SVC (VP9 or AV1 with `simulcast`) | ffmpeg cannot write spatial layers; the SVC encoders are build-time tools | Shipping the encoders as run-time dependencies breaks the single binary and adds encoding at scenario start |
| Changes outside `src/` in existing server setups (`tests/platforms/mediasoup/config.mjs`, possibly the Jitsi env) | mediasoup's router has no AV1 and only VP9 profile 2 | Patching servers' code is out of scope; their configuration is the normal way to enable a codec |
| Hand-written dependency descriptor, VP9 and AV1 payloaders | Pion has no descriptor and its payloaders carry no layers | A third-party module, against the no-new-module rule, for about 400 lines of well-specified code |
