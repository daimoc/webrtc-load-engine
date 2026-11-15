# Load test report: live-janus

janus · run 8ca20bcd · 2026-11-15 13:46:04 → 2026-11-15 13:47:22 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | janus |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 78.064 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 2.09 |
| Audio concealed | p95 69.60 % |
| Video freezes | 328 in total; freeze time p95 67.949 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 100.00 % |
| Latency p50 (audio / video) | not measured: frame_not_identified / 1.5 ms |
| Server CPU (total) | avg 0.80 threads, peak 0.89 threads |
| Server memory (total) | peak 15.9 MiB |
| Engine CPU / memory | avg 0.72 threads, peak 0.81 threads · peak 194.1 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 69.60 % | 69.60 % |

Latency not measured: frame_not_identified.

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 67.949 s | 67.949 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 328
- Resolution switches: 0
- Keyframe requests sent: 498, received: 695

| Height | Share of video time |
|---|---|
| 720 | 100.00 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.00 % | 0.00 % | 0.00 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 13:46:04 to 2026-11-15 13:47:22 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| janus | measured | 17 / 0 | 0.15 threads | 0.80 threads | 0.89 threads | 4.4 MiB | 15.9 MiB |
| **total** | 1 measured | 17 / 0 | 0.15 threads | 0.80 threads | 0.89 threads | 4.4 MiB | 15.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.72 threads | 0.81 threads | 194.1 MiB | 17 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:46:04 to 2026-11-15 13:47:22 (UTC).

## Network conditions

Seed 12710032839628108106 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–78.1 s | 1200 kbit/s, 1201 at most over 10 s | 71.40 % | 0.00 % | 192 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | step open to the end | | 82.42 % |

<details><summary>Per bot</summary>

- peer-live-c2cd8ca5-7, step 0: reaction 2.0 s
- peer-live-c2cd8ca5-8, step 0: reaction 2.0 s
- peer-live-c2cd8ca5-9, step 0: reaction 2.0 s

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 100.00 % | 100.00 % | 328 | 2.09 |
| no condition (7 bots) | 720: 100.00 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-c2cd8ca5-8 | 2.09 |
| peer-live-c2cd8ca5-9 | 2.09 |
| peer-live-c2cd8ca5-7 | 2.15 |
| peer-live-c2cd8ca5-0 | 4.37 |
| peer-live-c2cd8ca5-1 | 4.37 |
| peer-live-c2cd8ca5-2 | 4.37 |
| peer-live-c2cd8ca5-4 | 4.37 |
| peer-live-c2cd8ca5-6 | 4.37 |
| peer-live-c2cd8ca5-5 | 4.37 |
| peer-live-c2cd8ca5-3 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-c2cd8ca5-8 | 67.949 s |
| peer-live-c2cd8ca5-7 | 67.204 s |
| peer-live-c2cd8ca5-9 | 62.594 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-c2cd8ca5-7 | 3.5 ms |
| peer-live-c2cd8ca5-8 | 3.5 ms |
| peer-live-c2cd8ca5-9 | 3.5 ms |
| peer-live-c2cd8ca5-0 | 2.5 ms |
| peer-live-c2cd8ca5-1 | 2.5 ms |
| peer-live-c2cd8ca5-2 | 2.5 ms |
| peer-live-c2cd8ca5-3 | 2.5 ms |
| peer-live-c2cd8ca5-4 | 2.5 ms |
| peer-live-c2cd8ca5-5 | 2.5 ms |
| peer-live-c2cd8ca5-6 | 2.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-c2cd8ca5-0 | 100.00 % |
| peer-live-c2cd8ca5-1 | 100.00 % |
| peer-live-c2cd8ca5-2 | 100.00 % |
| peer-live-c2cd8ca5-3 | 100.00 % |
| peer-live-c2cd8ca5-4 | 100.00 % |
| peer-live-c2cd8ca5-5 | 100.00 % |
| peer-live-c2cd8ca5-6 | 100.00 % |
| peer-live-c2cd8ca5-7 | 100.00 % |
| peer-live-c2cd8ca5-8 | 100.00 % |
| peer-live-c2cd8ca5-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "1722",
    "url": "ws://localhost:8188"
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
      "janus"
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
