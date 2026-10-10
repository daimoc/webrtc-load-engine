---

description: "Task list for 004 synthetic media sending and media metrics"
---

# Tasks: Synthetic Media Sending and Media Metrics

**Input**: Design documents from `specs/004-synthetic-media/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires test coverage for the media engine, the signaling contract (contract tests for adapters) and lifecycle, and benchmarks for performance-sensitive paths. Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**: Go at `/usr/local/go/bin` (`export PATH=$PATH:/usr/local/go/bin`); code stays go1.23-compatible; `go vet ./... && go test -race ./...` stays green after every task. Live runs target the local docker-jitsi-meet ([quickstart.md](quickstart.md) prerequisites).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Media content and the new package

- [X] T001 Create `scripts/gen-media.sh` that runs ffmpeg through Docker (`docker run --rm -v "$PWD":/w -w /w jrottenberg/ffmpeg:<pinned tag>`; ffmpeg is not installed locally) and writes `src/media/assets/video.ivf` (VP8, 640x360, 30 fps, `-b:v 500k` CBR, `-g 30` keyframe every 30 frames, 10 s of `testsrc2`) and `src/media/assets/audio.ogg` (Opus, 48 kHz mono, 20 ms frames, 32 kbit/s, 10 s of a 440 Hz tone with a beep every second, never silent); run it and commit both files (total < 1 MB) (research R1)
- [X] T002 Create package `src/media` with `doc.go` stating its rule: no platform or signaling concept, only RTP, SDP media kinds and content (constitution II)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Configuration, the extended signaling contract and the metric definitions that every story uses

- [X] T003 [P] Write failing tests in `src/scenario/service_test.go` (or a new `src/scenario/scenario_test.go`) for `TestPlan.Media`: missing `media` → profile `receive_only`, `stall_timeout_seconds` 10; `profile` must be "one of `receive_only`, `audio`, `audio_video`"; `stall_timeout_seconds` must be "> 0"; invalid values make `CreateScenario` fail (HTTP 400 through the existing handler)
- [X] T004 Add `MediaConfig{Profile string; StallTimeoutSeconds int}` as `TestPlan.Media *MediaConfig` (`json:"media,omitempty"`), constants for the three profiles, `MediaKinds()` (profile → `[]string{"audio"}` / `{"audio","video"}` / empty) and defaults/validation in `NewScenario` in `src/scenario/scenario.go`; make T003 pass
- [X] T005 [P] Add the `media` property and `MediaConfig` schema from `contracts/test-plan-media.yaml` to `specs/002-scenario-execution-engine/contracts/openapi.yaml`
- [X] T006 Add `LocalMediaKinds() []string` and `Renegotiate(sdp string) (string, error)` to `platform.SignalingHandler` in `src/platform/adapter.go`, with the doc comments of `contracts/signaling-handler.md`; implement them in `src/execution/peer.go` (`LocalMediaKinds` returns the kinds given at construction, `Renegotiate` returns "not implemented" until T029) and in the test handlers of `src/platform/jitsi/adapter_test.go` and `src/execution/engine_test.go`
- [X] T007 [P] Define the `webrtc_media_*` collectors of `contracts/metrics.md` (names, types, help, labels `scenario_id`,`kind`; histogram buckets suited to ms-scale lateness/jitter/RTT) in `src/media/metrics.go`, registered in `init()` as in `src/execution/metrics.go`

**Checkpoint**: configuration accepted and validated, contract extended, build and race tests green, behavior unchanged

---

## Phase 3: User Story 1 - Bots publish audio and video (Priority: P1) 🎯 MVP

**Goal**: With profile `audio` or `audio_video`, each bot sends looped Opus (and VP8) once established, announced so that browsers see and hear it; `receive_only` behaves as before.

**Independent Test**: quickstart steps 2 and 3: 2 bots with `--media audio_video` in `IntriguingClimatesSupposeProbably`; a browser sees two moving, unmuted tiles and hears the tone within 5 s; Ctrl+C removes them.

### Tests for User Story 1

- [X] T008 [P] [US1] Write failing tests in `src/media/content_test.go`: the embedded video parses into 300 ±1 VP8 frames with the first one a keyframe and a 10 s loop; the audio parses into 500 ±1 Opus packets of 20 ms
- [X] T009 [P] [US1] Write failing tests in `src/media/source_test.go` using a fake clock and fake writers: frames are written at `start + k·loopDuration + offset(i)`; sequence numbers and timestamps grow monotonically across a loop boundary; every subscribed track gets identical packets; `Unsubscribe` stops writes to that track; a writer error does not stop the others; `Run` returns when its context is cancelled
- [X] T010 [P] [US1] Write `BenchmarkSourceFanOut` in `src/media/source_bench_test.go` (one video frame to 100 `TrackLocalStaticRTP` tracks bound to in-process peers)
- [X] T011 [P] [US1] Write a failing test in `src/execution/peer_test.go`: a peer created with kinds `audio`,`video` answers an offer from a Pion offerer with `sendrecv` audio and video sections carrying `a=ssrc` (cname, msid) (no RTX `FID` group: Pion v3 sends no RTX); after `Source` subscription the offerer's `OnTrack` fires for both kinds and reads RTP; a peer with no kinds still answers `recvonly` (FR-002)
- [X] T012 [P] [US1] Write failing contract tests in `src/platform/jitsi/adapter_test.go` with the fake server: for a handler sending audio+video, the join presence carries `<SourceInfo>` `{"<nick>-a0":{"muted":false},"<nick>-v0":{"muted":false,"videoType":"camera"}}` and `audiomuted`/`videomuted` false; the session-accept sources carry `name="<nick>-a0"`/`"<nick>-v0"`, `videoType="camera"` on video, and `<ssrc-info xmlns="http://jitsi.org/jitmeet" owner="<room>@<muc>/<nick>"/>`; a receive-only handler sends neither (research R4)

### Implementation for User Story 1

- [X] T013 [US1] Implement `content.go` in `src/media`: `//go:embed assets/*`, parse with `ivfreader`/`oggreader` once (`sync.Once`) into `Content{Kind, Codec webrtc.RTPCodecCapability, Frames []Frame{Data, Duration}, LoopDuration}`; make T008 pass
- [X] T014 [US1] Implement `Source` in `src/media/source.go` per data-model "Media Source": `NewSource(scenarioID string, c *Content)`, `Run(ctx)` (one goroutine, packetize each frame once with `rtp.NewPacketizer` and the VP8/Opus payloader, MTU 1200, timestamps continuing across loops, observe `webrtc_media_send_lateness_seconds`), `Subscribe`/`Unsubscribe(*webrtc.TrackLocalStaticRTP)`; count `webrtc_media_packets_sent_total`/`bytes_sent_total` per successful track write; make T009 pass and run T010 (research R2)
- [X] T015 [US1] In `src/execution/peer.go`: `NewPeer` takes the media kinds; for each kind create a `TrackLocalStaticRTP` (codec from `media.Content`, stream ID = peer ID) and `AddTrack` it before any offer; read each sender's RTCP in a goroutine (required for Pion's interceptors; parsed in T033); log kinds the offer does not include (spec edge case); expose the tracks to the engine; make T011 pass (research R3)
- [X] T016 [US1] In `src/execution/engine.go`: from `testPlan.Media` build one `media.Source` per sent kind, run them on the engine context, subscribe a peer's tracks when it reaches `established` and unsubscribe on disconnect/failure/stop; pass the kinds to `NewPeer`
- [X] T017 [P] [US1] Add `VideoType string` (`xml:"videoType,attr,omitempty"`) and `SSRCInfo *SSRCInfo` (`<ssrc-info xmlns="http://jitsi.org/jitmeet" owner=...>`) to `Source` in `src/platform/jitsi/xmpp_structs.go`, with a marshal/unmarshal test in `src/platform/jitsi/xmpp_structs_test.go`
- [X] T018 [US1] In `src/platform/jitsi/jingle.go` (`SendSessionAccept`): set on each local source the name `<nick>-a0`/`<nick>-v0` by content kind, `videoType="camera"` for video (not on RTX sources), and `ssrc-info` owner `<room>@<muc>/<nick>`
- [X] T019 [US1] In `src/platform/jitsi/signaling.go` (`mucJoinPresence`/`JoinMUC`): call `handler.LocalMediaKinds()` and add `<SourceInfo>` JSON plus `<audiomuted>`/`<videomuted>` (`http://jitsi.org/jitmeet/audio` / `.../video` namespaces as lib-jitsi-meet) for sent kinds; make T012 pass
- [X] T020 [P] [US1] Add `--participants` (default 1) and `--media` (default `receive_only`) flags to `cmd/jitsi-connect/main.go`, passed into the test plan
- [X] T021 [P] [US1] Add `JITSI_MEDIA` (profile, default `receive_only`) to `tests/live/jitsi_live_test.go`, set in the test plan; when sending, assert `webrtc_media_packets_sent_total` grows for each sent kind during the hold
- [X] T022 [US1] Validate quickstart steps 2 and 3 against the local deployment (backward compatibility; browser sees and hears 2 bots, unmuted, within 5 s; `--media audio` gives audio only); record the outcome and any Jicofo rejection in this file
  - 2026-10-10, local docker-jitsi-meet: receive-only unchanged (2 bots, 1.6 s). `audio_video`: Jicofo "Accepted initial sources" for each bot and forwarded them by source-add; JVB receives ~448 kbit/s VP8 per bot; 50 audio + 66 video packets/s per bot. A headless Chrome in the room (DevTools) sees both bots unmuted, decodes their 640x360 video at 30 fps and their audio energy grows (SC-001).
  - Blocker found: a bot joining a room where 2+ participants already send gets an offer with several sources per section, which Pion rejects ("Expected UnifiedPlan, but RemoteDescription is PlanB"). Needs the per-source expansion of T030/T031 (research R5); revalidate `--media audio` and late joiners after it.
  - Revalidated after T030/T031: 2 `audio` bots joining a room where 2 bots already send audio+video are offered 3 remote audio streams each and receive ~150 packets/s; no negotiation error.
  - JVB forwards no video to bots here: default receiver constraints are maxHeight 0 until a client sends ReceiverVideoConstraints over the bridge channel, which the bots did not have (correction 2026-10-10: Jicofo only offers the data content to clients advertising JSON-encoded sources, fixed in 003 T056; the bots still send no constraints, see feature 005). Browsers send theirs, so they see the bots.

