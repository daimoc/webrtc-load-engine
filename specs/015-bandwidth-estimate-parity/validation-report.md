# Validation report: Bandwidth Estimate Parity with a Browser (015)

Started 2026-11-14 at `096dc475d55d` (branch `rework`), with the 015 spec,
plan and tasks uncommitted. Machine: 16 hardware threads, Google Chrome
156.0.8078.79.

`go vet ./... && go test -race ./...` at the start: green (every package
`ok`, `src/execution` 40.4 s).

## Facts (R12)

Checked 2026-11-14 on the local servers with a DevTools probe (Node 24,
headless Chrome 156 with the R1 flags; script in the session's scratch
folder, not kept), 2 bots sending video.

1. **Headless Chrome plays received video on both pages, and `getStats`
   gives the R2 fields.** Jitsi Meet: 2 inbound video streams at 360 lines
   (bots without simulcast), `framesDecoded` +30/s, `freezeCount` and
   `totalFreezesDuration` 0, `bytesReceived` about 63 kB/s each. mediasoup
   demo: 2 streams at 720 lines with the bots' 720 simulcast, same fields.
   mediasoup also opens its RTP probation stream (`ssrc` 1234): bytes and
   no decoded frame, so the R2 rule "a stream counts in a second when it
   decoded a frame" leaves it out.
2. **Main frame on both pages** (`window.frames.length` 0 while
   receiving). **Departure from R2**: wrapping the `RTCPeerConnection`
   constructor sees no connection on the mediasoup demo page (its
   `@rtcstats/rtcstats-js` replaces the constructor with one around the
   native one). Patching the native prototype's methods
   (`setLocalDescription`, `setRemoteDescription`, `addTransceiver`,
   `addTrack`, `createDataChannel`, `createOffer`, `createAnswer`) to note
   `this` sees every connection on both pages; the injected script does
   that. Closed connections are skipped when sampling.
3. **Join-by-itself addresses**, sending nothing:
   - Jitsi Meet: `https://localhost:8443/<room>#config.prejoinConfig.enabled=false&config.startWithAudioMuted=true&config.startWithVideoMuted=true`
     (with `insecure_tls`, self-signed certificate). The page shows stage
     view: one tile large, the others as thumbnails, so its requested
     heights depend on the window and the number of participants.
   - mediasoup demo: `https://localhost:5555/?roomId=<room>&produce=false&mic=false&webcam=false&displayName=browser`
     with `insecure_tls`. The web app is not part of the local setup: it is
     the app of the pinned commit, run with Vite
     (`CONFIG_FILE=tests/platforms/mediasoup/config.mjs`, and the server's
     `.env` with `MEDIASOUP_TLS_CERT`/`MEDIASOUP_TLS_KEY` set, as the
     setup's README says). The bots then join with
     `MEDIASOUP_URL=wss://localhost:4443 MEDIASOUP_INSECURE_TLS=1`. The
     app does not adapt preferred layers to tile size: the browser asks
     for the highest layer, as a bot asking for 720.
   - Chrome's first navigation to the Jitsi page answered
     `errorText: net::ERR_CONNECTION_CLOSED` although the page loaded and
     joined. `page_load_failed` is therefore decided on the page's state
     after the navigation (main frame URL `chrome-error://…` or no load
     within 15 s), not on `errorText` alone.
4. **mediasoup gives no per-receiver estimate to read.** The worker logs
   its outgoing available bitrate only in development builds
   (`MS_DEBUG_DEV` in `Transport.cpp`), the demo enables no `bwe` trace
   event, and its `getTransportStats` request answers only about the
   asking peer's own transports. Per the spec's edge case ("the server
   gives no estimate to read"), the mediasoup comparison uses the layer
   forwarded over time: the browser's heights per second (its report
   part) and the bots' (a log line per height change, added for this,
   T024). Debug logging (`MEDIASOUP_LOG_LEVEL=debug`, `mediasoup:Channel*`)
   was tried and shows no estimate.
