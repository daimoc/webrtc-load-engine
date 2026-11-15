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
| 006 Media quality | Quality of what each bot receives (frame rate, freezes, resolution, audio gaps, estimated MOS), end‑to‑end latency between bots, end‑of‑run quality summary; simulcast (three layers, each bot asks for its height) |
| 007 Server load report | CPU and memory of the media server's local Docker containers and of the engine itself, sampled during the run, in the end‑of‑run summary and as Prometheus series |
| 008 LiveKit | LiveKit adapter: the same test plan runs against a LiveKit server (`"platform": "livekit"`), with the same quality and server‑load report; a pinned local LiveKit in `tests/platforms/livekit` |
| 010 mediasoup-demo | mediasoup-demo adapter (`"platform": "mediasoup"`): the demo web client's protoo signaling, SDP built from mediasoup's parameters, the same report; bots send RFC 7675 consent checks like browsers; a pinned local server built from the demo's commit in `tests/platforms/mediasoup` |
| 011 Janus | Janus VideoRoom adapter (`"platform": "janus"`): the Janus API over WebSocket, one multistream subscription per bot that follows the receive cap, the same report; a pinned local Janus built from its tag in `tests/platforms/janus` |
| 012 Video latency | Video latency on every platform, also with simulcast and on Janus: a 6-byte trailer after each frame identifies it where servers rewrite timestamps; audio is unchanged |
| 013 Remote servers | `sfu-remote` deploys, checks and removes one server of `tests/platforms` on another Linux machine over ssh; server load read through `DOCKER_HOST=ssh://` |
| 014 Video codecs | `"video_codec"`: VP8, VP9, AV1 or H.264 on every platform, at equal picture quality (VMAF); layered as simulcast (VP8, H.264) or SVC (VP9, AV1); simulcast and requested heights on LiveKit, mediasoup and Janus too; the codec and its limits in the report |
| 015 Bandwidth estimate parity | One headless browser beside the bots on request (`"browser"`), measured apart; every report says how completely the bots acknowledged what they received and which server settings were changed; the bots now acknowledge every packet that reaches them, which ends the Jitsi bridge's suspensions of 014 |
| 016 Network conditions | `"network"`: groups of bots with a schedule of bandwidth caps, delay and jitter, and random or burst loss, on each direction of their link, applied by the engine on the bots' own sockets; senders under an upload condition drop and restore layers like a browser; the report gives what each condition did, how fast the server reacted and recovered, and each group's quality |

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

* layers at 100 bots with the bridge on its own machine (on one laptop the bots take 2.5 cores next to the bridge),
* load distribution across several agents,
* topologies other than everyone sends to everyone (1→N, single speaker).

---
### Development

