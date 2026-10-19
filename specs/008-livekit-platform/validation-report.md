# Validation Report: LiveKit Platform (008)

**Date**: 2026-10-17. **Setup**: one laptop, 16 hardware threads (8 usable by the engine's session); LiveKit `livekit/livekit-server:v1.13.10` from `tests/platforms/livekit` (host network, single node); docker-jitsi-meet from `tests/platforms/docker-jitsi-meet` (JVB 2.3-326, 8 GB heap) for the same-day comparison. Only one server ran at a time. Go 1.27.2 local toolchain, `go 1.23.0` module.

`go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...` are green (three consecutive full runs).

## Success criteria

| Criterion | Result | Evidence |
|---|---|---|
| SC-001: a Jitsi test plan runs on LiveKit by changing only the platform and its settings; 10 bots receive 9 + 9 streams and leave within 10 s | **Met** | T021: the live test builds the plan with `platform: livekit` and its `platform_config`, nothing else changed. All 10 established in 1.2 s; 9.0 audio and 9.0 video streams offered per bot; bots left in 2 ms |
| SC-002: 100 bots, cap 10, 10 min on LiveKit, every bot connected, engine within the 2-core budget | **Met** | T027: 100/100 connected for the 10 min hold; engine 1.43 cores (`/usr/bin/time` and `load_machine` agree); no freezes, MOS p50 4.368 |
| SC-003: 10 bots, no freezes, gaps under 1%, MOS p50 ≥ 4.0 | **Met** | T025: 0 freezes, concealed ratio p50 0, MOS p50 4.369, frame rate p50 30.0, 360 lines 100% of the time |
| SC-004: LiveKit and Jitsi summaries of the same plan have the same fields | **Met** | T025: 96 and 101 key paths. The only differences are optional keys present only with a value: `video.latency_reason` on LiveKit; `video.latency_seconds.*` and `worst.most_freeze_seconds[]` entries on Jitsi. `TestEngine_LiveKitQualitySummary` checks the 006/007 key list on every test run |
| SC-005: every LiveKit plan with simulcast, requested heights or incomplete settings is rejected at creation | **Met** | `TestLiveKitCreationRules` (API, 400 with the contract's messages for missing url/room, key without secret, token with key, simulcast, requested heights); `TestParseConfig` and `TestValidate` cover every rule of `contracts/livekit-config.yaml` |
| SC-006: from a clean checkout, start the server and run 10 bots in under 10 minutes | **Met** | T026: 72 s from a copy of the commit contents to a passing 10-bot run, following `tests/platforms/livekit/README.md` (image and modules cached) |

## Same-day comparison, 10 bots (T025)

10 bots `audio_video`, no cap, 60 s hold.

| | LiveKit | Jitsi |
|---|---|---|
| Frame rate p50 | 30.0 | 30.0 |
| Video freezes over the run | 0 | 14 (4 during the hold; also 5 with the code before 008) |
| Audio concealed p50 / MOS p50 | 0 / 4.369 | 0 / 4.368 |
| Latency p50, audio / video | 0.5 ms / not available (`frame_not_identified`) | 1.5 ms / 1.5 ms |
| Server CPU threads, average / peak | 0.140 / 0.255 | 0.970 / 1.701 (jvb 0.954 / 1.549) |
| Server memory peak | 110 MB | jvb 725 MB + jicofo 222 + prosody 41 + web 20 |
| Engine CPU threads, average | 0.086 | 0.388 |
| Video packets/s received per bot | 586 (full rate) | 496 (0.84 of the sent rate) |

The LiveKit column uses the shipped setup (UDP port range). A first run with the single UDP port gave the same quality, but 0.717 server and 0.644 engine threads.

## Same-day comparison, 100 bots (T027)

100 bots `audio_video`, cap 10, ramp-up 20 s, 10 min hold, each server alone and restarted before its run.

| | LiveKit 1.13.10 | Jitsi (JVB, 8 GB heap) |
|---|---|---|
| Bots connected for the whole hold | 100/100 | 100/100 |
| Streams offered per bot | 10 audio + 10 video | 10 audio + 10 video |
| Frame rate p50 / freezes over the run | 30.0 / 0 | 30.0 / 236 (88 during the hold) |
| Audio concealed p95 / MOS p50 | 0.00005 / 4.368 | 0.0048 / 4.332 |
| Latency p50, audio / video | 1.5 ms / not available | 7.5 ms / 10.5 ms |
| Server CPU threads, average / peak | 2.28 / 5.41 | 8.65 / 8.95 (jvb 8.61) |
| Server memory peak | 1.2 GB | 4.2 GB (all containers) |
| Engine CPU, `/usr/bin/time` | 1.43 cores ((724.8 + 167.2) s / 625.5 s) | 2.90 cores ((1460.8 + 376.1) s / 632.6 s) |
| Engine CPU, `load_machine` average / RSS | 1.43 threads / 1.31 GB | 2.90 threads / 0.88 GB |

The first LiveKit run used the single UDP port of the original plan. All bots held, but audio arrived 1.3 s late (MOS 1.48, 150,802 freezes). The diagnosis is in tasks.md T027: kernel, sockets and the engine were cleared, and a one-track probe saw no delay. LiveKit's single-port mux queues what it sends at this rate. The setup now uses a UDP port range.

## Research checks (T002) and findings

- **R1**: protocol 17 gives the dual-PC model: `subscriber_primary` true, and a first subscriber offer (data channel only) right after the join.
- **Message order**: the server sends subscriber candidates before that offer (buffered by the peer), and a `refresh_token` first.
- **Track announcements**: a publisher's tracks are announced in later `ParticipantUpdate`s once its media flows, one by one.
- **R6**: subscriber offers carry `a=ssrc` lines per forwarded track (`sendrecv`), which the receive gate uses.
- **R12**: ICE connects with the server announcing 127.0.0.1.
- **R11**: LiveKit keeps the senders' Opus RTP timestamps and sequence numbers, but rewrites VP8 timestamps. Video latency is therefore reported as unavailable with `latency_reason: frame_not_identified`, never as zero.
- **Secrets (R10, corrected)**: `GET /scenarios/{id}` never returned the test plan. The leak was the service's creation log line, which printed the test plan with its secrets for both platforms. That line and every scenario the service returns now redact the secret keys.
- **007 bug found by the suite**: a server-load tick that started as the run ended was counted as a `docker_unreachable` miss (its requests were cancelled with the run). Such ticks are now dropped; the end-of-run tick covers the end (`TestSampler_TickCancelledByRunEnd`).
- **Two flaky tests fixed**: `TestProcessSample_CPU` (007) timed its busy loop by the wall clock and failed under a loaded machine; it now times the loop by the CPU counter. The LiveKit fake server listed participants in map order; it now uses join order.
