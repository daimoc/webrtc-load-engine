# Implementation Plan: Synthetic Media Sending and Media Metrics

**Branch**: `004-synthetic-media` (work continues on `rework`) | **Date**: 2026-10-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/004-synthetic-media/spec.md`

## Summary

Bots send looped, pre-encoded audio (Opus) and video (VP8) according to a
per-scenario media profile, receive every stream the media server forwards,
and the engine exports per-scenario media metrics (volumes, loss, jitter,
round trip, stalls, send lateness). Media content is embedded in the binary
and paced by one shared goroutine per scenario and kind that writes the same
RTP packets to every bot's track. On the Jitsi side, the adapter announces
the bots' sources as lib-jitsi-meet does, expands Jicofo's per-kind contents
into one SDP section per remote source so Pion can receive them all, and
renegotiates locally on `source-add` / `source-remove`.

## Technical Context

**Language/Version**: Go 1.23.0 (toolchain go1.27 installed; keep 1.23-compatible code)

**Primary Dependencies**: `pion/webrtc/v3` v3.3.6 (`TrackLocalStaticRTP`, `pkg/media/ivfreader`, `pkg/media/oggreader`), `pion/rtp` (packetizer, VP8/Opus payloaders), `pion/rtcp` (receiver reports), `prometheus/client_golang`, existing `mellium.im/xmpp` adapter

**Storage**: N/A (media content embedded with `go:embed`; state in memory)

**Testing**: `go test -race ./...` (unit tests with in-process Pion peer pairs and the fake XMPP server of 003); live test `-tags live` against the local docker-jitsi-meet

**Target Platform**: Linux, single static binary

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: 100 bots sending audio and video on one machine; p99 send lateness < 20 ms; at most 2 CPU cores on average (SC-003, SC-004)

**Constraints**: no real-time encoding or decoding (constitution IV, FR-004); media metrics labelled by scenario and kind only (FR-013); media problems never fail a peer (FR-014)

**Scale/Scope**: up to a few hundred bots per process; up to N−1 received streams per kind per bot, bounded by the bridge's forwarding (research R10)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: synthetic peers send synthetic media to load the SFU; nothing is rendered.
- [x] **II. Media Engine Independence**: the `media` package and `Peer` handle only RTP, SDP and media kinds. Source names, presence `SourceInfo` and the per-source SDP expansion live in the Jitsi adapter. Sent media is deterministic: same content, same schedule, same payloads for every bot (R2).
- [x] **III. Pluggable Signaling**: the `SignalingHandler` contract gains two platform-neutral methods (`LocalMediaKinds`, `Renegotiate`); see [contracts/signaling-handler.md](contracts/signaling-handler.md).
- [x] **IV. CPU Efficiency**: pre-encoded content, one pacer per scenario and kind, one packetization per frame, inline receive accounting instead of per-SSRC stats goroutines. The CPU increase is the purpose of the feature (sending and receiving RTP) and is bounded by SC-004; the reception risk is documented in R10.
- [x] **V. Deterministic Peer Lifecycle**: unchanged state machine; media state is read only while `established` and never triggers a transition.
- [x] **Observability**: Prometheus metrics correlated by `scenario_id`; per-peer detail in structured logs ([contracts/metrics.md](contracts/metrics.md)).
- [x] **Test Discipline**: the signaling contract changes come with contract tests on the fake XMPP server; the pacer gets a benchmark (performance-sensitive path).

Post-design re-check: still passing; no entry in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/004-synthetic-media/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R11
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1: validation guide
├── contracts/
│   ├── test-plan-media.yaml    # TestPlan.media schema
│   ├── metrics.md              # media metrics
│   └── signaling-handler.md    # SignalingHandler additions
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
scripts/
└── gen-media.sh                 # NEW: builds the content with ffmpeg (Docker), run once

src/
├── media/                       # NEW package: platform-free media
│   ├── assets/video.ivf         # embedded VP8 content
│   ├── assets/audio.ogg         # embedded Opus content
│   ├── content.go               # go:embed, parse IVF/Ogg into frames
│   ├── source.go                # pacer: packetize once, fan out to tracks, lateness
│   ├── inbound.go               # InboundStats: packets, bytes, loss, jitter (RFC 3550)
│   ├── rtt.go                   # round trip from receiver reports (LSR/DLSR)
│   ├── metrics.go               # webrtc_media_* collectors
│   └── *_test.go, source_bench_test.go
├── scenario/scenario.go         # TestPlan.Media, validation, defaults
├── platform/adapter.go          # SignalingHandler: LocalMediaKinds, Renegotiate
├── execution/
│   ├── engine.go                # media sources per scenario, stats collector, stall detection
│   ├── peer.go                  # local tracks, OnTrack/RTCP read loops, Renegotiate
│   └── *_test.go
└── platform/jitsi/
    ├── jingle_sdp.go            # per-source section expansion and collapse
    ├── sources.go               # NEW: session source state, source-add/remove, local source names
    ├── jingle.go                # keep remote description; renegotiation; local source attributes
    ├── signaling.go             # dispatch source-add/remove; SourceInfo in join presence
    ├── xmpp_structs.go          # Source videoType, ssrc-info owner
    └── *_test.go                # fake server: sources, SourceInfo, source-add/remove

cmd/jitsi-connect/main.go        # --media, --participants
tests/live/jitsi_live_test.go    # JITSI_MEDIA; media metric checks
```

**Structure Decision**: single project as in 002/003. Media code goes into a
new `src/media` package so the media layer stays separate from both the
engine's lifecycle code and the adapters (constitution II).

## Implementation Phases (input for `/speckit-tasks`)

1. **Configuration**: `TestPlan.Media` with defaults and validation; OpenAPI
   update in 002's contract from [contracts/test-plan-media.yaml](contracts/test-plan-media.yaml).
2. **Media package** (US1 foundation): content generation and embedding,
   loading, the pacer with its lateness histogram and benchmark.
3. **Sending** (US1): `LocalMediaKinds`, local tracks in `Peer`, pacer
   subscription on `established`, unsubscription on leave; Jitsi source
   names, `ssrc-info`, `videoType` and presence `SourceInfo`. Validation:
   quickstart steps 2 and 3. This is the MVP.
4. **Receiving** (US2 foundation): per-source SDP expansion and collapse,
   session source state, `source-add` / `source-remove` and `Renegotiate`;
   `OnTrack` and RTCP read loops in `Peer`.
5. **Metrics** (US2): `InboundStats`, round trip, collector, stall detection,
   live test and CLI flags. Validation: quickstart step 4.
6. **Scale** (US3): the 100-bot measurement of quickstart step 5, results
   recorded in tasks.md as for 002 T016, R10 follow-up if needed.

## Complexity Tracking

No constitution violation to justify.
