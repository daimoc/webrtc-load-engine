# Load test report: live-livekit

livekit · run b214ebce · 2026-11-15 13:57:56 → 2026-11-15 13:59:14 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.862 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 2.47 |
| Audio concealed | p95 47.13 % |
| Video freezes | 63 in total; freeze time p95 13.058 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 80.33 %, 360: 0.99 %, 180: 18.68 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.60 threads, peak 0.66 threads |
| Server memory (total) | peak 99.5 MiB |
| Engine CPU / memory | avg 0.82 threads, peak 0.90 threads · peak 204.3 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 47.13 % | 47.13 % |
| Latency | 0.5 ms | 29.5 ms | 29.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 13.058 s | 13.058 s |
| Requested-height share | 99.66 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 28.5 ms | 28.5 ms |

- Freezes: 63
- Resolution switches: 29
- Keyframe requests sent: 161, received: 179

| Height | Share of video time |
|---|---|
| 720 | 80.33 % |
| 360 | 0.99 % |
| 180 | 18.68 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 13:57:56 to 2026-11-15 13:59:14 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.33 threads | 0.60 threads | 0.66 threads | 16.8 MiB | 99.5 MiB |
| **total** | 1 measured | 17 / 0 | 0.33 threads | 0.60 threads | 0.66 threads | 16.8 MiB | 99.5 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.82 threads | 0.90 threads | 204.3 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:57:56 to 2026-11-15 13:59:14 (UTC).

## Network conditions

Seed 17670981742093840206 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–77.9 s | 1116 kbit/s, 1200 at most over 10 s | 13.29 % | 0.00 % | 90 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | step open to the end | | 8.57 % |

<details><summary>Per bot</summary>

- peer-live-89a2916c-7, step 0: reaction 2.0 s
- peer-live-89a2916c-8, step 0: reaction 2.0 s
- peer-live-89a2916c-9, step 0: reaction 2.0 s

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 31.80 %, 360: 3.33 %, 180: 64.87 % | 32.04 % | 63 | 2.77 |
| no condition (7 bots) | 720: 99.90 %, 360: 0.05 %, 180: 0.05 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-89a2916c-8 | 2.47 |
| peer-live-89a2916c-7 | 2.77 |
| peer-live-89a2916c-9 | 2.81 |
| peer-live-89a2916c-0 | 4.37 |
| peer-live-89a2916c-1 | 4.37 |
| peer-live-89a2916c-2 | 4.37 |
| peer-live-89a2916c-3 | 4.37 |
| peer-live-89a2916c-4 | 4.37 |
| peer-live-89a2916c-5 | 4.37 |
| peer-live-89a2916c-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-89a2916c-8 | 13.058 s |
| peer-live-89a2916c-7 | 7.758 s |
| peer-live-89a2916c-9 | 6.846 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-89a2916c-8 | 200.5 ms |
| peer-live-89a2916c-9 | 189.5 ms |
| peer-live-89a2916c-7 | 188.5 ms |
| peer-live-89a2916c-0 | 1.5 ms |
| peer-live-89a2916c-1 | 1.5 ms |
| peer-live-89a2916c-2 | 1.5 ms |
| peer-live-89a2916c-3 | 1.5 ms |
| peer-live-89a2916c-4 | 1.5 ms |
| peer-live-89a2916c-5 | 1.5 ms |
| peer-live-89a2916c-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-89a2916c-9 | 30.32 % |
| peer-live-89a2916c-8 | 32.04 % |
| peer-live-89a2916c-7 | 33.00 % |
| peer-live-89a2916c-3 | 99.66 % |
| peer-live-89a2916c-2 | 99.66 % |
| peer-live-89a2916c-0 | 100.00 % |
| peer-live-89a2916c-1 | 100.00 % |
| peer-live-89a2916c-4 | 100.00 % |
| peer-live-89a2916c-5 | 100.00 % |
| peer-live-89a2916c-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-slow1",
    "url": "ws://localhost:7880"
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
      "livekit-server"
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
