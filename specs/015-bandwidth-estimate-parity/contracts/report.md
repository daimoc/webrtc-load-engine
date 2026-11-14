# Contract: additions to the run report files (015)

Adds to the quality summary
([006](../../006-media-quality/contracts/quality-summary.yaml)) and the run
report files ([009](../../009-run-report-files/contracts/run-report.schema.yaml),
[Markdown](../../009-run-report-files/contracts/report-markdown.md)), after
the codec fields of [014](../../014-video-codec-comparison/contracts/report-codec.md).
Existing fields keep their names and meaning. `format_version` becomes 3.

## JSON: `quality.reception_reports`

Present in every run. How completely and regularly the bots reported their
reception to the server.

```json
"reception_reports": {
  "available": true,
  "bots": 10,
  "acknowledged_share": {"p50": 1.0, "p95": 1.0, "p99": 0.97},
  "late_share": {"p50": 0.0, "p95": 0.004, "p99": 0.03},
  "stamp_lag_max_ms": {"p50": 0.4, "p95": 2.1, "p99": 38.0},
  "limits": {"acknowledged_share_min": 0.99, "late_share_max": 0.01, "late_interval_ms": 200},
  "flagged": ["bot-7"],
  "per_bot": [
    {
      "peer_id": "bot-7",
      "packets_arrived": 61250,
      "packets_acknowledged": 59410,
      "acknowledged_share": 0.97,
      "feedbacks_sent": 598,
      "interval_ms": {"mean": 100.3, "max": 412.0},
      "late_share": 0.03,
      "stamp_lag_ms": {"mean": 0.6, "max": 38.0}
    }
  ]
}
```

| Field | Meaning |
|---|---|
| `available` | false when the measure could not be taken; then only `reason` is given |
| `reason` | `no_video`, `not_negotiated` (the server's offer carries no transport-wide sequence number) or `not_counted` |
| `bots` | bots measured |
| `acknowledged_share`, `late_share`, `stamp_lag_max_ms` | percentiles over bots |
| `limits` | the limits a bot is flagged against |
| `flagged` | bots under `acknowledged_share_min` or over `late_share_max` |
| `per_bot` | one entry per bot measured, in launch order |

No figure is ever zero for "not measured": the block says `available:
false` with its reason (FR-007).

## JSON: `browser`

Top level, beside `quality`. Present only when the test plan has a `browser`
section.

```json
"browser": {
  "status": "measured",
  "url": "https://localhost:8443/load015#config.prejoinConfig.enabled=false",
  "window": "1280x720",
  "browser_version": "Chrome/141.0.7390.54",
  "period": {"start": "2026-11-13T10:00:02Z", "end": "2026-11-13T10:01:07Z"},
  "video_streams": 4,
  "height_share": {"180": 0.06, "720": 0.94},
  "expected_height": 720,
  "expected_height_share": 0.94,
  "seconds_to_expected_height": 3.0,
  "received_bitrate_kbps": {"average": 5120, "peak": 6300},
  "freezes_total": 0,
  "freeze_seconds": 0.0,
  "freeze_source": "browser",
  "cpu_threads_average": 0.82,
  "samples": [
    {"t": 1.0, "heights": [180, 180], "kbps": 310},
    {"t": 2.0, "heights": [180, 180, 360, 360], "kbps": 1480}
  ]
}
```

| Field | Meaning |
|---|---|
| `status` | `measured`, `partial` or `unavailable` |
| `reason` | when not `measured`: `browser_not_found`, `browser_start_failed`, `page_load_failed`, `no_video_received`, `browser_exited` |
| `height_share` | height → share of decoded stream-seconds |
| `expected_height_share` | share at the plan's `expected_height`; without one, the field is absent and `expected_height_reason` is `not_stated` |
| `seconds_to_expected_height` | from the page's load to the first second at the expected height; absent if never reached |
| `freeze_source` | always `browser`: the browser's own freeze rule, not the bots' (006) |
| `cpu_threads_average` | CPU of the browser's processes; not in `load_machine.engine` |
| `samples` | one per second: offset from the page's load, heights decoding, video kbit/s |

With `unavailable`, only `status`, `reason`, `url` and `window` are given.
The browser is in no bot count, percentile or ranking (FR-017).

## Server settings

No new JSON object: the report's `test_plan.server_settings` is the
statement. The Markdown file words the three cases.

## Markdown

Key figures, after the codec rows:

```markdown
| Server settings | JVB_CC_TRUST_BWE=false (stated by the deployment) |
| Reception reports | acknowledged 100.00 % (lowest bot 97.00 %) · 1 bot flagged |
| Browser | 720 lines 94.00 % of the time · reached after 3.0 s · 0 freezes |
```

- Server settings, other cases: `none changed by the deployment`;
  `not known to the run`.
- Reception reports, not available: `not measured: <reason>`.
- Browser row only when the plan asks for one; not measured:
  `not measured: <reason>`.

Video section, when a bot is flagged, before the height figures:

```markdown
> Warning: 1 bot reported its reception incompletely or late (see Reception
> reports). Layered results of this run can reflect the bots.
```

New section "Reception reports", after "Video": the percentiles, the limits,
and a table of the flagged bots (at most 10, then "and N more"; the JSON
file has all).

New section "Browser", after "Load machine", only when asked for: address,
window, version, streams, height shares, share and time at the expected
height, bitrate, freezes ("as counted by the browser"), CPU.

"Load machine" gains, when a browser ran: `A browser ran on this machine
during the run: 0.82 threads on average, not counted in the engine's
figure.`

## API

`GET /api/v1/scenarios/{id}/quality` gains `reception_reports`. The
browser's result is in the report files only.
