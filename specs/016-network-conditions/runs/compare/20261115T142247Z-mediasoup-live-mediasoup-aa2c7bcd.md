# Load test report: live-mediasoup

mediasoup · run aa2c7bcd · 2026-11-15 14:21:00 → 2026-11-15 14:22:47 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 107.028 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.45 %, 360: 0.55 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.25 threads, peak 0.29 threads |
| Server memory (total) | peak 89.8 MiB |
| Engine CPU / memory | avg 0.70 threads, peak 0.82 threads · peak 179.3 MiB · 16 hardware threads |

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
| Requested-height share | 99.26 % | 100.00 % | 100.00 % |
| Latency | 2.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 0
- Resolution switches: 12
- Keyframe requests sent: 0, received: 116

| Height | Share of video time |
|---|---|
| 720 | 99.45 % |
| 360 | 0.55 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.10 % | 0.11 % | 0.11 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:21:00 to 2026-11-15 14:22:47 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.04 threads | 0.25 threads | 0.29 threads | 86.9 MiB | 89.8 MiB |
| **total** | 1 measured | 23 / 0 | 0.04 threads | 0.25 threads | 0.29 threads | 86.9 MiB | 89.8 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.70 threads | 0.82 threads | 179.3 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:21:00 to 2026-11-15 14:22:47 (UTC).

## Network conditions

Seed 16468019555756326667 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slowup | 2 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–60.0 s | 351 kbit/s, 392 at most over 10 s | 72.43 % | 0.00 % | 116 ms, 225 at most |

Senders under an upload condition, layers sent over time:

- peer-live-131585f5-8: 180+360+720 from 16.0 s; 180+360 from 31.4 s; 180 from 32.5 s; 180+360 from 35.0 s; 180 from 37.4 s; 180+360 from 41.5 s; 180 from 43.4 s; 180+360 from 51.5 s; 180 from 53.4 s; 180+360 from 61.5 s; 180+360+720 from 66.6 s
- peer-live-131585f5-9: 180+360+720 from 18.0 s; 180+360 from 31.3 s; 180 from 32.4 s; 180+360 from 35.2 s; 180 from 37.3 s; 180+360 from 41.3 s; 180 from 43.3 s; 180+360 from 51.4 s; 180 from 53.3 s; 180+360 from 61.3 s; 180+360+720 from 66.5 s

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slowup | 0 | 8544 kbit/s (62 samples, 8217 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slowup (2 bots) | 720: 99.86 %, 360: 0.14 % | 99.72 % | 0 | 4.37 |
| no condition (8 bots) | 720: 99.36 %, 360: 0.64 % | 99.25 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-131585f5-9 | 4.37 |
| peer-live-131585f5-8 | 4.37 |
| peer-live-131585f5-2 | 4.37 |
| peer-live-131585f5-1 | 4.37 |
| peer-live-131585f5-0 | 4.37 |
| peer-live-131585f5-7 | 4.37 |
| peer-live-131585f5-6 | 4.37 |
| peer-live-131585f5-3 | 4.37 |
| peer-live-131585f5-5 | 4.37 |
| peer-live-131585f5-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-131585f5-0 | 6.5 ms |
| peer-live-131585f5-1 | 6.5 ms |
| peer-live-131585f5-2 | 6.5 ms |
| peer-live-131585f5-3 | 6.5 ms |
| peer-live-131585f5-4 | 5.5 ms |
| peer-live-131585f5-5 | 5.5 ms |
| peer-live-131585f5-6 | 5.5 ms |
| peer-live-131585f5-7 | 5.5 ms |
| peer-live-131585f5-8 | 5.5 ms |
| peer-live-131585f5-9 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-131585f5-6 | 98.94 % |
| peer-live-131585f5-5 | 98.96 % |
| peer-live-131585f5-3 | 99.24 % |
| peer-live-131585f5-0 | 99.25 % |
| peer-live-131585f5-1 | 99.26 % |
| peer-live-131585f5-2 | 99.50 % |
| peer-live-131585f5-8 | 99.72 % |
| peer-live-131585f5-4 | 99.75 % |
| peer-live-131585f5-7 | 100.00 % |
| peer-live-131585f5-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-slowup",
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
