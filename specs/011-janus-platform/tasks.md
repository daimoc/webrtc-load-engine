---

description: "Task list for 011 Janus platform"
---

# Tasks: Janus Platform

**Input**: Design documents from `specs/011-janus-platform/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The spec asks for them (FR-016, FR-017, SC-007), and the constitution requires contract tests for signaling adapters. Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green after every task.
- **Boundaries** (constitution III, [contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)):
  - no Janus or VideoRoom name, type or JSON outside `src/platform/janus` and `src/platform/catalog`;
  - `src/execution`, `src/media`, `src/scenario` and `src/report` are not changed (test files excepted);
  - the only shared change is the catalog's `Entry.NewAdapter` signature (T006, plan "Constraints");
  - no new module in `go.mod`.
- **Metrics in tests**: tests that read global metrics use a unique scenario ID (uuid), as in 004–010.
- **Secrets**: `api_secret`, `token`, `pin` and `admin_key` are never in logs, answers or reports. Tests grep captured logs and files for their values.
- **Live runs**:
  - they target `tests/platforms/janus` with docker-jitsi-meet, LiveKit and mediasoup-demo stopped ([quickstart.md](quickstart.md)), except the alternated runs of T022;
  - runs that are compared must be taken in the same machine state, screen unlocked (010 research R13, T029).
- **Mirror the mediasoup adapter**: file layout, logging style, `Config.LogValue`, the per-bot work goroutine and the fake-server helpers of `src/platform/mediasoup` are the model. Copy their shape, not their protocol.
- **Before T001**: commit the pending 010 T029 changes (`specs/010-mediasoup-platform/tasks.md`, `validation-report.md`) on their own, so 011's work starts from a clean tree.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: the local server, and the real messages every later task is tested against. It also checks the research points marked "verify in T002" (research R3, R5, R8, R10).

- [X] T001 Create `tests/platforms/janus/` per research R10:
  - **`Dockerfile`**:
    - two stages from `debian:bookworm-slim`, with `ARG JANUS_VERSION=v1.4.3`;
    - the build stage `apt-get install`s `git build-essential autoconf automake libtool pkg-config gengetopt libnice-dev libsrtp2-dev libjansson-dev libconfig-dev libssl-dev libglib2.0-dev libcurl4-openssl-dev libwebsockets-dev libmicrohttpd-dev zlib1g-dev`;
    - it runs `git clone --depth 1 --branch $JANUS_VERSION https://github.com/meetecho/janus-gateway.git`, then `./autogen.sh && ./configure --prefix=/opt/janus --enable-websockets --disable-rabbitmq --disable-mqtt --disable-nanomsg --disable-unix-sockets --disable-data-channels --disable-docs && make -j"$(nproc)" && make install`;
    - the run stage installs only the runtime libraries (`libnice10 libsrtp2-1 libjansson4 libconfig9 libglib2.0-0 libcurl4 libwebsockets17 libmicrohttpd12 libssl3 zlib1g`) and copies `/opt/janus`;
    - `CMD ["/opt/janus/bin/janus", "--configs-folder=/opt/janus/etc/janus"]`;
    - if a library name differs on bookworm, use bookworm's and record it under this task.
  - **`conf/janus.jcfg`**, from the tag's `conf/janus.jcfg.sample.in`:
    - `general.debug_level` from the env (default 3);
    - `media.rtp_port_range = "20000-20999"`;
    - `nat.ice_enforce_list = "lo"`;
    - `plugins.disable` = every plugin except `libjanus_videoroom.so`;
    - `transports.disable` = every transport except `libjanus_websockets.so` (and HTTP, kept for `curl` checks: `libjanus_http.so` on 127.0.0.1:8088);
    - no `api_secret`, no `token_auth`.
  - **`conf/janus.transport.websockets.jcfg`**: `ws = true`, `ws_port = 8188`, `ws_ip = "127.0.0.1"`, `wss = false`, `admin_ws = false`.
  - **`conf/janus.plugin.videoroom.jcfg`**: `general: { string_ids = false }`, no `admin_key`, no static room.
  - **`conf/janus.transport.http.jcfg`**: `http = true`, `port = 8088`, `ip = "127.0.0.1"`, `https = false`.
  - **`docker-compose.yml`**, with host networking for both services:
    - `janus`: `build: .`, `image: webrtc-load-engine/janus:v1.4.3`, `container_name: janus`, `restart: unless-stopped`, `env_file: .env`, `./conf` mounted read-only on `/opt/janus/etc/janus`;
    - `janus-demos`: `nginx:1.27-alpine`, `container_name: janus-demos`, serving the build stage's `html/` (copy it into a `demos` image stage, or build with `target:`) on `127.0.0.1:8090`;
    - the demos' `settings.js` points at `ws://localhost:8188`.
  - **`env.example`**: `JANUS_DEBUG_LEVEL=3`, plus a comment on the RTP range and on `ice_enforce_list` (set the LAN interface if bots run in another network namespace).
  - **`README.md`**: a stub with start and stop; completed in T020.

  Add `tests/platforms/janus/.env` to `.gitignore`. Run `docker compose up -d --build`, then check:
  - `docker logs janus` shows the WebSockets transport and the VideoRoom plugin loaded;
  - `ss -lntu` shows TCP 8188 and 8088 on 127.0.0.1;
  - `curl -s -X POST -d '{"janus":"info","transaction":"x"}' http://127.0.0.1:8088/janus` answers `server_info` with version 1.4.3;
  - `http://localhost:8090/demos/videoroom.html` loads.

  Record under this task: the build time, the image size, and the libnice version (`docker exec janus dpkg -s libnice10 | grep Version`).
  - **Recorded 2026-10-22**:
    - **Build and versions**: built in 109 s (cold), image 250 MB. Janus 1.4.3, commit `1f0b75c`, libnice 0.1.21-1 (bookworm). The bookworm package names of the task were right. Configured with `--disable-all-plugins --enable-plugin-videoroom --disable-all-transports --enable-websockets --disable-all-handlers --disable-all-loggers --disable-data-channels`, so only what is used is built.
    - **No HTTP transport**: 127.0.0.1:8088 is taken on this machine by a process outside Docker. Janus's HTTP transport failed to bind, and the first container start restarted once. The HTTP transport and `libmicrohttpd` were dropped. The demos use the WebSocket transport through our `demos-settings.js`, which replaces `html/demos/settings.js`. The `curl` check is replaced by T002's probe, which talks to the server over WebSocket.
    - **Listening and logs**: TCP 8188 and 8090 on 127.0.0.1. The demo page is at `http://localhost:8090/demos/videoroom.html` (200). The log has three WARN lines at start: mDNS off, no STUN server, data channels not compiled. Plugins are not listed at debug level 3.
    - **ICE**: `ice_enforce_list = "lo"` does not work (T002), so the config now ignores the container bridges. See research R10.
