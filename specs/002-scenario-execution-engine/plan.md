# Implementation Plan: Scenario Execution Engine

**Branch**: `002-scenario-execution-engine` | **Date**: 2026-01-06 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/home/agent/webrtc-load-engine/specs/002-scenario-execution-engine/spec.md`

## Summary

This plan outlines the implementation of the scenario execution engine. This engine will take a created scenario, and using the specified platform adapter, will generate the described WebRTC load. This involves managing the lifecycle of synthetic peers, respecting the test plan parameters (participants, ramp-up, duration), and updating the scenario status.

## Technical Context

**Language/Version**: Go 1.23.0
**Primary Dependencies**: Pion, net/http, `google/uuid`
**Storage**: In-memory
**Testing**: Go standard testing library
**Target Platform**: Linux server
**Project Type**: Single project
**Performance Goals**: Must be CPU and memory efficient, allowing for hundreds of concurrent peers on a single machine.
**Constraints**: Must build as a single static binary. Execution should be asynchronous.
**Scale/Scope**: Execute a single scenario at a time.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Infrastructure-First | ✅ Pass | The execution engine is the core of the infrastructure-level testing. |
| II. Media Engine Independence | ✅ Pass | The engine will interact with a media engine (via Pion) but will be decoupled from signaling logic. |
| III. Pluggable Signaling | ✅ Pass | The engine will use the specified platform adapter, adhering to the pluggable signaling model. |
| IV. CPU Efficiency | ✅ Pass | A primary goal of the execution engine. |
| V. Deterministic Peer Lifecycle | ✅ Pass | The engine will be responsible for managing a deterministic peer lifecycle. |
| **Constraint: Language** | ✅ Pass | Go is the specified language. |
| **Constraint: Scenario Definition** | ✅ Pass | The engine consumes the scenario definition. |

## Project Structure

### Documentation (this feature)

```text
specs/002-scenario-execution-engine/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)
```text
src/
├── execution/
│   ├── engine.go
│   └── peer.go
├── platform/
│   └── jitsi/
│       └── adapter.go
├── scenario/
│   ├── scenario.go
│   └── service.go
└── main.go

tests/
├── unit/
│   └── execution/
│       └── engine_test.go
└── integration/
    └── execution/
        └── scenario_execution_test.go
```

**Structure Decision**: This builds on the existing structure. A new `execution` package will contain the core logic for the execution engine and peer management. A `platform` package will be created to house the different platform adapters, starting with `jitsi`.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| *None* | | |
