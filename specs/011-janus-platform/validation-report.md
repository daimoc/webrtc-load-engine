# Validation Report: Janus Platform

**Date**: 2026-10-22 and 2026-10-23.

**Setup**:
- one laptop, 16 hardware threads (8 usable by the engine's session);
- Janus **v1.4.3** (commit `1f0b75c`), libnice 0.1.21 (Debian bookworm),
  built from `tests/platforms/janus`: VideoRoom plugin and WebSocket
  transport only, plain `ws://` on 127.0.0.1, host networking;
- for the comparison: docker-jitsi-meet, LiveKit `v1.13.10` and
  mediasoup-demo commit `899109e` from `tests/platforms`, one server
  running at a time;
- Go 1.27.2 local toolchain, `go 1.23.0` module;
- screen unlocked (`loginctl` `LockedHint=no`) in every run reported here.

The machine's state matters (009 A/B run, 010 T029). This report adds a
second reason: under a 10-minute load the CPU package reaches about 83 °C
and the processor throttles (thousands of thermal throttle events since
boot). CPU figures are compared within one set of alternated runs, never
across days.

## Results per success criterion

| Criterion | Result | Evidence |
|---|---|---|
| SC-001: same plan as the other platforms, 10 bots `audio_video` 1 min, 9 + 9 streams each, all leave within 10 s | **Met** | T015: established in 1.22 s, 9.0 audio and 9.0 video streams offered per bot (the dummy's section not counted), bots left in 2 ms. The plan differs from mediasoup's only in `platform` and `platform_config` |
| SC-002: 50 bots, cap 10, 10 min, every bot connected the whole hold, alternated with the other three servers | **Met** | T022: both Janus runs PASS, 50/50 bots held their transport for 10 min, 10 + 10 streams per bot. See "Four servers, 50 bots" |
| SC-003: 10 bots idle machine, no freezes, gaps < 1%, MOS p50 ≥ 4.0 | **Met** | T015: 0 freezes, 0% audio concealed, MOS p50 4.37, 30.0 fps at 360 lines (`20261022T220002Z-janus-live-janus-60db2ce6`) |
| SC-004: same fields as the other platforms | **Met** | T017: the field sets of a Janus and a mediasoup 10-bot report differ only in latency, given by value on mediasoup and by `latency_reason` on Janus (FR-013) |
| SC-005: 100% of bad plans rejected at creation | **Met** | `TestParseConfig_Errors`, `TestValidate`, `TestCatalog_Janus`, `TestJanusCreationRules` (HTTP 400 for simulcast, requested heights, missing `url` or `room`) |
| SC-006: clean checkout to the 10-bot run in under 15 min | **Met** | T021: 2 min 54 s from `cp env.example .env` to PASS, image built with `--no-cache` (Janus started after 95 s) |
| SC-007: normal test run passes with the Janus tests, no network or server needed, race detection | **Met** | T021, T025: `go test -race ./...` green with Janus stopped; the Janus package 5 times in a row under `-race`. In a namespace with only `lo`, two receive tests fail, as mediasoup's do (an engine test follow-up, not Janus-specific) |

## 10 bots (T015–T019)

| | Janus | mediasoup-demo (same evening, T017) |
|---|---|---|
| Established in | 1.22 s | – |
| Streams offered per bot | 9 audio + 9 video | 9 + 9 |
| Packets/s received per bot | 445 audio, 586 video | – |
| Freezes, frame rate p50 | 0, 30.0 fps | 0, 30.0 fps |
| MOS p50, audio concealed p95 | 4.37, 0% | 4.37, 0% |
| Latency p50, audio / video | `frame_not_identified` both | 1.5 ms / 2.5 ms |
| Server CPU (average / peak), memory peak | 0.76 / 0.78 threads, 18 MB | 0.21 threads |
| Engine CPU (average) | 0.46 threads | 0.52 threads |

The server figure is the `janus` container only; `janus-demos` (nginx) is
not listed (T019). `docker stats janus` gave 73–79 % during a run, against
0.743 threads in that run's report.

Other checks (T015):
- **Receive cap 3 with replacement** (6 bots, an extra bot leaving
  halfway): 3 + 3 streams per bot to the end, 0 freezes.
- **A `receive_only` bot alone**: established in 204 ms, through the dummy
  publisher's subscription.
- **3-minute hold**: no session `timeout`; the 25 s keepalive holds the
  sessions past Janus's 60 s `session_timeout`.

## Four servers, 50 bots (T022)

The same plan on the four local servers, alternated twice (Jitsi, LiveKit,
mediasoup, Janus, then again) on 2026-10-22 from 22:15 to 23:40 UTC:
- 50 bots `audio_video`, cap 10, ramp-up 10 s, 10 min hold;
- each server alone, started 15 s before its run; mediasoup in its default
  mode (one worker for the room);
- one live test binary for all eight runs, engine CPU checked with
  `/usr/bin/time`.

All eight runs passed: 50/50 bots held their transport for the whole hold,
10 audio and 10 video streams offered per bot, about 500 audio and 650–658
video packets/s received per bot, established in 10.3–11.2 s.

| | Jitsi | LiveKit | mediasoup-demo | Janus |
|---|---|---|---|---|
| MOS p50 | 4.37 / 4.37 | 4.37 / 4.37 | 4.37 / 4.37 | 4.37 / 4.37 |
| Audio concealed p95 | 0.004% / 0.009% | 0.000% / 0.003% | 0.000% / 0.000% | 0.001% / 0.001% |
| Frame rate p50 | 30.0 / 30.0 | 30.0 / 30.0 | 30.0 / 30.0 | 30.0 / 30.0 |
| Video freezes over the run | 68 / 107 | 0 / 0 | 0 / 0 | 0 / 0 |
| Latency p50, audio / video | 0.5 ms / not identified (both runs, see below) | 0.5 / 1.5 ms both | 1.5 / 3.5 ms both | not identified |
| Server CPU threads, average / peak | 1.69 / 4.76, 1.55 / 4.38 | 0.97 / 2.93, 0.98 / 2.80 | 0.42 / 0.79, 0.41 / 0.81 | 1.75 / 7.10, 2.01 / 6.93 |
| Server memory peak | 2.43 / 2.43 GB | 0.65 / 0.60 GB | 0.50 / 0.50 GB | 0.06 / 0.06 GB |
| Engine CPU threads, average | 0.58 / 0.53 | 0.62 / 0.63 | 0.70 / 0.66 | 0.66 / 0.76 |

Pairs are run 1 / run 2. Jitsi's server figures cover jvb (1.68 and 1.54
threads), jicofo and prosody (0.01 or less each). Engine figures agree with
`/usr/bin/time` (user + system over the elapsed time, e.g. 0.66 for Janus
run 1). Reports: `reports/20261022T2{22609,23651,24722,25754,30824,31906,32937,34010}Z-*`.

Findings:
- **Quality**: the same audio and frame rate on the four servers. Only
  Jitsi froze video, as in 010 T029 (the bridge's start-up behaviour of
  006).
- **Server CPU**: Janus used the most CPU on average (1.75 and 2.01
  threads), about Jitsi's bridge, twice LiveKit and four to five times
  mediasoup's one worker. Its peak (7.1 and 6.9 threads) is the highest of
  the four, far above its average; the report does not say when in the run
  it happened.
- **Server memory**: Janus is the lightest by far, 60 MB at 50 bots
  against 0.5–2.4 GB for the others.
- **Engine CPU**: about the same for the four servers (0.53–0.76
  threads), so the platform hardly changes the load machine's cost.
- **Against 010 T029** (2026-10-20, same plan, three servers): quality and
  streams are the same. CPU figures are not: the engine used 1.14–1.75
  threads then, against 0.53–0.70 now, and the servers 1.4–2.9 times more,
  while Jitsi's and mediasoup's peaks were about the same (mediasoup 0.76
  and 0.79 threads), with a higher latency then (mediasoup video 5.5 ms
  against 3.5 ms). 10-bot runs cost the same on
  both days (mediasoup: engine 0.55 and 0.52, server 0.21 and 0.21), and
  the only code change between the two sets is the sources' initial
  timestamp. The difference is therefore the machine under sustained load
  (clock speed, thermal throttling), which was not recorded on 2026-10-20.
  The ratios between servers changed too (LiveKit over mediasoup 4.4 then,
  2.3 now), so only same-set comparisons are valid.

### Jitsi video latency (found in T022)

Jitsi's video latency was not identified in either Jitsi run, while T029
had measured it (5.5 ms). The cause was T026's first value, 2^31: Jitsi's
bridge keeps a video stream's timestamps only when they are newer than 0 in
RTP's modular order, and rebased them to 3001 otherwise. A timestamp probe
showed:
- sources at 0 and at 2^30: received timestamps equal to the sent ones,
  also for a bot joining 8 s into the stream;
- sources at 2^31 on Jitsi, and at 2^30 on Janus: rebased, never matched.

T026 now starts at 2^30 (research R8). A third 50-bot Jitsi run with it
(same plan, room `cmp50j5`, 23:48 UTC): **PASS**, 50/50 bots held, and
video latency is measured again, p50 1.5 ms (audio 1.5 ms), with 100
freezes as in the other Jitsi runs, server 1.66 threads, engine 0.58
(`20261022T235812Z-jitsi-live-f89231e3`). The other T022 figures do not
depend on the initial timestamp.

## Implementation findings that changed the design

All from T002, against the real server:
1. **Dummy publisher** (research R3): Janus refuses an empty subscription,
   so a bot alone in the room would never connect. Rooms are created with a
   silent audio dummy publisher that every bot subscribes to at join. It
   takes no publisher place, so rooms have one place per bot. Its SDP
   section's `a=ssrc`, `a=ssrc-group` and `a=msid` lines are removed before
   the peer sees the offer, so it is not counted as an offered stream.
2. **Timestamps** (research R8): Janus rebases every forwarded stream from
   0. With sources at 0, a bot subscribing *t* seconds late would have been
   matched against frames sent *t* seconds earlier, a wrong latency of
   about *t*. Sources now start at 2^30 (T026, the user's decision), and
   Janus's latency is left out with `frame_not_identified`.
3. **ICE** (research R10): Janus never gathers candidates on loopback
   (`ice.c` skips loopback interfaces), so the bots connect on the host's
   IPv4 address. Container bridges are in `ice_ignore_list`;
   `JANUS_ICE_ENFORCE_LIST` pins an interface.
4. **`update` replies** (research R6): an `update` sent before the
   previous offer was answered is acknowledged with `updating`, and its
   offer arrives later, unsolicited. The adapter handles both.
5. **Leaving**: a publisher's departure is announced as `unpublished` twice,
   then `leaving`; feed-gone handling is idempotent.

## Deviations from the plan

- `src/media/source.go` changed (T026): sources start their RTP timestamps
  at 2^30 instead of 0. Platform-neutral; 2^31 at first, moved to 2^30 in
  T022 for Jitsi.
- `catalog.Entry.NewAdapter` receives the test plan, so that the Janus
  entry can size the room from `participants`. The other entries use only
  its `platform_config`.
- The HTTP transport is not built: port 8088 is taken on this machine, and
  the demo pages use the WebSocket transport (`demos-settings.js`).
- The demo page was checked with headless Chrome (fake camera, 3 bots):
  it plays the bots' videos at 640x360. The page's own publish is refused
  because the room has exactly one place per bot (README updated).
- A race found while T022 ran: the receive cap was created in `join`, after
  room events could reach the worker. It is now created in `Connect`. The
  T022 binary was built before the fix; the T025 runs use it.

## Final checks (T025)

Run on 2026-10-23 with the final code (sources at 2^30, receive cap
created in `Connect`):
- `go vet ./... && go vet -tags live ./tests/live/ && go test -race
  -count=1 ./...`: all pass; `go test -race -count=5
  ./src/platform/janus/` passes.
- Boundary grep: outside `src/platform/janus` and the catalog, only test
  data in `src/report/writer_test.go`, and the comment of
  `initialRTPTimestamp` in `src/media/source.go` explaining the value.
- Since `68e7205` (before T001), `src/execution`, `src/media`,
  `src/scenario` and `src/report` changed only in test files and
  `src/media/source.go` (T026). `go.mod` and `go.sum` unchanged.
- Live runs, 60 s holds, all **PASS** (FR-011):

| Run | Established | Streams per bot | Freezes, MOS p50 | Latency p50, audio / video |
|---|---|---|---|---|
| Jitsi, 2 bots | 2.24 s | 1 + 1 | 0, 4.37 | 0.5 / 0.5 ms |
| LiveKit, 10 bots | 1.23 s | 9 + 9 | 0, 4.37 | 0.5 ms / `frame_not_identified` (as in 010) |
| mediasoup, 10 bots | 0.41 s | 9 + 9 | 0, 4.37 | 0.5 / 0.5 ms |
| Janus, 10 bots | 1.22 s | 9 + 9 | 0, 4.37 | `frame_not_identified` both |

- Janus's log over the follow-up runs: the expected `Room N already
  exists!` and `Failed to add some remote candidates` lines (T015), and one
  `Invalid plugin session` ERR, with no effect on any run.
