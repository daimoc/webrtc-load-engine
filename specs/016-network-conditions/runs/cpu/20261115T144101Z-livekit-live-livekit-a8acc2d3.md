# Load test report: live-livekit

livekit · run a8acc2d3 · 2026-11-15 14:38:36 → 2026-11-15 14:41:01 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.415 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.35, worst bot 4.35 |
| Audio concealed | p95 0.19 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 90.3 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 7.5 ms |
| Server CPU (total) | avg 5.12 threads, peak 5.81 threads |
| Server memory (total) | peak 1110.5 MiB |
| Engine CPU / memory | avg 4.50 threads, peak 4.89 threads · peak 952.9 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.35 | 4.36 | 4.36 |
| Concealed | 0.13 % | 0.19 % | 0.21 % |
| Latency | 4.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 90.3 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 7.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 142, received: 946

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:38:36 to 2026-11-15 14:41:01 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.56 threads | 5.12 threads | 5.81 threads | 17.0 MiB | 1110.5 MiB |
| **total** | 1 measured | 31 / 0 | 1.56 threads | 5.12 threads | 5.81 threads | 17.0 MiB | 1110.5 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.50 threads | 4.89 threads | 952.9 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:38:36 to 2026-11-15 14:41:01 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-2283963a-65 | 4.35 |
| peer-live-2283963a-76 | 4.35 |
| peer-live-2283963a-94 | 4.35 |
| peer-live-2283963a-50 | 4.35 |
| peer-live-2283963a-99 | 4.35 |
| peer-live-2283963a-85 | 4.35 |
| peer-live-2283963a-62 | 4.35 |
| peer-live-2283963a-56 | 4.35 |
| peer-live-2283963a-88 | 4.35 |
| peer-live-2283963a-78 | 4.35 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-2283963a-57 | 17.5 ms |
| peer-live-2283963a-60 | 17.5 ms |
| peer-live-2283963a-61 | 17.5 ms |
| peer-live-2283963a-64 | 17.5 ms |
| peer-live-2283963a-70 | 17.5 ms |
| peer-live-2283963a-71 | 17.5 ms |
| peer-live-2283963a-72 | 17.5 ms |
| peer-live-2283963a-74 | 17.5 ms |
| peer-live-2283963a-75 | 17.5 ms |
| peer-live-2283963a-78 | 17.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-2283963a-0 | 100.00 % |
| peer-live-2283963a-1 | 100.00 % |
| peer-live-2283963a-10 | 100.00 % |
| peer-live-2283963a-11 | 100.00 % |
| peer-live-2283963a-12 | 100.00 % |
| peer-live-2283963a-13 | 100.00 % |
| peer-live-2283963a-14 | 100.00 % |
| peer-live-2283963a-15 | 100.00 % |
| peer-live-2283963a-16 | 100.00 % |
| peer-live-2283963a-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-after-1",
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
  }
}
```

Engine devel (go1.27.2)