5. **Jitsi bridge debug state**: `GET http://127.0.0.1:8080/debug` lists
   conferences; `GET /debug/<conference>?full=true` gives per endpoint
   `bitrate_controller.bitrate_allocator.bweBps` (the estimate),
   `trustBwe`, `allocation.has_suspended_sources` and
   `allocation.suspended_sources`. Enabled in the local setup as it is
   (bridge 2.3.326). Endpoints are keyed by id: a bot's id is the first 8
   hex digits of its JID's local part (`02bc1d77-…@meet.jitsi`); the
   browser's endpoint is the one with a `stats_id` (display name), bots
   have none.
6. **Feedback used: transport-cc everywhere, for bots and browser.**
   Jitsi: `RTCP termination.num_RtcpFbTccPacket_rx` grows for both bots
   (about 10/s) and the browser (about 15/s); no REMB is created. mediasoup:
   the browser's remote description has `transport-cc` and the
   transport-wide extension and no `goog-remb`; the bots get the same from
   the consumers' RTP parameters (adapter `sdp.go`). R8 item 6 (REMB) is
   ruled out on both servers.
7. **Browser CPU: the process group, not Chrome's own process.** A 15 s
   run on the mediasoup demo page (2 streams at 720): Chrome's own process
   reports 2.16 s of CPU at its end (`ProcessState` user + system), the
   process group summed from `/proc/<pid>/stat` 14.8 s (0.99 thread). The
   renderer and GPU processes are not waited for when the group is killed,
   so the session sums the group (`src/browser/cpu.go`), as R4's fallback
   said.

First signal, not a finding: in a 2-bot mediasoup run with the browser
joining twice, one bot asking for 720 had it 55% of its time (the other
95%), while the browser received 720 on both streams 10 s after joining.

## Baselines

Engine CPU per bot at 100 bots, conditions of 014 T063 (local LiveKit
1.13.10 restarted before each run, 100 bots `audio_video` VP8 single
stream, receive cap 10, 20 s ramp-up, 2 minute hold), at `096dc475d55d`,
2026-11-14 00:26–00:37, `load_machine.engine.cpu_threads.average` / 100:

| Run | Engine CPU (threads) | Per bot | Freezes |
|---|---|---|---|
| base-1 | 4.005 | 0.0401 | 0 |
| base-2 | 4.165 | 0.0417 | 0 |
| base-3 | 3.897 | 0.0390 | 0 |

Mean 0.0403 per bot (014 T063 had 0.0435 and 0.0437). The live test ends
the engine with Stop, so the reports say `stopped`. Unit tests of this
feature ran on the same machine during base-2 and base-3, so these are
preliminary: FR-015 is decided by runs alternated before and after the
change in one session (T035).

## Verdicts

Gate T040: shown to the user before any correction is built.

| Server | Verdict | Runs behind it | Cause the measures point to |
|---|---|---|---|
| Jitsi stable-11248 (local, default bridge settings) | **the bots** | VP8 run 4, AV1 run 1 (below); VP8 runs 1–3 and 5 without the problem | R8 cause 2: the bots do not acknowledge the bridge's duplicate packets (Pion's SRTP replay check drops them before the feedback recorder) |
| mediasoup 3.30.1 (local) | **the bots** | VP8 run 3; runs 1 and 2 without the problem | not identified: every bot acknowledged 100%, feedback regular, stamp lag under 4 ms |

On both servers the browser, in the same room, received more than any bot
(10 streams against the bots' cap of 4 on mediasoup; a larger need on
Jitsi) and was never held low, with 0 freezes. SC-006 (a "bots" verdict
confirmed with the server on another machine) is not done yet (T039).

**What differs between the browser and a bot** (US1 scenario 5):

- mediasoup: the demo page consumes every participant (10 streams) at the
  highest layer, without `setConsumerPreferredLayers`; a bot consumes 4 and
  asks for its layer. The browser's need is therefore 2.5 times a
  720-bot's.
- Jitsi: the page shows stage view in a 1280x720 window and asks the
  bridge for its own heights; the bots send single 360-line streams, so
  every stream is 360 for both. The bridge's need for the browser (`ideal`
  5.3 Mbit/s in VP8 run 4) is above a bot's (4.7 Mbit/s).
- Both: the browser sends nothing; each bot sends audio and video.

## SC-001: estimates for a browser and the bots

