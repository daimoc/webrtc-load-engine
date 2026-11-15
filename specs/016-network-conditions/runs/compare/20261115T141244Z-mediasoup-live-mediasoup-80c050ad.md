# Load test report: live-mediasoup

mediasoup · run 80c050ad · 2026-11-15 14:11:19 → 2026-11-15 14:12:44 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 85.014 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.68 |
| Audio concealed | p95 15.02 % |
| Video freezes | 132 in total; freeze time p95 73.461 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 82.93 %, 360: 3.32 %, 180: 13.75 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.23 threads, peak 1.31 threads |
| Server memory (total) | peak 83.0 MiB |
| Engine CPU / memory | avg 0.67 threads, peak 4.31 threads · peak 198.1 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 15.02 % | 15.02 % |
| Latency | 0.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 73.461 s | 73.461 s |
| Requested-height share | 98.40 % | 99.67 % | 99.67 % |
| Latency | 2.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 132
- Resolution switches: 41
- Keyframe requests sent: 68, received: 189

| Height | Share of video time |
|---|---|
| 720 | 82.93 % |
| 360 | 3.32 % |
| 180 | 13.75 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.31 % | 0.31 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:11:19 to 2026-11-15 14:12:44 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.05 threads | 0.23 threads | 1.31 threads | 60.4 MiB | 83.0 MiB |
| **total** | 1 measured | 19 / 0 | 0.05 threads | 0.23 threads | 1.31 threads | 60.4 MiB | 83.0 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.67 threads | 4.31 threads | 198.1 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:11:19 to 2026-11-15 14:12:44 (UTC).

## Network conditions

Seed 13262994886629611282 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–85.0 s | 761 kbit/s, 1100 at most over 10 s | 29.60 % | 0.00 % | 35 ms, 209 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | step open to the end | | 20.23 % |

<details><summary>Per bot</summary>

- peer-live-dddc961e-7, step 0: reaction 2.0 s
- peer-live-dddc961e-8, step 0: reaction 2.0 s
- peer-live-dddc961e-9, step 0: reaction 2.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 491 kbit/s (166 samples, 140 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 31.00 %, 360: 10.75 %, 180: 58.25 % | 30.71 % | 132 | 3.82 |
| no condition (7 bots) | 720: 98.97 %, 360: 1.03 % | 99.03 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-dddc961e-9 | 3.68 |
| peer-live-dddc961e-8 | 3.82 |
| peer-live-dddc961e-7 | 3.90 |
| peer-live-dddc961e-2 | 4.37 |
| peer-live-dddc961e-1 | 4.37 |
| peer-live-dddc961e-6 | 4.37 |
| peer-live-dddc961e-3 | 4.37 |
| peer-live-dddc961e-5 | 4.37 |
| peer-live-dddc961e-0 | 4.37 |
| peer-live-dddc961e-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-dddc961e-7 | 73.461 s |
| peer-live-dddc961e-9 | 71.763 s |
| peer-live-dddc961e-8 | 52.047 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-dddc961e-8 | 197.5 ms |
| peer-live-dddc961e-9 | 194.5 ms |
| peer-live-dddc961e-7 | 190.5 ms |
| peer-live-dddc961e-0 | 5.5 ms |
| peer-live-dddc961e-1 | 5.5 ms |
| peer-live-dddc961e-2 | 5.5 ms |
| peer-live-dddc961e-3 | 5.5 ms |
| peer-live-dddc961e-4 | 4.5 ms |
| peer-live-dddc961e-5 | 4.5 ms |
| peer-live-dddc961e-6 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-dddc961e-9 | 27.09 % |
| peer-live-dddc961e-7 | 30.71 % |
| peer-live-dddc961e-8 | 36.10 % |
| peer-live-dddc961e-0 | 98.40 % |
| peer-live-dddc961e-1 | 98.40 % |
| peer-live-dddc961e-5 | 98.65 % |
| peer-live-dddc961e-3 | 99.03 % |
| peer-live-dddc961e-2 | 99.04 % |
| peer-live-dddc961e-6 | 99.65 % |
| peer-live-dddc961e-4 | 99.67 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-slow2",
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
