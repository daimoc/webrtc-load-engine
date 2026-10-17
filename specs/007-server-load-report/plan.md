# Implementation Plan: Server Load in the Run Report

**Branch**: `007-server-load-report` (work continues on `rework`) | **Date**: 2026-10-16 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/007-server-load-report/spec.md`

## Summary

A new platform-independent package, `src/hostload`, samples two things at
the same times: the named containers, through the Docker Engine API on the
local Unix socket with `net/http` (one one-shot stats request per container
per interval, R1), and the engine's own process, from `getrusage` and
`/proc/self/statm` (R2). CPU is the counter delta divided by the wall-clock
delta, in threads (R3); memory is the figure `docker stats` shows (usage
minus inactive file cache, R4). The engine runs one sampler goroutine per
scenario next to the media collector (R5); it exports gauges per scenario
and container (R6) and, at the end of the run, folds the samples into a
`server` and a `load_machine` section of the existing 006 quality summary,
served by the same endpoint (R7). Containers and interval come from a new
optional, platform-independent `server_monitoring` section of the test plan
(R8). Docker failures never fail the run: they are reported per container
with a reason (R9).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: standard library only (`net/http` over a Unix socket, `encoding/json`, `syscall.Getrusage`), `prometheus/client_golang`. No Docker SDK (R1). No new module.

**Storage**: N/A (in memory; the summary lives with the scenario, as 006)

**Testing**: `go test -race ./...`: unit tests for the CPU and memory arithmetic on recorded Docker stats bodies (cgroup v1 and v2) and for the summary folding; a fake Docker API served by `httptest` on a Unix socket in a temporary directory (missing container, stopped container, slow answers, restart); engine test with the fake and the mock adapter; scenario validation tests; live test `-tags live` on the local docker-jitsi-meet for SC-001/SC-002/SC-004

**Target Platform**: Linux (the engine's own figures read `/proc`; the server figures need a local Docker Engine API ≥ 1.41 for one-shot stats, local: 29.8 / API 1.56)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: ≤ 0.05 core for three containers at the default 5 s interval in the 100-bot run (SC-004); no per-bot work (FR-013)

**Constraints**: the run is never failed or delayed by monitoring (FR-010): requests have a timeout below the interval and run outside the engine's start/stop path; metrics labelled by scenario and container only; no platform concept in `src/hostload` (constitution III)

**Scale/Scope**: a handful of containers per scenario (up to 16, validated); runs of minutes to hours (samples folded as they come, not kept)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: measures the media server's load under synthetic peers; no browser.
- [x] **II. Media Engine Independence**: `src/hostload` knows containers and processes, nothing of media or signaling; the media layer is untouched.
- [x] **III. Pluggable Signaling**: containers are named in a platform-independent part of the test plan, not in `platform_config`; adapters are untouched, so 008/009 get it for free.
- [x] **IV. CPU Efficiency**: a few HTTP requests per interval, independent of the number of bots; bounded by SC-004.
- [x] **V. Deterministic Peer Lifecycle**: unchanged; monitoring never drives a transition or the scenario status.
- [x] **Observability**: new Prometheus gauges ([contracts/metrics.md](contracts/metrics.md)); one log line per unavailable container per run; summary through the existing API.
- [x] **Test Discipline**: unit tests for the arithmetic, a fake Docker API for every failure case of the spec, engine test, live validation.
- [x] **Incremental Complexity**: no Docker SDK, no event stream, no per-sample history in the summary; averages, peaks and first sample only.
- [x] **Scenario Definition**: the new `server_monitoring` section is optional; existing test plans are unchanged ([contracts/test-plan-server.yaml](contracts/test-plan-server.yaml)).

Post-design re-check: still passing; no entry in Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/007-server-load-report/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── test-plan-server.yaml      # server_monitoring section of the test plan
│   ├── server-load-summary.yaml   # server and load_machine sections of the quality summary
│   └── metrics.md                 # new Prometheus series
├── checklists/requirements.md
└── tasks.md                       # /speckit-tasks
```

### Source Code (repository root)

```text
src/
├── hostload/                  # new, platform-independent
│   ├── docker.go              # Engine API client over the Unix socket: one-shot stats, CPU/memory from a body
│   ├── process.go             # the engine's own CPU (getrusage) and RSS (/proc/self/statm), machine threads
│   ├── sampler.go             # samples containers + process at each tick, folds into per-target accumulators
│   ├── summary.go             # accumulators -> scenario.ServerLoad / scenario.LoadMachine
│   ├── metrics.go             # Prometheus gauges (contracts/metrics.md)
│   ├── overlap.go             # process-wide registry: which scenarios watch which container (FR-012)
│   └── *_test.go              # arithmetic, fake Docker API, sampler, summary
├── scenario/
│   ├── scenario.go            # TestPlan.ServerMonitoring + validation
│   └── quality.go             # QualitySummary gains Server and LoadMachine (types in the summary file)
└── execution/
    └── engine.go              # runs the sampler from Start to Stop; sets the summary's sections

tests/live/jitsi_live_test.go  # JITSI_SERVER_CONTAINERS, JITSI_SERVER_INTERVAL; logs the new sections
cmd/jitsi-connect/main.go      # --server-containers, --server-interval
```

**Structure Decision**: a new package rather than more code in `src/execution`, because the sampler knows nothing of peers or media and is the part 008/009 reuse unchanged. The summary types stay in `src/scenario` with the 006 summary they extend.

## Complexity Tracking

None.