```bash
go build ./...
go vet ./... && go test -race ./...

# HTTP API on :8080 (run reports in ./reports, or --reports-dir)
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

Bots send pre-encoded Opus audio and VP8 video by default (640x360, 30 fps,
about 500 kbit/s; other codecs below) built into the binary, selected per
scenario with
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
the latency through the media server: from the RTP timestamp of each audio
frame, and from a 6-byte trailer after each video frame's data (012), which
survives servers rewriting timestamps and does not change what decoders show.
These are exported as `webrtc_media_*` series per scenario
(`specs/006-media-quality/contracts/metrics.md`). At the end of a run,
`GET /api/v1/scenarios/{id}/quality` returns a summary: percentiles over
bots, totals, and the worst bots by score, freezes and latency
(`specs/006-media-quality/contracts/quality-summary.yaml`).
`"simulcast": {"max_height": 360}` (or 720) in the media part of the test
plan makes the bots send three layers. `"requested_heights": [{"height": 360,
"share": 0.5}, {"height": 90, "share": 0.5}]` sets the height each bot asks
for. On Jitsi each bot then gets the layer it asks for (at 10 bots, 100% of
the time with 360/90 requests). The bridge rewrites the timestamps of
simulcast video; video latency is still measured, from the frame trailer. At 100
bots the local bridge needs more than its default 3 GB heap; with 8 GB the
bots take 2.49 cores at 360 (MOS 4.10) and 2.51 cores at 720 (MOS 3.84),
no more than without simulcast on the same day, but the laptop they share
with the bridge sets that figure (`specs/006-media-quality/validation-report.md`).

**Video codecs (014).** `"video_codec": "vp9"` (or `vp8`, the default,
`av1`, `h264`) in the media part of the test plan makes every bot send that
codec; one codec per run (`--video-codec`, live tests
`<PLATFORM>_VIDEO_CODEC`). Each codec's content is pre-encoded by
`scripts/gen-media.sh` at the same picture quality as the VP8 content (VMAF
within 1.0), each at its own bitrate (`src/media/assets/content.json`): VP9
and AV1 need less than VP8, H.264 about the same. With `"simulcast"`, VP8
and H.264 send three simulcast layers and VP9 and AV1 one `L3T3_KEY` SVC
stream with the dependency descriptor; requested heights work the same way
on all four platforms. The frame trailer has a carrier per codec: after the
frame data for VP8, the RTP padding for VP9, an `OBU_PADDING` for AV1 and an
SEI in its own packet for H.264; Chrome and Firefox played every layered
stream and the VP9 and H.264 single streams with it. The report
names the codec the server actually forwarded (`quality.video.codec`), fails
the bots that receive another codec (`mismatch`), and flags a layered run in
which the server forwarded only the lowest layer (`layers_not_forwarded`).

Limits found on the servers: Jitsi forwards only the lowest H.264
simulcast layer; Jitsi and LiveKit drop RTP padding, so VP9 latency there is
`frame_not_identified`; on mediasoup 3.30.1, bots asking for 720 spend part
of the run on lower layers when they all join at once: a worker bug makes
the probes of most receiving transports fail SRTP, so their estimate stays
low (015; spread the joins, e.g. a 20 s ramp-up for 10 bots). On a remote machine, 10 bots,
the server CPU against VP8 was: H.264 +6% on Janus and +13% on LiveKit, VP9
lower on LiveKit and mediasoup (less traffic), AV1 +7% on LiveKit and the
same as VP8 elsewhere; on Jitsi the first run after a deploy is on a cold
bridge, so codecs could not be told apart. At 100 bots the engine spends 20% less CPU on VP9 and 12% more on
H.264 than on VP8, following the packets it receives
(`specs/014-video-codec-comparison/validation-report.md`).

`"server_monitoring": {"containers": ["docker-jitsi-meet-jvb-1",
"docker-jitsi-meet-jicofo-1"], "interval_seconds": 5}` in the test plan
makes the engine sample those local containers through the Docker Engine
API during the run (the user running the engine must be able to use Docker;
`DOCKER_HOST=unix://...` selects another socket). The quality summary then
has a `server` section, per container and in total: average and peak CPU in
threads (1.0 is one thread busy, `docker stats` % / 100), the first
interval's CPU, peak and first memory, samples and missed intervals, and a
status (`measured`, `partial`, `unavailable`) with the reason when a
container could not be read (`not_found`, `not_running`,
`docker_unreachable`, ...). It never fails or delays the run. A
`load_machine` section gives the engine's own CPU and memory over the same
instants, with the machine's hardware threads and those the engine may use;
it is there with or without `server_monitoring`. The samples are exported as
`webrtc_server_cpu_threads`, `webrtc_server_memory_bytes`,
`webrtc_engine_cpu_threads` and `webrtc_engine_memory_bytes`
(`specs/007-server-load-report/contracts/`). Live test:
`JITSI_SERVER_CONTAINERS` (comma-separated) and `JITSI_SERVER_INTERVAL`;
`jitsi-connect`: `--server-containers` and `--server-interval`. Against
`docker stats` the bridge's average CPU was within 2% and its peak memory
within 0.02%; the engine's average CPU within 0.3% of `/usr/bin/time`.
Sampling three containers costs 0.64 ms of CPU per sample (0.0001 thread at
the 5 s default); at 100 bots the bridge used 8.7 threads on average
(`specs/007-server-load-report/validation-report.md`).

**LiveKit (008).** `"platform": "livekit"` with a `platform_config` of
`url` (e.g. `ws://localhost:7880`), `room`, and `api_key` + `api_secret`
(each bot signs its own access token, its peer ID as identity) or a
ready-made `token`
(`specs/008-livekit-platform/contracts/livekit-config.yaml`). The plan is
checked when the scenario is created. Simulcast and requested heights work
as on Jitsi (014). Secrets are hidden (`"[redacted]"`) in
what the service returns and logs, for both platforms. Each bot publishes
its tracks on a connection it offers and receives on the one the server
offers. It subscribes to at most `max_received_streams` tracks per kind
itself (auto-subscribe off), earliest first, and replaces one that ends.
The quality summary has the same fields as on Jitsi. LiveKit rewrites the
video RTP timestamps in some runs; audio and video latency are both measured,
video from the frame trailer (012).