**Checkpoint**: MVP: bots load the bridge with real media; existing scenarios unchanged

---

## Phase 4: User Story 2 - Measure media received by bots (Priority: P2)

**Goal**: Bots receive every forwarded stream (including participants who join later) and the engine exports per-scenario volumes, loss, jitter, round trip, remote streams and stalls.

**Independent Test**: quickstart step 4: 10 sending bots for 30 s; metrics show 9 audio and up to 9 video remote streams per bot, received volume within 10% of streams × rate, zero stalls; a browser joining mid-run raises the remote video streams.

### Tests for User Story 2

- [X] T023 [P] [US2] Write failing tests in `src/platform/jitsi/jingle_sdp_test.go` for the expansion (research R5): a session-initiate with 2 remote audio and 2 remote video sources (with RTX `FID` groups) gives the main `audio`/`video` sections plus one `recvonly` section per remote source (RTX in the same section), all in BUNDLE, parseable by Pion; collapsing a Pion answer keeps only the `audio`/`video`/`data` contents, our own sources, and a BUNDLE group filtered to them
- [X] T024 [P] [US2] Write failing tests in `src/platform/jitsi/sources_test.go` for the session source state (data-model "Jitsi session source state"): add assigns a free section of the same kind or appends one; remove frees the section (rendered `inactive` without SSRC) and the next add reuses it; mids never change; `version` increments
- [X] T025 [P] [US2] Write failing contract tests in `src/platform/jitsi/adapter_test.go`: after the session is accepted the fake focus sends `source-add` then `source-remove`; the adapter acks each IQ and calls `handler.Renegotiate` with an offer that contains (then no longer contains) the source; a `source-add` received before the accept is applied after it; renegotiations of one session never overlap
- [X] T026 [P] [US2] Write failing tests in `src/media/inbound_test.go`: in-order packets → no loss; a gap of 3 → 3 lost; reordering does not count loss; sequence wrap-around (65535→0) is handled; jitter follows RFC 3550 A.8 on a synthetic arrival series and converts to seconds with the clock rate
- [X] T027 [P] [US2] Write failing tests in `src/media/rtt_test.go`: round trip from a receiver report's LSR/DLSR against a known compact NTP send time; LSR 0 gives no sample
- [X] T028 [P] [US2] Write failing tests in `src/execution/peer_test.go`: with two in-process Pion peers, a remote audio and video track are read and accounted (packets, bytes > 0, kind correct); `Renegotiate` with an offer adding a second video track makes it received too, without a new negotiation metric sample

