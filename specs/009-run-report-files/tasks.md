---

description: "Task list for 009 run report files"
---

# Tasks: Run Report Files

**Input**: Design documents from `specs/009-run-report-files/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires core components to be test-covered (plan "Constitution Check"): unit tests for names, collisions, failures, JSON and Markdown content, redaction; service and API tests for the report status; an engine test for the run result. Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`); code stays go1.23-compatible; `go vet ./... && go test -race ./...` stays green after every task. Unit and integration tests write reports only into `t.TempDir()`, never into the repository's `reports/`. `src/report` imports `src/scenario` only (no `execution`, `media`, `platform`: constitution II/III); `src/scenario` never imports `src/report` (import cycle, research R1). A report write never changes a run's status or blocks API readers (FR-012). No new module. Live runs: Jitsi from `tests/platforms/docker-jitsi-meet`, LiveKit from `tests/platforms/livekit`, one at a time ([quickstart.md](quickstart.md)).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: keep run reports out of git

- [X] T001 Add `/reports/` to `.gitignore` under a `# Run reports (009): two files per run` comment (research R11)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the run result carried by the completion callback, the shared result types and the redaction helper, used by every story

- [X] T002 [P] Create `src/scenario/result.go` with the types of data-model.md: `RunResult{Status, Error string; StartedAt, EndedAt time.Time; Quality *QualitySummary}` ("Status: `completed`, `stopped` or `failed`"; "Error: set when Status is `failed`; empty otherwise"; times UTC, "EndedAt ≥ StartedAt"); `ReportFile{Path string \`json:"path,omitempty"\`; Error string \`json:"error,omitempty"\`}` ("exactly one of Path and Error is set per file"); `ReportFiles{RunID string \`json:"run_id"\`; JSON ReportFile \`json:"json"\`; Markdown ReportFile \`json:"markdown"\`}`; `Reporter interface { WriteReport(sc *Scenario, r RunResult) ReportFiles }` with a doc comment saying `sc` is the redacted snapshot
- [X] T003 [P] Write a failing test in `src/scenario/catalog_test.go` for `RedactTestPlan(tp TestPlan, keys []string) TestPlan`: secret `platform_config` keys become `Redacted` (`"[redacted]"`), other keys and the rest of the plan unchanged, the input map not modified, nil config stays nil; implement it in `src/scenario/catalog.go` over `redactPlatformConfig` and make `Service.redacted` and `CreateScenario`'s log use it
- [X] T004 Change the completion callback everywhere from `func(scenarioID, status string, quality *QualitySummary)` to `func(scenarioID string, r scenario.RunResult)` (research R2): `ExecutionStarter.StartExecution` in `src/scenario/service.go`, `NewEngine` and the `onComplete` field in `src/execution/engine.go`, `MainEngineStarter.StartExecution` in `src/main.go`, `cmd/jitsi-connect/main.go`, `tests/live/jitsi_live_test.go`, `tests/live/livekit_live_test.go`, and the test call sites (`src/main_test.go`, `src/execution/engine_test.go`, `src/execution/server_load_test.go`, `src/execution/simulcast_test.go`, `src/scenario/service_test.go`, `src/scenario/catalog_test.go`, `src/platform/livekit/adapter_test.go`, `tests/integration/api/platform_api_test.go`, `tests/integration/execution/scenario_execution_test.go`); behaviour unchanged (find them with `grep -rn "QualitySummary)" --include=*.go`)
- [X] T005 Write a failing test in `src/execution/engine_test.go` (mock adapter): a completed run's `RunResult` has `StartedAt` before the first peer connects and `EndedAt` after the summary's `GeneratedAt`, both UTC, `Error` empty; a run whose `startMedia` fails (test plan with a `media.source_file` that does not exist) has `Status` `failed`, `Error` the startMedia error text, `Quality` nil, `EndedAt ≥ StartedAt`; then set `StartedAt` at the top of `Engine.Start`, `Error` where `finalStatus = "failed"`, and `EndedAt` in `Start`'s deferred function after `<-e.collectorDone` and `<-e.samplerDone`, in `src/execution/engine.go`, and make it pass

**Checkpoint**: the callback carries times, error and summary; nothing is written yet

---

## Phase 3: User Story 1 - Every run leaves a report file (Priority: P1) 🎯 MVP

