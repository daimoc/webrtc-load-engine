# Load test report: live-livekit

livekit · run f378c892 · 2026-11-15 12:40:10 → 2026-11-15 12:41:20 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 69.885 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 50.25 %, 360: 0.04 %, 180: 49.71 % |
| Latency p50 (audio / video) | 0.5 ms / 0.5 ms |
| Server CPU (total) | not measured: not_requested |
| Server memory (total) | not measured: not_requested |
| Engine CPU / memory | avg 0.66 threads, peak 0.82 threads · peak 180.8 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 0.00 % | 0.00 % |
| Latency | 0.5 ms | 0.5 ms | 0.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 30.0 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 0.000 s | 0.000 s |
| Requested-height share | 99.27 % | 100.00 % | 100.00 % |
| Latency | 0.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 0
- Resolution switches: 32
- Keyframe requests sent: 0, received: 27

| Height | Share of video time |
|---|---|
| 720 | 50.25 % |
| 360 | 0.04 % |
| 180 | 49.71 % |

## Reception reports

Not measured: not_negotiated.

## Server

Server load not requested.

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.66 threads | 0.82 threads | 180.8 MiB | 15 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:40:10 to 2026-11-15 12:41:20 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-85981b0d-0 | 4.37 |
| peer-live-85981b0d-1 | 4.37 |
| peer-live-85981b0d-2 | 4.37 |
| peer-live-85981b0d-3 | 4.37 |
| peer-live-85981b0d-4 | 4.37 |
| peer-live-85981b0d-5 | 4.37 |
| peer-live-85981b0d-6 | 4.37 |
| peer-live-85981b0d-7 | 4.37 |
| peer-live-85981b0d-8 | 4.37 |
| peer-live-85981b0d-9 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-85981b0d-0 | 2.5 ms |
| peer-live-85981b0d-1 | 2.5 ms |
| peer-live-85981b0d-2 | 2.5 ms |
| peer-live-85981b0d-3 | 2.5 ms |
| peer-live-85981b0d-4 | 2.5 ms |
| peer-live-85981b0d-5 | 2.5 ms |
| peer-live-85981b0d-6 | 1.5 ms |
| peer-live-85981b0d-7 | 1.5 ms |
| peer-live-85981b0d-8 | 1.5 ms |
| peer-live-85981b0d-9 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-85981b0d-1 | 98.91 % |
| peer-live-85981b0d-0 | 99.27 % |
| peer-live-85981b0d-3 | 99.27 % |
| peer-live-85981b0d-4 | 99.27 % |
| peer-live-85981b0d-2 | 99.27 % |
| peer-live-85981b0d-9 | 99.27 % |
| peer-live-85981b0d-8 | 99.64 % |
| peer-live-85981b0d-6 | 99.64 % |
| peer-live-85981b0d-7 | 100.00 % |
| peer-live-85981b0d-5 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r14-vp8",
    "url": "ws://localhost:7880"
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
        "height": 180,
        "share": 0.5
      },
      {
        "height": 720,
        "share": 0.5
      }
    ],
    "video_codec": "vp8"
  }
}
```

Engine devel (go1.27.2)
