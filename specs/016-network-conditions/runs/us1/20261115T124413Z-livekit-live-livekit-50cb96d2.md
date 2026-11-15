# Load test report: live-livekit

livekit · run 50cb96d2 · 2026-11-15 12:42:55 → 2026-11-15 12:44:13 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.860 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 2.12 |
| Audio concealed | p95 69.28 % |
| Video freezes | 395 in total; freeze time p95 101.026 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 83.18 %, 360: 0.32 %, 180: 16.49 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | avg 0.61 threads, peak 0.66 threads |
| Server memory (total) | peak 128.4 MiB |
| Engine CPU / memory | avg 0.74 threads, peak 0.81 threads · peak 205.1 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 69.28 % | 69.28 % |
| Latency | 0.5 ms | 188.5 ms | 188.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 101.026 s | 101.026 s |
| Requested-height share | 99.66 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 2.5 ms | 2.5 ms |

- Freezes: 395
- Resolution switches: 24
- Keyframe requests sent: 432, received: 318

| Height | Share of video time |
|---|---|
| 720 | 83.18 % |
| 360 | 0.32 % |
| 180 | 16.49 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 12:42:55 to 2026-11-15 12:44:13 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.33 threads | 0.61 threads | 0.66 threads | 122.7 MiB | 128.4 MiB |
| **total** | 1 measured | 17 / 0 | 0.33 threads | 0.61 threads | 0.66 threads | 122.7 MiB | 128.4 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.74 threads | 0.81 threads | 205.1 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:42:55 to 2026-11-15 12:44:13 (UTC).

## Network conditions

Seed 17790762453750774304 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 500 kbit/s, queue 200 ms | 30.0–77.9 s | 499 kbit/s, 500 at most over 10 s | 67.33 % | 0.00 % | 193 ms, 220 at most |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 41.52 %, 360: 1.01 %, 180: 57.47 % | 42.12 % | 395 | 2.12 |
| no condition (7 bots) | 720: 99.80 %, 360: 0.05 %, 180: 0.15 % | 99.66 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-7b386459-9 | 2.12 |
| peer-live-7b386459-8 | 2.12 |
| peer-live-7b386459-7 | 2.19 |
| peer-live-7b386459-0 | 4.37 |
| peer-live-7b386459-1 | 4.37 |
| peer-live-7b386459-2 | 4.37 |
| peer-live-7b386459-3 | 4.37 |
| peer-live-7b386459-4 | 4.37 |
| peer-live-7b386459-5 | 4.37 |
| peer-live-7b386459-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-7b386459-7 | 101.026 s |
| peer-live-7b386459-9 | 99.248 s |
| peer-live-7b386459-8 | 85.687 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-7b386459-8 | 211.5 ms |
| peer-live-7b386459-7 | 210.5 ms |
| peer-live-7b386459-9 | 210.5 ms |
| peer-live-7b386459-1 | 2.5 ms |
| peer-live-7b386459-6 | 2.5 ms |
| peer-live-7b386459-0 | 1.5 ms |
| peer-live-7b386459-2 | 1.5 ms |
| peer-live-7b386459-3 | 1.5 ms |
| peer-live-7b386459-4 | 1.5 ms |
| peer-live-7b386459-5 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-7b386459-8 | 32.77 % |
| peer-live-7b386459-9 | 42.12 % |
| peer-live-7b386459-7 | 49.64 % |
| peer-live-7b386459-4 | 99.66 % |
| peer-live-7b386459-3 | 99.66 % |
| peer-live-7b386459-1 | 99.66 % |
| peer-live-7b386459-0 | 99.66 % |
| peer-live-7b386459-2 | 100.00 % |
| peer-live-7b386459-5 | 100.00 % |
| peer-live-7b386459-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "us1-016b",
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
              "rate_kbps": 500,
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
