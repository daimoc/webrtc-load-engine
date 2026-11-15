# Load test report: live-mediasoup

mediasoup · run 8262528b · 2026-11-15 14:15:58 → 2026-11-15 14:17:45 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 107.069 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.01 |
| Audio concealed | p95 9.21 % |
| Video freezes | 117 in total; freeze time p95 75.125 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 94.55 %, 360: 1.60 %, 180: 3.85 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.25 threads, peak 0.30 threads |
| Server memory (total) | peak 87.1 MiB |
| Engine CPU / memory | avg 0.69 threads, peak 0.79 threads · peak 201.5 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 9.21 % | 9.21 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 75.125 s | 75.125 s |
| Requested-height share | 99.20 % | 99.75 % | 99.75 % |
| Latency | 2.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 117
- Resolution switches: 41
- Keyframe requests sent: 67, received: 204

| Height | Share of video time |
|---|---|
| 720 | 94.55 % |
| 360 | 1.60 % |
| 180 | 3.85 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.10 % | 0.51 % | 0.51 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:15:58 to 2026-11-15 14:17:45 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.04 threads | 0.25 threads | 0.30 threads | 85.7 MiB | 87.1 MiB |
| **total** | 1 measured | 23 / 0 | 0.04 threads | 0.25 threads | 0.30 threads | 85.7 MiB | 87.1 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.69 threads | 0.79 threads | 201.5 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:15:58 to 2026-11-15 14:17:45 (UTC).

## Network conditions

Seed 3820692738038656227 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 785 kbit/s, 1089 at most over 10 s | 56.23 % | 0.00 % | 80 ms, 209 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 3 of 3 | 6.0 s, 8.0 s | 33.18 % |

<details><summary>Per bot</summary>

- peer-live-aacf0eaa-7, step 0: reaction 2.0 s, recovery 6.0 s
- peer-live-aacf0eaa-8, step 0: reaction 2.0 s, recovery 8.0 s
- peer-live-aacf0eaa-9, step 0: reaction 2.0 s, recovery 6.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 270 kbit/s (63 samples, 119 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 81.85 %, 360: 4.17 %, 180: 13.97 % | 83.04 % | 117 | 4.04 |
| no condition (7 bots) | 720: 99.38 %, 360: 0.62 % | 99.25 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-aacf0eaa-9 | 4.01 |
| peer-live-aacf0eaa-8 | 4.04 |
| peer-live-aacf0eaa-7 | 4.08 |
| peer-live-aacf0eaa-2 | 4.37 |
| peer-live-aacf0eaa-1 | 4.37 |
| peer-live-aacf0eaa-0 | 4.37 |
| peer-live-aacf0eaa-6 | 4.37 |
| peer-live-aacf0eaa-3 | 4.37 |
| peer-live-aacf0eaa-5 | 4.37 |
| peer-live-aacf0eaa-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-aacf0eaa-9 | 75.125 s |
| peer-live-aacf0eaa-7 | 47.394 s |
| peer-live-aacf0eaa-8 | 43.278 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-aacf0eaa-9 | 64.5 ms |
| peer-live-aacf0eaa-7 | 16.5 ms |
| peer-live-aacf0eaa-8 | 15.5 ms |
| peer-live-aacf0eaa-0 | 6.5 ms |
| peer-live-aacf0eaa-1 | 6.5 ms |
| peer-live-aacf0eaa-2 | 6.5 ms |
| peer-live-aacf0eaa-3 | 6.5 ms |
| peer-live-aacf0eaa-4 | 5.5 ms |
| peer-live-aacf0eaa-5 | 5.5 ms |
| peer-live-aacf0eaa-6 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-aacf0eaa-9 | 78.37 % |
| peer-live-aacf0eaa-7 | 83.04 % |
| peer-live-aacf0eaa-8 | 84.24 % |
| peer-live-aacf0eaa-0 | 99.00 % |
| peer-live-aacf0eaa-6 | 99.20 % |
| peer-live-aacf0eaa-5 | 99.22 % |
| peer-live-aacf0eaa-2 | 99.25 % |
| peer-live-aacf0eaa-1 | 99.50 % |
| peer-live-aacf0eaa-4 | 99.75 % |
| peer-live-aacf0eaa-3 | 99.75 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-change2",
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
            "duration_seconds": 20,
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
