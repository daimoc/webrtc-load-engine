# Implementation Plan: Run Report Files

**Branch**: `009-run-report-files` (work continues on `rework`) | **Date**: 2026-10-18 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/009-run-report-files/spec.md`

## Summary

A new package, `src/report`, turns a finished run into two files in the
reports folder: a JSON document (run identity, redacted test plan, times,
outcome, engine version and the 006/007 quality summary as the API serves
it, R4) and a Markdown report rendered from the same data, key figures
first (R5). Files are named `<end time>-<platform>-<name>-<run id>` and
published atomically without overwrite (temporary file + `link`, R3). To
know the start and end times and a failure's error, the engine's
completion callback now passes a `scenario.RunResult` (R2). The three entry
points call the same writer: `scenario.Service` through an injected
`Reporter` (API), `cmd/jitsi-connect` and the live tests (R1). The folder
defaults to `reports/` (the module root's for live tests) and is set with
`--reports-dir` / `REPORTS_DIR` (R8). Write failures never change the run:
they are logged and returned in the scenario's new `report` field of
`GET /api/v1/scenarios/{id}`; a graceful shutdown waits for pending
reports (R9).

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: standard library only (`encoding/json`, `os`, `path/filepath`, `runtime/debug`, `strings`); `google/uuid` (already used) for run IDs. No new module.

**Storage**: files: two per run in the reports folder (`reports/` by default); the scenario keeps only the paths in memory

**Testing**: `go test -race ./...`: unit tests in `src/report` (base name and slug, collisions with forced run IDs, atomic publish, unwritable folder, JSON round trip equal to the summary, Markdown key figures and "not measured" lines from recorded summaries, redaction); service tests with a fake `Reporter` (status, report field, reset on re-run, shutdown waits); API integration test for the `report` field; engine test for `RunResult` times and error; live tests write reports (quickstart steps 3–6)

**Target Platform**: Linux (atomic no-overwrite publish relies on `link(2)`)

**Project Type**: single project (engine + HTTP API + CLI)

**Performance Goals**: reports written within 2 s of a run's end (SC-001), at 100 bots too (SC-008)

**Constraints**: no work during the run (FR-016, constitution IV); a write failure never changes the outcome (FR-012); no secret in a file (FR-010); the API's quality answer unchanged (FR-014); no platform logic in `src/report` (constitution III)

**Scale/Scope**: one pair of files per run, tens of kB at 100 bots (summary bounded by 006's 10 worst bots per ranking); no retention or listing

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **I. Infrastructure-First**: reports what the synthetic peers measured; no browser.
- [x] **II. Media Engine Independence**: the media layer is untouched; `src/report` reads the finished summary only.
- [x] **III. Pluggable Signaling**: `src/report` knows no platform; secret keys come from the platform catalog, as for API answers; adapters are untouched.
- [x] **IV. CPU Efficiency**: one write per run after the bots stop; nothing per bot or per packet during the run (SC-008).
- [x] **V. Deterministic Peer Lifecycle**: unchanged; the callback still fires once per run with the same statuses, now with times and error.
- [x] **Observability**: the run's results become durable files; one log line per written run and per failed file; report status in the API.
- [x] **Test Discipline**: every failure case of the spec has a unit or service test; live validation in quickstart.
- [x] **Incremental Complexity**: two files, no index, no retention, no comparison, no new dependency.
- [x] **Scenario Definition**: test plans are unchanged; the folder is an engine setting, not a test-plan field (R8).

Post-design re-check: still passing. The callback signature change (R2) is
internal (23 call sites, mechanical) and recorded in research; no entry in
Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/009-run-report-files/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── run-report.schema.yaml     # JSON report file
│   ├── report-markdown.md         # Markdown report layout and units
│   ├── api-scenario-report.yaml   # `report` field of GET /api/v1/scenarios/{id}
│   └── configuration.md           # --reports-dir / REPORTS_DIR per entry point
├── checklists/requirements.md
└── tasks.md                       # /speckit-tasks
```

### Source Code (repository root)

```text
src/
├── report/                    # new, platform-independent
│   ├── run.go                 # Run, NewRun (identity, redacted plan, times, outcome), EngineVersion
│   ├── name.go                # base name, slug, run ID
│   ├── writer.go              # Writer{Dir}: MkdirAll, atomic no-overwrite publish, returns scenario.ReportFiles, logging
│   ├── markdown.go            # Markdown(run): key figures, sections, units
│   ├── dir.go                 # DefaultDir (REPORTS_DIR or ./reports), ModuleRootDir for live tests
│   └── *_test.go              # names, collisions, failures, JSON = summary, Markdown, redaction
├── scenario/
│   ├── result.go              # RunResult; ReportFiles/ReportFile; Reporter interface
│   ├── catalog.go             # RedactTestPlan (exported wrapper of redactPlatformConfig)
│   ├── scenario.go            # Scenario.report
│   └── service.go             # new callback, Reporter call outside the lock, report reset, Shutdown waits
├── execution/
│   └── engine.go              # RunResult: StartedAt, EndedAt, Error
├── api/handlers/
│   └── scenario_handler.go    # ScenarioResponse.Report
└── main.go                    # --reports-dir, report.Writer as the service's Reporter

cmd/jitsi-connect/main.go      # --reports-dir; writes the report after the run; prints paths
tests/live/report_test.go      # shared helper: module-root default, write, log paths
tests/live/{jitsi,livekit}_live_test.go   # call the helper
.gitignore                     # /reports/
specs/002-scenario-execution-engine/contracts/openapi.yaml   # `report` in the scenario answer
```

**Structure Decision**: a new package rather than more code in
`src/scenario`, because the writer and the Markdown renderer are the bulk of
the feature and are needed by the CLI and the live tests, which do not go
through the service. The interface and the small result types stay in
`src/scenario` so the service does not import `src/report` (import cycle).

## Complexity Tracking

None.