2026-11-14 01:28–01:47, local servers one at a time, the engine's headless
Chrome in the room (T037, T038). Files in `estimates/`: per run the two
report files (`<server>-<codec>-<n>/`), the live test's log
(`<server>-<codec>-<n>.log`) and, for Jitsi, the bridge's estimates
(`jitsi-<codec>-<n>.jsonl`, one line per receiver per second, R7). The
logs and estimate files are kept compressed with `xz` (`.log.xz`,
`.jsonl.xz`; `estimates/README.md`).

### Jitsi, default bridge settings (T038)

10 bots `audio_video` single stream, 60 s hold, the bridge warm (it had run
several 10-bot rooms since its start). Browser: Jitsi Meet page, stage
view, 1280x720, `expected_height` 360.

| Run | Bot freezes | Bots flagged (unacknowledged) | Lowest acknowledged share | Bots under their need > 1 s after the browser | Browser |
|---|---|---|---|---|---|
| VP8 1 | 0 | 0 | 1.000 | none | measured, 0 freezes, estimate 20 Mbit/s throughout |
| VP8 2 | 0 | 0 | 1.000 | none | the same |
| VP8 3 | 0 | 0 | 1.000 | none | the same |
| VP8 4 | **15** | **1** | 0.985 | **1** (`95130814`, 10 s; peer 4) | the same |
| VP8 5 | 0 | 0 | 1.000 | none | the same |
| AV1 1 | **37** | **9** | 0.924 | **6** (3 to 24 s) | the same |

VP8 run 4: the bot held low is the bot flagged and the bot that froze
(peer 4, endpoint `95130814`: 48 s of freezes, 535 of 35,595 packets
unacknowledged). The bridge's estimate toward it fell to 2.56 Mbit/s against
a need of 4.65 Mbit/s, with streams suspended, for 10 s; toward the browser
it stayed at the bridge's 20 Mbit/s ceiling for the whole run. In AV1 run 1
the median bot acknowledged 96%.

The estimate is read as `-1` for each endpoint's first second (no estimate
yet); `estimate-verdict.py` leaves those readings out.

### mediasoup (T037)

10 bots VP8 simulcast up to 720, receive cap 4, `180:0.5,720:0.5`, 60 s
hold, server TLS on (the demo app needs it). Browser: the demo page,
`produce=false`, `expected_height` 720. mediasoup gives no per-receiver
estimate (R12 fact 4): the comparison is on the layer forwarded over time,
the browser's per-second heights against the bots' height changes.

| Run | Lowest share at the requested height (720 bots) | Bots flagged | Browser |
|---|---|---|---|
| VP8 1 | 1.000 | 0 | 720 99.3% of the time, first after 1.0 s, 10 streams |
| VP8 2 | 0.605 (two bots at 0.61) | 0 | `no_video_received` (the page loaded and received nothing: no comparison from this run) |
| VP8 3 | **0.278** (peer 5: 720 first after 52 s, under it 50.8 s after the browser + 1 s) | 0 | 720 99.1% of the time, first after 1.0 s, 10 streams |

