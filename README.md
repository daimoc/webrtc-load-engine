# WebRTC Load Engine

A modular, platform-agnostic load testing engine for WebRTC signaling and real-time media.

## Vision

**WebRTC Load Engine** is an open‑source tool for **load testing, resilience testing, and performance evaluation of WebRTC platforms**.

The long‑term vision is to provide a **generic, modular, and extensible engine** capable of testing any real‑time media architecture: SFUs, MCUs, SIP–WebRTC gateways, or hybrid solutions.

The tool is designed to be used:

* in **lab environments** (capacity testing, platform comparison),
* in **CI/CD pipelines** (performance regression testing),
* and in **controlled production environments** (planned load tests).

---

## Long‑term goals

* 🧪 Test **real‑world scalability** of WebRTC platforms
* 📡 Measure **media quality** (audio / video)
* 🔁 Validate **signaling robustness**
* 📊 Provide **actionable metrics** for SRE and capacity planning
* 🔌 Remain **platform‑agnostic** (Jitsi, Janus, mediasoup, LiveKit, etc.)
* 🤖 Be **fully automatable** (API, CLI, agents)

---

## Design principles

### 1. Strict separation of concerns

The tool is structured around three independent layers:

1. **Signaling**
2. **Media**
3. **Orchestration & scenarios**

This separation allows:

* adding new platforms without rewriting the core engine,
* replaying the same scenarios across different backends,
* evolving each layer independently.

---

## Target architecture

### 1. Signaling layer

Responsible for establishing and managing WebRTC connections.

Responsibilities:

* Session creation and teardown
* Room and participant management
* SDP / ICE negotiation

Implemented through **platform adapters**:

* Jitsi (XMPP / Colibri)
* Janus
* mediasoup
* LiveKit
* Others to come

Each adapter implements a shared interface.

---

### 2. Media layer

Responsible for sending and receiving real‑time media streams.

Responsibilities:

* Synthetic media generation (audio / video)
* Media reception and analysis
* Support for:

  * audio‑only
  * video
  * simulcast / SVC

Target metrics:

* actual bitrate
* end‑to‑end latency
* jitter
* packet loss
* perceived quality (MOS, long term)

---

### 3. Orchestration & scenario layer

Responsible for driving load test execution.

Responsibilities:

* Scenario definition:

  * number of participants
  * ramp‑up / ramp‑down
  * duration
  * topologies (1→N, N→N, single speaker)
* Load distribution across multiple agents
* Constraint injection:

  * bandwidth limitation
  * network instability (future)

Control interfaces:

* CLI
* HTTP API
* CI/CD integration
* Automated agents (human‑driven or AI‑driven)

---

## Deployment and execution

Planned targets:

* Headless execution (Chrome / native WebRTC)
* Docker containers
* On‑prem or cloud VMs
* Isolated environments for large‑scale tests

Distributed architecture:

* one **control plane**
* multiple **load agents**

---

## Observability

* Metrics export:

  * Prometheus
  * JSON / files
* Structured logs
* Correlation between signaling and media metrics

Goal: enable fine‑grained analysis of bottlenecks (SFU, network, client).

---

## Open‑source philosophy

* Open, modular, and well‑documented code
* Clear contribution interfaces
* Reproducible tests
* Vendor‑neutral and platform‑agnostic

The goal is to become a **shared building block** for WebRTC, SRE, and platform teams.

---

## Project status

🚧 **Phase 1 – Jitsi load testing on a single machine**

Implemented (each feature is specified, planned and validated under `specs/`):

| Feature | What it provides |
|---------|------------------|
| 001 Scenario API | Create and read test scenarios over HTTP |
| 002 Execution engine | Bots joining over a ramp‑up, fixed duration, clean teardown, per‑bot lifecycle, Prometheus metrics |
| 003 XMPP signaling | Jitsi adapter: bots join a room like the web client (XMPP over WebSocket, Jingle) and connect audio and video to the bridge |
| 004 Synthetic media | Bots send looped audio and video; per‑kind cap on received streams; media metrics |
| 005 Bridge channel subscriptions | Bots request video from the bridge and subscribe only to the streams they keep |
| 006 Media quality | Quality of what each bot receives (frame rate, freezes, resolution, audio gaps, estimated MOS), end‑to‑end latency between bots, end‑of‑run quality summary; simulcast sending (implemented, not yet working with the Jitsi bridge) |

Measured on a local docker‑jitsi‑meet, bots and Jitsi on one laptop (16 threads):

| Load | Bot CPU | Bot memory | Received per bot |
|------|---------|------------|------------------|
| 100 bots, receive only | 0.07 cores | 100 MB | nothing (no one sends) |
| 100 bots, audio + video, no cap | 3.85 cores | 3.25 GB | 99 audio streams, no video |
| 100 bots, audio + video, cap 10 | 1.05 cores | 818 MB | 10 audio + 10 video streams |
| Same, with quality measurement (006), rerun on another day | 1.72 cores (1.71 without it, same day) | 856 MB | 10 audio + 10 video streams |

The CPU of the same run varies with the host state (1.05 cores in 005, 1.71
for the same code a day later): compare runs made the same day. Quality
measurement costs about 0.05 core at 100 bots. In that run the bots received
30 frames/s at 360 lines, 0.27% of audio was concealed (95th percentile over
bots), the median estimated score was 4.35, and the median latency through
the bridge was 6.5 ms for video and 3.5 ms for audio.

