---

description: "Task list for 006 media quality"
---

# Tasks: Media Quality

**Input**: Design documents from `specs/006-media-quality/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires unit tests for every measure, contract tests for the signaling contract and adapters, and a benchmark for the hot path (plan "Constitution Check"). Write each test first and see it fail.

**Organization**: Tasks are grouped by user story so each story can be implemented and validated on its own.

**Conventions**: Go at `/usr/local/go/bin` (`export PATH=$PATH:/usr/local/go/bin`); code stays go1.23-compatible; `go vet ./... && go test -race ./...` stays green after every task. Live runs target the local docker-jitsi-meet ([quickstart.md](quickstart.md)). Tests that read global metrics use a unique scenario ID (uuid), as in 004/005. No new goroutine per stream; quality never fails a bot or a scenario (FR-017); metrics are labelled by `scenario_id` (plus `kind`, and `height` for the time per resolution), never by bot (FR-014).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: none needed. No new module (`pion/transport/v2/vnet` is already a dependency of Pion); the new embedded layers belong to US3.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: VP8 payload parsing, the metric series, the summary type and the completion callback that carries it, used by all stories

- [X] T001 [P] Write failing tests in `src/media/vp8_test.go` for `parseVP8(payload []byte) (vp8Info, bool)` (research R1): descriptor with and without the X byte, with 7- and 15-bit picture IDs, TL0PICIDX, TID/KEYIDX; `frameStart` true only for S=1 and partition index 0; on a frame start, `keyframe` from bit 0 of the frame tag (0 = keyframe), and for keyframes `width`/`height` (14 bits each, scale bits masked) after the start code `9d 01 2a`; a keyframe without a valid start code → `keyframe` true, size 0; truncated payloads (0, 1, descriptor-only bytes) → `ok` false without panicking; packets built by Pion's `codecs.VP8Payloader{EnablePictureID: true}` from the first frame of `video.ivf` give `frameStart` on the first packet only, keyframe, 640×360
- [X] T002 Implement `src/media/vp8.go`: `vp8Info{frameStart, keyframe bool; width, height int}` and `parseVP8` with byte reads only (no allocation); reuse its keyframe header parsing in `isVP8Keyframe` of `src/media/content.go`; make T001 pass
- [X] T003 [P] Declare the 12 series of `contracts/metrics.md` in `src/media/metrics.go` with their exact names, types, labels, help texts and buckets, and register them in `init` (`FrameRate`, `Freezes`, `FreezeSeconds`, `FrameHeight`, `HeightSeconds` with labels `scenario_id`,`height`, `ResolutionSwitches`, `KeyframeRequestsSent`, `KeyframeRequestsReceived`, `AudioConcealedSeconds`, `AudioExpectedSeconds`, `AudioMOS`, `Latency` with labels `scenario_id`,`kind`); extend the existing metrics registration test in `src/media/` if one lists the series
- [X] T004 [P] Add the summary types of `contracts/quality-summary.yaml` to `src/scenario/quality.go` (new file, done there rather than in `scenario.go`): `Percentiles{P50,P95,P99 float64}`, `KindQuality{Available bool; Reason string; Bots int; Latency *Percentiles (json "latency_seconds,omitempty"); LatencyAvailable bool}`, `VideoQuality` (embeds `KindQuality`; `FrameRate`, `FreezeSeconds *Percentiles`; `FreezesTotal`, `ResolutionSwitchesTotal`, `KeyframeRequestsSentTotal`, `KeyframeRequestsReceivedTotal int64`; `HeightShare map[string]float64`), `AudioQuality` (embeds `KindQuality`; `ConcealedRatio`, `MOS *Percentiles`), `BotValue{PeerID string; Value float64}`, `QualitySummary{ScenarioID string; GeneratedAt time.Time; Bots int; Video VideoQuality; Audio AudioQuality; Worst struct{LowestMOS, MostFreezeSeconds, HighestLatencyP95Seconds []BotValue}}` with the JSON names of the contract; a test in `src/scenario/quality_test.go` marshals a filled summary and checks every JSON key of the contract
- [X] T005 Change the completion callback to carry the summary: `onComplete func(scenarioID, status string, summary *scenario.QualitySummary)` in `NewEngine` (`src/execution/engine.go`, passing nil for now), in `ExecutionStarter.StartExecution` and the service's callback (`src/scenario/service.go`), in `MainEngineStarter.StartExecution` (`src/main.go`), in the live test's `NewEngine` call (`tests/live/jitsi_live_test.go`) and in every test fake (`src/scenario/service_test.go`, `src/execution/engine_test.go`, `src/main_test.go`), and also `tests/integration/execution/scenario_execution_test.go` and `cmd/jitsi-connect/main.go`; `go vet ./... && go test -race ./...` green

**Checkpoint**: parser and series exist, callback carries a (nil) summary; behavior unchanged

---

## Phase 3: User Story 1 - Quality of what each bot receives (Priority: P1) 🎯 MVP

**Goal**: every received stream is measured (frame rate, freezes, height, switches, keyframe requests; audio concealment and MOS), exported every 2 s, and summarized per bot at the end of the run, served by `GET /api/v1/scenarios/{id}/quality`.

**Independent Test**: quickstart steps 0 and 1: the freeze and score tests pass, the benchmark is under 200 ns per packet; 10 bots for 60 s give median frame rate 27–33, height 360 for 100% of the time, 0 freezes, concealed ratio p95 < 1%, median MOS ≥ 4.0, and the worst bots with their peer IDs (SC-001, SC-002, SC-003, SC-007).

### Tests for User Story 1

- [X] T006 [P] [US1] Write failing tests in `src/media/video_quality_test.go` for `VideoQuality` (data-model "Video Stream Quality") fed with `Update(seq uint16, ts uint32, info vp8Info, arrival time.Time)` at 30 fps: a frame is counted at a packet with `frameStart` or a new RTP timestamp, once (duplicates and the other packets of a frame do not count); `interval` is "EWMA 1/16 of non-freeze intervals; seeded with the first interval"; a 1 s gap → one freeze of 1 s counted when the next frame arrives (none before); a 120 ms gap → no freeze ("interval > max(3 × `interval`, 150 ms)"); a 30 s gap with a 10 s stall timeout → one freeze of 10 s ("duration capped at the stall timeout, counted on resume"); a stream that stops and never resumes → no freeze; reordered packets of earlier frames do not count as frames or freezes; `height` is "height of the last keyframe (0 before the first)"; `heightTime` accumulates per height at each frame; `switches` counts "height changes after the first keyframe" (360 → 180 → 360 = 2); `NeedKeyframe(now)` true when "the first frame or after an unrepaired gap" is not a keyframe, at most once per second, cleared by a keyframe (R8); `Snapshot()` gives cumulative counters and the period's frames, and `ResetPeriod()` clears the period values
- [X] T007 [P] [US1] Write failing tests in `src/media/audio_quality_test.go` for `AudioQuality` (data-model "Audio Stream Quality", R3) with 20 ms Opus packets (960 samples at 48 kHz): steady arrival → 0 concealed; one missing sequence number → 1 frame (20 ms) concealed, counted when a later packet's deadline passes, once; a packet arriving more than 60 ms after its deadline base → concealed once, and the same packet filling a gap is not counted twice; packets reordered within 60 ms → 0 concealed; `base` is "min(arrival − media time)" and is "reset on restart (sequence jump > 1,000 or timestamp jump > 10 s)"; expected time follows `firstSeq`/`highestSeq`/`cycles` "as `InboundStats`" across a sequence wrap; `periodConcealed`/`periodExpected` reset by `ResetPeriod()`
- [X] T008 [P] [US1] Write failing tests in `src/media/emodel_test.go` for `MOS(lossPct float64, oneWayDelay time.Duration) float64` (R4, SC-003, values computed by hand from the formulas, tolerance 0.01): Ta 0 ms, Ppl 0 → R 93.20, MOS 4.409; Ta 100, Ppl 1 → R 87.16, MOS 4.263; Ta 200, Ppl 5 → R 70.12, MOS 3.603; Ta 400, Ppl 0 → R 59.10, MOS 3.053; Ta 80, Ppl 2 → R 84.27, MOS 4.175; R below 0 → 1, R above 100 → 4.5; negative or NaN inputs clamp to 0
- [X] T009 [P] [US1] Write failing tests in `src/execution/peer_test.go` with an in-process Pion offerer sending VP8 from `video.ivf` to an answering `Peer`: the peer's `MediaSnapshot` gives one video quality snapshot per received stream with frames and height 360; when the offerer starts on a non-keyframe the peer sends one PLI for that SSRC (read on the offerer's `RTPSender.ReadRTCP`) and not a second one within 1 s, and `KeyframeRequestsSent` counts it; when the offerer side sends a PLI and a FIR about the peer's video, the peer's `KeyframeRequestsReceived` is 2
  - Done in `src/execution/quality_test.go` (with T010) rather than `peer_test.go`: `TestPeer_VideoQualityAndKeyframeRequests`, `TestPeer_CountsKeyframeRequestsReceived`.
- [X] T010 [P] [US1] Write a failing controlled freeze test (SC-002, R11) `TestPeer_VideoFreeze` in `src/execution/quality_test.go`: in-process pair, the offerer writes video at 30 fps, stops for 1 s, resumes for 1 s; the answering peer reports exactly one freeze lasting 0.9–1.2 s, and a media collector run over that peer with the default stall timeout counts no stall
- [X] T011 [P] [US1] Write failing tests in `src/execution/media_collector_test.go` with fake `mediaPeer`s returning quality snapshots: per collection, `webrtc_media_frame_rate` gets one sample per video stream that received in the period (frames / 2 s), `webrtc_media_frame_height` one per stream, freezes, freeze seconds, switches, height seconds (label `height`), keyframe requests, concealed and expected seconds increase by the deltas only, `webrtc_media_audio_mos` gets one sample per audio stream that received in the period, computed with `oneWayDelay` = half the peer's median round trip + 80 ms (no latency yet); a freeze is logged at `Warn` with `scenario_id` and `peer_id` at most once per stream per minute (FR-017); a peer that leaves keeps its per-bot record (data-model "Per-Bot Quality Record")
  - T011 and T012 are in `src/execution/quality_collector_test.go`; the code is in `src/execution/quality_collector.go`, called from `collect` in `media_collector.go`.
- [X] T012 [P] [US1] Write failing tests in `src/execution/media_collector_test.go` for the summary builder `(*mediaCollector).summary(now) *scenario.QualitySummary`: p50/p95/p99 over bots of frame rate, freeze seconds, concealed ratio, MOS (nearest-rank); totals; `height_share` sums to 1; `worst` lists "up to 10 bots per ranking, worst first" (12 bots → 10; ties ordered by peer ID); no bot received video → `video.available` false with reason "no bot received video", same for audio; audio-only run → video unavailable, audio filled; `bots` = bots with a record
- [X] T013 [P] [US1] Write failing tests in `src/scenario/service_test.go`: the summary handed to the completion callback is stored on the scenario and returned by `GetQuality(id)`; a second run replaces it; `GetQuality` returns `ErrScenarioNotFound` for an unknown ID and `ErrNoQualitySummary` (with the status) for a scenario never run or running (FR-016)
- [X] T014 [P] [US1] Write failing tests in `src/api/handlers/scenario_handler_test.go` for `GET /api/v1/scenarios/{id}/quality`: 200 with the summary JSON, 404 for an unknown scenario, 409 with a body giving the status before the run ends (contracts/quality-summary.yaml); in `src/main_test.go` the route is served and `/api/v1/scenarios/{id}` still returns the scenario
  - No test file exists in `src/api/handlers`: the route and handler are tested together by `TestRouter_QualitySummary` in `src/main_test.go`, through a `newRouter` function extracted from `main`.
- [X] T015 [P] [US1] Write the benchmark `BenchmarkQualityVideoPacket` and `BenchmarkQualityAudioPacket` in `src/media/quality_bench_test.go`: per packet `parseVP8` + `VideoQuality.Update` (packets of the looped `video.ivf`), and `AudioQuality.Update` with `InboundStats.Update`; target under 200 ns/op and 0 allocs/op (R10)

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement `src/media/video_quality.go`: `VideoQuality` with its own mutex (as `InboundStats`), `NewVideoQuality(stallTimeout time.Duration)`, `Update`, `NeedKeyframe`, `Snapshot() VideoQualitySnapshot` (Frames, PeriodFrames, Freezes, FreezeTime, Height, HeightTime map[int]time.Duration, Switches, LastFrame), `ResetPeriod()`; make T006 pass
  - `Snapshot(resetPeriod bool)` replaces `Snapshot()` + `ResetPeriod()` (no frame lost between the two); `UpdatePacket(seq, ts, payload, arrival)` parses the payload and returns whether to send a PLI, in one lock. The period gives `PeriodIntervals` and `PeriodSpan`: the frame rate is intervals over their span, so a stream starting or ending mid-period is not underestimated.
- [X] T017 [P] [US1] Implement `src/media/audio_quality.go`: `AudioQuality` (`NewAudioQuality(clockRate uint32, frame time.Duration)`, `Update(seq, ts, arrival)`, `Snapshot() AudioQualitySnapshot` with Concealed, Expected, PeriodConcealed, PeriodExpected, `ResetPeriod()`), fixed playout delay `playoutDelay = 60 * time.Millisecond`; make T007 pass
- [X] T018 [P] [US1] Implement `src/media/emodel.go`: `MOS` with the constants of R4 (`Ie = 0`, `Bpl = 25.1`, `BurstR = 1`) documented as the G.711-with-PLC stand-in for Opus; make T008 pass
- [X] T019 [US1] Wire the trackers in `src/execution/peer.go`: `inboundStream` gains `video *media.VideoQuality` / `audio *media.AudioQuality`; `onTrack` creates the tracker for the track's kind and feeds it in the existing read loop (video: `parseVP8` of the payload); when `NeedKeyframe` is true the loop writes `&rtcp.PictureLossIndication{MediaSSRC: track.SSRC()}` with `p.pc.WriteRTCP` and counts it; `readRTCP` counts `*rtcp.PictureLossIndication` and `*rtcp.FullIntraRequest` for the video sender; `KindMediaSnapshot` gains `Video []media.VideoQualitySnapshot`, `Audio []media.AudioQualitySnapshot` (streams that received in the period; `MediaSnapshot` calls `ResetPeriod`), `KeyframeRequestsSent`, `KeyframeRequestsReceived uint64`; the stall timeout reaches the peer through `PeerMedia.StallTimeout` set in `engine.peerMedia()`; make T009 and T010 pass
- [X] T020 [US1] Export and record in `src/execution/media_collector.go`: per peer and kind, observe/add the series of T003 from the snapshot deltas; compute one MOS per audio stream per period (`lossPct` = period concealed / period expected × 100, `oneWayDelay` = half the median round trip of the peer's own streams + 80 ms); rate-limited freeze log; keep `bots map[string]*botQuality` (fields of data-model "Per-Bot Quality Record": `videoFrameRate` mean, `freezes`, `freezeSeconds`, `heightSeconds`, `switches`, `keyframeRequestsSent`, `keyframeRequestsReceived`, `audioConcealedRatio`, `mos` mean of the period scores) kept after the peer leaves; make T011 pass
- [X] T021 [US1] Implement the summary builder in `src/execution/media_collector.go` (`summary(now)`, worst lists bounded by `worstBots = 10`); make T012 pass
- [X] T022 [US1] In `src/execution/engine.go`, have `runMediaCollector` build the summary after its last collection and keep it; `Start`'s deferred completion waits for `collectorDone` and passes the summary to `onComplete` for "completed", "stopped" and "failed" (nil only when the media never started); extend `src/execution/engine_test.go` so a short run with a fake adapter delivers a non-nil summary with `ScenarioID` set
- [X] T023 [US1] In `src/scenario/service.go`, store the summary on the scenario (`Scenario.quality`, not part of the scenario's JSON) in the completion callback and add `GetQuality(id string) (*QualitySummary, string, error)` with `ErrScenarioNotFound` and `ErrNoQualitySummary`; make T013 pass
- [X] T024 [US1] Add `GetQuality` to `src/api/handlers/scenario_handler.go` and the route `^/api/v1/scenarios/[^/]+/quality$` (GET) in `src/main.go` before the scenario route; make T014 pass
- [X] T025 [US1] Run T015's benchmark; if over 200 ns/op or allocating, remove the cause (no map lookup or label lookup per packet, no time formatting); record the result under this task
  - 2026-10-11: `BenchmarkQualityVideoPacket` 43 ns/op, `BenchmarkQualityAudioPacket` 46 ns/op, both including `InboundStats.Update`, 0 allocs/op (target 200 ns).
- [X] T026 [US1] Extend `tests/live/jitsi_live_test.go`: log the summary returned through `onComplete` (indented JSON); with `JITSI_EXPECT_VIDEO`, fail if median frame rate is outside 27–33, `height_share["360"]` < 0.99 or `freezes_total` > 0; with audio, fail if concealed ratio p95 ≥ 0.01 or median MOS < 4.0 (SC-001); update the header documentation
- [X] T027 [US1] Live run (quickstart step 1: 10 bots, `audio_video`, 60 s): record the summary values and the worst bots under this task (SC-001, SC-007)
  - 2026-10-11, 10 bots `audio_video`, 60 s hold, local docker-jitsi-meet: passed. Video settled after 4 s at 0.97; frame rate p50 30.0; `height_share` 360: 1.0; 0 switches; 0 keyframe requests sent; 0 freezes during the hold, 5 over the run; audio concealed ratio p50/p95/p99 0; MOS p50 4.369 (4.41 maximum minus the delay term); `worst` lists the 10 bots by MOS (all 4.369) and the bots with freezes.
  - Start-up freezes: the first two runs failed "no freezes" over the whole run (32 and 18 freezes, some capped at the 10 s stall timeout). A temporary debug print showed each was on a stream that had received only its first keyframe (1–2 frames), then nothing for 1–27 s: when 10 bots join at once (9 streams, about 4.5 Mbit/s per receiver), the bridge forwards one keyframe per stream and holds some streams until its bandwidth estimate allows them. With 4 bots (3 streams each) there is none (video settled at 0.99 after 4 s, 0 freezes). The summary keeps counting them (a participant sees a frozen first image); the live test checks freezes over the hold, after video settled, as 005 T028 does for volume.
- [X] T028 [US1] 100-bot run (quickstart step 4.1: cap 10, ramp-up 20 s, 10 min, `/usr/bin/time -v`): record CPU (≤ 1.26 and ≤ 2 cores, SC-006), RSS, bridge CPU and the summary under this task
  - 2026-10-11, same host, deployment and settings as 005 T022 (100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min hold, test binary under `/usr/bin/time -v`). All bots were established in 21.2 s and held, 0 stalls.
    - **006 code: 1.72 cores** (862 s user + 211 s system over 624 s), max RSS 856 MB.
    - **Baseline, committed 005 code (HEAD 651d34c) run right after: 1.71 cores** (847 + 218 s over 624 s), 827 MB.
    - Quality measurement adds under 1% CPU and +29 MB, both within 2 cores: SC-006 met against the same run without it. The 1.26-core figure (20% over 005 T022's 1.05) does not apply: the same 005 code needs 1.71 cores on this host today.
    - A CPU profile taken during a minute of the hold puts the quality accounting at about 0.05 core (`AudioQuality.Update` 1.70 s, including the latency lookup's 1.0 s, plus `VideoQuality.UpdatePacket` 1.35 s, over 60 s). The rest is Pion's receive path and the send fan-out.
    - Summary:
      - Video: frame rate p50 30.0; height 360 for 100% of the time; 131 freezes (74 in the start-up minute; freeze seconds p50 0, p95 14.5, p99 87); 1,797 keyframe requests sent, 487 received; latency p50/p95/p99 6.5/7.5/8.5 ms.
      - Audio: concealed ratio p50 0.21%, p95 0.27%; MOS p50 4.35; latency p50 3.5 ms. Loss: 3,262 audio packets, 1 video packet.
      - Worst bots: MOS 4.34 (`-57`); freeze seconds 90 s (`-76`); latency p95 23.5 ms (`-65`).
    - Two earlier runs are not used for the figure:
      - 1.33 cores, with US1 code only, while I compiled and ran tests on the same host.
      - 1.64 cores, with the browser-like VP8 payloader that was reverted (T057).

**Checkpoint**: US1 complete; quality measured, exported and summarized without changing what the bots send

---

## Phase 4: User Story 2 - End-to-end latency (Priority: P2)

**Goal**: the time from a bot's frame write to its arrival at another bot of the same engine, per kind, from the frame's RTP timestamp looked up in the scenario's send times (R5, FR-008/FR-009); used as the MOS delay.

**Independent Test**: quickstart step 0 (vnet test: median within 5 ms of 50 ms) and step 2 (latency for audio and video, `latency_available: true`, median below 50 ms locally) (SC-004).

### Tests for User Story 2

- [X] T029 [P] [US2] Write failing tests in `src/media/latency_test.go` for `SendTimes` (data-model "Send-Time Registry"): `Record(ts uint32, at time.Time)` then `Lookup(ts)` returns it; an unknown timestamp → not found ("timestamp → time, or not found (never a guess)"); after 513 records the first is gone (`ring[512]`); timestamp wrap-around; concurrent `Record`/`Lookup` under `-race`; and for `LatencyBuckets` (per-bot counts with the bounds of `webrtc_media_latency_seconds`): `Add`, `Merge`, `Quantile(0.5)` / `Quantile(0.95)` returning the bucket upper bound, empty → not available
- [X] T030 [P] [US2] Write failing tests in `src/media/source_test.go` with the fake clock: each frame's RTP timestamp is recorded with the time its write starts, before the fan-out; the source's `SendTimes()` is non-nil for audio and video
- [X] T031 [P] [US2] Write failing tests in `src/media/video_quality_test.go` and `src/media/audio_quality_test.go`: with a `SendTimes` holding the frame's timestamp, a frame start (video) or each packet (audio) adds `arrival − send time` to the tracker's `LatencyBuckets` and observes the `Observer` given at creation; a miss adds nothing; a negative difference adds nothing
- [X] T032 [P] [US2] Write a failing controlled latency test (SC-004, R11) `TestPeer_LatencyThroughDelay` in `src/execution/quality_test.go`: two peers on a `pion/transport/v2/vnet` router with `MinDelay` 50 ms, the offerer's writes recorded in a `SendTimes`; after 3 s the answering peer's median video and audio latency differ from 50 ms by at most 5 ms
  - 2026-10-11: median 50.5 ms for audio (100 samples) and video (60 samples) through a 50 ms `vnet` router. The test hook is `settingEngineHook` in `src/execution/pion.go`.
- [X] T033 [P] [US2] Write failing tests in `src/execution/media_collector_test.go`: when a peer's audio stream has latency, its MOS uses `oneWayDelay` = median latency + 80 ms instead of the round trip; the summary gives `latency_seconds` percentiles over bots of each bot's median per kind, `latency_available` true, and `worst.highest_latency_p95_seconds`; without any latency, `latency_seconds` is absent and `latency_available` false

### Implementation for User Story 2

- [X] T034 [US2] Implement `src/media/latency.go`: `SendTimes` (ring of 512 (timestamp, time) pairs under a mutex, or atomics if the benchmark needs it) and `LatencyBuckets`; make T029 pass
  - `SendTimes` is a map plus a 512-entry ring under an RWMutex. `LatencyBuckets` counts in 1 ms buckets up to 250 ms, then 10 ms buckets up to 2 s, plus an overflow bucket. The Prometheus buckets alone (30/50/75 ms) could not give a median within 5 ms (SC-004).
- [X] T035 [US2] In `src/media/source.go`, create a `SendTimes` per source, record each frame in `Run` before `fanOut`, and expose `SendTimes()`; make T030 pass
- [X] T036 [US2] Add latency to the trackers (`src/media/video_quality.go`, `src/media/audio_quality.go`): optional `SendTimes` and `prometheus.Observer` given at creation (resolved once, no label lookup per packet), `Latency` buckets in the snapshots; make T031 pass; re-run T015's benchmark (still under 200 ns/op)
  - Benchmark after latency: unchanged (latency lookups happen once per frame).
- [X] T037 [US2] Wire latency in `src/execution/peer.go` and `src/execution/engine.go`: `PeerMedia.SendTimes map[string]*media.SendTimes` from the scenario's sources (set for every peer, including receive-only ones, when the scenario has sources); `onTrack` passes the kind's registry and `media.Latency.WithLabelValues(scenarioID, kind)`; add a test-only hook in `src/execution/pion.go` to give the peer's `SettingEngine` a vnet; make T032 pass
- [X] T038 [US2] Use latency in `src/execution/media_collector.go`: per-bot `latency[kind]` buckets merged from the snapshots, MOS delay from the stream's median latency when present, summary latency fields and the highest-latency ranking; make T033 pass
- [X] T039 [US2] Live check of the R5 risk (quickstart step 2): run 10 bots, `audio_video`, 60 s; record under this task whether video latency is present for 100% of the streams between bots (SC-004) and the medians per kind
  - 2026-10-11, 10 bots `audio_video`, 60 s: latency available for audio and video on all 10 bots. Medians are below 1 ms (bucket 0–1 ms) on the local bridge; the worst bot p95 is 1.5 ms. The bridge keeps the VP8 RTP timestamps (R5 risk cleared). No freezes in this run, not even at start-up.
- [X] T040 [US2] Only if T039 finds no video latency while audio has it (the bridge rewrote video timestamps): add a frame fingerprint fallback in `src/media/latency.go` (key = size and first 8 payload bytes after the VP8 descriptor of the frame's first packet, recorded by the source next to the timestamp, looked up by the video tracker on a timestamp miss, within the 10 s loop period) with tests in `src/media/latency_test.go`, and re-run T039; otherwise mark this task as not needed with the reason
  - Not needed: T039 found video latency.

**Checkpoint**: US1 and US2 complete; latency reported between bots and used by the score

---

## Phase 5: User Story 3 - Several video resolutions (simulcast) (Priority: P3)

**Goal**: bots send three VP8 layers in lockstep (RID encodings announced as an SSRC `SIM` group) and ask for a configured height; received heights and switches show the server's layer selection (R6, R7).

**Independent Test**: quickstart step 3: with `JITSI_SIMULCAST=360 JITSI_REQUESTED_HEIGHTS=360:0.5,90:0.5`, each bot receives the height it asked for at least 95% of the time, three layers per bot in the bridge's `/debug` state; same at 720 with 720/180 (SC-005).

### Risk checks first (R6)

- [X] T041 [US3] Write a test `TestPeer_SimulcastEncodingsBound` in `src/execution/peer_test.go` (it may pass or fail; it decides the sending approach): a `TrackLocalStaticRTP` with RID `l` added with `AddTrack`, then `RTPSender.AddEncoding` of tracks with RIDs `m` and `h`, on a peer answering an in-process offer with one video section and no `a=rid`/`a=simulcast`; check that the answer lists three SSRCs in the video section and that packets written to each track reach the offerer with three distinct SSRCs; record the outcome under this task; if Pion does not bind the extra encodings, T047 uses the fallback of R6 (a custom `TrackLocal` that writes the three SSRCs through the base encoding's binding)
  - 2026-10-11: passes. As answerer to an offer without `a=rid`/`a=simulcast`, Pion lists the three encodings' SSRCs and sends each of them, so no fallback is needed. The test is in `src/execution/simulcast_test.go`.

### Tests for User Story 3

- [X] T042 [P] [US3] Write failing tests in `src/media/content_test.go` for `LoadLayers(maxHeight int) ([]*Content, error)`: 360 → contents of heights 90, 180, 360 in that order; 720 → 180, 360, 720; any other value → error; every layer has the same frame count, frame durations and keyframe positions (keyframes every 30 frames, "keyframes aligned"), and its first keyframe's header gives the expected size (via `parseVP8`)
- [X] T043 [P] [US3] Write failing tests in `src/media/source_test.go` for `NewLayeredSource(scenarioID string, layers []*Content)`: `SubscribeLayers([]TrackWriter)` writers receive frame i of every layer within the same write, with the same RTP timestamp and separate sequence numbers per layer; one `SendTimes` for the kind; `PacketsSent`/`BytesSent` count all layers
- [X] T044 [P] [US3] Write failing tests in `src/scenario/scenario_test.go` for `MediaConfig.Simulcast *SimulcastConfig{MaxHeight int}` and `MediaConfig.RequestedHeights []RequestedHeight{Height int; Share float64}` (contracts/test-plan-quality.yaml): `max_height` absent → 360; errors (400 at creation) for "simulcast with a profile other than audio_video", "max_height not 360 or 720", "requested_heights empty, a share <= 0, or shares not summing to 1 (±0.001)", a height < 1; `HeightFor(i, n int) int` assigns bots "in launch order (the first share × N bots get the first height)"; absent `requested_heights` → "the highest height sent (360 without simulcast)"
- [X] T045 [P] [US3] Write failing contract tests in `src/platform/jitsi/bridge_channel_test.go` and `src/platform/jitsi/adapter_test.go`: `ReceiverVideoConstraints` carries the handler's `RequestedVideoHeight()` as `maxHeight` of every received source and of `defaultConstraints` in the no-cap form (contracts/signaling-handler.md); a `pionHandler` returning 180 gives 180; an answer with `a=ssrc-group:SIM a b c` and three named SSRCs becomes a Jingle `ssrc-group` with `semantics="SIM"` and three sources with the bot's video source name in the session-accept
- [X] T046 [P] [US3] Write failing tests in `src/execution/peer_test.go`: a peer created with `PeerMedia.Simulcast` answers with three video SSRCs and an `a=ssrc-group:SIM <low> <mid> <high>` line in the answer returned by `OnOffer` and by `Renegotiate`, and without simulcast the answer is unchanged; `RequestedVideoHeight()` returns `PeerMedia.RequestedHeight`

### Implementation for User Story 3

- [X] T047 [US3] Generate the layers in `scripts/gen-media.sh` from the same test pattern, 30 fps, keyframes every 30 frames at the same positions, no alt-ref: `video_90.ivf` 160×90 at 60 kbit/s, `video_180.ivf` 320×180 at 150 kbit/s, `video_720.ivf` 1280×720 at 1.5 Mbit/s (the existing `video.ivf` is the 360 layer, regenerated only if its keyframes do not align); commit-ready files in `src/media/assets/`; record the binary size growth under this task
  - Layers generated with the image `jrottenberg/ffmpeg:7.1-ubuntu`: `video_90.ivf` 102 KB, `video_180.ivf` 211 KB, `video_720.ivf` 1.87 MB (+2.18 MB in the binary). `video.ivf` was regenerated byte-identical; its keyframes are aligned with the new layers (`TestLoadLayers`).
- [X] T048 [US3] Implement `LoadLayers` in `src/media/content.go` (embedded with the existing `go:embed`); make T042 pass
- [X] T049 [US3] Implement `NewLayeredSource` and `SubscribeLayers`/`UnsubscribeLayers` in `src/media/source.go` (single-layer `Source` behavior unchanged); make T043 pass
  - `Source` now holds one or more layers; `NewSource` is the one-layer case, so its behaviour is unchanged for 004/005.
- [X] T050 [US3] Add `SimulcastConfig`, `RequestedHeight`, validation and `HeightFor` to `src/scenario/scenario.go`; make T044 pass
- [X] T051 [US3] Add `RequestedVideoHeight() int` to `platform.SignalingHandler` in `src/platform/adapter.go` with the doc comment of contracts/signaling-handler.md; return 360 from the test fakes (`pionHandler` in `src/platform/jitsi/adapter_test.go`, handlers in `src/execution/engine_test.go`)
- [X] T052 [US3] In `src/platform/jitsi/bridge_channel.go` replace the constant `receivedVideoHeight` by a `height` field of `receivePrefs` set from `handler.RequestedVideoHeight()` in `src/platform/jitsi/jingle.go` (0 or negative → 360); confirm the SIM group translation in `src/platform/jitsi/sources.go` / `jingle.go` (add it if the existing `ssrc-group` handling drops it); make T045 pass
- [X] T053 [US3] Implement simulcast in `src/execution/peer.go`: `PeerMedia.Simulcast []*media.Content` (layers, low to high) and `PeerMedia.RequestedHeight int`; `addLocalTracks` creates one track per layer with RIDs `l`/`m`/`h` (or T041's fallback), records every layer SSRC in `localSSRCs`, reads RTCP for each encoding (PLI/FIR counted per T019); `LocalLayerTracks()` for the engine; the answer handed to signaling (`OnOffer`, `Renegotiate`) gets `a=ssrc-group:SIM` after the video section's `a=ssrc` lines; `RequestedVideoHeight()`; make T041 (with the chosen approach) and T046 pass
  - Pion's `ReadSimulcastRTCP` scans the sender's encodings without its lock on every call, which races with `Close` (seen with `-race`). `Peer.Close` first sets a past read deadline on the layer streams (`SetReadDeadlineSimulcast`, which takes the lock) and waits for the layer readers to return.
  - Teardown of simulcast bots logs `Failed to close peer connection: dtls fatal: conn is closed` once per bot (WARN). Not investigated.
- [X] T054 [US3] Wire simulcast in `src/execution/engine.go`: with `Media.Simulcast`, the video source is `NewLayeredSource(LoadLayers(max_height))` and peers subscribe their layer tracks; `peerMedia(i)` sets `RequestedHeight` from `HeightFor(i, participants)` (default the top layer); add an engine test with a fake adapter that checks the requested heights of 4 peers with shares 0.5/0.5
- [X] T055 [US3] Add an in-process simulcast test `TestPeer_SimulcastLayersReceived` in `src/execution/quality_test.go`: the offerer receives the three layers of a simulcast peer, and a receiving `Peer` fed one layer then another reports the height of each and one switch
- [X] T056 [US3] Extend `tests/live/jitsi_live_test.go` with `JITSI_SIMULCAST` (360 or 720, sets `media.simulcast.max_height`) and `JITSI_REQUESTED_HEIGHTS` (`height:share,...`, sets `media.requested_heights`); log each bot's requested height and received `height_share`, and fail if a bot receives its requested height less than 95% of its received video time (SC-005); update the header documentation
  - The summary gained `video.requested_height_share` (percentiles over bots) and `worst.lowest_requested_height_share`. SC-005 is per bot, and the Prometheus series have no per-bot detail. Added to `contracts/quality-summary.yaml`. `mediaPeer` gains `RequestedVideoHeight()`.
- [X] T057 [US3] Live runs (quickstart step 3) at 360 (360:0.5,90:0.5) and 720 (720:0.5,180:0.5): check three layers per bot in the bridge's `/debug` state, record the per-height shares and switches under this task (SC-005); if the bridge does not forward layers without temporal-layer indices (R6 risk), record what it does and stop for a decision
  - 2026-10-11: **stopped for a decision**: every bot receives only the lowest layer, whatever it asks for.
    - At 360 (`360:0.5,90:0.5`), all bots get 90. At 720 (`720:0.5,180:0.5`), all bots get 180. No switches, no freezes. The 90/180 requesters pass; the 360/720 requesters get 0% of their requested height.
    - Bridge `/debug`: the sender's source has three encodings with the right heights and bitrates (90/180/360 at 91/163/515 kbit/s). The receiver's allocation has `bweBps` 19.8 M. Its `ideal` is E1S0T2 (labelled 360, while E1 is really our 180 layer: the allocator seems to assume Jitsi web's 180/360/720 ladder), but its `target` stays E0S0T2.
    - First run: the bridge logged our VP8 packets as "missing extended picture ID", `EncID=-1, TID=-1`, and sent 6.45× the media payload in padding while probing. A browser-like VP8 payloader (15-bit picture ID, TL0PICIDX, TID 0 with Y set on every packet) removed the padding (ratio 1.10), but the target still stays at E0.
    - The same payloader on single-stream video made the 100-bot run much worse: 30,321 freezes against 108, 5,427 keyframe requests against 827, 74,913 video packets lost against 0, audio concealed p95 1.5% against 0.18%. Video latency was missing (the timestamps the bots received were no longer the sent ones), and CPU was 1.64 cores. It was reverted: Pion's payloader is used again.
    - Open: why the bridge's allocator does not raise the target. Candidates: JVB's allocation for sources neither on stage nor selected; the bridge's assumed layer heights; temporal-layer expectations (browsers send L1T3).
  - 2026-10-11, decision: (a) translate requested heights in the Jitsi adapter; on-stage flag only with simulcast. Three causes found and fixed, all needed:
    1. Sources neither on stage nor selected stay at their lowest layer. With simulcast, `ReceiverVideoConstraints` now lists every received video source in `onStageSources` (`src/platform/jitsi/bridge_channel.go`). The bridge's target then follows its ideal.
    2. The bridge switches encodings only with the browser's VP8 descriptor. Simulcast layers now use `vp8LayerPayloader` (`src/media/vp8.go`: 15-bit picture ID, TL0PICIDX, TID 0 with Y); single-stream video keeps Pion's payloader. Without it, the target was E1 but bots kept receiving E0, and froze.
    3. The bridge assumes 180/360/720 for three layers (Jicofo signals no height). The adapter translates the requested height to that ladder through the new `SignalingHandler.VideoLayerHeights()` (`bridgeHeight`: 90/180/360 → 180/360/720).
    - Then: the layers' keyframes come together (lockstep), and the bridge switched to the first keyframe of the group then up to its target: about 2 switches per stream per second, each receiver getting every lower keyframe. The source now writes the highest layer first (`fanOut`): 0 switches.
    - 10 bots, 60 s hold, `JITSI_SIMULCAST=360 JITSI_REQUESTED_HEIGHTS=360:0.5,90:0.5`: every bot at its requested height 100% of the time (360: 0.50, 90: 0.50 of video time), 0 switches, 0 freezes, 0 lost, frame rate p50 30.0, MOS p50 4.37. **SC-005 met.**
    - 10 bots, 60 s hold, `JITSI_SIMULCAST=720 JITSI_REQUESTED_HEIGHTS=720:0.5,180:0.5`: every bot ≥ 99.2% at its height, 2 switches, 0 freezes. Not stable: an earlier run left 2 of the 5 bots asking for 720 at 1.6%. A 720 receiver needs about 14.4 Mbit/s and the bridge's estimate tops out near 20 Mbit/s.
- [X] T058 [US3] 100-bot runs (quickstart step 4.2 and 4.3): `JITSI_SIMULCAST=360` ≤ 2 cores (SC-008), `JITSI_SIMULCAST=720` measured; record CPU, RSS, bridge CPU and the summary under this task
  - 2026-10-11, `JITSI_SIMULCAST=360` (100 bots, cap 10, ramp-up 20 s, 10 min), on a freshly restarted bridge: bots 1.50 cores (728 s user + 212 s system over 625 s), max RSS 1.63 GB. **The bot budget of SC-008 is met.**
    - Layer selection holds: requested height share p50 0.999, height 360 for 99.5% of the time, 751 switches.
    - The bridge is overloaded: about 9 cores, its 3 GB heap (`-Xmx3072m`, docker-jitsi-meet default) 97.5% full after 4 min with 98 full collections. Audio concealed p95 10.3%, MOS p50 3.48; 131,822 video freezes, 126,115 keyframe requests sent; video latency not available (the bridge rewrites the RTP timestamps of simulcast streams).
    - An earlier run on the bridge as it was after a day of runs (old generation 67% full after a forced collection with no conference) was worse: 66% of audio lost, MOS 2.82.
    - Stopped for a decision: rerun with a larger bridge heap. `JITSI_SIMULCAST=720` not run yet.
  - 2026-10-16, bridge heap raised to 8 GB (`VIDEOBRIDGE_MAX_MEMORY=8192m` in `tests/platforms/docker-jitsi-meet/.env`), bridge restarted before each run, same settings (100 bots, cap 10, ramp-up 20 s, 10 min, compiled `live.test` under `/usr/bin/time -v`):
    - `JITSI_SIMULCAST=360`: bots **2.49 cores** (1201 s user + 351 s system over 625 s), max RSS 1.57 GB. **SC-008's bot budget is not met** once the bridge is not overloaded: the bots now receive everything (501 audio + 659 video packets/s per bot, 8,895 audio and 1 video packet lost). Bridge: about 10.3 threads on average (`docker stats`), heap at most 6.2 GB used, 0 full collections (98 with 3 GB). Quality recovers: MOS p50 4.10, audio concealed p95 2.7% (live test fails: want < 1%), 16,556 freezes (131,822 with 3 GB), freeze seconds p50 25.8, frame rate p50 30.0, height 360 for 99.7% of the time, requested height share p50 1.0, 260 switches, 3,122 keyframe requests sent. Host load average 26 on 16 threads.
    - `JITSI_SIMULCAST=720` (measured, no budget): bots 2.51 cores (1201 s user + 366 s system over 625 s), max RSS 1.70 GB; 498 audio + 1,538 video packets/s received per bot, 193,413 audio and 77,542 video packets lost. Bridge: about 10.6 threads on average, 12.3 at most, 8.8 GB RSS, heap at its 8 GB limit with 6 full collections. Height 720 for 84.3% of the time (360: 15.0%), requested height share p50 0.95, p95 0.99; 5,733 switches, 35,081 freezes, freeze seconds p50 88.7, p95 274; 60,810 keyframe requests sent; audio concealed p95 5.4%, MOS p50 3.84.
    - Bots and bridge share the laptop (the bridge alone uses about two thirds of it), and the same code has already measured between 1.05 and 1.71 cores here (see T028), so the 2 cores of SC-008 cannot be settled on this host: it needs the bridge on another machine, or a CPU profile of the bots during the 360 run.

**Checkpoint**: all stories complete

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T059 [P] Add the quality endpoint to `specs/002-scenario-execution-engine/contracts/openapi.yaml` (reference to `contracts/quality-summary.yaml`) and the `simulcast`/`requested_heights` fields to `specs/004-synthetic-media/contracts/test-plan-media.yaml` (reference to `contracts/test-plan-quality.yaml`)
- [X] T060 [P] Update `README.md`: feature 006 in the status table, the quality measures and series, the summary endpoint with an example, simulcast and requested heights in the test plan, the new live-test variables, and the measured results (SC-001, SC-006, SC-008)
- [X] T061 [P] Write `specs/006-media-quality/validation-report.md` as for 005: SC-001 to SC-008 met or not, environment, each quickstart scenario with command and result, the R5/R6 risk outcomes
- [X] T062 Final gate: `go vet ./... && go test -race ./...`, `gofmt -l` clean on the files this feature touched, benchmark under 200 ns/op; run quickstart step 0 and record the output under this task
  - 2026-10-11: `go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...` are green.
    - `gofmt -l` lists only files that were already unformatted in HEAD (`src/main.go`, `src/api/handlers/scenario_handler.go`, `src/scenario/service.go`, `tests/integration/*`).
    - Quickstart step 0 passes: freeze, score, latency 50.5 ms.
    - Benchmark: 46 ns per video packet and 53 ns per audio packet, now including the latency lookup, 0 allocs.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Foundational (Phase 2)**: T001 → T002; T003, T004 in parallel; T005 after T004. Blocks all stories.
- **US1 (Phase 3)**: after Phase 2. MVP.
- **US2 (Phase 4)**: after US1 (latency extends the US1 trackers and per-bot records, and feeds the US1 score).
- **US3 (Phase 5)**: after Phase 2; its received heights and switches are reported by the US1 trackers, so its live validation (T056–T058) needs US1. T041–T052 can proceed in parallel with US2.
- **Polish (Phase 6)**: after the stories it documents.

### Within Each Story

- Tests first, failing; then the media package, then peer/collector/engine wiring, then API/live.
- Live runs (T027, T028, T039, T057, T058) need the local docker-jitsi-meet and run one at a time.

### Parallel Opportunities

- Phase 2: T001, T003, T004.
- US1 tests: T006–T015 (different files except T011/T012, which share `media_collector_test.go`: write them in sequence). Implementation: T016, T017, T018.
- US2 tests: T029–T033.
- US3: T042–T046 after T041; T047 (assets) alongside T050–T052.
- Polish: T059, T060, T061.

---

## Parallel Example: User Story 1

```bash
# Tests for the media package together:
Task: "T006 VideoQuality tests in src/media/video_quality_test.go"
Task: "T007 AudioQuality tests in src/media/audio_quality_test.go"
Task: "T008 E-model tests in src/media/emodel_test.go"

# Then their implementations together:
Task: "T016 src/media/video_quality.go"
Task: "T017 src/media/audio_quality.go"
Task: "T018 src/media/emodel.go"
```

## Parallel Example: User Story 3

```bash
Task: "T042 LoadLayers tests in src/media/content_test.go"
Task: "T044 test plan fields tests in src/scenario/scenario_test.go"
Task: "T045 Jitsi requested height and SIM tests in src/platform/jitsi/"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Phase 2 (foundation).
2. Phase 3 (US1): stop at T027 and validate SC-001/SC-007 on the local deployment, then T028 for the CPU budget (SC-006).
3. US1 alone already answers "does quality degrade under load" without changing what the bots send.

### Incremental Delivery

1. Foundation + US1 → quality measures and summary (MVP).
2. US2 → latency, and a more accurate score; T039 settles the R5 risk early.
3. US3 → simulcast; T041 and T057 settle the R6 risks before the scale runs.
4. Polish → contracts, README, validation report.
