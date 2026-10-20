# Local mediasoup-demo server (010)

The server part of the official
[mediasoup-demo](https://github.com/versatica/mediasoup-demo), built from a
pinned commit, for the engine's mediasoup runs
(specs/010-mediasoup-platform). It runs on the load machine with host
networking, so the bots reach it over loopback. Only its configuration
(`config.mjs`) is ours: the demo's code is unmodified.

- Pinned version: commit `899109e4046b45da1623a3e4c65a742040479439`
  (2026-10-19), mediasoup **3.30.1**, Node 22.
- Image: `webrtc-load-engine/mediasoup-demo:899109e`. The repository
  publishes no image and no release tags, so `docker compose` builds it from
  that commit and its `package-lock.json`.

Ports, all on 127.0.0.1:
- 4443/tcp: protoo WebSocket, plain `ws://`;
- 44444 and up, UDP and TCP: media, one port per worker.

They do not collide with docker-jitsi-meet or LiveKit, but stop the other
servers before a run, so they don't share the CPU.

## Start

```bash
cd tests/platforms/mediasoup
cp env.example .env
docker compose up -d --build        # the first start builds the image (about a minute)
docker logs mediasoup-demo 2>&1 | grep -E "launching|Server started"
```

Expected: `launching 2 mediasoup Workers...` then `Server started`.
`ss -lntu | grep -E '4443|4444'` shows TCP 4443 and UDP/TCP 44444–44445.

`.env` is not committed (`.gitignore`).

## Use it in a test plan

```json
{
  "name": "ms-10-bots",
  "platform": "mediasoup",
  "test_plan": {
    "participants": 10, "ramp_up_seconds": 10, "duration_seconds": 60,
    "media": { "profile": "audio_video", "max_received_streams": 10 },
    "server_monitoring": { "containers": ["mediasoup-demo"] },
    "platform_config": { "url": "ws://localhost:4443", "room": "load010" }
  }
}
```

The other settings (`origin`, `insecure_tls`, `unlimited_room_secret`,
`use_pipe_transports`) are in
`specs/010-mediasoup-platform/contracts/mediasoup-config.yaml`. Simulcast and
requested heights are rejected on mediasoup in this version.

Live test, from the repository root:

```bash
export MEDIASOUP_URL=ws://localhost:4443 MEDIASOUP_ROOM=load010 MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo
MEDIASOUP_PARTICIPANTS=10 MEDIASOUP_MEDIA=audio_video MEDIASOUP_HOLD=60s \
  go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

The variables are listed at the top of `tests/live/mediasoup_live_test.go`.
Each run writes its report (JSON and Markdown) to the repository's
`reports/` folder (feature 009).

## Workers

The server runs `MEDIASOUP_NUM_WORKERS` mediasoup workers (default 2). Each
is one process and one media thread. The server's load in a report is the
container's: the node process and all the workers.

The demo puts a room on **one worker**: all the media of a test runs on one
thread, whatever the number of workers. With `"use_pipe_transports": true`
in `platform_config`, the room uses two workers, one for producers and one
for consumers, piped together. That needs at least 2 workers. Changing the
number of workers or the mode changes the server's CPU figures, so keep them
the same in runs you compare. The mode used is in each report's
`platform_config`.

## Socket buffers and 100 bots

All the bots of a worker reach it on one UDP port (`WebRtcServer`). With the
kernel's default buffer (208 KiB), that socket dropped about 400,000
datagrams in 30 s at 100 bots, and some transports then expired. `.env` sets
`MEDIASOUP_SOCKET_BUFFER=4194304` (4 MiB, this machine's
`net.core.rmem_max`); raise `rmem_max` and `wmem_max` with `sysctl` for
more. Watch the drops of port 44444 in the last column of `/proc/net/udp`.

Even without drops, at 100 bots `audio_video` with a cap of 10, a room on
one worker saturates that worker (one thread) on the 16-thread test laptop
(specs/010-mediasoup-platform/validation-report.md).

## Room time limit

The demo closes a room 10 minutes after it is created, and blocks its ID for
5 minutes, unless the room is unlimited. Every room is unlimited when the
server has no `UNLIMITED_ROOM_SECRET`. Leave it unset here.

If a server sets one, give it to the bots as `unlimited_room_secret`. It is
never logged, and shows as `[redacted]` in answers and reports.

## Origin header

The demo refuses (HTTP 403) a WebSocket whose `Origin` doesn't match its
`domain` (here `localhost`, or `MEDIASOUP_DOMAIN`). The bots send
`http(s)://<url host>`. If they reach the server under another name (for
example `ws://127.0.0.1:4443`), set `"origin": "http://localhost"` in
`platform_config`.

## See the room in a browser

The demo's web app is not built here, and it always connects with
`wss://<page host>:4443`, so the server needs TLS for it:

1. Make a certificate in `certs/` (it exists in the repository, empty: the
   container mounts it):
   ```bash
   openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj /CN=localhost \
     -keyout certs/key.pem -out certs/cert.pem && chmod 644 certs/*.pem
   ```
2. Uncomment `MEDIASOUP_TLS_CERT` and `MEDIASOUP_TLS_KEY` in `.env`, then run
   `docker compose up -d --force-recreate`. The log shows `computed HTTP
   Origin header: 'https://localhost:4443'`.
3. Start the app from the same commit, in a checkout of mediasoup-demo:
   `cd app && npm ci && npm start`. Then open
   `https://localhost:5555/?roomId=load010`, after accepting the
   certificate at `https://localhost:4443` once.
4. The bots then use `"url": "wss://localhost:4443", "insecure_tls": true`
   (live test: `MEDIASOUP_URL=wss://localhost:4443 MEDIASOUP_INSECURE_TLS=1`).

`certs/` is not committed. Comment the two variables out again for plain
`ws://` runs. The engine side of TLS was checked (bots join over `wss://`
with `insecure_tls`, and are refused without it); the app steps come from
the demo's sources and were not run here.

## Pinned version: bump it

```bash
docker compose build --build-arg MEDIASOUP_DEMO_COMMIT=<commit>
```

Then update the commit in `docker-compose.yml`'s image tag, in this README,
and in `config.mjs`'s header if the demo's `server/config.example.mjs`
changed. Base the new `config.mjs` on that example and keep the changes
listed in its header comment.

## Stop

```bash
docker compose down
```

## If the bots cannot connect

- **403 at the handshake**: see "Origin header".
- **The transports stay in `connecting`**: the server announces 127.0.0.1 in
  its ICE candidates. Set `MEDIASOUP_ANNOUNCED_ADDRESS` to the host's LAN
  address in `.env`, and `docker compose up -d`, when the bots run in
  another network namespace.
- **Transports closed after about 20 s** with "ICE consent expired" in the
  server log: the demo closes a transport after 20 s without a STUN consent
  check. The engine sends one every 5 s (research R14), so this should not
  happen. If it does, check that the bots' UDP traffic reaches the server.
