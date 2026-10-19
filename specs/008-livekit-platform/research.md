# Research: LiveKit Platform (008)

Sources: `livekit/protocol` v1.52.1 protobufs (`livekit_rtc.proto`,
`livekit_models.proto`); the LiveKit server v1.13.10 `go.mod`, which pins
protocol v1.52.2-pre and Pion v4.2.18 (LiveKit fork); and the Go SDK's
`signalling/` package (`server-sdk-go`, main, protocol constant 17). All
were read on 2026-10-17.

## R1. Signaling transport and session model

- **Decision**: one WebSocket per bot to `{url}/rtc`.
  - **Query**: `access_token`, `auto_subscribe=0`, `protocol=17`, `sdk=go`, `version=<engine version>`, `os=linux`. The token is sent only in the query, and logs redact it (as Jitsi's `redactToken`).
  - **Frames**: binary protobuf `SignalRequest` from the bot, `SignalResponse` from the server.
  - **Sessions**: dual-PC model. The bot offers the publish session (`SignalRequest.offer`, answer in `SignalResponse.answer`). The server offers the subscribe session (`SignalResponse.offer`), and the bot answers with `SignalRequest.answer`. `TrickleRequest.target` (PUBLISHER=0, SUBSCRIBER=1) routes candidates. `candidateInit` is the JSON of an RTCIceCandidateInit (`candidate`, `sdpMid`, `sdpMLineIndex`).
- **Rationale**: dual PC is the model every LiveKit SDK has used since v1 and that the server still supports. Its subscribe side is exactly the server-offer flow the engine already handles for Jitsi, so only the publish side is new.
- **Checked first in implementation**: the pinned server answers `protocol=17` with `JoinResponse.subscriber_primary = true` and a first subscriber offer without the client publishing. If it does not, lower the protocol number until it does, and record the value.
- **Alternatives considered**:
  - Single-PC mode (`/rtc/v1`, `WrappedJoinRequest` with the publisher offer, `MediaSectionsRequirement`): the client offers everything, including receive sections sized by the server. It is newer, little documented, and would turn the receive path into client offers; for a later version if needed.
  - The Go SDK: brings Pion v4 next to the engine's v3 and its own media pipeline (constitution II/IV).

## R2. Protobuf encoding without the LiveKit module

- **Decision**: a hand-written codec in `src/platform/livekit/proto.go` over `google.golang.org/protobuf/encoding/protowire`, covering only the messages and fields of [contracts/signaling-messages.md](contracts/signaling-messages.md). Field numbers are copied from protocol v1.52.1 with a comment per message. Unknown fields and messages are skipped (`protowire.ConsumeFieldValue`), so speaker updates, connection quality and newer fields cost one skip.
  - **Golden tests**: a throwaway generator, run once in the scratchpad as its own module, marshals reference messages with the official `livekit/protocol` types. Its outputs are committed under `testdata/` as `.bin` files, and the codec must encode them byte for byte and decode them field for field. The generator is not committed and is not a dependency.
- **Rationale**:
  - `livekit/protocol` declares `go 1.26` and requires Pion v4, newer `pion/rtp`/`rtcp`/`interceptor` (shared with Pion v3: MVS would upgrade them under v3), Redis, gRPC, OpenTelemetry and psrpc. That is far beyond 12 messages.
  - `protowire` is in the module graph already (Prometheus' client_model) and is the supported low-level API.
- **Alternatives considered**:
  - `protoc` + `protoc-gen-go` on vendored `.proto` files: needs `protoc` on every developer machine (not installed), and the generated code imports the full option protos (`logger/options.proto`).
  - `bufbuild/protocompile` + `dynamicpb` at runtime: a new dependency, plus reflection per message.

## R3. Access tokens

- **Decision**: the adapter signs one HS256 JWT per bot. The signer (header, base64url, HMAC-SHA256) moves from `src/platform/jitsi/jwt.go` to `src/platform/internal/jwt`, and Jitsi keeps its claims and tests unchanged.
  - **Claims**: `iss` = API key, `sub` = peer ID (identity), `name` = peer ID, `nbf` = now − 10 s, `exp` = now + `token_ttl` (default 6 h), `jti` = peer ID, `video` = `{room, roomJoin: true, canPublish: true, canSubscribe: true, canPublishData: false}`.
  - **Ready-made token**: used as is, for all bots. The server then rejects a second bot with the same identity, so this mode is only for single-bot or debugging runs, and the config documents it.
- **Rationale**: same mechanism and secret handling as 003; no JWT library.
- **Alternatives considered**: `golang-jwt/jwt`: a dependency for 30 lines already written and tested.

## R4. A publish session in the peer

- **Decision**: `PeerMedia.PublishSession` (set by the engine when the adapter implements `platform.PublishSessionAdapter` and returns true) moves the peer's local tracks from the answered session to a second Pion PeerConnection, the publish session.
  - **Setup**: the publish session is built by `newPeerConnection` with the same codecs, NACK responder and RTCP reports, but without the receive gate and TWCC generator, because nothing is received on it. Its transceivers are `sendonly` and use the same `TrackLocalStaticRTP` tracks, so the scenario's sources write each packet once per track, as today. RTCP readers (keyframe requests, round trip) run on its senders.
  - **Answered session**: keeps everything it does today (receive gate, TWCC feedback, `OnTrack` quality). It carries no local track and answers `recvonly`.
  - **New boundary methods** ([contracts/platform-boundary.md](contracts/platform-boundary.md)): `CreatePublishOffer(trickle)` gathers like `OnOffer` (1 s, rest trickled) and returns the offer SDP. `ApplyPublishAnswer(sdp)` and `OnPublishICECandidate(mid, candidate)` complete it. `LocalVideoSize()` returns the size LiveKit's `AddTrackRequest` wants.
  - **Lifecycle**: the peer is `established` once every open session's transport is connected, and `failed` if either fails. `transport_connect_seconds` is observed when the last one connects, from the first offer or answer handed to signaling. `negotiation_seconds` gets one observation per session.
- **Rationale**: a platform-neutral addition. The media code still sees only SDP and candidates. Writers and tracks are shared, so CPU per sent packet is unchanged. mediasoup-demo (009) also sends on a session the client offers.
- **Alternatives considered**:
  - One PeerConnection with renegotiation in both directions (glare between client and server offers on one PC): not how LiveKit's dual-PC mode works.
  - A second `Peer` per bot: it would duplicate lifecycle and metrics, and count each bot twice.

## R5. Publishing the bot's tracks

- **Decision**: after `JoinResponse`, a sending bot:
  1. Calls `CreatePublishOffer`.
  2. Reads each media section's `msid` track ID from the offer.
  3. Sends one `AddTrackRequest` per track: `cid` = track ID, `name` = kind, `type` AUDIO/VIDEO, `source` MICROPHONE/CAMERA, `width`/`height` from `LocalVideoSize()` for video, `disable_red` = true for audio (Opus only, as the bots send).
  4. Waits for a `TrackPublishedResponse` per `cid`, then sends the offer.
  5. Applies the server's answer.

  The join is reported complete only after the answer is applied (FR-003). A missing `TrackPublishedResponse` or answer within the connect timeout fails the join with its stage.
- **Rationale**: the order the SDKs use: the server must know a track's `cid` before its section arrives. Parsing `msid` keeps the boundary free of track IDs.
- **Alternatives considered**: sending the offer before the track requests: the server then ignores the unknown sections.

## R6. Subscribe session

- **Decision**: the first `SignalResponse.offer` goes to `OnOffer` (its answer goes back as `SignalRequest.answer`, with later candidates trickled as SUBSCRIBER). Later offers go to `Renegotiate`. A received `SessionDescription.id` is echoed in the answer. Offers are handled in arrival order on the read loop's goroutine (serialized by the peer's `negMu`).
- **Rationale**: identical to the Jitsi path, so the receive gate (offered SSRCs only), TWCC feedback and quality measures apply unchanged.
- **Checked first in implementation**: LiveKit's subscriber offers carry `a=ssrc` lines for each down-track. The receive gate needs them; Pion's server side writes them.

## R7. Receive cap

- **Decision**: with `auto_subscribe=0`, the adapter keeps, per kind, the remote tracks it knows and the set it subscribed to.
  - **Known tracks** come from `JoinResponse.other_participants` and `ParticipantUpdate` (a participant DISCONNECTED or a track missing from its list is removed), and from `TrackUnpublishedResponse`.
  - **Subscribing**: while fewer than the cap (or all, with cap 0) are subscribed, the earliest-known waiting track is subscribed with `UpdateSubscription{track_sids, subscribe: true}`. An ended track is replaced the same way.
  - **Own tracks** (same participant SID) are never subscribed.
  - **Failures**: a `SubscriptionResponse` with an error drops that track and logs once.
- **Rationale**: the same rule as the Jitsi adapter (earliest first, replacement on end, 004/005). The cap holds whatever else is in the room (spec edge case).
- **Alternatives considered**: `auto_subscribe=1` and ignoring extra tracks: the server would still forward them and spend bandwidth and CPU on them, which would distort the server-load comparison.

## R8. Keepalive and leave

- **Decision**:
  - **Ping**: `SignalRequest.ping_req{timestamp, rtt}` every `JoinResponse.ping_interval` seconds (5 s if zero). No message for `ping_timeout` seconds (15 s if zero) means the signaling is lost (`OnSignalingLost`).
  - **Leave**: `Disconnect` sends `LeaveRequest{reason: CLIENT_INITIATED}`, closes the WebSocket, then closes nothing in the peer (the engine closes it, as for Jitsi). A `LeaveRequest` from the server, or the socket closing outside `Disconnect`, means signaling lost.
  - **Ignored**: `refresh_token` (no reconnect).
- **Rationale**: the server drops participants that stop pinging. The loss mapping matches Jitsi's keepalive handling.

## R9. Lifecycle mapping

- **Decision**:
  - `Connect` returns nil after: the WebSocket is open, `JoinResponse` is received, the first subscriber offer is answered, and (sending bots) the publish answer is applied. The engine then moves the peer to `connected`, and transports move it to `established` (R4).
  - **Join failures**: an HTTP 401 on the upgrade is reported as a failed join, `stage: signaling`, `reason: unauthorized`. Its body (the server's message) is logged without the URL query.
  - `alternative_url` in `JoinResponse` is a failed join (single node only).
- **Rationale**: the same stages and metrics as Jitsi (FR-007).

## R10. Per-platform validation at creation, and secrets

- **Decision**: a new `src/platform/catalog` maps a platform name to:
  - `Validate(TestPlan) error`;
  - `SecretKeys []string`;
  - `NewAdapter(logger, platform_config)`.

  `MainEngineStarter` implements an optional `scenario.PlatformCatalog` interface over it, so `scenario.Service` validates and redacts without importing an adapter. `supportedPlatforms` becomes the catalog's names.
  - **LiveKit validation** ([contracts/livekit-config.yaml](contracts/livekit-config.yaml)): the config is parsed and checked, and `media.simulcast` or `media.requested_heights` is rejected with `"media.simulcast is not supported on platform livekit"` (same for requested heights).
  - **Jitsi**: keeps parsing its config at execution, as today, so existing test plans without `platform_config` still create.
  - **Secrets**: the service's returned scenarios and its creation log line redact them as `"[redacted]"`: `api_secret` and `token` for LiveKit, `jwt_app_secret` and `jwt` for Jitsi. The stored test plan keeps them for execution.
- **Rationale**:
  - FR-002, FR-005 and FR-009 require creation-time errors and no secret in answers.
  - Today the creation log line prints the whole test plan, Jitsi secret included. Fixing it for Jitsi too costs one list entry and is consistent with FR-009.
  - *Corrected in implementation (T009)*: `GET /scenarios/{id}` answers only `id` and `status`, so no API answer ever carried the secret. The leak was the log line. Redacting the service's returned copies keeps future answers safe.
- **Alternatives considered**: validating Jitsi configs at creation as well: it breaks existing callers that create a Jitsi scenario before filling its config; out of scope.

## R11. Measures LiveKit may prevent (latency)

- **Decision**: video latency matches a received frame's RTP timestamp in the scenario's send times (006 R5). LiveKit's forwarder may offset video and audio timestamps per down-track, which the first live check finds out.
  - **If offset**: the summary's kind gets `latency_available: false` and the new `latency_reason: "frame_not_identified"` (frames received, none found in the send times), instead of a zero. The 006 content-based fallback is not built in this version.
  - **Reason values**: `latency_reason` is set whenever latency is unavailable for a kind that received media: `frame_not_identified`, or `not_measured` (no send times: media from another machine only). It stays omitted when latency is available or the kind received nothing (its `reason` already says so).
  - **Other measures**: frame rate, freezes, gaps, MOS and keyframes are computed from what arrives, so they are platform-independent. The MOS uses the round-trip estimate when latency is missing (006 FR-011).
- **Rationale**: FR-011, never zero; the summary schema stays the same for both platforms (SC-004, an optional field).
- **Alternatives considered**: per-stream timestamp offset learning (first frame's offset assumed zero delay): wrong by the first frame's real delay, so it biases every sample.

## R12. Local LiveKit setup

- **Decision**: the setup lives in `tests/platforms/livekit/` and is committed. `.env` stays local (ignored); `env.example` is committed. It contains:
  - `docker-compose.yml`: one service, `image: livekit/livekit-server:v1.13.10`, `container_name: livekit-server`, `network_mode: host`, `command: --config /etc/livekit.yaml --node-ip ${LIVEKIT_NODE_IP}`, `LIVEKIT_KEYS: "${LIVEKIT_API_KEY}: ${LIVEKIT_API_SECRET}"`.
  - `livekit.yaml`: `port: 7880`, `rtc.tcp_port: 7881`, `rtc.port_range_start: 50000` / `rtc.port_range_end: 60000` (one UDP port per PeerConnection), `rtc.use_external_ip: false`, no Redis, `room.empty_timeout: 30`, `logging.level: info`.
  - *Changed in implementation (T027)*: the plan used the single-port mux (`rtc.udp_port: 7882`). At 100 bots it queued everything the server sends (about 116,000 packets/s through one socket): uniform 1.3 s audio latency, MOS 1.5, 150,000 freezes. The kernel dropped nothing, the bots' readers were idle, and a one-track probe saw no delay; congestion control off changed nothing. With the port range, the same run gives 1.5 ms latency, MOS 4.37 and no freezes, and LiveKit's CPU halves.
  - `env.example` with `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET` (≥ 32 characters), `LIVEKIT_NODE_IP=127.0.0.1`.
  - `README.md`: start/stop, the matching `platform_config`, `server_monitoring.containers: [livekit-server]`.
- **Rationale**:
  - Host networking avoids Docker's UDP NAT on the media path and port-range mapping, and gives the same path as the bots' loopback traffic.
  - The ports (7880–7881/tcp, 50000–60000/udp) do not collide with docker-jitsi-meet (8000, 8443, 8080, 10000/udp), so both setups can be installed, but runs are one at a time for a fair comparison.
- **Checked first**: Pion v3 bots reach the server's 127.0.0.1 candidate. Pion skips loopback for its own host candidates, but the server's loopback candidate is reachable from the LAN-address socket. If not, set `LIVEKIT_NODE_IP` to the host's LAN address, as the README says.

## R13. Validation runs

- **Decision**: `tests/live/livekit_live_test.go` (`-tags live`), a sibling of the Jitsi test with the same knobs:
  - LiveKit settings: `LIVEKIT_URL`, `LIVEKIT_ROOM`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`;
  - run knobs: `LIVEKIT_PARTICIPANTS`, `LIVEKIT_MEDIA`, `LIVEKIT_MAX_RECEIVED`, `LIVEKIT_HOLD`, `LIVEKIT_RAMP_UP`, `LIVEKIT_SERVER_CONTAINERS`.

  It logs the full quality summary as JSON, so that the Jitsi and LiveKit reports of one test plan can be recorded side by side. No CLI tool is added. `jitsi-connect` stays Jitsi-only.
- **Rationale**:
  - SC-001 to SC-003 are measured the same way as 006/007's Jitsi runs.
  - The API server cannot run next to docker-jitsi-meet (port 8080, see 007), and the live test does not need it.