In run 3 every bot acknowledged 100% of its packets, its longest feedback
interval was 102 ms (one interval of 600–700 ms on four bots, 0.14% of
them, within the limits) and its longest stamp lag under 4 ms. The
other four 720 bots reached 720 under 2 s after the browser (0.8 s past the
1 s margin, at the browser's 1 s sampling).

Run 1's table from `estimate-verdict.py` compared no bot: the bots' first
height lines said `requested_height=0` (the requested height was set after
the first collection). Fixed in `quality_collector.go` before run 2; run 1's
report gives every bot 1.000 at its requested height, so no bot was held
low there.

## SC-002: reception reports

### One 10-bot run per server (T036)

2026-11-14 01:19–01:27, local servers one at a time, 10 bots
`audio_video` single stream VP8, 60 s hold, default settings everywhere.

| Server | Reception reports | Acknowledged share (p50 / lowest bot) | Late feedback share p95 | Longest stamp lag p95 | Flagged | Freezes |
|---|---|---|---|---|---|---|
| LiveKit 1.13.10 | `not_negotiated` (no transport-cc offered, SC-008 section) | | | | | 0 |
| Janus 1.4.3 | measured | 1.000 / 1.000 | 0 | 5.6 ms | none | 0 |
| mediasoup 3.30.1 | measured | 1.000 / 1.000 | 0.0015 | 3.1 ms | none | 0 |
| Jitsi stable-11248 | measured | 0.900 / 0.887 | 0 | 2.8 ms | **8 of 10** | **31** |

An audio-only LiveKit run gives `available: false`, `reason: no_video`
(the live test itself fails that run on its own check that bots receive
video, unrelated to 015). SC-002's other half, a bot made to leave packets
unacknowledged, is `TestReceptionReports_UnacknowledgedBotNamed`: with 1
packet in 20 skipped, that bot acknowledges about 0.95 and is the only one
flagged.

**Finding (Jitsi): the bots do not acknowledge duplicate packets.** On the
local Jitsi with its default settings, 8 bots acknowledged about 90% of the
packets that reached their socket, in the same run that froze 31 times.
Feedback left every 100 ms (longest interval 102 ms) and arrival times were
stamped within 3 ms of the socket read, so R8 causes 1 and 3 are not it.
Instrumented runs (a temporary per-SSRC count, not kept) found which
packets are missing:

- every packet that reached the socket and was not recorded for feedback
  repeats an RTP sequence number the bot had already received on the same
  stream (`missed` = `missed_dup` on every stream of every bot, two 10-bot
  runs);
- the bridge sends these copies with a new transport-wide sequence
  number, so for the bridge they are packets of their own;
- Pion's SRTP replay check drops the copy before the read buffer, where the
  bots record for feedback, so the bot tells the bridge the packet was lost
  (R8 cause 2);
- the amount varies: in one run one bot had 1389 such copies on one stream
  (4536 packets received against 3138 sent) and was the only bot affected;
  in another, 8 bots lost 10% each; at 4 bots, none.

Whether a browser receives such copies, and whether it acknowledges them,
is what the comparison (T038) settles.

## SC-003: bots reach their need within 1 s of the browser

### The Jitsi correction (T041, T042)

The bots now record every packet for feedback where the socket is read,
with that time (`transportFeedback.arrive`, called from `consentConn`); the
read buffer records only when a test replaced the network. Duplicates the
SRTP replay check drops, streams Pion never opens and probe packets are all
acknowledged. Unit tests: `TestTransportFeedback_RecordsAtTheSocket`,
`_ArrivalTimeIsTheSocketTime`, `_BufferRecordsWithoutSockets`.

2026-11-14 09:14–09:46, local Jitsi with default settings, after a warm-up
run, the conditions of T038 (10 bots single stream, ramp-up 0, 60 s hold,
browser in the room). Files `estimates/jitsi-<codec>-fixed<n>`,
`-ab{buf,sock}<n>`, `-ramp<n>`.

| Runs | Bots flagged | Readings with suspended streams | Bots under their need > 1 s after the browser | Bot freezes | Browser |
|---|---|---|---|---|---|
| VP8 fixed 1–5 | 0 in all | 0 in all | none | 16, 10, 10, 30, 0 | 0 freezes, 20 Mbit/s |
| AV1 fixed 1–5 | 0 in all | 31, 0, 0, 35, 0 | 5 (6–7 s, start), 0, 0, 1 (35 s from 5 s), 0 | 36, 24, 20, 16, 0 | the same |

Every bot acknowledged at least 99.99% in every run.

**A/B, VP8, alternated, the same session** (a temporary switch put the
recording back at the read buffer; removed afterwards):

| Recording | Freezes | Readings with suspended streams | Bots held low |
|---|---|---|---|
| read buffer (before) | 34, 5, 21 | 142, 0, 66 | 5, 0, 2 |
| socket (correction) | 5, 10, 0 | 0, 0, 0 | 0, 0, 0 |

The correction removes the bridge's suspensions in VP8.

**Freezes that remain are not the estimate.** In every VP8 run after the
correction, all freezes are logged in one collection 4 s after the start,
just after the 10 bots are established at once: about 1 s on some streams
(mostly the last bots to join), with 0 packets lost, 0 stalls and no
suspended stream. They happen in both arms of the A/B (5 freezes in a
buffer run without suspension), and not in last night's runs: a start-up
effect of this session, separate from the bandwidth estimate. SC-005's
"0 freezes" is therefore not met in these runs.

**AV1 at start-up.** The two AV1 runs with suspensions after the
correction have them only from the first seconds: the bridge's estimate
toward a bot jumps (13.4 Mbit/s), then drops once to 0.85 of what the bot
was receiving (2.66 Mbit/s) and climbs back by about 15% every 5 s, as
when the delay-based estimator sees one overuse; the bot had received
about 500 retransmitted packets more than the others. The browser in the
same room is never suspended. With a 10 s ramp-up instead of 0 (three AV1
runs, `-ramp<n>`), no stream is suspended and the runs have 2, 0 and 0
freezes. Likely cause (not proven): when 10 bots join at once, the engine is
busy and its socket reads come late, so the arrival times it reports carry
delay that is not on the network (R8 cause 1). A correction would take the
arrival time from the kernel (receive timestamps on the socket) instead of
the read; not built.

**Kernel receive timestamps, tried and removed (user's choice).** To test
the start-up explanation, the receiving sockets took the kernel's arrival
time (`SO_TIMESTAMPNS`, read with `ReadMsgUDP`; a unit test showed a
datagram read 100 ms late reported at its kernel time). Ten runs,
2026-11-14 10:17–10:31, after a warm-up, ramp-up 0 (`estimates/jitsi-<codec>-kts<n>`):

| Codec | Readings with suspended streams | Bots held low | Freezes |
|---|---|---|---|
| AV1 | 25, 32, 0, 0, 0 | 4, 4, 0, 0, 0 | 31, 40, 0, 16, 0 |
| VP8 | 0 in all | none | 20, 0, 2, 18, 13 |

The same as without them (AV1 suspensions in 2 runs of 5), so the late
socket read is not the cause of the AV1 start-up suspensions. The
timestamps were removed: no measured benefit, and one allocation more per
packet. A candidate not tested: in the first two seconds the bridge cannot
parse the AV1 dependency descriptor of packets sent before a keyframe (014
T055); the browser receives the same streams and is not suspended.

SC-003 itself (every bot within 1 s of the browser in five runs out of
five) is met in VP8 (5 of 5, twice) and not in AV1 at ramp-up 0 (3 of 5,
twice); with a 10 s ramp-up, AV1 3 of 3.

## SC-004: mediasoup, 720 at least 95% of the time

Not met: nothing was changed for mediasoup (verdict "the bots", mechanism
below). In 17 layered VP8 runs at 10 bots on 2026-11-14 the lowest 720-bot
share ranged from 0.38 to 1.000; the other codecs were not rerun, since
the bots are the same as in 014.

### Investigation of the mediasoup verdict (user's choice, 2026-11-14)

A validation-only adapter option, `log_server_stats`
(`MEDIASOUP_LOG_SERVER_STATS=1`), logs per bot the demo's
`consumerLayersChanged` and `consumerScore` notifications and, once per
second, `getTransportStats` of the bot's receiving transport (mediasoup's
`availableOutgoingBitrate` toward that bot, its send and probation
bitrates). Runs 2026-11-14 09:39–09:57, local mediasoup with plain `ws://`,
the T037 conditions without a browser; logs in the session's scratch folder.
Some runs also had temporary instrumentation (not kept) counting every
RTP packet per SSRC at the bot's socket, and the arrival of mediasoup's
probe packets (SSRC 1234).

What separates a bot held low from one that is not:

1. **Its estimate never leaves the start.** A held bot's
   `availableOutgoingBitrate` starts at the demo's 1 Mbit/s and grows by
   about 10% a second (1.0, 1.4 at 6 s, 2.3 at 15 s, 4–8 at 39 s); mediasoup
   raises its consumers one layer at a time, about every 3 s
   (`consumerLayersChanged`: 0 → 1 → 2, consumer by consumer, over 35 s). A
   bot not held low has 17–28 Mbit/s within its first second and all four
   consumers on layer 2 at once, as the browser had in T037.
2. **mediasoup says it probes the held bot, and the probes do not reach
   it.** For bot 5 of run `drop2` (0.59 at 720), mediasoup reports probation
   sent at 31.6, 27.5, 24.1, 20.7, 17.6 and 15.4 Mbit/s every 2 s; that bot's
   socket read 8 packets of SSRC 1234 in the whole run, none with a
   transport-wide sequence number. Bot 6 of the same run (not held) read
   479 with it, in clusters at 0, 3 and 8 s (up to 53 Mbit/s).
3. **Not the bots' feedback, not the kernel.** Every bot acknowledged every
   packet that reached its socket (packets arrived − acknowledged = 0 for
   all bots of three runs); the host's UDP `RcvbufErrors` did not grow over
   two runs with held bots.
4. **Which bots are held changes from run to run** (5–7, 6–9, 7 alone,
   5 alone), among those asking for 720; at 180 a bot needs only 0.6 Mbit/s,
   under the start value, so it does not show.

Mediasoup's probe packets carry fixed header extension IDs (MID 1,
abs-send-time 4, transport-wide-cc 5, worker `RTP/ProbationGenerator.cpp`),
and a probe gets a transport-wide sequence number only if it carries that
extension (`Transport::OnTransportCongestionControlClientSendRtpPacket`).
The one difference with the browser seen so far: mediasoup-client creates a
receive section for the probe stream (`mid` "probator", SSRC 1234, logged by
the demo page as "Consumer for RTP probation created"); the bots' receive
session has none. Nothing was changed in the bots
for mediasoup.