### Implementation for User Story 2

- [X] T029 [US2] Implement `Peer.Renegotiate` in `src/execution/peer.go`: serialize with the negotiation, `SetRemoteDescription`(offer), `CreateAnswer`, `SetLocalDescription`, return the answer SDP; no trickle re-arm, no `negotiationSeconds` sample
- [X] T030 [US2] Implement the per-source expansion in `convertJingleToSDP` and the collapse in `convertSDPToJingle` in `src/platform/jitsi/jingle_sdp.go`, driven by the section list of T031; make T023 pass
- [X] T031 [US2] Implement the session source state in `src/platform/jitsi/sources.go` (remote description, sections, version, add/remove); keep it per Jingle session in `src/platform/jitsi/jingle.go`; make T024 pass
- [X] T032 [US2] Dispatch `source-add`/`source-remove` from the focus in `src/platform/jitsi/signaling.go` (today acknowledged and ignored) to the session: update state, regenerate the offer, call `handler.Renegotiate` (answer dropped), serialized per session, queued until the session is accepted; make T025 pass (research R6)
- [X] T033 [US2] Implement `InboundStats` in `src/media/inbound.go` and the round-trip computation in `src/media/rtt.go`; make T026 and T027 pass
- [X] T034 [US2] In `src/execution/peer.go`: `OnTrack` starts a read loop per remote track updating its `InboundStats` and `lastReceived[kind]`, ending when the track ends; the sender RTCP loops of T015 parse receiver reports into `rtt[kind]`; expose a `MediaSnapshot()` (per-kind totals, jitter samples, rtt, remote stream count, last received); make T028 pass (research R7)
- [X] T035 [US2] Write failing tests then implement the per-scenario collector in `src/execution/media_collector.go` (+ `_test.go`): every 2 s, for each `established` peer, add `MediaSnapshot` deltas to the received/lost counters, observe jitter and round-trip histograms, set `webrtc_media_remote_streams`; detect stalls per research R8 with `stall_timeout_seconds` (counted once until media resumes, logged with `scenario_id`/`peer_id`, skipped when the peer has no remote source of that kind, never a lifecycle transition, FR-014); started and stopped by `src/execution/engine.go`
- [X] T036 [US2] Extend `tests/live/jitsi_live_test.go`: when sending, log per kind sent/received packets, loss and remote streams per bot at the end of the hold; assert remote streams per bot equal `participants-1` for audio, received packets grow for every sent kind, and no stall
- [X] T037 [US2] Validate quickstart step 4 against the local deployment (10 bots, then a browser late joiner); record received vs expected volume (SC-002) and the health queries of `contracts/metrics.md` (SC-005) in this file
  - 2026-10-10, 10 `audio_video` bots, 30 s: each bot offered 9 remote audio and 9 remote video streams; audio received 488 packets/s per bot against 9 × 50 × 32.5/30 = 487.5 expected over the measurement window (SC-002 met for audio), 0 lost, 0 stalls. Video received 0: this JVB forwards no video to a receiver until it sends ReceiverVideoConstraints over the bridge channel, which the bots did not open at the time (Jicofo offers it only to clients with JSON-encoded sources, fixed in 003 T056) and on which they send no constraints yet (feature 005); browsers do receive the bots' video (T022). SC-002 for video needs a deployment with a working bridge channel, plus bots sending their receiver constraints (not in this feature).
  - Late joiner (3 bots, headless Chrome with fake camera, `JITSI_METRICS_ADDR` to watch `/metrics`): remote streams 6 → 9 on join, back to 6 on leave, no stall. Found and fixed: Jicofo sends no source-remove when a participant leaves; its sources are now dropped on its unavailable MUC presence (`removeOwnerSources`), as lib-jitsi-meet does.
  - SC-005: received rate per kind, `webrtc_media_remote_streams`, `webrtc_media_packets_lost_total` and `webrtc_media_stalls_total` give the health of a run from `/metrics` alone (queries in `contracts/metrics.md`).

