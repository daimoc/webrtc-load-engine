# Load test report: live-livekit

livekit · run 2110a8a7 · 2026-11-15 14:06:49 → 2026-11-15 14:08:07 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.878 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.14 |
| Audio concealed | p95 2.20 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.72 %, 360: 0.10 %, 180: 0.17 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.64 threads, peak 0.71 threads |
| Server memory (total) | peak 136.7 MiB |
| Engine CPU / memory | avg 0.87 threads, peak 0.96 threads · peak 206.5 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 2.20 % | 2.20 % |
| Latency | 0.5 ms | 37.5 ms | 37.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 99.66 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 38.5 ms | 38.5 ms |

- Freezes: 0
- Resolution switches: 9
- Keyframe requests sent: 557, received: 405

| Height | Share of video time |
|---|---|
| 720 | 99.72 % |
| 360 | 0.10 % |
| 180 | 0.17 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:06:49 to 2026-11-15 14:08:07 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.28 threads | 0.64 threads | 0.71 threads | 127.0 MiB | 136.7 MiB |
| **total** | 1 measured | 17 / 0 | 0.28 threads | 0.64 threads | 0.71 threads | 127.0 MiB | 136.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.87 threads | 0.96 threads | 206.5 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:06:49 to 2026-11-15 14:08:07 (UTC).

## Network conditions

Seed 7 (test plan), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| lossy | 3 | 0 | down: delay 40 ± 10 ms, loss 3.00 % | 30.0–77.9 s | 6916 kbit/s, 7025 at most over 10 s | 0.00 % | 2.98 %, bursts of 1.0 | 40 ms, 50 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| lossy | 0 | 0 of 3 |  | step open to the end | | 0.00 % |

<details><summary>Per bot</summary>

- peer-live-78447123-7, step 0: no reaction (not_lowered)
- peer-live-78447123-8, step 0: no reaction (not_lowered)
- peer-live-78447123-9, step 0: no reaction (not_lowered)

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| lossy (3 bots) | 720: 99.52 %, 360: 0.24 %, 180: 0.24 % | 99.29 % | 0 | 4.15 |
| no condition (7 bots) | 720: 99.80 %, 360: 0.05 %, 180: 0.15 % | 99.66 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-78447123-9 | 4.14 |
| peer-live-78447123-8 | 4.15 |
| peer-live-78447123-7 | 4.17 |
| peer-live-78447123-0 | 4.37 |
| peer-live-78447123-1 | 4.37 |
| peer-live-78447123-2 | 4.37 |
| peer-live-78447123-3 | 4.37 |
| peer-live-78447123-4 | 4.37 |
| peer-live-78447123-5 | 4.37 |
| peer-live-78447123-6 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-78447123-7 | 50.5 ms |
| peer-live-78447123-8 | 50.5 ms |
| peer-live-78447123-9 | 50.5 ms |
| peer-live-78447123-0 | 2.5 ms |
| peer-live-78447123-5 | 2.5 ms |
| peer-live-78447123-1 | 1.5 ms |
| peer-live-78447123-2 | 1.5 ms |
| peer-live-78447123-3 | 1.5 ms |
| peer-live-78447123-4 | 1.5 ms |
| peer-live-78447123-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-78447123-9 | 99.26 % |
| peer-live-78447123-7 | 99.29 % |
| peer-live-78447123-3 | 99.66 % |
| peer-live-78447123-2 | 99.66 % |
| peer-live-78447123-1 | 99.66 % |
| peer-live-78447123-0 | 99.66 % |
| peer-live-78447123-4 | 100.00 % |
| peer-live-78447123-5 | 100.00 % |
| peer-live-78447123-6 | 100.00 % |
| peer-live-78447123-8 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-lossy",
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
    "seed": 7,
    "groups": [
      {
        "name": "lossy",
        "count": 3,
        "schedule": [
          {
            "start_seconds": 30,
            "downlink": {
              "delay_ms": 40,
              "jitter_ms": 10,
              "loss": {
                "share": 0.03,
                "mean_burst": 1
              }
            }
          }
        ]
      }
    ]
  }
}
```

Engine devel (go1.27.2)
