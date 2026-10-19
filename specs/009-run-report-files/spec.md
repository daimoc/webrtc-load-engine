# Feature Specification: Run Report Files

**Feature Branch**: `009-run-report-files` (work continues on `rework`)

**Created**: 2026-10-18

**Status**: Draft

**Input**: User description: "Run report files: at the end of every scenario run, the tool must write its test report into the repository's reports/ folder, so a run's results survive the engine process and can be compared and kept. Each run produces two files in reports/: a JSON file with the full end-of-run summary (the same quality summary the API returns: media quality of 006 and server/load-machine sections of 007), together with the test plan, the platform and the run's start/end times and outcome; and a human-readable Markdown report with the same content (key figures first: participants, platform, duration, MOS, freezes, server CPU/memory, then the details and the worst bots). One report per run and per platform, no cross-platform comparison report (as 007 FR-007). Applies to every way of running a scenario: the HTTP API, cmd/jitsi-connect, and the live tests. The folder is reports/ at the working directory by default, configurable; it is created if missing; file names identify the run (time, platform, scenario name or ID) and never overwrite an earlier report. Failure to write the report must not fail the scenario but must be logged and visible in the API. The API keeps serving the summary as today. Note: this is feature 009; the mediasoup-demo platform planned as 009 in the roadmap moves to 010."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Every run leaves a report file (Priority: P1)

As a load test operator, I want every scenario run to write its test report as a file in the `reports/` folder when it ends, so that the results of a run are kept after the engine stops, can be compared with other runs (other loads, other platforms, other days), and can be attached to a validation report without copying the summary by hand.

**Why this priority**: Today the end-of-run summary (006, 007) lives only in the engine's memory: it is lost when the engine stops, and the command-line tool and the live tests only print it to the log. Validation reports (005–008) were written by copying figures out of logs. A machine-readable file per run is the foundation every other use builds on.

**Independent Test**: Run 2 bots for 30 seconds through the HTTP API on the local docker-jitsi-meet; after the scenario ends, `reports/` holds one new JSON file for that run, whose summary is identical to what the API returns for the scenario, together with the test plan, platform, start and end times and outcome.

**Acceptance Scenarios**:

1. **Given** a scenario started through the HTTP API, **When** it ends (completed, stopped or failed), **Then** a JSON report file for that run is written in the reports folder, containing the run's identifier and name, platform, test plan, start time, end time, duration, outcome, and the full quality summary (media quality, server and load-machine sections) exactly as the API returns it.
2. **Given** a run started with the command-line tool or a live test, **When** it ends, **Then** the same kind of report is written in the same folder, in the same form.
3. **Given** the reports folder does not exist, **When** the first run ends, **Then** the folder is created and the report written.
4. **Given** two runs with the same scenario name and platform, even ending within the same second, **When** both end, **Then** two distinct reports exist and neither overwrites the other.
5. **Given** a run that ended, **When** the operator asks the API for the scenario, **Then** it tells where its report files were written (or why they were not), and the quality summary is still served as today.

---

### User Story 2 - A readable report next to the data (Priority: P2)

As a load test operator, I want each run to also produce a human-readable report next to the JSON file, which gives the key figures first, so that I can judge a run in a minute and paste it into a validation report or a review without any tool.

**Why this priority**: The JSON file is complete but long (percentiles per measure and media kind, worst bots, per-container load). Operators read results first to answer "was the run good, and how loaded was the server?"; the readable report answers that at a glance. It depends on the content of Story 1.

**Independent Test**: Run 10 bots with audio and video for 1 minute; next to the JSON report, a Markdown report exists whose first section gives participants, platform, duration, outcome, median and worst audio MOS, freezes, received frame rate and resolution, and server and engine CPU and memory, with values equal to those of the JSON file.

**Acceptance Scenarios**:

