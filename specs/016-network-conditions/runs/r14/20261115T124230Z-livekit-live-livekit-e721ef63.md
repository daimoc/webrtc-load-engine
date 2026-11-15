# Load test report: live-livekit

livekit · run e721ef63 · 2026-11-15 12:41:20 → 2026-11-15 12:42:30 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | livekit |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 69.890 s |
| Outcome | stopped |
| Video codec | vp9 (svc: 180, 360, 720) · received vp9 by 10 bots |
| Video content bitrate | 180: 168 kbit/s, 360: 750 kbit/s, 720: 2062 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_negotiated |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 50.00 %, 180: 50.00 % |
| Latency p50 (audio / video) | 0.5 ms / not measured: frame_not_identified |
| Server CPU (total) | not measured: not_requested |
| Server memory (total) | not measured: not_requested |
| Engine CPU / memory | avg 0.77 threads, peak 0.87 threads · peak 116.4 MiB · 16 hardware threads |

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
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |

Latency not measured: frame_not_identified.

- Freezes: 0
- Resolution switches: 0
- Keyframe requests sent: 0, received: 10

| Height | Share of video time |
|---|---|
| 720 | 50.00 % |
| 180 | 50.00 % |

## Reception reports

Not measured: not_negotiated.

## Server

Server load not requested.

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.77 threads | 0.87 threads | 116.4 MiB | 15 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 12:41:20 to 2026-11-15 12:42:30 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-e597dd79-0 | 4.37 |
| peer-live-e597dd79-1 | 4.37 |
| peer-live-e597dd79-2 | 4.37 |
| peer-live-e597dd79-3 | 4.37 |
| peer-live-e597dd79-4 | 4.37 |
| peer-live-e597dd79-5 | 4.37 |
| peer-live-e597dd79-6 | 4.37 |
| peer-live-e597dd79-7 | 4.37 |
| peer-live-e597dd79-8 | 4.37 |
| peer-live-e597dd79-9 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-e597dd79-1 | 1.5 ms |
| peer-live-e597dd79-3 | 1.5 ms |
| peer-live-e597dd79-4 | 1.5 ms |
| peer-live-e597dd79-5 | 1.5 ms |
| peer-live-e597dd79-6 | 1.5 ms |
| peer-live-e597dd79-7 | 1.5 ms |
| peer-live-e597dd79-9 | 1.5 ms |
| peer-live-e597dd79-0 | 0.5 ms |
| peer-live-e597dd79-2 | 0.5 ms |
| peer-live-e597dd79-8 | 0.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-e597dd79-0 | 100.00 % |
| peer-live-e597dd79-1 | 100.00 % |
| peer-live-e597dd79-2 | 100.00 % |
| peer-live-e597dd79-3 | 100.00 % |
| peer-live-e597dd79-4 | 100.00 % |
| peer-live-e597dd79-5 | 100.00 % |
| peer-live-e597dd79-6 | 100.00 % |
| peer-live-e597dd79-7 | 100.00 % |
| peer-live-e597dd79-8 | 100.00 % |
| peer-live-e597dd79-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "api_key": "devkey",
    "api_secret": "[redacted]",
    "room": "r14-vp9",
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
    "video_codec": "vp9"
  }
}
```

Engine devel (go1.27.2)
