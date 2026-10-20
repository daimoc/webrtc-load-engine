# Research: mediasoup-demo Platform

**Date**: 2026-10-20. **Source read**: `versatica/mediasoup-demo` at commit
`899109e4046b45da1623a3e4c65a742040479439` (2026-10-19, "update
mediasoup-client to 3.25.1"), server in TypeScript (`server/src`),
`mediasoup ^3.30.1`, `protoo-server ^4.0.8`. The repository has no release
tags and no Dockerfile.

## R1. Signaling: protoo over WebSocket

**Decision**: one WebSocket per bot to
`<url>/?roomId=<room>&peerId=<peer ID>` (plus `unlimitedRoomSecret` and
`usePipeTransports` when configured, R9), with the WebSocket subprotocol
`protoo`. Messages are protoo JSON text frames:

- request: `{"request":true,"id":N,"method":M,"data":{...}}`
- response: `{"response":true,"id":N,"ok":true,"data":{...}}`, or
  `{"response":true,"id":N,"ok":false,"errorCode":C,"errorReason":R}`
- notification: `{"notification":true,"method":M,"data":{...}}`

The handshake must carry an `Origin` header whose scheme (`http` or
`https`) and hostname match the server's configured `domain`
(`WsServer.ts`); otherwise the server answers 403, as it does without the
`protoo` subprotocol. The adapter sends `http(s)://<url host>` (ws→http,
wss→https), or the `origin` setting when the server's domain differs from
the address the bots use. An error response carries no `ok` field: any
response without `ok: true` is an error.

Both sides send requests: the bot sends `getRouterRtpCapabilities`,
`createWebRtcTransport`, `connectWebRtcTransport`, `join`, `produce`; the
server sends `newConsumer` and `newDataConsumer`, which the bot must answer.
Notifications used: from the server `consumerClosed`, `peerClosed`
(logged), the rest ignored; from the bot `resumeConsumer` (R6). There is no
leave message: closing the WebSocket removes the peer and closes its
transports, producers and consumers.

**Rationale**: it is the demo's own client protocol
(`server/src/signaling/protooMessages.ts`). `nhooyr.io/websocket` (already
used by 003 and 008) dials with a subprotocol, and `encoding/json` is enough
for the framing. A small `protoo` client type in the adapter matches
responses to request IDs and dispatches server requests.

**Alternatives considered**: the demo's HTTP "broadcaster" API (REST,
`ApiServer.ts`). It needs no WebSocket, but it is a different path in the
server (BroadcasterPeer), and the spec asks for the web client's signaling.
`protoo-client` has no Go port.

## R2. Mapping mediasoup's parameters onto the SDP boundary

mediasoup does not speak SDP: the server hands out ICE, DTLS and RTP
parameters as JSON, and `mediasoup-client` builds SDP locally from them. The
adapter does the same, so the peer keeps seeing only SDP offers and answers
(constitution II).

**Decision**: a `sdp.go` in the adapter, using `pion/sdp/v3` (already a
direct requirement), does two translations:

- **Receive side (server → bot)**: the adapter builds a remote *offer* from
  the receiving transport's `iceParameters`, `iceCandidates`,
  `dtlsParameters` and `sctpParameters`, plus one `sendonly` media section
  per accepted consumer, from the consumer's `rtpParameters` (mid, codec and
  RTX payload types, `rtcpFeedback`, header extensions, SSRCs and the
  `ssrc-group:FID`, CNAME). Session attributes: `ice-lite`, `ice-ufrag`,
  `ice-pwd`, `fingerprint` (the `sha-256` one), `setup:actpass`,
  `group:BUNDLE` of all mids. The bot answers with the existing `OnOffer`
  (first time) and `Renegotiate` (later). From the first answer the adapter
  reads the bot's fingerprint and DTLS role and sends
  `connectWebRtcTransport`. *Implementation finding (T006)*: Pion answers
  an ICE-lite `actpass` offer with `setup:passive`, so the bot is the DTLS
  server on its receiving transport (`role: "server"`); the adapter reads
  the role from the answer, as `mediasoup-client` does from its local SDP.
