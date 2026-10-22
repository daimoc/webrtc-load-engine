# Validation Report: mediasoup-demo Platform

**Date**: 2026-10-20.

**Setup**:
- one laptop, 16 hardware threads (8 usable by the engine's session);
- mediasoup-demo commit `899109e` (mediasoup **3.30.1**, Node 22) from
  `tests/platforms/mediasoup`, 2 workers, plain `ws://` on 127.0.0.1;
- LiveKit `v1.13.10` from `tests/platforms/livekit`;
- Jitsi stopped, only one server running during each run;
- Go 1.27.2 local toolchain, `go 1.23.0` module.

The machine's state matters: the 009 A/B run showed the same load costing
three times more CPU with the desktop on than with the screen locked. The
screen state of each comparison is given.

## Results per success criterion

| Criterion | Result | Evidence |
|---|---|---|
| SC-001: same plan as Jitsi/LiveKit, 10 bots `audio_video` 1 min, 9 + 9 streams each, all leave within 10 s | **Met** | T016: 10 bots established in 406 ms, 9.0 audio and 9.0 video streams offered per bot, bots left in 1–4 ms. The plan differs from LiveKit's only in `platform` and `platform_config` |
| SC-002: 100 bots, cap 10, 10 min, every bot connected the whole hold, engine CPU within noise of LiveKit's | **Not met** | T023: in the default mode the room's single worker saturates and its one UDP port drops packets, so about one bot per run loses ICE consent. With a 4 MiB socket buffer the drops stop, but the worker still saturates (2 bots lost, MOS 2.87). With pipe transports all bots hold, but audio degrades (MOS 2.99): mediasoup's own pipe socket between the two workers drops packets (T028). See "100 bots" below |
| SC-003: 10 bots idle machine, no freezes, gaps < 1%, MOS p50 ≥ 4.0 | **Met** | 0 freezes, 0.00% audio concealed, MOS p50 4.37, 30 fps at 360 lines (`20261020T070507Z-mediasoup-live-mediasoup-838d3854`) |
| SC-004: same fields as LiveKit and Jitsi | **Met** | T018: 93 report fields each. Differences: `platform_config` keys, and video latency, given by value on mediasoup and by `latency_reason` on LiveKit (FR-011). Identical `/metrics` series and labels (47) |
| SC-005: 100% of bad plans rejected at creation | **Met** | `TestValidate`, `TestCatalog_Mediasoup`, `TestMediasoupCreationRules` (HTTP 400 with the contract's messages) |
| SC-006: clean checkout to the 10-bot run in under 15 min | **Met** | T022: 121 s from the first README command to the end of the 10-bot test, image rebuilt (22 s without the build cache; base image already present) |

## 10 bots (T016–T020)

| | mediasoup-demo | LiveKit (same day, T018) |
|---|---|---|
| Established in | 0.41 s | – |
| Streams offered per bot | 9 audio + 9 video | 9 + 9 |
| Freezes, frame rate p50 | 0, 30.0 fps | 0, 30.0 fps |
| MOS p50, audio concealed p95 | 4.37, 0.00% | 4.37, 0.00% |
| Latency p50, audio / video | 0.5 ms / 2.5 ms | measured / `frame_not_identified` |
| Server CPU (average), memory peak | 0.21 threads, 53 MiB | – |
| Engine CPU (average) | 0.54 threads | – |

mediasoup keeps the senders' RTP timestamps, so video latency is measured
(T019). The server figure covers the node process and both
`mediasoup-worker` processes (T020).

Other checks (T016):
- **Receive cap 5 with replacement** (12 bots, an extra bot leaving
  halfway): 5 + 5 streams per bot to the end, 0 freezes.
- **`receive_only` bots** next to 3 senders: they received audio (MOS 4.37)
  and video (30 fps).
- **A `receive_only` bot alone** in an empty room: established in 204 ms.

## 100 bots (T023)

Alternated LiveKit, mediasoup, LiveKit, mediasoup, screensaver on from
run 1 onwards:

| Run | Platform | Result | Engine | Server avg / peak | MOS p50 |
|---|---|---|---|---|---|
| 1 | LiveKit | PASS (10 min) | 3.24 cores | 4.90 / 5.51 threads | 4.37 |
| 2 | mediasoup | FAIL at 26 s, 1 bot lost | – | – | – |
| 3 | LiveKit | PASS (10 min) | 3.19 cores | 4.87 / 5.53 threads | 4.37 |
| 4 | mediasoup | FAIL at 28 s, 1 bot lost | – | – | – |

What happens on mediasoup:

- The demo puts a room on one router, so on one worker: one thread for all
  the room's media. That worker reached ~90% of a core as the 100 bots
  finished joining; the second worker stayed idle.
- All its bots reach it on one UDP port. With the kernel's default buffer,
  that socket dropped ~400,000 datagrams in 30 s (`/proc/net/udp`), and none
  were dropped on the bots' side.
- About 30% of the bots' consent checks (RFC 7675) were lost among them.
  mediasoup counts any valid check, so the lost ones never reached it. A
  transport that lost four in a row (20 s) expired, about one bot per run.
- With `MEDIASOUP_SOCKET_BUFFER=4194304` (now the setup's default), the port
  drops nothing, but the worker still saturates: 2 bots lost, MOS 2.87, 53%
  audio concealed (p95), 7,792 freezes.
- With `use_pipe_transports` (producers and consumers on two workers), all
  100 bots held their transport for 60 s with 0 video freezes. But audio
  degraded (MOS 2.99, 14% concealed at p95). About 3,000–4,000 datagrams/s
  are dropped. T023 attributed them to the bots' sockets, which was wrong:
  per socket (T028) they all fall on one root-owned socket, mediasoup's pipe
  transport between its two workers, with its buffer full. The bots'
  sockets drop nothing. Video recovers through NACK and RTX; audio has no
  retransmission, so it shows the loss.

**Conclusion**: on this laptop, mediasoup-demo cannot carry 100 bots
`audio_video` with a cap of 10 in one room: its default mode saturates one
worker. This is a property of the demo (one worker per room), as research R9
anticipated, not of the adapter. It is also the comparison result the user
asked for: under the same load LiveKit holds (4.9 threads spread over the
machine), and the demo's single-threaded room does not.

Pipe mode's loss is inside mediasoup. The demo calls `pipeToRouter` without
socket options, so its pipe socket keeps the kernel default
(`net.core.rmem_default`, 208 KiB). No demo setting can change it. A larger
host default (`sysctl net.core.rmem_default`, root) would raise it, but it
would also change every other socket on the machine. Other next steps:
- a run with the server on its own machine;
- a step between 50 bots (holds, T029) and 100 to find where the demo
  saturates.

### Larger receive buffers on the bots' sockets (T028)

At the user's request, the engine was given a 4 MiB receive buffer for
each of its UDP sockets, set where the consent wrapper opens them. The same
100-bot pipe-mode run with and without it:

| Build | Drops on the bots' 1,600 sockets | Drops on mediasoup's pipe socket | MOS p50 | Audio concealed p95 |
|---|---|---|---|---|
| Without (T023 binary) | 0 | 172,228 in 45 s | 3.02 | 14.0% |
| With 4 MiB buffers | 0 | 172,996 in 45 s | 3.04 | 13.6% |

No measurable effect: the bots' sockets did not overflow with the default
buffer either. The change was therefore reverted; the engine keeps the
kernel's default buffers.

## Three servers, 50 bots (T029)

The same plan on the three local servers, alternated twice (Jitsi,
LiveKit, mediasoup, Jitsi, LiveKit, mediasoup) on 2026-10-20 from
19:59 to 21:02 UTC:
- 50 bots `audio_video`, cap 10, ramp-up 10 s (the 100-bot runs' 5 bots/s),
  10 min hold;
- each server alone, started just before its run, mediasoup in its default
  mode (one worker for the room);
- screen unlocked during all six runs (`loginctl` `LockedHint=no`).

All six runs passed: 50/50 bots held their transport for the whole hold, 10
audio and 10 video streams offered per bot.

| | Jitsi | LiveKit | mediasoup-demo |
|---|---|---|---|
| MOS p50 | 4.37 / 4.37 | 4.37 / 4.37 | 4.37 / 4.37 |
| Audio concealed p95 | 0.01% / 0.00% | 0.01% / 0.00% | 0.00% / 0.00% |
| Frame rate p50 | 30.0 / 30.0 | 30.0 / 30.0 | 30.0 / 30.0 |
| Video freezes over the run | 68 / 228 | 0 / 0 | 0 / 0 |
| Latency p50, audio / video | 3.5 / 5.5 ms, 2.5 / 4.5 ms | 2.5 / 3.5 ms both | 2.5 / 5.5 ms both |
| Server CPU threads, average / peak | 4.42 / 4.64, 4.49 / 4.69 | 2.64 / 4.04, 2.62 / 7.28 | 0.60 / 0.76, 0.60 / 0.77 |
| Server memory peak | 2.35 / 2.31 GB | 0.57 / 0.58 GB | 0.48 / 0.48 GB |
| Engine CPU threads, average | 1.67 / 1.69 | 1.74 / 1.75 | 1.15 / 1.14 |

Pairs are run 1 / run 2. Server figures cover all the server's containers:
jvb carries Jitsi's load (4.41 and 4.47 threads), jicofo and prosody
0.01 each. Engine figures agree with `/usr/bin/time` (user + system over
the elapsed time).

Findings:
- **Quality**: identical audio and frame rate on the three servers. Only
  Jitsi froze video, mostly at start-up: run 1 had 44 of its 48 logged
  freezes in the first minute; run 2 had 103 of 160 there, then a tail
  that faded over the hold (24, 10, 11, 7, then 1–3 a minute). This is
  the bridge's start-up behaviour of 006, larger in run 2.
- **Server CPU**: for the same load, the mediasoup-demo room used 0.6
  threads, LiveKit about 4x more and Jitsi's bridge about 7x more.
  mediasoup's one worker at this load is well under a core, while it
  saturated at 100 bots (T023). Its limit on this laptop is between 50 and
  100 bots.
- **Engine CPU** is a third lower against mediasoup (1.15 threads) than
  against LiveKit or Jitsi (1.7), for the same streams received. Not
  explained here; candidates are the RTCP and bandwidth probing each
  server sends.

Reports: `20261020T200913Z`, `T201949Z`, `T203025Z`, `T204111Z`,
`T205146Z` and `T210212Z` in `reports/` (not committed).

## Implementation findings that changed the design

1. **DTLS role** (T006): Pion answers an ICE-lite `actpass` offer with
   `setup:passive`. The adapter therefore reads the role from the bot's SDP
   instead of always sending `client` (research R2, contracts updated).
2. **Origin check** (T001): the demo refuses a WebSocket without the
   expected `Origin` (403). The adapter sends `http(s)://<url host>`, and the
   new `origin` setting covers other names (research R1, contract).
3. **ICE consent** (T016): the demo closes a transport after 20 s without an
   RFC 7675 check, and Pion sends none while media flows. The first 10-bot
   run lost every bot at 20 s. As the user chose, the engine now sends
   browser-like consent checks on every PeerConnection, every 5 s ±20%, for
   all platforms (`src/execution/consent.go`, research R14).
   `pion/ice/v2` and `pion/stun` move from indirect to direct requirements;
   no new module.
4. **Receive-only peers** (T016, an 008 bug): a receive-only peer of a
   publish-session platform opened a publish PeerConnection that never
   connected, so it was never established, on LiveKit too. It now opens
   none.
5. **Socket buffer** (T023): the setup's `config.mjs` sets the media
   sockets' buffers from `MEDIASOUP_SOCKET_BUFFER` (default 4 MiB in
   `env.example`).

## Deviations from the plan

- `src/execution` changed (consent checks, receive-only publish session);
  the plan had it unchanged. Both changes are platform-neutral; plan.md
  records the deviation.
- `config.mjs` has optional TLS (`MEDIASOUP_TLS_CERT` and
  `MEDIASOUP_TLS_KEY`) for the demo's web app, which always uses `wss://`.
  The browser steps were not run (no browser here); the engine side of TLS
  was (`insecure_tls`).
- The base image is `node:22-trixie`: the prebuilt mediasoup worker does not
  run on bookworm's glibc.
- Server warnings not explained: `srtp_protect() failed: replay check failed
  (index too old)`, in bursts on a fixed 5 s period during 10-bot runs, with
  no effect on media (0 freezes). Probably mediasoup's bandwidth probing.

## Room time limit (T024)

The server was given `UNLIMITED_ROOM_SECRET`:
- **Without the secret**, the demo closed the room 10 min after it was
  created. Both bots were reported lost (`signaling_lost`, "closed by
  protoo-server") at that instant.
- **With `unlimited_room_secret`**, both bots held for 11 min.
- The secret never appears in logs or reports (`TestAdapter_URLAndSecret`,
  `TestWriteNoSecrets`).
