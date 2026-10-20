# Implementation Plan: mediasoup-demo Platform

**Branch**: `010-mediasoup-platform` (work continues on `rework`) | **Date**: 2026-10-20 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/010-mediasoup-platform/spec.md`

## Summary

A new adapter, `src/platform/mediasoup`, speaks the mediasoup-demo web
client's protocol: protoo JSON requests, responses and notifications over
one WebSocket per bot, with the `protoo` subprotocol (R1). mediasoup has no
SDP. The adapter builds SDP from the server's ICE, DTLS and RTP parameters,
and turns the bot's publish offer back into producer parameters, as
`mediasoup-client` does (R2). The peer therefore keeps the 008 boundary
unchanged:

- the sending transport is the publish session the peer offers;
- the receiving transport is the session the remote side offers, answered
  with `OnOffer` and `Renegotiate`.

The receiving session opens at join with a data-channel-only offer, so
every bot reaches `established` whoever else is in the room (R3). The join
follows the demo client's order (R4), with capabilities derived from the
router's, limited to Opus and VP8 (R5).

The receive cap uses the demo's paused consumers. The bot accepts
`newConsumer` up to the cap, declines the rest (they stay paused on the
server), and on `consumerClosed` resumes the earliest waiting one with
`resumeConsumer` (R6). Loss and leave map onto the shared lifecycle (R7).
The quality measures are unchanged (R8).

The `platform_config` adds two settings of the demo. `unlimited_room_secret`
(redacted) works around the demo's 10-minute room limit, and
`use_pipe_transports` spreads a room over two workers (R9, R10). A
`mediasoup` catalog entry validates plans at creation and rejects simulcast
and requested heights.

A pinned local setup in `tests/platforms/mediasoup` builds the demo server
from a fixed commit, since no official image exists (R11). A live test
mirrors the LiveKit one (R12). CPU is compared with LiveKit on alternated
runs (R13).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code; toolchain go1.24)

**Primary Dependencies**:
- `pion/webrtc/v3` (two PeerConnections per sending bot, as on LiveKit)
- `pion/sdp/v3` (already a direct requirement): builds and parses the SDP
  the adapter translates
- `nhooyr.io/websocket` (already used): protoo WebSocket with subprotocol
- standard library `encoding/json`

No new module, and no mediasoup or protoo client library (none exists in Go).

**Storage**: N/A (in memory, as before)

**Testing**: `go test -race ./...`. Details in R12:
- unit tests for protoo framing, config, SDP translation (golden SDPs and
  parameters) and receive cap transitions
- a fake protoo and mediasoup server on `httptest`, with a Pion ICE-lite
  answerer that forwards RTP, driving real peers through the adapter
- catalog and service tests
- a live test (`-tags live`) against the local setup for SC-001 to SC-003

**Target Platform**: Linux load machine; mediasoup-demo server built from commit `899109e` (mediasoup 3.30.x, Node 22) in Docker on the same machine (host network, loopback)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: 100 bots `audio_video`, receive cap 10, 10 minutes, engine CPU within run-to-run noise of LiveKit's for the same plan on alternated runs (SC-002, R13). No per-packet work is added: SDP work is per stream change, and protoo messages are per join and per stream change.

**Constraints**:
- No mediasoup or protoo concept outside `src/platform/mediasoup`
  (constitution III).
- The peer sees SDP only (constitution II), and the 008 boundary is used
  unchanged.
- **Deviation, found in T016**: `src/execution` gains two platform-neutral
  changes, neither of them a mediasoup concept:
  - RFC 7675 consent checks (research R14);
  - no publish session for a receive-only peer (an 008 bug that kept it
    from being established).
- Jitsi and LiveKit behaviour, test plans and reports are unchanged
  (FR-009).
- The unlimited-room secret never appears in logs, API answers, metrics or
  reports (FR-002).
- The demo server is unmodified; only its config is ours (FR-014).

**Scale/Scope**:
- one room per scenario, up to a few hundred bots;
- one mediasoup-demo server (2 workers locally);
- audio and video without simulcast;
- no data channels, no reconnect or ICE restart, no TCP candidates.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: synthetic bots against mediasoup; no
  browser and no `mediasoup-client`.
- [x] **II. Media Engine Independence**: the adapter translates mediasoup's
  JSON parameters to and from SDP, and the peer only sees offers, answers
  and lifecycle events. Nothing is added to `src/execution` or `src/media`.
- [x] **III. Pluggable Signaling**: protoo, transports, producers and
  consumers live in the adapter. It reuses the 008 publish session; no
  boundary change ([contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)).
- [x] **IV. CPU Efficiency**: no per-packet work. The cap is enforced by the
  server (paused consumers), not by dropping packets. One SCTP association
  per bot carries heartbeats only (R3).
- [x] **V. Deterministic Peer Lifecycle**: the same states and the same
  established rule for every platform. The receiving session opens at join
  (R3) instead of a per-platform exception. Join, loss and leave are mapped
  in R7.
- [x] **Observability**: same metrics, logs and report files; the secret is
  redacted; protoo request failures are logged with method and reason.
- [x] **Test Discipline**: contract tests against a fake server with real
  Pion peers, golden SDP translation tests, and live validation.
- [x] **Incremental Complexity**:
  - two optional settings, both existing demo parameters (R9);
  - inactive sections are not reused (R6);
  - no TCP candidates, no simulcast, no data channels.
- [x] **Scenario Definition**: `platform: mediasoup` and its
  `platform_config` ([contracts/mediasoup-config.yaml](contracts/mediasoup-config.yaml));
  existing plans are untouched.

Post-design re-check: still passing; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/010-mediasoup-platform/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── mediasoup-config.yaml       # platform_config for platform mediasoup, creation-time rules
│   ├── signaling-messages.md       # protoo messages and fields the adapter sends and handles
│   ├── sdp-mapping.md              # mediasoup parameters <-> SDP, both directions
│   └── platform-boundary-use.md    # how the adapter drives the 008 boundary (no change)
├── checklists/requirements.md
└── tasks.md                        # /speckit-tasks
```

### Source Code (repository root)

```text
src/
├── platform/
│   ├── catalog/catalog.go     # + mediasoupEntry (Validate, SecretKeys, NewAdapter)
│   └── mediasoup/             # new adapter
│       ├── config.go          # platform_config parsing, validation, LogValue without secret
│       ├── protoo.go          # protoo client: framing, request IDs, server requests, notifications
│       ├── messages.go        # Go types of the mediasoup JSON (RtpCapabilities, RtpParameters, transport params)
│       ├── sdp.go             # receive offer from transport + consumers; publish offer -> rtpParameters; publish answer
│       ├── consumers.go       # receive cap: active per kind, FIFO waiting list, sections
│       ├── adapter.go         # Connect (join sequence), Disconnect, per-bot SDP goroutine, loss
│       └── *_test.go, testdata/  # golden SDPs and JSON, fake protoo server with a Pion ICE-lite forwarder
├── execution/consent.go         # RFC 7675 consent checks on every PeerConnection (research R14)
├── execution/peer.go            # publish session only for a peer that sends
└── (no change in media, scenario, report)

tests/live/mediasoup_live_test.go    # MEDIASOUP_* variables, logs the summary, writes the report
tests/platforms/mediasoup/           # Dockerfile, config.mjs, docker-compose.yml, env.example, README.md
```

**Structure Decision**: the adapter is its own package, like
`src/platform/livekit`. It joins the catalog with one entry. The engine
(publish session), scenario validation and redaction, report files and
server monitoring are reused as they are.

## Complexity Tracking

None.
