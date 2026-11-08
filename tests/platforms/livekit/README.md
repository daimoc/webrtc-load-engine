# Local LiveKit server (008)

A single-node LiveKit server, `livekit/livekit-server:v1.13.10`, for the
engine's LiveKit runs (specs/008-livekit-platform). It runs on the load
machine with host networking, so the bots reach it over loopback.

Ports: 7880/tcp (signaling and API), 7881/tcp (ICE/TCP), 50000–60000/udp
(media, one port per connection). They do not collide with
docker-jitsi-meet, but stop one server before a run on the other so they
don't share the CPU. The single-port UDP mode (`rtc.udp_port`) is not used:
at 100 bots on one machine it delays the media the server sends by more than
a second.

## Start

```bash
cd tests/platforms/livekit
cp env.example .env
sed -i "s/^LIVEKIT_API_SECRET=.*/LIVEKIT_API_SECRET=$(openssl rand -hex 24)/" .env
docker compose up -d
docker logs livekit-server 2>&1 | grep "starting LiveKit server"
```

`.env` is not committed (`.gitignore`).

## Use it in a test plan

```json
{
  "name": "lk-10-bots",
  "platform": "livekit",
  "test_plan": {
    "participants": 10, "ramp_up_seconds": 10, "duration_seconds": 60,
    "media": { "profile": "audio_video", "max_received_streams": 10 },
    "server_monitoring": { "containers": ["livekit-server"] },
    "platform_config": {
      "url": "ws://localhost:7880",
      "room": "load008",
      "api_key": "devkey",
      "api_secret": "<LIVEKIT_API_SECRET from .env>"
    }
  }
}
```

## Video codecs (014)

`media.video_codec` is `vp8` (default), `vp9`, `av1` or `h264`; LiveKit's
default `enabled_codecs` has all four, `livekit.yaml` does not change it.
With `media.simulcast`, VP8 and H.264 are sent as simulcast and VP9 and AV1
as SVC (`L3T3_KEY`); the bot announces its layers in `AddTrackRequest` and
asks for heights with `UpdateTrackSettings`. LiveKit v1.13 has no
scalability-mode field: it reads the SVC structure from the dependency
descriptor. LiveKit drops RTP padding when forwarding, so VP9 latency is
`frame_not_identified` (the VP9 frame marker is sent as padding; see
specs/014-video-codec-comparison/validation-report.md).

Live test (from the repository root):

```bash
set -a; . tests/platforms/livekit/.env; set +a
export LIVEKIT_URL=ws://localhost:7880 LIVEKIT_ROOM=load008 LIVEKIT_SERVER_CONTAINERS=livekit-server
go test -c -tags live -o live.test ./tests/live/
LIVEKIT_PARTICIPANTS=10 LIVEKIT_MEDIA=audio_video LIVEKIT_HOLD=60s ./live.test -test.v -test.run LiveKit
```

Each run writes its report (JSON and Markdown) to the repository's
`reports/` folder (feature 009).

## Stop

```bash
docker compose down
```

## If the bots cannot connect

The server announces `LIVEKIT_NODE_IP` (127.0.0.1 by default) in its ICE
candidates. If the bots run on another network namespace or the media
transport stays in `connecting`, set it to the host's LAN address in `.env`
and restart (`docker compose up -d`).
