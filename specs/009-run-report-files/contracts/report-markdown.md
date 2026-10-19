# Markdown report: reports/<base>.md (feature 009, FR-008, research R5)

Same base name as the JSON file, rendered from the same `report.Run`.
Sections in this order; a section whose data is missing says why in one line
(e.g. `Not measured: no_video_received.`), never shows zeros (FR-009).

Units: CPU in threads with 2 decimals (`1.05 threads`), memory in MiB with
1 decimal, ratios as percentages with 2 decimals, seconds with 3 decimals, latency in
milliseconds with 1 decimal (latencies between bots are often under 1 ms),
frame rate with 1 decimal, MOS with 2 decimals. Percentiles are written
`p50 / p95 / p99`.

````markdown
# Load test report: <scenario name>

<platform> · run <run_id> · <started_at> → <ended_at> (UTC) · <outcome status>

> Error: <outcome error>                      ← only for failed runs

## Key figures

| | |
|---|---|
| Platform | jitsi |
| Participants | 100 (bots measured: 100) |
| Test duration | 600 s (ramp-up 20 s); run 623.412 s |
| Outcome | completed |
| Audio MOS | median 4.38, worst bot 4.11 |
| Audio concealed | p95 0.12 % |
| Video freezes | 3 in total; freeze time p95 0.000 s |
| Video frame rate | 29.8 / 29.1 / 27.5 fps (p50 / p95 / p99) |
| Video resolution | 720: 97.40 %, 360: 2.60 % |
| Latency p50 (audio / video) | 21.0 ms / 34.0 ms · or "not measured: <reason>" per kind |
| Server CPU (total) | avg 1.84 threads, peak 2.61 threads |
| Server memory (total) | peak 1843.2 MiB |
| Engine CPU / memory | avg 1.02 threads, peak 1.40 threads · peak 412.5 MiB · 16 hardware threads |

## Audio
Bots, MOS p50/p95/p99, concealed ratio p50/p95/p99, latency.

## Video
Bots, frame rate, freeze seconds, freezes / resolution switches / keyframe
requests sent and received totals, height share table, requested height
share, latency.

## Server
One row per container: status (and reason), samples / missed, first CPU and
memory, average and peak CPU, peak memory; a total row; the overlap note of
007 FR-012 when present. "Server load not requested." when the test plan
names no containers.

## Load machine
Engine average and peak CPU, peak memory, samples, hardware threads, period.

## Worst bots
One table per ranking (lowest MOS, most freeze seconds, highest latency p95,
lowest requested-height share): peer ID and value; "None." for an empty
ranking.

## Test plan
```json
<redacted test plan, indented>
```

Engine <version> (<go_version>)
````

The key-figures row "Worst bot MOS" is the first entry of `worst.lowest_mos`
(not a percentile), so it names a real bot's value.
