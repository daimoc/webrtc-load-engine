# Load test report: live-mediasoup

mediasoup · run 9ad65fca · 2026-11-15 13:02:42 → 2026-11-15 13:04:29 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 107.042 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.88 |
| Audio concealed | p95 11.74 % |
| Video freezes | 115 in total; freeze time p95 69.502 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 93.05 %, 360: 2.05 %, 180: 4.90 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.24 threads, peak 0.31 threads |
| Server memory (total) | peak 60.6 MiB |
| Engine CPU / memory | avg 0.68 threads, peak 0.78 threads · peak 198.2 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 11.74 % | 11.74 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 69.502 s | 69.502 s |
| Requested-height share | 99.01 % | 99.75 % | 99.75 % |
| Latency | 2.5 ms | 4.5 ms | 4.5 ms |

- Freezes: 115
- Resolution switches: 38
- Keyframe requests sent: 66, received: 191

| Height | Share of video time |
|---|---|
| 720 | 93.05 % |
| 360 | 2.05 % |
| 180 | 4.90 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.10 % | 0.23 % | 0.23 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 13:02:42 to 2026-11-15 13:04:29 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.07 threads | 0.24 threads | 0.31 threads | 36.8 MiB | 60.6 MiB |
| **total** | 1 measured | 23 / 0 | 0.07 threads | 0.24 threads | 0.31 threads | 36.8 MiB | 60.6 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.68 threads | 0.78 threads | 198.2 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:02:42 to 2026-11-15 13:04:29 (UTC).

## Network conditions

Seed 1714622298101981007 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 798 kbit/s, 1003 at most over 10 s | 53.88 % | 0.00 % | 80 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) |
|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 2 of 3 | 8.0 s, 10.0 s |

<details><summary>Per bot</summary>

- peer-live-134b5bbc-7, step 0: reaction 2.0 s, recovery 8.0 s
- peer-live-134b5bbc-8, step 0: reaction 2.0 s, not recovered (not_recovered_before_end)
- peer-live-134b5bbc-9, step 0: reaction 2.0 s, recovery 10.0 s

</details>

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 72.71 %, 360: 6.66 %, 180: 20.63 % | 78.35 % | 115 | 4.08 |
| no condition (7 bots) | 720: 99.38 %, 360: 0.62 % | 99.50 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-134b5bbc-9 | 3.88 |
| peer-live-134b5bbc-7 | 4.08 |
| peer-live-134b5bbc-8 | 4.11 |
| peer-live-134b5bbc-2 | 4.37 |
| peer-live-134b5bbc-1 | 4.37 |
| peer-live-134b5bbc-0 | 4.37 |
| peer-live-134b5bbc-6 | 4.37 |
| peer-live-134b5bbc-3 | 4.37 |
| peer-live-134b5bbc-5 | 4.37 |
| peer-live-134b5bbc-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-134b5bbc-9 | 69.502 s |
| peer-live-134b5bbc-7 | 46.631 s |
| peer-live-134b5bbc-8 | 20.240 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-134b5bbc-9 | 181.5 ms |
| peer-live-134b5bbc-7 | 19.5 ms |
| peer-live-134b5bbc-8 | 12.5 ms |
| peer-live-134b5bbc-0 | 6.5 ms |
| peer-live-134b5bbc-1 | 6.5 ms |
| peer-live-134b5bbc-2 | 6.5 ms |
| peer-live-134b5bbc-3 | 6.5 ms |
| peer-live-134b5bbc-4 | 5.5 ms |
| peer-live-134b5bbc-5 | 5.5 ms |
| peer-live-134b5bbc-6 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-134b5bbc-8 | 43.15 % |
| peer-live-134b5bbc-9 | 78.35 % |
| peer-live-134b5bbc-7 | 79.90 % |
| peer-live-134b5bbc-5 | 98.96 % |
| peer-live-134b5bbc-0 | 99.01 % |
| peer-live-134b5bbc-6 | 99.20 % |
| peer-live-134b5bbc-3 | 99.50 % |
| peer-live-134b5bbc-2 | 99.50 % |
| peer-live-134b5bbc-4 | 99.74 % |
| peer-live-134b5bbc-1 | 99.75 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-us2",
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
