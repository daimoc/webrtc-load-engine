---

description: "Task list for 012 video latency on servers that rewrite timestamps"
---

# Tasks: Video Latency on Servers That Rewrite Timestamps

**Input**: Design documents from `specs/012-video-frame-latency/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/frame-trailer.md](contracts/frame-trailer.md), [quickstart.md](quickstart.md)

**Tests**: Included. The spec's SC-003 and SC-004 are automated tests, and the constitution requires test coverage of the media engine. Write each test first and see it fail.

**Organization**: The mechanism is shared, so it is foundational (Phase 2). Each story then proves it on its platform.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green at the end of each phase.
- **Boundaries** (FR-002, FR-007, FR-013):
  - production changes stay in `src/media`; `src/execution`, `src/platform`, `src/scenario` and `src/report` are not changed (test files excepted);
  - `VideoQuality.SetLatency(st, o)` keeps its signature;
  - no new module in `go.mod`;
  - audio latency code and `initialRTPTimestamp` stay as they are (research R6).
- **Names** (used across tasks): in `src/media/frame_trailer.go`: `frameTrailerSize = 6`, `scenarioTag(id string) [3]byte`, `appendFrameTrailer(p []byte, tag [3]byte, n uint32) []byte`, `readFrameTrailer(p []byte, tag [3]byte) (n uint32, ok bool)`, `newerFrame(n, last uint32) bool`. In `src/media/latency.go`: `NewVideoSendTimes(tag [3]byte) *SendTimes`, `(*SendTimes).Tag() [3]byte`.
- **Metrics in tests**: tests that read global metrics use a unique scenario ID (uuid), as in 004–011.
- **Live runs**: local servers in `tests/platforms/`. Compared runs are alternated and taken in the same machine state, screen unlocked: figures from different days are not comparable (011 validation report). Stop only what you started. Never `pkill -f`.
- **Prototype**: the probe's diff is not in the repository; [research R2, R3 and R7](research.md) describe what it showed. It used 8 bytes; the design uses 6.
- **Commits**: only when the user asks.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story the task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Keep a baseline for the comparisons of Phase 5.

- [X] T001 Create `specs/012-video-frame-latency/validation-report.md` with sections for each success criterion, record `git rev-parse HEAD` as the pre-012 baseline commit, and record the output of `go test -run '^$' -bench 'QualityVideoPacket' -benchmem ./src/media/` as the per-packet baseline

---

## Phase 2: Foundational (the mechanism)

**Purpose**: Frames carry a trailer and receivers read it. All stories depend on this phase.

**⚠️ CRITICAL**: No story's validation can begin until this phase, including the browser gate T011, is done.

- [X] T002 [P] Write failing tests in `src/media/frame_trailer_test.go` for the [contract](contracts/frame-trailer.md): layout with tag `a1 b2 c3` and frame 258 gives the last bytes `a1 b2 c3 00 01 02`; `readFrameTrailer` returns the number on a match and false on a wrong tag, on a payload under 6 bytes and on an empty payload; `scenarioTag` is stable for one ID and differs for two test UUIDs; `newerFrame` in 24-bit modular order: 2 after 1 true, 1 after 2 false, equal false, 0 after 16777215 true
- [X] T003 Implement `src/media/frame_trailer.go` with the names above: tag is "first 3 bytes of the 32-bit FNV-1a of the scenario ID"; frame number "big endian, from 1, +1 per frame, modulo 2^24"; `readFrameTrailer` does not allocate
- [X] T004 [P] Write failing tests in `src/media/latency_test.go`: `NewVideoSendTimes(tag).Tag()` returns the tag; recording and lookup by frame number; the memory stays 512 frames
- [X] T005 Add the tag to `SendTimes` in `src/media/latency.go`: `NewVideoSendTimes`, `Tag`; `NewSendTimes` stays for audio; update the type's comment (video is keyed by frame number, audio by RTP timestamp)
- [X] T006 [P] In `src/media/source_test.go`, write failing tests and adapt the existing ones (the checks near lines 139, 223 and 277, for video only; audio keeps looking up timestamps): every video frame's last packet ends with the tag and a frame number from 1, +1 per frame, continuing across a content loop; only that last packet does (no earlier packet of a frame); all simulcast layers of one frame carry the same number; every payload stays within `rtpPayloadMTU` with frames sized so that the data alone exactly fills 1, 2 and 3 packets; audio packets are byte-identical to before; `SendTimes().Lookup(n)` gives the frame's send time
- [X] T007 Implement sending in `src/media/source.go` (research R7): `NewLayeredSource` uses `NewVideoSendTimes(scenarioTag(scenarioID))` for video; `Run` increments the frame number (24 bits) once per frame for video and records it instead of `ts`; `packetize` calls the payloader with `rtpPayloadMTU - frameTrailerSize` for video and appends the trailer to the last payload, leaving a frame with no payloads alone
- [X] T008 [P] In `src/media/video_quality_test.go`, adapt `TestVideoQuality_Latency` to `UpdatePacket` with trailers, and write failing tests of the [sample rule](data-model.md): a frame's latency is measured from its first packet's arrival even when the trailer arrives later in a multi-packet frame; a retransmitted last packet counts once; a last packet whose timestamp is no longer the current frame's gives no sample; a lost last packet gives no sample and no failure; a wrong tag gives no sample; a frame number no longer in `SendTimes` gives no sample; a rebased timestamp changes nothing; `LatencyLookups` still counts every frame, found or not
- [X] T009 Implement reading in `src/media/video_quality.go`: stop looking up `ts` in `newFrame` (keep `lookups++`); in `UpdatePacket`, after `updateLocked` and also for packets it treats as old, apply the sample rule with `q.sendTimes.Tag()`, using `q.lastFrame` and `q.lastTS`; update the field and `Update` comments
- [X] T010 Update `src/media/quality_bench_test.go` so `BenchmarkQualityVideoPacket` runs with send times and trailers, and check under 200 ns per packet, 0 allocations (006 R10)
- [X] T011 **Gate, browser decoding (SC-008, research R5)**: with bots sending from this build, watch for 1 minute in Chrome and Firefox on `localhost`, on Jitsi's web client (`https://localhost:8443/<room>`, with and without simulcast) and on Janus's demo page ([quickstart](quickstart.md) step 4). Record the result in `specs/012-video-frame-latency/validation-report.md`. Ask the user to watch if no browser tool is available. **If the picture or console shows a problem, stop and tell the user: the spec must be revisited**

