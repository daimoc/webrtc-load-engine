# Load test report: live-livekit

livekit · run 4c109cc7 · 2026-11-15 14:46:12 → 2026-11-15 14:48:38 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 146.000 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.36, worst bot 4.35 |
| Audio concealed | p95 0.13 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 31.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 6.5 ms |
| Server CPU (total) | avg 5.01 threads, peak 5.64 threads |
| Server memory (total) | peak 978.9 MiB |
| Engine CPU / memory | avg 4.31 threads, peak 4.67 threads · peak 937.9 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.36 | 4.36 | 4.36 |
| Concealed | 0.10 % | 0.13 % | 0.15 % |
| Latency | 4.5 ms | 4.5 ms | 4.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 31.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 6.5 ms | 8.5 ms | 8.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 4, received: 862

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:46:12 to 2026-11-15 14:48:38 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.51 threads | 5.01 threads | 5.64 threads | 16.5 MiB | 978.9 MiB |
| **total** | 1 measured | 31 / 0 | 1.51 threads | 5.01 threads | 5.64 threads | 16.5 MiB | 978.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.31 threads | 4.67 threads | 937.9 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:46:12 to 2026-11-15 14:48:38 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-7e7181b7-55 | 4.35 |
| peer-live-7e7181b7-69 | 4.35 |
| peer-live-7e7181b7-77 | 4.35 |
| peer-live-7e7181b7-72 | 4.35 |
| peer-live-7e7181b7-66 | 4.35 |
| peer-live-7e7181b7-82 | 4.35 |
| peer-live-7e7181b7-86 | 4.35 |
| peer-live-7e7181b7-99 | 4.35 |
| peer-live-7e7181b7-91 | 4.35 |
| peer-live-7e7181b7-57 | 4.35 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-7e7181b7-97 | 17.5 ms |
| peer-live-7e7181b7-51 | 16.5 ms |
| peer-live-7e7181b7-62 | 16.5 ms |
| peer-live-7e7181b7-63 | 16.5 ms |
| peer-live-7e7181b7-68 | 16.5 ms |
| peer-live-7e7181b7-69 | 16.5 ms |
| peer-live-7e7181b7-70 | 16.5 ms |
| peer-live-7e7181b7-71 | 16.5 ms |
| peer-live-7e7181b7-72 | 16.5 ms |
| peer-live-7e7181b7-73 | 16.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-7e7181b7-0 | 100.00 % |
| peer-live-7e7181b7-1 | 100.00 % |
| peer-live-7e7181b7-10 | 100.00 % |
| peer-live-7e7181b7-11 | 100.00 % |
| peer-live-7e7181b7-12 | 100.00 % |
| peer-live-7e7181b7-13 | 100.00 % |
| peer-live-7e7181b7-14 | 100.00 % |
| peer-live-7e7181b7-15 | 100.00 % |
| peer-live-7e7181b7-16 | 100.00 % |
| peer-live-7e7181b7-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-before-3",
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
