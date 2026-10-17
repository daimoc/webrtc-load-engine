# Data Model: Server Load in the Run Report (007)

**Spec**: [spec.md](spec.md) | **Research**: [research.md](research.md)

## Server Monitoring (test plan, `server_monitoring`)

| Field | Type | Rules |
|-------|------|-------|
| `containers` | list of strings | 1–16 entries, non-empty, distinct; names as `docker ps` lists them |
| `interval_seconds` | integer | 1–60; default 5 |

Optional. Absent: no server section measured (`server.available = false`,
reason `not_requested`); the engine's own figures are still sampled every
5 s. Validated in `NewScenario`; never contacts Docker (FR-010).

## Load Sample (internal, not kept)

Taken per target (each container, and the engine's process) at each tick:

| Field | Meaning |
|-------|---------|
| `at` | wall time when the answer arrived |
| `cpu_ns` | cumulative CPU time (container: `cpu_stats.cpu_usage.total_usage`; process: user + system from `getrusage`) |
| `memory_bytes` | container: usage − inactive file cache (R4); process: resident set from `/proc/self/statm` |
| `err` | why the sample failed (one of the reasons below), or none |

## Target Accumulator (internal, one per target per run)

Folds samples as they come (R3):

| Field | Meaning |
|-------|---------|
| `first`, `last` | first and last successful samples of the current segment |
| `cpu_ns_total`, `wall_ns_total` | sums over closed segments (a counter that goes down closes a segment) |
| `peak_cpu_threads` | highest per-interval CPU |
| `first_interval_cpu_threads` | CPU of the first interval |
| `first_memory_bytes`, `peak_memory_bytes` | memory of the first sample, highest memory |
| `samples`, `missed` | successful samples; ticks without a sample (error, timeout, skipped, restart) |
| `reason` | last failure reason, kept for the summary |

Average CPU = (`cpu_ns_total` + current segment's Δ) / (`wall_ns_total` + current segment's Δ).

## Server Load (summary section `server`)

| Field | Type | Meaning |
|-------|------|---------|
| `available` | bool | at least one container measured |
| `reason` | string | when not available: `not_requested`, or the reason shared by all containers |
| `interval_seconds` | int | sampling interval |
| `period` | `{start, end}` | first and last sample times of the run |
| `containers` | list of Container Load | in test plan order |
| `total` | Load Figures | over measured containers only: sums of averages and of per-sample CPU/memory (peak of the sum, not sum of peaks); `containers_measured` |

**Container Load**: `name`, `status` (`measured`, `partial`, `unavailable`),
`reason` (when not `measured`), `shared_with` (scenario IDs, FR-012) and the
Load Figures.

**Load Figures**: `cpu_threads` `{average, peak, first}`, `memory_bytes`
`{peak, first}`, `samples`, `missed`.

## Load Machine (summary section `load_machine`)

| Field | Type | Meaning |
|-------|------|---------|
| `period` | `{start, end}` | same instants as `server.period` |
| `engine` | Load Figures | the engine's process |
| `hardware_threads` | int | threads of the machine (`/sys/devices/system/cpu/online`) |
| `usable_threads` | int | threads the engine may run on (CPU affinity) |

## Reasons

`not_requested`, `docker_unreachable`, `permission_denied`, `remote_docker_host`,
`not_found`, `not_running`, `no_samples` (run shorter than one interval and
the end sample failed).

## Relationships

`Scenario` 1 — 0..1 `Server Monitoring` (in its test plan). Each run's
`QualitySummary` (006) 1 — 1 `server`, 1 — 1 `load_machine`; replaced by the
next run, kept with the scenario (FR-009).
