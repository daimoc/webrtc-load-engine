# Load test report: live-livekit

livekit · run b7220c2e · 2026-11-15 14:05:10 → 2026-11-15 14:06:48 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 97.924 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 3.86 |
| Audio concealed | p95 8.55 % |
| Video freezes | 36 in total; freeze time p95 7.400 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 84.60 %, 360: 4.92 %, 180: 10.48 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.60 threads, peak 0.66 threads |
| Server memory (total) | peak 131.8 MiB |
| Engine CPU / memory | avg 0.80 threads, peak 0.89 threads · peak 204.0 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 8.55 % | 8.55 % |
| Latency | 0.5 ms | 0.5 ms | 0.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 7.400 s | 7.400 s |
| Requested-height share | 99.73 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 36
- Resolution switches: 52
- Keyframe requests sent: 28, received: 98

| Height | Share of video time |
|---|---|
| 720 | 84.60 % |
| 360 | 4.92 % |
| 180 | 10.48 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:05:10 to 2026-11-15 14:06:48 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 21 / 0 | 0.28 threads | 0.60 threads | 0.66 threads | 131.8 MiB | 131.8 MiB |
| **total** | 1 measured | 21 / 0 | 0.28 threads | 0.60 threads | 0.66 threads | 131.8 MiB | 131.8 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.80 threads | 0.89 threads | 204.0 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:05:10 to 2026-11-15 14:06:48 (UTC).

## Network conditions

Seed 3301338439464542659 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 1080 kbit/s, 1119 at most over 10 s | 4.27 % | 0.00 % | 58 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 3 of 3 | 34.0 s, 34.0 s | 13.88 % |

<details><summary>Per bot</summary>

- peer-live-3f3ca829-7, step 0: reaction 2.0 s, recovery 32.0 s
- peer-live-3f3ca829-8, step 0: reaction 2.0 s, recovery 34.0 s
- peer-live-3f3ca829-9, step 0: reaction 2.0 s, recovery 34.0 s

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 47.11 %, 360: 16.78 %, 180: 36.11 % | 47.77 % | 36 | 3.87 |
| no condition (7 bots) | 720: 99.92 %, 360: 0.08 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-3f3ca829-9 | 3.86 |
| peer-live-3f3ca829-8 | 3.87 |
| peer-live-3f3ca829-7 | 3.89 |
| peer-live-3f3ca829-0 | 4.37 |
| peer-live-3f3ca829-1 | 4.37 |
| peer-live-3f3ca829-2 | 4.37 |
| peer-live-3f3ca829-3 | 4.37 |
| peer-live-3f3ca829-4 | 4.37 |
| peer-live-3f3ca829-5 | 4.37 |
| peer-live-3f3ca829-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-3f3ca829-7 | 7.400 s |
| peer-live-3f3ca829-8 | 7.188 s |
| peer-live-3f3ca829-9 | 3.392 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-3f3ca829-8 | 87.5 ms |
| peer-live-3f3ca829-9 | 87.5 ms |
| peer-live-3f3ca829-7 | 85.5 ms |
| peer-live-3f3ca829-1 | 2.5 ms |
| peer-live-3f3ca829-0 | 1.5 ms |
| peer-live-3f3ca829-2 | 1.5 ms |
| peer-live-3f3ca829-3 | 1.5 ms |
| peer-live-3f3ca829-4 | 1.5 ms |
| peer-live-3f3ca829-5 | 1.5 ms |
| peer-live-3f3ca829-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-3f3ca829-9 | 43.75 % |
| peer-live-3f3ca829-8 | 47.77 % |
| peer-live-3f3ca829-7 | 49.74 % |
| peer-live-3f3ca829-4 | 99.73 % |
| peer-live-3f3ca829-3 | 99.73 % |
| peer-live-3f3ca829-0 | 100.00 % |
| peer-live-3f3ca829-1 | 100.00 % |
| peer-live-3f3ca829-2 | 100.00 % |
| peer-live-3f3ca829-5 | 100.00 % |
| peer-live-3f3ca829-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-change3",
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