**Checkpoint**: a run's media health is readable from `/metrics` alone

---

## Phase 5: User Story 3 - Sending at scale on one machine (Priority: P3)

**Goal**: 100 sending bots on one machine within the send-timing and CPU budget.

**Independent Test**: quickstart step 5.

- [X] T038 [US3] Run quickstart step 5 (100 bots, `audio_video`, 10 min hold) against the local deployment; record in this file: bots established/failed, p99 `webrtc_media_send_lateness_seconds` (SC-003, < 20 ms), average CPU cores and max RSS (SC-004, ≤ 2 cores), remote streams per bot; compare with the receive-only 100-bot run of 002 T016
  - 2026-10-10, local docker-jitsi-meet on the same laptop (i5-13600H, 16 threads), 100 bots `audio_video`, ramp-up 20 s, 10 min hold: all 100 established in 22.4 s, held 10 min, 0 failed, 0 stalls, no WARN/ERROR log. Sent 50 audio + 66 video packets/s per bot. Each bot offered 99 audio + 99 video streams; received 4,927 audio packets/s per bot (99.5% of 99 × 50), 0.6% lost overall (kernel UDP receive-buffer drops in the bot process); no video received (bridge channel unavailable, see T037).
  - SC-004 not met: 3.85 cores on average (`time`: 2072 s user + 342 s system over 626 s), max RSS 3.25 GB (receive-only 100 bots in 002 T016: 0.07 cores, 100 MB). SC-003 not met: about 96.5% of frames written within 20 ms at mid-run (p99 above 20 ms), with the bridge using about 9 cores on the same host.