```bash
cd tests/platforms/livekit && cp env.example .env   # set LIVEKIT_API_SECRET (32+ characters)
docker compose up -d && cd -
set -a; . tests/platforms/livekit/.env; set +a
LIVEKIT_URL=ws://localhost:7880 LIVEKIT_ROOM=load008 LIVEKIT_PARTICIPANTS=10 \
LIVEKIT_SERVER_CONTAINERS=livekit-server go test -tags live -count=1 -v -run LiveKit ./tests/live/
```

Same day, same machine, 10 bots `audio_video` for 60 s, each server alone:

| | LiveKit 1.13.10 | Jitsi (docker‑jitsi‑meet) |
|---|---|---|
| Frame rate p50, freezes | 30.0, 0 | 30.0, 14 |
| MOS p50, audio concealed p50 | 4.37, 0 | 4.37, 0 |
| Latency p50, audio / video | 0.5 ms / not available | 1.5 ms / 1.5 ms |
| Server CPU threads (average) | 0.14 | 0.97 (all four containers) |
| Engine CPU threads (average) | 0.09 | 0.39 |

Same day, 100 bots, cap 10, 10 minutes:

| | LiveKit | Jitsi |
|---|---|---|
| Freezes, MOS p50 | 0, 4.37 | 236, 4.33 |
| Latency p50, audio / video | 1.5 ms / not available | 7.5 ms / 10.5 ms |
| Server CPU threads (average), memory | 2.28, 1.2 GB | 8.65, 4.2 GB |
| Engine CPU (`/usr/bin/time`) | 1.43 cores | 2.90 cores |

The local LiveKit uses a UDP port range (50000–60000). Its single‑port mode
delayed everything it sent by 1.3 s at 100 bots
(`specs/008-livekit-platform/validation-report.md`).

**Run reports (009).** Every run, through the HTTP API, `jitsi-connect` or a
live test, ends by writing two files to `reports/`:
`<end time UTC>-<platform>-<name>-<run id>.json` and `.md`. The JSON file
holds the scenario's identity, the test plan (secrets as `"[redacted]"`),
start and end times, outcome (with the error of a failed run), engine
version and the quality summary exactly as `GET
/api/v1/scenarios/{id}/quality` returns it
(`specs/009-run-report-files/contracts/run-report.schema.yaml`). The Markdown
file gives the same figures for reading: key figures first (participants,
MOS, freezes, frame rate, resolution, latency, server and engine CPU and
memory), then audio, video, server, load machine, worst bots and the test
plan. Stopped and failed runs are reported too; a report never overwrites
another. The folder is `reports/` in the working directory (the
repository's for live tests), or `--reports-dir` (`go run ./src`,
`jitsi-connect`) or `REPORTS_DIR`; it is ignored by git. A write failure
never changes the run: it is logged and shown in the scenario's answer,
`GET /api/v1/scenarios/{id}` → `"report": {"run_id", "json": {"path" |
"error"}, "markdown": {...}}`.

**mediasoup-demo (010).** `"platform": "mediasoup"` with a `platform_config`
of `url` (e.g. `ws://localhost:4443`) and `room`, and optionally:
- `origin`: the demo refuses a WebSocket whose `Origin` doesn't match its
  domain; the default is `http(s)://<url host>`;
- `insecure_tls`: for `wss://` with a self-signed certificate;
- `unlimited_room_secret`: the server's secret for rooms without the demo's
  10-minute limit; redacted;
- `use_pipe_transports`: the room on two workers instead of one.

The details are in `specs/010-mediasoup-platform/contracts/mediasoup-config.yaml`.
Simulcast and requested heights work as on Jitsi (014).

The bots speak the demo web client's protoo protocol. mediasoup has no SDP:
the adapter builds each bot's receive offer and publish answer from the
server's ICE, DTLS and RTP parameters, as `mediasoup-client` does. The
receive cap uses the demo's paused consumers: a bot accepts `newConsumer` up
to `max_received_streams` per kind, declines the rest, and resumes the
earliest waiting one when an accepted one closes.

