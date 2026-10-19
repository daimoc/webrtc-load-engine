# Research: Run Report Files

All Technical Context unknowns are resolved below. Code references are to the
tree on `rework` at b8d2eb3 (+ 008 uncommitted).

## R1. Who writes the report: the scenario layer, not the engine

**Decision**: A new package `src/report` builds and writes the two files from
a `report.Run`. It is called by the three entry points: `scenario.Service`
(HTTP API) through an injected `scenario.Reporter` interface, and directly by
`cmd/jitsi-connect` and the live tests (`tests/live`, one shared helper).
`execution.Engine` only hands back the run's result (R2).

**Rationale**: The report needs the scenario's name, platform and the
platform's secret keys (R6); the engine knows none of these (it gets an ID,
a test plan and an adapter). `src/scenario` cannot import `src/report`
(which imports `scenario` for the summary types), so the service takes an
interface it defines, wired in `src/main.go`; a nil reporter (the existing
unit and integration tests) writes nothing. The three call sites share one
function, so content and form are the same (FR-002).

**Alternatives considered**: writing inside the engine (one call site, but it
would have to learn the scenario's name, platform and secret keys, and the
API would still need the result back); a process-wide hook (hidden global,
hard to test).

## R2. Carrying the run's result: `scenario.RunResult` in the completion callback

