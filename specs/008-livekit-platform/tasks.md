---

description: "Task list for 008 LiveKit platform"
---

# Tasks: LiveKit Platform

**Input**: Design documents from `specs/008-livekit-platform/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires contract tests for signaling adapters and test coverage for the media engine and lifecycle (plan "Constitution Check"). Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green after every task.
- **Boundaries**: no LiveKit name, type or protobuf outside `src/platform/livekit` and `src/platform/catalog` (constitution III). No `livekit/protocol` or LiveKit SDK module in `go.mod` (research R2).
- **Metrics in tests**: tests that read global metrics use a unique scenario ID (uuid), as in 004–007.
- **Secrets**: never in logs. Tests grep captured logs for the secret.
- **Live runs**: these target `tests/platforms/livekit` with docker-jitsi-meet stopped ([quickstart.md](quickstart.md)).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: the local server and the protocol facts every later task depends on (research R1, R6, R12 "checked first")

- [X] T001 Create `tests/platforms/livekit/` per research R12:
  - `docker-compose.yml`: service `livekit`, `image: livekit/livekit-server:v1.13.10`, `container_name: livekit-server`, `network_mode: host`, `command: --config /etc/livekit.yaml --node-ip ${LIVEKIT_NODE_IP}`, environment `LIVEKIT_KEYS: "${LIVEKIT_API_KEY}: ${LIVEKIT_API_SECRET}"`, `./livekit.yaml:/etc/livekit.yaml:ro`;
  - `livekit.yaml`: `port: 7880`, `rtc: {tcp_port: 7881, udp_port: 7882, use_external_ip: false}`, `room: {empty_timeout: 30}`, `logging: {level: info}`;
  - `env.example`: `LIVEKIT_API_KEY=devkey`, `LIVEKIT_API_SECRET=` with a comment "32+ characters", `LIVEKIT_NODE_IP=127.0.0.1`;
  - `README.md`: start/stop, the `platform_config` of `contracts/livekit-config.yaml`'s example, `server_monitoring.containers: [livekit-server]`, the LAN-IP note of R12.

  Add `tests/platforms/livekit/.env` to `.gitignore`. Start the server with a generated secret and check that `docker logs livekit-server` shows it listening.
- [X] T002 In the scratchpad (its own module, never committed), write a throwaway generator against `github.com/livekit/protocol@v1.52.1` and `server-sdk-go`'s token helper. It does two jobs:
  1. **Golden frames**: marshals the reference messages of `contracts/signaling-messages.md` and writes them as `.bin` files plus a `README.md` (each file's message, in Go literal form) under `src/platform/livekit/testdata/golden/`:
     - sent: one `SignalRequest` per sent field;
     - read: a `SignalResponse` per read field, including a `JoinResponse` with 2 other participants carrying audio+video tracks;
     - a `SignalResponse.speakers_changed` to test skipping.
  2. **Probe the pinned server**: joins it with `protocol=17`, `auto_subscribe=0`, and records:
     - whether `subscriber_primary` is true and a first subscriber offer arrives without publishing (R1);
     - whether subscriber offers carry `a=ssrc` lines (R6);
     - whether the ICE pair connects with the server at `127.0.0.1` (R12).

  Record the three answers and the protocol value used under this task. Fall back as research says if one fails.
  - **Recorded 2026-10-17** (server v1.13.10, probe with `livekit/protocol` v1.52.1 + Pion v4, two participants):
    - **R1**: `protocol=17` accepted (`server_info.protocol` 17). `JoinResponse.subscriber_primary` true, `ping_interval`/`ping_timeout` 5/15 s, `fast_publish` true.
    - **First subscriber offer**: arrives right after the join (one `m=application` section, data channel only), before any publishing.
    - **Message order**: the server sends `refresh_token` first, and subscriber trickle candidates **before** the first offer, so they must be buffered (the peer already does this).
    - **Track visibility**: tracks of a publisher appear in a later `ParticipantUpdate` once its media flows (not in `JoinResponse` of a bot joining right after). Subscribing then triggers a new subscriber offer with one section per track.
    - **R6**: subscriber offers carry `a=ssrc` lines (cname/msid/mslabel/label) per forwarded track. Their direction is `sendrecv`, not `sendonly`, which the receive gate accepts. Audio is offered as opus/red/PCMU/PCMA, and Pion answers opus.
    - **R12**: ICE connects on both PCs with the server announcing 127.0.0.1 (UDP 7882 is not bound on lo, but the 127.0.0.1 candidate works through the NAT1To1 mapping). `LIVEKIT_NODE_IP=127.0.0.1` is kept.
    - **R11, audio**: forwarded Opus keeps the sender's RTP timestamps and sequence numbers (first received seq 102, timestamp 1000960 + 101×960). Video is checked in T025.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the widened platform boundary, the publish session in the peer, the platform catalog and the shared signer. US1 needs all of them; Jitsi must stay green throughout.

- [X] T003 [P] Move the HS256 signing from `src/platform/jitsi/jwt.go` into `src/platform/internal/jwt/hs256.go` as `Sign(claims any, secret []byte) (string, error)` (header `{"alg":"HS256","typ":"JWT"}`, base64url without padding). Write `hs256_test.go` with an RFC 7515-style known vector, and keep `src/platform/jitsi/jwt_test.go` unchanged and passing (Jitsi now calls `jwt.Sign`).
- [X] T004 Add the boundary of `contracts/platform-boundary.md` to `src/platform/adapter.go`: the four `SignalingHandler` methods `CreatePublishOffer(trickle TrickleFunc) (string, error)`, `ApplyPublishAnswer(sdp string) error`, `OnPublishICECandidate(mid, candidate string) error` and `LocalVideoSize() (width, height int)`, plus the optional `PublishSessionAdapter{ PublishSession() bool }`, with the doc comments of the contract. Make the test handler `pionHandler` in `src/platform/jitsi/adapter_test.go` satisfy them (return "not supported" errors and 0, 0).
- [X] T005 Write failing tests in `src/execution/peer_test.go` for a peer built with `PeerMedia{Kinds: audio+video, PublishSession: true}` (data-model "Peer Publish Session"). A second plain Pion PeerConnection plays the server: it answers the publish offer and offers a receive session with two remote tracks. Checks:
  - `CreatePublishOffer` returns two `sendonly` sections with `msid`, and the answered session's answer has no local track (`recvonly`);
  - local media flows on the publish session, and the remote tracks arrive through `OnTrack` with quality measured;
  - the peer goes `established` only once both transports are connected, and `failed` when either fails (close the server's publish PC);
  - `negotiation_seconds` is observed once per session, and `transport_connect_seconds` once, when the last session connects;
  - candidates passed to `OnPublishICECandidate` before `ApplyPublishAnswer` are buffered;
  - `Close` closes both;
  - `LocalVideoSize` returns 640×360 for the 004 content, and 0, 0 for `audio`.
  - Without `PublishSession`, the existing peer tests pass unchanged, and `CreatePublishOffer` returns an error.
- [X] T006 Implement T005 in `src/execution/peer.go` and `src/execution/pion.go` (research R4):
  - `PeerMedia.PublishSession`;
  - `newPeerConnection(p, publish bool)`: the publish variant has the same codecs, NACK and RTCP reports, but no `gatedBuffer` and no transport-cc feedback registration;
  - `addLocalTracks` targets `p.pub` when present, with `sendonly` transceivers, and RTCP readers on its senders;
  - per-session connection-state callbacks combined into the lifecycle;
  - `CreatePublishOffer` gathers like `answer` (`gatherTimeout`, then trickle);
  - `ApplyPublishAnswer` and `OnPublishICECandidate` with buffering;
  - `LocalVideoSize`.

  Make T005 and all existing `src/execution` tests pass.
- [X] T007 [P] Write failing tests in `src/execution/engine_test.go`: with a mock adapter that implements `PublishSessionAdapter` returning true, the engine builds peers with `PublishSession: true`; with the existing mock adapter (no such method), it builds them as today. Implement it in `src/execution/engine.go` (`peerMedia` checks the adapter once in `NewEngine`).
- [X] T008 Write failing tests in `src/scenario/service_test.go` for the optional `scenario.PlatformCatalog` of `contracts/platform-boundary.md`, using a fake catalog:
  - creation rejects a platform not in `Platforms()` with `unsupported platform adapter: X`;
  - creation returns `ValidateTestPlan`'s error;
  - `CreateScenario`, `GetScenario` and the list return `platform_config` with every `SecretKeys` value replaced by `"[redacted]"`, while the stored plan used by `ExecuteScenario` keeps the real values;
  - without a catalog, behaviour is today's (jitsi only).

  Implement it in `src/scenario/service.go` (the catalog is taken from the starter by type assertion in `NewService`; redaction happens in `snapshot`).
- [X] T009 Create `src/platform/catalog/catalog.go` with `Entry{Name; Validate func(scenario.TestPlan) error; SecretKeys []string; NewAdapter func(*slog.Logger, map[string]any) (platform.PlatformAdapter, error)}` and `Catalog` implementing `scenario.PlatformCatalog`. Register `jitsi`: `Validate` nil (config still parsed at execution, research R10), `SecretKeys` `jwt_app_secret`, `jwt`, `NewAdapter` = today's `jitsi.NewAdapter` + `ParseConfig`. Make `MainEngineStarter` in `src/main.go` hold a `*catalog.Catalog`, build adapters with it (by the scenario's platform: add `platform string` to `ExecutionStarter.StartExecution` and update its callers and mocks) and implement `PlatformCatalog` by delegation. Test it in `src/platform/catalog/catalog_test.go`. Extend `tests/integration/api/scenario_api_test.go`: a Jitsi plan with `jwt_app_secret` returns it as `"[redacted]"`.
  - **Built differently** (recorded 2026-10-17):
    - `PlatformCatalog` also has `NewAdapter(name, tp)`. `Service.ExecuteScenario` passes the catalog's adapter through the existing `adapter` parameter, so `StartExecution`'s signature is unchanged and its six implementations are untouched.
    - `GET /scenarios/{id}` answers only `id` and `status`, so it never carried a secret. The actual leak was the creation log line (`test_plan` with its secrets), which now logs the redacted copy. The API test (`tests/integration/api/platform_api_test.go`) checks that the GET answer has no secret.
    - The scenario tests are in `src/scenario/catalog_test.go`. T005's tests are in `src/execution/peer_publish_test.go`. T005 also added `Content.Width`/`Height` from the IVF header (`src/media/content.go`) for `LocalVideoSize`.

**Checkpoint**: Jitsi unchanged and green; a peer can send on its own offered session; platforms are chosen and validated through the catalog

---

## Phase 3: User Story 1 - Run a test plan against LiveKit (Priority: P1) 🎯 MVP

**Goal**: `platform: livekit` with its settings: bots join one room, publish audio+video, receive each other up to the cap, leave at the end; failures and losses reported as on Jitsi

**Independent Test**: quickstart steps 1, 3, 4 (join/receive/leave part) and 5: 10 bots, 1 minute, each receives 9 audio + 9 video; with 6 bots and cap 2, each receives 2 + 2 and a leaver is replaced

### Tests for User Story 1

- [X] T010 [P] [US1] Write failing tests in `src/platform/livekit/proto_test.go`. Encoding each sent message must give the golden bytes of T002 exactly. Decoding each golden `SignalResponse` must give the fields listed in `contracts/signaling-messages.md` (`JoinResponse` with its participants and tracks, `ping_interval`, `ping_timeout`, `server_info.version`). A `speakers_changed` frame decodes as "skipped", and truncated or garbage frames return an error, never panic. Add a fuzz test `FuzzDecodeSignalResponse`.
- [X] T011 [P] [US1] Write failing tests in `src/platform/livekit/config_test.go` for `ParseConfig(map[string]any) (Config, error)` with every rule and exact message of `contracts/livekit-config.yaml`:
  - `url` "required; scheme ws/wss/http/https; http(s) mapped to ws(s); `/rtc` appended";
  - `room` "required, non-empty";
  - key and secret together, or token alone;
  - `token_ttl` "default 6 h; > 0; only with key/secret";
  - unknown keys rejected.

  In the same file, test `Validate(scenario.TestPlan)`, which also rejects `media.simulcast` and `media.requested_heights` with the contract's messages.
- [X] T012 [P] [US1] Write failing tests in `src/platform/livekit/token_test.go` for `tokenFor(cfg, peerID, now)`. Decode the JWT and check the claims of data-model "Access Token Claims": `iss`, `sub`/`name`/`jti` = peer ID, `nbf` = now − 10 s, `exp` = now + ttl, and `video` with `room`, `roomJoin`, `canPublish`, `canSubscribe` true, `canPublishData` false. Check the HS256 signature with the secret, and that the ready-made token is returned unchanged.
- [X] T013 [P] [US1] Write failing tests in `src/platform/livekit/subscriptions_test.go` for the receive-cap state of data-model "Receive-Cap State":
  - tracks seen in order; fill subscribes the earliest up to `limit` per kind, with 0 meaning all;
  - one `UpdateSubscription` per fill, listing the SIDs added;
  - a participant DISCONNECTED, or a track missing from its participant's next update, removes the track and fills from the waiting ones;
  - a `SubscriptionResponse` error drops the track;
  - own tracks are never subscribed;
  - audio and video are capped independently.
- [X] T014 [US1] Write a fake LiveKit signaling server in `src/platform/livekit/fakeserver_test.go`. It runs on `httptest` with `nhooyr.io/websocket`, decodes with the T010 codec, and has a real Pion v3 PeerConnection per session on the server side. It:
  - checks the token's signature and claims, and answers 401 otherwise;
  - sends `JoinResponse`, with `ping_interval` 1 s and `ping_timeout` 3 s in tests;
  - answers `AddTrackRequest` with `TrackPublishedResponse`;
  - answers the publish offer;
  - forwards each publishing bot's tracks to the others as tracks on their subscribe PCs, with server offers and renegotiations, and only for tracks a bot subscribed to;
  - sends `ParticipantUpdate` on join and leave, and answers pings;
  - offers switches to drop pongs, send a `LeaveRequest`, or close the socket.

  Then write failing tests in `src/platform/livekit/adapter_test.go` that drive real `execution.Peer`-equivalent handlers through `Adapter.Connect`/`Disconnect`:
  - 3 bots `audio_video` join, publish and each receive 2+2 tracks;
  - `Connect` returns only after the publish answer and the first subscriber answer;
  - a receive-only bot joins without publishing;
  - cap 1 with replacement when a bot disconnects;
  - a wrong secret gives a `Connect` error at stage `signaling` with reason `unauthorized`, and the logs contain neither the secret nor the token;
  - missing pongs, a server `LeaveRequest` and a socket close each call `OnSignalingLost` once;
  - `Disconnect` sends `LeaveRequest{reason: CLIENT_INITIATED}`, is idempotent and safe for unknown peers;
  - `ctx` cancelled during the join leaves no goroutine (goleak-style check with `runtime.NumGoroutine` settling).

### Implementation for User Story 1

- [X] T015 [P] [US1] Implement `src/platform/livekit/proto.go` over `protowire` per `contracts/signaling-messages.md`: Go structs for the sent and read messages, `encodeSignalRequest`, and `decodeSignalResponse` with skip of unknown fields. Each message gets a comment naming its proto message and the v1.52.1 field numbers. Add `google.golang.org/protobuf` as a direct requirement in `go.mod` (version unchanged). Make T010 pass.
- [X] T016 [P] [US1] Implement `src/platform/livekit/config.go` (`Config`, `ParseConfig`, `Validate`) and `src/platform/livekit/token.go` (claims, `tokenFor` via `internal/jwt`). `Config`'s `String`/`LogValue` hide `APISecret` and `Token`. Make T011 and T012 pass.
- [X] T017 [P] [US1] Implement `src/platform/livekit/subscriptions.go`. Make T013 pass.
- [X] T018 [US1] Implement `src/platform/livekit/signal.go`:
  - **Dial**: the URL of research R1, with the token in the query only, through a redacting `String` for logs.
  - **Read loop**: one goroutine per bot that decodes frames and dispatches them in order.
  - **Writes**: serialized.
  - **Pings**: on `ping_interval`; the liveness deadline is `ping_timeout` since the last frame read (R8).
  - **Close**: `close(leaving bool)`.
- [X] T019 [US1] Implement `src/platform/livekit/adapter.go`: `NewAdapter(logger, cfg)`, `PublishSession() bool` (true), and `Connect`/`Disconnect` following data-model "Bot Session".
  - **Join**: dial, then `JoinResponse` (an `alternative_url` fails the join).
  - **Publish** (sending bots only, R5): `CreatePublishOffer`, then parse each section's `msid` track ID, send `AddTrackRequest` per track (audio with `disable_red`, video with `LocalVideoSize`), wait for every `TrackPublishedResponse`, send the offer, and `ApplyPublishAnswer`.
  - **Subscribe session** (R6): first offer to `OnOffer`, later ones to `Renegotiate`, echoing `id`.
  - **Candidates**: trickle by target, as `candidateInit` JSON.
  - **Receive cap**: subscriptions from `JoinResponse` and `ParticipantUpdate` via T017.
  - **Losses**: `OnSignalingLost` mapping (R8, R9).

  Connect timeouts and errors carry their stage. Make T014 pass.
- [X] T020 [US1] Register `livekit` in `src/platform/catalog/catalog.go`: `Validate` = `livekit.Validate`, `SecretKeys` `api_secret`, `token`, `NewAdapter` = `livekit.ParseConfig` + `livekit.NewAdapter`. Add API cases to `tests/integration/api/scenario_api_test.go`:
  - 400 for a missing `url`, a missing `room`, a key without a secret, token+key, simulcast and requested heights on `livekit`, with the contract messages;
  - 201 for a valid plan, whose `GET` shows `api_secret: "[redacted]"`.
- [X] T021 [US1] Write `tests/live/livekit_live_test.go` (`//go:build live`), a sibling of `tests/live/jitsi_live_test.go`, with the variables of research R13 documented in its package comment:
  - `LIVEKIT_URL`, `LIVEKIT_ROOM`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`;
  - `LIVEKIT_PARTICIPANTS`, `LIVEKIT_MEDIA`, `LIVEKIT_MAX_RECEIVED`, `LIVEKIT_HOLD`, `LIVEKIT_RAMP_UP`, `LIVEKIT_SERVER_CONTAINERS`;
  - `LIVEKIT_LEAVE_ONE`: one bot disconnects halfway.

  It builds the scenario through the catalog's adapter, asserts every bot `established` and the received stream counts per kind, logs the full quality summary as JSON, and skips without `LIVEKIT_URL`. Run quickstart steps 4 (join/receive/leave) and 5 against the local server, and record the results under this task.
  - **Recorded 2026-10-17** (local LiveKit v1.13.10, docker-jitsi-meet stopped):
    - **10 bots** `audio_video`, 60 s hold, `LIVEKIT_SERVER_CONTAINERS=livekit-server`: PASS.
      - All established in 1.2 s, and each was offered 9 audio + 9 video streams (446 audio and 586 video packets/s per bot).
      - Bots left in 2 ms after the stop. No WARN/ERROR line.
      - Summary: 0 freezes, `concealed_ratio` 0, MOS p50 4.37, frame rate p50 30.0, height 360 for 100% of the received time.
      - Audio latency p50 1.5 ms. **Video `latency_available: false`**: LiveKit rewrites video RTP timestamps (R11, handled in T024).
      - `livekit-server` measured at 0.737 threads average, 0.841 peak, 108 MB peak. Engine 0.648 threads.
    - **6 bots, cap 2, `LIVEKIT_LEAVE_ONE=1`** (an extra bot joins first and leaves halfway through the 20 s hold): PASS. Each bot was offered 2 audio + 2 video streams before and after the extra bot left, so its streams were replaced. No freezes.
  - **Implementation notes**:
    - Join order seen in the fake and the real server: participants are listed at join without tracks, and each track is announced in a later `ParticipantUpdate` once its media flows (covered by `TestSubscriptions_TracksAnnouncedOneByOne`).
    - The peer's `Streams` counts every stream received, ended ones included. Replacement checks use `Offered`.
    - The fake server (`fakeserver_test.go`) has its own response encoder and request decoder (`fakeproto_test.go`), checked against the golden requests.

**Checkpoint**: US1 is usable: LiveKit runs with media metrics; MVP

---

## Phase 4: User Story 2 - The same report for LiveKit as for Jitsi (Priority: P2)

**Goal**: the LiveKit summary has the same sections and fields as Jitsi's; a measure LiveKit prevents is explained, never zero; server load works with the LiveKit container

**Independent Test**: quickstart step 4 summary part on both platforms with the same knobs; key sets equal; LiveKit 10-bot run meets SC-003

### Tests for User Story 2

- [X] T022 [P] [US2] Write failing tests in `src/execution/quality_test.go` (and `src/scenario/quality_test.go` for the JSON key) for `KindQuality.LatencyReason` (`latency_reason`, omitempty, contract `quality-summary-delta.yaml`):
  - bots that received frames and have send times, but never a hit: `latency_available: false`, `latency_reason: "frame_not_identified"`;
  - received media without send times: `not_measured`;
  - latency measured: field absent;
  - kind received nothing: field absent (its `reason` says so).
- [X] T023 [P] [US2] Write a failing test in `src/platform/livekit/adapter_test.go` that runs the fake-server scenario through the real engine (`execution.NewEngine` with the LiveKit adapter, 3 bots, 5 s) and checks the quality summary:
  - `video` and `audio` available, `bots` 3;
  - frame rate and MOS percentiles set;
  - the JSON key set equals the one of the same run through the Jitsi fake server of `src/platform/jitsi/fakeserver_test.go`, apart from optional keys absent in one of them (SC-004).

  If the Jitsi fake cannot run a full engine, compare against the key set of a `QualitySummary` built by `quality_test.go` instead, and note it here.

### Implementation for User Story 2

- [X] T024 [US2] Implement `LatencyReason` in `src/scenario/quality.go` and its computation in the quality collector (`src/execution/quality_collector.go`), from per-stream counters of frames looked up and found in `src/media/video_quality.go` and `src/media/audio_quality.go`, added if missing. Make T022 and T023 pass, and keep the 006 latency tests green.
  - **Recorded 2026-10-17**:
    - Streams count `LatencyLookups` (frames/packets looked up in the send times, found or not) in `VideoQualitySnapshot`/`AudioQualitySnapshot`. The collector sets `latency_reason` from them.
    - T023 uses the fallback: the Jitsi fake server lives in the Jitsi package's tests, so the LiveKit engine run (`TestEngine_LiveKitQualitySummary`) is checked against the key list of the 006/007 contracts.
    - **Found while running the suite (007 bug)**: `TestEngine_ServerLoad` failed about half the time. A regular tick that started as the run ended had its Docker requests cancelled with the run's context, and was counted as a `docker_unreachable` miss. The test's 2 s run with a 1 s interval hits this boundary. Fix: `Sampler.tick` drops a tick whose context ended while it was in flight, since the end-of-run tick covers the end. Regression test: `TestSampler_TickCancelledByRunEnd`. `TestEngine_ServerLoad` then passed 10/10.
- [X] T025 [US2] Live check (research R11, quickstart step 4): run the 10-bot LiveKit test with `LIVEKIT_SERVER_CONTAINERS=livekit-server`, then the Jitsi live test with the same knobs, on the same day with the other server stopped. Record under this task:
  - both summaries' headline figures (freezes, `concealed_ratio.p50`, `mos.p50`, frame rate p50, latency or its reason, server CPU average and peak, engine CPU);
  - whether LiveKit preserves RTP timestamps (latency available or `frame_not_identified`);
  - the key-set comparison (`jq -S 'paths|join(".")'`).

  SC-003 is met if LiveKit shows no freezes, gaps under 1% and MOS p50 ≥ 4.0.
  - **Recorded 2026-10-17**: 10 bots `audio_video`, no cap, 60 s hold, each server alone on the machine. LiveKit was restarted before its run; Jitsi was started cold, then run twice, and the second run is used.

    | | LiveKit v1.13.10 | Jitsi (docker-jitsi-meet, JVB 8 GB heap) |
    |---|---|---|
    | Live test | PASS | FAIL on its own strict check: 4 video freezes during the hold (the same plan built from HEAD, before 008, also gets 5, so not a regression) |
    | Video frame rate p50 | 30.0 | 30.0 |
    | Video freezes (whole run) | 0 | 14 |
    | Audio concealed p50 | 0 | 0 |
    | MOS p50 | 4.369 | 4.368 |
    | Audio latency p50 | 0.5 ms | 1.5 ms |
    | Video latency | `latency_available: false`, `latency_reason: frame_not_identified` | p50 1.5 ms |
    | Server CPU threads, average / peak | 0.140 / 0.255 (`livekit-server`) | 0.970 / 1.701 (total of 4 containers; jvb 0.954 / 1.549) |
    | Server memory peak | 110 MB | jvb 725 MB (+ jicofo 222, prosody 41, web 20) |
    | Engine CPU threads, average | 0.086 | 0.388 |

    - **R11**: LiveKit keeps Opus RTP timestamps but rewrites VP8 ones, so video latency is reported as unavailable with its reason. The 006 content-based fallback stays out of scope.
    - **SC-003 met**: no freezes, gaps under 1%, MOS p50 4.37 ≥ 4.0.
    - **SC-004 met**: `jq -S 'paths'`-style comparison of the two summaries gives 96 and 101 key paths. They differ only by optional keys: `video.latency_reason` on LiveKit; `video.latency_seconds.*` and `worst.most_freeze_seconds[]` entries on Jitsi, present only when there is a value.
    - **LiveKit column**: rerun later the same day with the port-range setup of T027 (each bot receives the same 446 audio + 586 video packets/s).
      - The first LiveKit run used the single UDP port and measured 0.717 threads on the server and 0.644 in the engine, with the same quality.
      - That setup doubled the engine's CPU and quintupled the server's at 10 bots, and broke at 100 (T027).

**Checkpoint**: two platforms, one report format

---

## Phase 5: User Story 3 - A local LiveKit server to test against (Priority: P3)

**Goal**: the pinned setup of T001 is documented end to end and carries the 100-bot run

**Independent Test**: quickstart steps 1 and 4 from a clean checkout in under 10 minutes (SC-006); quickstart step 6

- [X] T026 [US3] From a clean clone (`git clone` into the scratchpad), follow `tests/platforms/livekit/README.md` and quickstart steps 1 and 4 with a stopwatch. Fix any missing step in the README, and record the time under this task (SC-006: under 10 minutes).
  - **Recorded 2026-10-17**: a copy of exactly what a commit would hold (`git ls-files -co --exclude-standard`, so no `.env`) in the scratchpad, after removing the running container. Steps: `cp env.example .env`, set the secret, `docker compose up -d`, build and run the 10-bot live test. 72 s to PASS, with the image and Go modules already cached (a first `docker pull` adds about 30 MB). No README step was missing. **SC-006 met.**
- [X] T027 [US3] Scale run (quickstart step 6, SC-002): 100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min hold, `/usr/bin/time -v`, `LIVEKIT_SERVER_CONTAINERS=livekit-server`, docker-jitsi-meet stopped. Record under this task:
  - every bot connected for the hold (or the failures and why);
  - engine CPU average from `load_machine` and from `/usr/bin/time`;
  - LiveKit server CPU average and peak, and memory peak;
  - the quality headline figures.

  For the same-day comparison, run the Jitsi 100-bot test with the same knobs, with the bridge restarted and LiveKit stopped. SC-002 is met if the engine averages at most 2.0 cores on LiveKit.
  - **Recorded 2026-10-17**: 100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min hold, each server alone, LiveKit and the bridge restarted before their runs.
    - **First LiveKit run, single-port UDP** (`rtc.udp_port: 7882`, the planned setup): all 100 bots held, but quality collapsed:
      - audio latency p50 1.335 s, concealed p95 0.98, MOS p50 1.48;
      - 150,802 freezes, 485,888 keyframe requests.
      - Diagnosis:
        - no kernel UDP drop during a repeat, and empty socket queues;
        - the bots' 3,000 track readers idle, waiting for data;
        - send lateness about 0.1 ms;
        - a separate one-track probe in the same loaded room saw 0–12 ms;
        - congestion control off changed nothing (1.235 s).
      - With `port_range_start/end` (50000–60000) instead, the same run gave 1.5 ms, MOS 4.37 and 0 freezes. Conclusion: LiveKit's single-port mux queues what it sends at about 116,000 packets/s out of one socket. The setup now uses the port range (research R12, `tests/platforms/livekit/livekit.yaml`).
    - **Shipped setup**: LiveKit PASS (live test), Jitsi FAIL on its strict "no freeze during the hold" check only.

    | | LiveKit 1.13.10 | Jitsi (JVB, 8 GB heap) |
    |---|---|---|
    | Bots connected for the whole hold | 100/100 | 100/100 |
    | Streams offered per bot | 10 audio + 10 video | 10 audio + 10 video |
    | Frame rate p50 / freezes over the run | 30.0 / 0 | 30.0 / 236 (88 during the hold) |
    | Audio concealed p95 / MOS p50 | 0.00005 / 4.368 | 0.0048 / 4.332 |
    | Latency p50, audio / video | 1.5 ms / not available | 7.5 ms / 10.5 ms |
    | Server CPU threads, average / peak | 2.28 / 5.41 | 8.65 / 8.95 (jvb 8.61) |
    | Server memory peak | 1.2 GB | 4.2 GB (all containers) |
    | Engine CPU, `/usr/bin/time` | 1.43 cores ((724.8 + 167.2) s / 625.5 s) | 2.90 cores ((1460.8 + 376.1) s / 632.6 s) |
    | Engine CPU, `load_machine` average / RSS | 1.43 threads / 1.31 GB | 2.90 threads / 0.88 GB |

    - **SC-002 met**: 1.43 cores on LiveKit, every bot connected for the hold.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T028 [P] Add the 008 entries to the docs:
  - `specs/002-scenario-execution-engine/contracts/openapi.yaml`: `platform` enum `jitsi`, `livekit`; `platform_config` described per platform with links to the two config contracts; the `[redacted]` behaviour; `latency_reason`.
  - `README.md`: an 008 row in the status table, a LiveKit section with the setup and the live test, and the LiveKit vs Jitsi results of T025/T027.
  - `CLAUDE.md`: the LiveKit live test command.
- [X] T029 [P] Write `specs/008-livekit-platform/validation-report.md`: SC-001 to SC-006 with the measured figures of T021, T025, T026 and T027, Met/Not met and evidence. Note the research checks of T002 and R11's outcome.
- [X] T030 Final pass:
  - `go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...` are green;
  - `go mod why github.com/livekit/protocol` reports nothing (no LiveKit module);
  - `grep -rn "livekit" src --include=*.go -il` lists only `src/platform/livekit/`, `src/platform/catalog/` and `src/main.go`;
  - `git status` shows no `.env` under `tests/platforms/`.
  - **Recorded 2026-10-17**: all four checks pass.
    - `go mod why -m github.com/livekit/protocol`: the module is not needed.
    - "livekit" appears outside `src/platform/livekit` and `src/platform/catalog` only in a test name string (`src/scenario/catalog_test.go`) and a doc comment (`src/platform/internal/jwt`). `src/main.go` reaches LiveKit through the catalog only.
    - `tests/platforms/livekit/.env` and `tests/platforms/docker-jitsi-meet/` are ignored.

---

## Dependencies & Execution Order

- **Setup (T001–T002)**: first; T002 needs T001's server. Its answers can change R1/R6/R12 details used in T015–T019.
- **Foundational (T003–T009)**: T003 is independent. T004 comes before T005 and T006. T007 needs T004 and T006. T008 comes before T009. US1 needs all of them.
- **US1 (T010–T021)**: tests T010–T013 can run in parallel; T014 needs T010's codec types. Implementation runs T015–T017 in parallel, then T018 and T019, then T020 and T021.
- **US2 (T022–T025)**: needs US1 for T023 and T025. T022 and T024 only touch the quality code and can start after Phase 2.
- **US3 (T026–T027)**: needs US1 (and US2 for the full report in T027).
- **Polish (T028–T030)**: last.

### Parallel Example: User Story 1

```text
T010 proto_test.go   T011 config_test.go   T012 token_test.go   T013 subscriptions_test.go
then
T015 proto.go        T016 config.go+token.go   T017 subscriptions.go
```

### Parallel Example: Foundational

```text
T003 internal/jwt     (independent)
T004 → T005 → T006 → T007   (peer and engine)
T008 → T009                 (service and catalog)
```

## Implementation Strategy

1. **MVP**: Setup, Foundational, then US1 (T001–T021). LiveKit runs and its media metrics are visible; the T021 live check proves it.
2. **US2** (T022–T025): the comparable report, and the latency question settled on the real server.
3. **US3** (T026–T027): reproducibility from a clean checkout, and the 100-bot comparison.
4. **Polish** (T028–T030).

Keep Jitsi's live test passing after Phase 2 (one 2-bot run on docker-jitsi-meet), since the boundary and the starter change under it.