1. **Given** a run that ended, **When** its reports are written, **Then** a Markdown report with the same base name as the JSON report exists in the same folder.
2. **Given** the Markdown report, **When** an operator reads it, **Then** it starts with a key-figures section (participants, platform, test duration, outcome, audio MOS median and worst, freezes, video frame rate and resolution, server CPU average and peak and memory peak, engine CPU and memory), followed by the details per media kind, the server and load-machine sections, the worst bots with their peer IDs, and the test plan.
3. **Given** a summary in which some measures do not apply or were not measured (receive-only bots, no server containers named, container engine unreachable), **When** the Markdown report is written, **Then** it says so for those measures, as the summary does, rather than showing zeros or empty values.

---

### User Story 3 - Choose where reports go (Priority: P3)

As a load test operator, I want to choose the folder reports are written to, so that I can keep the reports of a campaign together, write them outside the repository, or point a shared folder at them.

**Why this priority**: The default (`reports/` in the working directory) suits most runs from the repository; choosing another folder is a convenience for campaigns and machines where the working directory is not the repository.

**Independent Test**: Start the engine (or the command-line tool, or a live test) with another reports folder configured; the run's two files are written there and nothing is written to `reports/`.

**Acceptance Scenarios**:

1. **Given** no reports folder configured, **When** a run ends, **Then** its reports are written to `reports/` in the working directory.
2. **Given** a reports folder configured for the engine, the command-line tool or the live tests, **When** a run ends, **Then** its reports are written to that folder.

---

### Edge Cases

- The reports folder cannot be created or written (permission denied, read-only file system, disk full): the scenario's outcome is unchanged, the failure is logged once with the scenario identifier and the reason, and the API shows that the report was not written and why.
- One of the two files is written and the other fails: the written one is kept, and the failure is reported for the other one.
- A run fails before any bot connects (platform unreachable, invalid credentials): a report is still written, with the outcome, the error and the measures that do not apply marked as such.
- The engine stops (interrupt) while a scenario runs: the scenario ends as stopped and its report is written before the engine exits, as long as the interruption is a graceful shutdown.
- A scenario is run again (the API allows re-running a completed, stopped or failed scenario): each run produces its own reports; earlier reports of the same scenario are kept.
- The scenario name contains characters that are not safe in file names (slashes, spaces, accents): file names stay valid and readable, the exact name is kept inside the reports.
- Secrets in the test plan (platform credentials, API secrets, JWT secrets and tokens): never written in clear in the reports.
- Very large runs (hundreds of bots): report size stays bounded, since the summary already lists a bounded number of worst bots (006).

## Requirements *(mandatory)*

### Functional Requirements

#### Writing the reports

- **FR-001**: At the end of every scenario run, whatever its outcome (completed, stopped, failed), the engine MUST write a report of that run as files in the reports folder.
- **FR-002**: This MUST hold for every way of running a scenario: the HTTP API, the command-line tool and the live tests, with the same content and form.
- **FR-003**: The reports folder MUST default to `reports/` in the working directory and be configurable for each of these entry points; it MUST be created if missing.
- **FR-004**: Each run MUST produce two files with the same base name: a machine-readable JSON report and a human-readable Markdown report.
- **FR-005**: The base name MUST identify the run by its end time (UTC, to the second, sortable), platform and scenario name (or identifier when unnamed), reduced to characters safe in file names; reports MUST never overwrite an earlier report, including two runs with the same name ending in the same second.
- **FR-006**: The reports MUST be written once the run's summary is final, and be complete when they appear under their final name (a reader never sees a partly written report).

#### Content