The demo closes a transport after 20 s without an ICE consent check. All
bots, on every platform, now send one every 5 s (RFC 7675), as browsers do.
The quality summary and report have the same fields as on LiveKit. Audio and
video latency are both measured.

```bash
cd tests/platforms/mediasoup && cp env.example .env && docker compose up -d --build && cd -
MEDIASOUP_URL=ws://localhost:4443 MEDIASOUP_ROOM=load010 MEDIASOUP_PARTICIPANTS=10 \
MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

10 bots `audio_video` for 60 s on the local server (mediasoup 3.30.1, 2
workers): 0 freezes, 30 fps, MOS p50 4.37, latency p50 0.5 ms audio and
2.5 ms video, server 0.21 threads, engine 0.54 threads.

At 100 bots with a cap of 10, the demo does not hold on this laptop, where
LiveKit does. It puts a room on one worker, one thread, which saturates: its
single UDP port drops packets and some bots lose ICE consent. With
`use_pipe_transports` (two workers), the bots hold their transports, but
audio degrades (MOS 2.99)
(`specs/010-mediasoup-platform/validation-report.md`).

**Janus (011).** `"platform": "janus"` with a `platform_config` of `url`
(e.g. `ws://localhost:8188`) and a numeric `room`, and optionally
`insecure_tls` and the server's own credentials: `api_secret`, `token`, the
room's `pin`, and the VideoRoom `admin_key` for creating rooms. The four
credentials are redacted. The details are in
`specs/011-janus-platform/contracts/janus-config.yaml`. Simulcast and
requested heights work as on Jitsi (014).

Each bot holds one Janus session over the WebSocket transport (subprotocol
`janus-protocol`), kept alive every 25 s, with two VideoRoom handles:
- a publisher, whose PeerConnection carries the bot's media;
- a subscriber, one multistream subscription that Janus offers.

The bots create the room if it does not exist, with one publisher place per
bot and the VideoRoom dummy publisher. Every bot subscribes to the dummy at
join, so that it is connected even alone in the room. The receive cap is
applied in the subscription: one `update` per change, swapping an ended
stream for the earliest waiting one.

The quality summary and report have the same fields as on the other
platforms. Janus rebases the RTP timestamps it forwards: video latency is
measured from the frame trailer (012), and audio latency stays left out with
`frame_not_identified`. The bots' sources start their timestamps at 2^30, so
that a rebased audio timestamp never matches a recently sent frame.
At 50 bots with receive cap 10, alternated with the other three servers,
Janus held every bot for 10 minutes with the same quality, using about
twice LiveKit's server CPU and 60 MB of memory
(`specs/011-janus-platform/validation-report.md`).

```bash
cd tests/platforms/janus && cp env.example .env && docker compose up -d --build && cd -
JANUS_URL=ws://localhost:8188 JANUS_ROOM=1011 JANUS_PARTICIPANTS=10 \
JANUS_SERVER_CONTAINERS=janus go test -tags live -count=1 -v -run Janus ./tests/live/
```

10 bots `audio_video` for 60 s on the local server (Janus 1.4.3): 0
freezes, 30 fps, MOS p50 4.37, server 0.76 threads, engine 0.46 threads.
Janus never offers loopback candidates: the bots reach it on the host's own
IPv4 address (`tests/platforms/janus/README.md`).

---

### A browser beside the bots (015)

A test plan can ask for one headless Chrome next to the bots, as a reference: the engine starts it, opens the plan's address, reads what the browser receives once per second, and stops it at the end of the run. The page must join the room by itself; the engine does nothing in it, so any server's web page works.

```json
"browser": {
  "url": "https://localhost:8443/room#config.prejoinConfig.enabled=false&config.startWithAudioMuted=true&config.startWithVideoMuted=true",
  "expected_height": 720,
  "insecure_tls": true
}
```

