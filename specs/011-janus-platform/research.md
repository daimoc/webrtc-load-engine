# Research: Janus Platform (011)

Sources: Janus `v1.4.3` (latest tag on 2026-10-22):
- `src/plugins/janus_videoroom.c`, with its API documentation in the header
  comment;
- `src/transports/janus_websockets.c`;
- `conf/janus.jcfg.sample.in`.

Points marked **verify in T002** are checked against the running server
before the adapter is written, as 010's T002 did for mediasoup.

## R1. Signaling: Janus JSON API over WebSocket

**Decision**: one WebSocket per bot, subprotocol `janus-protocol`
(`nhooyr.io/websocket`, already used). On it, one Janus session with two
plugin handles on `janus.plugin.videoroom`:
- the **publisher handle**, the bot as a participant;
- the **subscriber handle**, the media it receives.

Every request carries a random `transaction`, and the client matches the
replies to it:
- core requests (`create`, `attach`, `keepalive`, `trickle`, `destroy`)
  get `success`, `ack` or `error`;
- synchronous plugin requests (`create` room, `exists`) get `success` with
  `plugindata`;
- asynchronous plugin requests (`join`, `publish`, `start`, `update`,
  `leave`) get an `ack`, then an `event` with the same transaction and,
  where there is one, a `jsep`.

Unsolicited messages are routed by `sender` (the handle ID) to the
publisher or subscriber logic. They are:
- `event` without a transaction (room notifications);
- `webrtcup`, `media`, `slowlink`, `hangup`, `detached`;
- `timeout` (session expired).

Errors have two levels:
- the core's `{"janus":"error","error":{"code","reason"}}`;
- the plugin's `{"videoroom":"event","error_code","error"}`.

Both fail the request with the code and reason.

**Rationale**:
- WebSocket is what Janus's own `janus.js` uses for low latency, and it is
  the constitution's "WebSocket / REST (Janus)".
- One connection per bot isolates losses: a closed socket loses one bot,
  as on the other platforms.
- Janus allows many sessions per socket, but sharing one would make one
  socket's loss take down many bots.

**Alternatives considered**:
- HTTP long-poll transport: one long-poll goroutine and HTTP round trip
  per event, for no gain on loopback.
- One shared WebSocket for all bots: fewer sockets, but correlated losses
  and a single read loop for hundreds of bots.
- The Admin API: not needed.

## R2. Mapping onto the 008 boundary

**Decision**: no boundary change.

- **Publisher PeerConnection = publish session.** The bot offers with
  `CreatePublishOffer` (sendonly sections, Opus and VP8), sends it in
  `publish` with `jsep`, and applies the answer from the `configured`
  event with `ApplyPublishAnswer`.
- **Subscriber PeerConnection = the session the remote side offers.**
  - Janus offers in the `attached` event; the bot answers with `OnOffer`
    and sends the answer in `start`.
  - Each later offer (`updated` event with `jsep`) goes to `Renegotiate`,
    and its answer goes out in `start`.
- **Candidates.**
  - The bot's candidates go out in a core `trickle` request on the handle
    of the session they belong to: `{sdpMid, sdpMLineIndex, candidate}`,
    then `{completed: true}`.
  - Janus sends its own candidates inside its SDP (default half-trickle,
    `full_trickle` off), so `OnICECandidate` and `OnPublishICECandidate`
    are not called.
  - The adapter handles a `trickle` from Janus all the same, in case a
    server enables full trickle.
- `PublishSession()` returns true. A `receive_only` bot has no publish
  session (010 T016 fix).

**Rationale**:
- Janus negotiates real SDP itself, so no translation is needed, unlike
  mediasoup (010 R2).
- The publisher/subscriber split is exactly what 008's publish session
  models.
- Janus is a full ICE agent by default. It is controlling for the
  subscriber offer and controlled for the publish answer, which Pion
  handles as on LiveKit.

## R3. Opening the subscriber session at join

The peer is `established` only when its main session (the remote-offered
one) is connected, besides the publish session (`execution/peer.go`,
`onTransportState`). A Janus subscription cannot be empty:
- `join` as subscriber without streams falls back to the legacy `feed`
  field and fails without it;
- `subscribe` and `update` reject empty lists ("Empty subscription list").

The first bot of a room, or a `receive_only` bot alone, would therefore
never be established.

**Decision**: rooms are created with VideoRoom's **dummy publisher**:
- `dummy_publisher: true`;
- `dummy_streams: [{"codec":"opus"}]`: one placeholder audio stream.

At join, each bot's subscriber handle joins with
`streams: [{feed: <dummy id>}]`:
- Janus offers one audio section that carries no media;
- the bot answers, and the subscriber transport connects at join, whoever
  else is in the room.