**Checkpoint**: The mechanism works in tests and in a browser.

---

## Phase 3: User Story 1 - Video latency on Janus (Priority: P1) 🎯 MVP

**Goal**: A Janus run reports video latency.

**Independent Test**: 10 bots on the local Janus server for 1 minute: video latency available with percentiles, no reason.

- [X] T012 [US1] Write `TestPeer_LatencyThroughRewrite` in `src/execution/quality_test.go`, beside `TestPeer_LatencyThroughDelay`: subscribe a `TrackWriter` to the source that restarts timestamps from 0 and shifts sequence numbers, as Janus does, over the same 50 ms `vnet` delay. Median video latency within 5 ms of 50 ms (SC-003)
- [X] T013 [US1] In `qualityChecks` in `tests/live/jitsi_live_test.go` (shared by the live tests), when video is checked, fail unless `q.Video.LatencyAvailable`, and check that every bot that received video contributes
- [X] T014 [US1] Run [quickstart](quickstart.md) step 2 with 10 bots on a new `JANUS_ROOM`; record video latency, and audio still `frame_not_identified` (FR-012), against SC-001 in `specs/012-video-frame-latency/validation-report.md`

**Checkpoint**: Janus reports video latency.

---

## Phase 4: User Story 2 - Video latency on Jitsi with simulcast (Priority: P2)

**Goal**: A Jitsi run with simulcast reports video latency.

**Independent Test**: 10 Jitsi bots, simulcast 720, cap, 1 minute: video latency available.

- [X] T015 [US2] In `src/media/video_quality_test.go`, write `TestVideoQuality_LatencyAcrossLayerSwitch`: a stream switches layer mid-way, with a timestamp jump and frames of a different size, and frame numbers continuing: both layers' frames give samples, with no outlier at the switch (FR-004, spec US2 scenario 2)
- [X] T016 [US2] Run [quickstart](quickstart.md) step 3 with 10 bots, `JITSI_SIMULCAST=720`; record video and audio latency and the resolution split against SC-002 in `specs/012-video-frame-latency/validation-report.md`. If the bots stay on one layer, repeat with `JITSI_REQUESTED_HEIGHTS` so that both a low and a high layer are received

