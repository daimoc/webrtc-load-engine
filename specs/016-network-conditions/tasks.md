---

description: "Task list for 016 network conditions per bot"
---

# Tasks: Network Conditions per Bot

**Input**: Design documents from `specs/016-network-conditions/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/test-plan-network.md](contracts/test-plan-network.md), [contracts/report-network.md](contracts/report-network.md), [quickstart.md](quickstart.md)

**Tests**: Included. The constitution requires tests for core components: write each test first and see it fail. Real-server tests stay behind `-tags live`; unit tests need no server.

**Organization**:
- Phase 2 holds what every story needs: the test plan section, the per-bot shaper with the cap, the socket hook, and the report section's skeleton.
- US1 (slow receiver) is the MVP.
- US2 adds the schedule's reaction and recovery times.
- US3 adds delay, jitter and loss to the same shaper.
- US4 adds adapting senders, after its gate.
- US5 adds the server's estimate and the four-server comparison.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green at the end of each phase.
- **Boundaries** (plan, constitution II and III):
  - no new module in `go.mod`;
  - the shaper and layer policy live in `src/execution` and import no platform package;
  - no adapter reads `network`;
  - `src/media`'s shared `Source` is not changed: layers are dropped in the bot's own `videoTrack`;
  - a run without `network` creates no shaper, registers no sequence number interceptor and no estimator, and writes no `network` section (FR-013, FR-018).
- **Names** (used across tasks):
  - in `src/scenario/network.go`:
    - `NetworkConfig{Seed *uint64 \`json:"seed,omitempty"\`; SettleSeconds *int \`json:"settle_seconds,omitempty"\`; Groups []BotGroup \`json:"groups"\`}`;
    - `BotGroup{Name string; Count int; Share float64; Schedule []ScheduleStep}`;
    - `ScheduleStep{StartSeconds int; DurationSeconds *int; Downlink, Uplink *LinkCondition}`;
    - `LinkCondition{RateKbps int; QueueMs int; DelayMs int; JitterMs int; Loss *Loss}`;
    - `Loss{Share float64; MeanBurst float64}`;
    - JSON names exactly as in [contracts/test-plan-network.md](contracts/test-plan-network.md);
    - constants `DefaultSettleSeconds = 10`, `DefaultQueueMs = 200`;
    - `(*NetworkConfig).Assign(participants int) []string`: the group name per launch index, "" for no group, assigned from the end of the launch order with the first group first;
  - `TestPlan.Network *NetworkConfig \`json:"network,omitempty"\`` in `src/scenario/scenario.go`;
  - in `src/scenario/result.go`: `RunResult.Network *NetworkResult` and the types `NetworkResult`, `GroupResult`, `AppliedStep`, `BotNetwork`, `BotStep`, `DirectionMeasure`, `SenderResult`, `LayerChange`, `EstimateSample`, `ServerEstimate`, with the JSON of [contracts/report-network.md](contracts/report-network.md);
    - reasons `NotApplied = "not_applied"`, `NotAtRequestedHeight = "not_at_requested_height"`, `NoVideo = "no_video"`, `NotRecoveredBeforeEnd = "not_recovered_before_end"`, `NoUplinkCondition = "no_uplink_condition"`, `CannotAdapt = "cannot_adapt"`, `NoFeedback = "no_feedback"`, `ReceiveOnly = "receive_only"`, `NotGivenByServer = "not_given_by_server"`;
  - in `src/execution/netcond.go`:
    - `type shaper struct{down, up *linkShaper}`;
    - `newLinkShaper(seed uint64, clock func() time.Time) *linkShaper`;
    - `(*linkShaper).setCondition(step int, c *scenario.LinkCondition)` (nil: pass-through);
    - `(*linkShaper).counters() map[int]stepCounters`;
    - variables `shaperNow = time.Now` for tests;
  - `report.FormatVersion = 4`;
  - `platform.ReceiverEstimator` interface in `src/platform/adapter.go`: `ReceiverEstimate(ctx context.Context, peerID string) (kbps int, ok bool, err error)`;
  - live test variable `<PLATFORM>_NETWORK=<file.json>`; CLI flag `--network <file.json>`.
- **Live runs**:
  - Local servers in `tests/platforms`, one at a time. Stop only what you started.
  - Never `pkill -f`: kill by PID. Never edit Go files while background live runs build.
  - mediasoup runs use `ramp_up_seconds` 20 for 10 bots (015). Jitsi: one warm-up run after a bridge start, set aside.
  - Keep the report files of every comparison run under `specs/016-network-conditions/runs/`, logs compressed with xz.
- **Gates**:
  - T004 (TWCC feedback to publishers per server) before Phase 6. A server without it is recorded as `no_feedback`.
  - A gate that fails, or a fact that changes the spec, stops the work and goes to the user.
- **Commits**: only when the user asks.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Setup

- [X] T001 Record the CPU baseline before any 016 code: a worktree at `70c7b23` (`../wle-before-016`), the last commit without 016 code. Its runs are alternated with the 016 build in T044, under the 015 conditions (100 bots, LiveKit, VP8, cap 10, 20 s ramp-up, 2 min hold), because the laptop's CPU varies with its state (015 SC-008). Method in `validation-report.md` "Baseline".
- [X] T002 [P] Create `specs/016-network-conditions/validation-report.md` with one section per success criterion SC-001…SC-009, each marked "not checked", plus "Baseline" and "Gate T004".
- [X] T003 [P] Check the Pion facts research R7 relies on, in `~/go/pkg/mod/github.com/pion/interceptor@v0.1.29/pkg/gcc` and `pkg/twcc`:
  - `NewSendSideBWE` accepts `SendSideBWEPacer(gcc.NewNoOpPacer())` and `SendSideBWEInitialBitrate`;
  - `OnTargetBitrateChange` fires;
  - the increase is capped at 1.5 times the received rate;
  - `twcc.NewHeaderExtensionInterceptor` writes sequence numbers on every outgoing RTP packet of a session that negotiated the extension.

  Write the findings under research R7 ("verified"), or stop and report if one fails.
- [X] T004 Gate (research R7): for each local server, check whether it sends transport-wide feedback to a publisher whose packets carry the extension. Use a throwaway branch of `src/execution/pion.go` that registers `twcc.NewHeaderExtensionInterceptor` and the transport-cc extension on the sending session, and log the RTCP `TransportLayerCC` packets the session reads.
  - Run 2 bots, VP8, simulcast 720, for 20 s on each of Jitsi, LiveKit, mediasoup and Janus.
  - For mediasoup, check that `src/platform/mediasoup/sdp.go` carries the extension into the producer's `rtpParameters.headerExtensions`, and add it if the router's capabilities list it.
  - Record per server "feedback: yes/no, interval" in "Gate T004" of the validation report.
  - Revert the throwaway change, except the mediasoup mapping if it was needed, with its test in `src/platform/mediasoup/sdp_test.go`.

---

## Phase 2: Foundational (blocking all stories)

- [X] T005 [P] Write `src/scenario/network_test.go`. It covers every rule of [contracts/test-plan-network.md](contracts/test-plan-network.md), each one refused with a message naming the field:
  - "at least one group; group names unique; at most 16 groups";
  - name `[a-z0-9_-]{1,32}`;
  - "exactly one of count (1..participants) or share (0 < share ≤ 1)";
  - "1–32 steps, ordered, not overlapping";
  - `start_seconds` ≥ 0 and a step ending "before or at the end of the run (`ramp_up_seconds + duration_seconds`)";
  - `duration_seconds` ≥ 1;
  - "A step has at least one of `downlink`, `uplink`, and each given direction has at least one field";
  - `rate_kbps` 16–1000000;
  - `queue_ms` 10–2000, "only with rate_kbps";
  - `delay_ms` 0–5000;
  - `jitter_ms` 0..delay_ms;
  - loss `share` 0–1 and `mean_burst` ≥ 1;
  - `settle_seconds` 0–300;
  - groups' bots added together ≤ `participants`, with `share` giving `round(share × participants)`, at least 1;
  - `uplink` refused on `receive_only`;
  - unknown fields refused.

  It also checks the defaults (`settle_seconds` 10, `queue_ms` 200 when a rate is set, `mean_burst` 1) and `Assign` (10 bots, `slow` count 3, then `x` share 0.2 → launch indexes 7–9 `slow`, 5–6 `x`, others "").
- [X] T006 Implement `src/scenario/network.go` (types, `withDefaults` validation, `Assign`) and wire `TestPlan.Network` into the test plan validation in `src/scenario/scenario.go`, strict JSON as 015's `browser`. Make T005 pass.
- [X] T007 [P] Write `src/execution/netcond_test.go` for the rate cap (research R2). Send datagrams from one local UDP socket through a `linkShaper` to another, with a fake clock where possible and real time for the end-to-end case. Check:
  - over 10 s at 500 kbit/s offered 1 Mbit/s, the delivered rate is within 10% of 500 kbit/s in every 10-second window (SC-001);
  - each datagram is counted with 28 extra bytes;
  - the queue never holds more than `queue_ms` of data at the cap rate, and overflow is counted as dropped by the cap;
  - `setCondition(step, nil)` passes datagrams straight through with no added delay;
  - counters are kept per step index.
- [X] T008 Implement the cap part of `src/execution/netcond.go`: token bucket (burst one 1500-byte MTU), byte-bounded FIFO, one delivery goroutine and timer per `linkShaper`, `stepCounters` (offered and delivered packets and bytes, dropped by cap, delay sum and max, delivered bytes per second), and `close()`. Make T007 pass with `-race`.
- [X] T009 [P] Write a test in `src/execution/consent_test.go`: a `consentConn` whose `consentNet` has a `shaper` delays its reads through `shaper.down` and its writes through `shaper.up`. The arrival handed to `transportFeedback.arrive` carries the time the datagram leaves the shaper, not the time the socket read it (research R1). A `consentNet` without a shaper behaves as before.
- [X] T010 Hook the shaper into `src/execution/consent.go`. `consentNet` gains `shaper *shaper`. `consentConn.ReadFrom`/`ReadFromUDP` read the socket through the down shaper: one reader goroutine per shaped socket feeds the shaper, and reads return delivered datagrams. `WriteTo` goes through the up shaper. Consent answers are filtered after shaping. Make T009 pass.
- [X] T011 Give each bot one shaper shared by all its sessions, in `src/execution/peer.go` and `src/execution/pion.go`. `PeerMedia`/peer options gain the bot's group condition source. `newPeerConnection` sets `c.net.shaper` on the receive session and on the publish session. Bots in no group get nil. Add a test in `src/execution/peer_test.go` that both sessions of a publish-session peer share one shaper.
- [X] T012 [P] Add the result types listed under Names to `src/scenario/result.go`, and set `report.FormatVersion = 4` in `src/report/run.go` with `Run.Network`. Extend `src/report/run_test.go` so that a run without `network` has no `network` key, and a run with one round-trips the contract's example.

**Checkpoint**: plans with `network` are accepted or refused, a constrained bot's datagrams are capped in both directions, and the report can carry the section.

---

## Phase 3: User Story 1 - A receiver with a slow link (Priority: P1) 🎯 MVP

**Goal**: cap the downlink of a group, and report the groups, the measured conditions and each group's quality.

**Independent Test**: quickstart §2 on each local server (10 bots, layered VP8, all asking for 720, 1200 kbit/s toward 3 bots: 500 kbit/s is below four 180-line streams, R14 in the validation report).

- [X] T013 [P] [US1] Write `src/execution/schedule_test.go`:
  - a group's steps are applied to its bots' shapers at `start_seconds` from the run start, and removed at the end of `duration_seconds`, or kept to the end when absent;
  - a bot joining during a step gets that step at once;
  - `during_start_up` is true for a step that starts before the last join plus `settle_seconds` (FR-011);
  - actual start and end times are recorded.

  Use a fake clock.
- [X] T014 [US1] Implement `src/execution/schedule.go` (step timers per group, the time reference being the first launch, actual times, settling flag), and start it from `Engine.Start` in `src/execution/engine.go`. The engine creates the shapers from `NetworkConfig.Assign`, seeds them as research R4 says (the seed from the plan, or drawn with `crypto/rand`), and stops them in `stop()`. Make T013 pass. Log each step change with the group, the step and the condition.
- [X] T015 [P] [US1] Write tests in `src/execution/engine_test.go` with a fake adapter, 4 bots and one group of 2 with a downlink cap:
  - `RunResult.Network` lists the groups, the bots per group with their requested height, and the measured `downlink` per step;
  - `delivered_kbps` mean and `max_10s` come from the counters, never from the plan;
  - a bot that was never connected during a step gives reason `not_applied`.
- [X] T016 [US1] Build `RunResult.Network` in `src/execution/engine.go`, in a helper in `src/execution/adaptation.go`: the measured direction figures from `stepCounters` (shares with a `null` and reason when nothing was offered), seed and `seed_source`, `settle_seconds`, `timeline_resolution_seconds` 1. Make T015 pass.
- [X] T017 [P] [US1] Write a test in `src/execution/quality_collector_test.go`: `summarize` over a subset of bots gives the same figures as a run of only those bots. Then per-group `quality` and `unconstrained_quality` are filled for a run with groups.
- [X] T018 [US1] Add a subset summary to `src/execution/quality_collector.go` (the existing summary code over a given set of peer ids), and use it for `GroupResult.Quality` and `NetworkResult.UnconstrainedQuality`. Make T017 pass.
- [X] T019 [P] [US1] Write `src/report/markdown_016_test.go`. The "Network conditions" section has the groups and steps table (planned vs measured rate, drops, mean delay, `during_start_up` flag) and the group quality rows beside the unconstrained row (height shares, requested-height share, freezes, score). With no `network`, there is no section.
- [X] T020 [US1] Implement `src/report/markdown_016.go` and call it from the Markdown writer in `src/report`. Make T019 pass.
- [X] T021 [P] [US1] Add `--network <file.json>` to `cmd/jitsi-connect` (reads the `network` object, strict JSON), and a `networkFrom(t, prefix)` helper in `tests/live/report_test.go` reading `<PLATFORM>_NETWORK`, used by the Jitsi, LiveKit, mediasoup and Janus live tests in `tests/live/`.
- [X] T048 [US1] REMB where the server asks for it (research R14, FR-019), test first in `src/execution/remb_test.go`:
  - parsing of `abs-send-time` (24-bit 6.18 fixed point, wrap-around);
  - grouping by 5 ms send bursts;
  - the trendline detecting overuse on a synthetic growing delay and not on a flat one;
  - AIMD: +8%/s capped at 1.5 × incoming, 0.85 × incoming on overuse;
  - a REMB sent every second and at once on a fall of 3% or more, naming the received SSRCs.

  Implement it in `src/execution/remb.go`.
- [X] T049 [US1] Register `abs-send-time` and `goog-remb` on every receive session in `src/execution/pion.go`. Feed the estimator from `consentConn.arrived` in `src/execution/consent.go`. Enable it only when the applied offer negotiates `goog-remb` without `transport-cc`, read with the `transportFeedback.setExtension` code in `src/execution/twcc.go`. Send the REMB with the receive session's `WriteRTCP`. Add a test in `src/execution/peer_test.go`: a LiveKit-like offer (goog-remb, abs-send-time, no transport-cc) turns it on, a Jitsi-like one does not.
- [X] T050 [US1] Live: LiveKit with no condition, the 014 T059 plan (10 bots, layers up to 720, cap 4, `180:0.5,720:0.5`, 1 min), VP8 and one SVC codec. The lowest bot's share at its requested height must stay within 0.02 of 014's table (SC-010). Then quickstart §2 on LiveKit: the capped bots receive lower layers. Record both in the validation report, under "R14".
- [X] T022 [US1] Live: quickstart §2 on the four local servers, one run each. Check SC-002 and SC-003, and the measured rate against SC-001. Record the tables in the validation report and keep the reports in `specs/016-network-conditions/runs/us1/`.

**Checkpoint**: US1 works alone. A capped group is visible in both report files with measured figures and its own quality rows.

---

## Phase 4: User Story 2 - A link that changes during the run (Priority: P2)

**Goal**: reaction and recovery times per bot and per step.

**Independent Test**: quickstart §3 on each local server.

- [X] T023 [P] [US2] Write a test in `src/execution/quality_collector_test.go`: each collection appends, per received video stream, `{at, height, froze}` to the bot's timeline, with the stream's sender id where the adapter names it.
- [X] T024 [US2] Keep the per-stream timeline (`botTimeline`) in `src/execution/quality_collector.go`, taken from the snapshot the collector already reads each second, together with the sender of each stream: the remote participant the adapter gave for the track, or the 012 frame-trailer send-time lookup. Make T023 pass. Memory: one small struct per stream per collection (2 s), kept only when the plan has `network`. *Done without the sender of each stream, which only US4's receivers need: added in T038.*
- [X] T025 [P] [US2] Write tests in `src/execution/adaptation_test.go` on synthetic timelines (research R8):
  - reaction = from step start to the first sample where a stream that was at the requested height is lower or froze;
  - recovery = from the end of the limit to the first sample from which every such stream holds the requested height for 5 s with no freeze;
  - `recovered` false with `not_recovered_before_end`;
  - `not_at_requested_height` when no stream was at its height at step start;
  - `no_video`.
- [X] T026 [US2] Implement reaction and recovery in `src/execution/adaptation.go` and fill `BotStep.ReactionSeconds`, `RecoverySeconds`, `Recovered`, `Reason`. A step "lowers the bandwidth" when its downlink rate is below the content bitrate total of the bot's requested streams (014 `content.json` bitrates × received streams), or when it adds loss or delay. Make T025 pass. *Done: every step with a downlink condition gets the measure; a step that never lowers a bot gives `not_lowered`, which says the same more simply.*
- [X] T027 [US2] Add the per-step reaction and recovery table (median and maximum per group, per bot in a details list) to `src/report/markdown_016.go`, with its test in `src/report/markdown_016_test.go`.
- [X] T028 [US2] Live: quickstart §3 on the four local servers, one run each. Check SC-004, record it in the validation report, and keep the reports in `runs/us2/`.

**Checkpoint**: US1 and US2 work; a schedule gives per-bot reaction and recovery times.

---

## Phase 5: User Story 3 - Delay, jitter and loss (Priority: P3)

**Goal**: the remaining condition fields, seeded.

**Independent Test**: quickstart §4 on each local server.

- [X] T029 [P] [US3] Extend `src/execution/netcond_test.go`:
  - mean added delay within 5 ms of `delay_ms` with jitter ±`jitter_ms`, and delivery order kept (no reordering);
  - random loss within 1 percentage point of `share` over 50,000 datagrams;
  - Gilbert-Elliott with `mean_burst` 4 at share 0.05 gives a mean burst length within 15% of 4 and the share within 1 point;
  - the same seed gives the same drop sequence over the same datagrams;
  - two runs of 60,000 and 61,000 datagrams with the same seed give shares within 0.5 point (SC-005).
- [X] T030 [US3] Implement delay, jitter (uniform, monotonic delivery times), random and burst loss (transitions 1/L and p/(L·(1−p))) in `src/execution/netcond.go`, each `linkShaper` with its own PCG from (seed, launch index, direction). Count drops by loss, bursts and their lengths. Make T029 pass.
- [X] T031 [US3] Report `dropped_by_loss_share`, `mean_burst`, `added_delay_ms`. Also report `missing_after_retransmission_share`: the received-video loss the bot already measures (006) over the step, in `src/execution/adaptation.go` (FR-009). Add a test in `src/execution/adaptation_test.go`, and the Markdown columns in `src/report/markdown_016.go`.
- [X] T032 [US3] Live: quickstart §4 on the four local servers, run twice with seed 7 on one of them. Check SC-001 (delay, loss) and SC-005, and that the lossy bots' 012 video latency rises by the added delay. Record in the validation report and keep the reports in `runs/us3/`.

**Checkpoint**: all condition fields work; loss is repeatable from the seed.

---

## Phase 6: User Story 4 - A sender with a slow link (Priority: P4)

**Goal**: constrained senders drop and retry whole layers from Pion's send-side estimate.

**Independent Test**: quickstart §5 on each server whose T004 result is "feedback: yes".

- [X] T033 [P] [US4] Write tests in `src/execution/video_track_test.go`:
  - a simulcast layer's track with `enabled=false` writes nothing; enabled again, it writes from the next keyframe;
  - an SVC track with `maxSpatial=1` writes no packet whose dependency descriptor has spatial id 2, and restarts spatial 2 at its next keyframe;
  - sequence numbers written per binding stay contiguous across dropped packets;
  - a track with no limit writes the packets unchanged, as today.
- [X] T034 [US4] Implement layer control and own sequence numbers in `src/execution/video_track.go`, reading the spatial id and keyframe flag from the dependency descriptor the source wrote (`media.ParseDD` in `src/media/dependency_descriptor.go`). Keep the shared packets unchanged: only the header copy is changed. Make T033 pass.
- [X] T035 [P] [US4] Write `src/execution/sender_adapt_test.go` for the layer policy (research R7), with a fake estimate and clock:
  - drop the top layer after the estimate stays below the sent total for 1 s;
  - never drop the lowest layer;
  - try to restore after 2 s, then 4 s and 8 s at most after failed tries (a restored layer dropped within 3 s is a failed try), and reset after a success;
  - audio is never touched;
  - layer changes and an estimate sample per second are recorded;
  - a single-stream sender reports `cannot_adapt`.
- [X] T036 [US4] Implement `src/execution/sender_adapt.go`: Pion `gcc.NewSendSideBWE` with `SendSideBWEPacer(gcc.NewNoOpPacer())` and an initial bitrate equal to the total of the layers sent, fed through the interceptor registry; the policy ticking every 100 ms; the layers' content bitrates from `media` content. Make T035 pass.
- [X] T037 [US4] Register the TWCC sequence number interceptor, the transport-cc extension and the transport-cc RTCP feedback (LiveKit needs both, T004) on the sending kinds, and the estimator in `src/execution/pion.go`, only for bots whose group has an uplink step. This is the publish session, or Jitsi's single session. Add a test in `src/execution/peer_test.go`: an unconstrained bot's sending session registers neither, and its packets carry no transport-wide sequence number (FR-018).
- [X] T038 [US4] Fill `SenderResult` (adapts, reason `no_uplink_condition`, `cannot_adapt`, `no_feedback` when no transport-wide feedback arrived within 5 s of sending, or `receive_only`; layer changes; estimate samples). Also fill the receivers' results for constrained senders' streams (`constrained_senders` and their reaction and recovery, from the stream senders of T024). Both go in `src/execution/adaptation.go`, with tests in `src/execution/adaptation_test.go`. Add the senders' layers list to `src/report/markdown_016.go`. *Done for the senders. Not built: the receivers' results for the constrained senders' streams apart (`constrained_senders`). A receiving bot cannot tell which bot a stream comes from: every bot sends the same shared content with the same frame trailer, and the servers rewrite the SSRCs. Telling them apart needs each adapter to name a received stream's sender, which no adapter does today. The receivers' results stay in the run's and the groups' quality summaries.*
- [X] T039 [US4] Live: quickstart §5 on each local server with feedback, plus the same plan without `network`. Check SC-008 and SC-009. Record the servers without feedback as `no_feedback` behaviour. Keep the reports in `runs/us4/`.

**Checkpoint**: constrained senders adapt like a browser; unconstrained runs are unchanged.

---

## Phase 7: User Story 5 - The server's view and comparison (Priority: P5)

**Goal**: the server's estimate where its signaling gives it, and the four-server comparison.

**Independent Test**: the validation report's comparison tables (SC-007).

- [X] T040 [P] [US5] Add the optional `ReceiverEstimator` interface to `src/platform/adapter.go`. Write a test in `src/platform/mediasoup/adapter_test.go` against the fake server: `ReceiverEstimate` returns `availableOutgoingBitrate`/1000 of the bot's receive transport from `getTransportStats` (reply `{stats:[...]}`), and `ok=false` before the transport exists.
- [X] T041 [US5] Implement `ReceiverEstimate` in `src/platform/mediasoup/adapter.go`, sharing the request code of `logTransportStats`. Make T040 pass.
- [X] T042 [US5] In `src/execution/engine.go`, when the adapter implements `ReceiverEstimator`, sample it once per second for constrained bots only, with a 2 s timeout, and fill `ServerEstimate.Kbps`. Otherwise set reason `not_given_by_server`. Add a test in `src/execution/engine_test.go` with a fake adapter implementing it and one not.
- [X] T043 [US5] Live comparison: plans §2 and §3 on the four local servers, three runs each, and §4 and §5 once each. Jitsi gets a warm-up run first and its estimate read with `scripts/read-estimates.py jitsi`. Write one table per condition in the validation report with the run-to-run spread (SC-007), naming the 015 behaviours where they affected a run. Keep the reports in `runs/compare/`.

---

## Phase 8: Polish & Cross-Cutting Concerns

- [X] T044 CPU check (SC-006). At 100 bots on the local LiveKit with the T001 plan:
  - three runs without `network`: the engine CPU per bot is within 5% of T001;
  - three runs with 10 bots constrained both ways (downlink cap 500 kbit/s, uplink cap 400 kbit/s): record the difference.

  Record it in the validation report.
- [X] T045 [P] Document the `network` section in `README.md` (move "bandwidth limits and network instability" out of "Not done yet"), the server facts found (T004, T043), and the CLI and live-test variables. Add 016 to `CLAUDE.md` Recent Changes and Commands by hand: `update-agent-context.sh` truncates the list.
- [X] T046 [P] Update `specs/016-network-conditions/quickstart.md` with any command that changed during implementation.
- [X] T047 Run `go vet ./... && go test -race ./...`, `gofmt -l src cmd tests` (empty; *it lists four files 016 does not touch, unformatted before 016, left as they are*), and `go test -tags live -run xxx ./tests/live/` (build only). Fill every SC section of the validation report as met, not met (with the reason) or not checked.

---

## Dependencies & Execution Order

- **Setup (T001–T004)**: T001 before any 016 code is written. T004 is needed only before Phase 6.
- **Foundational (T005–T012)**: blocks every story. T006 needs T005. T008 needs T007. T010 needs T008 and T009. T011 needs T010.
- **US1 (T013–T022)**: after Phase 2. It is the MVP.
- **US2 (T023–T028)**: after US1's T016 (the per-bot step results it fills).
- **US3 (T029–T032)**: after Phase 2. It only extends the shaper, and can run beside US2; its report columns need T016.
- **US4 (T033–T039)**: after Phase 2 and the T004 gate. T038 uses T024 for the receivers' side.
- **US5 (T040–T043)**: T040–T042 after Phase 2. T043 after US1–US4.
- **Polish (T044–T047)**: last.

## Parallel Opportunities

- T002, T003 beside T001.
- In Phase 2: T005, T007, T009 and T012 (different files).
- US1: T013, T015, T017, T019 and T021 are tests or entry points in different files.
- US3's shaper work (T029–T030) beside US2 (T023–T027).
- US4's track work (T033–T034) beside its policy (T035–T036).
- US5's T040–T041 beside any story after Phase 2.

## Implementation Strategy

1. **MVP**: Phase 1 (T001–T003), Phase 2, then US1. Stop and check quickstart §2. A downlink cap per group is already a usable server comparison.
2. Add US2 (times) and US3 (loss, delay): the shaper is complete.
3. Run the T004 gate, then US4 (adapting senders), the largest and riskiest part.
4. US5 and the comparison, then the CPU check and the docs.
