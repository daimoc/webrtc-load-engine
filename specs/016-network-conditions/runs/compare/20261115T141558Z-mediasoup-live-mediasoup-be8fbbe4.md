# Load test report: live-mediasoup

mediasoup · run be8fbbe4 · 2026-11-15 14:14:10 → 2026-11-15 14:15:58 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 107.069 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.07 |
| Audio concealed | p95 7.76 % |
| Video freezes | 109 in total; freeze time p95 40.762 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 93.49 %, 360: 1.26 %, 180: 5.25 % |
| Latency p50 (audio / video) | 0.5 ms / 2.5 ms |
| Server CPU (total) | avg 0.23 threads, peak 0.30 threads |
| Server memory (total) | peak 86.7 MiB |
| Engine CPU / memory | avg 0.68 threads, peak 0.78 threads · peak 197.8 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 7.76 % | 7.76 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 40.762 s | 40.762 s |
| Requested-height share | 99.00 % | 99.75 % | 99.75 % |
| Latency | 2.5 ms | 4.5 ms | 4.5 ms |

- Freezes: 109
- Resolution switches: 33
- Keyframe requests sent: 64, received: 171

| Height | Share of video time |
|---|---|
| 720 | 93.49 % |
| 360 | 1.26 % |
| 180 | 5.25 % |

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

Sampled every 5 s from 2026-11-15 14:14:10 to 2026-11-15 14:15:58 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 23 / 0 | 0.05 threads | 0.23 threads | 0.30 threads | 82.1 MiB | 86.7 MiB |
| **total** | 1 measured | 23 / 0 | 0.05 threads | 0.23 threads | 0.30 threads | 82.1 MiB | 86.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.68 threads | 0.78 threads | 197.8 MiB | 23 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:14:10 to 2026-11-15 14:15:58 (UTC).

## Network conditions

Seed 796277038232195914 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 771 kbit/s, 1012 at most over 10 s | 53.00 % | 0.00 % | 68 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 1 of 3 | 4.0 s, 4.0 s | 30.13 % |

<details><summary>Per bot</summary>

- peer-live-958eea1e-7, step 0: reaction 2.0 s, not recovered (not_recovered_before_end)
- peer-live-958eea1e-8, step 0: reaction 2.0 s, not recovered (not_recovered_before_end)
- peer-live-958eea1e-9, step 0: reaction 2.0 s, recovery 4.0 s

</details>

| Group | Step | Server's estimate toward the group's bots (median) |
|---|---|---|
| slow | 0 | 283 kbit/s (63 samples, 240 to 8582) |

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 68.64 %, 360: 3.56 %, 180: 27.80 % | 50.51 % | 109 | 4.10 |
| no condition (7 bots) | 720: 99.27 %, 360: 0.73 % | 99.25 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-958eea1e-9 | 4.07 |
| peer-live-958eea1e-7 | 4.10 |
| peer-live-958eea1e-8 | 4.10 |
| peer-live-958eea1e-2 | 4.37 |
| peer-live-958eea1e-1 | 4.37 |
| peer-live-958eea1e-0 | 4.37 |
| peer-live-958eea1e-6 | 4.37 |
| peer-live-958eea1e-3 | 4.37 |
| peer-live-958eea1e-5 | 4.37 |
| peer-live-958eea1e-4 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-958eea1e-9 | 40.762 s |
| peer-live-958eea1e-8 | 11.413 s |
| peer-live-958eea1e-7 | 11.165 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-958eea1e-9 | 17.5 ms |
| peer-live-958eea1e-7 | 11.5 ms |
| peer-live-958eea1e-8 | 11.5 ms |
| peer-live-958eea1e-0 | 5.5 ms |
| peer-live-958eea1e-1 | 5.5 ms |
| peer-live-958eea1e-2 | 5.5 ms |
| peer-live-958eea1e-3 | 5.5 ms |
| peer-live-958eea1e-4 | 4.5 ms |
| peer-live-958eea1e-5 | 4.5 ms |
| peer-live-958eea1e-6 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-958eea1e-8 | 47.61 % |
| peer-live-958eea1e-7 | 50.51 % |
| peer-live-958eea1e-9 | 85.65 % |
| peer-live-958eea1e-5 | 98.44 % |
| peer-live-958eea1e-0 | 99.00 % |
| peer-live-958eea1e-3 | 99.24 % |
| peer-live-958eea1e-2 | 99.25 % |
| peer-live-958eea1e-1 | 99.50 % |
| peer-live-958eea1e-6 | 99.74 % |
| peer-live-958eea1e-4 | 99.75 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-change1",
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
