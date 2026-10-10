---

description: "Task list for 005 bridge channel subscriptions"
---

# Tasks: Bridge Channel Subscriptions

**Input**: Design documents from `specs/005-bridge-channel-subscriptions/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires contract tests for the signaling contract and adapters. Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**: Go at `/usr/local/go/bin` (`export PATH=$PATH:/usr/local/go/bin`); code stays go1.23-compatible; `go vet ./... && go test -race ./...` stays green after every task. Live runs target the local docker-jitsi-meet ([quickstart.md](quickstart.md)). Tests that read global metrics use a unique scenario ID (uuid), as in 004.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2)

---

## Phase 1: Setup

**Purpose**: none needed. No new module, package or asset: the work extends `src/platform`, `src/execution` and `src/platform/jitsi`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: the platform-neutral data channel contract (send and events) and its open gauge, used by both stories

- [X] T001 Write a failing contract test in `src/execution/peer_test.go` (extend `TestPeer_OpenDataChannelReachesRemote`): `OpenDataChannel(label, protocol, h)` on a peer answering an in-process Pion offerer returns a `platform.DataChannel`; `SendText` before open returns an error without panicking; after open `h.OnOpen` fires once, `SendText("x")` reaches the remote side, a text sent by the remote reaches `h.OnMessage`, a binary message does not; closing the remote fires `h.OnClose`; `webrtc_data_channels_open{scenario_id}` goes 0 → 1 → 0 and stays 0 after `Close()` of the peer (contracts/signaling-handler.md)
- [X] T002 Add `DataChannel` (`SendText(msg string) error`) and `DataChannelHandler` (`OnOpen()`, `OnMessage(text string)`, `OnClose()`) to `src/platform/adapter.go` and change `SignalingHandler.OpenDataChannel` to `OpenDataChannel(label, protocol string, h DataChannelHandler) (DataChannel, error)`, with the doc comments of `contracts/signaling-handler.md` ("called from the media stack's goroutines, one at a time per channel, and must not block")
- [X] T003 [P] Define the gauge `webrtc_data_channels_open` (label `scenario_id`) and the counter `webrtc_transport_bytes_received_total` (label `scenario_id`) in `src/execution/metrics.go` with the help texts of `contracts/metrics.md`
- [X] T004 Implement T002 in `src/execution/peer.go`: a `dataChannel` type wrapping `*webrtc.DataChannel` (`SendText` → `dc.SendText`), forwarding Pion's `OnOpen`/`OnMessage` (text only, `msg.IsString`)/`OnClose` to `h`, keeping the 003 message count and debug log; increment the gauge on open and decrement on close or in `Peer.Close` if still counted, exactly once (atomic flag per channel, data-model "Data channel"); make T001 pass
- [X] T005 Update the other `SignalingHandler` implementations to the new signature: `pionHandler` in `src/platform/jitsi/adapter_test.go` (record the handler and return a fake `DataChannel` that stores the sent texts and can be opened/closed by the test) and any handler in `src/execution/engine_test.go`; change the call in `negotiate` in `src/platform/jitsi/jingle.go` to pass a no-op handler for now; `go vet ./... && go test -race ./...` green

**Checkpoint**: contract changed, behavior unchanged (the adapter still sends nothing)

---

## Phase 3: User Story 1 - Bots receive video (Priority: P1) 🎯 MVP

**Goal**: each bot sends `ReceiverVideoConstraints` on the bridge channel for the video it receives, so the bridge forwards video to it.

**Independent Test**: quickstart step 1: 10 sending bots, 30 s; `webrtc_data_channels_open` = 10 and each bot's received video within 10% of 9 × one bot's sent video (SC-001).

### Tests for User Story 1

- [X] T006 [P] [US1] Write failing tests in `src/platform/jitsi/bridge_channel_test.go` for the message builder: without a cap → exactly `{"colibriClass":"ReceiverVideoConstraints","lastN":-1,"defaultConstraints":{"maxHeight":360}}` (compare decoded JSON); with a cap and video names `["a-v0","b-v0"]` → `lastN -1`, `defaultConstraints {"maxHeight":0}`, `constraints {"a-v0":{"maxHeight":360},"b-v0":{"maxHeight":360}}`; with a cap and an empty list → `constraints {}` with default 0; with a cap and an unnamed video source → the no-cap form (contracts/colibri-messages.md, research R1, R3)
- [X] T007 [P] [US1] Write failing tests in `src/platform/jitsi/bridge_channel_test.go` for the send rules with a fake `DataChannel`: nothing is sent before `OnOpen`; on `OnOpen` the current state is sent; an update while open sends only messages that differ from the last sent; an identical update sends nothing; after `OnClose` an update is stored, not sent, and the next `OnOpen` sends the latest state (FR-006); a failing `SendText` is logged once at `Info` and retried on the next update or open; `OnMessage` with `{"colibriClass":"ServerHello"}`, an unknown class or invalid JSON neither fails nor replies (FR-009); concurrent updates and opens under `-race` end with the latest state sent last
- [X] T008 [P] [US1] Write failing contract tests in `src/platform/jitsi/adapter_test.go` with the fake server and `pionHandler`'s fake channel: a session-initiate with SCTP and two remote video sources → after the fake channel opens, one `ReceiverVideoConstraints` is sent (no-cap form); with `limit` 1 and a `source-add` of a second video source → no new video message (the source waits); `source-remove` of the received one → a message naming the promoted source, sent after the renegotiation (within the test's 2 s, FR-005); `TestAdapter_NoBridgeChannel`: an offer without SCTP → no `OpenDataChannel` call, one `Info` log "bridge channel not offered", session accepted
- [X] T009 [P] [US1] Write a failing test in `src/platform/jitsi/sources_test.go` for `receivedNames(content)`: names of the primary sources of active sections in section order, removed (inactive) sections skipped, waiting sources excluded, and a flag when an active primary source has no `Name`

### Implementation for User Story 1

- [X] T010 [US1] Add `receivedNames(content string) (names []string, unnamed bool)` to `sessionSources` in `src/platform/jitsi/sources.go` (data-model "Receive Preferences": "at most `limit`; empty is valid"); make T009 pass
- [X] T011 [US1] Create `src/platform/jitsi/bridge_channel.go`: type `bridgeChannel` with fields `dc platform.DataChannel`, `open bool`, `lastSent map[string]string` ("cleared on close so that a reopen resends everything"), `logged map[string]bool`, `openTimer *time.Timer`, `mu sync.Mutex`, the current preferences (`capped`, `video []string`, `videoUnnamed`); `videoConstraintsMessage(prefs)` per contracts/colibri-messages.md with `maxHeight` constant 360; `update(prefs)`, `OnOpen`, `OnClose`, `OnMessage` (debug log of `colibriClass` only), `stop()`; it implements `platform.DataChannelHandler`; make T006 and T007 pass (video only)
- [X] T012 [US1] Wire it in `src/platform/jitsi/jingle.go`: give `JingleSession` a `bridge *bridgeChannel`; in `negotiate`, when `offersSCTP`, create it, pass it to `handler.OpenDataChannel` and store the returned `DataChannel`, start the 10 s `openTimer` that logs "bridge channel not open" once (research R8); when the offer has no SCTP log "bridge channel not offered" once at `Info`; after the initial negotiation and at the end of each successful `renegotiate`, compute the preferences from `session.sources` under `s.mu` and call `bridge.update` outside it; call `bridge.stop()` when the session ends (`handleSessionTerminate`, `SendSessionTerminate`, `TerminateSessions`); make T008 pass
- [X] T013 [US1] Enable the video checks in `tests/live/jitsi_live_test.go`: assert `webrtc_data_channels_open` equals the number of participants once all are established; with `JITSI_EXPECT_VIDEO` set, require received video per bot within 10% of `min(cap, N-1)` × the video packets one bot sends over the same window (SC-001)
- [X] T026 [US1] Write failing tests in `src/execution/twcc_test.go` (added after T014's first run, research R9): RTP packets carrying the transport-cc extension (ID taken from the applied offer's `a=extmap`) written to a peer's read buffers are all acknowledged in the next feedback packet, including packets of an SSRC the applied offer does not list (dropped by the gate); packets without the extension, or before an offer with the extension, are ignored
- [X] T027 [US1] Create `src/execution/twcc.go`: `transportFeedback` (extension ID from `recordOffered`, `twcc.Recorder` fed from `gatedBuffer.Write` for RTP buffers, feedback written every 100 ms with `pc.WriteRTCP` by one goroutine per peer stopped by `Close`); in `src/execution/pion.go` register NACK and RTCP reports plus the transport-cc header extension and feedback, without Pion's TWCC sender interceptor; make T026 pass
- [X] T014 [US1] Run quickstart step 1 (10 bots, `audio_video`, 30 s) against the local deployment; if video is held back by the bridge's bandwidth estimate, apply research R6 (`videobridge.cc.trust-bwe = false`) and note it; record the results under this task
  - 2026-10-10, local docker-jitsi-meet (jvb 2.3-326), 10 bots `audio_video`, 30 s hold: 10/10 bridge channels open; before T026/T027 the bots received video at 0.26 of the expected volume (the bridge's estimate stuck near 610 kbit/s, 8 of 9 sources suspended, 38% outgoing loss computed from the bots' TWCC feedback, research R9).
  - After T026/T027: 582 video packets/s received per bot, 0.98 of 9 × 66/s sent (SC-001 met; 0.99 with a 60 s hold); audio 450/s per bot; 0 lost, 0 stalls. No bridge setting changed (`trust-bwe` left at its default).

**Checkpoint**: bots receive video; MVP done

---

## Phase 4: User Story 2 - The server sends only what the bots receive (Priority: P1)

**Goal**: with a receive cap, bots subscribe to the audio they receive, so the bridge stops sending the other streams; the traffic the bots get from the server is measurable.

**Independent Test**: quickstart steps 2 and 3: 15 bots with cap 5, then 100 bots with cap 10 for 10 min; traffic ratio near step 1's, ≤ 2 cores (SC-002, SC-003).

### Tests for User Story 2

- [X] T015 [P] [US2] Extend `src/platform/jitsi/bridge_channel_test.go` with failing tests: with a cap and audio names `["a-a0","b-a0"]` → `{"colibriClass":"ReceiverAudioSubscription","mode":"Include","list":["a-a0","b-a0"]}`; with a cap and no audio → `"list":[]` (not null); without a cap → no audio message ever; with an unnamed audio source → no audio message and one `Info` log; on open both video and audio messages are sent (FR-003, FR-004, research R2)
- [X] T016 [P] [US2] Extend `src/platform/jitsi/adapter_test.go` with failing tests: `limit` 1, two remote participants with audio → the audio subscription names only the first; unavailable presence of the first participant → after the renegotiation, an audio subscription naming the second (promotion, US2 scenario 2) and a video message naming its video
- [X] T017 [P] [US2] Write a failing test in `src/execution/media_collector_test.go`: a fake `mediaPeer` whose `TransportBytesReceived()` returns 1000 then 3500 adds 1000 then 2500 to `webrtc_transport_bytes_received_total{scenario_id}`; a peer that is no longer established is not read again; the final collection includes the last delta

### Implementation for User Story 2

- [X] T018 [US2] Add the audio subscription to `src/platform/jitsi/bridge_channel.go` (`audioSubscriptionMessage(prefs)`, `audio []string` and `audioUnnamed` in the preferences, fed from `receivedNames("audio")` in `jingle.go`); make T015 and T016 pass
- [X] T019 [US2] Add `TransportBytesReceived() uint64` to `Peer` in `src/execution/peer.go` (RTP and RTCP bytes counted in `gatedBuffer.Write` before the SSRC gate; the selected candidate pair's byte count planned first is not filled by pion/ice v2.3.38; research R5), add it to the `mediaPeer` interface and add the per-peer delta to the counter in `src/execution/media_collector.go`; make T017 pass
- [X] T020 [US2] In `tests/live/jitsi_live_test.go`, log at the end of the run, per scenario, the transport bytes received, the media bytes received per kind and their ratio (contracts/metrics.md), and the per-bot received streams per kind
- [X] T021 [US2] Run quickstart step 2 (15 bots, cap 5), including the browser join/leave for SC-004 and the bridge `/debug` cross-check; record ratio, streams per bot and replacement delay under this task
  - 2026-10-10: transport ratio (RTP+RTCP bytes received / media payload kept) 1.03 for 10 bots without a cap (SC-001 runs: video 0.98, 0.94, 0.98, 0.83 of the expected volume over 30 s; the low one is the bridge's estimate ramping up at the start of the window, 0.99 over 60 s), and 1.03 for 15 bots with cap 5 (5 audio + 5 video streams per bot, video 0.98): the bridge sends nothing beyond the received streams. Its `/debug` state lists each bot's 5 video constraints (default `maxHeight` 0) and audio subscription.
  - SC-004 without a browser (headless Chrome joined without sources, as in 004): group A, 5 bots, joined first and left after 20 s; group B, 10 bots with cap 5, joined 5 s later, so A's streams were B's received ones. When A left, every B bot promoted 5 waiting audio and 5 waiting video streams: 100 replacements, first packet at most 0.50 s after the departure presence (100% within 5 s); 0 lost, 0 stalls.
- [X] T022 [US2] Run quickstart step 3 (100 bots, `audio_video`, cap 10, 10 min, `JITSI_METRICS_ADDR` set); record average CPU, max RSS, the traffic ratio, received streams per bot, stalls and the bridge's NET I/O under this task next to the 004 T050 numbers (2.60 cores, 723 MB); state whether SC-002 and SC-003 are met
  - 2026-10-10, same host and deployment as 004 T050, 100 bots `audio_video`, `max_received_streams` 10, ramp-up 20 s, 10 min hold, test binary under `/usr/bin/time -v`: all established in 21.1 s and held, 0 failed, 100/100 bridge channels open, 0 stalls, no WARN/ERROR log.
  - Each bot received 10 audio streams (501 packets/s, 114 lost out of 30.2 M) and 10 video streams (649 packets/s, 0.99 of 10 × 66/s sent; 0 lost): video is now forwarded (it was not in 004).
  - SC-002 met: 1.05 cores on average (529 s user + 129 s system over 624 s; 004 T050: 2.60 cores while receiving no video), max RSS 818 MB (004: 723 MB, now with video received).
  - SC-003 met: 42 575 MB of RTP/RTCP received for 41 300 MB of media payload kept, ratio 1.03, the same as the 10-bot runs without a cap (T021): the bridge sends only the received streams. The bridge used about 3.5 cores during the hold (about 10 in 004 T050), NET I/O sampled with `docker stats`.

**Checkpoint**: both stories validated

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T023 [P] Update `README.md`: bots request video and subscribe to their capped streams over Jitsi's bridge channel; the two new metrics and how to read the traffic ratio
- [X] T024 [P] Update `specs/004-synthetic-media/spec.md` (assumption on bot-side enforcement and edge case "Receive cap reached") and `specs/004-synthetic-media/contracts/test-plan-media.yaml` (`max_received_streams` description): with the bridge channel open the server no longer sends the ignored streams (005); the bot-side cap remains as a safety net
- [X] T025 Run `go vet ./... && go test -race ./...` and `go vet -tags live ./tests/live/`; review that no Colibri name or format appears outside `src/platform/jitsi` (`grep -rn colibriClass src | grep -v platform/jitsi` is empty, FR-010)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: blocks both stories (T001 → T002 → T004; T003 any time; T005 after T002)
- **US1 (Phase 3)**: after Foundational
- **US2 (Phase 4)**: T015–T018 after T011/T012 (same files: `bridge_channel.go`, `jingle.go`); T017/T019 only need Foundational; validation T021–T022 after US1
- **Polish (Phase 5)**: after both stories

### Within each story

- Tests first, failing, then implementation
- US1: T009 → T010; T006/T007 → T011; T008 → T012 (needs T010, T011); T013 → T014
- US2: T015/T016 → T018; T017 → T019; T020 → T021 → T022

### Parallel Opportunities

- Phase 2: T003 alongside T001/T002
- US1 tests T006–T009 together
- US2: the Jitsi track (T015, T016, T018) and the execution track (T017, T019) in parallel
- Polish: T023, T024 together

```text
# US1, launch the tests together:
T006, T007 src/platform/jitsi/bridge_channel_test.go
T008       src/platform/jitsi/adapter_test.go
T009       src/platform/jitsi/sources_test.go

# US2, two independent tracks:
Jitsi:      T015, T016 → T018  (src/platform/jitsi/)
Execution:  T017 → T019        (src/execution/)
```

## Implementation Strategy

### MVP (User Story 1)

1. Phase 2 (data channel contract)
2. Phase 3; stop at T014: bots receive video, 004 SC-002 checked for video

### Incremental delivery

1. + US2: the bridge sends only the capped streams; the 100-bot budget is measured again
2. Polish: docs and boundary check
3. Commit after each checkpoint once validated (only when asked)