- **FR-007**: The JSON report MUST contain: the scenario identifier and name, the platform, the test plan, the run's start time, end time, duration and outcome (with the error message for a failed run), the engine version, and the full end-of-run quality summary (006 media quality, 007 server and load-machine sections) in the same form as the API returns it.
- **FR-008**: The Markdown report MUST contain the same information as the JSON report, starting with a key-figures section: participants, platform, test duration, outcome, audio MOS (median and worst), freezes, received video frame rate and resolution, latency when measured, server CPU (average and peak) and memory (peak) in total over the named containers, and engine CPU and memory with the machine's hardware threads. It then gives the details per media kind, per server container, the worst bots with their peer IDs, and the test plan.
- **FR-009**: Measures that do not apply or were not measured MUST be shown as such in both reports, with the reason the summary gives (006 FR-015, 007 FR-010), never as zeros.
- **FR-010**: Secrets in the test plan (passwords, API secrets, signing secrets, tokens) MUST be masked in both reports; the rest of the test plan is written as given.
- **FR-011**: One report is written per run and per platform, in the same form whatever the platform; there is no cross-run or cross-platform comparison report (007 FR-007).

#### Failures and visibility

- **FR-012**: Failure to write a report MUST NOT change the scenario's outcome or delay its end by more than the time the write attempt takes; it MUST be logged once per run and file with the scenario identifier and the reason.
- **FR-013**: The API MUST show, for an ended scenario, the paths of the report files of its last run, or, for each file that was not written, the reason.
- **FR-014**: The API MUST keep serving the quality summary as today (006 FR-015, FR-016); the report files add to it and do not replace it.
- **FR-015**: The command-line tool and the live tests MUST print the paths of the reports they wrote at the end of the run.

#### Cost

- **FR-016**: Writing the reports MUST happen once per run, after the bots have stopped, and add no per-bot or per-packet work during the run (constitution IV).

### Key Entities

- **Run Report**: per run: run identity (scenario identifier and name, platform), test plan with secrets masked, start and end times, duration, outcome and error, engine version, full quality summary; written as a JSON file and a Markdown file with the same base name.
- **Reports Folder**: where the reports go; `reports/` in the working directory by default, configurable per entry point.
- **Report Status**: per ended scenario, kept with it and shown by the API: paths of the written report files, or the reason a file was not written.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: After any run (through the API, the command-line tool or a live test), its two reports are present in the reports folder within 2 seconds of the scenario's end, for 100% of the runs that end, including stopped and failed ones.
- **SC-002**: The quality summary in the JSON report is identical, value for value, to the summary the API returns for the same run.
- **SC-003**: Every value in the Markdown report's key-figures section equals the corresponding value in the JSON report (same units, rounded for display only).
- **SC-004**: 50 consecutive runs of the same scenario on the same platform produce 50 distinct pairs of reports; none is overwritten.
- **SC-005**: With the reports folder made unwritable, a run ends with the same outcome and within 1 second of the same run with a writable folder, and the API says the report was not written and why.
- **SC-006**: No secret from the test plan appears in any report file (checked by searching the files for the secrets used in the run).
- **SC-007**: An operator can tell from the Markdown report alone, within 1 minute, whether the run's quality was acceptable, how loaded the server and the load machine were, and which bots had the worst quality.
- **SC-008**: The 100-bot run (audio and video, receive cap 10, 10 minutes) uses no more CPU during the run than without report files, and its reports are written within 2 seconds of its end.

## Assumptions

- `reports/` is meant to hold run results; whether its content is committed to the repository is the operator's choice (the folder may be ignored by version control, while selected reports are copied into a feature's validation report).
- Reports are not deleted, rotated or uploaded by the engine; old reports are managed by the operator.
- The engine does not read reports back: listing, searching or serving past reports through the API, and comparing runs, are out of scope; the summary of a scenario kept in memory is still served as today.
- Reports reflect the run's end-of-run summary; the full time series stays in the Prometheus series (006, 007) and is not written to the reports.
- A run is one execution of a scenario; re-running a scenario produces a new run and new reports, and the API shows the reports of the last run.
- An abrupt kill of the engine (not a graceful shutdown) may leave a running scenario without a report; this is accepted.
- The live tests run from their own package folder, so their default is the repository's `reports/` (the folder of the module root), not `reports/` under the test package; a configured folder overrides it.
- The mediasoup-demo platform, planned as feature 009 in the roadmap of 2026-10-16, becomes feature 010.
