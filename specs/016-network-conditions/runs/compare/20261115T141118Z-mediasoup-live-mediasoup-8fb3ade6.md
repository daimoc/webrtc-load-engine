# Load test report: live-mediasoup

mediasoup · run 8fb3ade6 · 2026-11-15 14:09:53 → 2026-11-15 14:11:18 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 85.012 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.37 |
| Audio concealed | p95 23.26 % |
| Video freezes | 162 in total; freeze time p95 105.244 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 83.58 %, 360: 3.72 %, 180: 12.71 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.23 threads, peak 1.72 threads |
| Server memory (total) | peak 61.5 MiB |
| Engine CPU / memory | avg 0.66 threads, peak 4.90 threads · peak 196.2 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 23.26 % | 23.26 % |
| Latency | 0.5 ms | 4.5 ms | 4.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 105.244 s | 105.244 s |
| Requested-height share | 98.61 % | 100.00 % | 100.00 % |
| Latency | 2.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 162
- Resolution switches: 35
- Keyframe requests sent: 83, received: 203

| Height | Share of video time |
|---|---|
| 720 | 83.58 % |
| 360 | 3.72 % |
| 180 | 12.71 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.85 % | 0.85 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:09:53 to 2026-11-15 14:11:18 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.07 threads | 0.23 threads | 1.72 threads | 37.7 MiB | 61.5 MiB |
| **total** | 1 measured | 19 / 0 | 0.07 threads | 0.23 threads | 1.72 threads | 37.7 MiB | 61.5 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.66 threads | 4.90 threads | 196.2 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:09:53 to 2026-11-15 14:11:18 (UTC).

## Network conditions

Seed 13868513761214940653 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–85.0 s | 736 kbit/s, 1048 at most over 10 s | 30.88 % | 0.00 % | 41 ms, 209 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | step open to the end | | 19.07 % |

<details><summary>Per bot</summary>

- peer-live-e912d47d-7, step 0: reaction 2.0 s
- peer-live-e912d47d-8, step 0: reaction 2.0 s
- peer-live-e912d47d-9, step 0: reaction 2.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 389 kbit/s (166 samples, 115 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 33.36 %, 360: 12.61 %, 180: 54.03 % | 33.69 % | 162 | 3.79 |
| no condition (7 bots) | 720: 99.02 %, 360: 0.98 % | 98.73 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-e912d47d-9 | 3.37 |
| peer-live-e912d47d-7 | 3.79 |
| peer-live-e912d47d-8 | 3.89 |
| peer-live-e912d47d-6 | 4.37 |
| peer-live-e912d47d-3 | 4.37 |
| peer-live-e912d47d-2 | 4.37 |
| peer-live-e912d47d-5 | 4.37 |
| peer-live-e912d47d-1 | 4.37 |
| peer-live-e912d47d-0 | 4.37 |
| peer-live-e912d47d-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-e912d47d-9 | 105.244 s |
| peer-live-e912d47d-7 | 86.868 s |
| peer-live-e912d47d-8 | 43.033 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-e912d47d-9 | 202.5 ms |
| peer-live-e912d47d-8 | 184.5 ms |
| peer-live-e912d47d-7 | 162.5 ms |
| peer-live-e912d47d-0 | 5.5 ms |
| peer-live-e912d47d-1 | 5.5 ms |
| peer-live-e912d47d-2 | 5.5 ms |
| peer-live-e912d47d-3 | 5.5 ms |
| peer-live-e912d47d-4 | 4.5 ms |
| peer-live-e912d47d-5 | 4.5 ms |
| peer-live-e912d47d-6 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-e912d47d-9 | 29.31 % |
| peer-live-e912d47d-7 | 33.69 % |
| peer-live-e912d47d-8 | 38.25 % |
| peer-live-e912d47d-5 | 98.31 % |
| peer-live-e912d47d-6 | 98.61 % |
| peer-live-e912d47d-0 | 98.72 % |
| peer-live-e912d47d-1 | 98.73 % |
| peer-live-e912d47d-3 | 99.36 % |
| peer-live-e912d47d-2 | 99.36 % |
| peer-live-e912d47d-4 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-slow1",
    "url": "ws://localhost:4443"
  },
  "media": {
    "profile": "audio_video",
    "stall_timeout_seconds": 10,
    "max_received_streams": 4,
    "simulcast": {
      "max_height": 720
    },
    "requested_heights": [
      {
        "height": 720,
        "share": 1
      }
    ],
    "video_codec": "vp8"
  },
  "server_monitoring": {
    "containers": [
      "mediasoup-demo"
    ],
    "interval_seconds": 5
  },
  "network": {
    "groups": [
      {
        "name": "slow",
        "count": 3,
        "schedule": [
          {
            "start_seconds": 30,
            "downlink": {
              "rate_kbps": 1200,
              "queue_ms": 200
            }
          }
        ]
      }
    ]
  }
}
```

Engine devel (go1.27.2)