**Checkpoint**: Jitsi with simulcast reports video latency.

---

## Phase 5: User Story 3 - Trustworthy figures everywhere (Priority: P3)

**Goal**: No wrong value and no regression on any platform.

**Independent Test**: Automated negative tests, plus alternated runs against the pre-012 baseline.

- [X] T017 [US3] In `src/execution/quality_test.go`, write `TestPeer_NoLatencyForForeignFrames`: a bot receives video from a source of another scenario (other tag) and from a stream without trailers: `latency_available` false, reason `frame_not_identified` (SC-004, FR-006, FR-008). Add a case in `src/media/video_quality_test.go`: the same picture sent in a later content loop matches only its own frame number
- [X] T018 [US3] Confirm `TestPeer_LatencyThroughDelay` and `TestMediaCollector_LatencyReason` pass unchanged in `src/execution/quality_test.go` and `src/execution/quality_collector_test.go`: the reasons of the summary do not move (FR-007, FR-008)
- [X] T019 [US3] Run 10-bot live tests on LiveKit and mediasoup (`tests/live/livekit_live_test.go`, `tests/live/mediasoup_live_test.go`; T013's check applies): both report video latency. Record in `specs/012-video-frame-latency/validation-report.md`. A server that drops the trailer is a finding for the user, not something to work around
- [X] T020 [US3] Build the T001 baseline in a separate worktree. Alternate pre-012 and 012 runs at 50 bots on Jitsi without simulcast, LiveKit and mediasoup: latency available wherever it was, median within 2 ms, freezes, frame rate and MOS within run-to-run variation (SC-005). Record in `specs/012-video-frame-latency/validation-report.md`
- [X] T021 [US3] Alternate pre-012 and 012 runs at 100 bots, cap 10, in the same machine state: engine CPU +5% at most (SC-006). Record in `specs/012-video-frame-latency/validation-report.md`
- [X] T022 [P] [US3] Compute the lowest layer's bitrate with and without the trailer from `src/media/assets/video_90.ivf`, from a run's bytes sent or the frame sizes: +2% at most (SC-007). Record in `specs/012-video-frame-latency/validation-report.md`

**Checkpoint**: All figures can be trusted side by side.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T023 [P] Update `README.md`: the text near lines 276 and 287 (latency from the frame trailer, available with simulcast), near line 330 (LiveKit) and near line 405; keep past measured tables as they are; state that Janus audio latency stays unavailable
- [X] T024 [P] Update comments in `src/media/doc.go` and on `initialRTPTimestamp` in `src/media/source.go`: the 2^30 start now matters for audio only, because video no longer uses timestamps for latency; its value stays
- [X] T025 [P] Add `012-video-frame-latency` to Recent Changes in `CLAUDE.md`
- [X] T026 Run the whole [quickstart](quickstart.md) and `go vet ./... && go test -race ./...`; finish `specs/012-video-frame-latency/validation-report.md` with each SC as met or not met

---

## Dependencies & Execution Order

- **Setup (T001)**: first.
- **Foundational (T002–T011)**: blocks everything. Within it, each test comes before its code: T002→T003, T004→T005, T006→T007, T008→T009; then T010, then the gate T011.
- **US1 (T012–T014)**: after T011. T013 before T014.
- **US2 (T015–T016)**: after T011; uses T013's check.
- **US3 (T017–T022)**: after T011. T019–T021 use T013's check, and are best done after US1 and US2 show the mechanism is sound.
- **Polish**: last. T026 closes the feature.

### Parallel opportunities

T002 and T004 are independent. T006 and T008 touch different test files, but need T003 and T005 to compile. T023, T024 and T025 are independent. T022 can run at any time after T007. Live runs never overlap: they share the machine, and their CPU figures would be meaningless.

```text
Foundational tests: T002 (frame_trailer_test.go), T004 (latency_test.go)
Polish:             T023 (README.md), T024 (doc.go, source.go), T025 (CLAUDE.md)
```

## Implementation Strategy

**MVP (through T014)**: the mechanism, the browser gate, and Janus reporting video latency.

**Then**: US2 needs almost no code, only proof. US3 is mostly long alternated runs, so do them in one sitting in the same machine state.

**Stop conditions**: T011 failing, or T019 finding a server that drops the trailer, comes back to the user.
