# Research: Server Load in the Run Report (007)

**Date**: 2026-10-16 | **Spec**: [spec.md](spec.md) | **Plan**: [plan.md](plan.md)

## R1. Reading container statistics

**Decision**: call the Docker Engine API directly over its Unix socket with
`net/http` (a client whose `DialContext` dials the socket): `GET
/containers/{name}/stats?stream=false&one-shot=true`, one request per named
container per interval, in parallel, each with a timeout of
`min(interval/2, 2 s)`. The socket is `/var/run/docker.sock`, or the path of
`DOCKER_HOST` when it is a `unix://` URL; any other `DOCKER_HOST` scheme is
reported as unavailable ("remote Docker hosts are out of scope").

**Rationale**: the stats endpoint gives the cumulative CPU counter and the
memory figures `docker stats` uses. `one-shot=true` (API ≥ 1.41, Docker
20.10) answers at once instead of waiting about a second for a second
sample, so a request costs a few milliseconds. Checked locally: Docker
29.8.1, API 1.56, `cpu_stats.cpu_usage.total_usage` and
`memory_stats.{usage,stats}` present, `precpu_stats` empty with one-shot (so
deltas are computed between our own samples, R3). The user running the
engine is in the `docker` group, as for `docker stats`.

**Alternatives considered**:
- Docker Go SDK (`github.com/docker/docker/client`): a large module tree for
  two JSON fields; rejected (Incremental Complexity).
- Streaming stats (`stream=true`, one long request per container): fewer
  requests but one goroutine and connection per container for the whole run,
  a fixed 1 s cadence, and reconnection handling on restarts; rejected.
- Running `docker stats --no-stream`: what was done by hand in 006 T058;
  forks a process per sample and parses text; rejected.
- Reading cgroup files under `/sys/fs/cgroup` directly: needs the container
  ID → cgroup path mapping, which differs between cgroup v1/v2 and
  drivers; rejected.

## R2. The engine's own CPU and memory

**Decision**: CPU from `syscall.Getrusage(RUSAGE_SELF)` (user + system time)
at each sample; resident memory from `/proc/self/statm` (second field ×
page size). The machine's hardware threads from `/sys/devices/system/cpu/online`,
and the threads the engine may use from `runtime.NumCPU()` (its CPU affinity).

**Rationale**: `getrusage` is what `/usr/bin/time` reports, so SC-002
compares like with like. Peak RSS from `getrusage` (`Maxrss`) covers the
whole process life, not the run, hence `/proc/self/statm` per sample. Both
thread counts are reported because they differed in 006 T058 (16 on the
machine, 8 for the test process) and explained its figures.

**Alternatives considered**: the Prometheus process collector
(`process_cpu_seconds_total`): already a series, but not per scenario and
not in the summary; it stays registered as it is.

## R3. CPU in threads

**Decision**: for two samples, CPU (threads) = Δ CPU time (ns) / Δ wall time
(ns), the wall time taken when the answer arrives. The average over the
run = (last − first counter) / (last − first time); the peak is the highest
per-interval value; the "first sample" CPU is that of the first interval.
A counter that goes down (container restarted) starts a new segment: the
interval is skipped and counted as missed, and the average sums the
segments.

**Rationale**: `docker stats` computes `Δcontainer / Δsystem × online CPUs
× 100`, the same quantity in percent when the host clock and the system
counter advance together; dividing by wall time avoids reading the system
counter and keeps the unit independent of the host (FR-003). An average
from the counters is exact; a mean of per-interval rates would weigh
missed intervals wrong.

## R4. Memory as `docker stats` shows it

**Decision**: `usage − inactive_file` from `memory_stats.stats` on cgroup v2,
`usage − total_inactive_file` on cgroup v1, `usage` if neither is present
(the Docker CLI's `calculateMemUsageUnixNoCache`). Peak = highest sample.

**Rationale**: matches the figure operators read today (SC-001 checks it
within 5%).

## R5. When and how often to sample

**Decision**: one sampler goroutine per scenario, started by the engine's
`Start` before the first bot is launched, ticking every `interval_seconds`
(default 5); a sample at start (the baseline the first interval is measured
from), one per tick, one at the end of the run (when the scenario's context
is cancelled, before the bots disconnect, as the media collector's last
collection). Each tick samples every container and the process at once; a
tick that is still running when the next one is due is skipped and counted
as missed.

**Rationale**: the period then matches the media summary's. Sampling the
process at the same instants makes the two sections cover the same period
(US2 scenario 1).

## R6. Prometheus series

**Decision**: four gauges: `webrtc_server_cpu_threads` and
`webrtc_server_memory_bytes` labelled `scenario_id`, `container`;
`webrtc_engine_cpu_threads` and `webrtc_engine_memory_bytes` labelled
`scenario_id`. CPU is the last interval's value. Series of a scenario are
deleted when its run ends ([contracts/metrics.md](contracts/metrics.md)).

**Rationale**: gauges are what a scraper turns into the full time series;
the container label is bounded by the test plan (≤ 16). Deleting at the end
avoids flat lines for finished runs (as 006 resets its gauges).

## R7. Where the figures are reported

**Decision**: two new sections of the 006 `QualitySummary`, `server` and
`load_machine`, served by the existing `GET /api/v1/scenarios/{id}/quality`
([contracts/server-load-summary.yaml](contracts/server-load-summary.yaml)).
`server.available` is false with a reason when no container was named, or
none could be measured. Values are rounded to 3 decimals (CPU) and kept as
integers (bytes).

**Rationale**: the operator wants one report per run (spec FR-006); a second
endpoint would split it.

## R8. Test plan section

**Decision**: an optional top-level `server_monitoring` object in the test
plan: `containers` (1–16 non-empty, distinct names) and `interval_seconds`
(1–60, default 5). Validated when the scenario is created; Docker is not
contacted at creation (FR-010). The engine's own figures are always
collected, with or without this section, at the same interval (default 5 s).

**Rationale**: platform-independent, so 008/009 reuse it unchanged
(constitution III); unchanged test plans keep working.

## R9. Failures and overlap

**Decision**: per container, a status `measured`, `partial` (some samples
missed) or `unavailable` with a reason from a fixed set: `docker_unreachable`,
`permission_denied`, `not_found`, `not_running`, `no_samples`,
`remote_docker_host`. Logged once per container per run at WARN. A
process-wide registry records which scenarios are sampling which container;
when a run's period overlaps another scenario sampling the same container,
the container entry gets `shared_with` (the other scenario IDs) (FR-012).

**Rationale**: the operator needs to know why a figure is missing, not only
that it is; the registry is the only way to see overlap within one engine.

## R10. Validation

**Decision**: SC-001 by a live test that samples `docker stats --no-stream`
in parallel with the run and compares; SC-002 by running the compiled live
test under `/usr/bin/time -v` and comparing with `load_machine`; SC-003 by
the fake Docker API (unreachable socket, misspelt name) plus one live run
with a misspelt name; SC-004 by two same-day 100-bot runs with and without
three named containers, bridge restarted before each (006 T058 showed that
only same-day runs compare).
