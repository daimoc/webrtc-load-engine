# Implementation Plan: Bandwidth Estimate Parity with a Browser

**Branch**: `015-bandwidth-estimate-parity` (work continues on `rework`) | **Date**: 2026-11-13 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/015-bandwidth-estimate-parity/spec.md`

## Summary

Two results of 014 are open because the servers' bandwidth estimate for the
bots stays low on a loss-free path: mediasoup holds bots under 720 lines, and
the local Jitsi suspends streams unless its estimate is turned off. The
feature finds out whether the bots cause it, corrects them if they do, and
writes down what is the server's.

**A browser beside the bots** (US1). A test plan can ask for one headless
Chrome: the engine starts it, opens the address the plan gives, and reads
what the browser measures itself, once per second, through the DevTools
protocol (R1, R2). It works on any page, because the engine wraps the
page's `RTCPeerConnection` and reads `getStats`, and does nothing in the
page. A browser that fails never stops the run (R3). The server's own
estimates, per receiver and per second, are read by a validation script and
kept with the validation report, not in the run report (R7). The verdict
per server follows fixed rules (R11).

**What the bots tell the server** (US2). Each bot counts the packets that
reach its socket, the packets its feedback acknowledged, when its feedback
left, and how late its arrival times were stamped (R6). The report names
the bots beyond stated limits and warns on the layered results.

**The correction** (US3) is built only for the cause found. The research
lists six bot-side causes and what each measure shows (R8), and the
correction for each (R13). The most likely, late arrival stamps and packets
never acknowledged, share one change: recording at the socket.

**Server settings** (US4). The remote deployment declares the settings it
changes; they reach the test plan through the settings file, or the
operator states them; the report gives them, or says they are not known
(R9).

**Several facts are not yet checked** (R12): headless Chrome on the two
pages, the pages' join-by-itself addresses, where each server gives its
estimate, and which feedback each server uses. The first live task checks
them; each has a fallback.

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: `nhooyr.io/websocket` v1.8.17 (DevTools protocol, already used), `os/exec`, `encoding/json`; `pion/interceptor` v0.1.29 `pkg/twcc` and `pion/rtcp` (feedback, already used); `pion/webrtc/v3` v3.3.6. No new module. Run-time, optional: Chrome or Chromium on the load machine, only for a test plan with `browser`. Validation only: Python 3 standard library in `scripts/read-estimates.py` and `scripts/estimate-verdict.py`

**Storage**: report files as 009 (`FormatVersion` 3); estimate files (JSONL) beside the validation report

**Testing**: `go test -race ./...`: feedback counters against known packet sequences and a virtual network; `src/browser` against a fake DevTools server (no Chrome in unit tests); report golden files; test plan validation; `src/remote` overlay against the declared settings. Live (`-tags live`): each platform's test with `<PLATFORM>_BROWSER_URL`, local servers, then the 013 remote machine

**Target Platform**: Linux

**Project Type**: Go engine and CLI

**Performance Goals**: engine CPU per bot at 100 bots within 5% of before (FR-015); bots asking for 720 on mediasoup at it 95% of the time (SC-004); 0 freezes on the local Jitsi with default settings (SC-005)

**Constraints**: a run without `browser` starts no process and needs no browser (FR-018); a browser failure never fails a run (FR-019); servers' code unchanged, configuration only toward defaults; LiveKit, Janus and unlayered runs unchanged (FR-014); no per-packet allocation in the feedback counters

**Scale/Scope**: one browser per run; 2 servers investigated, 4 checked for no change; about 16 comparison runs at most (5 mediasoup, 5 Jitsi VP8, 5 Jitsi AV1, 1 remote)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status |
|---|---|
| I. Infrastructure-first | Pass, justified below. The principle says browser clients "may be added later". One reference browser per run is added; it generates no load, is in no bot figure, and is off unless the plan asks. |
| II. Media engine independence | Pass. Nothing changes in `src/media`. The feedback counters live in `src/execution`, the browser in its own package with no Pion import. |
| III. Pluggable signaling | Pass. `browser` and `server_settings` are platform-independent; no adapter reads them, and the browser needs no adapter because the page joins by itself. |
| IV. CPU efficiency | Pass, with a check. The browser decodes video, which the bots avoid: it is one process tree, optional, measured and reported apart from the engine (R4). The counters add atomic adds per packet; FR-015 measures 100 bots before and after. |
| V. Deterministic lifecycle | Pass. The browser has its own small state list outside the bots' lifecycle; no bot state is added. |
| Go, Pion, single static binary | Pass. No new module. Chrome is an optional external program found at run time, as ffmpeg is for `media.source_file`. |
| Incremental complexity | Pass. US3's corrections are built only for the cause found (R13). |

Re-checked after Phase 1 design: unchanged. The design adds one package,
no module, nothing platform-specific outside the adapters, and no browser
dependency for runs that do not ask for one.

## Project Structure

### Documentation (this feature)

```text
specs/015-bandwidth-estimate-parity/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── test-plan.md
│   └── report.md
├── estimates/               # written during validation (R7)
└── tasks.md                 # /speckit-tasks
```

### Source Code (repository root)

```text
src/browser/                     # new: no Pion, no platform
├── browser.go                   # start, stop, reasons, result
├── devtools.go                  # DevTools protocol over nhooyr websocket
├── stats.go                     # injected script, per-second samples, shares
└── *_test.go                    # fake DevTools server

