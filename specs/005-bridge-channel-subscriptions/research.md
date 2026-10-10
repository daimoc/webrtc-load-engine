# Research: Bridge Channel Subscriptions

Decisions taken for [spec.md](spec.md), from the bridge deployed locally
(`ghcr.io/jitsi/jvb:unstable`, package `jitsi-videobridge2`
2.3-326-gb92aba83c, inspected with `javap` on `jitsi-videobridge.jar` and its
`reference.conf`), Pion v3.3.6 and the Jitsi adapter of 003/004.

## R1. Video: `ReceiverVideoConstraints` keyed by source name

- **Decision**: each bot sends, over the bridge channel, a text message

  ```json
  {"colibriClass":"ReceiverVideoConstraints","lastN":-1,
   "defaultConstraints":{"maxHeight":0},
   "constraints":{"<source name>":{"maxHeight":360}, ...}}
  ```

  listing the remote video sources it receives (the active sections of the
  adapter's `sessionSources`, at most the cap). Without a cap it sends
  `{"colibriClass":"ReceiverVideoConstraints","lastN":-1,"defaultConstraints":{"maxHeight":360}}`,
  so every video source is forwarded, including those announced after the
  message, without resending it (FR-004). Full message formats are in
  [contracts/colibri-messages.md](contracts/colibri-messages.md).
- **Rationale**: the bridge's `ReceiverVideoConstraintsMessage` has the
  fields `lastN`, `selectedSources`, `onStageSources`, `defaultConstraints`
  and `constraints` (a map from source name to `VideoConstraints{maxHeight,
  maxFrameRate}`). Its `initial-max-height-px` is 0: before a client sends
  constraints, no video is forwarded, which is what 004 measured. A source
  whose constraint is `maxHeight 0` is not allocated, so `defaultConstraints
  0` plus explicit entries forwards exactly the received sources, whatever
  the bridge's speaker ranking. `lastN -1` avoids a second limit that would
  rank by speaker (the bridge's `jvb-last-n` and `initial-last-n` are -1, the
  load reducer is disabled by default, `conference-last-n-limits` is empty).
  360 is the height the bots send (004 R1), so a higher value brings nothing.
- **Alternatives considered**: `lastN = cap` with `defaultConstraints 360`
  (the bridge picks the N by dominant speaker, not the bot's received
  streams, so it sends video the bot drops and misses video it keeps);
  `selectedSources` (deprecated in favour of `constraints`, and it only
  raises priority); the legacy `ReceiverVideoConstraint` (singular, by
  endpoint id; removed from recent bridges).

## R2. Audio: `ReceiverAudioSubscription` with `Include`

- **Decision**: with a cap, each bot sends
  `{"colibriClass":"ReceiverAudioSubscription","mode":"Include","list":["<source name>", ...]}`
  with the remote audio sources it receives. Without a cap it sends nothing
  for audio (the bridge's default is every source).
- **Rationale**: the local bridge has `ReceiverAudioSubscriptionMessage`
  (modes `All`, `None`, `Include`, `Exclude`, plus a legacy
  `all`/`include`/`exclude` form) and an `AudioSubscriptionManager` that maps
  the names to SSRCs (`AudioSubscription.ssrcsForNames`) and filters per
  receiver (`isEndpointAudioWanted`), including sources announced after the
  message (`onConferenceSourceAdded`). So US2 can be met for audio on the
  validation deployment, which the spec's assumption left open. `Include`
  with an empty list is valid (nothing received yet) and keeps the bridge
  from sending everything during the first renegotiations.
- **Alternatives considered**: `Exclude` with the waiting sources (the
  bridge would forward any source the bot has not heard of yet, e.g. one
  joining between two messages); routing only the loudest speakers
  (`loudest.route-loudest-only`, a bridge-wide setting, not per bot, and it
  changes the forwarded set with voice activity).

## R3. Source names come from the session sources

- **Decision**: the adapter derives the preferences from `sessionSources`:
  for each content (audio, video), the `Name` of the primary source of every
  active section. A received source without a name (old Jicofo without
  source names) cannot be subscribed to by name: the adapter then sends the
  no-cap form for that kind and logs it once per bot (the bot-side cap still
  applies, R5).
- **Rationale**: `sessionSources` already decides which sources a bot
  receives under the cap, and promotes the oldest waiting one when an active
  one ends (004 FR-015). Source names are present in both the XML and the
  JSON source formats of Jicofo 1.0-1211 (`name` attribute, `"n"` key) and
  are the bridge's keys for both messages.
- **Alternatives considered**: keying by endpoint id (old bridges only);
  letting the execution layer compute the list (it would need source names,
  a Jitsi concept; constitution III and FR-010).

## R4. When preferences are sent: on open and on every change, latest wins

- **Decision**: the adapter keeps, per session, a `bridgeChannel` holding the
  current preferences and the last message sent per class. It sends the
  messages when the channel opens (and reopens), and after every change of
  the active sections (`source-add`, `source-remove`, departure of an
  occupant, promotion of a waiting source), right after the change is
  applied, without timer. Sends are serialized by a mutex and skipped when
  the message equals the last one sent; a message computed while the
  channel is closed is only stored, and the open handler sends the stored
  state.
- **Rationale**: changes are already serialized by the session's
  renegotiation, and a change is applied in milliseconds, well within the
  2 s of FR-005. Computing the message from the current state at send time
  makes "only the latest matters" (FR-006) hold without a queue. Messages
  are small (under 1 kB for 10 sources) and bursts are bounded by the
  number of signaling changes.
- **Alternatives considered**: a debounce timer (adds latency and a
  goroutine per bot for no measured need); sending before renegotiating (the
  bridge would start sending before the bot has the section, and the first
  packets would be dropped, a harmless but pointless order).

## R5. No acknowledgement: keep the bot-side cap, measure the transport

- **Decision**: the bot-side cap of 004 (sections only for received
  sources, SSRC gate in `gatedBuffer`) stays on in every case. The engine
  exports a new counter of the RTP and RTCP bytes each bot receives,
  `webrtc_transport_bytes_received_total{scenario_id}`, counted in
  `gatedBuffer.Write` (every decrypted packet, before the SSRC gate) and
  added by the existing 2 s media collector. (Planned from the selected ICE
  candidate pair's stats; pion/ice v2.3.38 does not fill their byte counts,
  found in T021.) Comparing it with
  `webrtc_media_bytes_received_total` shows how much the server sends beyond
  what the bots keep (SC-003). The spec's FR-007 and US2 scenario 4 were
  amended accordingly.
- **Rationale**: the bridge never answers `ReceiverVideoConstraints` or
  `ReceiverAudioSubscription`; an unknown `colibriClass` fails to parse on
  the bridge side and is only logged there (`MessageHandler.unhandledMessage`).
  The bot therefore cannot tell from the protocol whether a subscription is
  honored; only the traffic can. The read buffers see every packet,
  including those Pion drops for unknown SSRCs, and an atomic add per packet
  is negligible next to SRTP decryption.
- **Alternatives considered**: counting Pion's "Incoming unhandled RTP ssrc"
  log events (depends on a log message, and Pion logs per attempt, not per
  packet); `PeerConnection.GetStats` (builds the whole report, much more
  expensive for 100 bots); reading the server's traffic from `docker stats`
  (only works on a local deployment; kept as a cross-check in the
  quickstart).

## R6. Bridge bandwidth estimation may hold video back

- **Outcome**: see R9: the feedback was wrong, not the ramp-up time.
- **Decision**: rely on the bots' existing transport-wide congestion control
  feedback (pion/interceptor's TWCC sender, enabled since 004) so that the
  bridge's estimator ramps up; check during validation that the bridge
  forwards all requested streams (10 × 500 kbit/s = 5 Mbit/s per bot). If it
  does not within the run's warm-up, the quickstart documents setting
  `videobridge.cc.trust-bwe=false` on the local bridge (a test deployment
  setting, not an engine change), and the result records it.
- **Rationale**: the bridge allocates video within its bandwidth estimate
  per receiver (`trust-bwe = true` by default). Pion's TWCC sender gives it
  the feedback it needs; loopback has no real limit, so the estimate should
  reach the needed rate, but its ramp-up time is not known in advance.
- **Alternatives considered**: sending REMB from the bots (the bridge uses
  TWCC when negotiated); lowering the requested height (the bots send one
  360p layer, nothing lower to choose).

## R7. Data channel contract: send and events, still platform-neutral

- **Decision**: `SignalingHandler.OpenDataChannel` returns a
  `platform.DataChannel` (`SendText`) and takes a
  `platform.DataChannelHandler` (`OnOpen`, `OnMessage`, `OnClose`). The
  `Peer` implements them over Pion's `DataChannel`. It keeps counting
  incoming messages and exports a gauge
  `webrtc_data_channels_open{scenario_id}` (FR-008): for Jitsi, the bots
  with an open bridge channel. See
  [contracts/signaling-handler.md](contracts/signaling-handler.md).
- **Rationale**: the execution layer only knows "a data channel with a label
  and a protocol" (constitution II/III, FR-010); message formats and their
  meaning stay in the adapter. Incoming bridge messages (`ServerHello`,
  `ForwardedSources`, `DominantSpeakerEndpointChangeEvent`, statistics,
  ...) are passed to the adapter, which ignores them (FR-009) apart from a
  debug log of their class. Callbacks run on Pion goroutines and must not
  block.
- **Alternatives considered**: a generic gauge without a data channel
  concept (e.g. a "control channel" in the media layer: same thing, less
  accurate name); a Jitsi-specific gauge in the adapter (adapters do not own
  metrics today, and other platforms have equivalent channels).

## R8. Fallbacks and logging (FR-007)

- **Decision**: one log line per bot, at `Info`, for each of: the offer has
  no SCTP content (bridge channel not offered); the channel did not open
  within 10 s after the adapter opened it (one `time.AfterFunc` per session,
  stopped on open or session end); the channel closed while
  the session is up; a send failed (first failure only); a received source
  has no name (R3). In all these cases the bot keeps the 004 behavior.
- **Rationale**: "log once" per bot keeps 100-bot runs readable; the gauge
  of R7 and the transport counter of R5 give the aggregate view.
- **Alternatives considered**: failing the peer when the channel is
  missing (FR-007 and 004 FR-014: a media issue must not fail a scenario).

## R9. Transport feedback must acknowledge every packet that arrives (found in T014)

- **Finding**: the first live run (T014, 10 bots) received video at only 26%
  of the expected volume. The bridge's debug state showed `bweBps` stuck near
  610 kbit/s, 8 of 9 requested sources suspended, and an outgoing loss of 38%
  computed from the bot's transport-wide congestion control (TWCC) feedback,
  on a loss-free loopback. The bridge sends its bandwidth probes (padding) on
  its own video source (owner `jvb`), which the adapter leaves out of the
  offer (003); Pion drops packets of SSRCs without a section before its
  interceptors, and its TWCC generator (an interceptor on the read path)
  never sees them, so the feedback reports every probe as lost and the
  bridge's estimator never ramps up. The same happens to every stream the
  bot drops (004 receive cap): R6's risk, with a precise cause.
- **Decision**: the peer generates its TWCC feedback itself from every RTP
  packet that reaches its SRTP read buffers (`gatedBuffer.Write`, before the
  SSRC gate), with pion/interceptor's `twcc.Recorder`, sent every 100 ms
  with `PeerConnection.WriteRTCP`; Pion's TWCC sender interceptor is no
  longer registered (the header extension and the `transport-cc` feedback
  stay negotiated). The extension ID comes from the applied offer's
  `a=extmap` for the transport-cc URI.
- **Rationale**: TWCC describes what the transport delivered, not what the
  application keeps; acknowledging dropped packets is what a browser does
  (its transport receives them before any demultiplexing). It stays in the
  execution layer and needs nothing from the adapter (constitution II/III).
  The extra cost is one RTP header parse for packets that were dropped
  without parsing before.
- **Alternatives considered**: giving the bridge's source a section in the
  offer (it would be counted as a remote stream, and the cap's dropped
  streams would still be reported lost); disabling the bridge's probing or
  `trust-bwe` (hides a client bug behind a server setting that real
  deployments do not use).
