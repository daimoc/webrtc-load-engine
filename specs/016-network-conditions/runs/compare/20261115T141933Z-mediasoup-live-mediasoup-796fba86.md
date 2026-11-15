# Load test report: live-mediasoup

mediasoup · run 796fba86 · 2026-11-15 14:17:46 → 2026-11-15 14:19:33 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 107.059 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.80 |
| Audio concealed | p95 15.19 % |
| Video freezes | 141 in total; freeze time p95 55.321 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 91.91 %, 360: 1.85 %, 180: 6.24 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.24 threads, peak 0.29 threads |
| Server memory (total) | peak 87.4 MiB |
| Engine CPU / memory | avg 0.70 threads, peak 0.78 threads · peak 200.3 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 15.19 % | 15.19 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 55.321 s | 55.321 s |
| Requested-height share | 97.08 % | 100.00 % | 100.00 % |
| Latency | 2.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 141
- Resolution switches: 50
- Keyframe requests sent: 76, received: 201

| Height | Share of video time |
|---|---|
| 720 | 91.91 % |
| 360 | 1.85 % |
| 180 | 6.24 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.10 % | 0.35 % | 0.35 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:17:46 to 2026-11-15 14:19:33 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.04 threads | 0.24 threads | 0.29 threads | 86.2 MiB | 87.4 MiB |
| **total** | 1 measured | 23 / 0 | 0.04 threads | 0.24 threads | 0.29 threads | 86.2 MiB | 87.4 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.70 threads | 0.78 threads | 200.3 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:17:46 to 2026-11-15 14:19:33 (UTC).

## Network conditions

Seed 6884591836125048141 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 860 kbit/s, 1030 at most over 10 s | 54.77 % | 0.00 % | 91 ms, 210 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 3 of 3 | 6.0 s, 26.0 s | 29.78 % |

<details><summary>Per bot</summary>

- peer-live-f28a0686-7, step 0: reaction 2.0 s, recovery 26.0 s
- peer-live-f28a0686-8, step 0: reaction 2.0 s, recovery 6.0 s
- peer-live-f28a0686-9, step 0: reaction 2.0 s, recovery 4.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 353 kbit/s (63 samples, 141 to 8610) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 74.58 %, 360: 3.38 %, 180: 22.04 % | 80.64 % | 141 | 4.08 |
| no condition (7 bots) | 720: 98.47 %, 360: 1.27 %, 180: 0.25 % | 98.70 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-f28a0686-9 | 3.80 |
| peer-live-f28a0686-7 | 4.08 |
| peer-live-f28a0686-8 | 4.09 |
| peer-live-f28a0686-2 | 4.37 |
| peer-live-f28a0686-1 | 4.37 |
| peer-live-f28a0686-0 | 4.37 |
| peer-live-f28a0686-6 | 4.37 |
| peer-live-f28a0686-3 | 4.37 |
| peer-live-f28a0686-5 | 4.37 |
| peer-live-f28a0686-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-f28a0686-7 | 55.321 s |
| peer-live-f28a0686-9 | 49.735 s |
| peer-live-f28a0686-8 | 45.265 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-f28a0686-9 | 197.5 ms |
| peer-live-f28a0686-7 | 15.5 ms |
| peer-live-f28a0686-8 | 15.5 ms |
| peer-live-f28a0686-0 | 6.5 ms |
| peer-live-f28a0686-1 | 6.5 ms |
| peer-live-f28a0686-2 | 6.5 ms |
| peer-live-f28a0686-3 | 6.5 ms |
| peer-live-f28a0686-4 | 5.5 ms |
| peer-live-f28a0686-5 | 5.5 ms |
| peer-live-f28a0686-6 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-f28a0686-7 | 61.59 % |
| peer-live-f28a0686-9 | 80.64 % |
| peer-live-f28a0686-8 | 81.16 % |
| peer-live-f28a0686-1 | 96.76 % |
| peer-live-f28a0686-6 | 97.08 % |
| peer-live-f28a0686-0 | 98.25 % |
| peer-live-f28a0686-5 | 98.70 % |
| peer-live-f28a0686-3 | 99.24 % |
| peer-live-f28a0686-2 | 99.25 % |
| peer-live-f28a0686-4 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-change3",
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