Real streams are added with `update` after it. The dummy's section stays
for the whole session. It counts against no receive cap, since the cap is
applied to real publishers' streams (R6), and it produces no track because
no RTP arrives on it.

The dummy publisher is found in `joined.publishers` by `dummy: true`. A room
without one (created elsewhere) fails the join with "room <id> has no
dummy publisher: create it with dummy_publisher=true" (FR-009, edge case).

**Rationale**:
- The established rule stays the same for every platform (constitution V),
  as 010 R3 did with its data-channel section.
- The dummy publisher is VideoRoom's own mechanism for "subscriptions with
  placeholders ... even when they aren't used yet".
- The cost is one idle audio section per bot.

**Alternatives considered**:
- Counting a session as open only once it has been offered. It is a
  per-platform exception in disguise, and a `receive_only` bot alone would
  still never be established.
- Subscribing each bot to its own feed: it receives its own media, which
  skews every received-media measure.
- Waiting for the first other publisher: the first bot's
  connect time would depend on the second bot's.

**Verify in T002**:
- Janus marks the dummy stream `active` and sends its offer with the
  section `sendonly`;
- Pion answers it, and no `OnTrack` fires;
- whether the dummy counts against `publishers` (R4 sizing).

## R4. Room creation and join sequence

**Decision**: per bot, in `Connect`:

1. Dial `url` with subprotocol `janus-protocol`, then core `create` to get
   the session ID, then `attach` the publisher handle.
2. **Room.** Send the synchronous `create`:
   - `room: <room>`, `permanent: false`;
   - `publishers: participants + 1` (the dummy);
   - `audiocodec: "opus"`, `videocodec: "vp8"`;
   - `bitrate: 0` (no REMB cap from Janus);
   - `dummy_publisher: true` and `dummy_streams: [{"codec":"opus"}]`;
   - `admin_key` when configured.

   Error 427 (room exists) is success. Bots joining at the same time all
   send `create`: one wins and the others get 427, so there is no race
   (FR-004).
3. `join` with `ptype: publisher`, `room`, `display: <peer ID>`, and `pin`
   and `token` when configured. The `joined` event gives:
   - the bot's `id` and `private_id`;
   - the current publishers, dummy included.
4. If the bot sends:
   1. Send `publish` with the publish offer (`audiocodec: "opus"`,
      `videocodec: "vp8"`).
   2. Apply the `configured` answer. Error 432 (publishers full) fails the
      join.
5. `attach` the subscriber handle, then `join` with:
   - `ptype: subscriber`, `room`, `private_id`;
   - `use_msid: false`, `autoupdate: false`;
   - `streams: [{feed: dummy}]`.

   Answer the `attached` offer with `OnOffer` and send `start`.
6. Subscribe to the publishers already listed, within the cap (R6).
   `Connect` returns.

`participants` comes from the test plan. `catalog.Entry.NewAdapter` today
receives only `platform_config`, so it gains the test plan as a parameter
(catalog-internal; the other entries ignore it). The room's publisher
places then hold every bot of the plan plus the dummy.

A fixed, large `publishers` value would avoid the catalog change, but the
room would no longer reflect the test plan. Janus's own default is 3.

**Rationale**:
- This is the order of Janus's `videoroomtest.js`, with the room creation
  `janus.js` leaves to the application.
- A room that already exists keeps its own size: if it is too small, the
  extra bots fail with 432 and the server's reason (spec edge case). The
  README says to use a new room number, or to restart the server, when the
  bot count grows.

**Alternatives considered**:
- An `exists` check before `create`: one more round trip, still racy.
- `joinandconfigure`: it saves one round trip, but it mixes the join's
  publisher list with the answer and makes failures harder to attribute.
- Destroying the room when the run ends: the last bot is not known to the
  adapter, and a room in memory costs nothing.

## R5. Codecs and SDP

**Decision**:
- The rooms are created with Opus and VP8 only (R4), matching what the
  bots send (FR-005).
- The publish offer is Pion's, unchanged.
- The subscriber offers are Janus's, answered by Pion with the codecs it
  supports.
- No header extension is negotiated by the adapter: Janus offers its
  defaults (`mid`, `abs-send-time`, audio level, transport-wide-cc) and
  Pion keeps those it knows.

**Verify in T002**:
- Janus accepts Pion's publish offer as it is (BUNDLE, `rtcp-mux`, one
  sendonly section per track);
- the subscriber offer's VP8 payload type is the one Pion answers.

## R6. Receive cap: subscribe, unsubscribe, promote

**Decision**:
- `consumers.go`-style per-kind state, as in 010:
  - the active streams;
  - a FIFO waiting list of `(feed, mid)` keys from publishers'
    announcements.
