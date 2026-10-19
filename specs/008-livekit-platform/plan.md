# Implementation Plan: LiveKit Platform

**Branch**: `008-livekit-platform` (work continues on `rework`) | **Date**: 2026-10-17 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/008-livekit-platform/spec.md`

## Summary

A new adapter, `src/platform/livekit`, speaks LiveKit's client signaling
protocol: binary protobuf frames over one WebSocket per bot to the server's
`/rtc` endpoint (R1), encoded with a small hand-written codec over
`protowire` for the dozen messages used, so no LiveKit module (it needs Go
1.26 and Pion v4) enters the build (R2). Each bot presents an HS256 access
token the adapter signs from the key and secret, with a signer shared with
the Jitsi JWT code (R3).

LiveKit uses two media sessions per participant: the client offers the
publish session, and the server offers the subscribe session. The Jitsi
boundary only knows server offers. The platform boundary therefore gains an
optional publish session that the peer offers. `execution.Peer` opens a
second Pion PeerConnection for it when the adapter asks (R4). The adapter
publishes the bot's tracks (R5), answers the server's subscribe offers with
the existing `OnOffer`/`Renegotiate` (R6), and subscribes to at most the
receive cap per kind itself, because auto-subscribe is off (R7). It keeps
the connection alive with pings (R8) and maps joins, leaves and losses onto
the shared lifecycle (R9).

Test plans are validated per platform at creation. Simulcast and requested
heights are rejected for LiveKit, and secrets are redacted in API answers
(R10). The quality and server-load summaries are unchanged except for an
optional `latency_reason`, so a measure LiveKit prevents is explained rather
than zero (R11). A pinned LiveKit server setup is added in
`tests/platforms/livekit` (R12), and a live test mirrors the Jitsi one
(R13).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code; toolchain go1.24)

**Primary Dependencies**: `pion/webrtc/v3` (two PeerConnections per LiveKit bot), `nhooyr.io/websocket` (already used by 003), `google.golang.org/protobuf/encoding/protowire` (module already in go.sum through Prometheus; becomes a direct requirement), standard library (`crypto/hmac` HS256). No LiveKit SDK or `livekit/protocol` module (R2).

**Storage**: N/A (in memory, as before)

**Testing**: `go test -race ./...`:
- codec tests on golden frames recorded from the official `livekit/protocol` Go types by a throwaway generator kept outside the module (R2);
- a fake LiveKit signaling server on `httptest` (join, publish, offers, subscriptions, pings, leave, wrong token) that drives real Pion peers through the adapter;
- engine tests with the publish session;
- scenario and catalog validation tests;
- a live test `-tags live` against the local LiveKit setup for SC-001 to SC-003.

**Target Platform**: Linux load machine; LiveKit server `livekit/livekit-server:v1.13.10` in Docker on the same machine (host network)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: 100 bots `audio_video`, receive cap 10, within the 2-core engine budget of 005/006 (SC-002); the second PeerConnection per sending bot must not add per-packet work (same tracks, same writers)

**Constraints**:
- No LiveKit concept outside `src/platform/livekit`. The new boundary methods are platform-neutral: a publish session the peer offers (constitution III).
- The media layer stays signaling-free (constitution II).
- Jitsi behaviour and test plans are unchanged.
- Secrets never appear in logs, API answers or metrics (FR-009).

**Scale/Scope**: one room per scenario, up to a few hundred bots; one LiveKit node; dual-PC signaling only (no single-PC mode, no resume/reconnect)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: synthetic bots against a LiveKit SFU; no browser, no LiveKit client SDK.
- [x] **II. Media Engine Independence**: the peer only gains a second session built from the same tracks, plus offer/answer/candidate calls. No protobuf, token or room concept reaches `src/execution` or `src/media`.
- [x] **III. Pluggable Signaling**: the whole LiveKit protocol lives in its adapter. The boundary is widened with an optional, platform-neutral publish session (offer, answer, candidates), which 009's mediasoup-demo needs too ([contracts/platform-boundary.md](contracts/platform-boundary.md)).
- [x] **IV. CPU Efficiency**: no new per-packet work. The publish session carries the same pre-encoded tracks. Control messages are a few per second per bot. Bounded by SC-002.
- [x] **V. Deterministic Peer Lifecycle**: LiveKit join, leave and loss map to the existing states (R9). Established needs every open session's transport, and no state is added.
- [x] **Observability**: same metrics and logs as Jitsi; the token is redacted from logged URLs; the summary gains only the optional `latency_reason`.
- [x] **Test Discipline**: the adapter has contract tests against a fake server with real Pion peers, plus codec golden tests. Live validation covers SC-001 to SC-003.
- [x] **Incremental Complexity**: about 12 messages hand-encoded instead of a 60-module dependency. No simulcast, no single-PC mode, no reconnect.
- [x] **Scenario Definition**: `platform: livekit` and its `platform_config` ([contracts/livekit-config.yaml](contracts/livekit-config.yaml)); existing test plans are untouched.

Post-design re-check: still passing; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/008-livekit-platform/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── livekit-config.yaml        # platform_config for platform livekit, creation-time rules
│   ├── platform-boundary.md       # publish session added to SignalingHandler; optional adapter interface
│   ├── signaling-messages.md      # LiveKit messages and fields the adapter encodes/decodes
│   └── quality-summary-delta.yaml # latency_reason added to the 006 summary
├── checklists/requirements.md
└── tasks.md                       # /speckit-tasks
```

### Source Code (repository root)

```text
src/
├── platform/
│   ├── adapter.go             # SignalingHandler: publish session methods; PublishSessionAdapter
│   ├── catalog/               # new: platform name -> validate test plan, secret keys, new adapter
│   │   └── catalog.go
│   ├── internal/jwt/          # new: HS256 signer shared by jitsi and livekit
│   │   └── hs256.go
│   ├── jitsi/jwt.go           # uses internal/jwt (behaviour unchanged)
│   └── livekit/               # new adapter
│       ├── config.go          # platform_config parsing and validation
│       ├── token.go           # access token claims
│       ├── proto.go           # protowire codec for the messages used (contracts/signaling-messages.md)
│       ├── signal.go          # WebSocket connection, read loop, pings, leave
│       ├── adapter.go         # Connect/Disconnect, join, publish, subscribe offers
│       ├── subscriptions.go   # receive cap: tracks known, tracks subscribed, replacement
│       └── *_test.go, testdata/  # golden frames, fake server
├── execution/
│   ├── peer.go                # optional publish session (second PeerConnection)
│   ├── pion.go                # PeerConnection for the publish session (no receive gate)
│   └── engine.go              # PeerMedia.PublishSession from the adapter
├── scenario/
│   ├── service.go             # validates and redacts through the starter's catalog
│   └── quality.go             # KindQuality.LatencyReason
└── main.go                    # starter builds adapters from the catalog

tests/live/livekit_live_test.go      # LIVEKIT_* variables, logs the summary
tests/platforms/livekit/             # docker-compose.yml, livekit.yaml, env.example, README.md
```

**Structure Decision**: the adapter mirrors `src/platform/jitsi` as its own package. Platform selection moves out of `main.go`'s hard-coded Jitsi path into a small catalog package. Both the starter and creation-time validation use it, without `src/scenario` importing any adapter, which would be an import cycle.

## Complexity Tracking

None.
