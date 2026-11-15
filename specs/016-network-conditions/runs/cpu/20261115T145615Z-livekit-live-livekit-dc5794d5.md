# Load test report: live-livekit

livekit · run dc5794d5 · 2026-11-15 14:53:49 → 2026-11-15 14:56:15 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.860 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.36, worst bot 1.32 |
| Audio concealed | p95 91.68 % |
| Video freezes | 8773 in total; freeze time p95 718.874 s |
| Video frame rate | 30.0 / 30.0 / 31.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 3.5 ms / 5.5 ms |
| Server CPU (total) | avg 4.62 threads, peak 5.42 threads |
| Server memory (total) | peak 1122.0 MiB |
| Engine CPU / memory | avg 4.39 threads, peak 4.83 threads · peak 1029.5 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.36 | 4.36 | 4.36 |
| Concealed | 0.07 % | 91.68 % | 92.27 % |
| Latency | 3.5 ms | 195.5 ms | 196.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 31.0 fps |
| Freeze time | 0.000 s | 718.874 s | 785.869 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 5.5 ms | 9.5 ms | 10.5 ms |

- Freezes: 8773
- Resolution switches: 0
- Keyframe requests sent: 8079, received: 3940

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:53:49 to 2026-11-15 14:56:15 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.57 threads | 4.62 threads | 5.42 threads | 16.0 MiB | 1122.0 MiB |
| **total** | 1 measured | 31 / 0 | 1.57 threads | 4.62 threads | 5.42 threads | 16.0 MiB | 1122.0 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.39 threads | 4.83 threads | 1029.5 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:53:49 to 2026-11-15 14:56:15 (UTC).

## Network conditions

Seed 16623669773195092501 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| both | 10 | 0 | down: 500 kbit/s, queue 200 ms | 30.0–145.8 s (during start-up) | 500 kbit/s, 501 at most over 10 s | 93.58 % | 0.00 % | 202 ms, 220 at most |
| both | 10 | 0 | up: 400 kbit/s, queue 200 ms | 30.0–145.8 s (during start-up) | 400 kbit/s, 401 at most over 10 s | 69.19 % | 0.00 % | 201 ms, 225 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| both | 0 | 10 of 10 | 2.0 s, 2.0 s | step open to the end | | 92.05 % |

<details><summary>Per bot</summary>

- peer-live-49833f64-90, step 0: reaction 2.0 s
- peer-live-49833f64-91, step 0: reaction 2.0 s
- peer-live-49833f64-92, step 0: reaction 2.0 s
- peer-live-49833f64-93, step 0: reaction 2.0 s
- peer-live-49833f64-94, step 0: reaction 2.0 s
- peer-live-49833f64-95, step 0: reaction 2.0 s
- peer-live-49833f64-96, step 0: reaction 2.0 s
- peer-live-49833f64-97, step 0: reaction 2.0 s
- peer-live-49833f64-98, step 0: reaction 2.0 s
- peer-live-49833f64-99, step 0: reaction 2.0 s

</details>

Senders under an upload condition, layers sent over time:

- peer-live-49833f64-90: does not adapt (cannot_adapt)
- peer-live-49833f64-91: does not adapt (cannot_adapt)
- peer-live-49833f64-92: does not adapt (cannot_adapt)
- peer-live-49833f64-93: does not adapt (cannot_adapt)
- peer-live-49833f64-94: does not adapt (cannot_adapt)
- peer-live-49833f64-95: does not adapt (cannot_adapt)
- peer-live-49833f64-96: does not adapt (cannot_adapt)
- peer-live-49833f64-97: does not adapt (cannot_adapt)
- peer-live-49833f64-98: does not adapt (cannot_adapt)
- peer-live-49833f64-99: does not adapt (cannot_adapt)

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| both (10 bots) | 360: 100.00 % | 100.00 % | 8773 | 1.37 |
| no condition (90 bots) | 360: 100.00 % | 100.00 % | 0 | 4.36 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-49833f64-97 | 1.32 |
| peer-live-49833f64-93 | 1.32 |
| peer-live-49833f64-99 | 1.35 |
| peer-live-49833f64-94 | 1.37 |
| peer-live-49833f64-91 | 1.37 |
| peer-live-49833f64-90 | 1.37 |
| peer-live-49833f64-92 | 1.37 |
| peer-live-49833f64-98 | 1.37 |
| peer-live-49833f64-95 | 1.42 |
| peer-live-49833f64-96 | 1.42 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-49833f64-95 | 794.828 s |
| peer-live-49833f64-99 | 785.869 s |
| peer-live-49833f64-91 | 764.195 s |
| peer-live-49833f64-94 | 737.376 s |
| peer-live-49833f64-97 | 732.065 s |
| peer-live-49833f64-90 | 718.874 s |
| peer-live-49833f64-96 | 715.855 s |
| peer-live-49833f64-93 | 681.592 s |
| peer-live-49833f64-92 | 636.745 s |
| peer-live-49833f64-98 | 628.111 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-49833f64-96 | 395.0 ms |
| peer-live-49833f64-98 | 395.0 ms |
| peer-live-49833f64-99 | 395.0 ms |
| peer-live-49833f64-95 | 224.5 ms |
| peer-live-49833f64-97 | 222.5 ms |
| peer-live-49833f64-90 | 221.5 ms |
| peer-live-49833f64-91 | 221.5 ms |
| peer-live-49833f64-92 | 221.5 ms |
| peer-live-49833f64-94 | 221.5 ms |
| peer-live-49833f64-93 | 220.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-49833f64-0 | 100.00 % |
| peer-live-49833f64-1 | 100.00 % |
| peer-live-49833f64-10 | 100.00 % |
| peer-live-49833f64-11 | 100.00 % |
| peer-live-49833f64-12 | 100.00 % |
| peer-live-49833f64-13 | 100.00 % |
| peer-live-49833f64-14 | 100.00 % |
| peer-live-49833f64-15 | 100.00 % |
| peer-live-49833f64-16 | 100.00 % |
| peer-live-49833f64-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-shaped-2",
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
