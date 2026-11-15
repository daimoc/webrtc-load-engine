# Load test report: live-livekit

livekit · run 274bd748 · 2026-11-15 13:00:57 → 2026-11-15 13:02:35 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 97.901 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 3.88 |
| Audio concealed | p95 7.88 % |
| Video freezes | 37 in total; freeze time p95 7.130 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 82.72 %, 360: 5.24 %, 180: 12.04 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | avg 0.61 threads, peak 0.66 threads |
| Server memory (total) | peak 100.1 MiB |
| Engine CPU / memory | avg 0.80 threads, peak 0.86 threads · peak 206.5 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 7.88 % | 7.88 % |
| Latency | 0.5 ms | 0.5 ms | 0.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 7.130 s | 7.130 s |
| Requested-height share | 99.73 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 37
- Resolution switches: 54
- Keyframe requests sent: 24, received: 92

| Height | Share of video time |
|---|---|
| 720 | 82.72 % |
| 360 | 5.24 % |
| 180 | 12.04 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 13:00:57 to 2026-11-15 13:02:35 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 21 / 0 | 0.34 threads | 0.61 threads | 0.66 threads | 16.9 MiB | 100.1 MiB |
| **total** | 1 measured | 21 / 0 | 0.34 threads | 0.61 threads | 0.66 threads | 16.9 MiB | 100.1 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.80 threads | 0.86 threads | 206.5 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:00:57 to 2026-11-15 13:02:35 (UTC).

## Network conditions

Seed 14995045329630375807 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 1071 kbit/s, 1119 at most over 10 s | 3.74 % | 0.00 % | 57 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) |
|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 2 of 3 | 32.0 s, 34.0 s |

<details><summary>Per bot</summary>

- peer-live-7d3095d7-7, step 0: reaction 2.0 s, not recovered (not_recovered_before_end)
- peer-live-7d3095d7-8, step 0: reaction 2.0 s, recovery 32.0 s
- peer-live-7d3095d7-9, step 0: reaction 2.0 s, recovery 34.0 s

</details>

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 40.92 %, 360: 17.69 %, 180: 41.39 % | 46.87 % | 37 | 3.88 |
| no condition (7 bots) | 720: 99.81 %, 360: 0.15 %, 180: 0.04 % | 99.73 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-7d3095d7-8 | 3.88 |
| peer-live-7d3095d7-9 | 3.88 |
| peer-live-7d3095d7-7 | 3.89 |
| peer-live-7d3095d7-0 | 4.37 |
| peer-live-7d3095d7-1 | 4.37 |
| peer-live-7d3095d7-2 | 4.37 |
| peer-live-7d3095d7-3 | 4.37 |
| peer-live-7d3095d7-4 | 4.37 |
| peer-live-7d3095d7-5 | 4.37 |
| peer-live-7d3095d7-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-7d3095d7-7 | 7.130 s |
| peer-live-7d3095d7-8 | 7.109 s |
| peer-live-7d3095d7-9 | 6.855 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-7d3095d7-9 | 88.5 ms |
| peer-live-7d3095d7-8 | 86.5 ms |
| peer-live-7d3095d7-7 | 83.5 ms |
| peer-live-7d3095d7-5 | 2.5 ms |
| peer-live-7d3095d7-0 | 1.5 ms |
| peer-live-7d3095d7-1 | 1.5 ms |
| peer-live-7d3095d7-2 | 1.5 ms |
| peer-live-7d3095d7-3 | 1.5 ms |
| peer-live-7d3095d7-4 | 1.5 ms |
| peer-live-7d3095d7-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-7d3095d7-7 | 28.34 % |
| peer-live-7d3095d7-9 | 46.87 % |
| peer-live-7d3095d7-8 | 47.75 % |
| peer-live-7d3095d7-1 | 99.47 % |
| peer-live-7d3095d7-4 | 99.73 % |
| peer-live-7d3095d7-2 | 99.73 % |
| peer-live-7d3095d7-0 | 99.73 % |
| peer-live-7d3095d7-3 | 100.00 % |
| peer-live-7d3095d7-5 | 100.00 % |
| peer-live-7d3095d7-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-us2",
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
