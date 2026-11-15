# Load test report: live-janus

janus · run 252aff5c · 2026-11-15 13:49:59 → 2026-11-15 13:51:36 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | janus |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 10 s); run 96.929 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 3.59 |
| Audio concealed | p95 23.17 % |
| Video freezes | 231 in total; freeze time p95 39.534 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 100.00 % |
| Latency p50 (audio / video) | not measured: frame_not_identified / 1.5 ms |
| Server CPU (total) | avg 0.87 threads, peak 0.96 threads |
| Server memory (total) | peak 20.9 MiB |
| Engine CPU / memory | avg 0.74 threads, peak 0.83 threads · peak 199.6 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 23.17 % | 23.17 % |

Latency not measured: frame_not_identified.

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 39.534 s | 39.534 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 231
- Resolution switches: 0
- Keyframe requests sent: 209, received: 315

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

Sampled every 5 s from 2026-11-15 13:49:59 to 2026-11-15 13:51:36 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| janus | measured | 21 / 0 | 0.20 threads | 0.87 threads | 0.96 threads | 16.3 MiB | 20.9 MiB |
| **total** | 1 measured | 21 / 0 | 0.20 threads | 0.87 threads | 0.96 threads | 16.3 MiB | 20.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.74 threads | 0.83 threads | 199.6 MiB | 21 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 13:49:59 to 2026-11-15 13:51:36 (UTC).

## Network conditions

Seed 1610579236416749813 (drawn by the engine), settling time 10 s, timeline resolution 2 s. Measures are the links' own counts, not the plan's values.

| Group | Bots | Step | Planned | In force | Delivered | Dropped by the cap | Dropped by loss | Added delay |
|---|---|---|---|---|---|---|---|---|
| slow | 3 | 0 | down: 1200 kbit/s, queue 200 ms | 30.0–50.0 s | 1200 kbit/s, 1201 at most over 10 s | 71.30 % | 0.00 % | 192 ms, 208 at most |

Reaction: from the start of a step to the first lower height or freeze on a stream that was at its requested height. Recovery: from its end until every such stream holds that height for 5 s without a freeze. Resolution 2 s.

| Group | Step | Bots that reacted | Reaction (median, longest) | Recovered | Recovery (median, longest) | Video still missing after retransmission (mean) |
|---|---|---|---|---|---|---|
| slow | 0 | 3 of 3 | 2.0 s, 2.0 s | 3 of 3 | 4.0 s, 4.0 s | 76.99 % |

<details><summary>Per bot</summary>

- peer-live-576be81b-7, step 0: reaction 2.0 s, recovery 4.0 s
- peer-live-576be81b-8, step 0: reaction 2.0 s, recovery 4.0 s
- peer-live-576be81b-9, step 0: reaction 2.0 s, recovery 4.0 s

</details>

Server's estimate toward the bots: not given by this server's signaling.

| Bots | Received heights | At the requested height (p50) | Freezes | Audio score (p50) |
|---|---|---|---|---|
| slow (3 bots) | 720: 100.00 % | 100.00 % | 231 | 3.61 |
| no condition (7 bots) | 720: 100.00 % | 100.00 % | 0 | 4.37 |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-576be81b-9 | 3.59 |
| peer-live-576be81b-7 | 3.61 |
| peer-live-576be81b-8 | 3.61 |
| peer-live-576be81b-0 | 4.37 |
| peer-live-576be81b-1 | 4.37 |
| peer-live-576be81b-5 | 4.37 |
| peer-live-576be81b-3 | 4.37 |
| peer-live-576be81b-2 | 4.37 |
| peer-live-576be81b-4 | 4.37 |
| peer-live-576be81b-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-576be81b-7 | 39.534 s |
| peer-live-576be81b-8 | 38.610 s |
| peer-live-576be81b-9 | 33.439 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-576be81b-7 | 3.5 ms |
| peer-live-576be81b-8 | 3.5 ms |
| peer-live-576be81b-9 | 3.5 ms |
| peer-live-576be81b-0 | 2.5 ms |
| peer-live-576be81b-1 | 2.5 ms |
| peer-live-576be81b-2 | 2.5 ms |
| peer-live-576be81b-3 | 2.5 ms |
| peer-live-576be81b-4 | 2.5 ms |
| peer-live-576be81b-5 | 2.5 ms |
| peer-live-576be81b-6 | 2.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-576be81b-0 | 100.00 % |
| peer-live-576be81b-1 | 100.00 % |
| peer-live-576be81b-2 | 100.00 % |
| peer-live-576be81b-3 | 100.00 % |
| peer-live-576be81b-4 | 100.00 % |
| peer-live-576be81b-5 | 100.00 % |
| peer-live-576be81b-6 | 100.00 % |
| peer-live-576be81b-7 | 100.00 % |
| peer-live-576be81b-8 | 100.00 % |
| peer-live-576be81b-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 10,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "1833",
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
