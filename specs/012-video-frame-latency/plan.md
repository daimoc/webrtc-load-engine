# Implementation Plan: Video Latency on Servers That Rewrite Timestamps

**Branch**: `012-video-frame-latency` (work continues on `rework`) | **Date**: 2026-11-06 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/012-video-frame-latency/spec.md`

## Summary

Every video frame leaves with a 6-byte trailer after its VP8 data: a 3-byte
scenario tag and a 3-byte frame number (R1). The frame number replaces the
RTP timestamp as the key of the video send times. Each received packet's last
bytes are compared with the scenario tag, and a hit gives the frame's send
time (R3). The check needs no VP8 decoding, no signaling and no knowledge of
the platform.

There is one path for video on every platform, with no fallback to timestamps
(R4). Audio is untouched (R6).

**Already probed** (R2): a throwaway prototype with an 8-byte trailer gave
video latency on both target servers, with freezes, frame rate and MOS as
usual:

| Run (4 bots) | Video latency p50 | Freezes | Frame rate |
|---|---|---|---|
| Janus 1.4.3 | 0.5 ms (none before) | 0 | 30.0 fps |
| Jitsi, simulcast 720 | 1.5 ms (none before) | 0 | 30.1 fps |

**Browsers**: at planning time, whether a browser decodes the frames normally
was unproved. Implementation settled it first (R5).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: standard library (`hash/fnv`), `pion/rtp` (already used). No new module.

**Storage**: N/A (in-memory send times, as in 006)

**Testing**: `go test -race ./...`: unit tests for the trailer, `SendTimes` and `VideoQuality`; the existing in-process Pion test with a `vnet` delay; a new one where the path rewrites timestamps; the per-packet benchmark; live tests with `-tags live` on the four local servers; a browser check.

**Target Platform**: Linux, one load machine

**Project Type**: Go service with a CLI

**Performance Goals**: quality accounting stays under 200 ns per packet with no allocation (006 R10); engine CPU at 100 bots +5% at most (SC-006, alternated runs in the same machine state); lowest layer bitrate +2% at most (SC-007: 1.8% on the 90p layer, R1)

**Constraints**: no video decoding or encoding (FR-011); no platform knowledge in the media layer (FR-002, constitution II); no change to the summary, report files, metrics or test plan (FR-007, FR-013)

**Scale/Scope**: about 100 lines in `src/media`, plus tests and documents

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Result |
|---|---|
| I. Infrastructure-first | Pass: bots only, no browser in the engine |
| II. Media engine independence | Pass: all in `src/media`, platform-blind. Determinism: the trailer is a function of the scenario ID and the frame's position. The scenario ID differs between runs, so bytes differ by run in the trailer only, which FR-010 allows |
| III. Pluggable signaling | Pass: no adapter changes |
| IV. CPU efficiency | Pass: one short compare per packet, no decoding; checked by the benchmark and SC-006 |
| V. Lifecycle | Not concerned |
| Test discipline | Pass: tests first; benchmark kept |
| Incremental complexity | Pass: one path replaces another; no option, no fallback |

Re-checked after Phase 1: no change.

## Project Structure

### Documentation (this feature)

```text
specs/012-video-frame-latency/
├── plan.md              # This file
├── research.md          # Decisions R1–R8 and the probe
├── data-model.md        # Trailer, send times, stream state
├── quickstart.md        # Validation runs
├── contracts/
│   └── frame-trailer.md # Bytes on the wire
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
src/media/
├── frame_trailer.go     # New: tag from the scenario ID, append, read
├── source.go            # Video: reserve room, number frames, append the trailer
├── latency.go           # SendTimes: carries the tag; video keyed by frame number
├── video_quality.go     # Reads the trailer; lookups stay counted per frame
└── quality_bench_test.go # Keeps the per-packet budget honest

tests/live/              # Jitsi and Janus tests check that video latency is available
```

`src/execution` is unchanged: `SetLatency(st, o)` keeps its signature, and the
tag travels with `SendTimes`.

**Structure Decision**: the existing single project. Everything lives in
`src/media`, beside the 006 latency code.

## Complexity Tracking

No violations.