- **Updates.**
  - New publishers come from `joined.publishers` and
    `event.publishers`.
  - Their audio and video streams go active while the kind has room, and
    wait otherwise.
  - `disabled` streams and `data` streams are skipped.
  - The dummy is never subscribed beyond R3.
- **Ended streams.** On `unpublished: <id>` or `leaving: <id>`, the feed's
  streams leave both lists. For each active one that ended, the earliest
  waiting stream of the same kind is promoted.
- **One `update` per change.** It carries
  `unsubscribe: [{feed, mid}...]` and `subscribe: [{feed, mid}...]`.
  - With `autoupdate: false`, Janus does not renegotiate on its own when a
    publisher leaves, so the adapter controls the renegotiations.
  - Janus answers `updated`, with a `jsep` offer when the SDP changed. It
    goes to `Renegotiate` and its answer to `start`.
  - An `updating` event means that Janus queued the change; the next
    `updated` covers it.
- **Order.** The updates and the `start` answers run one at a time per
  bot, on one goroutine, as in 010. The next `update` is not sent until
  the previous offer is answered.

**Rationale**:
- Janus's multistream subscription is VideoRoom's documented way to add
  and remove streams on one PeerConnection.
- Combining both lists in `update` gives one offer per change.
- The cap is enforced server-side: streams beyond it are never forwarded
  (FR-006, constitution IV).

**Alternatives considered**:
- `autoupdate: true`. Janus renegotiates when a feed leaves, and the
  promotion then needs a second offer.
- `switch` (remapping an m-line to another feed without renegotiation).
  It is efficient, but it changes the stream a section carries without
  SSRC signaling. Kept for a later feature if renegotiation costs show up.

## R7. Lifecycle, keepalive and loss

**Decision**:
- **Keepalive.** Every 25 s, each session sends
  `{"janus":"keepalive","session_id"}`. Janus's default `session_timeout`
  is 60 s (FR-007).
- **Connect.** It succeeds when R4 step 6 is done. `connected` and
  `established` follow from the transports, as on LiveKit and mediasoup.
- **Failures during the join.** A failed request fails `Connect` with the
  request and Janus's code and reason. A refused WebSocket handshake fails
  it with the HTTP status.
- **Loss after `Connect`.** Any of these calls `OnSignalingLost` with the
  reason:
  - the WebSocket closed or failing;
  - a `timeout` event;
  - `detached` or `hangup` on either handle;
  - a `kicked` event, or a `destroyed` event for the room.
- **Disconnect.** It sends `{"janus":"destroy"}` (Janus detaches both
  handles, and the bot leaves the room), waits for the reply up to 2 s,
  then closes the WebSocket. Disconnecting unknown or already-disconnected
  peers is a no-op.

**Rationale**:
- Constitution V: the same states and failure stages as on the other
  platforms.
- A session dropped by a missing keepalive is the Janus-specific loss to
  guard against.

## R8. Latency and quality measures

**Decision**: no change to the 006 measures. Janus's subscriber path
(`janus_rtp_header_update`) rewrites:
- SSRCs and sequence numbers;
- timestamps: to keep them continuous across source switches, it may
  rebase them from the first packet.

**Verify in T002**: whether the forwarded RTP timestamps keep the
publisher's values. If they don't, latency is left out with the 008
`latency_reason` (FR-013). Freezes, frame rate, audio gaps, MOS and
keyframe requests are computed as before. Janus forwards a PLI from a
subscriber to the publisher. It also sends its own PLI every `fir_freq`
seconds when configured: the room is created without `fir_freq`, so it is
0 and Janus sends none.

## R9. Configuration and validation

**Decision**: `platform_config` for `janus`
([contracts/janus-config.yaml](contracts/janus-config.yaml)):
- `url`: required. `ws://` or `wss://`, or `http(s)://` turned into
  `ws(s)://`, for example `ws://localhost:8188`.
- `room`: required. A positive integer, or a string of digits.
- `insecure_tls`: optional, default false.
- `api_secret`, `token`, `pin`, `admin_key`: optional and write-only.
  - They are sent as the core `apisecret` and `token`, and as the
    VideoRoom `pin` and `admin_key`.
  - They are redacted in API answers and report files (catalog
    `SecretKeys`) and never logged: `LogValue` shows only whether each is
    set.

The catalog entry rejects `media.simulcast` and `media.requested_heights`
at creation (FR-010), as for mediasoup.

**Rationale**:
- These are the server's own authentication points: the API secret and
  stored tokens of `janus.jcfg`, and the room PIN and room-creation admin
  key of `janus.plugin.videoroom.jcfg`.
- String room IDs (`string_ids`) are out of scope. The local setup and
  default Janus use numeric IDs.

## R10. Local setup

