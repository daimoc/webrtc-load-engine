---

description: "Task list for 007 server load in the run report"
---

# Tasks: Server Load in the Run Report

**Input**: Design documents from `specs/007-server-load-report/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires core components to be test-covered (plan "Constitution Check"): unit tests for the arithmetic, a fake Docker API for every failure case of the spec, an engine test. Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`); code stays go1.23-compatible; `go vet ./... && go test -race ./...` stays green after every task. Live runs target the local docker-jitsi-meet in `tests/platforms/docker-jitsi-meet` ([quickstart.md](quickstart.md)); restart the bridge (`docker compose restart jvb`) before scale runs. Tests that read global metrics use a unique scenario ID (uuid), as in 004–006. `src/hostload` imports nothing from `src/execution`, `src/media` or `src/platform` (constitution II/III). Monitoring never fails, delays or changes the status of a scenario (FR-010). No new module: no Docker SDK (research R1).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2)

---

## Phase 1: Setup

**Purpose**: none needed. No new module; `src/hostload/` is created by its first task.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the summary types, the test plan section, the metric series and the per-target accumulator, used by both stories

- [X] T001 [P] Add the types of `contracts/server-load-summary.yaml` to `src/scenario/quality.go`: `Period{Start, End time.Time}`, `CPUThreads{Average, Peak, First float64}`, `MemoryBytes{Peak, First int64}`, `LoadFigures{CPUThreads CPUThreads; MemoryBytes MemoryBytes; Samples, Missed int}` (JSON `cpu_threads`, `memory_bytes`, `samples`, `missed`), `ContainerLoad` (embeds `LoadFigures`; `Name`, `Status` one of `measured`, `partial`, `unavailable`; `Reason string` omitempty; `SharedWith []string` JSON `shared_with` omitempty), `ServerTotal` (embeds `LoadFigures`; `ContainersMeasured int`), `ServerLoad{Available bool; Reason string omitempty; IntervalSeconds int; Period *Period; Containers []ContainerLoad; Total *ServerTotal}`, `LoadMachine{Period *Period; Engine LoadFigures; HardwareThreads, UsableThreads int}`; reason constants `not_requested`, `docker_unreachable`, `permission_denied`, `remote_docker_host`, `not_found`, `not_running`, `no_samples`; add `Server ServerLoad` (JSON `server`) and `LoadMachine LoadMachine` (JSON `load_machine`) to `QualitySummary`; extend `src/scenario/quality_test.go` to marshal a filled summary and check every JSON key of the contract
- [X] T002 [P] Write failing tests in `src/scenario/scenario_test.go` for an optional `TestPlan.ServerMonitoring *ServerMonitoringConfig` (JSON `server_monitoring`, `containers`, `interval_seconds`) validated in `NewScenario` (data-model "Server Monitoring"): absent → nil, scenario created; `interval_seconds` 0/absent → 5; `containers` "1–16 entries, non-empty, distinct" and `interval_seconds` "1–60", with the error texts of `contracts/test-plan-server.yaml` (`server_monitoring.containers must list 1 to 16 containers`, `server_monitoring.containers: empty name at index N`, `server_monitoring.containers: duplicate name "X"`, `server_monitoring.interval_seconds must be between 1 and 60, got N`); then implement it in `src/scenario/scenario.go` (a `withDefaults` like `MediaConfig`'s) and make them pass; an API test in `tests/integration/api/` posts a scenario with 17 containers and gets 400
- [X] T003 [P] Declare the 4 gauges of `contracts/metrics.md` in `src/hostload/metrics.go` with their exact names, labels and help texts (`ServerCPUThreads`, `ServerMemoryBytes` labelled `scenario_id`, `container`; `EngineCPUThreads`, `EngineMemoryBytes` labelled `scenario_id`), registered in `init` like `src/media/metrics.go`, and `deleteSeries(scenarioID string, containers []string)` that removes a scenario's label values
- [X] T004 Write failing tests in `src/hostload/accumulator_test.go` for `accumulator.add(s sample)` and `accumulator.figures() scenario.LoadFigures` (data-model "Target Accumulator", research R3), with synthetic samples: CPU threads = Δcpu_ns / Δwall_ns per interval; average = total CPU / total wall from first to last sample (not the mean of interval rates); peak = highest interval; `first` = first interval's CPU and first sample's memory; peak memory = highest sample; a failed sample → `missed`+1, reason kept, next interval measured from the last good sample; a counter that goes down (restart) closes the segment, the interval is missed, the average sums the segments; one sample only → `samples` 1, CPU 0 with no interval (documented), no sample → `samples` 0; CPU above the machine's threads is never clamped; then implement `src/hostload/accumulator.go` and make them pass

**Checkpoint**: types, validation, series and arithmetic in place; nothing is sampled yet

---

## Phase 3: User Story 1 - Media server load in the report (Priority: P1) 🎯 MVP

**Goal**: the summary of every run with `server_monitoring` has a `server` section per container and in total, the gauges follow the run, and Docker failures are reported, never fatal

**Independent Test**: 10 bots, 1 minute, local docker-jitsi-meet with jvb and jicofo named: `server` section present, both `measured`, bridge average CPU within 10% of `docker stats` over the same minute (quickstart step 2)

### Tests for User Story 1

- [X] T005 [P] [US1] Record two real stats bodies under `src/hostload/testdata/` (cgroup v2 from the local bridge: `curl --unix-socket /var/run/docker.sock 'http://d/containers/docker-jitsi-meet-jvb-1/stats?stream=false&one-shot=true'`; a cgroup v1 body with `total_inactive_file`, written by hand from the Docker API reference) and write failing tests in `src/hostload/docker_test.go` for `parseStats(body) (cpuNS uint64, memBytes int64, err error)` (research R4): CPU from `cpu_stats.cpu_usage.total_usage`; memory `usage − inactive_file` (v2), `usage − total_inactive_file` (v1), `usage` when neither
- [X] T006 [P] [US1] Write failing tests in `src/hostload/docker_api_test.go` against a fake Docker API (`httptest.NewUnstartedServer` on a Unix socket in `t.TempDir()`) for `dockerClient.stats(ctx, name)`: 200 → figures; 404 → `not_found`; 409 or `"running": false` in `/containers/{name}/json` → `not_running`; no socket → `docker_unreachable`; socket without permission (chmod 000) → `permission_denied`; a handler slower than the timeout → error within `min(interval/2, 2 s)`; `newDockerClient` with `DOCKER_HOST=unix:///path` uses that path, with `tcp://…` returns `remote_docker_host` (research R1)
- [X] T007 [P] [US1] Write failing tests in `src/hostload/overlap_test.go` for the process-wide registry (research R9): `watch(scenarioID, containers) (stop func())`; `sharedWith(scenarioID, container) []string` returns every other scenario that watched the same container at any time during this scenario's watch, including ones that stopped before it; concurrent use under `-race`
  - Built as `watch(...) (stop func() map[string][]string)`: `stop` returns the shared-with lists itself, because a later `watch` may prune records an earlier scenario still needs (found by the test).
- [X] T008 [US1] Write failing tests in `src/hostload/sampler_test.go` for `Sampler` with the fake Docker API and a fake clock (`tick` called by the test): `NewSampler(scenarioID string, cfg *scenario.ServerMonitoringConfig, logger)`, `Start(ctx)` samples at once (baseline), then every interval, and once when ctx is cancelled; `Summary() (scenario.ServerLoad, ...)` gives per container `measured` (no miss), `partial` (some missed) or `unavailable` with its reason, `shared_with` from the registry, `total` over measured containers only with `containers_measured`, peak of the per-sample sum (not sum of peaks); `period` from first to last sample; nil config → `available: false`, reason `not_requested`, no request made; all containers unavailable for the same reason → `available: false` with that reason; a tick still running when the next is due is skipped and counted as missed; one WARN log per container per run, not per sample (FR-011); gauges set per sample and deleted after `Summary`
- [X] T009 [P] [US1] Write a failing engine test in `src/execution/engine_test.go`: with the mock adapter and a fake Docker socket (`DOCKER_HOST=unix://…`), a scenario with `server_monitoring` naming one existing and one missing container completes with the same status and in the same time as without it, and its summary has `server.available: true`, the first container `measured`, the second `unavailable`/`not_found`

### Implementation for User Story 1

- [X] T010 [US1] Implement `src/hostload/docker.go`: `dockerClient` with an `http.Client` whose `DialContext` dials the socket (`/var/run/docker.sock` or the `unix://` path of `DOCKER_HOST`), `stats(ctx, name)` calling `GET /containers/{name}/stats?stream=false&one-shot=true` and, on a non-200 answer, `GET /containers/{name}/json` to tell `not_found` from `not_running`, mapping dial errors to `docker_unreachable`/`permission_denied`; `parseStats`; make T005 and T006 pass
  - No `/json` lookup needed: Docker 29.8 answers 404 for an unknown name and 200 with a zero `read` time and empty figures for a stopped container (checked on the local engine), so `parseStats` reports `not_running` from the body; a timeout is reported as `docker_unreachable`.
- [X] T011 [US1] Implement `src/hostload/overlap.go` (mutex-guarded map container → scenario watches with start/stop times) and make T007 pass
- [X] T012 [US1] Implement `src/hostload/sampler.go` and `src/hostload/summary.go`: per tick, all containers in parallel, each with the client timeout, results folded into one accumulator per container and a per-tick total for the peak of the sum; gauges updated per sample; `Summary` builds `scenario.ServerLoad` (containers in test plan order, CPU rounded to 3 decimals); make T008 pass
- [X] T013 [US1] Wire the sampler into `src/execution/engine.go`: created in `NewEngine` from `testPlan.ServerMonitoring`, `Start` launches it before the first bot, its last sample is taken when the context is cancelled, in parallel with the media collector's last collection (never blocking `stop` longer than the client timeout), and `e.quality.Server` is set before `onComplete`; make T009 pass
- [X] T014 [P] [US1] Add `JITSI_SERVER_CONTAINERS` (comma-separated) and `JITSI_SERVER_INTERVAL` (seconds) to `tests/live/jitsi_live_test.go` (documented in its package comment, mapped to `TestPlan.ServerMonitoring`), and log the `server` section with the quality summary; add `--server-containers` and `--server-interval` to `cmd/jitsi-connect/main.go`, printing the section at the end of the run
- [X] T015 [US1] Live validation, quickstart step 2 (SC-001) and step 4 (SC-003): 10 bots `audio_video`, 60 s, jvb + jicofo named, `docker stats` sampled in parallel; then a misspelt container and `DOCKER_HOST=unix:///nonexistent.sock`; record under this task the bridge's average/peak CPU and peak memory from the summary against `docker stats`, the deviations (≤ 10% CPU, ≤ 5% memory), the failure runs' durations and statuses
  - 2026-10-17, local docker-jitsi-meet (jvb 2.3-326, bridge heap 8 GB), 10 bots `audio_video`, 60 s hold, jvb + jicofo named, interval 5 s, `docker stats --no-stream` every ~5 s in parallel (12 readings within the summary's period): bridge average **0.874 threads** against 0.857 from `docker stats` (**2.0%**), peak 1.234; peak memory **3,749,871,616 B** against 3,750,580,191 B (**0.02%**); jicofo 0.004 against 0.002 threads (both near zero); 14 samples, 0 missed per container. **SC-001 met.**
    - `first` CPU is the first interval, which includes the bots joining when ramp-up is short (bridge 1.234 threads, jicofo 0.025); `first` memory is the baseline sample, taken before any bot joins.
  - SC-003, 2 bots, 15 s hold: no monitoring 19.19 s; `no-such-container` named next to the bridge 19.20 s, bridge `measured` (5 samples), `no-such-container` `unavailable`/`not_found`; `DOCKER_HOST=unix:///nonexistent.sock` 19.19 s, `server.available: false`, reason `docker_unreachable`. All passed; one WARN per unavailable container. **SC-003 met.**

**Checkpoint**: US1 complete; server load in every run's summary and in the metrics

---

## Phase 4: User Story 2 - The bots' own load in the same report (Priority: P2)

**Goal**: the summary of every run has a `load_machine` section with the engine's CPU and memory over the same period as `server`, and the machine's thread counts

**Independent Test**: 10 bots, 1 minute: `load_machine.engine.cpu_threads.average` within 5% of `/usr/bin/time`'s (user + system) / elapsed over the summary's period (quickstart step 3)

### Tests for User Story 2

- [X] T016 [P] [US2] Write failing tests in `src/hostload/process_test.go` for `processSample() (cpuNS uint64, rssBytes int64, err error)` (research R2): CPU from `syscall.Getrusage(RUSAGE_SELF)` user + system grows after a busy loop of 50 ms by 40–100 ms; RSS from `/proc/self/statm` (second field × `os.Getpagesize()`) grows after allocating and touching 64 MB; `hardwareThreads()` parses `/sys/devices/system/cpu/online` ranges (`0-15` → 16, `0-3,8-11` → 8, test with a string parser `parseCPUList`); `usableThreads()` = `runtime.NumCPU()`
- [X] T017 [US2] Extend `src/hostload/sampler_test.go` with failing tests: the process is sampled at the same ticks as the containers, also with a nil `ServerMonitoringConfig` (default interval 5 s); `Summary` also returns `scenario.LoadMachine` whose `period` equals `server.period` when containers are named; engine gauges set per sample and deleted after `Summary`

### Implementation for User Story 2

- [X] T018 [US2] Implement `src/hostload/process.go` (`processSample`, `parseCPUList`, `hardwareThreads`, `usableThreads`) and add the process accumulator to the sampler and `scenario.LoadMachine` to `Summary` in `src/hostload/sampler.go` and `src/hostload/summary.go`; the engine always creates the sampler (with or without containers) and sets `e.quality.LoadMachine` in `src/execution/engine.go`; make T016 and T017 pass and extend T009's engine test to check `load_machine`
- [X] T019 [US2] Log the `load_machine` section in `tests/live/jitsi_live_test.go` and print it in `cmd/jitsi-connect/main.go`
- [X] T020 [US2] Live validation, quickstart step 3 (SC-002): 10 bots `audio_video`, 60 s, compiled `live.test` under `/usr/bin/time -v`; record under this task the summary's engine average/peak CPU and peak memory, `/usr/bin/time`'s figures, the deviation (≤ 5%) and both thread counts
  - 2026-10-17, same run as T015 under `/usr/bin/time -v`: engine average **0.399 threads** (peak 0.425, peak RSS 99 MB) against (18.56 s user + 7.08 s system) / 64.39 s = **0.398** (**0.3%**), although `/usr/bin/time` also covers start-up and shutdown; `hardware_threads` 16, `usable_threads` 8 (the shell's CPU affinity, as in 006 T058). **SC-002 met.**

**Checkpoint**: both stories complete

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T021 Cost at scale, quickstart step 5 (SC-004): two same-day 100-bot runs (`audio_video`, cap 10, ramp-up 20 s, 10 min, `/usr/bin/time -v`), the bridge restarted before each, with `JITSI_SERVER_CONTAINERS` set to jvb, jicofo and prosody and without; record both CPU figures (≤ 0.05 core apart), the bridge's average and peak CPU and peak memory from the summary, and the engine's figures under this task
  - 2026-10-17, 100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min, compiled `live.test` under `/usr/bin/time -v`, bridge (8 GB heap) restarted before each run, same evening:
    - **Without monitoring: 2.972 cores** (1474.85 s user + 381.21 s system over 624.53 s), max RSS 841 MB, passed.
    - **With jvb, jicofo and prosody named: 2.987 cores** (1489.30 + 376.68 s over 624.65 s), max RSS 780 MB, passed: **+0.015 core, SC-004 met.** Summary: bridge average **8.711 threads**, peak 9.172, first interval 3.575 (bots joining), peak memory 3.70 GB; jicofo 0.006 (peak 0.19), prosody 0.013 (peak 0.40); total 8.730, peak 9.179, 4.24 GB; 126 samples, 0 missed each. `docker stats` over the run: bridge 8.79 threads. Engine in the summary: 2.987 threads (peak 3.217), 871 MB. MOS p50 4.34, audio concealed p95 0.44%, 294 freezes.
    - Direct cost, `BenchmarkTick_Docker` (`src/hostload/sampler_test.go`) against the three real containers: **0.64 ms of CPU per tick**, 0.00013 thread at the 5 s default (0.0006 at 1 s), about 400 times under the budget: the A/B difference above is run-to-run noise.
    - A first no-monitoring run stopped after 5 min 9 s: one bot's XMPP keepalive write timed out (`keepalive ping failed: failed to write: context deadline exceeded`) on the saturated host (bridge about 8.7 threads, load from both runs' bots), and the live test stops when a bot drops; 2.908 cores until then. Not used.
- [X] T022 [P] Reference the new sections in `specs/002-scenario-execution-engine/contracts/openapi.yaml` (quality endpoint description: `server` and `load_machine` from `specs/007-server-load-report/contracts/server-load-summary.yaml`; `TestPlan.server_monitoring` from `contracts/test-plan-server.yaml`)
- [X] T023 [P] Update `README.md`: 007 in the status table, the `server_monitoring` test plan section with an example for docker-jitsi-meet, the two summary sections, the four series, the live-test variables and CLI flags, and the measured results (SC-001, SC-002, SC-004)
- [X] T024 Write `specs/007-server-load-report/validation-report.md` (as 006's): summary table of SC-001…SC-005 with targets, results and status, environment, and the runs of T015, T020, T021
- [X] T025 Run quickstart steps 1 and 6 (`go vet ./... && go test -race ./...`, the API run with `server_monitoring` and the 400 cases) and record the result under this task
  - 2026-10-17: `go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...` green. Step 6 is run in process by `TestServerLoadThroughAPI` (`tests/integration/execution/server_load_api_test.go`): create with `server_monitoring` over HTTP, `_execute`, `GET .../quality` returns both sections (Docker pointed at a missing socket: `docker_unreachable`, engine sampled); the 400 for 17 containers is T002's API test. The API server's fixed `:8080` is taken by the local bridge's Colibri port, so it was not started by hand.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: T001–T003 in parallel; T004 needs T001 (types). Blocks both stories.
- **US1 (Phase 3)**: needs Phase 2. Tests T005–T007 and T009 in parallel, T008 after T004; implementation T010 → T011 → T012 → T013, then T014, then T015 (live).
- **US2 (Phase 4)**: needs Phase 2 and the sampler of T012 (it adds the process to the same ticks) and the engine wiring of T013; T016 in parallel with US1's tests.
- **Polish (Phase 5)**: T021 needs both stories; T022, T023 in parallel; T024 after T015, T020, T021; T025 last.

### Within Each Story

- Tests first, failing; then the implementation that makes them pass; live validation last.

### Parallel Opportunities

- T001, T002, T003 (three files in two packages).
- T005, T006, T007, T009, T016 (separate test files).
- T014 alongside T013 (live test and CLI, not the engine).
- T022, T023 (contracts and README).

---

## Parallel Example: User Story 1

```bash
# Tests for US1, together:
Task: "T005 parseStats tests on recorded bodies in src/hostload/docker_test.go"
Task: "T006 fake Docker API tests in src/hostload/docker_api_test.go"
Task: "T007 overlap registry tests in src/hostload/overlap_test.go"
Task: "T009 engine test with a fake Docker socket in src/execution/engine_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 2 (T001–T004).
2. US1 (T005–T015): the server section is what the comparison of platforms needs.
3. **Stop and validate**: quickstart steps 2 and 4 on the local Jitsi.

### Incremental Delivery

1. US1 → server load in every report (MVP).
2. US2 → the engine's own load next to it, replacing `/usr/bin/time`.
3. Polish → cost at scale, contracts, README, validation report. 008 (LiveKit) then reuses `server_monitoring` unchanged.
