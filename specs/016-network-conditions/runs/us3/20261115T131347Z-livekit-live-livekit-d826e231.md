# Load test report: live-livekit

livekit · run d826e231 · 2026-11-15 13:12:29 → 2026-11-15 13:13:47 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 77.876 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.14 |
| Audio concealed | p95 2.18 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 99.79 %, 360: 0.21 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.64 threads, peak 0.69 threads |
| Server memory (total) | peak 94.7 MiB |
| Engine CPU / memory | avg 0.87 threads, peak 0.99 threads · peak 207.2 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 2.18 % | 2.18 % |
| Latency | 0.5 ms | 43.5 ms | 43.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 45.5 ms | 45.5 ms |

- Freezes: 0
- Resolution switches: 6
- Keyframe requests sent: 561, received: 272

| Height | Share of video time |
|---|---|
| 720 | 99.79 % |
| 360 | 0.21 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 13:12:29 to 2026-11-15 13:13:47 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 17 / 0 | 0.32 threads | 0.64 threads | 0.69 threads | 16.3 MiB | 94.7 MiB |
| **total** | 1 measured | 17 / 0 | 0.32 threads | 0.64 threads | 0.69 threads | 16.3 MiB | 94.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.87 threads | 0.99 threads | 207.2 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:12:29 to 2026-11-15 13:13:47 (UTC).

## Network conditions

Seed 7 (test plan), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| lossy | 3 | 0 | down: delay 40 ± 10 ms, loss 3.00 % | 30.0–77.9 s | 6924 kbit/s, 7052 at most over 10 s | 0.00 % | 3.01 %, bursts of 1.0 | 47 ms, 50 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| lossy | 0 | 0 of 3 |  | step open to the end | | 0.00 % |

<details><summary>Per bot</summary>

- peer-live-befc9f01-7, step 0: no reaction (not_lowered)
- peer-live-befc9f01-8, step 0: no reaction (not_lowered)
- peer-live-befc9f01-9, step 0: no reaction (not_lowered)

</details>

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| lossy (3 bots) | 720: 99.76 %, 360: 0.24 % | 100.00 % | 0 | 4.15 |
| no condition (7 bots) | 720: 99.80 %, 360: 0.20 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-befc9f01-7 | 4.14 |
| peer-live-befc9f01-9 | 4.15 |
| peer-live-befc9f01-8 | 4.16 |
| peer-live-befc9f01-0 | 4.37 |
| peer-live-befc9f01-1 | 4.37 |
| peer-live-befc9f01-2 | 4.37 |
| peer-live-befc9f01-3 | 4.37 |
| peer-live-befc9f01-4 | 4.37 |
| peer-live-befc9f01-5 | 4.37 |
| peer-live-befc9f01-6 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-befc9f01-7 | 51.5 ms |
| peer-live-befc9f01-8 | 51.5 ms |
| peer-live-befc9f01-9 | 51.5 ms |
| peer-live-befc9f01-1 | 2.5 ms |
| peer-live-befc9f01-6 | 2.5 ms |
| peer-live-befc9f01-0 | 1.5 ms |
| peer-live-befc9f01-2 | 1.5 ms |
| peer-live-befc9f01-3 | 1.5 ms |
| peer-live-befc9f01-4 | 1.5 ms |
| peer-live-befc9f01-5 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-befc9f01-9 | 99.26 % |
| peer-live-befc9f01-0 | 99.33 % |
| peer-live-befc9f01-4 | 99.66 % |
| peer-live-befc9f01-1 | 99.66 % |
| peer-live-befc9f01-2 | 100.00 % |
| peer-live-befc9f01-3 | 100.00 % |
| peer-live-befc9f01-5 | 100.00 % |
| peer-live-befc9f01-6 | 100.00 % |
| peer-live-befc9f01-7 | 100.00 % |
| peer-live-befc9f01-8 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r-us3",
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
