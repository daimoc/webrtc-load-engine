# Load test report: live-mediasoup

mediasoup · run b5956d98 · 2026-11-15 14:12:45 → 2026-11-15 14:14:10 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 85.017 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.16 |
| Audio concealed | p95 28.75 % |
| Video freezes | 163 in total; freeze time p95 119.513 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 83.89 %, 360: 3.15 %, 180: 12.96 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.23 threads, peak 0.60 threads |
| Server memory (total) | peak 83.5 MiB |
| Engine CPU / memory | avg 0.67 threads, peak 1.19 threads · peak 194.8 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 28.75 % | 28.75 % |
| Latency | 0.5 ms | 4.5 ms | 4.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 119.513 s | 119.513 s |
| Requested-height share | 98.09 % | 99.36 % | 99.36 % |
| Latency | 2.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 163
- Resolution switches: 44
- Keyframe requests sent: 91, received: 216

| Height | Share of video time |
|---|---|
| 720 | 83.89 % |
| 360 | 3.15 % |
| 180 | 12.96 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.50 % | 0.50 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:12:45 to 2026-11-15 14:14:10 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.04 threads | 0.23 threads | 0.60 threads | 82.1 MiB | 83.5 MiB |
| **total** | 1 measured | 19 / 0 | 0.04 threads | 0.23 threads | 0.60 threads | 82.1 MiB | 83.5 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.67 threads | 1.19 threads | 194.8 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:12:45 to 2026-11-15 14:14:10 (UTC).

## Network conditions

Seed 11636995182085136693 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–85.0 s | 699 kbit/s, 1050 at most over 10 s | 31.85 % | 0.00 % | 44 ms, 210 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | step open to the end | | 19.66 % |

<details><summary>Per bot</summary>

- peer-live-65bdd6cb-7, step 0: reaction 2.0 s
- peer-live-65bdd6cb-8, step 0: reaction 2.0 s
- peer-live-65bdd6cb-9, step 0: reaction 2.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 297 kbit/s (168 samples, 93 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 33.02 %, 360: 9.46 %, 180: 57.52 % | 26.93 % | 163 | 3.80 |
| no condition (7 bots) | 720: 98.69 %, 360: 1.31 % | 98.73 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-65bdd6cb-8 | 3.16 |
| peer-live-65bdd6cb-9 | 3.80 |
| peer-live-65bdd6cb-7 | 4.01 |
| peer-live-65bdd6cb-2 | 4.37 |
| peer-live-65bdd6cb-1 | 4.37 |
| peer-live-65bdd6cb-0 | 4.37 |
| peer-live-65bdd6cb-6 | 4.37 |
| peer-live-65bdd6cb-3 | 4.37 |
| peer-live-65bdd6cb-5 | 4.37 |
| peer-live-65bdd6cb-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-65bdd6cb-8 | 119.513 s |
| peer-live-65bdd6cb-9 | 102.568 s |
| peer-live-65bdd6cb-7 | 13.410 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-65bdd6cb-8 | 203.5 ms |
| peer-live-65bdd6cb-7 | 178.5 ms |
| peer-live-65bdd6cb-9 | 166.5 ms |
| peer-live-65bdd6cb-0 | 5.5 ms |
| peer-live-65bdd6cb-1 | 5.5 ms |
| peer-live-65bdd6cb-2 | 5.5 ms |
| peer-live-65bdd6cb-3 | 5.5 ms |
| peer-live-65bdd6cb-4 | 4.5 ms |
| peer-live-65bdd6cb-5 | 4.5 ms |
| peer-live-65bdd6cb-6 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-65bdd6cb-9 | 24.75 % |
| peer-live-65bdd6cb-8 | 26.93 % |
| peer-live-65bdd6cb-7 | 59.46 % |
| peer-live-65bdd6cb-2 | 98.09 % |
| peer-live-65bdd6cb-0 | 98.09 % |
| peer-live-65bdd6cb-5 | 98.31 % |
| peer-live-65bdd6cb-1 | 98.73 % |
| peer-live-65bdd6cb-4 | 99.01 % |
| peer-live-65bdd6cb-6 | 99.31 % |
| peer-live-65bdd6cb-3 | 99.36 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-slow3",
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
