# Research: Bandwidth Estimate Parity with a Browser (015)

Decisions for [spec.md](spec.md), from the current code (Pion webrtc v3.3.6,
`pion/interceptor` v0.1.29, the feedback code of 005, the adapters, the report
of 009 and 014, the remote deployment of 013) and from the 014 validation
report. Facts marked **verify** are not checked on a running server or browser
yet; the first live task (R12) checks them before any code depends on them.

## What exists today

- **Feedback the bots send.** Each bot acknowledges arriving packets with
  transport-wide feedback (TWCC) every 100 ms (`src/execution/twcc.go`),
  built by Pion's `twcc.Recorder`. A packet is recorded, with `time.Now()`,
  when Pion writes it into the stream's read buffer
  (`gatedBuffer.Write`, `src/execution/pion.go`): after the socket read, the
  ICE and DTLS demultiplexing and the SRTP decryption, and before the
  receive gate, so streams the bot drops are acknowledged too (005 R9).
- **Other reports.** Pion's default NACK and receiver-report interceptors.
  Receiver reports cover only the streams the bot reads.
- **What the bots advertise.** Pion's default codecs carry `goog-remb`
  feedback; the bots add `transport-cc` and its header extension. The bots
  never send REMB.
- **Sockets.** Every PeerConnection opens its sockets through `consentNet`
  (`src/execution/consent.go`, 010), unless a test replaces the network.
- **Nothing measures the feedback.** No counter says how many arriving
  packets were acknowledged, or when the feedback left.
- **Server load and engine load** come from `src/hostload` (007). The
  engine's figure is its own process only.
- **Server settings.** The remote Jitsi overlay sets
  `JVB_CC_TRUST_BWE=false` (`tests/platforms/docker-jitsi-meet/docker-compose.remote.yml`,
  checked by `src/remote/overlay_test.go`). Nothing passes it to a run or a
  report. The local setup keeps the default.
- **Server sides.** The local Jitsi compose file publishes the bridge's
  private HTTP port on `127.0.0.1:8080`. The mediasoup demo is built from a
  pinned commit and logs with the level of `MEDIASOUP_LOG_LEVEL`; the 014
  report read receiving sessions' estimates from that log.
- **Browser.** Chrome is at `/usr/bin/google-chrome` on the workstation. The
  engine starts no browser; `nhooyr.io/websocket` is already a dependency.
- **Report.** `FormatVersion` is 2. The JSON report holds the test plan,
  the outcome and the quality summary; per-bot figures appear only in the
  "worst bots" lists.

## R1. A headless browser the engine starts

- **Decision**: a new package `src/browser` starts Chrome as a child
  process and drives it over the DevTools protocol, with the standard
  library and `nhooyr.io/websocket`. No new module.
  - Start: `--headless=new`, a fresh `--user-data-dir` in a temporary
    folder, `--remote-debugging-port=0` (the port is read from the profile's
    `DevToolsActivePort` file), `--window-size=<w>,<h>`,
    `--autoplay-policy=no-user-gesture-required`,
    `--use-fake-ui-for-media-stream`, `--use-fake-device-for-media-stream`,
    `--no-first-run`, `--disable-background-timer-throttling`,
    `--disable-renderer-backgrounding`; `--ignore-certificate-errors` only
    when the test plan sets `insecure_tls`.
  - Executable: the test plan's `browser.executable`, else the first of
    `google-chrome`, `google-chrome-stable`, `chromium`, `chromium-browser`
    found in `PATH`.
  - Stop: at the end of the run the process group is killed and the profile
    folder removed, also when the run fails or is stopped.
- **Rationale**: FR-016, FR-018. Chrome is an optional program found at run
  time, as ffmpeg is for `media.source_file`: the binary stays single and
  static, and a run that does not ask for a browser never looks for one.
- **Alternatives considered**: `chromedp` or another driver module (the
  project adds no module for about 300 lines of JSON over a WebSocket);
  a separate command beside the run (rejected by the clarification: the
  engine starts it); Firefox (spec: Chrome only).
