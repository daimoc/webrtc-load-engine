# Data Model: Run Report Files

## RunResult (`src/scenario`)

What the engine hands back when a run ends (replaces the `status, quality`
arguments of the completion callback, research R2).

| Field | Type | Rule |
|-------|------|------|
| Status | string | `completed`, `stopped` or `failed` (unchanged values) |
| Error | string | Set when Status is `failed`; empty otherwise |
| StartedAt | time.Time | UTC, set at the top of `Engine.Start` |
| EndedAt | time.Time | UTC, set when the summary is final; ≥ StartedAt |
| Quality | *QualitySummary | The 006/007 summary; nil when the run failed before the media collector started |

## Run (`src/report`)

One report, serialized as the JSON file and rendered as the Markdown file
([contracts/run-report.schema.yaml](contracts/run-report.schema.yaml)).

| Field | JSON | Source / rule |
|-------|------|---------------|
| FormatVersion | `format_version` | Constant 1 |
| RunID | `run_id` | 8 hex characters, new for each run (R3) |
| Scenario | `scenario` {`id`, `name`, `platform`} | Scenario identity; `name` kept exactly as given |
| TestPlan | `test_plan` | `scenario.TestPlan` with secrets replaced by `[redacted]` (R6) |
| StartedAt, EndedAt | `started_at`, `ended_at` | From RunResult, RFC 3339 UTC |
| DurationSeconds | `duration_seconds` | EndedAt − StartedAt, 3 decimals |
| Outcome | `outcome` {`status`, `error`} | From RunResult |
| Engine | `engine` {`version`, `go_version`} | R7 |
| Quality | `quality` | RunResult.Quality, the struct the API serves; `null` if nil |

Built by `report.NewRun(sc *scenario.Scenario, result scenario.RunResult)`
from a scenario whose test plan is already redacted (the service passes its
redacted snapshot; the CLI and live tests call `scenario.RedactTestPlan`).
`NewRun` never fails.

## ReportFiles and ReportFile (`src/scenario`)

The outcome of writing a run's reports, kept with the scenario and shown by
the API ([contracts/api-scenario-report.yaml](contracts/api-scenario-report.yaml)).

| Field | JSON | Rule |
|-------|------|------|
| RunID | `run_id` | Same as Run.RunID |
| JSON | `json` | ReportFile |
| Markdown | `markdown` | ReportFile |
| ReportFile.Path | `path` | Absolute path when written; empty otherwise |
| ReportFile.Error | `error` | Why the file was not written; empty when written |

Exactly one of Path and Error is set per file.

## Writer (`src/report`)

| Field | Rule |
|-------|------|
| Dir | Absolute folder; default `<working dir>/reports` (R8) |
| Logger | Logs each failed file once, with `scenario_id`, `run_id`, `error` |

`Write(run) scenario.ReportFiles`: creates Dir if missing, picks the base name (R3), writes
the JSON file then the Markdown file, each atomically and without
overwrite. Never returns an error: failures are in the returned `ReportFiles`.

## Scenario (`src/scenario`), changed

| Field | Change |
|-------|--------|
| report | New, unexported: `*ReportFiles` of the last run; nil before the first run ends and while a run is running (reset at `ExecuteScenario`) |

`Service` gains an optional `Reporter` (interface
`WriteReport(sc *Scenario, r RunResult) ReportFiles`, called with the
redacted snapshot; set with `Service.SetReporter`; nil writes nothing), and a
wait group of pending completions used by `Shutdown` (R9). An adapter in
`src/main.go` implements it with `report.NewRun` + `Writer.Write`.

The result types live in `src/scenario` because the service stores them and
cannot import `src/report` (import cycle); `report.Writer` returns them
directly.

## State per scenario run

```text
ExecuteScenario ──► running (report = nil)
                       │ engine ends
                       ▼
            completion callback: status, quality set (under lock)
                       │ lock released
                       ▼
            Reporter.WriteReport ──► report = files (under lock)
```

`GET /api/v1/scenarios/{id}` returns `report` only once set; between the end
of the run and the end of the write (well under 2 s, SC-001) it is absent
while `status` already shows the outcome.
