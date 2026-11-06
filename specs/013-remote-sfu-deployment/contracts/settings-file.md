# Contract: Settings file (013)

Path: `$XDG_CONFIG_HOME/webrtc-load-engine/sfu-remote/<host>.env`, by default under
`~/.config`. Mode 0600. Sourced with `set -a; . <file>; set +a`.

It uses the variables the live tests already read, plus `DOCKER_HOST`:

| Server | Variables |
|---|---|
| All | `DOCKER_HOST=ssh://<target>` |
| Jitsi | `JITSI_ROOM_URL`, `JITSI_XMPP_DOMAIN=meet.jitsi`, `JITSI_MUC_DOMAIN=muc.meet.jitsi`, `JITSI_WS_URL`, `JITSI_SERVER_CONTAINERS` |
| LiveKit | `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`, `LIVEKIT_SERVER_CONTAINERS` |
| mediasoup | `MEDIASOUP_URL`, `MEDIASOUP_ORIGIN`, `MEDIASOUP_SERVER_CONTAINERS` |
| Janus | `JANUS_URL`, `JANUS_SERVER_CONTAINERS` |

Rooms, bot counts and durations stay the run's own choice. The file is the only
copy of the secrets outside the machine, and `remove` deletes it.
