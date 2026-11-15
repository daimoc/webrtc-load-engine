# Load test report: live-livekit

livekit · run 2be69a47 · 2026-11-15 14:41:08 → 2026-11-15 14:43:33 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.388 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.36, worst bot 4.35 |
| Audio concealed | p95 0.13 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 6.5 ms |
| Server CPU (total) | avg 5.00 threads, peak 5.49 threads |
| Server memory (total) | peak 1027.9 MiB |
| Engine CPU / memory | avg 4.32 threads, peak 5.40 threads · peak 960.1 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.36 | 4.36 | 4.36 |
| Concealed | 0.09 % | 0.13 % | 0.13 % |
| Latency | 4.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 6.5 ms | 8.5 ms | 8.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 52, received: 859

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:41:08 to 2026-11-15 14:43:33 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.58 threads | 5.00 threads | 5.49 threads | 16.0 MiB | 1027.9 MiB |
| **total** | 1 measured | 31 / 0 | 1.58 threads | 5.00 threads | 5.49 threads | 16.0 MiB | 1027.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.32 threads | 5.40 threads | 960.1 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:41:08 to 2026-11-15 14:43:33 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-a6cf808d-79 | 4.35 |
| peer-live-a6cf808d-90 | 4.35 |
| peer-live-a6cf808d-83 | 4.35 |
| peer-live-a6cf808d-29 | 4.35 |
| peer-live-a6cf808d-68 | 4.35 |
| peer-live-a6cf808d-93 | 4.35 |
| peer-live-a6cf808d-88 | 4.35 |
| peer-live-a6cf808d-87 | 4.35 |
| peer-live-a6cf808d-45 | 4.36 |
| peer-live-a6cf808d-64 | 4.36 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-a6cf808d-50 | 16.5 ms |
| peer-live-a6cf808d-59 | 16.5 ms |
| peer-live-a6cf808d-60 | 16.5 ms |
| peer-live-a6cf808d-61 | 16.5 ms |
| peer-live-a6cf808d-62 | 16.5 ms |
| peer-live-a6cf808d-63 | 16.5 ms |
| peer-live-a6cf808d-65 | 16.5 ms |
| peer-live-a6cf808d-66 | 16.5 ms |
| peer-live-a6cf808d-67 | 16.5 ms |
| peer-live-a6cf808d-69 | 16.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-a6cf808d-0 | 100.00 % |
| peer-live-a6cf808d-1 | 100.00 % |
| peer-live-a6cf808d-10 | 100.00 % |
| peer-live-a6cf808d-11 | 100.00 % |
| peer-live-a6cf808d-12 | 100.00 % |
| peer-live-a6cf808d-13 | 100.00 % |
| peer-live-a6cf808d-14 | 100.00 % |
| peer-live-a6cf808d-15 | 100.00 % |
| peer-live-a6cf808d-16 | 100.00 % |
| peer-live-a6cf808d-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-before-2",
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
