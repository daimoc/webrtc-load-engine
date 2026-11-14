# Load test report: live

jitsi · run ed3ee7ae · 2026-11-14 08:27:24 → 2026-11-14 08:28:28 (UTC) · stopped

## Key figures

| | |
|---|---|
| Platform | jitsi |
| Participants | 10 (bots measured: 10) |
| Test duration | 3600 s (ramp-up 0 s); run 64.341 s |
| Outcome | stopped |
| Video codec | vp8 (single) · received vp8 by 10 bots |
| Video content bitrate | 360: 500 kbit/s |
| Server settings | not known to the run |
| Reception reports | not measured: not_counted |
| Browser | 360 lines 100.00 % of the time · reached after 2.0 s · 0 freezes |
| Audio MOS | median 4.37, worst bot 4.37 |
| Audio concealed | p95 0.00 % |
| Video freezes | 34 in total; freeze time p95 51.027 s |
| Video frame rate | 29.6 / 30.0 / 30.0 fps (p50 / p95 / p99) |
| Video resolution | 360: 100.00 % |
| Latency p50 (audio / video) | 1.5 ms / 1.5 ms |
| Server CPU (total) | avg 0.79 threads, peak 0.89 threads |
| Server memory (total) | peak 1482.0 MiB |
| Engine CPU / memory | avg 0.43 threads, peak 0.46 threads · peak 100.5 MiB · 16 hardware threads |

## Audio

Bots receiving audio: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| MOS | 4.37 | 4.37 | 4.37 |
| Concealed | 0.00 % | 0.00 % | 0.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

## Video

Bots receiving video: 10.

| | p50 | p95 | p99 |
|---|---|---|---|
| Frame rate | 29.6 fps | 30.0 fps | 30.0 fps |
| Freeze time | 0.000 s | 51.027 s | 51.027 s |
| Requested-height share | 100.00 % | 100.00 % | 100.00 % |
| Latency | 1.5 ms | 1.5 ms | 1.5 ms |

- Freezes: 34
- Resolution switches: 0
- Keyframe requests sent: 0, received: 23

| Height | Share of video time |
|---|---|
| 360 | 100.00 % |

## Reception reports

Not measured: not_counted.

## Server

Sampled every 5 s from 2026-11-14 08:27:24 to 2026-11-14 08:28:28 (UTC).

| Container | Status | Samples / missed | First CPU | Avg CPU | Peak CPU | First memory | Peak memory |
|---|---|---|---|---|---|---|---|
| docker-jitsi-meet-jvb-1 | measured | 14 / 0 | 0.79 threads | 0.78 threads | 0.84 threads | 1216.1 MiB | 1230.0 MiB |
| docker-jitsi-meet-jicofo-1 | measured | 14 / 0 | 0.10 threads | 0.01 threads | 0.10 threads | 250.0 MiB | 252.3 MiB |
| **total** | 2 measured | 14 / 0 | 0.89 threads | 0.79 threads | 0.89 threads | 1466.1 MiB | 1482.0 MiB |

## Load machine

| | Avg CPU | Peak CPU | Peak memory | Samples / missed |
|---|---|---|---|---|
| Engine | 0.43 threads | 0.46 threads | 100.5 MiB | 14 / 0 |

Hardware threads: 16 (usable by the engine: 16).
Period: 2026-11-14 08:27:24 to 2026-11-14 08:28:28 (UTC).

A browser ran on this machine during the run: 1.33 threads on average, not counted in the engine's figure.

## Browser

Address https://localhost:8443/c015jvp8abbuf1#config.prejoinConfig.enabled=false&config.startWithAudioMuted=true&config.startWithVideoMuted=true, window 1280x720.

- Browser: Chrome/156.0.8078.79
- Video streams decoding at once: at most 10
- At 360 lines: 100.00 % of the time, first after 2.0 s
- Received video: 4911 kbit/s on average, peak 5161 kbit/s over one second
- Freezes: 0, 0.000 s (as counted by the browser)
- CPU: 1.33 threads on average

| Height | Share of decoded stream time |
|---|---|
| 360 | 100.00 % |

## Worst bots

### Lowest MOS

| Peer | Value |
|---|---|
| peer-live-d5d8b225-4 | 4.37 |
| peer-live-d5d8b225-5 | 4.37 |
| peer-live-d5d8b225-2 | 4.37 |
| peer-live-d5d8b225-8 | 4.37 |
| peer-live-d5d8b225-0 | 4.37 |
| peer-live-d5d8b225-1 | 4.37 |
| peer-live-d5d8b225-9 | 4.37 |
| peer-live-d5d8b225-7 | 4.37 |
| peer-live-d5d8b225-3 | 4.37 |
| peer-live-d5d8b225-6 | 4.37 |

### Most freeze seconds

| Peer | Value |
|---|---|
| peer-live-d5d8b225-6 | 51.027 s |
| peer-live-d5d8b225-4 | 50.000 s |
| peer-live-d5d8b225-5 | 20.806 s |
| peer-live-d5d8b225-8 | 7.010 s |
| peer-live-d5d8b225-1 | 7.002 s |

### Highest latency p95

| Peer | Value |
|---|---|
| peer-live-d5d8b225-0 | 2.5 ms |
| peer-live-d5d8b225-1 | 2.5 ms |
| peer-live-d5d8b225-2 | 2.5 ms |
| peer-live-d5d8b225-3 | 2.5 ms |
| peer-live-d5d8b225-4 | 2.5 ms |
| peer-live-d5d8b225-5 | 2.5 ms |
| peer-live-d5d8b225-7 | 2.5 ms |
| peer-live-d5d8b225-8 | 2.5 ms |
| peer-live-d5d8b225-9 | 2.5 ms |
| peer-live-d5d8b225-6 | 1.5 ms |

### Lowest requested-height share

| Peer | Value |
|---|---|
| peer-live-d5d8b225-0 | 100.00 % |
| peer-live-d5d8b225-1 | 100.00 % |
| peer-live-d5d8b225-2 | 100.00 % |
| peer-live-d5d8b225-3 | 100.00 % |
| peer-live-d5d8b225-4 | 100.00 % |
| peer-live-d5d8b225-5 | 100.00 % |
| peer-live-d5d8b225-6 | 100.00 % |
| peer-live-d5d8b225-7 | 100.00 % |
| peer-live-d5d8b225-8 | 100.00 % |
| peer-live-d5d8b225-9 | 100.00 % |

## Test plan

```json
{
  "participants": 10,
  "ramp_up_seconds": 0,
  "duration_seconds": 3600,
  "platform_config": {
    "connect_timeout": "20s",
    "debug": false,
    "muc_domain": "muc.meet.jitsi",
    "no_bridge_channel": false,
    "room_name": "c015jvp8abbuf1",
    "websocket_url": "ws://localhost:8000/xmpp-websocket?room=c015jvp8abbuf1",
    "xmpp_domain": "meet.jitsi"
  },
  "media": {
    "profile": "audio_video",
    "stall_timeout_seconds": 10,
    "video_codec": "vp8"
  },
  "server_monitoring": {
    "containers": [
      "docker-jitsi-meet-jvb-1",
      "docker-jitsi-meet-jicofo-1"
    ],
    "interval_seconds": 5
  },
  "browser": {
    "url": "https://localhost:8443/c015jvp8abbuf1#config.prejoinConfig.enabled=false\u0026config.startWithAudioMuted=true\u0026config.startWithVideoMuted=true",
    "expected_height": 360,
    "window": "1280x720",
    "insecure_tls": true
  }
}
```

Engine devel (go1.27.2)
