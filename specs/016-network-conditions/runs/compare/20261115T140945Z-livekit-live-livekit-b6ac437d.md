# Load test report: live-livekit

livekit · run b6ac437d · 2026-11-15 14:08:08 → 2026-11-15 14:09:45 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 97.919 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.86 %, 360: 0.11 %, 180: 0.03 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.62 threads, peak 0.66 threads |
| Server memory (total) | peak 134.7 MiB |
| Engine CPU / memory | avg 0.81 threads, peak 0.88 threads · peak 186.3 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 0.00 % | 0.00 % |
| Latency | 0.5 ms | 0.5 ms | 0.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 0
- Resolution switches: 5
- Keyframe requests sent: 0, received: 36

| Height | Share of video time |
|---|---|
| 720 | 99.86 % |
| 360 | 0.11 % |
| 180 | 0.03 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:08:08 to 2026-11-15 14:09:45 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 21 / 0 | 0.28 threads | 0.62 threads | 0.66 threads | 134.7 MiB | 134.7 MiB |
| **total** | 1 measured | 21 / 0 | 0.28 threads | 0.62 threads | 0.66 threads | 134.7 MiB | 134.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.81 threads | 0.88 threads | 186.3 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:08:08 to 2026-11-15 14:09:45 (UTC).

## Network conditions

Seed 18121221107586038494 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slowup | 2 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–60.0 s | 339 kbit/s, 379 at most over 10 s | 64.42 % | 0.00 % | 105 ms, 225 at most |

Senders under an upload condition, layers sent over time:

- peer-live-5e838bc1-8: 180+360+720 from 8.0 s; 180+360 from 31.7 s; 180 from 32.9 s; 180+360 from 34.9 s; 180 from 36.7 s; 180+360 from 40.8 s; 180 from 42.4 s; 180+360 from 50.5 s; 180 from 52.4 s; 180+360 from 60.5 s; 180+360+720 from 65.6 s
- peer-live-5e838bc1-9: 180+360+720 from 9.0 s; 180+360 from 31.3 s; 180 from 32.4 s; 180+360 from 34.7 s; 180 from 36.3 s; 180+360 from 40.3 s; 180 from 42.6 s; 180+360 from 50.6 s; 180 from 52.6 s; 180+360 from 60.6 s; 180+360+720 from 65.8 s

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slowup (2 bots) | 720: 99.58 %, 360: 0.42 % | 99.43 % | 0 | 4.37 |
| no condition (8 bots) | 720: 99.93 %, 360: 0.03 %, 180: 0.03 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-5e838bc1-0 | 4.37 |
| peer-live-5e838bc1-1 | 4.37 |
| peer-live-5e838bc1-2 | 4.37 |
| peer-live-5e838bc1-3 | 4.37 |
| peer-live-5e838bc1-4 | 4.37 |
| peer-live-5e838bc1-5 | 4.37 |
| peer-live-5e838bc1-6 | 4.37 |
| peer-live-5e838bc1-7 | 4.37 |
| peer-live-5e838bc1-8 | 4.37 |
| peer-live-5e838bc1-9 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-5e838bc1-0 | 2.5 ms |
| peer-live-5e838bc1-1 | 2.5 ms |
| peer-live-5e838bc1-2 | 2.5 ms |
| peer-live-5e838bc1-5 | 2.5 ms |
| peer-live-5e838bc1-7 | 2.5 ms |
| peer-live-5e838bc1-8 | 2.5 ms |
| peer-live-5e838bc1-9 | 2.5 ms |
| peer-live-5e838bc1-3 | 1.5 ms |
| peer-live-5e838bc1-4 | 1.5 ms |
| peer-live-5e838bc1-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-5e838bc1-9 | 99.43 % |
| peer-live-5e838bc1-8 | 99.72 % |
| peer-live-5e838bc1-2 | 99.73 % |
| peer-live-5e838bc1-0 | 99.74 % |
| peer-live-5e838bc1-1 | 100.00 % |
| peer-live-5e838bc1-3 | 100.00 % |
| peer-live-5e838bc1-4 | 100.00 % |
| peer-live-5e838bc1-5 | 100.00 % |
| peer-live-5e838bc1-6 | 100.00 % |
| peer-live-5e838bc1-7 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-slowup",
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
        "name": "slowup",
        "count": 2,
        "schedule": [
          {
            "start_seconds": 30,
            "duration_seconds": 30,
            "uplink": {
              "rate_kbps": 400,
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