**Goal**: every run, through the API, the CLI or a live test, writes its JSON report to `reports/` (default folder), never overwriting, and the API tells where it went or why it did not

**Independent Test**: 2 bots, 30 s through the HTTP API: one new JSON file in `reports/` whose `quality` equals `GET /api/v1/scenarios/{id}/quality`, with test plan, platform, times and outcome; `GET /api/v1/scenarios/{id}` gives its path (quickstart step 2)

### Tests for User Story 1

- [X] T006 [P] [US1] Write failing tests in `src/report/name_test.go` for `baseName(endedAt time.Time, platform, name, scenarioID, runID string) string` and `slug(string) string` (research R3): `20261018T153012Z-jitsi-ten-bots-3f9a1c2e` for end time 2026-10-18 15:30:12 UTC (a non-UTC time is converted), platform `jitsi`, name `Ten bots`, run `3f9a1c2e`; slug keeps `[a-z0-9]`, lower-cases, maps every other character (slash, space, accent, dot) to `-`, collapses and trims dashes, cuts at 40 characters; an empty slug (name `"///"` or `""`) uses the scenario ID's slug; `newRunID()` returns 8 lowercase hex characters (`^[0-9a-f]{8}$`) and differs across calls
- [X] T007 [P] [US1] Write failing tests in `src/report/run_test.go` for `NewRun(sc *scenario.Scenario, r scenario.RunResult) Run` and its JSON (contracts/run-report.schema.yaml): every required key present (`format_version` 1, `run_id`, `scenario`{`id`,`name`,`platform`}, `test_plan`, `started_at`, `ended_at`, `duration_seconds`, `outcome`{`status`,`error`}, `engine`{`version`,`go_version`}, `quality`); the name kept exactly as given (accents, slashes); `duration_seconds` = EndedAt − StartedAt rounded to 3 decimals; `outcome.error` omitted when empty; `quality` is `null` when `r.Quality` is nil; the `quality` object re-encoded equals `json.Marshal(r.Quality)` byte for byte (SC-002); `EngineVersion()` returns `devel` or a non-empty string under `go test`, and `go_version` = `runtime.Version()`
- [X] T008 [US1] Write failing tests in `src/report/writer_test.go` for `Writer{Dir string; Logger *slog.Logger}.Write(run Run) scenario.ReportFiles` (JSON part only for now; data-model "Writer"): the folder is created if missing (nested path); the JSON file is `<Dir>/<base>.json`, mode `0644`, absolute path returned, content parses back to the run; no `.tmp` file left in the folder after success or failure; 50 writes of runs with the same end time, platform, name **and run ID** produce 50 distinct JSON files (`<base>.json`, `<base>-2.json` … `<base>-50.json`) and none is overwritten (SC-004); an existing file with the target name is left unchanged; `Dir` under a `chmod 555` folder → `JSON.Path` empty, `JSON.Error` contains `permission denied`, one WARN log with `scenario_id`, `run_id`, `error`, no panic (skip when running as root); a reader polling the folder never sees a partly written file (publish by link, R3)
- [X] T009 [P] [US1] Write failing tests in `src/scenario/service_test.go` with a fake `Reporter` (records calls, returns fixed `ReportFiles`, optionally sleeps): after a run ends, `WriteReport` is called once with the redacted snapshot (secret keys `[redacted]`) and the `RunResult`; `GetScenario` returns the files through a new `Scenario.Report() *ReportFiles` accessor; while a re-run is running `Report()` is nil (reset in `ExecuteScenario`); a `Reporter` sleeping 500 ms does not block `GetScenario`/`GetQuality` (lock released before the write); nil reporter writes nothing and changes nothing; `Shutdown` with a running scenario returns only after its `WriteReport` returned (up to 10 s); the scenario's status is the engine's status whatever `ReportFiles` holds (FR-012)
- [X] T010 [P] [US1] Write a failing API test in `tests/integration/api/report_api_test.go`: a service wired with a `report.Writer` on `t.TempDir()` and a fast mock engine; after the run, `GET /api/v1/scenarios/{id}` has `report.run_id` (`^[0-9a-f]{8}$`), `report.json.path` pointing to an existing file in the temp folder, and the file's `quality` deep-equals the body of `GET /api/v1/scenarios/{id}/quality`; before the first run and while running, `report` is absent; the quality endpoint's body has no new field (FR-014); a Writer on an unwritable folder → `report.json.error` set, `status` `completed` (contracts/api-scenario-report.yaml)

