# Load test report: live-mediasoup

mediasoup · run 6bd49c4c · 2026-11-15 14:19:34 → 2026-11-15 14:20:59 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 85.016 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.09 |
| Audio concealed | p95 2.55 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 97.05 %, 360: 2.61 %, 180: 0.34 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.25 threads, peak 0.29 threads |
| Server memory (total) | peak 87.7 MiB |
| Engine CPU / memory | avg 0.72 threads, peak 1.47 threads · peak 200.4 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 2.55 % | 2.55 % |
| Latency | 0.5 ms | 40.5 ms | 40.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 96.48 % | 99.67 % | 99.67 % |
| Latency | 2.5 ms | 41.5 ms | 41.5 ms |

- Freezes: 0
- Resolution switches: 34
- Keyframe requests sent: 637, received: 358

| Height | Share of video time |
|---|---|
| 720 | 97.05 % |
| 360 | 2.61 % |
| 180 | 0.34 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.15 % | 0.15 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 14:19:34 to 2026-11-15 14:20:59 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.05 threads | 0.25 threads | 0.29 threads | 86.5 MiB | 87.7 MiB |
| **total** | 1 measured | 19 / 0 | 0.05 threads | 0.25 threads | 0.29 threads | 86.5 MiB | 87.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.72 threads | 1.47 threads | 200.4 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:19:34 to 2026-11-15 14:20:59 (UTC).

## Network conditions

Seed 7 (test plan), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| lossy | 3 | 0 | down: delay 40 ± 10 ms, loss 3.00 % | 30.0–85.0 s | 7401 kbit/s, 7906 at most over 10 s | 0.00 % | 2.99 %, bursts of 1.0 | 40 ms, 50 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| lossy | 0 | 3 of 3 | 4.0 s, 6.0 s | step open to the end | | 0.00 % |

<details><summary>Per bot</summary>

- peer-live-d26c2d6d-7, step 0: reaction 4.0 s
- peer-live-d26c2d6d-8, step 0: reaction 4.0 s
- peer-live-d26c2d6d-9, step 0: reaction 6.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| lossy | 0 | 7164 kbit/s (165 samples, 5306 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| lossy (3 bots) | 720: 95.59 %, 360: 4.41 % | 95.84 % | 0 | 4.11 |
| no condition (7 bots) | 720: 97.61 %, 360: 1.92 %, 180: 0.47 % | 97.57 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-d26c2d6d-8 | 4.09 |
| peer-live-d26c2d6d-9 | 4.11 |
| peer-live-d26c2d6d-7 | 4.12 |
| peer-live-d26c2d6d-2 | 4.37 |
| peer-live-d26c2d6d-1 | 4.37 |
| peer-live-d26c2d6d-6 | 4.37 |
| peer-live-d26c2d6d-0 | 4.37 |
| peer-live-d26c2d6d-3 | 4.37 |
| peer-live-d26c2d6d-5 | 4.37 |
| peer-live-d26c2d6d-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-d26c2d6d-7 | 52.5 ms |
| peer-live-d26c2d6d-8 | 52.5 ms |
| peer-live-d26c2d6d-9 | 52.5 ms |
| peer-live-d26c2d6d-0 | 6.5 ms |
| peer-live-d26c2d6d-1 | 6.5 ms |
| peer-live-d26c2d6d-2 | 6.5 ms |
| peer-live-d26c2d6d-3 | 6.5 ms |
| peer-live-d26c2d6d-4 | 5.5 ms |
| peer-live-d26c2d6d-5 | 5.5 ms |
| peer-live-d26c2d6d-6 | 5.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-d26c2d6d-7 | 94.65 % |
| peer-live-d26c2d6d-0 | 94.87 % |
| peer-live-d26c2d6d-9 | 95.84 % |
| peer-live-d26c2d6d-8 | 96.32 % |
| peer-live-d26c2d6d-1 | 96.48 % |
| peer-live-d26c2d6d-2 | 96.80 % |
| peer-live-d26c2d6d-6 | 97.57 % |
| peer-live-d26c2d6d-5 | 98.65 % |
| peer-live-d26c2d6d-3 | 99.35 % |
| peer-live-d26c2d6d-4 | 99.67 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-lossy",
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