* Chrome or Chromium is needed only for such a plan (`executable`, else the first of `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser` in `PATH`). A plan without `browser` starts nothing and looks for nothing.
* The report gives the browser's part apart from the bots': heights over time, the share at `expected_height` and when it was first reached, received bitrate, freezes as the browser counts them, its CPU. The browser is in no bot figure, and its CPU is not in the engine's.
* A browser that cannot start, a page that does not load, or a page that receives no video never fails the run: the browser's part gives the reason (`browser_not_found`, `browser_start_failed`, `page_load_failed`, `no_video_received`, `browser_exited`).
* Tokens in the address (`jwt`, `token`, `access_token`) are hidden in reports and answers.
* Live tests: `<PLATFORM>_BROWSER_URL`, `_BROWSER_HEIGHT`, `_BROWSER_WINDOW`, `_BROWSER_INSECURE_TLS=1`. `jitsi-connect`: `--browser-url`, `--browser-height`, `--browser-window`, `--browser-insecure-tls`.
* The mediasoup demo's page is not part of `tests/platforms/mediasoup`: run the demo's app as its README says, then use `https://localhost:5555/?roomId=<room>&produce=false&mic=false&webcam=false`.

Every run also reports how the bots acknowledged what they received (`reception_reports`): per bot, the share of arriving packets acknowledged to the server, how regularly the feedback left, and how late arrivals were stamped. A bot under 99% acknowledged, or with more than 1% of its feedback intervals over 200 ms, is named, and the layered results carry a warning.

The report also says what it knows of the server's settings (`server_settings`): those `sfu-remote` changed (written to its settings file as `<PLATFORM>_SERVER_SETTINGS`), those an operator states in the plan, or "not known to the run".

### Network conditions per bot (016)

A test plan can put groups of bots behind a slow or unstable link. The engine applies the conditions itself, on each bot's own sockets: per bot, with no administrator rights, the same against a local or a remote server.

```json
"network": {
  "seed": 7,
  "groups": [
    { "name": "slow", "count": 3, "schedule": [
      { "start_seconds": 30, "duration_seconds": 20, "downlink": { "rate_kbps": 1200 } } ] },
    { "name": "lossy", "share": 0.2, "schedule": [
      { "start_seconds": 30, "downlink": { "delay_ms": 40, "jitter_ms": 10, "loss": { "share": 0.03, "mean_burst": 2 } },
        "uplink": { "rate_kbps": 400 } } ] }
  ]
}
```

* **Groups** take bots from the end of the launch order (`requested_heights` takes them from the start). Each group has a schedule of steps, in seconds from the first launch; between steps, no condition. A step that starts less than `settle_seconds` (10) after the last join is flagged.
* **Conditions**, per direction (`downlink`: server to bot, `uplink`: bot to server):
  * `rate_kbps`, a cap behind a drop-tail queue of `queue_ms` (200 ms);
  * `delay_ms` with `jitter_ms`, held 20 ms at a time and never reordering;
  * `loss`, random or in bursts of `mean_burst` packets.

  Loss and jitter come from `seed`, drawn and reported when absent.
* **The report** (format 4, `network`, and a "Network conditions" section in the Markdown) gives:
  * what each condition did, measured by the link: rate delivered, packets dropped by the cap and by loss, delay added;
  * per bot and step, the reaction time (first lower height or freeze) and the recovery time (the requested height back for 5 s without a freeze), at the collector's 2 s resolution;
  * the video still missing after retransmission;
  * the quality of each group and of the bots in no group.
* **Senders under an upload condition** adapt like a browser: they estimate their upload from the server's transport-wide feedback (Pion's estimator) and drop their top simulcast or SVC layer after 1 s of congestion. They try it again after 2, 4, then 8 s, and a layer comes back at its next keyframe. A single-stream sender cannot adapt, and the report says so. Bots without an upload condition send exactly as before.
* **mediasoup** gives its estimate toward each constrained bot through its signaling, and the report has it; the other servers give none.
* **Live tests and `jitsi-connect`:** `<PLATFORM>_NETWORK=<file.json>`, `--network <file.json>` (the file holds the `network` object).
* **Two bot behaviours changed in every run:**
  * where a server asks for REMB and not for transport-cc (LiveKit on its defaults), the bots estimate their download and send REMB, as a browser does; before 016, LiveKit never lowered a slow bot;
  * a bot asks at most 10 times for a lost packet, as libwebrtc does; Pion asked forever, and retransmissions filled a slow link 14-fold on Jitsi.

