# Data Model: Media Quality

In-memory only. Trackers and the score live in `src/media`; per-bot records
and the summary builder in `src/execution`; the summary type in
`src/scenario`.

## Video Stream Quality (`media.VideoQuality`, per received video stream)

Updated by the stream's read loop, read by the collector (mutex, as
`InboundStats`).

| Field | Meaning | Rule |
|-------|---------|------|
| `frames` | frames started (cumulative) | a packet with S=1 and partition 0, or a new RTP timestamp (R1) |
| `lastFrame` | arrival of the last frame start | |
| `interval` | usual frame interval | EWMA 1/16 of non-freeze intervals; seeded with the first interval |
| `freezes`, `freezeTime` | cumulative count and duration | interval > max(3 × `interval`, 150 ms), duration capped at the stall timeout, counted on resume (R2) |
| `height` | height of the last keyframe (0 before the first) | RFC 6386 keyframe header |
| `heightTime` | time spent per height | accumulated at each frame |
| `switches` | height changes after the first keyframe | |
| `needKeyframe` | waiting for a keyframe | set at the first frame or after an unrepaired gap when the frame is not a keyframe; triggers a PLI at most once per second (R8) |
| `latencyBuckets` | latency counts per bucket | from the send-time registry (R5) |

## Audio Stream Quality (`media.AudioQuality`, per received audio stream)

| Field | Meaning | Rule |
|-------|---------|------|
| `base` | min(arrival − media time) | over the stream; reset on restart (sequence jump > 1,000 or timestamp jump > 10 s) |
| `firstSeq`, `highestSeq`, `cycles` | expected frames | as `InboundStats` |
| `concealed` | frames not there at their deadline | missing at a later packet's deadline, or arrived late; each frame once (R3) |
| `periodConcealed`, `periodExpected` | values for the current collection period | reset by the collector |
| `latencyBuckets` | as video | |

The score is computed by the collector for each audio stream that received
in the period: `media.MOS(lossPct, oneWayDelay)` (R4).

## Send-Time Registry (`media.SendTimes`, per scenario and kind)

| Field | Meaning |
|-------|---------|
| `ring[512]` | (RTP timestamp, write start time) of the last frames, written by the source |
| lookup | timestamp → time, or not found (never a guess) |

With simulcast the three layers share one timeline, so one registry per
kind still applies.

## Per-Bot Quality Record (`execution`, per bot, kept until the scenario ends)

Folded from the bot's stream trackers at each collection, kept after the
bot leaves.

| Field | Meaning |
|-------|---------|
| `videoFrameRate` | mean frames/s over the bot's received video streams and periods |
| `freezes`, `freezeSeconds` | totals |
| `heightSeconds` | time per received height |
| `switches` | resolution switches |
| `keyframeRequestsSent`, `keyframeRequestsReceived` | PLI sent as receiver; PLI and FIR received as sender |
| `audioConcealedRatio` | concealed / expected over the run |
| `mos` | mean of the period scores |
| `latency[kind]` | bucket counts, giving median and 95th percentile |

## Quality Summary (`scenario.QualitySummary`, per scenario)

Built at the last collection, stored on the scenario, served by the API
([contracts/quality-summary.yaml](contracts/quality-summary.yaml)).

| Field | Meaning |
|-------|---------|
| `generated_at`, `bots` | build time, bots with a record |
| `video` | percentiles (p50, p95, p99) over bots of frame rate, freeze seconds, latency (bot median); totals (freezes, switches, keyframe requests); time share per height; `available: false` with a reason when no bot received video |
| `audio` | percentiles over bots of concealed ratio, MOS, latency; totals; `available` as for video |
| `worst` | up to 10 bots per ranking: lowest MOS, most freeze seconds, highest latency p95; each with its peer ID and value |

## Resolution Preference (test plan, `scenario.MediaConfig`)

| Field | Rule |
|-------|------|
| `simulcast.max_height` | absent: single layer (360, as 004); 360 → layers 90/180/360; 720 → 180/360/720 |
| `requested_heights` | absent: top layer sent for every bot; else list of `{height, share}`; shares > 0 and sum to 1 (±0.001); bots assigned in launch order |

Validation errors are returned at scenario creation, as for the 004 fields.
