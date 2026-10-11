# Implementation Plan: Media Quality

**Branch**: `006-media-quality` (work continues on `rework`) | **Date**: 2026-10-11 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/006-media-quality/spec.md`

## Summary

Each received stream gets a quality tracker fed by the read loop that
already accounts for it (004): video frames, freezes, keyframe heights and
layer switches from the VP8 payload headers (R1, R2); audio frames concealed
under a fixed 60 ms playout delay (R3); an E-model score per audio stream
(R4). Latency comes from the RTP timestamp of each received frame, looked up
in the send times recorded by the scenario's shared media source, since all
bots send the same frames at the same time (R5, FR-009 amended). Simulcast
sends three pre-encoded layers in lockstep with Pion's RID encodings,
announced as an SSRC `SIM` group; receivers ask for a height through a new
platform-neutral `RequestedVideoHeight()` that the Jitsi adapter puts in its
video constraints (R6, R7). The media collector exports the measures every
2 s and keeps a per-bot record; at the end of a run the engine builds a
quality summary served by `GET /api/v1/scenarios/{id}/quality` (R9).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: `pion/webrtc/v3` v3.3.6 (RID encodings, RTCP PLI), `pion/rtp` (VP8 payload descriptor), `pion/rtcp`, `prometheus/client_golang`; `pion/transport/v2/vnet` in tests only (already a dependency of Pion). New embedded content generated with ffmpeg (`scripts/gen-media.sh`). No new module.

**Storage**: N/A (in-memory; the summary lives with the scenario)

**Testing**: `go test -race ./...`: unit tests for the trackers and the score (`src/media`), a benchmark of the per-packet cost, in-process Pion peer pairs for freezes, latency (vnet delay), keyframe requests and simulcast; fake XMPP server for the requested height; API tests for the summary; live test `-tags live` on the local docker-jitsi-meet

**Target Platform**: Linux, single static binary (larger by the new layers, about 2.5 MB)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: quality accounting under 200 ns per kept packet (R10); 100 bots, audio and video, cap 10, ≤ 2 cores and ≤ 1.26 cores (20% over 005) with quality on (SC-006); simulcast at 360 ≤ 2 cores (SC-008)

**Constraints**: no decoding (FR-007); metrics labelled by `scenario_id` (plus `kind`, and `height` for the time per resolution), never by bot (FR-014); no new goroutine per stream; quality never fails a bot (FR-017); platform concepts stay in adapters (constitution III)

**Scale/Scope**: up to 1,000 received streams per process (100 bots × 10); summary with 10 worst bots per ranking

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: measures what the media server delivers (forwarding, pacing, layer selection), from synthetic peers; no browser emulation.
- [x] **II. Media Engine Independence**: trackers, score, latency registry and simulcast content are in `src/media` and know RTP and codecs only; the `SIM` group is SDP, added by the peer.
- [x] **III. Pluggable Signaling**: the requested height crosses the `SignalingHandler` contract as a number; only the Jitsi adapter knows it becomes a Colibri constraint ([contracts/signaling-handler.md](contracts/signaling-handler.md)).
- [x] **IV. CPU Efficiency**: no decoding; per-packet byte reads; per-frame lookup; budget enforced by a benchmark and SC-006/SC-008. Simulcast sends more packets: justified by US3 and bounded by SC-008 at the default top layer.
- [x] **V. Deterministic Peer Lifecycle**: unchanged; quality problems are reported, never transitions (FR-017).
- [x] **Observability**: new Prometheus series per scenario ([contracts/metrics.md](contracts/metrics.md)), freezes logged with scenario and peer IDs (rate-limited), summary through the API.
- [x] **Test Discipline**: unit tests for every measure, controlled in-process tests for SC-002/SC-004, adapter contract test for the requested height, benchmark for the hot path.
- [x] **Incremental Complexity**: fixed playout delay and fixed E-model values, no configuration for them; one tracker type per kind; the summary is a plain struct.
- [x] **Scenario Definition**: the new test plan fields are optional and versioned with the media config ([contracts/test-plan-quality.yaml](contracts/test-plan-quality.yaml)).

Post-design re-check: still passing; no entry in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/006-media-quality/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R11
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1: validation guide
├── contracts/
│   ├── metrics.md               # new Prometheus series
│   ├── quality-summary.yaml     # GET /api/v1/scenarios/{id}/quality
│   ├── test-plan-quality.yaml   # media.simulcast, media.requested_heights
│   └── signaling-handler.md     # RequestedVideoHeight(), SIM group in the answer
├── checklists/requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
scripts/gen-media.sh                 # + 90, 180 and 720 line layers
src/
├── media/
│   ├── assets/                      # + video_90.ivf, video_180.ivf, video_720.ivf
│   ├── content.go                   # layer contents (aligned frames)
│   ├── source.go                    # send-time ring per kind; lockstep layers
│   ├── vp8.go                       # NEW: payload descriptor, keyframe size
│   ├── video_quality.go             # NEW: frames, freezes, height, switches, keyframe need
│   ├── audio_quality.go             # NEW: playout deadline, concealment
│   ├── emodel.go                    # NEW: R and MOS
│   ├── latency.go                   # NEW: send-time registry, per-bot buckets
│   ├── metrics.go                   # new series
│   └── *_test.go, quality_bench_test.go
├── execution/
│   ├── peer.go                      # trackers in onTrack, PLI, PLI/FIR counting, simulcast encodings, SIM group, RequestedVideoHeight
│   ├── media_collector.go           # fold trackers, per-bot records, summary
│   ├── engine.go                    # summary at the end, layered source, requested heights
│   └── *_test.go                    # peer pairs: freeze, latency (vnet), PLI, simulcast
├── scenario/
│   ├── scenario.go                  # MediaConfig.Simulcast, RequestedHeights; QualitySummary type
│   └── service.go                   # store the summary with the scenario
├── api/handlers/scenario_handler.go # GetQuality
├── main.go                          # route /api/v1/scenarios/{id}/quality
└── platform/
    ├── adapter.go                   # RequestedVideoHeight()
    └── jitsi/bridge_channel.go      # maxHeight from the requested height
tests/live/jitsi_live_test.go        # quality checks and summary report
```

**Structure Decision**: single project as in 002–005. The measures go into
the media package next to `InboundStats`; the execution layer wires them and
builds the summary; the scenario package owns the summary type so that the
service and the API do not import the engine.

## Implementation Phases (input for `/speckit-tasks`)

1. **Foundation**: VP8 descriptor parsing (R1); tracker interfaces;
   `QualitySummary` type and the `onComplete` change that carries it;
   Prometheus series declared.
2. **US1 receiver quality (MVP)**: video tracker (frame rate, freezes,
   height, keyframe need → PLI), audio tracker (concealment), E-model score
   with round-trip delay, PLI/FIR counting on senders; collector export and
   per-bot records; summary and API; freeze test (SC-002), score tests
   (SC-003), benchmark; live run (SC-001) and 100-bot run (SC-006).
3. **US2 latency**: send-time ring in the source, lookup per received frame,
   latency in the score; vnet test (SC-004); live check that the bridge keeps
   video timestamps (R5 risk, fallback if not); summary latency.
4. **US3 simulcast**: layer content (gen-media), lockstep layered source,
   RID encodings and SIM group (R6 risks first), test plan fields,
   `RequestedVideoHeight` and the Jitsi constraint; simulcast pair test; live
   run (SC-005) and 100-bot runs at 360 and 720 (SC-008).
5. **Docs**: README, CLAUDE.md, OpenAPI (002) and test plan contract (004)
   references, validation report.

## Complexity Tracking

No constitution violation to justify.
