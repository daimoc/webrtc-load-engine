# Load test report: live-livekit

livekit · run 3a32c351 · 2026-11-15 14:48:44 → 2026-11-15 14:51:10 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.933 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.35, worst bot 4.34 |
| Audio concealed | p95 0.21 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 7.5 ms |
| Server CPU (total) | avg 5.12 threads, peak 5.62 threads |
| Server memory (total) | peak 1086.1 MiB |
| Engine CPU / memory | avg 4.50 threads, peak 4.88 threads · peak 958.6 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.35 | 4.36 | 4.36 |
| Concealed | 0.15 % | 0.21 % | 0.23 % |
| Latency | 4.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 7.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 83, received: 873

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:48:44 to 2026-11-15 14:51:10 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.53 threads | 5.12 threads | 5.62 threads | 16.1 MiB | 1086.1 MiB |
| **total** | 1 measured | 31 / 0 | 1.53 threads | 5.12 threads | 5.62 threads | 16.1 MiB | 1086.1 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.50 threads | 4.88 threads | 958.6 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:48:44 to 2026-11-15 14:51:10 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-5f65ffc3-20 | 4.34 |
| peer-live-5f65ffc3-73 | 4.34 |
| peer-live-5f65ffc3-28 | 4.35 |
| peer-live-5f65ffc3-37 | 4.35 |
| peer-live-5f65ffc3-50 | 4.35 |
| peer-live-5f65ffc3-86 | 4.35 |
| peer-live-5f65ffc3-91 | 4.35 |
| peer-live-5f65ffc3-54 | 4.35 |
| peer-live-5f65ffc3-69 | 4.35 |
| peer-live-5f65ffc3-62 | 4.35 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-5f65ffc3-34 | 17.5 ms |
| peer-live-5f65ffc3-47 | 17.5 ms |
| peer-live-5f65ffc3-48 | 17.5 ms |
| peer-live-5f65ffc3-51 | 17.5 ms |
| peer-live-5f65ffc3-60 | 17.5 ms |
| peer-live-5f65ffc3-66 | 17.5 ms |
| peer-live-5f65ffc3-67 | 17.5 ms |
| peer-live-5f65ffc3-68 | 17.5 ms |
| peer-live-5f65ffc3-69 | 17.5 ms |
| peer-live-5f65ffc3-70 | 17.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-5f65ffc3-0 | 100.00 % |
| peer-live-5f65ffc3-1 | 100.00 % |
| peer-live-5f65ffc3-10 | 100.00 % |
| peer-live-5f65ffc3-11 | 100.00 % |
| peer-live-5f65ffc3-12 | 100.00 % |
| peer-live-5f65ffc3-13 | 100.00 % |
| peer-live-5f65ffc3-14 | 100.00 % |
| peer-live-5f65ffc3-15 | 100.00 % |
| peer-live-5f65ffc3-16 | 100.00 % |
| peer-live-5f65ffc3-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-after-3",
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
