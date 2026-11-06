# Validation Report: Video Latency on Servers That Rewrite Timestamps (012)

**Baseline** (T001): commit `356166e`. `BenchmarkQualityVideoPacket`: 36.96 ns/op, 0 allocs/op (13th Gen Intel i5-13600H), without send times in the benchmark.

## Browser decoding (T011, SC-008)

**Passed.** Gate cleared on 2026-11-06, in headless browsers. The picture was
checked by screenshot rather than watched by a person for a minute.

Pixel comparison through each browser's own VP8 decoder (WebCodecs): every
bundled frame decoded with and without a 6-byte trailer, comparing all planes.

| Browser | Frames | Layers | Decoded, without / with | Different | Errors |
|---|---|---|---|---|---|
| Chrome 156 | 300 each | 90p, 180p, 360p, 720p | 300 / 300 | 0 | none |
| Firefox 158 | 300 each | 90p, 180p, 360p, 720p | 300 / 300 | 0 | none |

Limit: both ran headless with no GPU, so these are software decoders.
Hardware VP8 decoders were not tested.

Live sessions, Chrome 156 receiving 4 bots from this build, 10 s sample:

| Server | Streams | Decoded in 10 s | Freezes | Dropped in the sample | Keyframe requests in the sample |
|---|---|---|---|---|---|
| Jitsi, simulcast 720 | 4 at 320x180 | 300–301 each | 0 | 0 | 0 |
| Janus | 1 at 640x360 | 300 | 0 | 0 | 0 |

Screenshots show the test pattern, with its running clock, on every stream.
The only console errors were Jitsi XMPP `service-unavailable` messages, not
media. Firefox was not run through a live session; its decoder is covered above.

## SC-001: Janus (T014)

**Met.** 10 bots, `audio_video`, 60 s hold, live test passed: video latency p50
1.5 ms, 10 of 10 bots contributing (checked by the live test), audio left out
with `frame_not_identified` (FR-012). 0 freezes, 30.0 fps, MOS 4.37.

## SC-002: Jitsi with simulcast (T016)

**Met.** 10 bots, simulcast 720, cap 4, live tests passed, all bots contributing.

| Heights asked for | Resolution received | Latency p50, audio / video | Freezes |
|---|---|---|---|
| Default | 720: 100% | 0.5 ms / 1.5 ms | 0 |
| 720 and 180, half each | 720: 51%, 180: 49% | 0.5 ms / 1.5 ms | 0 |

## SC-003, SC-004: automated tests

**Met.**
- SC-003: `TestPeer_LatencyThroughDelay` and `TestPeer_LatencyThroughRewrite`: 51.5 ms median with a 50 ms delay, for video through a path that rewrites timestamps too. Audio through that path gets 0 samples, which shows the rewrite defeats timestamp matching.
- SC-004: `TestPeer_NoLatencyForForeignFrames` (another scenario, no trailer), `TestVideoQuality_LatencyIgnoresWrongTag`, `TestVideoQuality_LatencyAcrossContentLoop`.
- Most latency tests were written after the code they cover, so each guard was checked by mutation: removing the tag check, the current-frame check, the newer-frame check, or the send time check, using the last packet's arrival, or looking up by timestamp each fails at least one test. The send time check first survived, so a test was added for it, seen failing without the guard.
- `BenchmarkQualityVideoPacket`, now with send times and trailers: 53 ns/op, 0 allocs/op, under the 200 ns budget. Baseline 37 ns/op, without send times.

## LiveKit and mediasoup (T019)

10 bots, `audio_video`, 60 s hold, live tests passed. Both keep the trailer.

| Server | Latency p50, audio / video | Freezes | Frame rate | MOS |
|---|---|---|---|---|
| LiveKit | 0.5 ms / 1.5 ms | 0 | 30.0 fps | 4.37 |
| mediasoup-demo | 1.5 ms / 2.5 ms | 0 | 30.0 fps | 4.37 |