### Implementation for User Story 1

- [X] T011 [US1] Implement `src/report/name.go` (`baseName`, `slug`, `newRunID` from `uuid.New()`) and `src/report/run.go` (`Run` with the JSON tags of contracts/run-report.schema.yaml, `NewRun`, `EngineVersion()` from `runtime/debug.ReadBuildInfo`: `vcs.revision` cut to 12 characters plus `+dirty` when `vcs.modified=true`, else the main module version if not `(devel)`, else `devel`, research R7); make T006 and T007 pass
- [X] T012 [US1] Implement `src/report/writer.go`: `Write` runs `os.MkdirAll(Dir, 0o755)`, picks the base with `baseName`, publishes the JSON file (`json.MarshalIndent`, two spaces, trailing newline) with `publish(dir, base, ext, data) (path string, err error)`: `os.CreateTemp(dir, "."+base+".*.tmp")`, write, close, `os.Chmod 0o644`, `os.Link(tmp, final)`, on `os.ErrExist` retry with `-2`, `-3`… (the JSON file decides the base, R3), always remove the temp file; logs one WARN per failed file and one INFO `report written` with both paths; make T008 pass
- [X] T013 [US1] Wire the reporter into the service in `src/scenario/service.go` and `src/scenario/scenario.go`: `Scenario.report *ReportFiles` (copied by `snapshot`), `Report()` accessor, `Service.SetReporter(Reporter)`; the completion callback sets status and quality under the lock, takes the redacted snapshot, releases the lock, calls `WriteReport`, then stores the files under the lock; `ExecuteScenario` resets `report` to nil; a `sync.WaitGroup` counts pending completions (Add in `ExecuteScenario`, Done at the end of the callback) and `Shutdown` waits for it after stopping the engines, at most 10 s (log a WARN on timeout); make T009 pass
  - The tests are in `src/scenario/service_report_test.go`. A run counter on the scenario drops a late report when a newer run has started meanwhile; `Shutdown` releases the engine lock before waiting (the completion callback takes it).
- [X] T014 [US1] Expose the report in the API: `ScenarioResponse` gains `Report *scenario.ReportFiles \`json:"report,omitempty"\`` filled from `Report()` in `src/api/handlers/scenario_handler.go`; in `src/main.go` build `report.Writer{Dir: <abs path of "reports">, Logger: logger}` and set an adapter `reportWriter` (implements `scenario.Reporter` with `report.NewRun` + `Write`) on the service; update `specs/002-scenario-execution-engine/contracts/openapi.yaml` `ScenarioResponse` with `report` per contracts/api-scenario-report.yaml; make T010 pass
  - `report.Writer` implements `scenario.Reporter` itself (`WriteReport` = `Write(NewRun(sc, r))`) instead of an adapter in `src/main.go`, so the integration test wires the real writer too.
- [X] T015 [US1] Write the report from the CLI in `cmd/jitsi-connect/main.go`: keep the `RunResult` from the callback (both the normal end and the Ctrl+C path wait for it), build the reported scenario from `sc` with `Platform` `jitsi` and `TestPlan.PlatformConfig` = the `config` map given to the adapter, redact it with `scenario.RedactTestPlan(tp, catalog.New(logger).SecretKeys("jitsi"))`, write with `report.Writer{Dir: abs("reports")}`, and print the paths (`report written json=… markdown=…`) or one warning per failed file (FR-015)
- [X] T016 [US1] Add `tests/live/report_test.go` with `writeReport(t *testing.T, sc *scenario.Scenario, r scenario.RunResult, secretKeys []string)`: redacts the plan, writes with a `report.Writer` whose folder is `moduleRoot()/reports` (walk up from the working directory to the folder holding `go.mod`, research R8), and `t.Logf`s the paths or `t.Errorf`s a failed file; call it at the end of `TestJitsiLive_JoinAndEstablishMedia` in `tests/live/jitsi_live_test.go` (name `jitsi-live`, platform `jitsi`, the adapter config as platform config) and `TestLiveKitLive_JoinAndEstablishMedia` in `tests/live/livekit_live_test.go` (name `livekit-live`, platform `livekit`); build with `go vet -tags live ./tests/live/`
  - The live tests keep their scenario names (`live`, `live-livekit`). `moduleRoot` lives in `tests/live/report_test.go` until T023.
