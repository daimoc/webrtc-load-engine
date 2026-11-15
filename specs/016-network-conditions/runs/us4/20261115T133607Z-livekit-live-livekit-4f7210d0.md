# Load test report: live-livekit

livekit · run 4f7210d0 · 2026-11-15 13:34:29 → 2026-11-15 13:36:07 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 97.921 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.81 %, 360: 0.11 %, 180: 0.08 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.61 threads, peak 0.68 threads |
| Server memory (total) | peak 103.8 MiB |
| Engine CPU / memory | avg 0.83 threads, peak 0.89 threads · peak 185.7 MiB · 16 hardware threads |

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
- Resolution switches: 7
- Keyframe requests sent: 0, received: 37

| Height | Share of video time |
|---|---|
| 720 | 99.81 % |
| 360 | 0.11 % |
| 180 | 0.08 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 13:34:29 to 2026-11-15 13:36:07 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 21 / 0 | 0.31 threads | 0.61 threads | 0.68 threads | 16.9 MiB | 103.8 MiB |
| **total** | 1 measured | 21 / 0 | 0.31 threads | 0.61 threads | 0.68 threads | 16.9 MiB | 103.8 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.83 threads | 0.89 threads | 185.7 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:34:29 to 2026-11-15 13:36:07 (UTC).

## Network conditions

Seed 17664719450965690738 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slowup | 2 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–60.0 s | 339 kbit/s, 379 at most over 10 s | 66.48 % | 0.00 % | 107 ms, 225 at most |

Senders under an upload condition, layers sent over time:

- peer-live-15fba96a-8: 180+360+720 from 8.0 s; 180+360 from 31.8 s; 180 from 33.0 s; 180+360 from 36.3 s; 180 from 38.5 s; 180+360 from 42.6 s; 180 from 44.8 s; 180+360 from 52.8 s; 180 from 54.8 s; 180+360 from 62.8 s; 180+360+720 from 67.9 s
- peer-live-15fba96a-9: 180+360+720 from 9.0 s; 180+360 from 31.5 s; 180 from 32.6 s; 180+360 from 34.9 s; 180 from 36.6 s; 180+360 from 40.7 s; 180 from 42.6 s; 180+360 from 50.7 s; 180 from 52.7 s; 180+360 from 60.7 s; 180+360+720 from 65.9 s

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slowup (2 bots) | 720: 99.58 %, 360: 0.28 %, 180: 0.14 % | 99.15 % | 0 | 4.37 |
| no condition (8 bots) | 720: 99.87 %, 360: 0.07 %, 180: 0.07 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-15fba96a-0 | 4.37 |
| peer-live-15fba96a-1 | 4.37 |
| peer-live-15fba96a-2 | 4.37 |
| peer-live-15fba96a-3 | 4.37 |
| peer-live-15fba96a-4 | 4.37 |
| peer-live-15fba96a-5 | 4.37 |
| peer-live-15fba96a-6 | 4.37 |
| peer-live-15fba96a-7 | 4.37 |
| peer-live-15fba96a-8 | 4.37 |
| peer-live-15fba96a-9 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-15fba96a-2 | 2.5 ms |
| peer-live-15fba96a-5 | 2.5 ms |
| peer-live-15fba96a-7 | 2.5 ms |
| peer-live-15fba96a-8 | 2.5 ms |
| peer-live-15fba96a-9 | 2.5 ms |
| peer-live-15fba96a-0 | 1.5 ms |
| peer-live-15fba96a-1 | 1.5 ms |
| peer-live-15fba96a-3 | 1.5 ms |
| peer-live-15fba96a-4 | 1.5 ms |
| peer-live-15fba96a-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-15fba96a-9 | 99.15 % |
| peer-live-15fba96a-2 | 99.47 % |
| peer-live-15fba96a-3 | 99.73 % |
| peer-live-15fba96a-1 | 99.74 % |
| peer-live-15fba96a-0 | 100.00 % |
| peer-live-15fba96a-4 | 100.00 % |
| peer-live-15fba96a-5 | 100.00 % |
| peer-live-15fba96a-6 | 100.00 % |
| peer-live-15fba96a-7 | 100.00 % |
| peer-live-15fba96a-8 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-us4b",
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
