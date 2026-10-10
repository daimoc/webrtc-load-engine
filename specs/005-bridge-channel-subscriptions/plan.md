# Implementation Plan: Bridge Channel Subscriptions

**Branch**: `005-bridge-channel-subscriptions` (work continues on `rework`) | **Date**: 2026-10-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/005-bridge-channel-subscriptions/spec.md`

## Summary

Bots send Jitsi Colibri messages on the bridge channel they already open
(003 T056): `ReceiverVideoConstraints` listing, by source name, the remote
video sources they receive at up to 360 lines, so the bridge forwards video
to them (US1); and, with a receive cap, `ReceiverAudioSubscription` in
`Include` mode with the received audio sources, so the bridge stops sending
the streams the bots ignore (US2). Both are derived from the adapter's
existing session source state (004 FR-015) and resent when it changes or the
channel reopens. The local bridge (jitsi-videobridge2 2.3-326) supports both
(research R1, R2). Since the bridge never acknowledges them, the bot-side cap
stays on, and a new transport byte counter shows whether the server honors
them (R5, spec FR-007 amended). The data channel contract gains send and
event methods; message formats stay in the Jitsi adapter.

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: `pion/webrtc/v3` v3.3.6 (`DataChannel`, `SettingEngine.BufferFactory` read buffers), `encoding/json`, `prometheus/client_golang`; existing `mellium.im/xmpp` adapter. No new module.

**Storage**: N/A (in-memory per-session state)

**Testing**: `go test -race ./...`: in-process Pion peer pair for the data channel contract; the fake XMPP server of 003 with a fake `DataChannel` for message content and timing; live test `-tags live` against the local docker-jitsi-meet

**Target Platform**: Linux, single static binary

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: 100 bots, audio and video, cap 10, ≤ 2 CPU cores on average (SC-002); preferences sent within 2 s of a change (FR-005), replacement received within 5 s (SC-004)

**Constraints**: message formats only in `src/platform/jitsi` (FR-010, constitution III); no new per-bot goroutine (one stopped timer per session at most); bot-side cap kept (FR-007); metrics labelled by `scenario_id` only

**Scale/Scope**: one data channel per bot; at most two messages per source change per bot, under 1 kB each

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: the bots ask the SFU for what a real client would ask; this makes the SFU's video forwarding part of the load.
- [x] **II. Media Engine Independence**: `Peer` exposes a generic data channel (label, protocol, text messages); it never parses them. The media package is unchanged.
- [x] **III. Pluggable Signaling**: Colibri messages, source names and the bridge channel's meaning stay in the Jitsi adapter; the `SignalingHandler` change is platform-neutral ([contracts/signaling-handler.md](contracts/signaling-handler.md)).
- [x] **IV. CPU Efficiency**: the purpose is to cut the bots' CPU (SC-002). Messages are sent only on changes; no polling; one atomic add per received packet.
- [x] **V. Deterministic Peer Lifecycle**: unchanged; the bridge channel never drives a transition (FR-007, 004 FR-014).
- [x] **Observability**: two new Prometheus series per scenario ([contracts/metrics.md](contracts/metrics.md)); fallbacks logged once per bot with `scenario_id`/`peer_id`.
- [x] **Test Discipline**: contract tests for the data channel change and for the messages on the fake server; live validation on the local deployment.
- [x] **Incremental Complexity**: no new package; the bridge channel state is one small type; the transport counter is the only way to observe a non-acknowledged subscription (R5).

Post-design re-check: still passing; no entry in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/005-bridge-channel-subscriptions/
├── plan.md              # This file
├── research.md          # Phase 0: decisions R1–R8
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1: validation guide
├── contracts/
│   ├── colibri-messages.md     # messages sent and received on the bridge channel
│   ├── signaling-handler.md    # OpenDataChannel change
│   └── metrics.md              # webrtc_data_channels_open, webrtc_transport_bytes_received_total
├── checklists/requirements.md
└── tasks.md             # Phase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
src/
├── platform/adapter.go          # DataChannel, DataChannelHandler; OpenDataChannel signature
├── execution/
│   ├── peer.go                  # dataChannel wrapper, gauge accounting, TransportBytesReceived
│   ├── metrics.go               # webrtc_data_channels_open, webrtc_transport_bytes_received_total
│   ├── media_collector.go       # transport byte deltas
│   └── *_test.go                # peer pair data channel contract test, collector test
└── platform/jitsi/
    ├── bridge_channel.go        # NEW: bridgeChannel state, message building, send rules, fallback logs
    ├── sources.go               # receivedNames(content) from active sections
    ├── jingle.go                # open with handler, update preferences after each change, stop on terminate
    ├── signaling.go             # occupant departure → preferences update (via removeOwnerSources)
    └── *_test.go                # bridge_channel_test.go; adapter tests with a fake DataChannel

tests/live/jitsi_live_test.go    # data channel gauge, video check, transport ratio report
```

**Structure Decision**: single project as in 002–004. The Colibri code goes
into one new file of the Jitsi adapter; the execution layer only gets the
generic data channel and the two metrics.

## Implementation Phases (input for `/speckit-tasks`)

1. **Data channel contract** (foundation): `platform.DataChannel` and
   `DataChannelHandler`, new `OpenDataChannel` signature, `Peer`
   implementation with the open gauge; update the callers and test fakes
   (adapter `pionHandler`, engine test adapters). Contract test with a Pion
   peer pair.
2. **Video constraints** (US1, MVP): `bridgeChannel` in the adapter with the
   no-cap and capped `ReceiverVideoConstraints`; sent on open and after each
   change of the active sections; incoming messages ignored with a debug
   log; fallback logs (R8). Fake-server tests (message content, resend on
   reopen, no duplicate, no SCTP → no message). Validation: quickstart
   step 1.
3. **Audio subscription** (US2): `ReceiverAudioSubscription` `Include` with a
   cap; unnamed-source fallback. Tests for promotion of a waiting source
   (message within the same renegotiation) and occupant departure.
4. **Transport metric** (US2, SC-003): `TransportBytesReceived` on `Peer`,
   collector delta, live test report of the ratio. Validation: quickstart
   step 2.
5. **Scale and docs**: quickstart step 3 (100 bots), results in tasks.md
   next to 004's; README and CLAUDE.md; update 004 spec's assumption and
   edge case "Receive cap reached" (the server no longer sends ignored
   streams when the bridge channel is open).

## Complexity Tracking

No constitution violation to justify.
