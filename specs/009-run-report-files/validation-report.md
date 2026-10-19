# Validation Report: Run Report Files

**Date**: 2026-10-18. **Setup**: one laptop, 16 hardware threads (8 usable by
the engine's session); Jitsi from `tests/platforms/docker-jitsi-meet`
(`unstable` images, 8 GB bridge heap); LiveKit `v1.13.10` from
`tests/platforms/livekit`. Only one server ran during the 100-bot run. Go
1.27.2 local toolchain, `go 1.23.0` module. `go vet ./...`,
`go vet -tags live ./tests/live/` and `go test -race -count=1 ./...` pass.

## Results per success criterion

| Criterion | Result | Evidence |
|---|---|---|
| SC-001: both reports within 2 s of the end, for every run that ends (stopped and failed included) | **Met** | CLI completed (T017), CLI stopped by Ctrl+C (written 2 ms after the disconnect), Jitsi live test (stopped), 100-bot LiveKit (files 23 ms after `ended_at`, T027). Failed runs: `TestEngine_RunResultFailedStart` and `TestMarkdownFailedRun` |
| SC-002: JSON summary = API summary | **Met** | `TestRunReportInScenarioAnswer` (file's `quality` deep-equals `GET …/quality`); `TestNewRunJSON` (byte-equal to `json.Marshal` of the summary) |
| SC-003: Markdown key figures = JSON values | **Met** | `TestMarkdownKeyFigures`; live 10-bot run checked figure by figure (T021) |
| SC-004: 50 runs, 50 distinct pairs | **Met** | `TestWritePairsNeverOverwrite` and `TestWriteNeverOverwrites` (same end time, name and run ID; `-2`…`-51`; an existing file kept) |
| SC-005: unwritable folder, same outcome and time, API says why | **Met** | CLI: `completed`, one WARN per file, 10.34 s vs 10.01 s (T024); API: `TestRunReportNotWritten`; service: `TestService_ReportFailureKeepsStatus` |
| SC-006: no secret in any report | **Met (unit)** | `TestWriteNoSecrets` (Jitsi `jwt_app_secret`, `jwt`; LiveKit `api_secret`, `token`; both files). Not run against a token-auth Jitsi: none runs locally |
| SC-007: run judged from the Markdown in 1 minute | **Met** | Key-figures table first (T021 excerpt below) |
| SC-008: 100-bot run, no more CPU during the run, reports within 2 s | **Partly met** | Reports in 23 ms. CPU: engine 3.17 cores, server 4.94 threads on average, against 1.43 and 2.28 in 008's run on 2026-10-17. See below |

## 10 bots on Jitsi, key figures (T021)

`reports/20261018T212553Z-jitsi-jitsi-connect-0e0fd6ab.md` (4.3 kB; JSON 7.8 kB):

| | |
|---|---|
| Participants | 10 (bots measured: 10) |
| Test duration | 60 s (ramp-up 0 s); run 60.010 s |
| Audio MOS | median 4.37, worst bot 4.37 |
| Video freezes | 27 in total; freeze time p95 44.966 s |
| Video frame rate | 29.9 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Latency p50 (audio / video) | 1.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.97 threads, peak 1.42 threads |
| Server memory (total) | peak 813.0 MiB |
| Engine CPU / memory | avg 0.36 threads, peak 0.39 threads · peak 92.0 MiB · 16 hardware threads |

The freezes come from the CLI run (no settle wait before measuring, 9 videos
per bot), not from the report.

## 100 bots on LiveKit (T027)

100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min hold,
`/usr/bin/time -v`, `LIVEKIT_SERVER_CONTAINERS=livekit-server`, Jitsi
stopped. PASS: 100 bots established in 20.5 s, no freezes, MOS p50 4.365.

- Reports: `reports/20261018T213912Z-livekit-live-livekit-c7d02ec7.{json,md}`
  (6.7 kB, 3.9 kB), modified 23 ms after `ended_at`.
- Engine: `/usr/bin/time` (1602.6 + 383.8) s / 625.8 s = **3.17 cores**;
  `load_machine` average 3.175 threads; max RSS 1.18 GiB.
- LiveKit server: average **4.94 threads**, peak 5.49, memory peak 1.10 GiB.
- 008 (2026-10-17, same plan): engine 1.43 cores, server 2.28 / 5.41
  threads.

**Not confirmed: "no more CPU than without report files".** Both the engine
and the server, which 009 does not touch, used about twice the CPU of 008's
run, with the same quality. Feature 009 adds no work during a run: the
engine only records two timestamps and the error, and the report is built
after the bots have stopped (`TestService_ShutdownWaitsForReports`, live
logs). The doubling therefore points at the machine (the load average stayed
at 16–17 during the run on 16 threads; the LiveKit container had been up 5
hours and a 10-bot Jitsi load had just stopped). It is not proven: a same-day
A/B run against a build without 009 would settle it. Until then, SC-008's
CPU part is open.

## Deviations from the plan

- `report.Writer` implements `scenario.Reporter` itself; no adapter in
  `src/main.go` (T014).
- Latency in the Markdown is in milliseconds, not seconds (T018/T019).
- The live tests keep their scenario names (`live`, `live-livekit`).
- `full.json` test data is a 2-bot run (T017), not a 10-bot run.
