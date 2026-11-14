# Load test report: live-mediasoup

mediasoup · run 8fabef01 · 2026-11-14 00:40:14 → 2026-11-14 00:41:25 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 71.116 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Browser | 720 lines 99.13 % of the time · reached after 1.0 s · 0 freezes |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 49.03 %, 360: 5.90 %, 180: 45.06 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.27 threads, peak 0.29 threads |
| Server memory (total) | peak 86.4 MiB |
| Engine CPU / memory | avg 0.57 threads, peak 0.63 threads · peak 177.5 MiB · 16 hardware threads |

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
| Requested-height share | 98.57 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 0
- Resolution switches: 18
- Keyframe requests sent: 0, received: 48

| Height | Share of video time |
|---|---|
| 720 | 49.03 % |
| 360 | 5.90 % |
| 180 | 45.06 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.00 % | 0.14 % | 0.14 % |
| Longest stamp lag | 0.7 ms | 3.3 ms | 3.3 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-14 00:40:14 to 2026-11-14 00:41:25 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 16 / 0 | 0.26 threads | 0.27 threads | 0.29 threads | 82.0 MiB | 86.4 MiB |
| **total** | 1 measured | 16 / 0 | 0.26 threads | 0.27 threads | 0.29 threads | 82.0 MiB | 86.4 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.57 threads | 0.63 threads | 177.5 MiB | 16 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-14 00:40:14 to 2026-11-14 00:41:25 (UTC).

A browser ran on this machine during the run: 2.64 threads on average, not counted in the engine's figure.

## Browser

Address https://localhost:5555/?roomId=c015mvp83&produce=false&mic=false&webcam=false&displayName=browser, window 1280x720.

- Browser: Chrome/156.0.8078.79
- Video streams decoding at once: at most 10
- At 720 lines: 99.13 % of the time, first after 1.0 s
- Received video: 14836 kbit/s on average, peak 16138 kbit/s over one second
- Freezes: 0, 0.000 s (as counted by the browser)
- CPU: 2.64 threads on average

| Height | Share of decoded stream time |
|---|---|
| 720 | 99.13 % |
| 360 | 0.43 % |
| 180 | 0.43 % |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-8fa3e0a1-1 | 4.37 |
| peer-live-8fa3e0a1-2 | 4.37 |
| peer-live-8fa3e0a1-0 | 4.37 |
| peer-live-8fa3e0a1-9 | 4.37 |
| peer-live-8fa3e0a1-6 | 4.37 |
| peer-live-8fa3e0a1-5 | 4.37 |
| peer-live-8fa3e0a1-7 | 4.37 |
| peer-live-8fa3e0a1-8 | 4.37 |
| peer-live-8fa3e0a1-3 | 4.37 |
| peer-live-8fa3e0a1-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-8fa3e0a1-0 | 6.5 ms |
| peer-live-8fa3e0a1-2 | 6.5 ms |
| peer-live-8fa3e0a1-1 | 5.5 ms |
| peer-live-8fa3e0a1-3 | 5.5 ms |
| peer-live-8fa3e0a1-4 | 5.5 ms |
| peer-live-8fa3e0a1-5 | 5.5 ms |
| peer-live-8fa3e0a1-9 | 5.5 ms |
| peer-live-8fa3e0a1-6 | 4.5 ms |
| peer-live-8fa3e0a1-7 | 4.5 ms |
| peer-live-8fa3e0a1-8 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-8fa3e0a1-5 | 27.78 % |
| peer-live-8fa3e0a1-7 | 98.57 % |
| peer-live-8fa3e0a1-6 | 98.57 % |
| peer-live-8fa3e0a1-9 | 98.57 % |
| peer-live-8fa3e0a1-8 | 98.57 % |
| peer-live-8fa3e0a1-0 | 100.00 % |
| peer-live-8fa3e0a1-1 | 100.00 % |
| peer-live-8fa3e0a1-2 | 100.00 % |
| peer-live-8fa3e0a1-3 | 100.00 % |
| peer-live-8fa3e0a1-4 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "insecure_tls": true,
    "room": "c015mvp83",
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
    "url": "https://localhost:5555/?roomId=c015mvp83\u0026produce=false\u0026mic=false\u0026webcam=false\u0026displayName=browser",
    "expected_height": 720,
    "window": "1280x720",
    "insecure_tls": true
  }
}
```

Engine devel (go1.27.2)
