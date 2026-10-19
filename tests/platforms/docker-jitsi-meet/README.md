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

## Token authentication

Uncomment the JWT block of `.env` (set `JWT_APP_SECRET`), then
`docker compose up -d` and run the bots with
`JITSI_APP_SECRET=... --jwt-app-id my_app`.

## Stop

```bash
docker compose down
```