- [X] T002 In the scratchpad (never committed), write a throwaway Go probe with `nhooyr.io/websocket` (subprotocol `janus-protocol`) and Pion, against T001's server. Two clients, A and B, each with a session and a publisher handle, both send room `create` for room 9002 (sizing as research R4: `publishers: 3`, `dummy_publisher: true`, `dummy_streams: [{"codec":"opus"}]`, `audiocodec: "opus"`, `videocodec: "vp8"`, `bitrate: 0`, `permanent: false`) at the same time. Then:
  1. Both join as publishers (`display` `probe-a` and `probe-b`).
  2. B publishes audio and video from a Pion PeerConnection (sendonly Opus and VP8 tracks), writing RTP with known timestamps (start at 1000, +960 per 20 ms audio frame).
  3. A attaches a subscriber handle, joins with `streams: [{feed: dummy}]`, answers, and `start`s.
  4. A sends `update` with `subscribe` for B's audio and video, answers the `updated` offer, and receives RTP.
  5. B unpublishes, then leaves.
  6. A stays silent (no keepalive) for 70 s.

  **Golden JSON**: save the raw frames, pretty-printed, under `src/platform/janus/testdata/golden/`:
  - `create_session.json` and `attach.json`;
  - `room_created.json`, and `room_exists.json` (the 427 the loser of the race gets);
  - `joined_publisher.json` (with the dummy in `publishers`) and `event_publishers.json` (A notified of B);
  - `ack.json` and `configured.json` (with `jsep` answer);
  - `attached_subscriber.json` (with `jsep` offer), `started.json`, `updated.json`, and `updating.json` if one occurs (send two `update`s back to back to provoke it);
  - `unpublished.json` and `leaving.json`;
  - `webrtcup.json`, `media.json` and `hangup.json`;
  - `timeout.json` (after the silent 70 s);
  - `error_core.json` (a `message` on a destroyed session) and `error_plugin_432.json` (a third publisher in a room created with `publishers: 1`).

  Replace the probe's SDP bodies with short placeholders in the goldens; keep the JSON structure.

  **Check and record under this task**:
  - the race: exactly one `created`, the other gets 427;
  - **dummy subscription (R3)**: the `attached` offer's section is a single audio section with direction `sendonly`; Pion answers it; no `OnTrack` fires on it during 10 s; A's subscriber transport reaches `connected` with nobody else publishing;
  - **dummy and room size**: whether the dummy counts against `publishers`. Create a room with `publishers: 1` and dummy; does one real publisher fit?
  - **codecs (R5)**: Janus accepts Pion's publish offer unchanged; the subscriber offer's Opus and VP8 payload types and header extensions;
  - **ICE (R10)**: the candidates in Janus's SDP. With `ice_enforce_list = "lo"`, is there a `127.0.0.1` host candidate? If not, set the LAN interface, update `env.example`, `conf/janus.jcfg` and research R10;
  - **RTP timestamps (R8)**: the timestamps A receives vs. those B sent (offset or equal);
  - `pin`: is it checked on the subscriber `join` too (create a room with `pin`, join a subscriber without it);
  - the HTTP status of a WebSocket handshake without the subprotocol;
  - whether `updated` arrives with the same `transaction` as the `update` request, or unsolicited.

  If a fact differs from research R1–R8 or [contracts/signaling-messages.md](contracts/signaling-messages.md), update them before Phase 2.
  - **Recorded 2026-10-22** (Janus 1.4.3). The 25 goldens are saved, with SDP bodies replaced by placeholders. Two goldens were added beyond the list: `unpublished_second.json` and `updated_unsolicited.json`. `error_plugin_429_pin.json` comes from the PIN check below.
    - **Race**: one `created`, and the other gets 427 "Room 9002 already exists" inside a `success` reply (synchronous plugin request).
    - **Dummy subscription (R3) holds**:
      - the `attached` offer has one audio section (`mid 0`, Opus 111, `sendonly`, extmaps transport-cc and mid), and Pion answers it;
      - the subscriber transport is `connected` 50 ms after `start`, with nobody else publishing;
      - no `OnTrack` fires in 3 s.
    - **Room size**: **the dummy takes no publisher place**. In a room created with `publishers: 1` and the dummy, one real publisher publishes and the second gets 432 "Maximum number of active publishers (1) already reached" on `publish` (not on `join`). R4 now sizes `publishers: participants`.
    - **Codecs (R5)**:
      - Janus accepts Pion's publish offer unchanged and answers with Opus 111 and VP8 96 + rtx 97, `recvonly`;
      - the subscriber offers carry Opus 111 and VP8 96, each section with transport-cc and mid extmaps;
      - the `configured` event lists the published `streams` (Opus with `fec: true`).
    - **ICE (R10)**:
      - Janus skips loopback interfaces (`ice.c`, `IFF_LOOPBACK`). With `ice_enforce_list = "lo"` nothing is enforced, and libnice gathered every address: two global IPv6, 192.168.1.65 and a link-local one;
      - after the config change, one candidate remains, `192.168.1.65` UDP in the RTP range, and the bots connect to it.
    - **RTP timestamps (R8)**:
      - B sent audio timestamps from 1000; A received 0, 960, 1920, with sequence numbers from 1 and SSRC rewritten;
      - Janus rebases every forwarded stream, which would make latency wrong for early subscribers. Fixed by T026 (the user's decision).
    - **PIN**: the subscriber `join` without the room's PIN fails with 429 "Missing mandatory element (pin)". The bots send `pin` on both joins. The publisher-join check of the probe used a handle that had already joined (425), so it proves nothing; the publisher join is documented to need the PIN too.
    - **Handshake**: a WebSocket upgrade without `janus-protocol` is closed with no HTTP response (`curl` gets an empty reply), and a plain GET gets 403. The adapter reports a dial error without a status in that case.
    - **`update` replies**:
      - the first `update` got `updated` + jsep as its own event (same transaction);
      - an `update` sent before that offer was answered got `updating`. Its `updated` + jsep came later, **unsolicited**, after the first `start`. The adapter must handle both (R6).
    - **Leaving**:
      - with `autoupdate: false`, B's departure sent A no offer;
      - A's publisher handle got `unpublished: <B>` **twice** (once for unpublish, once more on leave), then `leaving: <B>`, so feed-gone handling must be idempotent;
      - B's own replies were `unpublished: "ok"` and `leaving: "ok"`, plus a `hangup` with `reason: "Close PC"`.
    - **IDs and other events**:
      - session, handle and feed IDs are integers up to about 2^52 (for example 7342466929253127). Decode them as `uint64`, never `float64`;
      - `media` events carry `mid`, `type` and `receiving`;
      - `webrtcup` has only `sender`.
    - **Timeout**: A, silent from 23:38:14.6, got `{"janus":"timeout"}` at 23:39:10.9 (56 s, Janus checks sessions periodically), then the server closed nothing else. The 25 s keepalive of R7 stands.
    - **Core error**: a request on a destroyed session gets `{"janus":"error","error":{"code":458,"reason":"No such session …"}}`.

---

## Phase 2: Foundational

**Purpose**: config, the Janus JSON client, the message types, the catalog signature and the fake server that every story uses.

- [X] T003 [P] Write `src/platform/janus/config_test.go`, then `config.go`, per [contracts/janus-config.yaml](contracts/janus-config.yaml) and data-model "Config":
  - `ParseConfig(map[string]any) (Config, error)`;
  - `Validate(scenario.TestPlan) error`;
  - `Config.LogValue` and `String` without the secrets.

  Rules:
  - `url` is "required; `ws://` or `wss://` (`http(s)://` turned into `ws(s)://`), no query";
  - `room` is "required; > 0; from a JSON number or a string of digits". JSON numbers arrive as `float64`: reject non-integers;
  - `insecure_tls` is a boolean, default false;
  - `api_secret`, `token`, `pin` and `admin_key` are strings;
  - unknown keys are rejected with `platform_config: unknown key %q for platform janus`;
  - `media.simulcast` and `media.requested_heights` are rejected with the contract's messages.

  Each error message of the contract has a test. A test logs a config with all four secrets and greps the output for their values.
- [X] T004 [P] Write `src/platform/janus/messages.go`: Go structs with JSON tags for what [contracts/signaling-messages.md](contracts/signaling-messages.md) uses:
  - `Message`: the envelope, with `janus`, `transaction`, `session_id`, `sender`, `data.id`, `error{code,reason}`, `plugindata{plugin,data}` and `jsep{type,sdp}`;
  - `RoomEvent`: the VideoRoom `data`, with `videoroom`, `room`, `id`, `private_id`, `publishers[]`, `streams[]`, `unpublished`, `leaving`, `kicked`, `configured`, `started`, `error_code` and `error`;
  - `Publisher` (`id`, `display`, `dummy`, `streams[]`);
  - `PublisherStream` (`type`, `mid`, `disabled`, `codec`);
  - `SubStream` (`mid`, `type`, `feed_id`, `feed_mid`, `active`).

  `unpublished` and `leaving` are a number, or the string `"ok"`: give them a type that decodes both. In `messages_test.go`, decode every T002 golden file and check the fields the adapter uses.
- [X] T005 [P] Write `src/platform/janus/client_test.go`, then `client.go`: a Janus client over a `*websocket.Conn` (research R1).
  - **Requests**:
    - `Request(ctx, body map[string]any) (Message, error)` adds a unique `transaction` and the configured `apisecret` and `token`, then waits up to 15 s for the reply: `success`, `error` or `ack`;
    - `Message(ctx, handle, body, jsep) (Message, error)` sends a plugin message:
      - if the reply is `success` with `plugindata` (synchronous), it returns it;
      - if the reply is `ack`, it waits, under the same timeout, for the `event` with the same transaction and returns it;
      - a plugin `error_code` in the event's data is returned as an error;
    - errors name the request (`create`, `attach`, or the body's `request`), with the code and reason.
  - **Unsolicited messages** (no pending transaction) go to a handler registered per `sender` handle ID, or to a session handler for `timeout`.
  - **Keepalive**: `StartKeepalive(interval)` sends `keepalive` until `Close`.
  - **Lifecycle**: `Close()` fails pending requests with "connection closed"; `Done()` and `Err()` report the read loop's end.

  Tests run against an `httptest` WebSocket server that speaks Janus:
  - concurrent requests answered out of order;
  - ack, then event;
  - a synchronous `success`;
  - a core error and a plugin error;
  - a timeout;
  - unsolicited events routed by `sender`;
  - keepalives at a short interval;
  - `apisecret` and `token` present on every frame when configured;
  - `Close` with pending requests;
  - the server closing the socket.
- [X] T006 Change `catalog.Entry.NewAdapter` in `src/platform/catalog/catalog.go` to `func(logger *slog.Logger, tp scenario.TestPlan) (platform.PlatformAdapter, error)`. Each entry reads `tp.PlatformConfig` itself:
  - the Jitsi, LiveKit and mediasoup entries keep their behaviour;
  - `Catalog.NewAdapter` passes the plan.

  Update `catalog_test.go` if it builds entries. `go test -race ./src/platform/catalog/ ./src/...` passes, with no other change.
- [X] T026 [P] (Added after T002; research R8, the user's decision.) In `src/media/source.go`, start each source's RTP timestamp at `1 << 30` instead of 0 (`ts uint32 = initialRTPTimestamp`, a named constant with a comment citing RFC 3550 and research R8). Test first in `src/media/source_test.go`: the first frame's packets carry timestamp 2^30, the second 2^30 + the first frame's samples, and `SendTimes` records the same values. Fix any media or execution test that assumed a start at 0. `go test -race ./...` passes.
  - **Changed in T022**: first done with `1 << 31`. Jitsi's bridge rebases video timestamps that are not newer than 0 in RTP order, so Jitsi video latency was lost (research R8, "Found in T022"). Now `1 << 30`, checked with a timestamp probe on Jitsi (kept) and Janus (rebased, never matched).
- [X] T007 Write `src/platform/janus/fakeserver_test.go`, a test helper (research R11): an `httptest` WebSocket server that speaks Janus as T002 recorded it, with subprotocol `janus-protocol`.
  - **Sessions and handles**: core `create`, `attach`, `keepalive`, `trickle`, `destroy`. The session expires after a configurable timeout without keepalive and sends `timeout`.
  - **Rooms**:
    - `create` with 427 on an existing room;
    - a dummy publisher when `dummy_publisher`;
    - the `publishers` limit, giving 432.
  - **Publishers**:
    - `join` returns `joined` with the current publishers, dummy included;
    - `publish` answers the bot's offer from a Pion PeerConnection on 127.0.0.1 and sends `event.publishers` to the room's other publishers;
    - `leave` and `destroy` send `unpublished` and `leaving`.
  - **Subscribers**:
    - `join` with `streams` sends `attached` with an offer from a Pion PeerConnection: the dummy as a silent audio section, and one section per subscribed stream;
    - `start` applies the answer;
    - `update` sends `updated` with a new offer (sections added, or removed as `inactive`, as Janus does).

    It forwards each publisher's RTP to the subscribed sections, rewriting SSRC and payload type only.
  - **Hooks** let tests:
    - fail a given request with a code;
    - refuse the handshake with a status;
    - close a bot's WebSocket;
    - expire a session;
    - read each subscriber's `update` requests and current streams;
    - read the frames each bot sent, to grep for secrets.

**Checkpoint**: config, client, types and the catalog signature pass; the fake server can drive real peers.

---

## Phase 3: User Story 1 - Run a test plan against Janus (Priority: P1) 🎯 MVP

**Goal**: bots join a Janus room, send and receive audio and video within the receive cap, keep their sessions alive, and leave at the end (spec US1, FR-001 to FR-011).

**Independent Test**: quickstart step 4 (10 bots, 1 minute) passes, and steps 6–7 (cap, replacement, receive-only, session lifetime) behave as written.

### Tests for User Story 1

- [X] T008 [P] [US1] Write `src/platform/janus/streams_test.go` for the receive cap state of data-model "Receive cap". Cases:
  - streams are active up to `limit` per kind, with audio and video counted separately;
  - beyond it they wait, in FIFO order;
  - `limit` 0 means no limit;
  - the dummy publisher and `data` and `disabled` streams are ignored;
  - "announced again (same feed and mid)" is ignored;
  - a feed gone frees its active slots, each promoting the earliest waiting stream of that kind, and drops its waiting streams;
  - an unknown feed gone is a no-op;
  - the returned `update` has the exact `subscribe` and `unsubscribe` lists, and is empty when nothing changes;
  - invariant: `len(active) ≤ limit` per kind when `limit > 0`.

  The state is pure data (no I/O): methods return the update for the adapter to send.
- [X] T009 [P] [US1] Write `src/platform/janus/adapter_test.go` against T007's fake server, with real `execution.Peer`s built as the engine builds them (`PeerMedia{Kinds, ReceiveLimit, PublishSession: true}`), plus the media sources of 004 writing to their tracks. Cases:
  - (a) 3 bots `audio_video`, no cap, started together: one room `create` succeeds and the other two get 427. All reach `established`. Each receives RTP on 2 audio and 2 video streams (`MediaSnapshot`). The request order matches research R4;
  - (b) the first bot alone is `established` before the second joins: its subscription has only the dummy (research R3);
  - (c) a `receive_only` bot: no `publish`, no publish session; it is established and receives the senders' streams;
  - (d) 4 senders, cap 2:
    - each bot subscribes to 2 streams per kind;
    - when an active sender disconnects, every bot that received it sends one `update` with `unsubscribe` and `subscribe` (the earliest waiting stream of each kind), and then receives its RTP;
    - no `update` is sent while an offer is unanswered;
  - (e) keepalives: with the session timeout at 2 s and the keepalive at 500 ms (test settings), the bots stay connected for 5 s, and no `timeout` is sent;
  - (f) `Disconnect` sends `destroy` and closes the WebSocket. No SDP call runs after it returns (an SDP-call counter in a test `SignalingHandler` wrapper), and a second `Disconnect` is a no-op;
  - (g) a rejected `join` (432) fails `Connect` with a message naming `join`, 432 and the reason, and leaves nothing open (goroutine leak check as in the mediasoup tests);
  - (h) a refused handshake (status 403) fails `Connect` with the status;
  - (i) each of these calls `OnSignalingLost` exactly once after the join:
    - the server closing the WebSocket;
    - a session `timeout`;
    - `hangup` on the subscriber handle;
  - (j) a room without a dummy publisher (the fake server's room created beforehand without it) fails the join with "has no dummy publisher";
  - (k) the room is created with `publishers` = participants and with `admin_key` when configured;
  - (l) the bots' frames carry `apisecret`, `token` and `pin` when configured, and the captured logs contain none of their values.
- [X] T010 [P] [US1] Extend `src/platform/catalog/catalog_test.go` with `TestCatalog_Janus`:
  - `Platforms()` lists `jitsi`, `livekit`, `mediasoup`, `janus`;
  - `ValidateTestPlan("janus", …)` accepts a valid plan, and returns the contract's messages for simulcast, requested heights, a missing room and `room: "abc"`;
  - `SecretKeys("janus")` is `[api_secret token pin admin_key]`;
  - `NewAdapter` returns an adapter that implements `platform.PublishSessionAdapter` with `PublishSession() == true`, sized from the plan's `Participants`.

  Extend `tests/integration/api/platform_api_test.go` with `TestJanusCreationRules`, the HTTP equivalent of quickstart step 3: the 400 messages, and `"[redacted]"` for each secret in the answer.

### Implementation for User Story 1

- [X] T011 [US1] Write `src/platform/janus/streams.go` to pass T008.
- [X] T012 [US1] Write `src/platform/janus/adapter.go` to pass T009. It implements `platform.PlatformAdapter` and `platform.PublishSessionAdapter`. `NewAdapter(logger, cfg Config)`, where `cfg.Participants` is set by the catalog.
  - **`Connect`**:
    - dials `cfg.URL` with subprotocol `janus-protocol`, and `InsecureSkipVerify` when `insecure_tls`; a handshake error carries the HTTP status;
    - runs research R4 steps 1–6 in order, under the context;
    - the room `create` treats 427 as success;
    - the dummy is the publisher with `dummy: true` in `joined` (R3);
    - senders `CreatePublishOffer`, `publish` it, then `ApplyPublishAnswer` the `configured` jsep;
    - the subscriber joins with `use_msid: false`, `autoupdate: false`, `streams: [{feed: dummy}]` and `private_id`; its offer goes to `OnOffer` and the answer to `start`;
    - every subscriber offer (`attached`, `updated`) loses the `a=ssrc`, `a=ssrc-group` and `a=msid` lines of the dummy's section before `OnOffer` or `Renegotiate` (research R3), so that the dummy is not counted as an offered stream. Unit-test it on `testdata/golden/updated_offer.sdp` (the real offer of T002, candidates anonymized);
    - the publishers already in `joined` go to T011, then the first `update`;
    - each trickle function sends core `trickle` on its own handle (`sdpMid`, `sdpMLineIndex` from the mid's index in the SDP, `candidate`), then `{completed: true}` after gathering ends;
    - on any error, everything opened is closed before returning.
  - **Per-bot work goroutine** ([contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)):
    - it serializes the cap's updates and the subscriber's offers;
    - announce and gone events update T011's state; a non-empty update is sent as `update`;
    - the `updated` event's jsep goes to `Renegotiate`, its answer to `start`;
    - an `updating` event waits for the next `updated`;
    - the next `update` waits for the current offer to be answered;
    - `Renegotiate` errors are logged with the peer ID.
  - **Handlers** (by `sender`):
    - publisher handle: `event.publishers`, `unpublished` and `leaving` drive T011; `kicked` (own ID) and `destroyed` are lost;
    - subscriber handle: unsolicited `updated`;
    - either handle: `hangup` and `detached` after `Connect` are lost; `webrtcup`, `media` and `slowlink` are logged at debug;
    - session: `timeout` is lost;
    - `trickle` from Janus goes to `OnICECandidate` or `OnPublishICECandidate`.
  - **Keepalive**: 25 s (R7). The tests can lower it through an unexported field.
  - **Loss**: the read loop ending, or a lost event after `Connect`, calls `OnSignalingLost(err)` once, unless `Disconnect` started.
  - **`Disconnect`**: sends `destroy` (2 s timeout), stops the goroutine and closes the WebSocket. It is idempotent and safe for unknown peers.
  - **Logging**: one logger per bot with `peer_id`, the session ID, and `Config.LogValue`.
- [X] T013 [US1] Add `janusEntry()` to `src/platform/catalog/catalog.go`:
  - `Name: "janus"`, `Validate: janus.Validate`, `SecretKeys: []string{"api_secret", "token", "pin", "admin_key"}`;
  - `NewAdapter` parses `tp.PlatformConfig` with `janus.ParseConfig`, sets `cfg.Participants = tp.Participants`, and returns `janus.NewAdapter(logger, cfg)`.

  Append it to `New`'s entries. This passes T010.
- [X] T014 [US1] Write `tests/live/janus_live_test.go` (`//go:build live`), copied from `mediasoup_live_test.go` and adapted:
  - `TestJanusLive_JoinAndEstablishMedia` is skipped without `JANUS_URL`;
  - variables:
    - `JANUS_URL`, `JANUS_ROOM` (default `1011`), `JANUS_PARTICIPANTS`, `JANUS_RAMP_UP`, `JANUS_MEDIA`, `JANUS_MAX_RECEIVED`, `JANUS_HOLD`;
    - `JANUS_SERVER_CONTAINERS`, `JANUS_SERVER_INTERVAL`, `JANUS_METRICS_ADDR`;
    - `JANUS_INSECURE_TLS`, `JANUS_API_SECRET`, `JANUS_TOKEN`, `JANUS_PIN`, `JANUS_ADMIN_KEY`;
    - `JANUS_LEAVE_ONE`: an extra sending bot outside the engine that joins first and leaves halfway, as `joinExtraBot` in the mediasoup test;
  - the same checks, summary log and `writeReport` call, with scenario name `live-janus` and platform `janus`.

  Document the variables in the file's header comment, as the mediasoup one does. `go vet -tags live ./tests/live/` passes.
- [X] T015 [US1] Live check (quickstart steps 4, 6, 7). Start T001's server and run:
  - 10 bots `audio_video` for 60 s;
  - 6 bots with cap 3 and `JANUS_LEAVE_ONE=1`;
  - 1 `receive_only` bot alone in a new room;
  - 2 bots with a 3-minute hold (longer than the 60 s session timeout).

  During the 10-bot run, open `http://localhost:8090/demos/videoroom.html?room=1011` and check that the page shows the bots.

  Record under this task, for each run:
  - PASS/FAIL and the time to establish;
  - streams received and packets per second per bot;
  - freezes and MOS p50;
  - the WARN and ERR lines of `docker logs janus`.

  If the real server differs from the fake one (message order, SDP accepted by Pion, ICE, `updated` without jsep), fix the adapter and add the case to T007 and T009.
  - **Recorded 2026-10-23** (Janus 1.4.3, only `janus` and `janus-demos` running, screen unlocked). All four runs passed on the first try, with no adapter change:
    - **10 bots `audio_video`, 60 s** (room 2001): **PASS**.
      - Established in 1.22 s; video settled at 0.99 after 4 s.
      - 9.0 audio and 9.0 video streams offered per bot. The dummy's section is not counted (research R3).
      - 445 audio and 586 video packets/s per bot.
      - 0 freezes, MOS p50 4.37, audio concealed 0%, 30.0 fps at 360p.
      - Latency is left out with `frame_not_identified` for both kinds (T026, research R8).
      - Server `janus` 0.76 threads on average (peak 0.78), 18 MB peak memory; engine 0.46 threads.
      - Bots left in 2 ms. Reports `20261022T220002Z-janus-live-janus-60db2ce6.{json,md}`.
    - **6 bots, cap 3, `JANUS_LEAVE_ONE=1`, 60 s** (room 2002): **PASS**.
      - Established in 1.23 s.
      - 3.0 streams per kind per bot after the extra bot left halfway, so the promotion worked: 148 audio and 195 video packets/s per bot.
      - 0 freezes.
    - **1 `receive_only` bot alone** (room 2003): **PASS**, established in 204 ms through the dummy subscription (research R3).
    - **2 bots, 3-minute hold** (room 2004): **PASS**. No `timeout`, so the 25 s keepalive holds the sessions past Janus's 60 s timeout (FR-007). 0 freezes.
    - **Janus log**, WARN and ERR lines over the four runs:
      - `Room N already exists!`, one per bot after the first: the creation race of research R4, expected;
      - 12 × `Publisher … not found, not unsubscribing…`: the bots' `unsubscribe` of the extra bot's streams, after Janus had already dropped that feed. The `subscribe` half of the same `update` was applied, and the promotion worked. Harmless; the adapter could skip unsubscribing feeds that left, if it ever matters;
      - 8 × `Failed to add some remote candidates (added 0, expected 5)`: libnice drops the bots' candidates it cannot use (IPv6 and container-bridge addresses; Janus gathers on IPv4 only). The pair is found anyway, and every bot connected.
    - **Demo page**: `videoroom.js` takes `?room=`, but no browser was available in this session, so the visual check (the page showing the bots) is **left to the user**: `http://localhost:8090/demos/videoroom.html?room=<room>` during a run.
      - **Done later** (2026-10-23, headless Chrome 154 driven over the DevTools protocol, fake camera, 3 bots in room 5001): the page joined the room and played the 3 bots' videos (640x360, about 495 kbit/s each, labelled with their peer IDs). Its own camera publish was refused with `Maximum number of active publishers (3) already reached`, since the room has one place per bot; the README now says so, and that the display name must be alphanumeric.

**Checkpoint**: Janus runs from the API and the live test. US1 is independently usable as a load test.

---

## Phase 4: User Story 2 - The same report for Janus (Priority: P2)

**Goal**: a Janus run's summary, report files and metrics have the same form as the other platforms' (spec US2, FR-012 to FR-014).

**Independent Test**: quickstart step 4's field-set diff against a mediasoup report is empty apart from indexes and `platform_config` keys; latency and server load are reported or explained.

- [X] T016 [P] [US2] Extend `src/report/writer_test.go` `TestWriteNoSecrets` with a `janus` scenario whose `platform_config` has `api_secret`, `token`, `pin` and `admin_key`. The values are absent from both files, and `"[redacted]"` is in their place.
- [X] T017 [US2] Field-set check (SC-004, FR-012). Run quickstart step 4's `jq` diff between T015's 10-bot Janus report and a 10-bot mediasoup report of the same plan, taken the same day with mediasoup started from `tests/platforms/mediasoup`. Record the diff under this task: only array indexes and `platform_config` keys may differ.
  - **Recorded 2026-10-23**:
    - **Runs compared**: T015's 10-bot Janus report (`20261022T220002Z-janus-live-janus-60db2ce6.json`) and a 10-bot mediasoup run of the same plan, taken 7 minutes later with mediasoup-demo started from `tests/platforms/mediasoup` (`20261022T220654Z-mediasoup-live-mediasoup-d63e5419.json`, PASS: 9 + 9 streams per bot, 0 freezes, MOS 4.37).
    - **Tool**: `jq` is not installed on this machine, so the paths were compared with an equivalent Python script (array indexes folded).
    - **Result**: no difference except latency. mediasoup has `quality.{audio,video}.latency_seconds` and `worst.highest_latency_p95_seconds`. Janus has `quality.{audio,video}.latency_reason` in their place. This is FR-013's "left out with its reason" (research R8), the same optional fields as on any platform where latency is not measured. Every other field matches, `platform_config` keys included (`url`, `room`).
- [X] T018 [US2] Latency (FR-013, research R8, quickstart step 5). In T015's 10-bot summary, record whether `audio.latency` and `video.latency` are present and their p50s. Compare with T002's timestamp finding. If one is missing:
  - check that the summary gives the 008 `latency_reason`;
  - say in research R8 which Janus behaviour removed it.
  - **Recorded 2026-10-23**: in T015's 10-bot summary, `audio` and `video` both have `latency_available: false` and `latency_reason: frame_not_identified`. This matches T002: Janus rebases timestamps, so no received timestamp matches a sent frame since T026. mediasoup's run of T017, taken after T026, still measures latency (p50 1.5 ms audio, 2.5 ms video), so the new initial timestamp leaves platforms that keep timestamps unchanged. research R8 already names the Janus behaviour.
    - **Corrected in T022**: true for mediasoup and LiveKit, not for Jitsi's video. With 2^31, Jitsi's bridge rebased video timestamps, so T022's Jitsi runs had no video latency. T026 now uses 2^30 (research R8, "Found in T022").
- [X] T019 [US2] Server monitoring (FR-014). In T015's 10-bot report, the server section lists `janus` as `measured`, with CPU and memory, and does not list `janus-demos`. Check the CPU against `docker stats janus` during a run (same order of magnitude). Record both under this task.
  - **Recorded 2026-10-23** (a 10-bot, 60 s run in room 2005, PASS):
    - the server section lists `janus` as `measured`, with 0.743 threads on average (peak 0.78) and 17.8 MB peak memory. `janus-demos` is not listed: only named containers are sampled;
    - `docker stats janus`, sampled 6 times during the hold, gave 73–79 % CPU (0.73–0.79 threads) and 16–17 MiB, which agrees with the report.

---

## Phase 5: User Story 3 - A local Janus server and repeatable tests (Priority: P3)

**Goal**: the setup starts with one documented command, the tests run without a server, and Janus joins the cross-platform comparison (spec US3, FR-015 to FR-017, SC-002, SC-006, SC-007).

**Independent Test**: T021's clean-checkout run, and T022's alternated 50-bot runs.

- [X] T020 [US3] Complete `tests/platforms/janus/README.md`:
  - **start**: copy `env.example` to `.env`, `docker compose up -d --build`; the first build takes about T001's build time;
  - **settings**: the `platform_config` to use, with `url: ws://localhost:8188` and a numeric `room`;
  - **containers**: `janus` for server monitoring (`server_containers`); `janus-demos` is not part of the server's load;
  - **rooms**: the bots create the room with the dummy publisher and one publisher place per bot (the dummy takes none, T002). A room keeps the size it was created with, so use a new room number when the bot count grows, or restart Janus (research R4);
  - **watching a room**: `http://localhost:8090/demos/videoroom.html?room=<room>`;
  - the RTP port range, and `ice_enforce_list` for bots on another network namespace;
  - **the live test command** of quickstart step 4;
  - **stop**: `docker compose down`.
- [X] T021 [US3] Clean-checkout run (SC-006):
  1. In a fresh `git worktree` of the branch in the scratchpad, remove the image (`docker rmi webrtc-load-engine/janus:v1.4.3`).
  2. Follow only the README, up to the end of the 10-bot live test.
  3. Time it: it must take under 15 minutes.
  4. Run `go test -race ./src/platform/janus/` with the network off (`unshare -n` or no Janus running): it passes (SC-007).

  Record the times and any README fix under this task.
  - **Recorded 2026-10-23**:
    - **Setup**: the worktree was made from HEAD (`68e7205`), with this feature's uncommitted changes applied on top (`git diff HEAD` plus the untracked files), so it is a clean checkout of the work as it stands. The `janus` and `janus-demos` images were removed first.
    - **Timing**: following only the README, with `docker compose build --no-cache` for a worst-case build, Janus started in **95 s** and the README's 10-bot live test passed (established in 1.22 s, 9 + 9 streams per bot). That is **2 min 54 s** from `cp env.example .env` to PASS (SC-006: under 15 min). The Go module cache was warm, as it is on any machine that has built the engine.
    - **README fix**: there are four start-up WARN lines, not three (also "Couldn't access logger plugins folder", since no loggers are built).
    - **No server**: with both Janus containers stopped, `go test -race ./src/platform/janus/` passes (SC-007).
    - **No network**: in a network namespace with only `lo` (`unshare -rn`), `TestAdapter_ReceiveOnlyBot` and `TestAdapter_ReceiveCapAndPromotion` fail: the bots are offered the streams but receive no packets. **mediasoup's tests of the same name fail in the same way there**, so this is not Janus-specific. The engine's test peers need a non-loopback interface, which every real machine has. Left as a follow-up of the engine's tests; SC-007's "no server" holds.
- [X] T022 [US3] 50 bots, alternated with the other three servers (SC-002, research R12, quickstart step 8):
  - build the four live test binaries once;
  - run, in this order: Jitsi, LiveKit, mediasoup, Janus, then the same order again;
  - each run is 50 bots `audio_video`, cap 10, ramp-up 10 s, 10 min hold;
  - each server runs alone, started just before its run, with the screen unlocked (`loginctl` `LockedHint=no`).

  Record under this task, per run:
  - PASS/FAIL, and bots established for the whole hold;
  - MOS p50, audio concealed p95, frame rate p50, freezes and latency p50;
  - server CPU threads (average and peak) and memory peak;
  - engine CPU threads (average, checked against `/usr/bin/time`).

  Add a Janus column to the table in `specs/010-mediasoup-platform/validation-report.md` "Three servers, 50 bots", or repeat the table, with all eight runs, in 011's validation report (T024).
  - **Recorded 2026-10-23** (22:15–23:40 UTC on 2026-10-22, screen unlocked in all eight runs): all eight runs **PASS**, 50/50 bots held for 10 min, 10 + 10 streams per bot, MOS p50 4.37 and 30 fps everywhere. Freezes: Jitsi 68 and 107, the others 0. Server CPU average: Jitsi 1.69 / 1.55, LiveKit 0.97 / 0.98, mediasoup 0.42 / 0.41, Janus 1.75 / 2.01 threads (Janus peak 7.1 / 6.9); Janus memory peak 62 MB. Engine 0.53–0.76 threads, agreeing with `/usr/bin/time`. The full table is in the validation report "Four servers, 50 bots".
    - **Jitsi video latency** was `frame_not_identified` in both Jitsi runs: the 2^31 start of T026 makes the bridge rebase video timestamps. T026 moved to 2^30; a third 50-bot Jitsi run with it PASS, video latency p50 1.5 ms (research R8).
    - **CPU against 010 T029**: 1.4–2.9 times lower for the same plan, with 10-bot runs unchanged: the machine under sustained load (thermal throttling), not the code. Only same-set comparisons are valid.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T023 [P] Update the repository's `README.md`, in the style of the 010 entries:
  - the features table gets a row "011 Janus";
  - "adapters for other platforms (Janus)" is removed from the "Not done yet" list;
  - a "**Janus (011).**" paragraph: platform, `platform_config`, rejected options, the local setup command and the live test command.

  Update `CLAUDE.md`:
  - the "Commands" block gets the Janus live test command (`JANUS_URL=ws://localhost:8188 JANUS_ROOM=1011 JANUS_PARTICIPANTS=10 JANUS_SERVER_CONTAINERS=janus go test -tags live -count=1 -v -run Janus ./tests/live/`);
  - the "Recent Changes" 011 line drops "(planned)".
- [X] T024 Write `specs/011-janus-platform/validation-report.md`, as 010's:
  - the setup (machine, Janus version, libnice version, screen state);
  - a table of SC-001 to SC-007 with result and evidence (T015, T017–T019, T021, T022, T025);
  - T002's findings (dummy subscription, timestamps, ICE), the latency finding (T018) and the four-server table (T022);
  - deviations from the plan.
- [X] T025 Final checks:
  - `go vet ./... && go vet -tags live ./tests/live/ && go test -race -count=1 ./...` all pass;
  - this prints nothing apart from test data in `src/report/writer_test.go` (boundary, constitution III):
    `grep -rni "janus\|videoroom" src --include=*.go | grep -v "src/platform/janus\|src/platform/catalog"`;
  - `git diff <commit before T001> -- src/execution src/media src/scenario src/report` shows only test files, and `src/media/source.go` (T026);
  - `go.mod` has no new requirement;
  - one 2-bot Jitsi live run, one 10-bot LiveKit live run and one 10-bot mediasoup live run still pass (FR-011).

  Record the results under this task.
  - **Recorded 2026-10-23**: vet and race tests pass; the boundary grep shows only `writer_test.go` test data and the explanatory comment in `src/media/source.go`; the engine packages changed only in tests and `source.go` (T026); `go.mod` unchanged. Jitsi 2-bot, LiveKit 10-bot, mediasoup 10-bot and Janus 10-bot live runs (60 s) all PASS. Details in the validation report "Final checks".

---

## Dependencies & Execution Order

- **Setup (T001–T002)**: first. T002 needs T001's server and produces the goldens of T004, T005 and T007. Its findings may change research R3, R5, R8 and R10 and the contracts before Phase 2.
- **Foundational (T003–T007)**:
  - T003, T004 and T005 are independent files and can run in parallel;
  - T006 touches only the catalog and can run alongside them;
  - T026 (added after T002) touches only `src/media/source.go` and can run alongside them;
  - T007 needs T004 and T005.
- **US1 (T008–T015)**:
  - the tests T008–T010 can run in parallel (T010 needs T006);
  - T011 needs T008; T012 needs T009 and T011; T013 needs T010 and T012;
  - T014 needs T013; T015 needs T014 and T001.
- **US2 (T016–T019)**: T016 needs T013. T017–T019 need T015's runs (and the mediasoup server for T017).
- **US3 (T020–T022)**: T020 can start after T015, and T021 needs T020. T022 needs T015 and the three other local servers.
- **Polish (T023–T025)**: last. T024 needs every recorded task.

### Parallel Example: Foundational

```text
T003 config.go   T004 messages.go   T005 client.go   T006 catalog signature   T026 source timestamps
then
T007 fakeserver_test.go
```

### Parallel Example: User Story 1

```text
T008 streams_test.go   T009 adapter_test.go   T010 catalog_test.go + platform_api_test.go
then
T011 streams.go → T012 adapter.go → T013 catalog.go → T014 live test → T015 live check
```

### Parallel Example: User Story 2

```text
T016 writer_test.go (after T013)   T017 field-set diff   T018 latency   T019 server load (after T015)
```

## Implementation Strategy

1. **MVP** (T001–T015): Setup, Foundational, then US1. Janus runs from the API, and T015 proves it on the real server.
2. **US2** (T016–T019): the comparable report, with latency and server load checked on the real server.
3. **US3** (T020–T022): reproducibility from a clean checkout, and the four-server comparison at 50 bots.
4. **Polish** (T023–T025).

The engine is not changed. The shared changes are the catalog signature
(T006) and the sources' initial RTP timestamp (T026), so T025's checks on
the other platforms matter.
Still run T025's checks on the other platforms before calling the feature
done.
