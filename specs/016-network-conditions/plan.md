# Implementation Plan: Network Conditions per Bot

**Branch**: `016-network-conditions` (work continues on `rework`) | **Date**: 2026-11-15 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/016-network-conditions/spec.md`

## Summary

A test plan's `network` section puts groups of bots under conditions in each direction: a rate cap with a queue, delay with jitter, and random or burst loss, following a schedule of steps. The conditions are applied in the engine, on each constrained bot's own UDP sockets, through the network wrapper every session already uses (R1). The report (FormatVersion 4) gives, per bot and step:
- what the condition measurably did;
- the reaction and recovery times, from a per-stream height timeline;
- the server's estimate where the adapter can read it (mediasoup).

It also gives one quality summary per group. Constrained senders adapt as a browser does: Pion's send-side estimator, fed by the servers' transport-wide feedback, drives a layer policy that drops and retries whole simulcast or SVC layers (R7). Bots without a condition run exactly as today.

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**:
- `pion/webrtc/v3` v3.3.6, plus `pion/interceptor` v0.1.29: `pkg/gcc` (`SendSideBWE` with a no-op pacer) and `pkg/twcc` (header extension sender interceptor), for constrained senders only.
- `pion/transport/v2` (the `transport.Net`/`UDPConn` wrapper of 010).
- Standard library `math/rand/v2` (PCG) and `time`.
- No new module.

**Storage**: report files (009), FormatVersion 4

**Testing**:
- `go test -race ./...`;
- shaper tests between two local UDP sockets (SC-001, SC-005);
- adapter fake servers (mediasoup estimate);
- live tests with `<PLATFORM>_NETWORK`.

**Target Platform**: Linux load machine; servers local or remote (013)

**Project Type**: Go library, HTTP API and CLIs (existing)

**Performance Goals**: engine CPU per bot at 100 bots without conditions within 5% of before (SC-006); the cost with conditions is measured and stated

**Constraints**:
- no administrator rights and no change on the load machine or the servers;
- conditions per bot;
- the same behaviour on the four platforms.

**Scale/Scope**: up to 100 bots per run, up to 16 groups and 32 steps per group; validation at 10 bots on the four local servers

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Check |
|-----------|-------|
| I. Infrastructure first | Pass. Conditions stress how servers handle real links; no browser behaviour is emulated beyond sender adaptation, which synthetic senders need to be realistic. |
| II. Media engine independence | Pass. The shaper sits in the execution layer's network wrapper; the layer policy reads only RTCP feedback and content bitrates; no signaling concept enters it. Loss and jitter draws are seeded, so they are reproducible (R4). |
| III. Pluggable signaling | Pass. The server's estimate comes through an optional adapter interface (`platform.ReceiverEstimator`, R9); the engine knows nothing of `getTransportStats`. Test plan conditions have no platform field (FR-014). |
| IV. CPU efficiency | Pass, with justification. Bots without a condition pay one nil check per datagram. Constrained bots pay a goroutine and timer per direction. Constrained senders pay Pion's estimator. Measured (R11, SC-006). |
| V. Deterministic lifecycle | Pass. No new peer states; conditions are applied to sockets whatever the state. |
| Scenario stable and versioned | Pass. `network` is optional and additive; the report FormatVersion goes 3 → 4. |
| Observability | Pass. Structured logs on each step change and layer change; per-bot measures in both report files. |

Post-design re-check: unchanged, all pass.

## Project Structure

### Documentation (this feature)

```text
specs/016-network-conditions/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── test-plan-network.md
│   └── report-network.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
src/scenario/
├── network.go            # NetworkConfig, validation, group assignment (new)
├── network_test.go
├── scenario.go           # TestPlan.Network
└── result.go             # RunResult.Network, NetworkResult types
src/execution/
├── netcond.go            # shaper, linkShaper, token bucket, queue, Gilbert-Elliott (new)
├── netcond_test.go       # SC-001/SC-005 between local sockets
├── consent.go            # consentConn reads/writes go through the bot's shaper
├── schedule.go           # step timers per group, settling flag (new)
├── sender_adapt.go       # SendSideBWE + layer policy (new)
├── sender_adapt_test.go
├── video_track.go        # layer enable / max spatial id, own sequence numbers
├── pion.go               # twcc sender interceptor + estimator for constrained senders
├── quality_collector.go  # per-stream height/freeze timeline
├── adaptation.go         # reaction/recovery times, per-group summaries (new)
└── engine.go             # shapers per bot, schedule, estimate sampling, result
src/platform/
├── adapter.go            # optional ReceiverEstimator interface
└── mediasoup/            # ReceiverEstimate via getTransportStats
src/report/
├── run.go                # FormatVersion 4, Network
└── markdown_016.go       # "Network conditions" section (new)
cmd/jitsi-connect/        # --network <file>
tests/live/               # <PLATFORM>_NETWORK helper
```

**Structure Decision**: The existing single Go module. New files next to the code they extend; no new package. The shaper is internal to `src/execution`, because it wraps that package's sockets.

## Phase order and gates

1. **Gate (first task)**: for each server, check whether it sends transport-wide feedback to a publisher whose packets carry the extension (R7). Its result decides whether US4 is validated on that server or recorded as `no_feedback`.
2. Shaper and test plan (US1 core), then report, then schedule (US2), loss and delay (US3, same shaper), sender adaptation (US4), estimate and comparison (US5).
3. CPU A/B before closing (SC-006).

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Send-side estimator and layer policy in the bots (US4) | The user chose adapting senders: a capped sender that keeps sending every layer only loses packets, which no browser does | Not adapting (rejected by the user); padding probes (servers handle padding-only packets differently, R7) |
