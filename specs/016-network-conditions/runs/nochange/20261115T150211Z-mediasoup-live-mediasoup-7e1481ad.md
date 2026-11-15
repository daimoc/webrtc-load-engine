# Load test report: live-mediasoup

mediasoup · run 7e1481ad · 2026-11-15 15:00:44 → 2026-11-15 15:02:11 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | mediasoup |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 20 s); run 87.010 s |
| Outcome | stopped |
| Video codec | vp8 (simulcast: 180, 360, 720) · received vp8 by 10 bots |
| Video content bitrate | 180: 150 kbit/s, 360: 500 kbit/s, 720: 1500 kbit/s |
| Server settings | not known to the run |
| Reception reports | acknowledged 100.00 % (lowest bot 100.00 %) · 0 bots flagged |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 0 in total; freeze time p95 0.000 s |
| Video frame rate | 30.0 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 720: 47.13 %, 360: 0.40 %, 180: 52.47 % |
| Latency p50 (audio / video) | 0.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.20 threads, peak 0.23 threads |
| Server memory (total) | peak 54.9 MiB |
| Engine CPU / memory | avg 0.55 threads, peak 0.64 threads · peak 176.0 MiB · 16 hardware threads |

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
| Requested-height share | 99.65 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 3.5 ms | 3.5 ms |

- Freezes: 0
- Resolution switches: 5
- Keyframe requests sent: 0, received: 107

| Height | Share of video time |
|---|---|
| 720 | 47.13 % |
| 360 | 0.40 % |
| 180 | 52.47 % |

## Reception reports

How completely and how regularly the bots reported their reception to the server (10 bots measured).

| | p50 | p95 | p99 |
|---|---|---|---|
| Acknowledged share | 100.00 % | 100.00 % | 100.00 % |
| Late feedback share | 0.13 % | 0.15 % | 0.15 % |
| Longest stamp lag | 0.0 ms | 0.0 ms | 0.0 ms |

A bot is flagged when it acknowledged under 0.99 of its arriving packets, or when over 1.00 % of its feedback intervals were longer than 200 ms.

No bot flagged.

## Server

Sampled every 5 s from 2026-11-15 15:00:44 to 2026-11-15 15:02:11 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| mediasoup-demo | measured | 19 / 0 | 0.05 threads | 0.20 threads | 0.23 threads | 37.4 MiB | 54.9 MiB |
| **total** | 1 measured | 19 / 0 | 0.05 threads | 0.20 threads | 0.23 threads | 37.4 MiB | 54.9 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.55 threads | 0.64 threads | 176.0 MiB | 19 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-15 15:00:44 to 2026-11-15 15:02:11 (UTC).

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-48fb2bcd-8 | 4.37 |
| peer-live-48fb2bcd-9 | 4.37 |
| peer-live-48fb2bcd-7 | 4.37 |
| peer-live-48fb2bcd-2 | 4.37 |
| peer-live-48fb2bcd-6 | 4.37 |
| peer-live-48fb2bcd-3 | 4.37 |
| peer-live-48fb2bcd-1 | 4.37 |
| peer-live-48fb2bcd-0 | 4.37 |
| peer-live-48fb2bcd-5 | 4.37 |
| peer-live-48fb2bcd-4 | 4.37 |

### Most freeze seconds

None.

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-48fb2bcd-0 | 5.5 ms |
| peer-live-48fb2bcd-1 | 5.5 ms |
| peer-live-48fb2bcd-2 | 5.5 ms |
| peer-live-48fb2bcd-3 | 5.5 ms |
| peer-live-48fb2bcd-4 | 4.5 ms |
| peer-live-48fb2bcd-5 | 4.5 ms |
| peer-live-48fb2bcd-6 | 4.5 ms |
| peer-live-48fb2bcd-7 | 4.5 ms |
| peer-live-48fb2bcd-8 | 4.5 ms |
| peer-live-48fb2bcd-9 | 4.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-48fb2bcd-5 | 98.68 % |
| peer-live-48fb2bcd-8 | 98.93 % |
| peer-live-48fb2bcd-9 | 99.26 % |
| peer-live-48fb2bcd-6 | 99.32 % |
| peer-live-48fb2bcd-7 | 99.65 % |
| peer-live-48fb2bcd-0 | 100.00 % |
| peer-live-48fb2bcd-1 | 100.00 % |
| peer-live-48fb2bcd-2 | 100.00 % |
| peer-live-48fb2bcd-3 | 100.00 % |
| peer-live-48fb2bcd-4 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 20,
  "duration_seconds": 3600,
  "platform_config": {
    "room": "r-nc",
    "url": "ws://localhost:4443"
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
  }
}
```

Engine devel (go1.27.2)