Details: `specs/005-bridge-channel-subscriptions/validation-report.md`,
`specs/006-media-quality/validation-report.md`.

Not done yet:

* adapters for other platforms (Janus, mediasoup, LiveKit),
* simulcast with the Jitsi bridge (the bots send three layers but the bridge forwards only the lowest one) and SVC,
* load distribution across several agents,
* bandwidth limits and network instability,
* topologies other than everyone sends to everyone (1→N, single speaker).

---
### Development

```bash
go build ./...
go vet ./... && go test -race ./...

# HTTP API on :8080
go run ./src

# Manual single-bot Jitsi join (colour-coded XMPP trace with --log-xmpp)
go run ./cmd/jitsi-connect --domain meet.example.org --room test --log-xmpp

# Several bots sending looped audio and video (receive_only, audio, audio_video)
go run ./cmd/jitsi-connect --domain meet.example.org --room test --participants 2 --media audio_video

# The same, sending a video file's audio and video (needs ffmpeg in PATH)
go run ./cmd/jitsi-connect --domain meet.example.org --room test --participants 2 --media audio_video --media-file clip.mp4
```

Live test against a Jitsi deployment (here the local docker‑jitsi‑meet; the
MUC domain is required, otherwise the join times out):

```bash
JITSI_ROOM_URL=http://localhost:8000/loadtest JITSI_XMPP_DOMAIN=meet.jitsi \
JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
JITSI_PARTICIPANTS=10 JITSI_MEDIA=audio_video JITSI_EXPECT_VIDEO=1 JITSI_HOLD=30s \
  go test -tags live -count=1 -v ./tests/live/
```

The other variables (receive cap, ramp‑up, metrics address, simulcast, …)
are listed at the top of `tests/live/jitsi_live_test.go`. The test logs the
run's quality summary at the end.

Bots send pre-encoded Opus audio and VP8 video (640x360, 30 fps, about
500 kbit/s) built into the binary, selected per scenario with
`"media": {"profile": "audio_video"}` in the test plan; without it they only
receive. `"source_file": "/path/to/clip.mp4"` in the media part of the plan
(`--media-file`, live test `JITSI_MEDIA_FILE`) makes the bots send a file
of yours instead: its audio and video are transcoded with ffmpeg, which must
be in the engine's PATH, to the same formats when the scenario starts, and
looped. `"max_received_streams": N` (or `--max-received N`) caps the remote
streams each bot receives per kind, to keep large rooms within one load
machine's budget. Per-scenario media metrics (`webrtc_media_*` on `/metrics`: volumes,
loss, jitter, round trip, remote streams, stalls, send lateness) are described
in `specs/004-synthetic-media/contracts/metrics.md`.

On Jitsi, each bot tells the bridge what it receives over the bridge channel,
as browsers do: it asks for the video of the streams it receives (up to 360
lines), and with a cap it subscribes to its received audio only, so the
bridge stops sending the other streams. The bots acknowledge every packet
that arrives in their congestion control feedback, so the bridge's bandwidth
estimate reflects the real link. `webrtc_data_channels_open` shows how many
bots have the bridge channel open, and the ratio of
`webrtc_transport_bytes_received_total` to the `webrtc_media_bytes_received_total`
of both kinds (about 1.03 when the bridge sends only the received streams)
shows whether the bridge honours the subscriptions; see
`specs/005-bridge-channel-subscriptions/contracts/`. The adapter option
`no_bridge_channel` (live test: `JITSI_NO_BRIDGE_CHANNEL=1`) leaves the
bridge channel closed, for comparison runs against the behavior without
subscriptions.

Every bot measures the quality of what it receives, from packet headers only
(no decoding). For video it measures frame rate, freezes, resolution, resolution switches and
keyframe requests; for audio, the share it would not play in time with a
60 ms playout delay, and an E‑model listening score (MOS, at most 4.41 with
the values used for Opus). Between bots of the same process it also measures
the latency through the media server, from the RTP timestamp of each frame.
These are exported as `webrtc_media_*` series per scenario
(`specs/006-media-quality/contracts/metrics.md`). At the end of a run,
`GET /api/v1/scenarios/{id}/quality` returns a summary: percentiles over
bots, totals, and the worst bots by score, freezes and latency
(`specs/006-media-quality/contracts/quality-summary.yaml`).
`"simulcast": {"max_height": 360}` (or 720) in the media part of the test
plan makes the bots send three layers. `"requested_heights": [{"height": 360,
"share": 0.5}, {"height": 90, "share": 0.5}]` sets the height each bot asks
for. With the Jitsi bridge these do not work yet: every bot receives the
lowest layer (`specs/006-media-quality/tasks.md` T057).

---

## AI-assisted development

This project may experiment with an **AI agent–guided coding workflow** if it proves effective.

Potential use cases include:

* bootstrapping adapters and boilerplate code,
* generating and refining test scenarios,
* accelerating refactoring and documentation,
* assisting with exploratory performance analysis.

AI agents are considered **supporting tools**, not a replacement for human review, architectural decisions, or code ownership.

Contributions and feedback are welcome.