- **verify**: headless Chrome decodes and plays the received video with
  these flags, on the mediasoup demo page and on the Jitsi Meet page.

## R2. Reading the browser's measures from a page the engine does not know

- **Decision**: the engine never touches the page's interface. Before the
  page loads, it installs a small script
  (`Page.addScriptToEvaluateOnNewDocument`) that patches the native
  `RTCPeerConnection.prototype` methods (`setLocalDescription`,
  `setRemoteDescription`, `addTransceiver`, `addTrack`,
  `createDataChannel`, `createOffer`, `createAnswer`) to note `this`, and
  so keeps every connection the page uses. (Checked in R12: wrapping the
  constructor misses the mediasoup demo's connections, because a library
  of the page replaces the constructor with one around the native one.) Once per second it evaluates `getStats()` on each of them
  (`Runtime.evaluate`, awaiting the promise) and reads, per `inbound-rtp`
  video stream: `frameHeight`, `framesDecoded`, `freezeCount`,
  `totalFreezesDuration` and `bytesReceived`.
- From these samples (data-model, "Browser result"):
  - **heights over time**: per second, the heights of the streams that
    decoded at least one frame during that second;
  - **height shares** and the **share at the expected height**: decoded
    stream-seconds at each height over all decoded stream-seconds, the same
    time-weighted rule as the bots' `requested_height_share`;
  - **time to the expected height**: from the page's load to the first
    second in which a stream is at the expected height (FR-013 needs it);
  - **received bitrate**: from `bytesReceived`, video streams;
  - **freezes**: Chrome's own count and duration.
- **Stated difference**: Chrome's freeze rule (a frame later than
  `max(3 × average, average + 150 ms)`) is not the rule of 006 used for the
  bots. The report names the source of each figure; the two are compared as
  "0 or not", as SC-005 does, not number against number.
- **Limit**: a page that runs its connections in a cross-origin frame is a
  separate DevTools target and is not read; the report then says
  `no_video_received`. Neither comparison page does this (**verify**).
- **Rationale**: FR-016 (no joining step), FR-017. `getStats` is the same on
  every page, so one method serves any server.
- **Alternatives considered**: reading each page's own state (per-server
  code, against "any server"); `chrome://webrtc-internals` dumps (a file
  format made for people, and no live figures).

## R3. Browser failures never stop the run

- **Decision**: the browser runs beside the engine's run loop. Every
  failure becomes a reason in the browser's part of the report, and the run
  goes on:

  | Reason | When |
  |---|---|
  | `browser_not_found` | no executable at the given path or in `PATH` |
  | `browser_start_failed` | the process exits or DevTools does not answer within 15 s |
  | `page_load_failed` | the navigation fails (address, certificate) |
  | `no_video_received` | the page loaded and no video stream decoded a frame during the run |
  | `browser_exited` | the process ends before the run does; the measures up to then are kept and marked partial |
- **Rationale**: FR-019 and its clarification; the same rule as server load
  (007): a missing measure is named, never a zero.

## R4. The browser's CPU

- **Decision**: the engine's CPU figure stays its own process, so FR-015
  and every earlier comparison keep their meaning. The browser's CPU time
  is read when its process is waited for (`ProcessState` user and system
  time, which include the child processes Chrome waited for) and reported
  beside the engine's figure as `browser.cpu_threads_average`, with the
  line "a browser ran on the load machine" in the load machine section.
- **Spec wording**: the edge case "The browser costs CPU" says the
  browser's CPU is "part of the load machine's figure". It is given in that
  section, on its own line, not added to the engine's number.
- **verify**: the waited-for time covers Chrome's renderer and GPU
  processes when the group is killed. If it does not, the figure is the sum
  of `/proc/<pid>/stat` over the process group at the last sample.
- **Rationale**: constitution IV. One browser, off by default, measured.

## R5. The test plan

- **Decision**: two optional sections
  ([contract](contracts/test-plan.md)), both platform-independent:
  - `browser`: `url` (required, `http` or `https`), `expected_height`,
    `window` (default `1280x720`), `insecure_tls`, `executable`. One object,
    so at most one browser per run (FR-016).
  - `server_settings`: `source` (`operator` by default, or `deployment`) and
    `values`, a map of setting names to values (R9).