**Where the probes go: mediasoup refuses to encrypt them.** The bots'
sockets read nothing of them, the host's UDP `NoPorts` grew by 20 over a
run with two held bots and `RcvbufErrors` not at all: the probes never
leave the server. mediasoup's own log has `RTC::SrtpSession::EncryptRtp()
| srtp_protect() failed: replay check failed (index too old)`, and
`webrtc::ProbeController::Process() | kWaitingForProbingResult: timeout`
(its probes went unanswered), in proportion to the bots held low, four
runs in a row (2026-11-14 10:05–10:10):

| Lowest share at 720 | Bots under 0.95 | SRTP replay warnings | Probe timeouts |
|---|---|---|---|
| 0.51 | 3 | 7,261 | 40 |
| 0.475 | 1 | 777 | 2 |
| 0.971 | 0 | 731 | 10 |
| 0.989 | 0 | 52 | 0 |

The probe generator (`RTP/ProbationGenerator.cpp`) numbers its packets
forward from a random start, one generator per transport; the refusal is
in the server's sending, which no receive-side setting of a bot reaches.
The probe section suggested above (mediasoup-client's "probator") would
only let a bot accept SSRC 1234: it was not built, since the probes are
refused before they leave. What in the bots' sessions makes mediasoup
refuse them, and a browser's not (never held in T037), is not found. The
"too many tuples, removing the oldest non selected one" warning comes once
per bot in every run, held or not.

## SC-005: local Jitsi, default bridge settings

Not met as worded (0 freezes in five VP8 and five AV1 runs). With the
correction, after a warm-up, ramp-up 0: no VP8 run has a suspended stream
(15 runs), and freezes come once, about 1 s each, 4 s after the start
while the 10 bots join (0 to 30 per run, also with the old recording in the
A/B, no loss); AV1 has suspensions at start-up in 2 runs of 5 (twice), none
with a 10 s ramp-up. The browser in the room had 0 freezes in every run.
Details under SC-003.

## SC-006: a "bots" verdict on another machine

2026-11-14 22:50–23:10, Jitsi `stable-11248` deployed with `sfu-remote` on
the 013 machine (now `fetis@192.168.1.59`, its DHCP address changed; IPv6
had to be disabled again by the user after a reboot), with the remote
overlay **without** `JVB_CC_TRUST_BWE` (a temporary edit, restored after),
so the bridge used its estimate as by default. After a warm-up: 10 bots VP8
single stream, ramp-up 0, 60 s hold, from the workstation over the LAN;
the headless browser on the remote Jitsi Meet page (the browser package's
live test, so that the uncorrected bots, a worktree at `096dc475d55d`, have
the same browser); the bridge's estimates through an ssh tunnel to its
debug interface. Alternated, files `estimates/remote-jitsi-vp8-<arm><n>`.

| Run | Bots | Suspended readings after the first 20 s (receivers) | Bots held low against the browser | Bot freezes |
|---|---|---|---|---|
| head 1 | uncorrected | 342 (8 bots) | 9 | 35 |
| fixed 2 | corrected | 1 (the browser) | 0 | 45 |
| head 2 | uncorrected | 264 (8 bots and the browser) | 8 | 37 |
| fixed 3 | corrected | 0 | 4 (start-up: suspended to 16 s, the browser to 12 s) | 34 |
| head 3 | uncorrected | 16 (3 bots) | 4 | 26 |
| fixed 4 | corrected | 0 | 0 | 0 |
| fixed 1 (before the A/B) | corrected | 0 | 0 | 31 |

(In head 1 the browser's page did not load: `page_load_failed`; the
comparison of that run is against the bots that were not held.)

**The verdict "the bots" holds on another machine**: with the uncorrected
bots the bridge keeps suspending several bots' streams for the rest of the
run; with the correction it suspends none after the start.

**And part is the server's**: on this machine the bridge starts every
receiver, the browser included, at a 2.5 Mbit/s estimate against a need of
about 6 Mbit/s, and suspends streams for the first 10 to 20 s, as long for
the browser as for the bots (fixed 1: all until 13 s; fixed 2: the browser
until 20 s). Most freezes of the corrected runs are in that window; the
browser counts 1 freeze in its own rule. Locally the bridge's estimate rose
to its 20 Mbit/s ceiling at once (loopback), so this did not show there.

**FR-011 (T050)**: on these results the user chose, 2026-11-14, to return
the remote Jitsi to the bridge's default: `JVB_CC_TRUST_BWE` is removed from
`tests/platforms/docker-jitsi-meet/docker-compose.remote.yml` and from
`Server.Settings` (`TestJitsiRemote_DefaultBandwidthEstimate`,
`TestOverlays_DeclareTheirSettings`); a deployment's settings file now
gives an empty `JITSI_SERVER_SETTINGS` ("none changed by the deployment").
FR-010's five passing VP8 runs on the remote machine are not met as worded:
the corrected runs have freezes in the bridge's start-up window, which the
browser has too; that part, the server's, is documented in the README and
`tests/platforms/docker-jitsi-meet/README.md` (FR-013).

## SC-007: server settings in the report

2026-11-14 22:45, Jitsi deployed with `sfu-remote` and the repository's
overlay: the settings file has `JITSI_SERVER_SETTINGS='JVB_CC_TRUST_BWE=false'`;
a 3-bot `jitsi-connect` run with the settings file sourced gives
`test_plan.server_settings` = `{"source": "deployment", "values":
{"JVB_CC_TRUST_BWE": "false"}}` in the JSON report and `Server settings |
JVB_CC_TRUST_BWE=false (stated by the deployment)` in the Markdown. Local
runs with nothing stated say `not known to the run` (T025, T026). The
other servers' settings files get an empty `<PLATFORM>_SERVER_SETTINGS`
(`TestSettingsVars`), which reads "none changed by the deployment".

## SC-008: LiveKit and Janus unchanged, engine CPU

### Engine CPU at 100 bots (T035, FR-015)

2026-11-14 01:00–01:17, the conditions of the baselines, alternated:
"before" is a worktree at `096dc475d55d` (no 015 code), "after" the
working tree with the browser package and the reception counters (no
browser asked for). LiveKit restarted before each run.

| Run | Before (threads) | After (threads) |
|---|---|---|
| 1 | 4.249 | 4.282 |
| 2 | 4.317 | 4.295 |
| 3 | 4.315 | 4.295 |
| Mean | 4.294 (0.0429 per bot) | 4.291 (0.0429 per bot) |

After is 0.1% under before, within 5% (FR-015); 0 freezes in all six.
(The earlier baselines, 0.039–0.042 per bot, ran beside unit tests.)

**With the correction (T042, T051)**, 2026-11-14 10:36–10:53, the same
conditions, alternated with a worktree at `096dc475d55d`, the final code
(socket recording, no browser asked for):

| Run | Before (threads) | After (threads) |
|---|---|---|
| 1 | 4.385 | 4.414 |
| 2 | 4.390 | 4.398 |
| 3 | 4.362 | 4.401 |
| Mean | 4.379 (0.0438 per bot) | 4.404 (0.0440 per bot) |

After is 0.6% over before, within 5% (FR-015, SC-008); 0 freezes.

### No change on LiveKit and Janus (T051)

The 014 T059 runs (10 bots, layers up to 720, receive cap 4,
`180:0.5,720:0.5`, 1 minute hold), one per codec, 2026-11-14 10:53–11:05;
lowest bot's share at its requested height, against 014's table:

| Server | VP8 | H.264 | VP9 | AV1 |
|---|---|---|---|---|
| Janus 1.4.3 | 1.000 (014: 1.000) | 1.000 (1.000) | 1.000 (0.990) | 0.991 (0.991) |
| LiveKit 1.13.10 | 0.989 (0.985) | 0.985 (0.985) | 1.000 (1.000) | 1.000 (1.000) |

All within 0.02 of 014, 0 freezes. Janus's reception reports are measured
with no bot flagged; LiveKit's are `not_negotiated` (below). The unlayered
runs of T036 (10 bots, every server) are the "runs without layers" of
FR-014: no freeze on Janus, LiveKit and mediasoup.

**Finding: the bots send no transport-wide feedback to LiveKit.** In
these runs `reception_reports` is `not_negotiated`. LiveKit 1.13.10's
offer to a bot's receiving session has `abs-send-time` and `goog-remb`
and no transport-cc extension (logged from a 2-bot run), so the bots have
nothing to acknowledge; they send no REMB either. LiveKit met its 014
criteria anyway; it estimates toward a bot without either.

## SC-009: 014 open items

## FR-016 to FR-019: the browser in a run

Seen in the comparison runs: in mediasoup run 2 the demo page loaded and
received no video for the whole run (`no_video_received`); the run itself
completed with its bots measured, as FR-019 asks. Its cause is not known
(the page joins in every other run).

T025, local mediasoup with TLS and the demo app (R12 fact 3), 3 bots
`audio_video` single stream, 20 s hold, 2026-11-14 00:47–00:50, each run a
`MediasoupLive` test passing:

| Case | Browser part of the report |
|---|---|
| Demo address, `expected_height` 360, `insecure_tls` | `measured`: 3 streams, 360 100% of the time, first after 1.0 s, 1469 kbit/s, 0 freezes, 0.84 threads |
| `http://localhost:1/` | `unavailable`, `page_load_failed` |
| Demo address without `insecure_tls` (certificate refused) | `unavailable`, `page_load_failed` |
| Demo address of an empty room | `unavailable`, `no_video_received` |
| `PATH` without Chrome | `unavailable`, `browser_not_found` |
| No browser asked | no browser part, no "Browser started" log line, no Chrome process with a `wle-browser` profile after the run |