- **Send side (bot → server)**: the adapter takes the bot's publish offer
  from `CreatePublishOffer`, reads its fingerprint for
  `connectWebRtcTransport` (`role: "client"`), and for each media section
  turns the offer into a producer's `rtpParameters` (mid, codec with its
  payload type and RTCP feedback, RTX if offered, header extensions the
  router supports, encodings `[{ssrc, rtx:{ssrc}}]`, `rtcp.cname`). It then
  builds the remote *answer* from the sending transport's parameters with
  `setup:passive` (the server is the DTLS server) and, per section, the same
  codec and extensions, `recvonly`. `ApplyPublishAnswer` applies it.

Only the UDP candidates are put in the SDP. The bots run on the server's
machine, and Pion's ICE-TCP is passive only.

Local candidates are not sent: mediasoup is ICE-lite and learns the bot's
address from its STUN checks. The `TrickleFunc` given to the peer drops
them.

**Rationale**: no boundary change (the 008 boundary already has a session
the peer offers and a session the remote side offers). It mirrors
`mediasoup-client`'s `RemoteSdp` and `ortc` code, so the server sees the same
messages as from a browser.

**Alternatives considered**: a mediasoup-specific boundary (passing ICE,
DTLS and RTP parameters to the peer). It was rejected by constitution II and
III: the media layer would learn mediasoup's model.

## R3. Opening the receiving session at join

The peer is `established` once *every open session* is connected (008), and
the session the remote side offers is the main one. On mediasoup, that
session gets media sections only when another peer produces, so the first
bot of a room (or a `receive_only` bot alone) would never get an offer and
never be established.

**Decision**: right after creating the receiving transport, the adapter
calls `OnOffer` with an offer whose only section is
`m=application 9 UDP/DTLS/SCTP webrtc-datachannel` (mid `datachannel`, as
`mediasoup-client`: consumer mids are `"0"`, `"1"`…; `sctp-port`
and `max-message-size` from the transport's `sctpParameters`), then
`connectWebRtcTransport`. The receiving transport connects at join for
every bot, whoever else is in the room. Consumer sections are added after
it, in the order they are accepted. The bot opens no data channel and
refuses `newDataConsumer` (the demo's chat "bot", R6).

**Rationale**: the established rule stays the same for every platform. The
transport is up before the first consumer is accepted, so the consumer's
first packets are not lost to ICE. It is what `mediasoup-client` does for a
receiving transport that has a data consumer. The demo creates every
transport with SCTP enabled, so the section is always accepted. The cost is
one SCTP association per bot with heartbeats only.

**Alternatives considered**:
- Counting the bot as established without the receiving session. This
  needs a per-platform rule in the peer (constitution V).
- Accepting the demo's chat bot data consumer to get a first section. This
  depends on a demo feature and opens a channel we do not use.

## R4. Join sequence

**Decision**: `Connect` runs, with the context's deadline:

1. Dial the WebSocket (R1). The server closes a peer that does not `join`
   within 10 s (`JOIN_TIMEOUT_MS`).
2. `getRouterRtpCapabilities`. Fail the join if the router has no
   `audio/opus` (48000/2) or `video/VP8` for a kind the bot sends.
3. `createWebRtcTransport {forceTcp:false, appData:{direction:"consumer"}}`,
   then open the receiving session (R3).
4. If the bot sends: `createWebRtcTransport {direction:"producer"}`.
5. `join {displayName: <peer ID>, device: {flag:"unknown",
   name:"webrtc-load-engine", version}, rtpCapabilities}`. The capabilities
   are derived from the router's (R5). The receiving transport must exist
   before `join`: on join, the server consumes the other peers' producers
   for the newcomer only if it has a consumer transport (`Peer.consume`).
6. If the bot sends: `CreatePublishOffer`, `connectWebRtcTransport`, one
   `produce {kind, rtpParameters, appData:{source:"mic"|"webcam"}}` per
   track, `ApplyPublishAnswer`.

`Connect` returns after step 6. `newConsumer` requests that arrive from
step 5 on are handled by the receive cap (R6) as they come.

**Rationale**: it is the order of the demo's web client and of the server's
checks.

## R5. RTP capabilities, codecs and header extensions

**Decision**: the bot's `rtpCapabilities` in `join` are the router's
capabilities filtered to `audio/opus`, `video/VP8` and VP8's `video/rtx`,
keeping the router's `preferredPayloadType`s. mediasoup matches a
consumer's RTX codec by the router's payload types. Header extensions kept:

- `urn:ietf:params:rtp-hdrext:sdes:mid`
- `http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time`
- `http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01`
  with `transport-cc` feedback: the bots already send their own TWCC
  feedback on the receiving session (005)
- `urn:ietf:params:rtp-hdrext:ssrc-audio-level` (audio)

Consumers are therefore VP8 and Opus with the router's payload types. Pion
answers with the remote payload types. A browser in the room that sends
another codec (VP9, H264) cannot be consumed by the bots: the server checks
`canConsume` and never offers it, and the cap counts only what is offered.

For producers, the payload types are those of the bot's offer (Pion's
defaults), and mediasoup maps them. Header extensions are the offer's
extensions that the router supports, with the offer's IDs.

**Rationale**: it is what `mediasoup-client` computes
(`getExtendedRtpCapabilities`). No VP9 or H264 is needed: the bots send VP8
only (004).

## R6. Receive cap: accept, decline, resume

The server creates every consumer *paused*, sends `newConsumer`, and
resumes the consumer only if the bot accepts. If the bot answers with an
error, the consumer stays on the server, paused, and the bot can resume it
later with the `resumeConsumer` notification (`Peer.consume`,
`handleProtooNotification`).

**Decision**: `consumers.go`, per bot and per kind:

- On `newConsumer`, if fewer than `ReceiveLimit()` consumers of that kind
  are active (or the limit is 0): add its section to the receive offer,
  `Renegotiate`, then accept. The order matters: the section must exist
  before the first packet. The server resumes the consumer after the
  response.
- Otherwise, answer with an error (`errorCode 403`, reason "receive cap"),
  and keep the consumer in the kind's waiting list (consumer ID,
  `rtpParameters`, arrival order).
