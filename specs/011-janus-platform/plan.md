# Implementation Plan: Janus Platform

**Branch**: `011-janus-platform` (work continues on `rework`) | **Date**: 2026-10-22 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/011-janus-platform/spec.md`

## Summary

A new adapter, `src/platform/janus`, speaks the Janus JSON API over one
WebSocket per bot (subprotocol `janus-protocol`). It holds one Janus
session with two VideoRoom handles, a publisher and a subscriber (R1).

Janus negotiates SDP itself, so the 008 boundary is used unchanged (R2):
- the publisher PeerConnection is the publish session the bot offers;
- the subscriber PeerConnection is the session Janus offers, answered with
  `OnOffer` and `Renegotiate`, and its answers go back in `start`.

**The dummy publisher.** A Janus subscription cannot be empty. The bots
therefore create the room with VideoRoom's dummy publisher, and every bot
subscribes at join to its one idle audio placeholder. The subscriber
transport connects at join, whoever else is in the room, and the
established rule stays the same for every platform (R3). This is the
Janus counterpart of 010's data-channel section.

**Joining.** Every bot sends room `create`, sized for the test plan's bots
(the dummy takes no publisher place, T002). "Room exists" (427) counts as success, so concurrent joins
do not race (R4). Rooms use Opus and VP8 only (R5).

**The receive cap** is applied in the multistream subscription (R6):
- per kind, a FIFO waiting list;
- one `update` (`subscribe` + `unsubscribe`) per change;
- `autoupdate: false`, so each change gives one renegotiation.

**Sessions** are kept alive every 25 s (Janus's default timeout is 60 s).
Losses map to the shared lifecycle (R7). The quality measures are
unchanged (R8). `platform_config` takes the URL and a numeric room. It may
also take the server's API secret or token, the room PIN and the
room-creation admin key, all redacted (R9).

**Local setup.** Janus has no official image, so `tests/platforms/janus`
builds tag `v1.4.3` (R10). A second container serves the demos, so that a
room can be watched without adding the web server's CPU to the server's
load.

**Tests** (R11):
- golden messages recorded from the real server;
- unit tests;
- a fake Janus server with real Pion peers in the normal test run;
- a live test mirroring the mediasoup one.

The comparison repeats 010 T029 with Janus as a fourth server (R12).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code; toolchain go1.24)

**Primary Dependencies**:
- `pion/webrtc/v3`: two PeerConnections per sending bot, as on LiveKit and
  mediasoup;
- `nhooyr.io/websocket` (already used): the Janus WebSocket, with its
  subprotocol;
- the standard library's `encoding/json`.

No new module, and no Janus client library (none is maintained in Go).

**Storage**: N/A (in memory, as before)

**Testing**: `go test -race ./...`. Details in R11:
- unit tests for the JSON client, config, receive cap and golden messages;
- a fake Janus server on `httptest`, with Pion on both sides, that drives
  real peers through the adapter;
- catalog and service tests;
- a live test (`-tags live`, `JANUS_*` variables) against the local setup.

**Target Platform**: Linux load machine; Janus v1.4.3 built from source in Docker on the same machine (host network, loopback)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**:
- 50 bots `audio_video`, receive cap 10, 10 minutes, every bot connected
  for the whole hold, on alternated runs with Jitsi, LiveKit and
  mediasoup (SC-002, R12);
- no per-packet work added: JSON messages come at join, per stream change
  and every 25 s per bot.

**Constraints**:
- No Janus concept outside `src/platform/janus` (constitution III).
- The peer sees SDP only (constitution II). The 008 boundary is used
  unchanged, and the 010 consent checks apply as they are.
- One change outside the adapter, internal to the catalog:
  `catalog.Entry.NewAdapter` also receives the test plan, so that the
  Janus entry can size the room from `Participants` (R4). The other three
  entries ignore it.
- **Deviation, found in T002** (the user's decision, 2026-10-22): the
  media sources start their RTP timestamps at 2^30 instead of 0
  (`src/media/source.go`, research R8). It is platform-neutral, and
  without it latency would be wrong on Janus. (First 2^31, which made
  Jitsi's bridge rebase video timestamps; moved to 2^30 in T022.)
- Jitsi, LiveKit and mediasoup behaviour, test plans and reports are
  unchanged (FR-011).
- Secrets never appear in logs, API answers, metrics or reports (FR-002).
- Janus is unmodified; only its config is ours (FR-015).

**Scale/Scope**:
- one numeric room per scenario, up to a few hundred bots;
- one Janus server;
- audio and video without simulcast;
- no data channels, no `switch`, no reconnect or ICE restart, no string
  room IDs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: synthetic bots against Janus; no browser
  and no `janus.js`. The demos page is only for manual checks.
- [x] **II. Media Engine Independence**: Janus's offers and answers go to
  the peer as SDP. Nothing is added to `src/execution` or `src/media`.
- [x] **III. Pluggable Signaling**: sessions, handles, transactions,
  VideoRoom requests and the dummy publisher live in the adapter
  ([contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)).
- [x] **IV. CPU Efficiency**:
  - no per-packet work;
  - the cap is applied by Janus (streams not subscribed are not
    forwarded);
  - the dummy section carries no RTP;
  - each bot sends one keepalive every 25 s.
- [x] **V. Deterministic Peer Lifecycle**: the same states and established
  rule. The dummy subscription (R3) avoids a per-platform exception. Join
  failures and losses are mapped in R7 and data-model.md.
- [x] **Observability**: same metrics, logs and report files. Secrets are
  redacted, and Janus errors are logged with request, code and reason.
- [x] **Test Discipline**: contract tests against a fake server with real
  Pion peers, golden messages from the real server, and live validation.
- [x] **Incremental Complexity**:
  - four optional settings, each one of the server's own authentication
    points (R9);
  - no `switch`, no string IDs, no HTTP transport, no Admin API.
- [x] **Scenario Definition**: `platform: janus` and its `platform_config`
  ([contracts/janus-config.yaml](contracts/janus-config.yaml)). Existing
  plans are untouched.

Post-design re-check: still passing; Complexity Tracking empty.

## Project Structure

### Documentation (this feature)

```text
specs/011-janus-platform/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── janus-config.yaml           # platform_config for platform janus, creation-time rules
│   ├── signaling-messages.md       # Janus core and VideoRoom messages sent and handled
│   └── platform-boundary-use.md    # how the adapter drives the 008 boundary (no change)
├── checklists/requirements.md
└── tasks.md                        # /speckit-tasks
```

### Source Code (repository root)

```text
src/
├── platform/
│   ├── catalog/catalog.go     # + janusEntry (Validate, SecretKeys, NewAdapter with the plan's participants)
│   └── janus/                 # new adapter
│       ├── config.go          # platform_config parsing, validation, LogValue without secrets
│       ├── client.go          # Janus JSON client: transactions, ack/event, sender routing, errors, keepalive
│       ├── messages.go        # Go types of the core and VideoRoom messages used
│       ├── streams.go         # receive cap: active per kind, FIFO waiting list, update lists
│       ├── adapter.go         # Connect (room create, join, publish, subscribe), Disconnect, per-bot goroutine, loss
│       └── *_test.go, testdata/golden/  # recorded messages, fake Janus server with Pion
└── (no change in execution, media, scenario, report)

tests/live/janus_live_test.go    # JANUS_* variables, logs the summary, writes the report
tests/platforms/janus/           # Dockerfile, conf/*.jcfg, docker-compose.yml, env.example, README.md
```

**Structure Decision**: the adapter is its own package, like
`src/platform/mediasoup`, and joins the catalog with one entry. The engine
(publish session, consent checks), scenario validation and redaction,
report files and server monitoring are reused as they are.

## Complexity Tracking

None.