In every case the run completed and the 3 bots were measured (FR-019); the
report counts 3 bots, the browser apart (FR-017). JSON `format_version` 3;
the Markdown key figures read `Browser | 360 lines 100.00 % of the time ·
reached after 1.0 s · 0 freezes` and the load machine section `A browser ran
on this machine during the run: 0.84 threads on average, not counted in the
engine's figure.`

T026, local Jitsi (default settings), 3 bots `audio_video`, 30 s hold, the
Jitsi Meet address of R12 fact 3 with `insecure_tls`: `measured`, 3 streams
at 360 100% of the time, first after 3.0 s, 1404 kbit/s, 0 freezes, 0.66
threads; the `JitsiLive` test passes.

Two changes came from the first live runs: the session enables the
DevTools `Page` domain before installing its script (without it, the script
was not run on the demo page), and each sample is bounded to 5 s.

## Not met or not checked

- SC-004 (mediasoup, 720 at least 95%): not met; verdict "the bots", the
  held bots never receive mediasoup's probes, which mediasoup's own SRTP
  refuses ("replay check failed (index too old)"); what in those bots'
  sessions triggers it is not found; nothing changed for mediasoup.
- SC-005 (local Jitsi, 0 freezes in 5 VP8 and 5 AV1 runs): not met as
  worded. VP8: no suspended stream after the correction, but a one-off
  freeze of about 1 s on some streams 4 s after the start (with either
  recording, no loss). AV1 at ramp-up 0: start-up suspensions in 2 runs of
  5; none with a 10 s ramp-up.
- SC-003 in AV1 at ramp-up 0: 3 runs of 5 (twice).
- FR-010 on the remote machine: no suspension after the start with the
  corrected bots, but the bridge's 10–20 s start-up suspension (a
  browser's too) leaves freezes; documented as the server's.
- In one mediasoup comparison and one remote Jitsi run the browser's page
  did not join (`no_video_received`, `page_load_failed`); those runs
  completed with their bots measured.
- The bots send no transport-wide feedback to LiveKit (none offered):
  `reception_reports` is `not_negotiated` there.
