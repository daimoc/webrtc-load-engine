# Local Jitsi Meet

A minimal [docker-jitsi-meet](https://github.com/jitsi/docker-jitsi-meet)
for the engine's Jitsi runs: web, prosody, jicofo and jvb (`unstable`
images), nothing else. The bots join over XMPP WebSocket
(`ws://localhost:8000/xmpp-websocket`, XMPP domain `meet.jitsi`, MUC
`muc.meet.jitsi`).

Ports: 8000/tcp, 8443/tcp (web), 10000/udp (media), 127.0.0.1:8080
(bridge Colibri REST, so the API server of `go run ./src` cannot run at the
same time) and 127.0.0.1:8888 (jicofo REST). Stop the LiveKit server of
`tests/platforms/livekit` before a run so they don't share the CPU.

## Start

```bash
cd tests/platforms/docker-jitsi-meet
./setup.sh          # .env with fresh passwords, ~/.jitsi-meet-cfg directories
docker compose up -d
```

Open `https://localhost:8443/<room>` (self-signed certificate). `.env` is
not committed (`.gitignore`). Changes to `.env` take effect after
`docker compose up -d` (the containers rewrite their configuration at each
start).

## Use it

```bash
# from the repository root
go run ./cmd/jitsi-connect --domain meet.jitsi --muc muc.meet.jitsi \
  --ws ws://localhost:8000/xmpp-websocket --room test --participants 2 --media audio_video \
  --server-containers docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1

JITSI_ROOM_URL=http://localhost:8000/loadtest JITSI_XMPP_DOMAIN=meet.jitsi \
JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
JITSI_PARTICIPANTS=10 go test -tags live -count=1 -v -run Jitsi ./tests/live/
```

The live test lists its other variables at the top of
`tests/live/jitsi_live_test.go`.

Each run writes its report (JSON and Markdown) to the repository's
`reports/` folder (feature 009).

## Video codecs (014)

`media.video_codec` (`JITSI_VIDEO_CODEC`, `jitsi-connect --video-codec`) is
`vp8` (default), `vp9`, `av1` or `h264`. Jicofo `stable-11248` offers all
four by default (`ENABLE_CODEC_*` is not set in `.env`), with the AV1
dependency descriptor. With `media.simulcast`, VP8 and H.264 are sent as
simulcast (SIM group) and VP9 and AV1 as SVC (one source and its RTX);
requested heights go through `ReceiverVideoConstraints`.

Limits found (specs/014-video-codec-comparison/validation-report.md):
- H.264 simulcast: the bridge forwards only the lowest layer (it has no
  H.264 packet parser); reports set `layers_not_forwarded`.
- The bridge drops RTP padding, so VP9 latency is `frame_not_identified`.
- At 10 bots locally, the default bandwidth estimate made the bridge stop
  some streams in VP8 and AV1 runs; the 014 figures are with
  `JVB_CC_TRUST_BWE=false` (a compose override; the remote overlay set it
  until 015). This setup keeps the default, and so does the remote overlay
  since 015.

## Bandwidth estimate (015)

The suspended streams of 014 came from the bots: the bridge sends some
packets twice, each copy with its own transport-wide sequence number, and
the bots' WebRTC stack dropped the copy before acknowledging it, so the
bridge counted it lost and lowered its estimate. Since 015 the bots
acknowledge every packet that reaches their socket, and with the bridge's
default settings VP8 runs at 10 bots have no suspended stream
(specs/015-bandwidth-estimate-parity/validation-report.md, SC-003).

Left: with AV1 and all bots joining at once (ramp-up 0), the bridge may
still suspend some bots' streams for the first seconds (2 runs in 5); with
a 10 s ramp-up it did not. A headless browser in the same room was never
suspended. Use a ramp-up for AV1 runs.

On a remote machine (013, a LAN), the bridge starts every receiver at
2.5 Mbit/s and suspends streams of a 10-bot room for its first 10 to 20 s,
a browser's as long as the bots'; after that, no stream is suspended with
the corrected bots, where uncorrected ones stayed suspended for the rest of
the run.

The bridge's estimate per receiver is readable during a run on its private
debug interface, published on `127.0.0.1:8080`:
`python3 -I scripts/read-estimates.py jitsi` writes it once per second.

## Token authentication

Uncomment the JWT block of `.env` (set `JWT_APP_SECRET`), then
`docker compose up -d` and run the bots with
`JITSI_APP_SECRET=... --jwt-app-id my_app`.

## Stop

```bash
docker compose down
```