src/execution/
├── twcc.go                      # counters: arrived, acknowledged, feedback timing, stamp lag
├── consent.go                   # consentNet counts RTP arrivals at the socket (and, per R13, records them)
├── pion.go                      # gatedBuffer: stamp lag; after R13 no longer records
├── engine.go                    # starts and stops the browser beside the run; result in RunResult
└── quality_collector.go         # reception_reports in the summary

src/scenario/
├── scenario.go                  # browser and server_settings sections, validation, url redaction
├── quality.go                   # ReceptionReports
└── result.go                    # RunResult.Browser

src/report/                      # FormatVersion 3; browser, reception reports, server settings; golden files

src/remote/
├── servers.go                   # Server.Settings per server
├── settings.go                  # <PLATFORM>_SERVER_SETTINGS in the settings file
└── overlay_test.go              # each overlay matches its declared settings

cmd/jitsi-connect/               # --browser-url, --browser-height, --browser-window, --browser-insecure-tls, --server-settings
tests/live/                      # <PLATFORM>_BROWSER_*, <PLATFORM>_SERVER_SETTINGS
scripts/read-estimates.py        # new, validation: Jitsi per-receiver estimates to JSONL
scripts/estimate-verdict.py      # new, validation: per-receiver table of a comparison run

tests/platforms/docker-jitsi-meet/
├── docker-compose.remote.yml    # JVB_CC_TRUST_BWE removed if FR-010 holds remotely (FR-011)
└── README.md                    # estimate setting; debug interface used for validation

README.md                        # browser in a test plan; server limits and run conditions (FR-013)
```

**Structure Decision**: the existing single Go project. One new package,
`src/browser`, because the browser is neither media nor a platform; the
engine owns its start and stop. Feedback stays in `src/execution`. The
estimate reader is a script outside the engine, as validation tooling.

## Delivery order

1. **Facts** (R12), then **the browser in a run** (FR-016 to FR-019) and
   the estimate reader (R7). Checkable alone: quickstart step 2.
2. **US2** measures (R6). Small, and built before the comparison runs so
   that every one of them carries the bots' figures.
3. **US1** comparison runs and the verdict per server (R11). This is the
   gate: it decides what step 4 contains.
4. **US3**: the correction for the cause found (R13), the comparison
   again, then the layered runs of 014 and the no-change checks. With a
   verdict of "the server" for both, this step is documentation only.
5. **US4**: server settings in the report (R9), the documentation of what
   remains the server's (FR-013), and the remote Jitsi overlay (FR-011).

US4's report part depends on nothing above and can be done at any point.

## Complexity Tracking

| Deviation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| A browser started by the engine (constitution I: "out of scope for the initial architecture") | The open question of 014 is whether the bots differ from a browser; the answer needs a browser in the same room, and the clarification asks for it in the test plan so that a comparison repeats without the operator | A browser opened by hand: no results in the report, and up to 16 runs to attend. A separate command: rejected in clarification |
| Hand-written DevTools client | Starting Chrome, one script, one evaluation per second | A driver module, against the no-new-module rule, for about 300 lines |
| The browser's CPU on its own line, not inside the load machine's figure (spec edge case wording) | The engine's figure is compared across features (FR-015, 014) and must stay the engine alone | Adding it in would make every run with a browser incomparable with the others |
| Stamp lag, a measure the spec does not name | It is the only view of the most likely bot-side cause (R8, item 1) without a packet capture | Timing of the feedback alone does not show when arrival times are wrong |
