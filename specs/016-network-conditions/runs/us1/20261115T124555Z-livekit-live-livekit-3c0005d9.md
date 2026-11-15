# Load test report: live-livekit

livekit · run 3c0005d9 · 2026-11-15 12:44:37 → 2026-11-15 12:45:55 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.850 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 2.44 |
| Audio concealed | p95 50.21 % |
| Video freezes | 113 in total; freeze time p95 36.469 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 80.29 %, 360: 1.19 %, 180: 18.52 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | avg 0.60 threads, peak 0.66 threads |
| Server memory (total) | peak 126.6 MiB |
| Engine CPU / memory | avg 0.76 threads, peak 0.82 threads · peak 205.0 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 50.21 % | 50.21 % |
| Latency | 0.5 ms | 40.5 ms | 40.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 36.469 s | 36.469 s |
| Requested-height share | 99.66 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 20.5 ms | 20.5 ms |

- Freezes: 113
- Resolution switches: 33
- Keyframe requests sent: 184, received: 206

| Height | Share of video time |
|---|---|
| 720 | 80.29 % |
| 360 | 1.19 % |
| 180 | 18.52 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 12:44:37 to 2026-11-15 12:45:55 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.32 threads | 0.60 threads | 0.66 threads | 126.6 MiB | 126.6 MiB |
| **total** | 1 measured | 17 / 0 | 0.32 threads | 0.60 threads | 0.66 threads | 126.6 MiB | 126.6 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.76 threads | 0.82 threads | 205.0 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:44:37 to 2026-11-15 12:45:55 (UTC).

## Network conditions

Seed 15366674653497286415 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–77.8 s | 1117 kbit/s, 1201 at most over 10 s | 19.67 % | 0.00 % | 94 ms, 208 at most |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 31.74 %, 360: 4.03 %, 180: 64.23 % | 32.17 % | 113 | 2.64 |
| no condition (7 bots) | 720: 99.85 %, 360: 0.05 %, 180: 0.10 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-0f7c2bfb-8 | 2.44 |
| peer-live-0f7c2bfb-7 | 2.64 |
| peer-live-0f7c2bfb-9 | 2.89 |
| peer-live-0f7c2bfb-0 | 4.37 |
| peer-live-0f7c2bfb-1 | 4.37 |
| peer-live-0f7c2bfb-2 | 4.37 |
| peer-live-0f7c2bfb-3 | 4.37 |
| peer-live-0f7c2bfb-4 | 4.37 |
| peer-live-0f7c2bfb-5 | 4.37 |
| peer-live-0f7c2bfb-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-0f7c2bfb-8 | 36.469 s |
| peer-live-0f7c2bfb-7 | 9.871 s |
| peer-live-0f7c2bfb-9 | 6.103 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-0f7c2bfb-8 | 200.5 ms |
| peer-live-0f7c2bfb-7 | 198.5 ms |
| peer-live-0f7c2bfb-9 | 158.5 ms |
| peer-live-0f7c2bfb-0 | 1.5 ms |
| peer-live-0f7c2bfb-1 | 1.5 ms |
| peer-live-0f7c2bfb-2 | 1.5 ms |
| peer-live-0f7c2bfb-3 | 1.5 ms |
| peer-live-0f7c2bfb-4 | 1.5 ms |
| peer-live-0f7c2bfb-5 | 1.5 ms |
| peer-live-0f7c2bfb-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-0f7c2bfb-9 | 29.96 % |
| peer-live-0f7c2bfb-8 | 32.17 % |
| peer-live-0f7c2bfb-7 | 33.04 % |
| peer-live-0f7c2bfb-0 | 99.33 % |
| peer-live-0f7c2bfb-2 | 99.66 % |
| peer-live-0f7c2bfb-1 | 100.00 % |
| peer-live-0f7c2bfb-3 | 100.00 % |
| peer-live-0f7c2bfb-4 | 100.00 % |
| peer-live-0f7c2bfb-5 | 100.00 % |
| peer-live-0f7c2bfb-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "us1-016c",
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
    ]
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