- [X] T039 [US3] If SC-003 or SC-004 is not met: profile (`go test -cpuprofile` on the live test) and fix hot paths in `src/media/source.go` / `src/execution/peer.go`; if the cost is reception of full forwarding, rerun with the bridge's last-N set (`videobridge.cc.jvb-last-n`) and record both results and the follow-up decision in `research.md` R10
  - Profile (30 s CPU, heap after GC, 100 bots): the cost is receiving 495,000 audio packets/s (100 × 99 × 50). Pion's receive path dominates: one recvfrom per packet (14%), SRTP demux/decrypt and packetio buffers (about 25%), scheduling and channels; our own accounting (`InboundStats.Update`, read loops) is about 2%. Removing the TWCC feedback and NACK generator interceptors was measured with a temporary patch (not kept): 3.79 vs 3.86 cores, so not worth losing their realism. No hot path in this project's code to fix.
  - Memory: Pion v3 parses SDP without copying (`UnmarshalString`), so the strings it keeps from a parsed offer pin the whole offer; with 201-section offers (128 KB) and ~50 renegotiations per bot during the ramp-up, about 640 MB of offers stay live. It stays flat once the room is stable (no growth over the 10 min hold).
  - The bridge's last-N only limits video; audio is forwarded from every participant, so the reception load cannot be lowered from the bot side without changing what it receives. Follow-up needs a decision (research R10): measure with the bridge routing only the loudest audio (`videobridge.loudest.route-loudest-only`), run the bridge on another host than the bots, or bound reception on the bot side (spec change).

**Checkpoint**: scale verified, or the gap and its cause documented

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T040 [P] Document the `media` test plan field and the media metrics in `specs/003-xmpp-signaling/quickstart.md` ("Observing a Run") and `README.md`; add `--media`/`--participants` to the commands in `CLAUDE.md`
- [X] T041 [P] Mark FR-002/FR-008 coverage: confirm no file under `src/media` or `src/execution` imports `src/platform/jitsi` or mentions XMPP/Jingle (`grep`), and that metrics carry no peer label
- [X] T042 Run `gofmt -l` on the files touched by this feature, `go vet ./...`, `go vet -tags live ./tests/live/`, `go test -race ./...`; all clean

---

## Phase 7: Receive cap (US3, FR-015, research R12; added after T038/T039)

**Goal**: each bot receives at most `max_received_streams` remote streams per kind, so that 100 bots fit the SC-004 budget.

**Independent Test**: quickstart step 5 with `JITSI_MAX_RECEIVED=10`.