- [X] T017 [US1] Live check (quickstart steps 3 and 4) on the local Jitsi: `jitsi-connect` 2 bots 30 s, then Ctrl+C on a second run after 10 s (status `stopped`, file still written), then the Jitsi live test with 2 bots; record under this task the file names, that `tests/live/reports` does not exist, and that `jq .quality` of the CLI report shows the same server section the CLI printed
  - **Recorded 2026-10-18** (local docker-jitsi-meet, `unstable` images; LiveKit container idle):
    - CLI, 2 bots `audio_video`, 30 s, jvb + jicofo named: `reports/20261018T212058Z-jitsi-jitsi-connect-b4a88cb6.json` (4.8 kB), outcome `completed`, duration 30.008 s, engine `b8d2eb3029e9+dirty`; `platform_config` is the CLI's adapter config (room `r009`); quality: 2 bots, MOS p50 4.37; server: jvb `measured` avg 0.453 threads, jicofo `measured` avg 0.045, as printed by the CLI.
    - CLI, Ctrl+C after 12 s: `reports/20261018T212148Z-jitsi-jitsi-connect-325ff28f.json` written 2 ms after the disconnect, outcome `stopped`.
    - Jitsi live test, 2 bots `audio_video`: PASS, `reports/20261018T212208Z-jitsi-live-3b67625a.json` (scenario `live`, outcome `stopped`, room `r009c`) in the repository's `reports/`; `tests/live/reports` does not exist.

**Checkpoint**: US1 complete: every run leaves a JSON report; the API tells where

---

## Phase 4: User Story 2 - A readable report next to the data (Priority: P2)

**Goal**: each run also writes `<base>.md`, key figures first, values equal to the JSON file, "not measured" where a measure does not apply

**Independent Test**: 10 bots audio+video for 1 minute: the Markdown file's key-figures table gives participants, platform, duration, outcome, MOS median and worst, freezes, frame rate, resolution, server and engine CPU and memory, equal to the JSON values (quickstart step 3)

### Tests for User Story 2

- [X] T018 [P] [US2] Add recorded summaries under `src/report/testdata/`: `full.json` (a real 10-bot audio+video Jitsi summary with server and load_machine sections, copied from a T017 report), `receive_only.json` (audio and video `available: false` with their reasons, `server.available: false` reason `not_requested`), `failed.json` (a failed run: `quality` null); then write failing tests in `src/report/markdown_test.go` for `Markdown(run Run) []byte` against contracts/report-markdown.md: sections in the order title, Key figures, Audio, Video, Server, Load machine, Worst bots, Test plan; title `# Load test report: <name>`; the failed run shows `> Error: <error>` and `Not measured` lines, no table of zeros; each key figure equals the JSON value after the contract's rounding (CPU 2 decimals in threads, memory MiB 1 decimal, ratios % 2 decimals, seconds 3 decimals, frame rate 1 decimal, MOS 2 decimals) (SC-003); "worst bot" MOS is `worst.lowest_mos[0].value`; receive-only shows `not measured: <reason>` for MOS, freezes and frame rate and `Server load not requested.`; an empty worst-bots ranking shows `None.`; the test plan block contains `[redacted]` and not the secret; the output ends with `Engine <version> (<go_version>)`
  - `full.json` is the 2-bot `audio_video` CLI run of T017 (`b4a88cb6`: every section filled, server measured); the failed run is built in the test (`quality` nil). Latency is shown in milliseconds (1 decimal), not seconds: bot-to-bot latencies are often under 1 ms; contracts/report-markdown.md updated.

### Implementation for User Story 2

- [X] T019 [US2] Implement `src/report/markdown.go` (`Markdown(run Run) []byte` with a `strings.Builder` and helpers `threads(float64)`, `mib(int64)`, `pct(float64)`, `pctl(*scenario.Percentiles, format)`, `notMeasured(reason)`; layout and units of contracts/report-markdown.md) and make T018 pass
- [X] T020 [US2] Write the Markdown file in `Writer.Write` in `src/report/writer.go` with the JSON file's base (`<base>.md`, same publish), attempted even when the JSON file failed (then with the base the JSON attempt chose); extend `src/report/writer_test.go`: both files share the base, the 50-collision test yields 50 distinct pairs, a read-only folder fails both files each with its own error, and a JSON failure alone (a directory already named `<base>.json`) still writes the Markdown file (spec edge case "one of the two files")
  - The JSON failure is forced through a `marshalJSON` variable: a directory named `<base>.json` only makes the name taken (the JSON file moves to `-2`), it does not fail.
