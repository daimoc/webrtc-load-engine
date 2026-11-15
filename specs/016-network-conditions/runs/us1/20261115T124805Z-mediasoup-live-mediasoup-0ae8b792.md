# Load test report: live-mediasoup

mediasoup · run 0ae8b792 · 2026-11-15 12:46:40 → 2026-11-15 12:48:05 (UTC) · stopped

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
| Audio MOS | median 4.37, worst bot 3.47 |
| Audio concealed | p95 19.41 % |
| Video freezes | 135 in total; freeze time p95 77.698 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 87.71 %, 360: 1.57 %, 180: 10.72 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.24 threads, peak 1.28 threads |
| Server memory (total) | peak 59.9 MiB |
| Engine CPU / memory | avg 0.62 threads, peak 3.72 threads · peak 195.7 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 19.41 % | 19.41 % |
| Latency | 0.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 77.698 s | 77.698 s |
| Requested-height share | 98.65 % | 100.00 % | 100.00 % |
| Latency | 2.5 ms | 11.5 ms | 11.5 ms |

- Freezes: 135
- Resolution switches: 30
- Keyframe requests sent: 63, received: 168

| Height | Share of video time |
|---|---|
| 720 | 87.71 % |
| 360 | 1.57 % |
| 180 | 10.72 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.50 % | 0.50 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 12:46:40 to 2026-11-15 12:48:05 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.07 threads | 0.24 threads | 1.28 threads | 36.4 MiB | 59.9 MiB |
| **total** | 1 measured | 19 / 0 | 0.07 threads | 0.24 threads | 1.28 threads | 36.4 MiB | 59.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.62 threads | 3.72 threads | 195.7 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:46:40 to 2026-11-15 12:48:05 (UTC).

## Network conditions

Seed 5116445819804703123 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–85.0 s | 729 kbit/s, 1200 at most over 10 s | 35.60 % | 0.00 % | 44 ms, 208 at most |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 44.65 %, 360: 3.68 %, 180: 51.68 % | 43.67 % | 132 | 3.64 |
| no condition (7 bots) | 720: 98.93 %, 360: 1.03 %, 180: 0.05 % | 99.05 % | 3 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-27ca9555-9 | 3.47 |
| peer-live-27ca9555-8 | 3.64 |
| peer-live-27ca9555-7 | 3.86 |
| peer-live-27ca9555-2 | 4.37 |
| peer-live-27ca9555-6 | 4.37 |
| peer-live-27ca9555-3 | 4.37 |
| peer-live-27ca9555-5 | 4.37 |
| peer-live-27ca9555-0 | 4.37 |
| peer-live-27ca9555-1 | 4.37 |
| peer-live-27ca9555-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-27ca9555-8 | 77.698 s |
| peer-live-27ca9555-9 | 66.786 s |
| peer-live-27ca9555-7 | 12.846 s |
| peer-live-27ca9555-1 | 2.214 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-27ca9555-7 | 202.5 ms |
| peer-live-27ca9555-9 | 199.5 ms |
| peer-live-27ca9555-8 | 198.5 ms |
| peer-live-27ca9555-0 | 5.5 ms |
| peer-live-27ca9555-1 | 5.5 ms |
| peer-live-27ca9555-2 | 5.5 ms |
| peer-live-27ca9555-3 | 5.5 ms |
| peer-live-27ca9555-4 | 4.5 ms |
| peer-live-27ca9555-5 | 4.5 ms |
| peer-live-27ca9555-6 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-27ca9555-9 | 27.48 % |
| peer-live-27ca9555-8 | 43.67 % |
| peer-live-27ca9555-7 | 96.71 % |
| peer-live-27ca9555-1 | 97.46 % |
| peer-live-27ca9555-5 | 98.65 % |
| peer-live-27ca9555-3 | 99.04 % |
| peer-live-27ca9555-0 | 99.05 % |
| peer-live-27ca9555-2 | 99.05 % |
| peer-live-27ca9555-6 | 99.31 % |
| peer-live-27ca9555-4 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "us1-016",
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
    ]
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