## SC-005: no regression at 50 bots (T020)

**Met.** Pre-012 (commit `356166e`) and 012, in the order A B B A per server: 50
bots, `audio_video`, cap 10, ramp-up 10 s, 120 s hold, 2026-11-06. Each server
ran alone, except that the idle `jmt-jitsi` stack stayed up during the LiveKit
and mediasoup runs. Screen state was not controlled. Holds are shorter than the
10 minutes of 010 T029.

| Server, build | Video latency p50 | Audio latency p50 | Freezes (whole run) | Frame rate p50 | MOS p50 |
|---|---|---|---|---|---|
| Jitsi, pre-012 | 4.5, 5.5 ms | 3.5, 3.5 ms | 68, 40 | 29.9, 30.0 | 4.37, 4.37 |
| Jitsi, 012 | 5.5, 5.5 ms | 3.5, 3.5 ms | 73, 49 | 29.9, 30.0 | 4.36, 4.36 |
| LiveKit, pre-012 | 3.5, 3.5 ms | 2.5, 2.5 ms | 0, 0 | 30.0, 30.0 | 4.37, 4.37 |
| LiveKit, 012 | 3.5, 3.5 ms | 2.5, 2.5 ms | 0, 0 | 30.0, 30.0 | 4.37, 4.37 |
| mediasoup, pre-012 | 8.5, 8.5 ms | 5.5, 5.5 ms | 0, 0 | 30.0, 30.0 | 4.37, 4.37 |
| mediasoup, 012 | 8.5, 8.5 ms | 5.5, 5.5 ms | 0, 0 | 30.0, 30.0 | 4.37, 4.37 |

**Jitsi live tests failed on both builds.** All 4 alternated runs ended in FAIL.
The reports show freezes on both builds, as in earlier feature runs at 50 bots
(about 100 on Jitsi, 011 T022). One more 012 run was captured: its only failure
was the existing check `39 freezes during the hold, want none`, with the new
latency checks passing. The failure text of the pre-012 runs was not captured;
their reports show the same freezes.

## SC-006: engine CPU at 100 bots (T021)

**Met.** LiveKit, 100 bots, cap 10, same plan, A B B A: engine average 4.22 and
4.27 threads pre-012, 4.30 and 4.28 with 012, so +1.1% on the means. All 4 live
tests passed; video latency p50 5.5 ms pre-012, 6.5 ms with 012, within SC-005's
2 ms; freezes 0.

Engine CPU at 50 bots, means of 2 runs, same order:

| Server | Pre-012 | 012 | Change |
|---|---|---|---|
| Jitsi | 2.20 | 2.26 | +2.7% |
| LiveKit | 2.20 | 2.18 | −0.7% |
| mediasoup | 1.71 | 1.73 | +1.2% |

## SC-007: lowest layer bitrate (T022)

**Met.** `video_90.ivf`: 300 frames, 98,478 bytes. The trailer adds 1,800 bytes,
1.83% of the video data, and less of the bytes on the wire, where RTP headers
also count. Reserving 6 bytes per packet does not change its packet count: 312
packets per loop before and after. The next layer, 180p, gets +0.87%.

## Summary

| Criterion | Result |
|---|---|
| SC-001 Janus | Met |
| SC-002 Jitsi simulcast | Met |
| SC-003 Accuracy with a known delay | Met (51.5 ms for 50 ms) |
| SC-004 No false values | Met |
| SC-005 No regression | Met; Jitsi's 50-bot live test fails on both builds (existing freezes check) |
| SC-006 Engine CPU | Met (+1.1%) |
| SC-007 Bitrate | Met (+1.83%) |
| SC-008 Browsers | Met in headless Chrome and Firefox, with the limits above |

`go vet ./...` and `go test -race ./...` pass. Janus audio latency stays
`frame_not_identified`, as specified.
