---

description: "Task list for 015 bandwidth estimate parity with a browser"
---

# Tasks: Bandwidth Estimate Parity with a Browser

**Input**: Design documents from `specs/015-bandwidth-estimate-parity/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/test-plan.md](contracts/test-plan.md), [contracts/report.md](contracts/report.md), [quickstart.md](quickstart.md)

**Tests**: Included. The plan and the constitution require them: write each test first and see it fail. Real-server and real-browser tests stay behind `-tags live`; unit tests need neither Chrome nor a server.

**Organization**: the test plan sections and the report's format version are foundational (Phase 2). US1 builds the headless browser in a run and the estimate reader, then makes the comparison runs and the verdict. US2 measures the bots' reception reports; its code is built before the US1 comparison runs so that every comparison run carries the bots' figures. US3 is the correction, built only for the cause the verdict names. US4 records server settings and documents what remains the server's.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green at the end of each phase and needs no server and no browser.
- **Boundaries** (plan, constitution II and III):
  - no new module in `go.mod`;
  - nothing changes in `src/media`;
  - `src/browser` imports no Pion package and no platform package;
  - no adapter in `src/platform` reads `browser` or `server_settings`;
  - servers' code is not changed; server configuration only returns toward defaults (`tests/platforms/docker-jitsi-meet/docker-compose.remote.yml`, FR-011);
  - a run whose plan has no `browser` section starts no process and looks for no executable (FR-018).
- **Names** (used across tasks):
  - in `src/scenario/scenario.go`: `TestPlan` gains `Browser *BrowserConfig \`json:"browser,omitempty"\`` and `ServerSettings *ServerSettingsConfig \`json:"server_settings,omitempty"\``; `BrowserConfig{URL string \`json:"url"\`; ExpectedHeight int \`json:"expected_height,omitempty"\`; Window string \`json:"window,omitempty"\`; InsecureTLS bool \`json:"insecure_tls,omitempty"\`; Executable string \`json:"executable,omitempty"\`}`; `(*BrowserConfig).WindowSize() (w, h int)`; `ServerSettingsConfig{Source string \`json:"source,omitempty"\`; Values map[string]string \`json:"values,omitempty"\`}`; constants `ServerSettingsOperator = "operator"`, `ServerSettingsDeployment = "deployment"`, `DefaultBrowserWindow = "1280x720"`;
  - in `src/scenario/catalog.go`: `RedactTestPlan` also replaces, in `Browser.URL`, the values of the query and fragment parameters `jwt`, `token`, `access_token` with `Redacted`;
  - in `src/scenario/quality.go`: `ReceptionReports{Available bool; Reason string; Bots int; AcknowledgedShare, LateShare, StampLagMaxMs *Percentiles; Limits ReceptionLimits; Flagged []string; PerBot []BotReception}` as `QualitySummary.ReceptionReports \`json:"reception_reports"\``; `ReceptionLimits{AcknowledgedShareMin float64; LateShareMax float64; LateIntervalMs int}`; `BotReception{PeerID string; PacketsArrived, PacketsAcknowledged uint64; AcknowledgedShare float64; FeedbacksSent uint64; IntervalMs, StampLagMs MeanMax; LateShare float64}`; `MeanMax{Mean, Max float64}`; reasons `ReceptionNoVideo = "no_video"`, `ReceptionNotNegotiated = "not_negotiated"`, `ReceptionNotCounted = "not_counted"`; JSON field names exactly as in [contracts/report.md](contracts/report.md);
  - in `src/scenario/result.go`: `RunResult.Browser *BrowserResult`; `BrowserResult` with the JSON fields of the contract's `browser` object; `BrowserSample{T float64; Heights []int; Kbps float64}`; statuses `BrowserMeasured = "measured"`, `BrowserPartial = "partial"`, `BrowserUnavailable = "unavailable"`; reasons `BrowserNotFound = "browser_not_found"`, `BrowserStartFailed = "browser_start_failed"`, `PageLoadFailed = "page_load_failed"`, `NoVideoReceived = "no_video_received"`, `BrowserExited = "browser_exited"`; `ExpectedHeightNotStated = "not_stated"`;
  - in `src/browser`: `type Options{URL string; Window [2]int; InsecureTLS bool; Executable string; ExpectedHeight int}`; `Start(ctx context.Context, o Options, logger *slog.Logger) *Session` (never returns an error: a failure is held in the session); `(*Session).Stop() *scenario.BrowserResult` (idempotent); `FindExecutable(given string) (string, error)` looking for `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser` in that order; a variable `sampleInterval = time.Second` for tests;
  - in `src/execution/twcc.go`: `transportFeedback` gains atomic counters `arrived`, `acknowledged`, `feedbacks`, and guarded interval and stamp-lag accumulators; `(f *transportFeedback) arrive(pkt []byte, at time.Time)` called from `consentConn.ReadFrom`; `(f *transportFeedback) snapshot() feedbackSnapshot`; limits `ackShareMin = 0.99`, `lateShareMax = 0.01`, `lateInterval = 200 * time.Millisecond` in one `const` block;
  - in `src/remote/servers.go`: `Server.Settings map[string]string` (the settings the remote overlay changes from the server's defaults and that change results); in `src/remote/settings.go`: `<PLATFORM>_SERVER_SETTINGS` (`JITSI_SERVER_SETTINGS`, `LIVEKIT_SERVER_SETTINGS`, `MEDIASOUP_SERVER_SETTINGS`, `JANUS_SERVER_SETTINGS`) as `name=value` pairs joined by commas, sorted by name, empty when none;
  - live test variables: `<PLATFORM>_BROWSER_URL`, `<PLATFORM>_BROWSER_HEIGHT`, `<PLATFORM>_BROWSER_WINDOW`, `<PLATFORM>_BROWSER_INSECURE_TLS=1`, `<PLATFORM>_SERVER_SETTINGS`;
  - `report.FormatVersion = 3`.
- **Live runs**: local servers in `tests/platforms` first, one at a time; the 013 remote machine `fetis@192.168.1.95` for SC-006 and SC-007. Stop only what you started. Never `pkill -f`: kill by PID. Never run `sfu-remote allow`, never pass `--external-firewall`. Keep report files, logs and estimate files of every comparison run (`specs/015-bandwidth-estimate-parity/estimates/`); report files of other runs may stay in the session's scratch folder.
- **Gates**: T002 (R12 facts) before T019 and the US1 browser code depends on any page behaviour; T040 (verdicts) decides Phase 5: build only the corrections of the causes it names. A gate that fails, or a fact that changes the spec, stops the work and goes to the user.
- **Commits**: only when the user asks.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story the task belongs to (US1 to US4)

---

## Phase 1: Setup

**Purpose**: Open the validation report, take the baselines, and check the facts the design rests on (research R12).

- [X] T001 Create `specs/015-bandwidth-estimate-parity/validation-report.md` with one section per success criterion (SC-001 to SC-009), a "Facts (R12)" section, a "Verdicts" section and a "Baselines" section; create the empty folder `specs/015-bandwidth-estimate-parity/estimates/` with a `README.md` naming the file pattern `<server>-<codec>-<n>.jsonl` and the fields of data-model "Receiver estimate"; record `git rev-parse HEAD` and the output of `go vet ./... && go test -race ./...`
- [X] T002 Check the seven R12 facts and record each answer, with the command and its output, in `validation-report.md` "Facts (R12)": (1) start `google-chrome --headless=new` with the R1 flags on the local mediasoup demo page and on the local Jitsi Meet page (`https://localhost:8443/<room>`) while 2 bots send video, and confirm in DevTools (`Runtime.evaluate` of `getStats` on the page's connections, wrapped as in R2) that `frameHeight`, `framesDecoded`, `freezeCount`, `totalFreezesDuration` and `bytesReceived` are present and grow; (2) both pages create their `RTCPeerConnection` in the main frame; (3) the address of each page that joins the room by itself and sends nothing (mediasoup demo: room id plus its options for no camera and no microphone; Jitsi Meet: fragment options skipping the pre-join page, starting audio and video muted, tile view); (4) the mediasoup log level and tags that give each receiving transport's estimate at least once per second, and how a line is tied to its participant (else the `getTransportStats` fallback of R7); (5) the Jitsi bridge's debug address on `127.0.0.1:8080` for one conference, whether it answers in the local setup (else the variable that enables it), and the field holding each endpoint's estimate; (6) which feedback each server uses toward a bot and toward the browser (transport-cc or REMB: from the server's log or debug state, and the browser's `chrome://webrtc-internals`); (7) whether `ProcessState.SysUsage()` after killing Chrome's process group covers its renderer and GPU processes (compare with summed `/proc/<pid>/stat` of the group just before). Stop what you started
- [X] T003 [P] Record the baselines in `validation-report.md` "Baselines": engine CPU per bot at 100 bots in the conditions of 014 T063 (local LiveKit restarted before each run, 100 bots `audio_video` VP8 single stream, receive cap 10, 20 s ramp-up, 2 minute hold; the report's `load_machine.engine.cpu_threads.average` divided by 100), three runs, for FR-015 and SC-008

**Checkpoint**: if a fact of T002 does not hold and its fallback does not either, stop and tell the user.

---

## Phase 2: Foundational (test plan sections and report version)

**Purpose**: A plan can state a browser and server settings, the report knows the new version, and the run result can carry a browser result. All stories depend on this phase.

- [X] T004 [P] Write failing tests in `src/scenario/browser_test.go` for `browser` ([contract](contracts/test-plan.md)): accepted with only `url`, giving `Window` `1280x720`; `url` missing, not `http`/`https`, or "without a host" refused naming `browser.url`; `expected_height` accepted "1–2160" (1 and 2160 pass), refused at -1 and 2161 (0 means not given); `window` accepted `320x240` and `3840x2160`, refused `319x240`, `3841x2160`, `1280`, `1280x`, `ax720`; `executable` not checked at creation (a missing path is accepted); unknown fields in the section refused; a plan without the section round-trips unchanged through JSON (FR-018)
- [X] T005 [P] Write failing tests in `src/scenario/browser_test.go` for `server_settings`: `source` defaults to `operator`; `deployment` with no value accepted; another `source` refused; "at most 16" values (17 refused); an empty name refused; `source: operator` with no value refused ("it states nothing"); values are copied, not shared with the caller
- [X] T006 [P] Write failing tests in `src/scenario/catalog_test.go`: `RedactTestPlan` replaces with `Redacted` the values of `jwt`, `token` and `access_token` in the query and in the fragment of `browser.url` (for example `https://h/r?jwt=abc#config.x=1&token=def`), keeps every other parameter and the order, leaves the caller's plan unchanged, and leaves a plan without `browser` as before
- [X] T007 Implement in `src/scenario/browser.go` (fields in `src/scenario/scenario.go`) the `BrowserConfig` and `ServerSettingsConfig` types, `WindowSize`, the constants and their validation in `NewScenario` (through `withDefaults` methods, as `ServerMonitoringConfig`), with the Names above, until T004 and T005 pass
- [X] T008 Implement the `browser.url` redaction in `src/scenario/catalog.go` (`RedactTestPlan`), until T006 passes
- [X] T009 [P] Add to `src/scenario/result.go` the `BrowserResult`, `BrowserSample` types, statuses and reasons of the Names, and `RunResult.Browser`; JSON names exactly as [contracts/report.md](contracts/report.md) `browser`; with `unavailable`, only `status`, `reason`, `url` and `window` marshal (test in `src/scenario/quality_test.go`)
- [X] T010 [P] Add to `src/scenario/quality.go` the `ReceptionReports`, `ReceptionLimits`, `BotReception`, `MeanMax` types and reasons of the Names, as `QualitySummary.ReceptionReports`; with `Available` false only `available` and `reason` marshal (test in `src/scenario/quality_test.go`)
- [X] T011 Raise `FormatVersion` to 3 in `src/report/run.go` (comment: "3 adds quality.reception_reports, browser and server settings (015)"), add `Browser *scenario.BrowserResult \`json:"browser,omitempty"\`` to `Run` set from `RunResult.Browser` in `NewRun`; update `src/report/testdata/full.json` and `receive_only.json` and the tests in `src/report/run_test.go`; `go test -race ./src/report` passes

**Checkpoint**: `go vet ./... && go test -race ./...` green; a plan with both sections can be created through the API; reports say version 3.

---

## Phase 3: User Story 1 - A browser beside the bots (Priority: P1) 🎯 MVP

**Goal**: one headless Chrome, started by the engine from the test plan, measured in the report; the server's estimate per receiver read once per second; one verdict per server for mediasoup and Jitsi.

**Independent Test**: on the local mediasoup, 10 bots sending layered VP8 with half asking for 720, and a plan asking for a browser on the demo page for the same room: the report has the browser's height shares and its time to 720, the estimate file has one line per receiver per second, and `validation-report.md` states the verdict for mediasoup (quickstart steps 2 and 4).

### Tests for the browser package (no Chrome needed)

- [X] T012 [P] [US1] Write a fake DevTools server in `src/browser/fake_test.go`: an `httptest` server answering `/json/version` and a WebSocket target that records `Page.addScriptToEvaluateOnNewDocument`, `Page.navigate` and `Runtime.evaluate` calls and answers `Runtime.evaluate` with scripted `getStats` results; and a fake executable (a test binary re-executed with an environment variable, writing `DevToolsActivePort` into its `--user-data-dir` and serving the fake) so `Start` runs without Chrome
- [X] T013 [P] [US1] Write failing tests in `src/browser/browser_test.go`: `FindExecutable("")` with a `PATH` holding only `chromium` returns it, with an empty `PATH` returns an error; `FindExecutable("/no/such")` errors; `Start` with no executable gives a result `unavailable`/`browser_not_found`; a fake that exits at once, or never writes `DevToolsActivePort` within 15 s (shortened by a test variable), gives `browser_start_failed`; a navigation answered with `errorText` gives `page_load_failed`; samples with no decoded frame over the run give `no_video_received`; the fake exiting mid-run gives `partial`/`browser_exited` with the samples up to then; `Stop` twice returns the same result; after `Stop` the process group is gone and the profile folder removed; the start arguments contain `--headless=new`, `--window-size=1280,720`, `--remote-debugging-port=0`, `--user-data-dir=`, `--autoplay-policy=no-user-gesture-required`, `--use-fake-ui-for-media-stream`, `--use-fake-device-for-media-stream`, `--no-first-run`, `--disable-background-timer-throttling`, `--disable-renderer-backgrounding`, and `--ignore-certificate-errors` only with `InsecureTLS`
- [X] T014 [P] [US1] Write failing tests in `src/browser/stats_test.go` for turning per-second `getStats` samples into the result (research R2): a stream counts in a second when its `framesDecoded` grew in it; `height_share` is decoded stream-seconds per height over all decoded stream-seconds; `expected_height_share` is the share at `ExpectedHeight`, and without one the field is absent and `expected_height_reason` is `not_stated`; `seconds_to_expected_height` is the offset from page load of the first second with a stream at the expected height, absent if never; `received_bitrate_kbps` average and peak from video `bytesReceived` deltas; `freezes_total` and `freeze_seconds` from the last `freezeCount` and `totalFreezesDuration` summed over streams, `freeze_source` `browser`; `video_streams` is the most streams decoding in one second; a stream that disappears (connection closed) keeps its counted seconds; one `samples` entry per second with its heights sorted

### Browser package

- [X] T015 [US1] Implement `src/browser/devtools.go`: read `DevToolsActivePort` from the profile folder (port and browser path), connect with `nhooyr.io/websocket` to the page target (from `/json/list`, or `Target.createTarget` when there is none), send commands with increasing ids and match answers, surface `Inspector.detached` and a closed socket as the browser gone; no new module
- [X] T016 [US1] Implement `src/browser/stats.go`: the injected script (a constant string) that wraps `window.RTCPeerConnection` so every connection the page creates is kept in a page-global list and removed on `close`, and a `collect()` function returning, for every kept connection, its `inbound-rtp` video entries with `id`, `frameHeight`, `framesDecoded`, `freezeCount`, `totalFreezesDuration`, `bytesReceived`; and the Go side turning the samples into the result, until T014 passes
- [X] T017 [US1] Implement `src/browser/browser.go`: `FindExecutable`, `Start` (temporary profile folder, the R1 arguments, own process group via `SysProcAttr.Setpgid`, wait for DevTools up to 15 s, `Page.addScriptToEvaluateOnNewDocument` before `Page.navigate`, then one `Runtime.evaluate` of `collect()` with `awaitPromise` every `sampleInterval`), `Stop` (kill the group, wait, read CPU time per R4 and set `cpu_threads_average` as CPU time over the period's wall time, remove the folder, build the result), the browser version from `/json/version`, and the reasons of R3, until T013 passes; every failure is logged once at warning with the reason

### Engine and entry points

- [X] T018 [US1] Write failing tests in `src/execution/engine_test.go`: with a plan without `browser`, `browser.Start` is never called (a test hook counts calls) and `RunResult.Browser` is nil; with `browser`, it is started when the run starts (before the first bot), stopped when the run ends, also when the run fails or is stopped, and its result is in `RunResult.Browser`; a browser result `unavailable` leaves the run's status and the bots' summary unchanged (FR-019); the browser is not in `QualitySummary.Bots` (FR-017)
- [X] T019 [US1] Start and stop the browser in `src/execution/engine.go` (`Start`: after the media sources are ready and before the ramp-up; stop next to the sampler's last sample, before `onComplete`), through a package variable `startBrowser = browser.Start` replaceable in tests, until T018 passes
- [X] T020 [P] [US1] Add to `src/report/markdown.go` the "Browser" key figure row and the "Browser" section after "Load machine" ([contract](contracts/report.md) Markdown), and the load machine line "A browser ran on this machine during the run: <x> threads on average, not counted in the engine's figure."; nothing about a browser when the plan has none; golden tests in `src/report/markdown_test.go` for measured, partial, unavailable (each reason) and absent
- [X] T021 [P] [US1] Add `--browser-url`, `--browser-height`, `--browser-window`, `--browser-insecure-tls` to `cmd/jitsi-connect` (set `TestPlan.Browser` only when `--browser-url` is given); update its usage text
- [X] T022 [P] [US1] Read `<PLATFORM>_BROWSER_URL`, `<PLATFORM>_BROWSER_HEIGHT`, `<PLATFORM>_BROWSER_WINDOW`, `<PLATFORM>_BROWSER_INSECURE_TLS` in `tests/live/mediasoup_live_test.go`, `tests/live/jitsi_live_test.go`, `tests/live/livekit_live_test.go` and `tests/live/janus_live_test.go` (one shared helper in `tests/live/report_test.go`), list them in each file's header comment, and log the browser's status and expected-height share at the end of the test

### Estimate reader (validation tooling)

- [X] T023 [P] [US1] Write `scripts/read-estimates.py jitsi [--interval 1]` (Python standard library: no `jq` on this machine) per research R7 (after R12): one JSON line per receiver per second on stdout with `t`, `server`, `receiver` (bot endpoint id, or `browser` for the endpoint with a `stats_id`), `estimate_bps` (`bitrate_controller.bitrate_allocator.bweBps`), `suspended` (`allocation.has_suspended_sources`), `method` (`jvb-debug`), polling `http://127.0.0.1:8080/debug` and `/debug/<conference>?full=true`; `mediasoup` exits with the message that the server gives no per-receiver estimate (R12 fact 4); stops on SIGINT/SIGTERM; usage in the script's docstring. And, for mediasoup's layer-over-time comparison, an `Info` log line `Received video height changed` (bot, previous and new height, seconds since the run's start) in `src/execution/quality_collector.go` `collectVideo` when the height a bot receives most of in a sample changes, with a test in `src/execution/quality_collector_test.go`
- [X] T024 [P] [US1] Write `scripts/estimate-verdict.py` (Python 3 standard library, run with `python3 -I`) computing, per research R11: Jitsi from the estimates file and the report (each receiver's need: content bitrates of `quality.video.codec.content_bitrate_kbps` at the heights it asked for, times the streams it received, plus audio; the browser's from its result's best second), the second each receiver reached its need, the time each spent under it after the browser reached its own, and the bots "held low" (more than 1 s after the browser, or falling back under it later while the browser does not); mediasoup from the report's browser `samples` and the bots' `Received video height changed` log lines (time to the requested height, time spent under it after the browser's); output a Markdown table for `validation-report.md`; test it on hand-written files in `scripts/testdata/estimates-sample.jsonl` and `scripts/testdata/heights-sample.log` with `python3 -I scripts/estimate-verdict.py --self-test`

### Live: browser in a run

- [X] T025 [US1] Run quickstart step 2 on the local mediasoup with 3 bots and the T002 address: the report's browser part is `measured` with height shares and a share at 360; then each failure case of the quickstart table (`page_load_failed`, `browser_not_found` with a `PATH` without Chrome, `no_video_received` with an address that waits for a click, no section: no Chrome process during the run, checked with `pgrep -x chrome` by PID listing, not `pkill`); record results in `validation-report.md` under FR-016 to FR-019
- [X] T026 [US1] Run the same 3-bot browser run on the local Jitsi with `--browser-insecure-tls` and the T002 address; record it

**Checkpoint**: the browser works in a run on both pages, and estimates are read on both servers. The comparison runs (T037 to T040) wait for US2's code (T027 to T034) so every run carries the bots' reception figures.

---

## Phase 4: User Story 2 - What the bots tell the server (Priority: P2)

**Goal**: every run with video reports, per bot, the share of arriving packets acknowledged, how often and how regularly feedback left, and how late arrivals were stamped; bots beyond the limits are named and layered results warned.

**Independent Test**: a 10-bot, 1-minute run on one server has `quality.reception_reports` with one entry per bot; an engine test where one bot skips recording a share of its packets names that bot and puts the warning in the Markdown file (SC-002).

- [X] T027 [P] [US2] Write failing tests in `src/execution/twcc_test.go`: `arrive` counts an RTP packet with the transport-cc extension of the applied offer, and not a packet without it, an RTCP packet, a STUN packet or a DTLS record (first-byte demultiplexing, RFC 7983); `arrive` before `setExtension` counts nothing; acknowledged counts the packets marked received in the status chunks of the feedback packets actually written (a failing `write` adds none), for run-length and status-vector chunks; `feedbacks` counts successful writes; interval mean and max and the share of intervals over `lateInterval`; stamp lag mean and max from the socket time given to `arrive` to the `record` call for the same transport sequence number; packets arriving after `close` are not counted (data-model counting rule); no allocation per packet in `arrive` (`testing.AllocsPerRun` = 0)
- [X] T028 [US2] Implement the counters of the Names in `src/execution/twcc.go` until T027 passes: `arrive` parses only the RTP header fixed part and its extension (`rtp.Header.Unmarshal` on the plain header bytes; RTP headers are not encrypted by SRTP) and keeps the socket time per transport sequence number in a fixed-size ring for the stamp lag; `run` counts acknowledged packets from each `rtcp.TransportLayerCC` it writes; the three limits in one `const` block
- [X] T029 [US2] Call `arrive` from `consentConn.ReadFrom` in `src/execution/consent.go` for the receiving PeerConnection only (the publish session has no feedback), with `time.Now()` taken right after the read; with a network replaced by `settingEngineHook`, mark the peer's measure `not_counted`; test in `src/execution/consent_test.go` that packets read through `consentConn` reach `arrive` and STUN answers to consent checks do not
- [X] T030 [P] [US2] Write failing tests in `src/execution/quality_collector_test.go` for `summary`: `reception_reports` with one `per_bot` entry per bot measured, in launch order; percentiles of `acknowledged_share`, `late_share` and `stamp_lag_max_ms` over bots; `flagged` lists bots with `acknowledged_share < 0.99` or `late_share > 0.01`; `limits` gives `0.99`, `0.01`, `200`; reasons: `no_video` for audio-only and receive-only runs, `not_negotiated` when no bot's offer had transport-cc, `not_counted` with a replaced network; never a zero figure in place of a reason (FR-007)
- [X] T031 [US2] Build `ReceptionReports` in `src/execution/quality_collector.go` from each peer's `feedback.snapshot()` (taken when the peer leaves or the run ends), until T030 passes
- [X] T032 [US2] Add a test-only switch in `src/execution/twcc.go` (package variable `skipRecordEvery int`, 0 in production) that skips recording one packet in N for one chosen peer; test in `src/execution/engine_test.go` (virtual network of `pion/transport/v2/vnet`, a fake server sending transport-cc packets, or the existing engine test harness): with N=20 for one bot, that bot's `acknowledged_share` is about 0.95 and it is in `flagged`; the other bots are not (SC-002)
- [X] T033 [P] [US2] Add to `src/report/markdown.go` the "Reception reports" key figure row, the warning before the Video section's height figures when `flagged` is not empty, and the "Reception reports" section after "Video" (percentiles, limits, at most 10 flagged bots then "and N more") per [contracts/report.md](contracts/report.md); golden tests in `src/report/markdown_test.go` for available, flagged, more than 10 flagged, and each reason; update `src/report/testdata/full.json`
- [X] T034 [P] [US2] Expose `reception_reports` in `GET /api/v1/scenarios/{id}/quality` (the summary already carries it; add a test in `tests/integration/api/` checking the field is present after a run with the fake platform)
- [X] T035 [US2] Measure the engine CPU per bot at 100 bots as in T003 with the counters in, three runs; record against the baseline in `validation-report.md` (FR-015). Over 5%: profile `arrive`, reduce, and measure again before going on
- [X] T036 [US2] Live: one 10-bot, 1-minute `audio_video` run on each local server (Janus, LiveKit, mediasoup, Jitsi): `reception_reports.available` true with 10 entries, the acknowledged share and late share per server recorded in `validation-report.md` SC-002; one audio-only run gives `no_video`

**Checkpoint**: every run with video carries the bots' reception figures.

---

## Phase 3 (continued): User Story 1 - comparison runs and verdicts

- [X] T037 [US1] mediasoup comparison (quickstart step 4): `scripts/read-estimates.py mediasoup` into `estimates/mediasoup-vp8-<n>.jsonl` and a 10-bot run with `MEDIASOUP_SIMULCAST=720`, `MEDIASOUP_MAX_RECEIVED=4`, `MEDIASOUP_REQUESTED_HEIGHTS=180:0.5,720:0.5`, 60 s hold, `MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo`, and the browser at the T002 address with `MEDIASOUP_BROWSER_HEIGHT=720`; repeat until a run shows a bot held low, or five runs show none (US1 scenario 4); keep each run's two report files beside its estimate file; record per run the `estimate-verdict.sh` table, the engine's and the browser's CPU, the reception figures of the bots held low, and what differed between the browser's streams and a bot's (US1 scenario 5)
- [X] T038 [US1] Jitsi comparison, local setup with the bridge's default settings (no `JVB_CC_TRUST_BWE` override): one warm-up 10-bot run set aside, then VP8 runs (`--video-codec vp8`, `--duration 60`, `--server-containers docker-jitsi-meet-jvb-1`, the browser at the T002 address with `--browser-height 720 --browser-insecure-tls`) with `scripts/read-estimates.py jitsi` into `estimates/jitsi-vp8-<n>.jsonl`, repeated as in T037; then the same for AV1 (`estimates/jitsi-av1-<n>.jsonl`); record as in T037, plus the freezes and suspended streams per run
- [X] T039 [US1] Where T037 or T038 points to "the bots": the same comparison once with the server deployed on the remote machine (`go run ./cmd/sfu-remote deploy <server> --host fetis@192.168.1.95`, its settings file sourced, estimates read over ssh: run the script on the machine or forward the bridge's debug port), and remove the deployment after (SC-006); record it
- [X] T040 [US1] Write the verdict per server in `validation-report.md` "Verdicts" (FR-003): one of "the bots", "the server", both, "not reproduced", with the runs behind it (report and estimate file names), the bots held low and for how long, the browser's time to its need, and, for "the bots", which causes of research R8 the reception figures point to. **Gate**: this decides Phase 5; show it to the user before building any correction

**Checkpoint**: US1 is delivered whatever the verdict.

---

## Phase 5: User Story 3 - Bots estimated like a browser (Priority: P3)

**Goal**: for a server whose verdict includes "the bots", correct the bots so the server's estimate for them reaches their need within 1 s of a browser's; then layered runs pass on default settings.

**Independent Test**: on the local Jitsi with default settings, five 10-bot VP8 runs after a warm-up with 0 freezes; on the local mediasoup, five layered runs with half asking for 720 where those bots get it at least 95% of the time (SC-004, SC-005).

Build only the tasks of the causes T040 names (research R13). With "the server" for both servers, skip T041 to T047 and do T048 and T049 only.

- [X] T041 [P] [US3] Causes 1 and 2 (stamped late, never acknowledged): write failing tests in `src/execution/twcc_test.go` that the arrival recorded for feedback is the socket time given to `arrive` (stamp lag 0 by construction), and that a packet of a stream Pion never opens, or one rejected after the socket, is still acknowledged
- [X] T042 [US3] Causes 1 and 2: record for feedback in `arrive` (`src/execution/twcc.go`, called from `src/execution/consent.go`) instead of in `gatedBuffer.Write` (`src/execution/pion.go`), parsing the header once; keep the stamp lag measure meaningful by reporting it as 0 with a note in the code; until T041 passes; measure CPU at 100 bots as in T035
- [X] T043 [P] [US3] Cause 3 (bursts): write a failing test in `src/execution/twcc_test.go` that feedback leaves at most `transportFeedbackInterval` plus 20 ms after the previous one under a delayed ticker (fake clock), and also after every 100 recorded packets; then implement it in `src/execution/twcc.go` — **not built: feedback intervals were regular in every run (late share ≤ 0.15%), cause 3 not named by T040**
- [X] T044 [P] [US3] Cause 4 (recorder): reproduce the failing sequence from a run's trace as a test in `src/execution/twcc_test.go`, and fix it in a feedback builder in `src/execution/twcc.go` (replacing `twcc.Recorder` for that case only) — **not built: every packet that reached the socket was acknowledged after T042, cause 4 not named**
- [X] T045 [P] [US3] Cause 5 (receiver reports): write a failing test in `src/execution/peer_test.go` that a receiver report block is sent for every SSRC that arrives, dropped streams included; implement it in `src/execution/pion.go` (an RTCP sender over the counters of `gatedBuffer`) — **not built: cause 5 not named by T040 (the held bots on both servers are explained otherwise)**
- [X] T046 [P] [US3] Cause 6 (REMB advertised, never sent): write a failing test in `src/execution/peer_test.go` that the receiving PeerConnection's answer has no `goog-remb` feedback; remove it from the receiving `MediaEngine` in `src/execution/pion.go` — **not built: neither Jitsi nor mediasoup offers the bots REMB (R12 fact 6), cause 6 ruled out**
- [X] T047 [US3] After the corrections: the comparison of T037/T038 again, five runs each for the servers concerned: every bot's estimate reaches its need within 1 s of the browser's and none stays below (SC-003); record in `validation-report.md`
- [X] T048 [US3] mediasoup layered runs (SC-004): five 10-bot, 1-minute VP8 runs with `MEDIASOUP_SIMULCAST=720`, `MEDIASOUP_MAX_RECEIVED=4`, `MEDIASOUP_REQUESTED_HEIGHTS=180:0.5,720:0.5`, then one run each of VP9, AV1 and H.264; record the lowest `requested_height_share` of the 720 bots per run; where under 0.95, give the browser's share at 720 in the same conditions from T037 (SC-004's "same for a browser")
- [X] T049 [US3] Jitsi local default settings (SC-005): one warm-up, then five VP8 and five AV1 10-bot, 1-minute runs; record freezes and suspended streams per run; where not 0, give whether the browser was suspended too (from T038's runs)
- [X] T050 [US3] FR-011 on the remote machine: deploy Jitsi with `JVB_CC_TRUST_BWE` removed from a scratch copy of the overlay (do not change the repository file yet), run the five VP8 runs of T049 there; if they pass, remove `JVB_CC_TRUST_BWE` and its comment from `tests/platforms/docker-jitsi-meet/docker-compose.remote.yml`, update `src/remote/overlay_test.go` to check it is absent, and set Jitsi's `Server.Settings` to none (T053); if not, keep it and write why in the overlay's comment and `tests/platforms/docker-jitsi-meet/README.md`; record in `validation-report.md`
- [X] T051 [US3] No-change checks (FR-014, SC-008): the 014 layered 10-bot runs on the local LiveKit and Janus, one per codec (VP8, H.264, VP9, AV1) with `180:0.5,720:0.5`; shares within 0.02 of the 014 T059 table; one unlayered `audio_video` run per server with results as before; engine CPU per bot at 100 bots within 5% of T003; record in `validation-report.md`

**Checkpoint**: the 014 open items end as corrected or as a documented server behaviour (SC-009).

---

## Phase 6: User Story 4 - Server limits and lab settings stated (Priority: P4)

**Goal**: the report names the server settings that differ from defaults, from the deployment or the operator, or says they are not known; the documentation gives each remaining server behaviour.

**Independent Test**: deploy Jitsi on the remote machine, run 3 bots: both report files name the setting and its value with source `deployment`; the same on the local server with no setting given says "not known to the run" (quickstart step 6).

Independent of Phases 3 to 5 for T052 to T058: they can be done at any point after Phase 2.

- [X] T052 [P] [US4] Write failing tests in `src/remote/servers_test.go` and `src/remote/overlay_test.go`: each server's `Settings` matches what its `docker-compose.remote.yml` sets that changes results (Jitsi: `JVB_CC_TRUST_BWE` `false` while the overlay has it; none for LiveKit, mediasoup, Janus); a setting in an overlay's `environment` not declared, or declared and absent, fails the test
- [X] T053 [US4] Add `Server.Settings` to `src/remote/servers.go` with the values of T052 until it passes
- [X] T054 [P] [US4] Write failing tests in `src/remote/settings_test.go`: `settingsVars` gives `JITSI_SERVER_SETTINGS=JVB_CC_TRUST_BWE=false` for Jitsi and an empty `<PLATFORM>_SERVER_SETTINGS` for the others; several values sorted by name, joined by commas; the settings file shell-quotes the value; update `specs/013-remote-sfu-deployment/contracts/settings-file.md` with the new variable
- [X] T055 [US4] Write the variable in `settingsVars` (`src/remote/settings.go`) until T054 passes
- [X] T056 [P] [US4] Parse `<PLATFORM>_SERVER_SETTINGS` in the shared helper of `tests/live/report_test.go`: present (even empty) gives `server_settings` with `source: deployment` and its values; absent gives no section; use it in the four live tests; and in `cmd/jitsi-connect`: `--server-settings name=value,...` gives `source: operator`, else `$JITSI_SERVER_SETTINGS` gives `source: deployment`; a value without `=` is refused with the flag named
- [X] T057 [P] [US4] Add the "Server settings" key figure row to `src/report/markdown.go` with the three cases of data-model "Server settings": the values with "(stated by the deployment)" or "(stated by the operator)"; "none changed by the deployment"; "not known to the run"; never "defaults" alone; golden tests in `src/report/markdown_test.go` for each case
- [X] T058 [US4] Live (SC-007): deploy Jitsi on the remote machine, source its settings file, 3 bots with `jitsi-connect`; both report files name the setting and its source (or "none changed by the deployment" after T050); the same on the local Jitsi with nothing given says "not known to the run"; remove the deployment; record in `validation-report.md`
- [X] T059 [US4] Documentation (FR-013): for each server whose verdict includes "the server", a "Bandwidth estimate" paragraph in `README.md` (and in the server's `tests/platforms/<server>/README.md`): the behaviour, the time a browser needed to reach the requested height (from T037/T038), and the run conditions (ramp-up, hold) under which the layered criteria can be met; the browser in a test plan (`browser` section, Chrome needed only then); `server_settings`; update `tests/platforms/docker-jitsi-meet/README.md` on the estimate setting

**Checkpoint**: every run says what it knows of the server's settings.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T060 [P] Update `CLAUDE.md` "Commands" with the browser variables on a live test, `scripts/read-estimates.py`, `--browser-url` and `--server-settings`, and "Recent Changes" with a 015 line (verdicts and limits found)
- [X] T061 [P] Update `specs/015-bandwidth-estimate-parity/checklists/requirements.md` notes: the browser is now started by the engine from the test plan (clarifications of 2026-11-13), justified against constitution I in plan.md Complexity Tracking
- [X] T062 Close the 014 open items (SC-009): in `specs/014-video-codec-comparison/validation-report.md` "Not met or not checked", next to SC-008 on mediasoup (T053, T059, T060) and local Jitsi (T035, T044, T064), add one line each pointing to the 015 verdict and its outcome (corrected, or documented server behaviour with a browser measurement)
- [X] T063 Final checks: `go build ./... && go vet ./... && go test -race ./...` green; `gofmt -l .` lists no file changed by 015; `go.mod` unchanged; `grep -rn "src/browser" src/platform src/media` finds nothing; a run without `browser` on each local server shows no Chrome process (list by PID); every SC section of `validation-report.md` filled, with a "Not met or not checked" section for what remains

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none. T002 gates T016, T017, T023, T025 and every task that relies on a page or a server's estimate.
- **Foundational (Phase 2)**: after Setup; blocks all stories.
- **US1 code (T012–T026)** and **US2 (T027–T036)**: both after Phase 2, independent of each other; can run in parallel.
- **US1 comparison runs (T037–T040)**: after T026 and T036, so every comparison run has the browser and the bots' reception figures.
- **US3 (T041–T051)**: after T040; which tasks run depends on its verdict.
- **US4 (T052–T058)**: after Phase 2, any time; T059 after T040 (and T048–T049 for the browser times); T050 changes `Server.Settings` of T053.
- **Polish (Phase 7)**: after everything chosen is done.

### Within Each Story

- Tests first, failing, then the implementation that makes them pass.
- `src/scenario` types before `src/execution` and `src/report` code that uses them.
- `src/browser` (T015–T017) before the engine wiring (T019).
- Code before live runs; live runs before the report sections that cite them.

### Parallel Opportunities

- T003 with T002.
- T004, T005, T006, T009, T010 in parallel; then T007, T008.
- US1 tests T012, T013, T014 in parallel; T020, T021, T022, T023, T024 in parallel once T019 is done (T023, T024 even before).
- US2 tests T027 and T030 in parallel; T033 and T034 in parallel with T031.
- US1 code and US2 code in parallel (different packages and files).
- US3 correction tasks T041/T043/T044/T045/T046 are independent of each other; only those T040 names are built.
- US4 T052, T054, T056, T057 in parallel.

---

## Parallel Example: User Story 1

```bash
# Tests for the browser package, together (no Chrome needed):
Task: "T012 Fake DevTools server and executable in src/browser/fake_test.go"
Task: "T013 Failing start/stop/reason tests in src/browser/browser_test.go"
Task: "T014 Failing sample-to-result tests in src/browser/stats_test.go"

# Once the engine starts the browser (T019):
Task: "T020 Browser section in src/report/markdown.go"
Task: "T021 --browser-* flags in cmd/jitsi-connect"
Task: "T022 <PLATFORM>_BROWSER_* in tests/live"
Task: "T023 scripts/read-estimates.py"
Task: "T024 scripts/estimate-verdict.py"
```

## Parallel Example: User Story 2

```bash
Task: "T027 Failing counter tests in src/execution/twcc_test.go"
Task: "T030 Failing summary tests in src/execution/quality_collector_test.go"
# After T031:
Task: "T033 Reception reports in src/report/markdown.go"
Task: "T034 reception_reports in the quality API test"
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Phase 1, with T002's facts recorded.
2. Phase 2.
3. US1 code (T012–T026): a browser in a run, estimates read.
4. US2 code (T027–T036), so comparison runs carry the bots' figures.
5. US1 comparison runs and verdicts (T037–T040). **Stop and show the verdicts to the user**: they settle the open question of 014 and decide Phase 5.

### Incremental Delivery

1. Setup + Foundational: plans accept the sections; reports are version 3.
2. US1 code: a browser in any run, measured; useful on its own.
3. US2: every run says how the bots reported; useful on its own.
4. US1 verdicts: the open question of 014 answered.
5. US3: corrections for the causes found, or nothing to build.
6. US4: settings in every report and the documentation of server limits.