- On `consumerClosed`: if the consumer was active, mark its section
  `inactive` (no SSRC lines, so the peer stops counting it as offered),
  then take the earliest waiting consumer of that kind. Add its section,
  `Renegotiate`, then notify `resumeConsumer`. If it was waiting, drop it
  from the list.
- `newDataConsumer` and consumers of other kinds: declined with an error,
  and the bot goes on.

Inactive sections are never reused for a new consumer. A section's mid is
the consumer's (mediasoup gives each consumer a new mid), and the count
grows only with streams that end during a run.

All SDP work of a bot (initial offer, renegotiations, publish) goes through
one goroutine per bot, in arrival order, so offers never overlap. The
server may send a bot's `newConsumer` requests in parallel.

**Rationale**: the cap is enforced by the server not forwarding media, not
by the bot dropping packets, as on Jitsi and LiveKit (008 FR-005). The
server is unchanged, and each waiting stream costs only a paused consumer.
The waiting order is FIFO, as on LiveKit.

**Alternatives considered**:
- Accepting everything and pausing beyond the cap with `pauseConsumer`.
  The bot would get the first packets of every stream, and the cap would be
  briefly wrong after each join.
- Reusing inactive sections, as `mediasoup-client` does. Pion's support for
  recycling an m-section under a new mid is not needed for the run lengths
  of this feature.

## R7. Lifecycle and loss

**Decision**:

- `Connect` succeeds when the join sequence (R4) ends. `connected` and then
  `established` follow from the transports, as on LiveKit (both sessions
  for a sending bot, the receiving session alone for `receive_only`).
- Any request rejected during the join fails `Connect` with the method and
  the server's `errorReason`. A WebSocket refused at the handshake fails it
  with the HTTP status: for example the demo's "room is blocked" after a
  time-limited room expired, or 400 for a missing room or peer ID.
- A WebSocket closed or failing after `Connect` calls
  `OnSignalingLost`. This includes a room closed by its time limit (R9) and
  the server going away.
- `Disconnect` closes the WebSocket. The server then closes the bot's
  transports and tells the other peers (`peerClosed`, `consumerClosed`).

