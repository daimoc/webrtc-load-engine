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

Added by 015 (FR-012): `<PLATFORM>_SERVER_SETTINGS` for every server
(`JITSI_SERVER_SETTINGS`, `LIVEKIT_SERVER_SETTINGS`,
`MEDIASOUP_SERVER_SETTINGS`, `JANUS_SERVER_SETTINGS`): the settings the
remote overlay changes from the server's defaults and that change results,
as `name=value` pairs joined by commas and sorted by name (today
`JITSI_SERVER_SETTINGS='JVB_CC_TRUST_BWE=false'`); present and empty when
the overlay changes none. The live tests and `jitsi-connect` put it in the
test plan's `server_settings` as stated by the deployment. A file written
before 015 has no such variable: the run then says the server's settings
are not known to it.

Rooms, bot counts and durations stay the run's own choice. The file is the only
copy of the secrets outside the machine, and `remove` deletes it.