- [X] T021 [US2] Live check (quickstart step 3): `jitsi-connect` 10 bots `audio_video` 60 s with jvb and jicofo named; record under this task the key-figures table of the Markdown report and that each value matches `jq` on the JSON file
  - **Recorded 2026-10-18**: `reports/20261018T212553Z-jitsi-jitsi-connect-0e0fd6ab.{json,md}` (7.8 kB, 4.3 kB). Key figures: participants 10 (bots measured 10); 60 s, run 60.010 s; completed; MOS median 4.37, worst bot 4.37; concealed p95 0.00 %; video freezes 27, freeze time p95 44.966 s; frame rate 29.9 / 30.0 / 30.0 fps; 360: 100.00 %; latency p50 1.5 ms / 1.5 ms; server avg 0.97 threads, peak 1.42, peak 813.0 MiB; engine avg 0.36, peak 0.39 threads, 92.0 MiB, 16 hardware threads. Every value equals the JSON file's after rounding (checked with Python; no `jq` on this machine). The freezes come from the CLI run itself (no settle wait, every bot receives 9 videos), not from the report.

**Checkpoint**: US2 complete: every run has a readable report next to its data

---

## Phase 5: User Story 3 - Choose where reports go (Priority: P3)

**Goal**: the reports folder is set per entry point (contracts/configuration.md)

**Independent Test**: CLI, API and live test with another folder configured write there and not to `reports/` (quickstart steps 2 and 4)

### Tests for User Story 3

- [X] T022 [P] [US3] Write failing tests in `src/report/dir_test.go` for `ResolveDir(flagValue string) (string, error)`: flag set → its absolute path; flag empty and `REPORTS_DIR` set (`t.Setenv`) → that, absolute; neither → `<working dir>/reports`; and `ModuleRootDir(start string) (string, error)` walking up to the folder holding `go.mod` (temp tree with `go.mod` two levels up), error when none

### Implementation for User Story 3

- [X] T023 [US3] Implement `src/report/dir.go` (`ResolveDir`, `ModuleRootDir`; move `moduleRoot` of T016 to it) and make T022 pass; add `--reports-dir` to `go run ./src` in `src/main.go` (use `flag`; default empty → `ResolveDir`) and to `cmd/jitsi-connect/main.go` (help: `Folder of the run reports (default $REPORTS_DIR, else ./reports)`); in `tests/live/report_test.go` use `REPORTS_DIR` when set, else `ModuleRootDir(".")/reports`; log the resolved folder once at start in the server and the CLI
- [X] T024 [US3] Check quickstart step 4's `REPORTS_DIR=/tmp/r009-live` live run and step 5's read-only folder with the CLI (run ends `completed` in the same time, one warning per file, SC-005); record the results under this task
  - **Recorded 2026-10-18**: Jitsi live test, 2 bots, `REPORTS_DIR=<scratch>/r009-live`: PASS, both files written there, the repository's `reports/` unchanged (5 files before and after). CLI, 2 bots, 10 s, `--reports-dir <scratch>/ro/reports` under a `chmod 555` folder: `completed`, two WARN lines (`file=json`, `file=markdown`, `mkdir …: permission denied`), 10.34 s wall time against 10.01 s for the same run into a writable folder (difference is start-up noise, under SC-005's 1 s).

