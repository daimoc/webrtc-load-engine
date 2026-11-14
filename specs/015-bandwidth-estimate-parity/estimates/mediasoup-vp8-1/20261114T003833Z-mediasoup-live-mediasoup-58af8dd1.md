# Load test report: live-mediasoup

mediasoup · run 58af8dd1 · 2026-11-14 00:37:24 → 2026-11-14 00:38:33 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 69.092 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 99.99 %) · 0 bots flagged |
| Browser | 720 lines 99.26 % of the time · reached after 1.0 s · 0 freezes |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 54.29 %, 180: 45.71 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.27 threads, peak 0.34 threads |
| Server memory (total) | peak 69.6 MiB |
| Engine CPU / memory | avg 0.53 threads, peak 0.59 threads · peak 177.3 MiB · 16 hardware threads |

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
- Resolution switches: 0
- Keyframe requests sent: 0, received: 35

| Height | Share of video time |
|---|---|
| 720 | 54.29 % |
| 180 | 45.71 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.15 % | 0.29 % | 0.29 % |
| Longest stamp lag | 0.6 ms | 3.9 ms | 3.9 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-14 00:37:24 to 2026-11-14 00:38:33 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 15 / 0 | 0.33 threads | 0.27 threads | 0.34 threads | 36.9 MiB | 69.6 MiB |
| **total** | 1 measured | 15 / 0 | 0.33 threads | 0.27 threads | 0.34 threads | 36.9 MiB | 69.6 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.53 threads | 0.59 threads | 177.3 MiB | 15 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-14 00:37:24 to 2026-11-14 00:38:33 (UTC).

A browser ran on this machine during the run: 2.60 threads on average, not counted in the engine's figure.

## Browser

Address https://localhost:5555/?roomId=c015mvp81&produce=false&mic=false&webcam=false&displayName=browser, window 1280x720.

- Browser: Chrome/156.0.8078.79
- Video streams decoding at once: at most 10
- At 720 lines: 99.26 % of the time, first after 1.0 s
- Received video: 14872 kbit/s on average, peak 16019 kbit/s over one second
- Freezes: 0, 0.000 s (as counted by the browser)
- CPU: 2.60 threads on average

| Height | Share of decoded stream time |
|---|---|
| 720 | 99.26 % |
| 360 | 0.15 % |
| 180 | 0.59 % |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-421132ec-2 | 4.37 |
| peer-live-421132ec-6 | 4.37 |
| peer-live-421132ec-9 | 4.37 |
| peer-live-421132ec-8 | 4.37 |
| peer-live-421132ec-3 | 4.37 |
| peer-live-421132ec-5 | 4.37 |
| peer-live-421132ec-0 | 4.37 |
| peer-live-421132ec-7 | 4.37 |
| peer-live-421132ec-1 | 4.37 |
| peer-live-421132ec-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-421132ec-2 | 4.5 ms |
| peer-live-421132ec-5 | 4.5 ms |
| peer-live-421132ec-6 | 4.5 ms |
| peer-live-421132ec-0 | 3.5 ms |
| peer-live-421132ec-1 | 3.5 ms |
| peer-live-421132ec-3 | 3.5 ms |
| peer-live-421132ec-4 | 3.5 ms |
| peer-live-421132ec-8 | 3.5 ms |
| peer-live-421132ec-9 | 3.5 ms |
| peer-live-421132ec-7 | 2.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-421132ec-0 | 100.00 % |
| peer-live-421132ec-1 | 100.00 % |
| peer-live-421132ec-2 | 100.00 % |
| peer-live-421132ec-3 | 100.00 % |
| peer-live-421132ec-4 | 100.00 % |
| peer-live-421132ec-5 | 100.00 % |
| peer-live-421132ec-6 | 100.00 % |
| peer-live-421132ec-7 | 100.00 % |
| peer-live-421132ec-8 | 100.00 % |
| peer-live-421132ec-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "insecure_tls": true,
    "room": "c015mvp81",
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
    "url": "https://localhost:5555/?roomId=c015mvp81\u0026produce=false\u0026mic=false\u0026webcam=false\u0026displayName=browser",
    "expected_height": 720,
    "window": "1280x720",
    "insecure_tls": true
  }
}
```

Engine devel (go1.27.2)
