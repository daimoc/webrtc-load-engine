# Validation Report: Server Load in the Run Report (007)

**Date**: 2026-10-17 | **Spec**: [spec.md](spec.md) | **Guide**: [quickstart.md](quickstart.md)
| **Detailed records**: [tasks.md](tasks.md) (T015, T020, T021, T025)

## Summary

| Criterion | Target | Result | Status |
|-----------|--------|--------|--------|
| SC-001 server figures | bridge average CPU within 10% of `docker stats`, peak memory within 5% | 0.874 against 0.857 threads (2.0%); 3,749,871,616 against 3,750,580,191 B (0.02%) | Met |
| SC-002 engine figures | engine average CPU within 5% of its CPU time / period | 0.399 against 0.398 threads from `/usr/bin/time` (0.3%) | Met |
| SC-003 failures | run unaffected; summary says which container and why | misspelt name 19.20 s, Docker unreachable 19.19 s, against 19.19 s without monitoring; `not_found`, `docker_unreachable` | Met |
| SC-004 cost | ≤ 0.05 core more at 100 bots with three containers | 2.987 against 2.972 cores (+0.015); 0.64 ms CPU per sample, 0.0001 thread at 5 s | Met |
| SC-005 summary readable | server average, peak and memory next to quality, same form for any platform | one `server` and one `load_machine` section in the quality summary; platform-independent test plan section | Met |

## Environment

- Host: Intel i5-13600H laptop, 16 threads, running the bots and Jitsi; the
  engine ran with CPU affinity on 8 of them (`usable_threads` 8).
- Jitsi: local docker-jitsi-meet in `tests/platforms/docker-jitsi-meet`,
  jitsi-videobridge2 2.3-326, bridge heap 8 GB (`VIDEOBRIDGE_MAX_MEMORY`, 006 T058).
- Docker 29.8.1, Engine API 1.56, cgroup v2.
- Engine: branch `rework`, Go 1.24 toolchain (go1.23 code), live test
  `tests/live/jitsi_live_test.go` with `JITSI_SERVER_CONTAINERS`, compiled
  and run under `/usr/bin/time -v`.

## Scenario 1: unit, fake Docker and engine tests

`go vet ./... && go test -race ./...` green. `src/hostload` covers the CPU
and memory arithmetic on recorded cgroup v1/v2 bodies, the Docker client
against a fake Engine API on a Unix socket (missing, stopped, slow,
unreachable, permission denied, remote `DOCKER_HOST`), the accumulator
(missed intervals, restarts, peaks), the sampler (per-container and total
figures, peak of the sum, skipped ticks, overlap between scenarios, one log
per container) and the engine's own figures. `src/execution` runs the engine
with a fake Docker socket; `tests/integration/execution` runs a scenario
through the HTTP API and reads both sections from the quality endpoint.

## Scenario 2: 10 bots, 60 s, against `docker stats` (SC-001, SC-002)

jvb and jicofo named, 5 s interval, `docker stats --no-stream` read in
parallel (12 readings within the summary's period), the test binary under
`/usr/bin/time -v`.

| | Summary | Reference |
|---|---|---|
| Bridge average CPU | 0.874 threads (peak 1.234) | 0.857 (`docker stats` mean) |
| Bridge peak memory | 3,749,871,616 B | 3,750,580,191 B |
| jicofo average CPU | 0.004 | 0.002 |
| Engine average CPU | 0.399 threads (peak 0.425) | 0.398 ((18.56 + 7.08 s) / 64.39 s) |
| Samples / missed | 14 / 0 per container and for the engine | |

`first` CPU covers the first interval, from the baseline sample taken before
the first bot is launched: with no ramp-up the bots join during it (bridge
1.234 threads). `first` memory is the baseline itself.

## Scenario 3: failures (SC-003)

2 bots, 15 s hold, all passed: 19.19 s without monitoring; 19.20 s with
`no-such-container` next to the bridge (bridge `measured`, the other
`unavailable` / `not_found`); 19.19 s with
`DOCKER_HOST=unix:///nonexistent.sock` (`server.available: false`, reason
`docker_unreachable`). One WARN line per unavailable container.

## Scenario 4: 100 bots, 10 min (SC-004)

`audio_video`, cap 10, ramp-up 20 s, bridge restarted before each run, the
same evening:

| | Without monitoring | jvb, jicofo, prosody named |
|---|---|---|
| Engine CPU (`/usr/bin/time`) | 2.972 cores | 2.987 cores |
| Engine max RSS | 841 MB | 780 MB |
| Bridge (summary) | n/a | average 8.711 threads, peak 9.172, 3.70 GB |
| Bridge (`docker stats`) | 8.65 threads | 8.79 threads |
| jicofo / prosody (summary) | n/a | 0.006 / 0.013 threads |
| MOS p50, audio concealed p95 | 4.33, 0.46% | 4.34, 0.44% |

The direct cost, from `BenchmarkTick_Docker` against the three real
containers, is 0.64 ms of CPU per sample: 0.00013 thread at the 5 s default,
0.0006 at the 1 s minimum. The +0.015 core between the two runs is
run-to-run noise.

A first no-monitoring run stopped at 5 min 9 s when one bot's XMPP keepalive
write timed out on the saturated host (the live test stops when a bot
drops); it was rerun and is not used.

## Notes

- The API server listens on a fixed `:8080`, which the local bridge's
  Colibri port (`127.0.0.1:8080`) takes while docker-jitsi-meet runs; the
  API path is validated in process instead (quickstart step 6).
- Reasons for unavailable containers come from the Engine API's answers:
  404 for an unknown name, 200 with a zero `read` time for a stopped
  container; a request slower than `min(interval/2, 2 s)` counts as
  `docker_unreachable` for that interval.