- [X] T043 [US3] Write failing tests in `src/scenario/scenario_test.go` then add `MaxReceivedStreams int` (`json:"max_received_streams,omitempty"`, "≥ 0", default 0 = no cap) to `MediaConfig` in `src/scenario/scenario.go`; add it to `specs/002-scenario-execution-engine/contracts/openapi.yaml`
- [X] T044 [US3] Add `ReceiveLimit() int` to `platform.SignalingHandler` in `src/platform/adapter.go`; `NewPeer` takes it (with the kinds) and returns it; engine passes `testPlan.Media.MaxReceivedStreams`; test handlers implement it (`src/execution/peer.go`, `src/execution/engine.go`, `src/platform/jitsi/adapter_test.go`)
- [X] T045 [P] [US3] Write failing tests in `src/platform/jitsi/sources_test.go`: with limit 2, a session-initiate with 3 audio sources offers 2 audio sections; the 3rd is waiting; removing an active source (source-remove or owner leaving) promotes the waiting one in a new section; limit 0 offers all; the bridge's sources do not count
- [X] T046 [US3] Implement the cap in `src/platform/jitsi/sources.go` (per-kind active count, waiting queue, promotion on remove/removeOwner), set from the handler's `ReceiveLimit()` when the session is created in `src/platform/jitsi/jingle.go`; make T045 pass
- [X] T047 [P] [US3] Write failing tests in `src/execution/peer_test.go`: a remote stream whose SSRC is not in the applied offer is not reported by `MediaSnapshot` and its packets are dropped by a buffer that never grows; after a renegotiation that offers it, it is received
- [X] T048 [US3] In `src/execution/peer.go`: record the offered SSRCs on each applied offer; give Pion a `SettingEngine` with a `BufferFactory` whose buffers drop packets (writes) and fail reads at once while their SSRC is not offered, and behave as a 1 MB `packetio.Buffer` once it is; route Pion's logs to the peer's structured logger (errors as warnings, `Incoming unhandled RTP ssrc` as debug); make T047 pass
- [X] T049 [US3] Add `JITSI_MAX_RECEIVED` to `tests/live/jitsi_live_test.go` (sets `max_received_streams`; the remote audio streams check becomes `min(cap, participants-1)` per bot) and `--max-received` to `cmd/jitsi-connect/main.go`
- [X] T050 [US3] Rerun quickstart step 5 with `JITSI_MAX_RECEIVED=10` (100 bots, `audio_video`, 10 min); record CPU, RSS, send lateness (SC-003) and received/remote streams per bot here and in research R12; run `go vet ./... && go test -race ./...`
  - 2026-10-10, same host and deployment as T038, 100 bots `audio_video`, `max_received_streams` 10, ramp-up 20 s, 10 min hold: all established in 21.3 s and held, 0 failed, 0 stalls, no WARN/ERROR log. Each bot offered 10 audio + 10 video streams, received 499 audio packets/s (10 × 50), 672 lost out of 30.1 M.
  - SC-003 met: 99.97% of audio frames and 99.90% of video frames written within 20 ms (99.5% / 99.0% within 5 ms).
  - SC-004 not met: 2.60 cores on average (1237 s user + 389 s system over 624 s; was 3.85 without the cap), max RSS 723 MB (was 3.25 GB). The bridge still sends all 99 audio streams to each bot (495,000 packets/s): the profile shows the remaining cost is spent before a packet can be dropped (receive system calls and ICE/mux about 45%, SRTP decryption about 24%); the 10 received streams cost about 5%. The bridge itself used about 10 cores on the same host. Going lower needs the bridge to stop sending the streams a bot does not receive (Jitsi's receiver audio subscription / last-N over the bridge channel; the channel is available since 003 T056, the messages are feature 005).

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none
- **Foundational (Phase 2)**: after Setup; blocks all stories
- **US1 (Phase 3)**: after Foundational
- **US2 (Phase 4)**: after Foundational for implementation (T023–T035 do not need US1 code); its validation (T036–T037) needs US1's senders
- **US3 (Phase 5)**: after US1 and US2 (measures the full send and receive path)
- **Polish (Phase 6)**: after the stories you ship

### Within each story

- Tests first, failing, then implementation
- US1: T013 → T014 → T015 → T016; T017 → T018 → T019; T020/T021 any time after T016
- US2: T031 → T030 → T032; T033 → T034 → T035; T029 before T032 is exercised end to end

### Parallel Opportunities

- Phase 2: T003, T005, T007 in parallel; T006 alone (touches several files)
- US1 tests T008–T012 together; T017, T020, T021 alongside the media work
- US2 tests T023–T028 together; the Jitsi track (T030–T032) and the media track (T033–T035) in parallel

```text
# US1, launch the tests together:
T008 src/media/content_test.go
T009 src/media/source_test.go
T010 src/media/source_bench_test.go
T011 src/execution/peer_test.go
T012 src/platform/jitsi/adapter_test.go

# US2, two independent tracks:
Jitsi:  T031 → T030 → T032  (src/platform/jitsi/)
Media:  T033 → T034 → T035  (src/media/, src/execution/)
```

## Implementation Strategy

### MVP (User Story 1)

1. Phases 1 and 2
2. Phase 3; stop at T022: bots publish audio and video into a real conference
3. This alone gives real media load on the bridge, observed from the bridge side

### Incremental delivery

1. + US2: bots receive everything forwarded and the run reports media health
2. + US3: the 100-bot measurement and any fixes it requires
3. Commit after each checkpoint once validated