**Decision**: `tests/platforms/janus/`:
- **`Dockerfile`**:
  - based on `debian:bookworm-slim`;
  - builds Janus at tag `v1.4.3` (build argument `JANUS_VERSION`) with
    Debian's `libnice`, `libsrtp2`, `libjansson`, `libconfig`,
    `libwebsockets` and `libmicrohttpd`;
  - configured with `--enable-websockets --disable-rabbitmq --disable-mqtt
    --disable-data-channels`, and only the VideoRoom plugin enabled at
    run time;
  - the demos (`html/`) are copied into a second stage.
- **`conf/`**: our `janus.jcfg`, `janus.transport.websockets.jcfg` and
  `janus.plugin.videoroom.jcfg`, mounted read-only:
  - WebSocket on `127.0.0.1:8188`, no secure port;
  - `rtp_port_range = "20000-20999"`;
  - `ice_enforce_list` set to loopback;
  - debug level 3 (warnings);
  - no API secret;
  - VideoRoom without `admin_key`, with no static room.
- **`docker-compose.yml`**: host network, two services:
  - `janus` (container `janus`);
  - `janus-demos`, nginx serving the demos on `127.0.0.1:8090`, so that
    the web server's CPU is not counted in the server's load.
- **`env.example`**: the debug level and the RTP port range. `.env` is not
  committed.
- **`README.md`**: start, settings, container name, room sizing, how to
  watch a room (`http://localhost:8090/videoroom.html?room=<room>`), stop.

**Verify in T002**:
- libnice gathers a loopback host candidate with `ice_enforce_list = "lo"`;
  otherwise the host's LAN address is used, set in `.env`;
- the demo page joins a room created by the bots (it creates rooms
  only with its own settings).

**Rationale**:
- FR-015. Janus has no official image, so building a pinned tag is the
  only reproducible way, as for the mediasoup demo (010 R11).
- Host networking keeps the bots on loopback, as for LiveKit and
  mediasoup.

**Alternatives considered**: third-party Janus images, rejected because
they are unpinned and unofficial.

## R11. Tests

**Decision**:
- **Golden messages.** T002 records the server's real messages from a
  2-bot session against the local setup into `testdata/golden/`:
  - `create` and `attach` replies;
  - room `create` and 427;
  - `joined` with the dummy publisher;
  - the `configured` answer;
  - `attached`, `updated` and `updating`;
  - `event` with `publishers`, `unpublished` and `leaving`;
  - errors (426, 432, 433, core 403 for a bad API secret);
  - `timeout`, `hangup` and `detached`.
- **Unit tests**:
  - the JSON client: transactions, ack then event, sender routing, error
    levels;
  - config validation and redaction;
  - the receive cap: subscribe, wait, promote, FIFO, `update` contents,
    dummy skipped, data streams skipped;
  - the parsing of the golden messages.
- **A fake Janus server** on `httptest` with real Pion on both sides
  (constitution: contract tests). It plays Janus's side:
  - it answers the publish offer and offers subscriptions, forwarding the
    publishers' RTP;
  - it drives real peers through the adapter: join, room creation race,
    publish, subscribe within the cap, a publisher leaves and a waiting
    stream is promoted, keepalives (short interval in the test), a
    rejected request, a closed WebSocket, and a session `timeout`.
- **Engine and catalog tests**, extended with the platform.
- **A live test**, `tests/live/janus_live_test.go` (`-tags live`). It
  mirrors the mediasoup one:
  - variables `JANUS_URL`, `JANUS_ROOM`, `JANUS_PARTICIPANTS`, `JANUS_MEDIA`,
    `JANUS_MAX_RECEIVED`, `JANUS_HOLD`, `JANUS_RAMP_UP`,
    `JANUS_SERVER_CONTAINERS`, `JANUS_METRICS_ADDR`, `JANUS_LEAVE_ONE`,
    `JANUS_INSECURE_TLS`, `JANUS_API_SECRET`, `JANUS_TOKEN`, `JANUS_PIN`;
  - the same checks, summary and report files (SC-001, SC-003, SC-004).

**Rationale**: FR-016 and FR-017. The fake server checks protocol order and
the cap; the live test catches what only Janus does: timestamps, PLI,
pacing, the dummy publisher.

## R12. Comparison method

**Decision**: SC-002 repeats 010 T029's protocol with Janus as a fourth
server:
- 50 bots `audio_video`, cap 10, ramp-up 10 s, 10 min hold;
- each server alone and started just before its run, screen unlocked;
- the four servers alternated twice: Jitsi, LiveKit, mediasoup, Janus,
  then again in the same order.

The results go to `validation-report.md` next to T029's table.

**Rationale**: the 009 A/B and 010 T029 runs showed that only alternated
same-state runs are comparable on this laptop.
