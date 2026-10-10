# Quickstart: Jitsi XMPP Signaling

This guide shows how to configure the Load Engine to test a Jitsi Meet instance using the XMPP adapter.

## Prerequisites
- A running Jitsi Meet instance (public or private).
- Network access to the XMPP WebSocket (usually port 443 wss).

## Configuration

In your scenario definition (JSON), set the `platform` to `jitsi` and provide the `platform_config`:

```json
{
  "name": "Basic Jitsi Join",
  "platform": "jitsi",
  "test_plan": {
    "participants": 2,
    "ramp_up_seconds": 0,
    "duration_seconds": 30,
    "platform_config": {
      "xmpp_domain": "meet.jit.si",
      "room_name": "MyLoadTestRoom",
      "muc_domain": "conference.meet.jit.si",
      "debug": true
    }
  }
}
```

## Running the Test

```bash
# Start the engine
go run ./src

# Create the scenario (returns {"id": "scenario-<uuid>", "status": "created"})
curl -X POST http://localhost:8080/api/v1/scenarios \
  -H 'Content-Type: application/json' -d @scenario.json

# Trigger it; an invalid platform_config is rejected here with 400
curl -X POST http://localhost:8080/api/v1/scenarios/scenario-<uuid>/_execute

# Poll status
curl http://localhost:8080/api/v1/scenarios/scenario-<uuid>
```

For a single manual join without the API, use the CLI:

```bash
go run ./cmd/jitsi-connect --domain meet.example.org --room MyLoadTestRoom --log-xmpp
```

The CLI runs the same engine with one participant: it joins the room, answers
the bridge's offer and stays until Ctrl+C (or `--duration` seconds).

To check a deployment end to end (join, Jingle negotiation, ICE+DTLS
established with the bridge, clean leave), run the live smoke test with the
room link. For deployments with token authentication, the link's `jwt`
parameter is used (or set `JITSI_TOKEN`):

```bash
JITSI_ROOM_URL='https://meet.example.org/MyLoadTestRoom?jwt=<token>' JITSI_LOG_XMPP=1 \
  go test -tags live -count=1 -v ./tests/live/
```

The WebSocket defaults to `wss://<domain>/xmpp-websocket` and the MUC to
`conference.<domain>`, even when the web app lives under a path such as
`/meet/`. Multi-tenant deployments set `JITSI_WS_URL` and `JITSI_MUC_DOMAIN`
from their `config.js`. With two participants, Jitsi clients also offer a
direct P2P session; the bot declines it and only negotiates with the bridge.

Against a local [docker-jitsi-meet](https://github.com/jitsi/docker-jitsi-meet)
(XMPP domain `meet.jitsi`, plain WebSocket on port 8000), with N bots. Jicofo
only starts a conference with at least two participants, so use at least 2:

```bash
JITSI_ROOM_URL=http://localhost:8000/loadtest JITSI_XMPP_DOMAIN=meet.jitsi \
JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
JITSI_PARTICIPANTS=100 JITSI_RAMP_UP=0 JITSI_HOLD=30s \
  go test -tags live -count=1 -v ./tests/live/
```

Bots join with the occupant nick Prosody expects (the first 8 characters of
their anonymous JID) and show their peer ID as display name in the Jitsi UI.
They advertise their features to Jicofo (disco#info and entity capabilities)
and open the bridge's "JVB data channel" when the offer includes SCTP. They
advertise JSON-encoded sources, as browsers do: Jicofo only includes the
data (SCTP) content in offers to such clients.

To make the bots send media, add `"media": {"profile": "audio_video"}` (or
`"audio"`) to the test plan, `--media audio_video` to `jitsi-connect`, or
`JITSI_MEDIA=audio_video` to the live test; see
`specs/004-synthetic-media/quickstart.md`.

## Observing a Run

Per-peer progress is exported on `/metrics`, labelled by `scenario_id`:

- `webrtc_peers{state=...}`: peers per lifecycle state (`connecting`, `connected` = joined the room, `established` = ICE+DTLS up, `failed`, `disconnected`)
- `webrtc_peer_signaling_connect_seconds`, `webrtc_peer_negotiation_seconds`, `webrtc_peer_transport_connect_seconds`
- `webrtc_peer_failures_total{stage=signaling|negotiation|transport|signaling_lost}`

- `webrtc_media_*{kind=audio|video}`: media sent and received, loss, jitter, round trip, remote streams offered, stalls and send lateness (specs/004-synthetic-media/contracts/metrics.md)

Join failures are logged with a cause: `xmpp server unreachable`, `xmpp authentication failed`
(the deployment does not offer SASL ANONYMOUS), `conference focus unavailable`, `room unavailable`,
`removed from room` (kicked/banned) or `xmpp connection lost`.
