---

description: "Task list for 010 mediasoup-demo platform"
---

# Tasks: mediasoup-demo Platform

**Input**: Design documents from `specs/010-mediasoup-platform/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires contract tests for signaling adapters (plan "Constitution Check"). Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green after every task.
- **Boundaries**: no mediasoup or protoo name, type or JSON outside `src/platform/mediasoup` and `src/platform/catalog` (constitution III). `src/execution`, `src/media`, `src/scenario` and `src/report` are not changed (T016 found two platform-neutral exceptions in `src/execution`, see plan.md) (plan; [contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)). No new module in `go.mod`.
- **Metrics in tests**: tests that read global metrics use a unique scenario ID (uuid), as in 004–009.
- **Secret**: `unlimited_room_secret` is never in logs, answers or reports. Tests grep captured logs and files for it.
- **Live runs**: these target `tests/platforms/mediasoup` with docker-jitsi-meet and LiveKit stopped ([quickstart.md](quickstart.md)), except the alternated runs of T023. The machine state (desktop on or screen locked) must be the same in runs that are compared (research R13).
- **Mirror the LiveKit adapter**: file layout, logging style, `Config.LogValue`, and the fake-server test helpers of `src/platform/livekit` are the model. Copy their shape, not their protocol.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: the local server, and the real messages every later task is tested against (research R1, R11)

- [X] T001 Create `tests/platforms/mediasoup/` per research R11:
  - **`Dockerfile`**:
    - `FROM node:22-bookworm`, `ARG MEDIASOUP_DEMO_COMMIT=899109e4046b45da1623a3e4c65a742040479439`;
    - `git clone https://github.com/versatica/mediasoup-demo.git /demo && git -C /demo checkout $MEDIASOUP_DEMO_COMMIT`;
    - `WORKDIR /demo/server`, `RUN npm ci` (the lock file pins mediasoup; if `npm ci` does not run the TypeScript build through `prepare`, add `RUN npm run typescript:build`);
    - `CMD ["node", "lib/index.js"]`, or the entry `package.json`'s `bin` points at; check it.
  - **`config.mjs`**: copied from the commit's `server/config.example.mjs`, with these changes:
    - the `http.tls` block removed (plain HTTP and WS);
    - `http.listenIp: '127.0.0.1'`, `listenPort: 4443`;
    - `mediasoup.numWorkers: Number(process.env.MEDIASOUP_NUM_WORKERS ?? 2)`;
    - `webRtcServerOptions` listen infos on `ip: '127.0.0.1'`, `announcedAddress: process.env.MEDIASOUP_ANNOUNCED_ADDRESS`, port 44444;
    - `workerSettings.logLevel: 'warn'`.

    Mount it as `/demo/server/config.mjs:ro` (or set `CONFIG_FILE`).
  - **`docker-compose.yml`**: service `mediasoup`, `build: .`, `image: webrtc-load-engine/mediasoup-demo:899109e`, `container_name: mediasoup-demo`, `network_mode: host`, `restart: unless-stopped`, `env_file: .env`, `environment: DEBUG: "mediasoup-demo-server:INFO* mediasoup-demo-server:WARN* mediasoup-demo-server:ERROR* mediasoup:WARN* mediasoup:ERROR*"`.
  - **`env.example`**: `MEDIASOUP_NUM_WORKERS=2`, `MEDIASOUP_ANNOUNCED_ADDRESS=` (comment: set the LAN address if bots run on another network namespace). Add a commented `# UNLIMITED_ROOM_SECRET=` line saying it must stay unset for unlimited rooms (research R9).
  - **`README.md`**: written in T025; a stub with start and stop now.

  Add `tests/platforms/mediasoup/.env` to `.gitignore`. Run `docker compose up -d --build`, then check:
  - the log shows 2 workers;
  - `ss -lntu` shows TCP 4443 and UDP/TCP 44444–44445 on 127.0.0.1.

  Record the build time and the image's mediasoup version (`docker exec mediasoup-demo node -e "console.log(require('mediasoup').version)"`) under this task.
  - **Recorded 2026-10-20**: built in 24 s (cached base). Base image changed to `node:22-trixie`: the prebuilt `mediasoup-worker` "fails to run in this host" on bookworm (glibc 2.36), and `npm ci` then tries to build it with pip, which is missing. `npm ci` runs `prepare` (TypeScript build), so no extra step; entry `node lib/index.js`. mediasoup **3.30.1**. Log: "launching 2 mediasoup Workers", "Server started". Listening on 127.0.0.1: TCP 4443, UDP and TCP 44444–44445.
  - **Origin check**: the server rejects (403) a WebSocket whose `Origin` header is missing or whose scheme and hostname differ from `http(s)://<config domain>` (`WsServer.ts`, `utils.areSameHttpOrigins`). `config.mjs` takes `domain` from `MEDIASOUP_DOMAIN` (default `localhost`). The adapter must send an `Origin` (research R1 updated: derived from the URL, or the `origin` setting).
