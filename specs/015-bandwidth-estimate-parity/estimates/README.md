# Receiver estimates (015)

One file per comparison run, written by `scripts/read-estimates.sh`:
`<server>-<codec>-<n>.jsonl` (for example `mediasoup-vp8-1.jsonl`), kept
beside the run's two report files.

One JSON object per line, one line per receiver per second
([data-model.md](../data-model.md), "Receiver estimate"):

| Field | Meaning |
|---|---|
| `t` | UTC time of the reading |
| `server` | `mediasoup` or `jitsi` |
| `receiver` | the bot's endpoint id, or `browser` |
| `endpoint` | the server's id of the receiver |
| `estimate_bps` | the server's estimate toward that receiver |
| `ideal_bps` | what the server would send with no limit (the receiver's need) |
| `target_bps` | what the server allocates within the estimate |
| `suspended` | whether some of the receiver's streams are held back |
| `method` | `jvb-debug` |

Jitsi only (`scripts/read-estimates.py jitsi`). mediasoup gives no
per-receiver estimate: its runs keep the live test's log
(`mediasoup-<codec>-<n>.log`, with the bots' `Received video height
changed` lines) beside the report files instead.
Tables: `python3 -I scripts/estimate-verdict.py jitsi <file>.jsonl` or
`... mediasoup <report>.json <run>.log`.

The estimate files and the live tests' logs are compressed (`<run>.log.xz`, also the browser's
`<run>-browser.log.xz` for the remote runs): read one with `xzcat`, for
example `xzcat jitsi-vp8-4.jsonl.xz > /tmp/e.jsonl` before
`estimate-verdict.py jitsi /tmp/e.jsonl`, or `xzcat mediasoup-vp8-3.log.xz >
/tmp/m.log` before `estimate-verdict.py mediasoup`.
