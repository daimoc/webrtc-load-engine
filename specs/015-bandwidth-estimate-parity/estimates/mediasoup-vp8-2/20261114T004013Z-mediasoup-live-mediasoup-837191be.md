# Load test report: live-mediasoup

mediasoup · run 837191be · 2026-11-14 00:39:04 → 2026-11-14 00:40:13 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 69.104 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Browser | not measured: no_video_received |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 42.26 %, 360: 5.52 %, 180: 52.22 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.21 threads, peak 0.24 threads |
| Server memory (total) | peak 84.3 MiB |
| Engine CPU / memory | avg 0.56 threads, peak 0.60 threads · peak 178.1 MiB · 16 hardware threads |

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
| Latency | 1.5 ms | 2.5 ms | 2.5 ms |

- Freezes: 0
- Resolution switches: 16
- Keyframe requests sent: 0, received: 44

| Height | Share of video time |
|---|---|
| 720 | 42.26 % |
| 360 | 5.52 % |
| 180 | 52.22 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.15 % | 0.15 % | 0.15 % |
| Longest stamp lag | 0.9 ms | 1.5 ms | 1.5 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-14 00:39:04 to 2026-11-14 00:40:13 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 15 / 0 | 0.24 threads | 0.21 threads | 0.24 threads | 61.1 MiB | 84.3 MiB |
| **total** | 1 measured | 15 / 0 | 0.24 threads | 0.21 threads | 0.24 threads | 61.1 MiB | 84.3 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.56 threads | 0.60 threads | 178.1 MiB | 15 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-14 00:39:04 to 2026-11-14 00:40:13 (UTC).

## Browser

Address https://localhost:5555/?roomId=c015mvp82&produce=false&mic=false&webcam=false&displayName=browser, window 1280x720.

Not measured: no_video_received.

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-327e0954-0 | 4.37 |
| peer-live-327e0954-1 | 4.37 |
| peer-live-327e0954-3 | 4.37 |
| peer-live-327e0954-4 | 4.37 |
| peer-live-327e0954-9 | 4.37 |
| peer-live-327e0954-7 | 4.37 |
| peer-live-327e0954-8 | 4.37 |
| peer-live-327e0954-5 | 4.37 |
| peer-live-327e0954-6 | 4.37 |
| peer-live-327e0954-2 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-327e0954-0 | 5.5 ms |
| peer-live-327e0954-1 | 5.5 ms |
| peer-live-327e0954-3 | 5.5 ms |
| peer-live-327e0954-4 | 5.5 ms |
| peer-live-327e0954-2 | 4.5 ms |
| peer-live-327e0954-6 | 4.5 ms |
| peer-live-327e0954-7 | 4.5 ms |
| peer-live-327e0954-8 | 4.5 ms |
| peer-live-327e0954-9 | 4.5 ms |
| peer-live-327e0954-5 | 3.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-327e0954-6 | 60.47 % |
| peer-live-327e0954-8 | 60.84 % |
| peer-live-327e0954-0 | 100.00 % |
| peer-live-327e0954-1 | 100.00 % |
| peer-live-327e0954-2 | 100.00 % |
| peer-live-327e0954-3 | 100.00 % |
| peer-live-327e0954-4 | 100.00 % |
| peer-live-327e0954-5 | 100.00 % |
| peer-live-327e0954-7 | 100.00 % |
| peer-live-327e0954-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "insecure_tls": true,
    "room": "c015mvp82",
    "url": "wss://localhost:4443"
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
  },
  "server_monitoring": {
    "containers": [
      "mediasoup-demo"
    ],
    "interval_seconds": 5
  },
  "browser": {
    "url": "https://localhost:5555/?roomId=c015mvp82\u0026produce=false\u0026mic=false\u0026webcam=false\u0026displayName=browser",
    "expected_height": 720,
    "window": "1280x720",
    "insecure_tls": true
  }
}
```

Engine devel (go1.27.2)
