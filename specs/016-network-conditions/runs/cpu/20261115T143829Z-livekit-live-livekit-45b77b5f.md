# Load test report: live-livekit

livekit · run 45b77b5f · 2026-11-15 14:36:04 → 2026-11-15 14:38:29 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.531 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.36, worst bot 4.35 |
| Audio concealed | p95 0.17 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 6.5 ms |
| Server CPU (total) | avg 4.94 threads, peak 5.60 threads |
| Server memory (total) | peak 964.7 MiB |
| Engine CPU / memory | avg 4.29 threads, peak 4.71 threads · peak 944.7 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.36 | 4.36 | 4.36 |
| Concealed | 0.12 % | 0.17 % | 0.19 % |
| Latency | 4.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 6.5 ms | 8.5 ms | 9.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 11, received: 840

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:36:04 to 2026-11-15 14:38:29 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.63 threads | 4.94 threads | 5.60 threads | 16.0 MiB | 964.7 MiB |
| **total** | 1 measured | 31 / 0 | 1.63 threads | 4.94 threads | 5.60 threads | 16.0 MiB | 964.7 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.29 threads | 4.71 threads | 944.7 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:36:04 to 2026-11-15 14:38:29 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-275ee479-72 | 4.35 |
| peer-live-275ee479-78 | 4.35 |
| peer-live-275ee479-67 | 4.35 |
| peer-live-275ee479-74 | 4.35 |
| peer-live-275ee479-86 | 4.35 |
| peer-live-275ee479-50 | 4.35 |
| peer-live-275ee479-44 | 4.35 |
| peer-live-275ee479-0 | 4.35 |
| peer-live-275ee479-62 | 4.35 |
| peer-live-275ee479-45 | 4.35 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-275ee479-84 | 17.5 ms |
| peer-live-275ee479-88 | 17.5 ms |
| peer-live-275ee479-92 | 17.5 ms |
| peer-live-275ee479-94 | 17.5 ms |
| peer-live-275ee479-97 | 17.5 ms |
| peer-live-275ee479-98 | 17.5 ms |
| peer-live-275ee479-30 | 16.5 ms |
| peer-live-275ee479-31 | 16.5 ms |
| peer-live-275ee479-52 | 16.5 ms |
| peer-live-275ee479-53 | 16.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-275ee479-0 | 100.00 % |
| peer-live-275ee479-1 | 100.00 % |
| peer-live-275ee479-10 | 100.00 % |
| peer-live-275ee479-11 | 100.00 % |
| peer-live-275ee479-12 | 100.00 % |
| peer-live-275ee479-13 | 100.00 % |
| peer-live-275ee479-14 | 100.00 % |
| peer-live-275ee479-15 | 100.00 % |
| peer-live-275ee479-16 | 100.00 % |
| peer-live-275ee479-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-before-1",
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
