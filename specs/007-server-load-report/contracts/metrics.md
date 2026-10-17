# Metrics: Server Load (007)

Next to the media metrics of 004 and the quality metrics of 006. All are
gauges, updated at each sample and deleted when the scenario's run ends.

| Name | Labels | Meaning |
|------|--------|---------|
| `webrtc_server_cpu_threads` | `scenario_id`, `container` | CPU of the container over the last interval, in threads (1.0 = one thread) |
| `webrtc_server_memory_bytes` | `scenario_id`, `container` | Memory of the container as `docker stats` shows it |
| `webrtc_engine_cpu_threads` | `scenario_id` | CPU of the engine's process over the last interval, in threads |
| `webrtc_engine_memory_bytes` | `scenario_id` | Resident memory of the engine's process |

`container` values come from the test plan (at most 16 per scenario). A
container that cannot be sampled has no series until it can. The engine
series are per scenario; with several scenarios running at once they carry
the same process-wide values.