- [X] T002 In the scratchpad (never committed), write a throwaway Go probe with `nhooyr.io/websocket` (subprotocol `protoo`) against T001's server. Two clients join room `probe`. Client A does the requests of research R4 steps 2–5 without media (`join` with capabilities filtered per R5). Client B does the same, plus a `produce` of audio with hand-made `rtpParameters` (Opus PT 111, SSRC 1111, mid `0`, cname `probe`) on a producer transport that it never connects.
  1. **Golden JSON**: save the raw frames under `src/platform/mediasoup/testdata/golden/`, each pretty-printed: `router_rtp_capabilities.json`, `create_transport_consumer.json`, `create_transport_producer.json`, `join_response.json`, `new_consumer_audio.json` (the `newConsumer` A receives for B's producer), `new_data_consumer.json`, `consumer_closed.json` (after B disconnects), `peer_closed.json`, `mediasoup_version.json`, and one error response (send `join` twice: `error_response.json`).
  2. **Check and record under this task**:
     - the subprotocol is required (a dial without it is refused);
     - the candidates' addresses, ports and protocols;
     - the transport's `sctpParameters`;
     - whether `newConsumer` arrives before or after A's `join` response;
     - the `newConsumer` request's `id` type;
     - after A answers `newConsumer` with `ok:false`, whether a later `resumeConsumer` notification produces no error in the server log (research R6);
     - the HTTP status of a handshake without `roomId`.

  If a fact differs from research R1/R4/R6, update research.md and the contracts before Phase 2.
  - **Recorded 2026-10-20** (mediasoup 3.30.1): goldens saved, `new_consumer_audio.json` with Opus PT 100, consumer mid `"0"`, `type: "simple"`; `newDataConsumer` (the demo's chat bot) arrives right after the `join` response, `newConsumer` after the producer appears.
    - Handshake statuses: without the `protoo` subprotocol **403**; without `Origin` **403** (see T001); without `roomId` **400**.
    - Candidates: `127.0.0.1:44444` UDP (foundation `udpcandidate`) and TCP passive; fields `address`, `ip`, `port`, `priority`, `protocol`, `type`, `tcpType`. `iceParameters.iceLite: true`. DTLS `role: "auto"` with sha-1…sha-512 fingerprints.
    - `sctpParameters`: `port 5000`, `OS/MIS 65535`, `maxMessageSize 5000000`, `isDataChannel: true`.
    - Request IDs are JSON numbers (random 7-digit).
    - **An error response has no `ok` field** (`{"response":true,"id":…,"errorCode":500,"errorReason":"Peer already joined"}`): treat any response without `ok: true` as an error.
    - After declining `newConsumer` (403 "receive cap"), the server logs two WARN lines (`<-- error response [name:'newConsumer']`, `consume() | failed`); a later `resumeConsumer` notification for it logs no error (R6 holds).
    - Router codecs: Opus 100 (`nack`, `transport-cc`), VP8 101 (`nack`, `nack pli`, `ccm fir`, `goog-remb`, `transport-cc`), rtx 102 (apt 101), plus VP9/H264. Header extensions carry `kind` and `direction`; audio `transport-cc` is `recvonly` (server side), so audio consumers do not carry it. Passing the router's whole extension list in `join` gives consumers `playout-delay` and `urn:mediasoup:params:rtp-hdrext:packet-id` too: the R5 filter is needed.

---

## Phase 2: Foundational

**Purpose**: config, protoo client, mediasoup types and both SDP translations, which every story uses. Each is pure code with golden tests.

- [X] T003 [P] Write `src/platform/mediasoup/config_test.go`, then `config.go`, per [contracts/mediasoup-config.yaml](contracts/mediasoup-config.yaml) and data-model "Config":
  - `ParseConfig(map[string]any) (Config, error)`;
  - `Validate(scenario.TestPlan) error`;
  - `Config.LogValue` and `String` without the secret.

  Rules:
  - `url` is "required. `ws://`, `wss://`, `http://` or `https://`; http(s) becomes ws(s); no query";
  - `room` is "required, non-empty";
  - `insecure_tls` and `use_pipe_transports` are booleans, default false;
  - `unlimited_room_secret` is a string;
  - unknown keys are rejected with `platform_config: unknown key %q for platform mediasoup`;
  - `media.simulcast` and `media.requested_heights` are rejected with the contract's messages.

  Each error message of the contract has a test. A test logs a config with a secret and greps the output.
- [X] T004 [P] Write `src/platform/mediasoup/messages.go`: Go structs with JSON tags for the mediasoup types used in [contracts/signaling-messages.md](contracts/signaling-messages.md):
  - `RtpCapabilities`, `RtpCodecCapability`, `RtpHeaderExtension`;
  - `RtpParameters`, `RtpCodecParameters`, `RtcpFeedback`, `RtpHeaderExtensionParameters`, `RtpEncodingParameters` (with `rtx.ssrc`), `RtcpParameters`;
  - `IceParameters`, `IceCandidate`, `DtlsParameters`, `DtlsFingerprint`, `SctpParameters`;
  - `TransportInfo` (the `createWebRtcTransport` response);
  - `NewConsumerRequest`.

  Codec `parameters` is a `map[string]any`. In `messages_test.go`, decode every T002 golden file into its type and re-encode it: no field the adapter uses is lost.
- [X] T005 [P] Write `src/platform/mediasoup/protoo_test.go`, then `protoo.go`: a protoo client over a `*websocket.Conn` (research R1, contract "Framing"):
  - `Request(ctx, method, data, &resp) error`: IDs are unique per connection, from a random start; the call waits for the matching response or its 15 s timeout. A response with `ok:false` returns an error naming the method, `errorCode` and `errorReason`;
  - `Notify(method, data) error`;
  - a read loop that dispatches server requests and notifications to handlers. A server request gets exactly one response: the handler returns data or an error code and reason. With no handler, the answer is 405;
  - `Close()` fails pending requests with "connection closed";
  - `Done() <-chan struct{}` and `Err()` report the read loop's end.

  Tests run against an `httptest` WebSocket server that speaks protoo:
  - concurrent requests answered out of order;
  - an error response;
  - a timeout;
  - a server request answered ok and with an error;
  - an unknown server request answered 405;
  - notifications;
  - a close with requests pending;
  - a dial without the `protoo` subprotocol, refused by the test server.
- [X] T006 [P] Write `src/platform/mediasoup/sdp_test.go` (receive part) first, then the receive side of `sdp.go`, per [contracts/sdp-mapping.md](contracts/sdp-mapping.md) "Receive offer":
  - `type recvOffer` holds the transport (T004 `TransportInfo`) and the ordered sections of data-model "Receive section";
  - `(*recvOffer).SDP() (string, error)` uses `pion/sdp/v3`, with the session version incremented per call.

  The application section is first, with mid `datachannel`. Consumer sections are `sendonly` with SSRC lines; inactive ones are `a=inactive` without SSRC lines. Only UDP candidates are kept.

  Golden tests on T002's JSON:
  - (a) application only;
  - (b) application, then audio and video consumers (video with RTX);
  - (c) as (b) with the audio section inactive.

  Each output also goes through a Pion `PeerConnection` (`SetRemoteDescription`, `CreateAnswer`, `SetLocalDescription` with no error), and the sections the peer counts as offered (those with `a=ssrc` lines and not `inactive`/`recvonly`, as `execution.Peer.recordOffered` counts them) are 1 audio and 1 video in (b), and 0 audio and 1 video in (c).
- [X] T007 [P] Write the publish half of `sdp_test.go` first, then the publish side of `sdp.go`, per [contracts/sdp-mapping.md](contracts/sdp-mapping.md):
  - `producerParams(offer string, routerCaps RtpCapabilities) ([]ProducerSpec, error)`: one per offered section, with kind, mid and `RtpParameters`;
  - `publishAnswer(offer string, t TransportInfo, specs []ProducerSpec) (string, error)`: `setup:passive`, `recvonly`, same mids and order;
  - `localFingerprint(sdp string) (DtlsFingerprint, error)`: reads the `sha-256` fingerprint, at media or session level.

  Golden tests use a real Pion publish offer (`TrackLocalStaticRTP` Opus and VP8 tracks on a PeerConnection built as `execution`'s publish session does: default codecs, no TWCC) and T002's router capabilities:
  - the produced parameters keep Pion's payload types, the RTX `apt`, the first SSRC and the FID SSRC, the cname, and only the extensions the router supports;
  - the answer is accepted by the offering Pion PeerConnection (`SetRemoteDescription` with no error);
  - an offer without Opus or VP8 is an error naming the codec.
- [X] T008 Write `src/platform/mediasoup/fakeserver_test.go`, a test helper (research R12): an `httptest` server speaking protoo as mediasoup-demo does, per [contracts/signaling-messages.md](contracts/signaling-messages.md). For each peer:
  - it answers `getRouterRtpCapabilities` (T002's golden);
  - `createWebRtcTransport` returns the parameters of a real Pion PeerConnection set up as ICE-lite (`SettingEngine.SetLite(true)`) on 127.0.0.1, whose ICE/DTLS parameters are those it returns;
  - `connectWebRtcTransport` records the role and fingerprint and checks the fingerprint against the bot's later SDP;
  - `join` and `produce` (it plays the producer's offer section into its Pion PC from the received `rtpParameters`, and forwards each producer's RTP to its consumers).

  After a peer joins and a producer appears, the server sends `newConsumer` to every other joined peer with a consumer transport, plus one `newDataConsumer` per joined peer. It sends `consumerClosed` and `peerClosed` when a peer leaves.

  Hooks let tests:
  - fail a given request;
  - refuse the handshake with a status;
  - close a peer's WebSocket;
  - read what each peer accepted or declined, and its `resumeConsumer` notifications.

  The forwarding may be simplified: rewrite SSRC and payload type only, timestamps untouched.

**Checkpoint**: config, protoo, types and SDP translation pass on goldens. The adapter can be built on them.

---

## Phase 3: User Story 1 - Run a test plan against mediasoup-demo (Priority: P1) 🎯 MVP

**Goal**: bots join a mediasoup-demo room, send and receive audio and video within the receive cap, and leave at the end (spec US1, FR-001 to FR-009).

**Independent Test**: quickstart step 3 (10 bots, 1 minute) passes, and steps 5–6 (cap, replacement, receive-only) behave as written.

### Tests for User Story 1

- [X] T009 [P] [US1] Write `src/platform/mediasoup/consumers_test.go` for the receive cap state of data-model "Receive cap". Cases:
  - accept up to `limit` per kind (audio and video counted separately);
  - decline beyond it, with the waiting list in FIFO order;
  - `limit` 0 means no limit;
  - closing an active consumer frees its slot and promotes the earliest waiting one of that kind;
  - closing a waiting consumer drops it;
  - closing an unknown ID is a no-op;
  - the invariants "`len(active) ≤ limit` per kind, when `limit > 0`" and "A waiting consumer is never in the receive offer".

  The state is pure data (no I/O): methods return the action (`accept`, `decline`, `promote <consumer>`, `deactivate <section>`) for the adapter to carry out.
- [X] T010 [P] [US1] Write `src/platform/mediasoup/adapter_test.go` against T008's fake server, with real `execution.Peer`s built as the engine builds them (`PeerMedia{Kinds, ReceiveLimit, PublishSession: true}`), plus the media sources of 004 writing to their tracks:
  - (a) 3 bots `audio_video`, no cap: all reach `established`; each is offered 2 audio and 2 video streams and receives RTP on them (`MediaSnapshot`). The fake server saw `connectWebRtcTransport` with `role: "client"` on both transports, and the R4 request order;
  - (b) the first bot alone is `established` before the second joins (research R3);
  - (c) a `receive_only` bot (no kinds): no producer transport is created, it is established, and it receives the senders' streams;
  - (d) 4 senders and a cap of 2: each bot accepts 2 per kind and declines the rest, and the fake server records the declines. When one accepted sender disconnects, every bot sends `resumeConsumer` for the earliest waiting consumer of each kind, sends it only after its `Renegotiate` returned, and then receives its RTP;
  - (e) `newDataConsumer` is declined, and the bot stays established;
  - (f) `Disconnect` closes the WebSocket, no SDP call runs after it returns (an SDP-call counter in a test `SignalingHandler` wrapper), and a second `Disconnect` is a no-op;
  - (g) a `join` rejected by the fake server fails `Connect` with a message naming `join` and the reason, and leaves nothing open (goroutine leak check as in the LiveKit adapter tests);
  - (h) a refused handshake (status 403) fails `Connect` with the status;
  - (i) the server closing the WebSocket after the join calls `OnSignalingLost` once;
  - (j) a router without VP8 fails the join of a video-sending bot, naming the codec;
  - (k) the secret and URL query are absent from captured logs (config with `unlimited_room_secret`); the dial URL had `unlimitedRoomSecret` and, with `use_pipe_transports`, `usePipeTransports=true`.
- [X] T011 [P] [US1] Extend `src/platform/catalog/catalog_test.go` with `TestCatalog_Mediasoup`:
  - `Platforms()` lists `jitsi`, `livekit`, `mediasoup`;
  - `ValidateTestPlan("mediasoup", …)` accepts a valid plan and returns the contract's messages for simulcast, requested heights and a missing room;
  - `SecretKeys("mediasoup")` is `[unlimited_room_secret]`;
  - `NewAdapter` returns an adapter that implements `platform.PublishSessionAdapter` with `PublishSession() == true`.

  Extend `tests/integration/api/platform_api_test.go` with `TestMediasoupCreationRules`, the HTTP equivalent of quickstart step 2 (400 messages, `"[redacted]"` in the answer).

### Implementation for User Story 1

- [X] T012 [US1] Write `src/platform/mediasoup/consumers.go` to pass T009.
- [X] T013 [US1] Write `src/platform/mediasoup/adapter.go` to pass T010. It implements `platform.PlatformAdapter` and `platform.PublishSessionAdapter`.
  - **`Connect`**: dials `<url>/?roomId=…&peerId=…` with optional `unlimitedRoomSecret` and `usePipeTransports=true`, subprotocol `protoo`, and `InsecureSkipVerify` when `insecure_tls`. The handshake error carries the HTTP status. Then it runs research R4 steps 2–6, in order, under the context:
    - build `rtpCapabilities` per R5 from the router's: Opus, VP8 and its RTX with the router's `preferredPayloadType`s, and the four header extensions listed there;
    - open the receiving session with `OnOffer` on the data-channel-only offer (R3), then `connectWebRtcTransport` with `role:"client"` and the answer's fingerprint;
    - for senders: `CreatePublishOffer`, `connectWebRtcTransport`, `produce` per spec (`appData.source` `mic` or `webcam`), then `ApplyPublishAnswer`.

    Trickle functions drop candidates. On any error, everything opened is closed before returning.
  - **Per-bot SDP goroutine** ([contracts/platform-boundary-use.md](contracts/platform-boundary-use.md)): it runs the receive offer changes and the cap actions of T012 in arrival order:
    - accept: add the section, `Renegotiate`, then answer `ok:true`;
    - decline: answer 403 "receive cap";
    - promote: add the section, `Renegotiate`, then `Notify("resumeConsumer")`;
    - deactivate: mark the section inactive, then `Renegotiate`.

    `Renegotiate` errors are logged, and the server request is answered with an error.
  - **Handlers**:
    - `newDataConsumer`: 403 "data channels not used";
    - `consumerClosed`: drives T012;
    - `peerClosed`: debug log;
    - `mediasoupVersion`: logged once per adapter at INFO.
  - **Loss**: the protoo read loop ending after `Connect` calls `OnSignalingLost(err)` once, unless `Disconnect` started.
  - **`Disconnect`**: stops the SDP goroutine and closes the WebSocket. It is idempotent and safe for unknown peers.
  - **Logging**: one logger per bot with `peer_id`, the URL without query, and `Config.LogValue`.
- [X] T014 [US1] Add `mediasoupEntry()` to `src/platform/catalog/catalog.go`: `Name: "mediasoup"`, `Validate: mediasoup.Validate`, `SecretKeys: []string{"unlimited_room_secret"}`, and `NewAdapter` via `mediasoup.ParseConfig` and `mediasoup.NewAdapter(logger, cfg)`. Append it to `New`'s entries. This passes T011.
- [X] T015 [US1] Write `tests/live/mediasoup_live_test.go` (`//go:build live`), copied from `livekit_live_test.go` and adapted:
  - `TestMediasoupLive_JoinAndEstablishMedia` is skipped without `MEDIASOUP_URL`;
  - variables: `MEDIASOUP_URL`, `MEDIASOUP_ROOM` (default `load010`), `MEDIASOUP_PARTICIPANTS`, `MEDIASOUP_RAMP_UP`, `MEDIASOUP_MEDIA`, `MEDIASOUP_MAX_RECEIVED`, `MEDIASOUP_HOLD`, `MEDIASOUP_SERVER_CONTAINERS`, `MEDIASOUP_SERVER_INTERVAL`, `MEDIASOUP_METRICS_ADDR`, `MEDIASOUP_UNLIMITED_ROOM_SECRET`, `MEDIASOUP_PIPE` (sets `use_pipe_transports`), `MEDIASOUP_LEAVE_ONE` (an extra sending bot outside the engine that joins first and leaves halfway, as `joinExtraBot`);
  - the same checks, summary log and `writeReport` call, with scenario name `live-mediasoup` and platform `mediasoup`.

  Document the variables in the file's header comment, as the LiveKit one does. `go vet -tags live ./tests/live/` passes.
- [X] T016 [US1] Live check (quickstart steps 3, 5, 6). Start T001's server and run:
  - 10 bots `audio_video` for 60 s;
  - 12 bots with cap 5 and `MEDIASOUP_LEAVE_ONE=1`;
  - the receive-only pair in the same room as a 3-bot sending run;
  - a receive-only bot alone.

  Record under this task, for each run: PASS/FAIL, time to establish, streams offered and packets per second per bot, freezes, MOS p50, and the server log's WARN/ERROR lines. If the real server differs from the fake one (message order, SDP accepted by Pion, ICE), fix the adapter and add the case to T008/T010.
  - **Recorded 2026-10-20** (mediasoup 3.30.1, 2 workers, LiveKit and Jitsi stopped):
    - **First 10-bot run FAILED at 20 s**: mediasoup closed every transport, "ICE consent expired due to timeout". The demo creates transports with `iceConsentTimeout: 20` (`Room.ts`, not configurable), and Pion sends its keepalive Binding request only on an idle pair (`pion/ice` v2.3.38 `checkKeepalive`). With media flowing it never sends one, so consent (RFC 7675) is never refreshed. With the check off in the container (temporary, then recreated): PASS. **Fix, as the user chose (2026-10-20)**: the engine now sends browser-like consent checks, a Binding request every 5 s ±20% on the selected pair's own socket, for every platform (`src/execution/consent.go`; `TestConsent_ChecksWhileTrafficFlows` sees 0 requests without it). `pion/ice/v2` and `pion/stun` move from indirect to direct requirements (no new module).
    - **Second bug, from 008**: a receive-only peer with `PublishSession` opened a publish PeerConnection that never connects, so the peer was never established (LiveKit too). Fixed: the publish session is created only for a peer that sends (`TestPeer_ReceiveOnlyHasNoPublishSession`).
    - 10 bots `audio_video`, 60 s (unmodified demo): **PASS**, established in 406 ms, 9.0 audio and 9.0 video streams offered per bot, 446 audio and 586 video packets/s per bot, video settled at 0.99 after 4 s, 0 freezes, MOS p50 4.37, latency p50 0.5 ms audio / 2.5 ms video, server 0.21 threads avg, engine 0.54 threads. Reports `20261020T070507Z-mediasoup-live-mediasoup-838d3854.{json,md}`.
    - 12 bots, cap 5, `MEDIASOUP_LEAVE_ONE=1`, 60 s: **PASS**, established in 409 ms, 5.0 streams per kind per bot after the extra bot left halfway (247 audio and 324 video packets/s), 0 freezes. Server log: the declined consumers' WARN pairs (`<-- error response [name:'newConsumer']`, `consume() | failed: Error: receive cap`), no ERROR.
    - 2 `receive_only` bots in a room with 3 senders: **PASS** (3 senders: 2 streams per kind each, 0 freezes); the receive-only bots got audio (MOS 4.37) and video (30 fps). Latency `not_measured`: the senders run in another process, so there are no send times.
    - 1 `receive_only` bot alone in an empty room: **PASS**, established in 204 ms (research R3).
    - **Server warnings**:
      - `srtp_protect() failed: replay check failed (index too old)`: about 1,000 over the 10-bot minute, in bursts on a fixed 5 s period across all transports. Consent checks are jittered, so they are not the cause. Media was unaffected (0 freezes, 0.99 volume). Cause not established, probably mediasoup's bandwidth probation stream.
      - At close: `DTLS remotely closed`, and `failed to send notification [name:'peerClosed']`, which is the demo notifying bots that already left.
    - **Bot warning**: one Pion `Failed to accept data channel: … Payload Protocol Identifier … WebRTC String` per bot, the demo's SCTP welcome message on the application section (research R3). Harmless.

**Checkpoint**: mediasoup runs from the API and the live test. US1 is independently usable as a load test.

---

## Phase 4: User Story 2 - The same report for mediasoup (Priority: P2)

**Goal**: a mediasoup run's quality summary, report files and Prometheus series have exactly the fields of Jitsi's and LiveKit's (spec US2, FR-010 to FR-012).

**Independent Test**: quickstart steps 3 (field-set diff) and 4 (latency) on the reports of T016 and of a LiveKit run of the same plan.

- [X] T017 [P] [US2] Extend `src/report/writer_test.go` `TestWriteNoSecrets` with a `mediasoup` scenario whose `platform_config` has `unlimited_room_secret`. The secret is absent from both files, and `"[redacted]"` is present in the JSON. Use `catalog.New(nil).SecretKeys("mediasoup")` as the live tests do. Extend `tests/integration/api/report_api_test.go` only if it has a per-platform case; otherwise note "covered by T011" under this task.
  - **Recorded 2026-10-20**: `TestWriteNoSecrets` has a `mediasoup` scenario with `unlimited_room_secret`. The secret is absent from both files, and "[redacted]" is present. The keys are written out, as for the other platforms in that test, not read through the catalog. `report_api_test.go` has no per-platform case: covered by T011 (`TestMediasoupCreationRules`).
- [X] T018 [US2] Field-set check (SC-004, FR-010). Run quickstart step 3's `jq` diff between T016's 10-bot mediasoup report and a 10-bot LiveKit report of the same plan, taken the same day, with LiveKit started from `tests/platforms/livekit`. Record the diff, which must be empty apart from array indexes and `platform_config` keys. Run `curl -s localhost:9100/metrics` during a `MEDIASOUP_METRICS_ADDR=127.0.0.1:9100` run and check that the `webrtc_*` series and labels match a LiveKit run's (list the series names under this task).
  - **Recorded 2026-10-20**, 10 bots, 60 s on each platform, the other server stopped. Reports: `20261020T071500Z-mediasoup-live-mediasoup-9a29d77a` and `20261020T071627Z-livekit-live-livekit-7a805a10`. No `jq` on this machine, so the diff was done with Python over the leaf paths:
    - both reports have **93 fields**;
    - the differences are the `platform_config` keys (LiveKit's `api_key` and `api_secret`), and video latency: mediasoup reports `latency_seconds.{p50,p95,p99}`, LiveKit `latency_reason` instead. FR-011 provides for this (a value or its reason), and it matches 008's finding that LiveKit rewrites video timestamps;
    - the `/metrics` series and label sets taken during each run are **identical** (47 `webrtc_*` series): `webrtc_engine_*`, `webrtc_media_*` (audio MOS, bytes, frame height and rate, freezes, jitter, keyframe requests, latency, packets, remote streams, resolution switches, round trip, send lateness), `webrtc_peer_*` (negotiation, signaling and transport connect), `webrtc_peers`, `webrtc_server_*`, `webrtc_transport_bytes_received_total`.
- [X] T019 [US2] Latency (FR-011, research R8, quickstart step 4). In T016's 10-bot summary, record whether `audio.latency` and `video.latency` are present. If one is missing:
  - check that its `latency_reason` is set and the value is not 0;
  - capture 2 s of RTP on a bot (debug log of the first received timestamps, as 008 T025 did) to say whether mediasoup rewrote the timestamps;
  - update research R8 with the finding.
  - **Recorded 2026-10-20**: in the 10-bot summary, `audio.latency` and `video.latency` are both present: p50 0.5 ms and 2.5 ms (report `…838d3854`). mediasoup keeps the senders' RTP timestamps, so research R8's expectation holds; no capture was needed. Receive-only bots whose senders run in another process get `latency_reason: not_measured`, as designed (no send times).
- [X] T020 [US2] Server monitoring (FR-012). In T016's 10-bot report, the server section lists `mediasoup-demo` as `measured`, with CPU and memory. Check against `docker top mediasoup-demo` during a run that the container holds the node process and the 2 `mediasoup-worker` processes, so the figure covers all of them. Record the CPU average and peak.

  - **Recorded 2026-10-20**: the 10-bot report lists `mediasoup-demo` as measured: 0.21 threads average, peak 0.23, memory peak 53.2 MiB. `docker top` during a 10-bot run shows `node lib/index.js` (2.5% CPU) and **two** `mediasoup-worker` processes, one at 15.3% and one at 0.0%. The container figure covers all of them, and one room uses one worker (research R9).
**Checkpoint**: the three platforms' reports read side by side.

---

## Phase 5: User Story 3 - A local mediasoup-demo server (Priority: P3)

**Goal**: a pinned setup that starts with one command and reproduces results, including the 100-bot comparison (spec US3, FR-013, FR-014).

**Independent Test**: quickstart steps 1 and 3 from a clean checkout, in under 15 minutes (SC-006).

- [X] T021 [US3] Complete `tests/platforms/mediasoup/README.md`:
  - **Start**: `cp env.example .env`, then `docker compose up -d --build`, and the check of T001;
  - **Use it in a test plan**: JSON with `"platform": "mediasoup"`, the contract's `platform_config` example, and `server_monitoring.containers: ["mediasoup-demo"]`;
  - **Live test**: the T015 command;
  - **Workers**: 2 by default, a room uses one worker unless `use_pipe_transports` is set (research R9), and changing `MEDIASOUP_NUM_WORKERS` changes the server's CPU;
  - **Room time limit**: keep `UNLIMITED_ROOM_SECRET` unset;
  - **Pinned version**: the commit, mediasoup's version from T001, and how to bump it (build argument);
  - **Seeing the room in a browser**: the demo's `app` is not built here. Run it from the same commit with `npm ci && npm start` in `app`, pointed at `localhost:4443`, and check the exact steps;
  - **Stop**: `docker compose down`;
  - **If the bots cannot connect**: `MEDIASOUP_ANNOUNCED_ADDRESS`.
  - **Recorded 2026-10-20**: README written. Added beyond the list:
    - an "Origin header" section;
    - optional TLS for the web app: `MEDIASOUP_TLS_CERT` and `MEDIASOUP_TLS_KEY` in `config.mjs` and `.env`, `certs/` mounted and git-ignored. The demo's app always uses `wss://<page host>:4443`. The engine side was checked against a self-signed certificate: refused without `insecure_tls`, PASS with it (`MEDIASOUP_URL=wss://localhost:4443 MEDIASOUP_INSECURE_TLS=1`, 2 bots, default Origin `https://localhost:4443`). The app steps come from its sources (`urlFactory.js`, `vite.config.ts`, port 5555) and were not run: no browser here;
    - a "Transports closed after about 20 s" note (research R14).
- [X] T022 [US3] Clean-checkout run (SC-006). In a fresh `git worktree` of the branch in the scratchpad, remove the image (`docker rmi webrtc-load-engine/mediasoup-demo:899109e`) and follow only the README up to the end of quickstart step 3. Record the time and anything the README missed, and fix the README.
  - **Recorded 2026-10-20**: nothing is committed yet, so the "checkout" is a copy of the tracked and unignored files (no `reports/`, `.env` or `certs/`) in the scratchpad, after `docker compose down` and `docker rmi` of the image. Followed only the README:
    - `cp env.example .env && docker compose up -d --build`: Server started with 2 workers. A `--no-cache` rebuild plus start takes **22 s**; `node:22-trixie` was already pulled, and a new machine also downloads it (~1 GB);
    - the README's 10-bot live test: PASS, 9 + 9 streams per bot, 0 freezes;
    - **121 s** from the first command to the end of the test (SC-006: under 15 minutes).

    **README gap found and fixed**: with no `certs/` folder, Docker creates it owned by root for the bind mount, so the README's later `openssl … -keyout certs/key.pem` would fail. `certs/.gitkeep` is now committed: `.gitignore` has `certs/*` and `!certs/.gitkeep`, and the README step no longer runs `mkdir`.
- [X] T023 [US3] 100 bots, alternated with LiveKit (SC-002, research R13, quickstart step 7). Build both live test binaries once. Run LiveKit, mediasoup, LiveKit, mediasoup:
  - 009's T027 plan (100 bots, `audio_video`, cap 10, ramp-up 20 s, 10 min hold);
  - the server under test restarted before each run, the other one stopped;
  - the same screen state for all four runs; record which.

  Record a table under this task, per run:
  - engine cores (`/usr/bin/time`) and the `load_machine` average;
  - server CPU average and peak;
  - memory peak;
  - MOS p50, freezes, establishment time, and the report file names.

  If the mediasoup server's average is close to 1 thread per worker in use, repeat both mediasoup runs with `MEDIASOUP_PIPE=1` and add them to the table.
  - **Recorded 2026-10-20**: 009's T027 plan, alternated LiveKit, mediasoup, LiveKit, mediasoup, from 09:23 to 09:48. The screensaver came on during run 1 and stayed on (the same state for runs 2–4; run 1 started with the desktop on).

    | Run | Platform | Result | Engine (`/usr/bin/time`) | Server avg / peak | MOS p50 | Freezes | Report |
    |---|---|---|---|---|---|---|---|
    | 1 | LiveKit | PASS, 100 established in 21.4 s | 3.24 cores | 4.90 / 5.51 threads, 1.07 GiB | 4.37 | 0 | `…073330Z-livekit-live-livekit-e93eaf18` |
    | 2 | mediasoup | **FAIL at 26 s**: 1 bot lost, 97 established | – | – | – | – | `…073451Z-mediasoup-live-mediasoup-e3c36282` |
    | 3 | LiveKit | PASS, 100 established in 20.4 s | 3.19 cores | 4.87 / 5.53 threads, 1.07 GiB | 4.37 | 0 | `…074613Z-livekit-live-livekit-2e23e19e` |
    | 4 | mediasoup | **FAIL at 28 s**: 1 bot lost, 98 established | – | – | – | 10 | `…074737Z-mediasoup-live-mediasoup-0e9b7ee0` |

    **Diagnosis** (debug runs of 100 bots, ramp-up 20 s):
    - The lost bots' transports expired 20 s after their last counted consent check. The bots sent a check every ~5 s (instrumented build), but mediasoup answered only 471 of 676 checks. mediasoup counts any valid check, so the unanswered ones never reached it.
    - `/proc/net/udp`: every drop is on mediasoup's port 44444, the one `WebRtcServer` socket that receives every bot of the room's worker: 3,000 at t+10 s, 119,000 at t+20 s, 420,000 at t+26 s. None on the bots' sockets.
    - The room's worker was at 20%, then 73%, then 90% of a core over the same times; the second worker was idle (one room = one worker, research R9). A transport is lost only when four checks in a row are dropped, hence about one bot per run.
    - **With a 4 MiB socket buffer** (`MEDIASOUP_SOCKET_BUFFER`, now in the setup): no drops on 44444. But the worker stays at ~90%, 2 bots were lost at 56 s, MOS 2.87, 53% audio concealed (p95), 22.7 fps, 7,792 freezes.
    - **With the buffer and `use_pipe_transports`**: all 100 bots held their transport for 60 s, 10 + 10 streams offered per bot, 0 video freezes, 29.7 fps. But MOS was 2.99 and 14% of audio concealed (p95), and this time the *bots'* sockets dropped 140,000 datagrams in 40 s.

    **SC-002 is not met for mediasoup-demo**: at 100 bots, cap 10, the demo's room cannot carry this load on this laptop. The default mode saturates its one worker; pipe mode still degrades audio, with drops on the engine side that this task did not resolve. The engine's CPU could not be compared, since no mediasoup run completed its 10 minutes. LiveKit's same-state runs are the reference above. Follow-ups are listed in validation-report.md.
- [X] T024 [US3] Room time limit (quickstart step 8, research R9). Set `UNLIMITED_ROOM_SECRET=s` in `.env` and restart. Then:
  - a 2-bot run with an 11-minute hold and no secret: record when and how the bots are reported lost (state, reason in the log);
  - the same with `MEDIASOUP_UNLIMITED_ROOM_SECRET=s` in another room: it completes.

  Unset the variable afterwards and restart.

  - **Recorded 2026-10-20**, server with `UNLIMITED_ROOM_SECRET=s`, two 2-bot runs at once with an 11-minute hold:
    - **without the secret** (room `t024-limited`): the server logged "closing time limited room" at 08:14:31Z, 10 min after the room was created. Both bots got `signaling_lost` ("closed by protoo-server", StatusCode 4000) at the same instant, and the test failed with "0/2 bots still established after 10m1s". The loss is reported, never a silent stop;
    - **with `MEDIASOUP_UNLIMITED_ROOM_SECRET=s`** (room `t024-unlimited`): PASS, both bots held their transport for 11 min.

    The variable was removed from `.env` afterwards and the server recreated.
**Checkpoint**: results are reproducible, and the three-platform comparison has its first numbers.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T025 [P] Update the repository's `README.md`, in the style of the 008 entries:
  - the features table gets a row "010 mediasoup-demo";
  - the "next" list no longer says mediasoup is next;
  - a "**mediasoup-demo (010).**" paragraph: platform, `platform_config`, rejected options, the local setup command and the live test command.

  Update `CLAUDE.md`: the "Commands" block gets the mediasoup live test command, and the "Recent Changes" 010 line drops "(planned)".
- [X] T026 Write `specs/010-mediasoup-platform/validation-report.md`, as 008's and 009's:
  - the setup (machine, server commit and mediasoup version, workers, screen state);
  - a table of SC-001 to SC-006 with result and evidence (T016, T018–T020, T022, T023);
  - the latency finding (T019), the 100-bot table (T023), the time-limit check (T024);
  - deviations from the plan.
- [X] T027 Final checks:
  - `go vet ./... && go vet -tags live ./tests/live/ && go test -race -count=1 ./...` all pass;
  - `grep -rn "mediasoup\|protoo" src --include=*.go | grep -v "src/platform/mediasoup\|src/platform/catalog"` prints nothing (boundary, constitution III);
  - `git diff main -- src/execution src/media src/scenario src/report` shows no change made by this feature, except the two platform-neutral `src/execution` changes found in T016 (consent checks, receive-only publish session; plan.md deviation);
  - `go.mod` has no new requirement;
  - one 2-bot Jitsi live run and one 10-bot LiveKit live run still pass (FR-009).

  Record the results under this task.

  - **Recorded 2026-10-20**:
    - `go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race -count=1 ./...` all pass. `gofmt -l` lists 4 files that were already unformatted at HEAD (`src/main.go`, `src/api/handlers/scenario_handler.go`, two integration tests), not touched here;
    - **boundary**: no mediasoup or protoo name in `src` outside `src/platform/mediasoup` and `src/platform/catalog`, except `src/report/writer_test.go`, which uses `"mediasoup"` as test data, as it does for jitsi and livekit. The consent code's comments were made platform-neutral;
    - **shared packages**: `src/execution/peer.go` (no publish session for a receive-only peer), `pion.go` and the new `consent.go` (consent checks), plus their tests and `src/report/writer_test.go` (T017). `src/media` and `src/scenario` are unchanged;
    - **go.mod**: `pion/ice/v2 v2.3.38` and `pion/stun v0.6.1` move from indirect to direct; no new module;
    - **FR-009**: Jitsi 2-bot `audio_video` live run PASS (established in 2.2 s, 0 freezes); LiveKit 10-bot live run PASS (9 + 9 streams per bot, 0 freezes); a receive-only LiveKit bot alone is now established (202 ms), confirming the 008 fix.
---

## Dependencies & Execution Order

- **Setup (T001–T002)**: first; T002 needs T001's server and produces the goldens of T004, T006, T007 and T008. Its findings may change research R1/R4/R6 and the contracts before Phase 2.
- **Foundational (T003–T008)**:
  - T003–T007 are independent files and can run in parallel; T006 and T007 both edit `sdp.go`, so write them in one session or one after the other;
  - T008 needs T004, T005 and T007, since it decodes the bot's parameters.
- **US1 (T009–T016)**:
  - tests T009–T011 can run in parallel;
  - T012 needs T009; T013 needs T010 and T012; T014 needs T011 and T013's `NewAdapter`;
  - T015 needs T014; T016 needs T015 and T001.
- **US2 (T017–T020)**: T017 needs T014; T018–T020 need T016's runs (and a LiveKit server for T018).
- **US3 (T021–T024)**: T021 can start after T016. T022 needs T021. T023 needs T016 and LiveKit. T024 needs T015.
- **Polish (T025–T027)**: last; T026 needs every recorded task.

### Parallel Example: Foundational

```text
T003 config.go   T004 messages.go   T005 protoo.go   T006+T007 sdp.go
then
T008 fakeserver_test.go
```

### Parallel Example: User Story 1

```text
T009 consumers_test.go   T010 adapter_test.go   T011 catalog_test.go + platform_api_test.go
then
T012 consumers.go → T013 adapter.go → T014 catalog.go → T015 live test → T016 live check
```

## Implementation Strategy

1. **MVP**: Setup, Foundational, then US1 (T001–T016). mediasoup runs from the API, and T016 proves it on the real server.
2. **US2** (T017–T020): the comparable report, with latency and server load checked on the real server.
3. **US3** (T021–T024): reproducibility from a clean checkout, and the 100-bot comparison with LiveKit.
4. **Polish** (T025–T027).

The engine is not changed, so Jitsi and LiveKit cannot regress through
shared code. The only shared file is `catalog.go` (T014). Still run T027's
Jitsi and LiveKit checks before calling the feature done.