**Decision**: The engine's completion callback changes from
`func(scenarioID, status string, quality *QualitySummary)` to
`func(scenarioID string, r scenario.RunResult)`, with
`RunResult{Status, Error, StartedAt, EndedAt, Quality}`. The engine sets
`StartedAt` at the top of `Start`, `EndedAt` when the summary is final (in
`Start`'s deferred function, after `collectorDone` and `samplerDone`), and
`Error` when it fails (today only `startMedia`).

**Rationale**: The report needs the start and end times and the error of a
failed run (FR-007), which the callback does not carry; a failed start has
no quality summary at all (`e.quality` is nil when `startMedia` fails), yet
must still be reported (spec edge case). One struct keeps the callback
stable for later additions. The change is mechanical (23 call sites:
engine, service, main, CLI, live tests, unit and integration tests).

**Alternatives considered**: an `Engine.Result()` getter (the service only
holds the `ExecutionEngine` interface and the callback runs in the engine's
goroutine; ordering would be fragile); adding the fields to
`QualitySummary` (changes the API's quality answer, and is nil on failure).

A scenario whose adapter cannot be built (`ExecuteScenario` returns an error,
the scenario never runs) is not a run and gets no report.

## R3. File names and no overwrite

**Decision**: Base name
`<end time UTC, 20061018T150405Z>-<platform>-<name slug>-<run id>`, e.g.
`20261018T153012Z-jitsi-lk-10-bots-3f9a1c2e`. The run ID is 8 hex characters
of a new UUID per run (a re-run keeps the scenario ID, so the scenario ID
cannot tell runs apart). The slug keeps `[a-z0-9]`, maps everything else to
`-`, collapses and trims dashes, and is cut at 40 characters; an empty slug
uses the scenario ID. Each file is written to a temporary file in the same
folder (`os.CreateTemp`, `.<base>.*.tmp`), then published with `os.Link`
to its final name, which fails if the name exists; the temporary file is
removed. On `EEXIST` the base gets `-2`, `-3`… (both files keep the same
base: the JSON file is published first and decides the base).

**Rationale**: Sortable by time in `ls` (SC-004), readable (platform and
name), unique (run ID), and atomic without overwrite (FR-005, FR-006):
`rename` would overwrite silently; `link` + `unlink` publishes a complete
file under a name that did not exist. Linux is the target platform.

**Alternatives considered**: `O_EXCL` on the final name (a reader can see a
partly written file); a per-run subfolder (two files per run do not need
one, and `ls reports/` stays a list of runs); scenario ID in the name (long,
and not unique across re-runs).

## R4. JSON content: one document, the summary embedded as the API serves it

**Decision**: `report.Run` is serialized with `encoding/json`
(`MarshalIndent`, two spaces) as one object: `format_version` (1),
`run_id`, `scenario` {`id`, `name`, `platform`}, `test_plan` (redacted, R6),
`started_at`, `ended_at`, `duration_seconds`, `outcome` {`status`,
`error`}, `engine` {`version`, `go_version`}, `quality` (the
`*scenario.QualitySummary`, the same struct the API encodes; `null` when the
run failed before the media collector started). Contract:
[contracts/run-report.schema.yaml](contracts/run-report.schema.yaml).

**Rationale**: Embedding the struct the API serves makes SC-002 hold by
construction (same type, same encoder); a version field lets later features
add fields without breaking readers.

**Alternatives considered**: the summary alone, with run data in the file
name (loses the test plan and the error); NDJSON or one file per section
(harder to diff and attach).

## R5. Markdown rendering: Go code over the same `report.Run`

**Decision**: `report.Markdown(run) []byte`, built with a `strings.Builder`
and small helpers (`threads`, `mib`, `pct`, `pctl`, `orNotMeasured`). Layout
in [contracts/report-markdown.md](contracts/report-markdown.md): title, key
figures table, then Audio, Video, Server, Load machine, Worst bots, Test
plan (a fenced JSON block of the redacted plan). Measures that do not apply
print the summary's reason (`not measured: <reason>`), never `0`
(FR-009). Values come from the same `report.Run` as the JSON file, rounded
for display only (SC-003).

**Rationale**: The "not measured" logic and unit conversions are easier to
test in Go than in a `text/template`; no dependency.

**Alternatives considered**: `text/template` with `go:embed` (logic moves
into template functions anyway); HTML (not chosen by the user).

## R6. Secrets

**Decision**: The test plan in the report goes through the existing
`scenario.redactPlatformConfig` with the platform's `SecretKeys` (catalog:
Jitsi `jwt_app_secret`, `jwt`; LiveKit `api_secret`, `token`), exported as
`scenario.RedactTestPlan(tp, keys)`. The CLI and the live tests put the
platform config they pass to the adapter into the reported test plan, so the
report says what was run, and redact it the same way.

**Rationale**: One redaction rule for API answers, logs and reports
(FR-010, SC-006); no second list of secret names.

**Alternatives considered**: dropping `platform_config` from reports (loses
the room, URL and domain the operator needs to compare runs).

## R7. Engine version

**Decision**: `report.EngineVersion()` reads `runtime/debug.ReadBuildInfo()`:
`vcs.revision` (12 characters) with `+dirty` when `vcs.modified=true`, else
the main module version, else `devel`; plus `runtime.Version()`.

**Rationale**: `go build` in the repository stamps VCS data for free; `go run`
and `go test` do not, and `devel` says so honestly.

**Alternatives considered**: `-ldflags -X` (needs a build script the project
does not have).

## R8. Configuration of the folder

**Decision**: Default `reports` relative to the working directory, made
absolute when the writer is created. Overrides: HTTP API `--reports-dir`
flag on `go run ./src` (with `REPORTS_DIR` as the flag's default);
`cmd/jitsi-connect --reports-dir` (same default); live tests `REPORTS_DIR`,
defaulting to `<module root>/reports` (module root found by walking up to
`go.mod` from the test's working directory, since `go test` runs in
`tests/live`). The folder is created with `os.MkdirAll(dir, 0o755)` at each
write, so deleting it between runs is harmless. Files are `0o644`.

**Rationale**: FR-003 and the spec's live-test assumption; one variable name
across the three entry points.

**Alternatives considered**: a `reports_dir` field in the test plan (where
results go is a property of the machine, not of the load).

## R9. Failures, visibility and shutdown

**Decision**: `Writer.Write` returns `scenario.ReportFiles{RunID, JSON,
Markdown ReportFile}`, each `ReportFile{Path, Error}`. Each file is attempted
independently (the Markdown file is still written if the JSON one fails,
with its own base); errors are logged once per file with `scenario_id`,
`run_id` and the reason, and never change the run's status (FR-012). The
service stores the `ReportFiles` with the scenario (`Scenario.report`, reset when
a run starts) and `GET /api/v1/scenarios/{id}` returns them as `report`
([contracts/api-scenario-report.yaml](contracts/api-scenario-report.yaml));
the quality endpoint is unchanged (FR-014). The write runs in the
completion callback after the service lock is released, so a slow disk
never blocks API readers. `Service.Shutdown` waits (up to 10 s) for the
completion callbacks of the engines it stops, so a graceful shutdown writes
their reports; the CLI writes after `<-done` on Ctrl+C.

**Rationale**: FR-012–FR-015 and the shutdown edge case. Today `Shutdown`
calls `engine.Stop()` and returns while `Start`'s deferred completion may
still be running; `main` would exit before the report is written.

**Alternatives considered**: a write queue goroutine (unneeded: one small
write per run).

## R10. Cost

**Decision**: The report is built once, after `EndedAt`, from the final
summary (already bounded: 10 worst bots per ranking, 006). No work during
the run (FR-016, constitution IV). Expected size: a few tens of kB at 100
bots.

**Rationale**: SC-008.

## R11. Version control of `reports/`

**Decision**: Add `/reports/` to `.gitignore`. Reports kept as evidence are
copied or quoted into a feature's `validation-report.md`, as today.

**Rationale**: Every run (including unit-style live runs) adds two files;
committing them by default would flood the history. The spec leaves this
to the operator; ignoring is the safe default and is one line to undo.