- The browser's `url` can carry a token. In the report and the API answer,
  the values of the query and fragment parameters `jwt`, `token` and
  `access_token` are hidden, as secret `platform_config` values are.
- **Rationale**: FR-016, FR-012. The platform adapters do not see either
  section (constitution III).

## R6. What the bots tell the server: three measures

- **Decision**: `transportFeedback` counts, per bot, with atomic counters
  and no allocation per packet:
  - **arrived**: RTP packets carrying a transport sequence number, counted
    where the bot's socket is read (`consentNet`), before Pion handles
    them. RTP headers and their extensions are not encrypted by SRTP, so
    the number can be read there. This is "every packet that reaches the
    bot" of FR-004: a packet lost between the socket and the read buffer
    (unknown stream, replay check, full buffer) counts as arrived and not
    acknowledged.
  - **acknowledged**: packets reported as received in a feedback whose
    write succeeded, counted from the feedback packets themselves
    (`rtcp.TransportLayerCC` status chunks), so that a packet the recorder
    drops is seen.
  - **feedback timing**: the number of feedbacks sent, the mean and the
    longest time between two, and the share of intervals over 200 ms (twice
    the bot's interval; a browser's longest is 250 ms).
  - **stamp lag**: the mean and the longest time between a packet's socket
    read and its recording for feedback. It measures how far the arrival
    time the server is told can be from the real one.
- **Limits** (FR-006), constants in one place: a bot is named when its
  acknowledged share is under 0.99, or more than 1% of its feedback
  intervals are over 200 ms. They are revised once if the browser
  comparison shows a tighter or looser bound matters, and the contract is
  updated.
- **Reasons** (FR-007): `no_video` (the run has no video),
  `not_negotiated` (the server's offer has no transport-wide sequence
  number), `not_counted` (a test replaced the network, so the socket is not
  seen).
- **Where it goes**: `quality.reception_reports`
  ([contract](contracts/report.md)): figures per bot in the JSON file,
  percentiles and the named bots in the Markdown file, and a warning line
  on the layered results when a bot is named.
- **Rationale**: FR-004 to FR-007, SC-002. The stamp lag is not asked by
  name; it is how "late" is made visible for the cause R8 lists first.
- **Alternatives considered**: counting arrivals at the read buffer (cannot
  see packets Pion drops before it, which is one of the suspected faults);
  capturing packets (what the story is meant to avoid).
- **Test of SC-002**: a test-only switch makes one bot skip the recording
  of a share of its packets; its acknowledged share drops and the report
  names it.

## R7. Reading the server's estimate, per receiver, once per second

Validation tooling, not part of the engine or the run report (clarification
of 2026-11-13): a script, `scripts/read-estimates.py <server>`, that writes
one line per receiver per second to a JSONL file kept with the validation
report (`specs/015-bandwidth-estimate-parity/estimates/`).

