# Load test report: live-livekit

livekit · run 5d8fc3bb · 2026-11-15 14:43:40 → 2026-11-15 14:46:05 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 100 (bots measured: 100) |
| Test duration | 3600 s (ramp-up 20 s); run 145.430 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 100 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.35, worst bot 4.34 |
| Audio concealed | p95 0.24 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 260.9 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 4.5 ms / 7.5 ms |
| Server CPU (total) | avg 5.09 threads, peak 5.75 threads |
| Server memory (total) | peak 1125.3 MiB |
| Engine CPU / memory | avg 4.49 threads, peak 4.93 threads · peak 964.9 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.35 | 4.36 | 4.36 |
| Concealed | 0.18 % | 0.24 % | 0.25 % |
| Latency | 4.5 ms | 5.5 ms | 5.5 ms |

## Video

Bots receiving video: 100.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 260.9 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 7.5 ms | 9.5 ms | 9.5 ms |

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 440, received: 995

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Sampled every 5 s from 2026-11-15 14:43:40 to 2026-11-15 14:46:05 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| livekit-server | measured | 31 / 0 | 1.54 threads | 5.09 threads | 5.75 threads | 15.3 MiB | 1125.3 MiB |
| **total** | 1 measured | 31 / 0 | 1.54 threads | 5.09 threads | 5.75 threads | 15.3 MiB | 1125.3 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 4.49 threads | 4.93 threads | 964.9 MiB | 31 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 14:43:40 to 2026-11-15 14:46:05 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-96452f96-99 | 4.34 |
| peer-live-96452f96-92 | 4.34 |
| peer-live-96452f96-90 | 4.34 |
| peer-live-96452f96-22 | 4.34 |
| peer-live-96452f96-93 | 4.34 |
| peer-live-96452f96-30 | 4.34 |
| peer-live-96452f96-89 | 4.34 |
| peer-live-96452f96-56 | 4.34 |
| peer-live-96452f96-54 | 4.34 |
| peer-live-96452f96-34 | 4.34 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-96452f96-62 | 17.5 ms |
| peer-live-96452f96-68 | 17.5 ms |
| peer-live-96452f96-74 | 17.5 ms |
| peer-live-96452f96-76 | 17.5 ms |
| peer-live-96452f96-82 | 17.5 ms |
| peer-live-96452f96-83 | 17.5 ms |
| peer-live-96452f96-84 | 17.5 ms |
| peer-live-96452f96-87 | 17.5 ms |
| peer-live-96452f96-88 | 17.5 ms |
| peer-live-96452f96-89 | 17.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-96452f96-0 | 100.00 % |
| peer-live-96452f96-1 | 100.00 % |
| peer-live-96452f96-10 | 100.00 % |
| peer-live-96452f96-11 | 100.00 % |
| peer-live-96452f96-12 | 100.00 % |
| peer-live-96452f96-13 | 100.00 % |
| peer-live-96452f96-14 | 100.00 % |
| peer-live-96452f96-15 | 100.00 % |
| peer-live-96452f96-16 | 100.00 % |
| peer-live-96452f96-17 | 100.00 % |

## Test plan

```json
{
  "participants": 100,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "cpu-after-2",
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
