# Load test report: live-livekit

livekit · run 7f251ce9 · 2026-11-15 12:07:44 → 2026-11-15 12:09:02 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.851 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 2.13 |
| Audio concealed | p95 69.60 % |
| Video freezes | 295 in total; freeze time p95 134.260 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.75 %, 360: 0.18 %, 180: 0.07 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | avg 0.68 threads, peak 0.78 threads |
| Server memory (total) | peak 124.1 MiB |
| Engine CPU / memory | avg 0.81 threads, peak 0.88 threads · peak 204.3 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 69.60 % | 69.60 % |
| Latency | 0.5 ms | 185.5 ms | 185.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 134.260 s | 134.260 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 295
- Resolution switches: 8
- Keyframe requests sent: 328, received: 222

| Height | Share of video time |
|---|---|
| 720 | 99.75 % |
| 360 | 0.18 % |
| 180 | 0.07 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 12:07:44 to 2026-11-15 12:09:02 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.35 threads | 0.68 threads | 0.78 threads | 16.2 MiB | 124.1 MiB |
| **total** | 1 measured | 17 / 0 | 0.35 threads | 0.68 threads | 0.78 threads | 16.2 MiB | 124.1 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.81 threads | 0.88 threads | 204.3 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:07:44 to 2026-11-15 12:09:02 (UTC).

## Network conditions

Seed 2134229028989897847 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 500 kbit/s, queue 200 ms | 30.0–77.8 s | 502 kbit/s, 501 at most over 10 s | 92.60 % | 0.00 % | 197 ms, 220 at most |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 99.62 %, 360: 0.26 %, 180: 0.13 % | 100.00 % | 295 | 2.14 |
| no condition (7 bots) | 720: 99.80 %, 360: 0.15 %, 180: 0.05 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-6ab33a99-8 | 2.13 |
| peer-live-6ab33a99-9 | 2.14 |
| peer-live-6ab33a99-7 | 2.19 |
| peer-live-6ab33a99-0 | 4.37 |
| peer-live-6ab33a99-1 | 4.37 |
| peer-live-6ab33a99-2 | 4.37 |
| peer-live-6ab33a99-3 | 4.37 |
| peer-live-6ab33a99-4 | 4.37 |
| peer-live-6ab33a99-5 | 4.37 |
| peer-live-6ab33a99-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-6ab33a99-8 | 134.260 s |
| peer-live-6ab33a99-7 | 110.220 s |
| peer-live-6ab33a99-9 | 108.175 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-6ab33a99-7 | 202.5 ms |
| peer-live-6ab33a99-8 | 202.5 ms |
| peer-live-6ab33a99-9 | 202.5 ms |
| peer-live-6ab33a99-0 | 2.5 ms |
| peer-live-6ab33a99-1 | 2.5 ms |
| peer-live-6ab33a99-2 | 1.5 ms |
| peer-live-6ab33a99-3 | 1.5 ms |
| peer-live-6ab33a99-4 | 1.5 ms |
| peer-live-6ab33a99-5 | 1.5 ms |
| peer-live-6ab33a99-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-6ab33a99-9 | 98.73 % |
| peer-live-6ab33a99-1 | 99.33 % |
| peer-live-6ab33a99-3 | 99.66 % |
| peer-live-6ab33a99-2 | 99.66 % |
| peer-live-6ab33a99-0 | 100.00 % |
| peer-live-6ab33a99-4 | 100.00 % |
| peer-live-6ab33a99-5 | 100.00 % |
| peer-live-6ab33a99-6 | 100.00 % |
| peer-live-6ab33a99-7 | 100.00 % |
| peer-live-6ab33a99-8 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "us1-016",
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