- **mediasoup** — **Decision (after R12)**: mediasoup gives no
  per-receiver estimate to read without changing its code: the worker logs
  its outgoing estimate only in development builds, the demo enables no
  `bwe` trace event, and `getTransportStats` answers only about the asking
  peer's own transports. The comparison uses the layer forwarded over
  time, as the spec's edge case says: the browser's heights per second
  from its report part, and each bot's height changes from an `Info` log
  line the engine writes when the height a bot receives changes
  (`Received video height changed`, with the bot, the height and the
  offset from the run's start). The validation report says so.
- **Jitsi** — **Decision**: poll the bridge's private HTTP debug interface
  once per second: `GET http://127.0.0.1:8080/debug/<conference>?full=true`,
  field `endpoints.<id>.bitrate_controller.bitrate_allocator.bweBps`, and
  `allocation.has_suspended_sources` beside it (checked in R12, enabled in
  the local setup). A bot's endpoint id is the first 8 hex digits of its
  JID's local part; the browser is the endpoint with a `stats_id`.
- **Where a server gives no estimate**: the forwarded layer over time, from
  the run report (spec edge case); the validation report says so.
- **Rationale**: FR-002. Both methods read the server's own value and need
  no change to the server's code.

## R8. What could make the server estimate a bot lower than a browser

Nothing here is a finding. It is the list the comparison (US1) and the
measures (R6) are built to sort, most likely first.

1. **Arrival times stamped late and unevenly.** The server's delay-based
   estimator reads growing delay between packets as congestion, with no
   packet lost — which is what both servers show. The bots stamp after
   decryption, in a goroutine the scheduler may delay; a browser stamps at
   the socket. Ten bots, a server and a browser on one laptop make this
   worse. Seen by: stamp lag (R6), and the comparison on the remote server
   (SC-006).
2. **Packets that reach the socket and are never acknowledged.** Packets of
   a stream Pion has not opened yet, the server's probe stream, packets the
   replay check rejects. The server counts them as lost. Seen by: the
   acknowledged share (R6).
3. **Feedback sent in bursts.** A delayed ticker sends two feedbacks close
   together after a gap; the server's estimator sees no acknowledgement for
   a while. Seen by: feedback timing (R6).
4. **Feedback the recorder builds wrongly** for reordered packets or large
   gaps. Seen by: acknowledged counted from the feedback packets against
   arrived (R6).
5. **Receiver reports missing for streams the bot drops**, where a server
   takes round-trip time or loss from them. Seen by: comparing with the
   browser, and reading the server's state for the bot (R7).
6. **REMB advertised and never sent.** If a server prefers REMB when both
   are offered, it waits for an estimate that never comes. **verify** per
   server which feedback it uses for a bot and for the browser.
7. **The server's own ramp-up.** mediasoup starts each receiving session at
   1 Mbit/s and probes upward; the bridge allocates within its estimate.
   If the browser is held the same way, the verdict is "the server".

## R9. Server settings in the report

- **Decision**:
  - `src/remote` declares, per server, the settings its remote overlay
    changes from the server's defaults and that change results
    (`Server.Settings`; today Jitsi's `JVB_CC_TRUST_BWE=false`, none for
    the others). A test checks each overlay against the declaration.
  - `sfu-remote deploy` writes them to the settings file as
    `<PLATFORM>_SERVER_SETTINGS` (`name=value` pairs, comma-separated; the
    variable is present and empty when the deployment changes nothing).
  - The live tests and `jitsi-connect` (`--server-settings`) put the
    variable into the test plan's `server_settings` with `source:
    deployment`. An operator writes the section by hand with `source:
    operator`.
  - The report shows three cases: the values with their source; "none
    changed by the deployment" (source `deployment`, no value); "not known
    to the run" (no section). It never says "defaults" on its own
    (FR-012, US4 scenario 3).
- **Rationale**: FR-012, SC-007. The run learns the settings the same way it
  learns the server's address: from the settings file of 013.
- **Alternatives considered**: the engine reading the server's
  configuration over Docker (per-server code in the engine, and nothing for
  a server deployed by other means).

## R10. Report layout

- **Decision**: `FormatVersion` 3. Three additions
  ([contract](contracts/report.md)): `quality.reception_reports`, a
  top-level `browser`, and `server_settings` shown from the test plan.
  Existing fields keep their names and meaning. `RunResult` gains the
  browser's result.
- A run without a `browser` section has no `browser` object in its report,
  and its Markdown has no browser section (FR-018).

## R11. The comparison and its verdict

- **Need of a receiver** ("what its streams need"), as built after R12:
  on Jitsi, the bridge's own `idealBps` for that receiver (what it would
  send with no limit), read beside the estimate; reached when the estimate
  is at or above it with no suspended stream. On mediasoup, which gives no
  estimate, "at its need" is every stream it decodes at the height it asked
  for (the browser: its `expected_height`). The browser's streams and
  heights differ from a bot's (its page asks for every participant, a bot
  for 4 at most): the validation report states it (US1 scenario 5).
- **Reached**: the first reading at its need, counted from the receiver's
  first reading (the browser: from its page's load).
- **Held low**: a bot whose estimate is under its need for more than 1 s
  after the browser reached its own (FR-008's margin), or that falls back
  under it later while the browser does not.
- **Verdict per server**: "the bots" when at least one run shows a bot held
  low and the browser not; "the server" when the browser is held low as the
  bots are; both when the browser is held and bots for longer; "not
  reproduced" after five runs with no bot held low (FR-003).
- **Runs**: the 014 conditions of FR-001, the browser asked for in the test
  plan with a window of 1280x720, a warm-up run first on Jitsi, the
  engine's and the browser's CPU noted with each run.
- **Browser address**: one at which the page joins by itself and sends
  nothing. **verify** for each page: the mediasoup demo's room address with
  sending turned off, and the Jitsi Meet address that skips the pre-join
  page, starts muted and shows the tile view.

## R12. Facts to check first (one live task)

Before code depends on them:

1. Headless Chrome with the flags of R1 plays received video on both pages,
   and `getStats` gives `frameHeight` and `freezeCount` (R1, R2).
2. Both pages create their connections in the main frame (R2).
3. The join-by-itself addresses of both pages (R11).
4. The mediasoup log lines for per-transport estimates, their rate, and the
   tie to a participant (R7).
5. The bridge's debug address, whether it is enabled locally, and the
   estimate's field (R7).
6. Which feedback each server uses toward a bot and toward the browser
   (R8, item 6).
7. The browser's CPU time as read at exit covers its child processes (R4).

A fact that does not hold changes the method named with it, not the
feature; the fallback is written beside each.

## R13. What a "bots" verdict leads to (US3)

Built only for the cause the comparison and the measures point to:

| Cause (R8) | Correction |
|---|---|
| 1 | record the arrival where the socket is read (`consentNet`), with that time; the read buffer no longer stamps |
| 2 | acknowledge from the socket-level record, so no packet depends on Pion opening a stream |
| 3 | send feedback from a timer that does not drift, or on packet count as well as time |
| 4 | replace the recorder for the failing case, with a test from the recorded trace |
| 5 | receiver reports for every stream that arrives |
| 6 | stop advertising REMB |

Corrections 1 and 2 share one change: recording at the socket parses the
RTP header once there instead of once at the buffer, so the per-packet cost
does not grow (FR-015; measured at 100 bots before and after).

After a correction: the comparison is run again (SC-003), then the layered
runs of SC-004 and SC-005, the LiveKit and Janus runs of SC-008, and, if
FR-010 holds on the remote machine, `JVB_CC_TRUST_BWE` leaves the remote
overlay and `Server.Settings` (FR-011).

## R14. What was built, after the verdicts (2026-11-14)

- **Jitsi, R8 cause 2 (corrections 1 and 2 of R13)**: the bots record
  every packet for feedback where the socket is read
  (`transportFeedback.arrive`, from `consentConn`), so the copies the
  bridge sends with their own transport sequence number, which Pion's SRTP
  replay check drops, are acknowledged. The read buffer records only when
  a test replaced the network (`recordFromBuffer`); the stamp lag is
  measured on that path only.
- **Jitsi, R8 cause 1, user's choice after AV1 suspensions at start-up**:
  kernel receive timestamps (`SO_TIMESTAMPNS`) on the receiving sockets
  were built and tried: the AV1 start-up suspensions were the same (2 of 5
  runs with and without), so they were removed (user's choice): no measured
  benefit, one allocation more per packet.
- **mediasoup**: nothing changed in the bots. Root cause (2026-11-15): a
  worker bug, every transport's probe generator sharing one packet buffer
  (`static thread_local` in `RTP/ProbationGenerator.cpp`), so new transports
  reset the others' probe sequence numbers and their SRTP refuses the
  probes; verdict "the server". Confirmed with a patched debug worker (no
  refusal, every 720 bot at 0.985 or more at ramp-up 0) and by a 20 s
  ramp-up on the release worker. A receive section for the probe stream
  (mediasoup-client's "probator") was considered and not built. A validation option,
  `log_server_stats`, logs mediasoup's estimate and layer decisions per
  bot.