No keepalive is sent. The server's WebSocket library sends pings, which
`nhooyr.io/websocket` answers while the read loop runs.

**Rationale**: constitution V, with the same states and failure stages as
on the other platforms.

## R8. Latency and quality measures

**Decision**: no change to the 006 measures. The latency measure
identifies frames by their RTP timestamp. mediasoup's `SimpleConsumer`
rewrites SSRCs and sequence numbers; it is expected to forward RTP
timestamps unchanged (not checked in the mediasoup worker's source), so
latency should be available. The live test checks it (quickstart step 4). If it is
missing, the summary gives the 008 `latency_reason`, and FR-011 is still
met. Freezes, frame rate, audio gaps, MOS and keyframe requests (PLI on the
receiving session, forwarded by mediasoup to the producer) are computed
from what the bots receive, as before.

**Rationale**: FR-010 and FR-011. The summary format is unchanged, with no
new field.

## R9. Room time limit and pipe transports

The demo closes a room 10 minutes after its creation unless the room is
"unlimited": the `unlimitedRoomSecret` URL parameter must equal the
server's `UNLIMITED_ROOM_SECRET` environment variable. When neither is set,
both are undefined and every room is unlimited (`Server.getOrCreateRoom`).
For the next 5 minutes after it expires, the room ID is blocked.

**Decision**:

- The `platform_config` key `unlimited_room_secret` (optional, secret,
  redacted) is passed as `unlimitedRoomSecret`.
- The local setup does not set `UNLIMITED_ROOM_SECRET`, so its rooms are
  unlimited.
- A room closed by its time limit makes the bots lost (R7), with the
  close's reason in the log.

The demo runs a room on one router, so on one mediasoup worker, which is one
CPU thread for all its media. With `usePipeTransports=true`, it uses two
routers on two workers (producers and consumers) piped together.

**Decision**: the optional key `use_pipe_transports` (bool, default false)
is passed as `usePipeTransports=true`. The local setup runs 2 workers so
that both modes work, and documents that the default mode puts the whole
room on one worker. The run report already carries the `platform_config`
(009), so the mode used is in every report.

**Rationale**: the 100-bot run (SC-002) may saturate one worker. That is
the demo's real behaviour and a valid result, but the operator must be able
to tell it apart, and to try the demo's own option. Both keys are URL
parameters of the demo, not new behaviour.

## R10. Configuration and validation

**Decision**: `platform_config` for `mediasoup`
([contracts/mediasoup-config.yaml](contracts/mediasoup-config.yaml)):

- `url` (required): `ws://`, `wss://`, `http://` or `https://`; http(s) is
  turned into ws(s)
- `room` (required)
- `origin` (string): the handshake's `Origin` header (R1)
- `insecure_tls` (bool, default false): do not check the server's
  certificate (wss)
- `unlimited_room_secret` (string, secret)
- `use_pipe_transports` (bool)

Unknown keys are rejected, as for LiveKit. At creation, `Validate` also
rejects `media.simulcast` and `media.requested_heights` for
`mediasoup` (FR-008), with LiveKit's messages and the platform's name
(`media.simulcast is not supported on platform mediasoup`). The
catalog gets a `mediasoup` entry with `SecretKeys:
["unlimited_room_secret"]`.

**Rationale**: the same shape as 008's `livekit` entry, and redaction in
the API and the reports (009) works through `SecretKeys` with no new code.

## R11. Local setup

**Decision**: `tests/platforms/mediasoup/`:

- `Dockerfile`: `node:22-bookworm`. It clones `mediasoup-demo` at the
  pinned commit (build argument `MEDIASOUP_DEMO_COMMIT`, default `899109e…`),
  runs `npm ci` in `server` (installs the exact `package-lock.json` versions;
  mediasoup fetches its prebuilt worker, and the image has the build tools
  if it must build it), and runs the server's TypeScript build.
- `config.mjs`: our server config, mounted in the container:
  - plain HTTP and WebSocket on `127.0.0.1:4443` (no TLS)
  - `numWorkers` from `MEDIASOUP_NUM_WORKERS` (default 2)
  - the WebRTC server on `127.0.0.1`, UDP and TCP port 44444 + worker index
  - Opus and VP8 (the example's codec list is kept)
  - log level `warn`
- `docker-compose.yml`: container `mediasoup-demo`, host network,
  `restart: unless-stopped`, environment from `.env`.
- `env.example`: `MEDIASOUP_NUM_WORKERS`, and `MEDIASOUP_ANNOUNCED_ADDRESS`
  for bots on another namespace. `.env` is not committed.
- `README.md`: start, settings, container name, workers, the room
  time-limit note, stop.

With host networking and 127.0.0.1, the bots reach the server over
loopback, as for LiveKit, and the server's load is its container (node plus
the worker processes; 007 reads the container's cgroup).

**Rationale**: FR-013 and FR-014. No official image exists, so building
from a pinned commit with its lock file is the only reproducible way. The
demo's web client is not built: it is not needed by the bots. For checks in
a browser (US1 independent test), the README points at the demo's `app`
started separately or at the server's room list in its log.

**Alternatives considered**: third-party mediasoup-demo images, rejected
because they are unpinned and unofficial. Serving TLS with a generated
certificate is not needed on loopback; the `insecure_tls` setting covers
TLS servers elsewhere.

## R12. Tests

**Decision**:

- Unit tests:
  - protoo framing and request matching
  - config validation and redaction
  - SDP building (receive offer with an application section and
    consumers; publish answer)
  - offer to `rtpParameters`, on golden SDPs from Pion and golden JSON
    parameters copied from the demo's messages
  - receive cap transitions (accept, decline, close, promote; FIFO)
- A fake protoo server on `httptest`, with real Pion on both ends. It plays
  mediasoup's side with a Pion PeerConnection that answers as `ice-lite`
  and forwards the producers' RTP to the consumers. It drives real peers
  through the adapter: join, produce, consume with the cap, close and
  promote, data consumer declined, rejected request, WebSocket closed.
- The engine and catalog tests are extended with the platform.
- A live test, `tests/live/mediasoup_live_test.go` (`-tags live`,
  `MEDIASOUP_*` variables), mirrors the LiveKit one against the local setup
  for SC-001 to SC-003.

**Rationale**: constitution "Test Discipline": adapters have contract
tests. The fake server's job is protocol order and SDP correctness. The
live test catches what only the real mediasoup does: timestamps, PLI,
pacing.

## R14. ICE consent freshness (found in T016)

mediasoup-demo creates every transport with `iceConsentTimeout: 20`
(`Room.ts`, not configurable). mediasoup closes a transport that has not
received a valid STUN Binding request for 20 s (RFC 7675). Browsers send one
every ~5 s. Pion (`pion/ice` v2.3.38, `checkKeepalive`) sends its keepalive
only when the selected pair has been idle, so a bot with media flowing never
refreshes consent, and the first 10-bot run lost every bot at 20 s.

**Decision** (the user's, 2026-10-20): the engine sends browser-like consent
checks on every PeerConnection, for all platforms (`src/execution/consent.go`):
- a Binding request every 5 s ±20% on the selected pair;
- it carries the remote ufrag and password from the remote description, and
  the ICE role and local priority;
- it is sent from the pair's own socket: the PeerConnection's UDP sockets
  are opened through a wrapper of the standard network (`SettingEngine.SetNet`);
- the wrapper drops the answers before Pion reads them (Pion would log them
  as unknown transactions).

**Rationale**: the demo stays unmodified, and the bots behave like browsers
toward any server that enforces consent. The cost is one ~100-byte packet
per transport every 5 s.

**Alternatives considered**:
- Patching the demo's timeout to 0. It changes the system under test, and
  other servers that enforce consent would still drop the bots.
- Forking `pion/ice`.

## R13. CPU comparison method

**Decision**: SC-002 is checked with alternated runs, LiveKit and mediasoup
taking turns with the same plan in the same machine state. This follows the
009 A/B run of 2026-10-19: on this laptop the same 100-bot load costs three
times more CPU with the desktop on than with the screen locked.

**Rationale**: a fixed CPU budget on this machine would be meaningless.
