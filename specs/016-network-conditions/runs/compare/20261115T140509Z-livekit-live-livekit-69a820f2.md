# Load test report: live-livekit

livekit · run 69a820f2 · 2026-11-15 14:03:31 → 2026-11-15 14:05:09 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 97.920 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 3.87 |
| Audio concealed | p95 7.99 % |
| Video freezes | 37 in total; freeze time p95 7.001 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 83.10 %, 360: 6.11 %, 180: 10.78 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | avg 0.60 threads, peak 0.64 threads |
| Server memory (total) | peak 133.9 MiB |
| Engine CPU / memory | avg 0.79 threads, peak 0.87 threads · peak 209.1 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 7.99 % | 7.99 % |
| Latency | 0.5 ms | 0.5 ms | 0.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 7.001 s | 7.001 s |
| Requested-height share | 99.73 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 37
- Resolution switches: 55
- Keyframe requests sent: 27, received: 104

| Height | Share of video time |
|---|---|
| 720 | 83.10 % |
| 360 | 6.11 % |
| 180 | 10.78 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:03:31 to 2026-11-15 14:05:09 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 21 / 0 | 0.31 threads | 0.60 threads | 0.64 threads | 119.2 MiB | 133.9 MiB |
| **total** | 1 measured | 21 / 0 | 0.31 threads | 0.60 threads | 0.64 threads | 119.2 MiB | 133.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.79 threads | 0.87 threads | 209.1 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:03:31 to 2026-11-15 14:05:09 (UTC).

## Network conditions

Seed 344013542211951370 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 1071 kbit/s, 1120 at most over 10 s | 3.96 % | 0.00 % | 57 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 2 of 3 | 32.0 s, 32.0 s | 14.65 % |

<details><summary>Per bot</summary>

- peer-live-357a16ae-7, step 0: reaction 2.0 s, recovery 32.0 s
- peer-live-357a16ae-8, step 0: reaction 2.0 s, recovery 32.0 s
- peer-live-357a16ae-9, step 0: reaction 2.0 s, not recovered (not_recovered_before_end)

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 42.25 %, 360: 20.59 %, 180: 37.16 % | 49.46 % | 37 | 3.88 |
| no condition (7 bots) | 720: 99.81 %, 360: 0.19 % | 99.74 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-357a16ae-9 | 3.87 |
| peer-live-357a16ae-8 | 3.88 |
| peer-live-357a16ae-7 | 3.89 |
| peer-live-357a16ae-0 | 4.37 |
| peer-live-357a16ae-1 | 4.37 |
| peer-live-357a16ae-2 | 4.37 |
| peer-live-357a16ae-3 | 4.37 |
| peer-live-357a16ae-4 | 4.37 |
| peer-live-357a16ae-5 | 4.37 |
| peer-live-357a16ae-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-357a16ae-8 | 7.001 s |
| peer-live-357a16ae-7 | 6.849 s |
| peer-live-357a16ae-9 | 6.531 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-357a16ae-8 | 87.5 ms |
| peer-live-357a16ae-9 | 84.5 ms |
| peer-live-357a16ae-7 | 83.5 ms |
| peer-live-357a16ae-5 | 2.5 ms |
| peer-live-357a16ae-0 | 1.5 ms |
| peer-live-357a16ae-1 | 1.5 ms |
| peer-live-357a16ae-2 | 1.5 ms |
| peer-live-357a16ae-3 | 1.5 ms |
| peer-live-357a16ae-4 | 1.5 ms |
| peer-live-357a16ae-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-357a16ae-9 | 27.01 % |
| peer-live-357a16ae-7 | 49.46 % |
| peer-live-357a16ae-8 | 50.02 % |
| peer-live-357a16ae-5 | 99.73 % |
| peer-live-357a16ae-3 | 99.73 % |
| peer-live-357a16ae-2 | 99.73 % |
| peer-live-357a16ae-0 | 99.74 % |
| peer-live-357a16ae-1 | 99.74 % |
| peer-live-357a16ae-4 | 100.00 % |
| peer-live-357a16ae-6 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-change2",
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
