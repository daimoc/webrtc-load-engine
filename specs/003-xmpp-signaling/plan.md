# Implementation Plan: XMPP Signaling Logic

**Branch**: `003-xmpp-signaling` | **Date**: 2026-01-07 | **Spec**: [specs/003-xmpp-signaling/spec.md](spec.md)
**Input**: Feature specification from `specs/003-xmpp-signaling/spec.md`

## Summary

Implement the Jitsi platform adapter using `gosrc.io/xmpp` to handle XMPP signaling. The adapter will support anonymous SASL authentication, join MUC rooms, and negotiate Jingle (XEP-0166) sessions by acting as a responder to Jicofo's `session-initiate` requests. It will translate Jingle XML to standard SDP for the core media engine.

## Technical Context

**Language/Version**: Go 1.23.0
**Primary Dependencies**: `gosrc.io/xmpp` (Fluux), `pion/webrtc/v3`
**Storage**: N/A (In-memory session state)
**Testing**: Unit tests with mock XMPP server/stanzas, Integration against live Jitsi
**Target Platform**: Jitsi Meet (XMPP WebSocket)
**Project Type**: single (CLI/Engine)
**Performance Goals**: Support 100s of concurrent XMPP sessions per node
**Constraints**: Must adhere to Jitsi's specific Jingle dialect (Colibri/ICE-UDP)
**Scale/Scope**: Signaling layer only

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: Focuses on backend signaling, no browser emulation.
- [x] **II. Media Engine Independence**: Jingle logic is confined to the adapter; core engine receives only SDP.
- [x] **III. Pluggable Signaling**: Implemented as a specific `PlatformAdapter` ("jitsi").
- [x] **IV. CPU Efficiency**: Using efficient XML parsing; avoiding heavy runtime reflection where possible.
- [x] **V. Deterministic Peer Lifecycle**: Jingle states (Pending/Active/Ended) map cleanly to Engine Peer lifecycle.

## Project Structure

### Documentation (this feature)

```text
specs/003-xmpp-signaling/
├── plan.md              # This file
├── research.md          # Protocol details and library choice
├── data-model.md        # Jingle state machine and XML structs
├── quickstart.md        # Configuration guide
├── contracts/           # Configuration schema
│   └── configuration.yaml
└── tasks.md             # To be generated
```

### Source Code (repository root)

```text
src/
├── platform/
│   └── jitsi/
│       ├── adapter.go          # Main adapter implementation (updates existing)
│       ├── signaling.go        # XMPP connection and MUC logic
│       ├── jingle.go           # Jingle state machine and SDP translation
│       └── xmpp_structs.go     # XML mapping for Jingle/ICE-UDP
```

**Structure Decision**: Option 1 (Single project). Expanding `src/platform/jitsi` with specific implementation files.

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | | |