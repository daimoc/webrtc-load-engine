# Data Model: Bandwidth Estimate Parity with a Browser (015)

## Browser request (test plan)

`browser` ([contract](contracts/test-plan.md)); absent: no browser.

| Field | Type | Meaning |
|---|---|---|
| `url` | string, required | address the browser opens; the page must join the room by itself |
| `expected_height` | int, optional | video height the browser's share is measured against |
| `window` | string, default `1280x720` | window size, `<width>x<height>` |
| `insecure_tls` | bool, default false | accept a self-signed certificate |
| `executable` | string, optional | path of the browser; else found in `PATH` |

Validation at creation: `url` is `http` or `https` with a host;
`expected_height` from 1 to 2160; `window` from `320x240` to `3840x2160`.
The executable is not looked for at creation: a missing browser is a reason
in the run's report (FR-019), never a refusal.

## Browser result (run)

One per run whose plan has `browser` ([contract](contracts/report.md)).

| Field | Meaning |
|---|---|
| status | `measured`, `partial` (the browser ended before the run) or `unavailable` |
| reason | when not `measured`: `browser_not_found`, `browser_start_failed`, `page_load_failed`, `no_video_received`, `browser_exited` |
| url | as given, token values hidden |
| window, browser_version | the window size used; the version Chrome reports |
| period | first and last sample |
| video_streams | most video streams decoding at once |
| height_share | height → share of decoded stream-seconds |
| expected_height, expected_height_share | the plan's height and the share at it; left out with `expected_height_reason: not_stated` |
| seconds_to_expected_height | from page load to the first second at the expected height; absent if never |
| received_bitrate_kbps | average and peak over one second, video |
| freezes_total, freeze_seconds | Chrome's own count and duration |
| cpu_threads_average | the browser's processes over the run |
| samples | per second: time offset, the heights decoding, received kbit/s |

Rules:

- The browser is not a bot: it is in no bot count, percentile or "worst
  bots" list (FR-017).
- A stream counts in a second when it decoded at least one frame in it.
- `partial` keeps the measures up to the browser's end.

State: `starting` → `loading` → `sampling` → `stopped`; any failure goes to
`unavailable` or `partial` with its reason. None of these states is a state
of the run or of a bot.

## Reception report quality (per bot, per run)

In `quality.reception_reports` ([contract](contracts/report.md)).

| Field | Meaning |
|---|---|
| packets_arrived | RTP packets with a transport sequence number read from the bot's socket |
| packets_acknowledged | packets reported as received in feedback that was sent |
| acknowledged_share | acknowledged / arrived |
| feedbacks_sent | feedback packets written |
| interval_ms | mean and longest time between two feedbacks |
| late_share | share of intervals over 200 ms |
| stamp_lag_ms | mean and longest time between the socket read and the recording |

Run level:

- `available` and `reason` (`no_video`, `not_negotiated`, `not_counted`);
- percentiles over bots of `acknowledged_share`, `late_share` and the
  longest `stamp_lag_ms`;
- `flagged`: the bots with `acknowledged_share < 0.99` or
  `late_share > 0.01`, each with its figures;
- when `flagged` is not empty, the layered results carry a warning in both
  files (FR-006).

Counting rules:

- Packets that arrive after the bot's last feedback is sent are not in
  `packets_arrived` (they could not be acknowledged).
- The publish session (008) receives no media and has no entry.

## Server settings (test plan and report)

`server_settings` ([contract](contracts/test-plan.md)); absent: not known.

| Field | Type | Meaning |
|---|---|---|
| `source` | `operator` (default) or `deployment` | who stated them |
| `values` | map string → string, up to 16 | setting name → value, as the server names them |

Three cases in the report (FR-012):

| Test plan | Report says |
|---|---|
| no section | server settings not known to the run |
| `deployment`, no value | none changed by the deployment |
| any source, values | each name and value, with the source |

`src/remote` side: each server declares the settings its remote overlay
changes (`Server.Settings`), and `deploy` writes
`<PLATFORM>_SERVER_SETTINGS` to the settings file of 013.

## Receiver estimate (validation only)

One line per receiver per second in
`specs/015-bandwidth-estimate-parity/estimates/<server>-<run>.jsonl`,
written by `scripts/read-estimates.py`; not in the run report.

| Field | Meaning |
|---|---|
| `t` | UTC time of the reading |
| `server` | `mediasoup` or `jitsi` |
| `receiver` | the bot's endpoint id, or `browser` |
| `endpoint` | the server's id of the receiver |
| `estimate_bps` | the server's estimate toward that receiver |
| `ideal_bps` | what the server would send the receiver with no limit: what its streams need |
| `target_bps` | what the server allocates within the estimate |
| `suspended` | whether the server holds back some of the receiver's streams |
| `method` | how it was read (research R7): `jvb-debug` |

Jitsi only: mediasoup gives no per-receiver estimate (R12 fact 4); its
comparison uses the browser's heights per second and the bots'
`Received video height changed` log lines.

Derived for the verdict (research R11): each receiver's need, the second its
estimate reached it, and the time spent under it.

## Verdict (validation report)

Per server: `the bots`, `the server`, both, or `not reproduced`; the runs
behind it (report files and estimate files), the bots held low and for how
long, the time the browser needed, the engine's and the browser's CPU, and
what differed between the browser's streams and a bot's.