What the servers do (`specs/016-network-conditions/validation-report.md`):
* With a cap toward 3 of 10 bots, LiveKit, mediasoup and Jitsi lower those bots' layers within one collection (2 s). They give 720 back after the cap ends: mediasoup in about 6 s, LiveKit in about 32 s, Jitsi in about 38 s (medians of three runs). The other bots keep 720. Janus keeps forwarding the layer asked for: it has no downlink estimation, so its capped bots freeze.
* With 3% loss, LiveKit, mediasoup and Jitsi retransmit nearly everything, and Janus almost nothing.
* A constrained sender's top layers stop within 2 s of the upload cap and come back 6–7 s after it, on every server.

### Remote servers

`sfu-remote` runs one server of `tests/platforms` on another Linux machine, so that bots and server stop sharing a CPU. It drives the machine with your own `ssh`, and opens no Docker port.

A deployed Jitsi uses its bridge's bandwidth estimate as by default (since 015). Until then the remote overlay turned it off (`JVB_CC_TRUST_BWE=false`), because the bridge dropped some bots' streams; 015 found the bots caused it (they did not acknowledge the copies the bridge sends) and corrected them. What remains is the bridge's own start: it begins every receiver, a browser included, at 2.5 Mbit/s and can suspend streams for the first 10 to 20 s of a 10-bot room on a LAN. Use a ramp-up, or a hold long enough that those seconds do not weigh in the figures. Reports say which settings a deployment changed (`server_settings`).

```bash
go run ./cmd/sfu-remote deploy janus --host user@host   # or jitsi, livekit, mediasoup
go run ./cmd/sfu-remote status --host user@host
set -a; . ~/.config/webrtc-load-engine/sfu-remote/host.env; set +a   # settings for the live tests
JANUS_ROOM=2013 JANUS_PARTICIPANTS=10 JANUS_HOLD=60s go test -tags live -count=1 -v -run Janus ./tests/live/
go run ./cmd/sfu-remote remove --host user@host
```

**Preparing the machine**: Linux on x86_64, SSH key login, Docker with the Compose plugin, the user in the `docker` group (which is equivalent to root on that machine). For mediasoup, raise the socket buffers, or `deploy` warns:

```bash
printf 'net.core.rmem_max=4194304\nnet.core.wmem_max=4194304\n' | sudo tee /etc/sysctl.d/90-sfu-remote.conf
sudo sysctl --system
```

**Who can reach the server**: servers listen on every interface, so `deploy` judges the machine by all its addresses. If any is public, **IPv6 included**, it refuses unless given one of:

* `--allow <cidr>` (repeatable): firewall rules let only these addresses reach the server's ports. Needs passwordless `sudo iptables`, and `ip6tables` with public IPv6. `allow` changes the addresses while a server runs. **These rules have unit tests only: they have not been run on a real machine yet, so check them with an outside port scan before relying on them.**
* `--external-firewall`: you state that a firewall outside the machine restricts access. Nothing checks this: only a scan from outside proves it.

Every device on a private network can reach a deployed server, and signaling is `ws://`, as in the local setups.

**Good to know**:

* A reboot leaves nothing running, on purpose: firewall rules do not survive one. Run `deploy` again.
* Sourcing the settings file sets `DOCKER_HOST` in that shell, so its own `docker` commands also go to the machine.
* The file holds the deployment's secrets (mode 0600), and `remove` deletes it.
* For a browser check on Jitsi, open `https://<address>:8443/<room>` and accept the self-signed certificate.
* Reports read server CPU and memory over SSH, through `DOCKER_HOST=ssh://...`. It is an environment variable and not a test plan field, so that an API caller cannot make the engine start `ssh`.

**Traffic**, from the bundled media, before packet headers:

| Bots | Cap | Bots' upload | Bots' download | Downloaded per hour |
|---|---|---|---|---|
| 10 | none (9) | 6 Mbps | 50 Mbps | about 25 GB |
| 50 | 10 | 28 Mbps | 280 Mbps | about 125 GB |
| 100 | 10 | 56 Mbps | 560 Mbps | about 250 GB |

Over a home connection or a metered provider, start with 10 bots. Latency includes the real network path, so compare it between servers on one path, not with local figures.

## AI-assisted development

This project may experiment with an **AI agent–guided coding workflow** if it proves effective.

Potential use cases include:

* bootstrapping adapters and boilerplate code,
* generating and refining test scenarios,
* accelerating refactoring and documentation,
* assisting with exploratory performance analysis.

AI agents are considered **supporting tools**, not a replacement for human review, architectural decisions, or code ownership.

Contributions and feedback are welcome.

