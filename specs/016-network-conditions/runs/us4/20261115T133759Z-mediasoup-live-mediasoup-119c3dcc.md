# Load test report: live-mediasoup

mediasoup · run 119c3dcc · 2026-11-15 13:36:14 → 2026-11-15 13:37:59 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 105.062 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.44 %, 360: 0.56 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.25 threads, peak 0.41 threads |
| Server memory (total) | peak 58.7 MiB |
| Engine CPU / memory | avg 0.67 threads, peak 1.31 threads · peak 181.2 MiB · 16 hardware threads |

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
| Requested-height share | 99.23 % | 100.00 % | 100.00 % |
| Latency | 2.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 0
- Resolution switches: 12
- Keyframe requests sent: 0, received: 123

| Height | Share of video time |
|---|---|
| 720 | 99.44 % |
| 360 | 0.56 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.10 % | 0.12 % | 0.12 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 13:36:14 to 2026-11-15 13:37:59 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.07 threads | 0.25 threads | 0.41 threads | 37.6 MiB | 58.7 MiB |
| **total** | 1 measured | 23 / 0 | 0.07 threads | 0.25 threads | 0.41 threads | 37.6 MiB | 58.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.67 threads | 1.31 threads | 181.2 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:36:14 to 2026-11-15 13:37:59 (UTC).

## Network conditions

Seed 9184458041645154352 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slowup | 2 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–60.0 s | 353 kbit/s, 391 at most over 10 s | 72.93 % | 0.00 % | 120 ms, 225 at most |

Senders under an upload condition, layers sent over time:

- peer-live-8a3d1778-8: 180+360+720 from 16.0 s; 180+360 from 31.3 s; 180 from 32.4 s; 180+360 from 36.2 s; 180 from 38.3 s; 180+360 from 42.4 s; 180 from 44.3 s; 180+360 from 52.3 s; 180 from 54.4 s; 180+360 from 62.5 s; 180+360+720 from 67.6 s
- peer-live-8a3d1778-9: 180+360+720 from 18.0 s; 180+360 from 31.4 s; 180 from 32.6 s; 180+360 from 35.3 s; 180 from 37.4 s; 180+360 from 41.4 s; 180 from 43.3 s; 180+360 from 51.4 s; 180 from 53.4 s; 180+360 from 61.5 s; 180+360+720 from 66.6 s

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slowup (2 bots) | 720: 100.00 % | 100.00 % | 0 | 4.37 |
| no condition (8 bots) | 720: 99.31 %, 360: 0.69 % | 99.22 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-8a3d1778-9 | 4.37 |
| peer-live-8a3d1778-8 | 4.37 |
| peer-live-8a3d1778-2 | 4.37 |
| peer-live-8a3d1778-1 | 4.37 |
| peer-live-8a3d1778-0 | 4.37 |
| peer-live-8a3d1778-7 | 4.37 |
| peer-live-8a3d1778-6 | 4.37 |
| peer-live-8a3d1778-3 | 4.37 |
| peer-live-8a3d1778-5 | 4.37 |
| peer-live-8a3d1778-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-8a3d1778-0 | 6.5 ms |
| peer-live-8a3d1778-1 | 6.5 ms |
| peer-live-8a3d1778-2 | 6.5 ms |
| peer-live-8a3d1778-3 | 6.5 ms |
| peer-live-8a3d1778-4 | 5.5 ms |
| peer-live-8a3d1778-5 | 5.5 ms |
| peer-live-8a3d1778-6 | 5.5 ms |
| peer-live-8a3d1778-7 | 5.5 ms |
| peer-live-8a3d1778-8 | 5.5 ms |
| peer-live-8a3d1778-9 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-8a3d1778-5 | 98.94 % |
| peer-live-8a3d1778-2 | 98.98 % |
| peer-live-8a3d1778-6 | 99.19 % |
| peer-live-8a3d1778-4 | 99.22 % |
| peer-live-8a3d1778-3 | 99.23 % |
| peer-live-8a3d1778-1 | 99.49 % |
| peer-live-8a3d1778-7 | 99.72 % |
| peer-live-8a3d1778-0 | 99.75 % |
| peer-live-8a3d1778-8 | 100.00 % |
| peer-live-8a3d1778-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-us4b",
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