**Checkpoint**: all stories complete

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T025 [P] Document the reports in `README.md` (where they go, the two files, `--reports-dir` / `REPORTS_DIR`, the `report` field of the scenario answer) and in `tests/platforms/docker-jitsi-meet/README.md` and `tests/platforms/livekit/README.md` (one line: the run's reports are in `reports/`); add a `go run ./cmd/jitsi-connect ... --reports-dir /tmp/r` line to the Commands of `CLAUDE.md`
- [X] T026 [P] Secret check (SC-006): a test in `src/report/writer_test.go` writes a Jitsi run whose plan has `jwt_app_secret: "s3cr3t-009"` and a LiveKit run with `api_secret: "s3cr3t-lk"` through `scenario.RedactTestPlan` with the catalog's keys, and asserts neither secret occurs in any file of the folder; on a token-auth Jitsi, if available, run quickstart step 5's `grep` and record it
  - `TestWriteNoSecrets` covers the four secret keys of both platforms (`jwt_app_secret`, `jwt`, `api_secret`, `token`) in both files. No token-auth Jitsi runs locally (the local `.env` keeps the JWT block commented out), so the `grep` step was not run against a live deployment.
- [X] T027 Scale and cost (SC-001, SC-008, quickstart step 6): the 100-bot LiveKit run (audio+video, cap 10, 10 min, `LIVEKIT_SERVER_CONTAINERS=livekit-server`); record under this task the time from the run's `ended_at` to both files' modification time (≤ 2 s), the report sizes, and the engine CPU against 008's validation report (no increase beyond run-to-run noise)
  - **Recorded 2026-10-18** (Jitsi stopped): PASS, 100 bots established in 20.5 s, no freezes, MOS p50 4.365; reports `20261018T213912Z-livekit-live-livekit-c7d02ec7.{json,md}` (6.7 kB, 3.9 kB) 23 ms after `ended_at`. Engine 3.17 cores (`/usr/bin/time`; `load_machine` 3.175), server 4.94 threads average, peak 5.49, 1.10 GiB. 008 recorded 1.43 cores and 2.28 threads on 2026-10-17: both doubled, including the server, which 009 does not touch. **SC-008's CPU part is not confirmed**; an A/B run against a build without 009 is needed (validation-report.md).
- [X] T028 Write `specs/009-run-report-files/validation-report.md` with the results of T017, T021, T024, T026, T027 per success criterion SC-001…SC-008, and run `go vet ./... && go test -race ./...` and `go vet -tags live ./tests/live/` a last time (all green)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: T001, independent.
- **Foundational (Phase 2)**: T002 and T003 in parallel; T004 needs T002 (`RunResult`); T005 after T004. Blocks all stories.
- **US1 (Phase 3)**: needs Phase 2. Tests T006, T007, T009, T010 in parallel, T008 after T011's types exist or alongside it; implementation T011 → T012 → T013 → T014; T015 and T016 after T012 (in parallel with T013/T014); T017 last.
- **US2 (Phase 4)**: needs T011/T012 (Run, Writer). T018 needs a real summary (T017) for `full.json`; T019 → T020 → T021.
- **US3 (Phase 5)**: needs T014–T016 (the three entry points); T022 → T023 → T024. Independent of US2.
- **Polish (Phase 6)**: T025, T026 in parallel after US1–US3; T027 after T020; T028 last.

### Within Each Story

- Tests first, failing; then the implementation that makes them pass; live check last.

### Parallel Opportunities

- T002, T003 (two files in `src/scenario`).
- T006, T007, T009, T010 (separate test files in three packages).
- T015, T016 alongside T013–T014 (CLI and live tests vs service and API).
- US2 and US3 once US1 is done (`src/report/markdown.go` vs `src/report/dir.go` and the entry points' flags; both touch `src/report/writer.go` only in T020).
- T025, T026.

---

## Parallel Example: User Story 1

```bash
# Tests for US1, together:
Task: "T006 baseName and slug tests in src/report/name_test.go"
Task: "T007 NewRun and JSON tests in src/report/run_test.go"
Task: "T009 service tests with a fake Reporter in src/scenario/service_test.go"
Task: "T010 API test for the report field in tests/integration/api/report_api_test.go"
```

## Parallel Example: after US1

```bash
Task: "T018–T020 Markdown report (US2) in src/report/markdown.go"
Task: "T022–T023 reports folder settings (US3) in src/report/dir.go, src/main.go, cmd/jitsi-connect/main.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 1–2 (T001–T005).
2. US1 (T006–T017): every run leaves a JSON report in `reports/`, and the API says where.
3. **Stop and validate**: quickstart steps 2–4.

### Incremental Delivery

1. US1 → durable, machine-readable report of every run (MVP).
2. US2 → readable Markdown report next to it.
3. US3 → configurable folder.
4. Polish → docs, secrets, 100-bot cost, validation report. 010 (mediasoup-demo) then gets reports for free.
