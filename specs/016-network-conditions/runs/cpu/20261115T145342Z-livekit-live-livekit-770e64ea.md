# Load test report: live-livekit

livekit · run 770e64ea · 2026-11-15 14:51:16 → 2026-11-15 14:53:42 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.776 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.36, worst bot 1.33 |
| Audio concealed | p95 91.70 % |
| Video freezes | 8502 in total; freeze time p95 695.561 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 6.5 ms |
| Server CPU (total) | avg 4.71 threads, peak 5.39 threads |
| Server memory (total) | peak 1116.6 MiB |
| Engine CPU / memory | avg 4.51 threads, peak 4.89 threads · peak 1033.6 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.36 | 4.36 | 4.36 |
| Concealed | 0.09 % | 91.70 % | 92.26 % |
| Latency | 4.5 ms | 194.5 ms | 195.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 695.561 s | 784.948 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 6.5 ms | 10.5 ms | 10.5 ms |

- Freezes: 8502
- Resolution switches: 0
- Keyframe requests sent: 7984, received: 4518

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:51:16 to 2026-11-15 14:53:42 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.55 threads | 4.71 threads | 5.39 threads | 15.6 MiB | 1116.6 MiB |
| **total** | 1 measured | 31 / 0 | 1.55 threads | 4.71 threads | 5.39 threads | 15.6 MiB | 1116.6 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.51 threads | 4.89 threads | 1033.6 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:51:16 to 2026-11-15 14:53:42 (UTC).

## Network conditions

Seed 5741751997888338451 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| both | 10 | 0 | down: 500 kbit/s, queue 200 ms | 30.0–145.8 s (during start-up) | 500 kbit/s, 501 at most over 10 s | 93.79 % | 0.00 % | 202 ms, 220 at most |
| both | 10 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–145.8 s (during start-up) | 400 kbit/s, 401 at most over 10 s | 70.18 % | 0.00 % | 201 ms, 225 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| both | 0 | 10 of 10 | 2.0 s, 2.0 s | step open to the end | | 91.93 % |

<details><summary>Per bot</summary>

- peer-live-d4253bc8-90, step 0: reaction 2.0 s
- peer-live-d4253bc8-91, step 0: reaction 2.0 s
- peer-live-d4253bc8-92, step 0: reaction 2.0 s
- peer-live-d4253bc8-93, step 0: reaction 2.0 s
- peer-live-d4253bc8-94, step 0: reaction 2.0 s
- peer-live-d4253bc8-95, step 0: reaction 2.0 s
- peer-live-d4253bc8-96, step 0: reaction 2.0 s
- peer-live-d4253bc8-97, step 0: reaction 2.0 s
- peer-live-d4253bc8-98, step 0: reaction 2.0 s
- peer-live-d4253bc8-99, step 0: reaction 2.0 s

</details>

Senders under an upload condition, layers sent over time:

- peer-live-d4253bc8-90: does not adapt (cannot_adapt)
- peer-live-d4253bc8-91: does not adapt (cannot_adapt)
- peer-live-d4253bc8-92: does not adapt (cannot_adapt)
- peer-live-d4253bc8-93: does not adapt (cannot_adapt)
- peer-live-d4253bc8-94: does not adapt (cannot_adapt)
- peer-live-d4253bc8-95: does not adapt (cannot_adapt)
- peer-live-d4253bc8-96: does not adapt (cannot_adapt)
- peer-live-d4253bc8-97: does not adapt (cannot_adapt)
- peer-live-d4253bc8-98: does not adapt (cannot_adapt)
- peer-live-d4253bc8-99: does not adapt (cannot_adapt)

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| both (10 bots) | 360: 100.00 % | 100.00 % | 8502 | 1.37 |
| no condition (90 bots) | 360: 100.00 % | 100.00 % | 0 | 4.36 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-d4253bc8-96 | 1.33 |
| peer-live-d4253bc8-99 | 1.33 |
| peer-live-d4253bc8-97 | 1.37 |
| peer-live-d4253bc8-92 | 1.37 |
| peer-live-d4253bc8-93 | 1.37 |
| peer-live-d4253bc8-91 | 1.38 |
| peer-live-d4253bc8-90 | 1.38 |
| peer-live-d4253bc8-98 | 1.38 |
| peer-live-d4253bc8-95 | 1.39 |
| peer-live-d4253bc8-94 | 1.41 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-d4253bc8-95 | 835.386 s |
| peer-live-d4253bc8-94 | 784.948 s |
| peer-live-d4253bc8-99 | 759.800 s |
| peer-live-d4253bc8-93 | 754.780 s |
| peer-live-d4253bc8-92 | 745.358 s |
| peer-live-d4253bc8-90 | 695.561 s |
| peer-live-d4253bc8-97 | 673.606 s |
| peer-live-d4253bc8-96 | 650.402 s |
| peer-live-d4253bc8-91 | 643.348 s |
| peer-live-d4253bc8-98 | 608.068 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-d4253bc8-98 | 395.0 ms |
| peer-live-d4253bc8-94 | 240.5 ms |
| peer-live-d4253bc8-97 | 226.5 ms |
| peer-live-d4253bc8-99 | 225.5 ms |
| peer-live-d4253bc8-95 | 224.5 ms |
| peer-live-d4253bc8-96 | 224.5 ms |
| peer-live-d4253bc8-91 | 223.5 ms |
| peer-live-d4253bc8-93 | 223.5 ms |
| peer-live-d4253bc8-90 | 221.5 ms |
| peer-live-d4253bc8-92 | 221.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-d4253bc8-0 | 100.00 % |
| peer-live-d4253bc8-1 | 100.00 % |
| peer-live-d4253bc8-10 | 100.00 % |
| peer-live-d4253bc8-11 | 100.00 % |
| peer-live-d4253bc8-12 | 100.00 % |
| peer-live-d4253bc8-13 | 100.00 % |
| peer-live-d4253bc8-14 | 100.00 % |
| peer-live-d4253bc8-15 | 100.00 % |
| peer-live-d4253bc8-16 | 100.00 % |
| peer-live-d4253bc8-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-shaped-1",
    "url": "ws://localhost:7880"
  },
  "media": {
    "profile": "audio_video",
    "stall_timeout_seconds": 10,
    "max_received_streams": 10
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
        "name": "both",
        "count": 10,
        "schedule": [
          {
            "start_seconds": 30,
            "downlink": {
              "rate_kbps": 500,
              "queue_ms": 200
            },
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
