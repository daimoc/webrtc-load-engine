# Local Janus server (011)

The [Janus WebRTC Server](https://github.com/meetecho/janus-gateway), built
from a pinned tag, for the engine's Janus runs (specs/011-janus-platform).
The engine uses its VideoRoom plugin (multistream) over the WebSocket
transport. It runs on the load machine with host networking. Only its
configuration (`conf/`, `demos-settings.js`) is ours: Janus's code is
unmodified.

- **Pinned version**: tag `v1.4.3` (commit `1f0b75c`), libnice 0.1.21
  (Debian bookworm).
- **Images**: `webrtc-load-engine/janus:v1.4.3` and
  `webrtc-load-engine/janus-demos:v1.4.3`. Janus publishes no official
  image, so `docker compose` builds both from the tag. Only the VideoRoom
  plugin and the WebSocket transport are built.
- **Containers**:
  - `janus`: the server, the one to monitor;
  - `janus-demos`: nginx serving Janus's demo pages, kept apart so that the
    web server's CPU is not counted in the server's load.

Ports:
- 8188/tcp on 127.0.0.1: the Janus API over WebSocket (`janus-protocol`),
  plain `ws://`;
- 8090/tcp on 127.0.0.1: the demo pages;
- 20000–20999/udp, on the host's IPv4 address: media, one port per
  PeerConnection.

They do not collide with docker-jitsi-meet, LiveKit or mediasoup-demo, but
stop the other servers before a run, so they don't share the CPU.

## Start

```bash
cd tests/platforms/janus
cp env.example .env
docker compose up -d --build     # the first start builds Janus (about 2 minutes)
docker logs janus 2>&1 | grep -E "Starting Meetecho Janus|WARN|ERR"
```

Expected:
- `Starting Meetecho Janus (WebRTC Server) v1.4.3`;
- four WARN lines: no logger plugins folder, mDNS off, no STUN server, data
  channels not compiled;
- `ss -lnt | grep -E '8188|8090'` shows both ports.

`.env` is not committed (`.gitignore`).

## Use it in a test plan

```json
{
  "name": "janus-10-bots",
  "platform": "janus",
  "test_plan": {
    "participants": 10, "ramp_up_seconds": 10, "duration_seconds": 60,
    "media": { "profile": "audio_video", "max_received_streams": 10 },
    "server_monitoring": { "containers": ["janus"] },
    "platform_config": { "url": "ws://localhost:8188", "room": 1011 }
  }
}
```

`room` is a number. The other settings (`insecure_tls`, `api_secret`,
`token`, `pin`, `admin_key`) are in
`specs/011-janus-platform/contracts/janus-config.yaml`. The four secrets are
never logged, and show as `[redacted]` in answers and reports. This server
needs none of them. Simulcast and requested heights are rejected on Janus
in this version.

Live test, from the repository root:

```bash
export JANUS_URL=ws://localhost:8188 JANUS_SERVER_CONTAINERS=janus
JANUS_ROOM=1011 JANUS_PARTICIPANTS=10 JANUS_MEDIA=audio_video JANUS_HOLD=60s \
  go test -tags live -count=1 -v -run Janus ./tests/live/
```

The variables are listed at the top of `tests/live/janus_live_test.go`. Each
run writes its report (JSON and Markdown) to the repository's `reports/`
folder (feature 009).

## Rooms

The bots create their room when it does not exist, so no static room is
configured. Every bot asks for it, one wins, and the others get "room
exists". Each such answer logs one `Room N already exists!` ERR line in
Janus's log, which is expected. The room is created:
- with one publisher place per bot of the test plan;
- with Opus and VP8 only, and no bitrate cap;
- with a **dummy publisher**: a silent audio placeholder that every bot
  subscribes to at join, so that it is connected even alone in the room
  (research R3). It takes no publisher place.

A room keeps the size it was created with, until Janus restarts (rooms are
not permanent). If a run has more bots than the room's first run, the extra
bots fail with `432 Maximum number of active publishers`. Use a new room
number, or restart Janus (`docker compose restart janus`). A room created
some other way, without a dummy publisher, is refused with `room N has no
dummy publisher`.

## Watch a room in a browser

Open `http://localhost:8090/demos/videoroom.html?room=1011` during a run
and follow the page's prompts (the display name must be alphanumeric). The
demo joins the room over the WebSocket transport (`demos-settings.js`
replaces the demo's `settings.js`; the HTTP transport is not built) and
shows the bots' videos, up to five, named after their peer IDs. It also
tries to publish your camera. The bots size the room to their own count,
so that publish is refused with `Maximum number of active publishers (N)
already reached`: close the dialog, the bots' videos keep playing. The
multistream variant is `mvideoroom.html?room=1011`.

## Network: no loopback

Janus never gathers ICE candidates on loopback: its `ice.c` skips loopback
interfaces. It therefore offers the host's own IPv4 address, for example
192.168.1.65, and the bots connect to it like LAN clients. The machine needs
one non-loopback IPv4 interface that is up.
- Container bridges (`docker`, `br-`, `veth`, `virbr`) are ignored
  (`conf/janus.jcfg`, `ice_ignore_list`), and IPv6 is off.
- To pin one interface, set `JANUS_ICE_ENFORCE_LIST=<interface>` in `.env`
  (see `ip -4 -br addr`), then run `docker compose up -d`.
- Janus logs `Failed to add some remote candidates` for bot candidates it
  cannot use (IPv6, bridges). That is harmless: the bots connect on the
  IPv4 pair.

## Sessions

Janus drops a session that sends nothing for 60 s (`session_timeout` in
`conf/janus.jcfg`). The bots send a keepalive every 25 s.

## Pinned version: bump it

```bash
docker compose build --build-arg JANUS_VERSION=<tag>
```

Then update the tag in `docker-compose.yml`'s image names, in the
`Dockerfile`'s default and in this README. Compare `conf/` with the tag's
`conf/*.jcfg.sample*`.

## Stop

```bash
docker compose down
```

## If the bots cannot connect

- **The handshake fails with no HTTP status**: Janus closes a WebSocket that
  does not ask for the `janus-protocol` subprotocol. The engine always
  does; check the URL (`ws://host:8188`, not the demo port).
- **The PeerConnections stay in `connecting`**: the bots cannot reach the
  address Janus offers. Check `ip -4 -br addr`, and set
  `JANUS_ICE_ENFORCE_LIST` to an interface the bots can reach.
- **`432 Maximum number of active publishers`**: see "Rooms".
- **`403 Unauthorized request`**: the server has an `api_secret` or token
  authentication; give it as `api_secret` or `token` in `platform_config`.
