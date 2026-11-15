# Validation Report: Network Conditions per Bot (016)

Each success criterion is marked met, not met (with the reason), or not checked.

## Baseline

Engine CPU per bot at 100 bots is measured as in 015 (SC-008 there): alternated runs, before vs after, LiveKit restarted before each run, 100 bots `audio_video` VP8 single stream, receive cap 10, 20 s ramp-up, 2 minute hold. "Before" is a worktree at `70c7b23` (the last commit with no 016 code), created at the start of the work (T001). The runs themselves are made in T044, alternated with the 016 build: the laptop's CPU varies with its state, so a baseline taken hours earlier is not comparable (013, 015).

## Gate T004

2026-11-15 12:40–12:50, local servers, 2 bots, VP8, 20–25 s. A temporary probe (since removed) added transport-wide sequence numbers to the sending session's packets (`twcc.NewHeaderExtensionInterceptor`), and counted the `TransportLayerCC` packets the session read.

| Server | Feedback to the sender | Interval | Note |
|---|---|---|---|
| Janus 1.4.3 | yes | ~200 ms | with the extension alone |
| LiveKit 1.13.10 | yes, **only when the offer also lists `a=rtcp-fb:* transport-cc`** | ~85 ms | with the extension alone: the answer accepts it but no feedback and no REMB came |
| mediasoup 3.30.1 | yes | ~100 ms | `sdp.go` already carries every offered extension the router supports into the producer: no change |
| Jitsi stable-11248 | yes | ~30 ms | single session |

Consequence for T037: the sending session of a constrained sender registers the extension **and** the transport-cc RTCP feedback, as a browser's offer does. With that, all four servers can be validated for US4; none is `no_feedback`.

## SC-001: conditions are applied as stated

Met:
- the rate, within 0.1% of the cap over 10 s;
- the delay, 39.8–40.6 ms added for 40 ± 10 ms;
- random loss, 2.91–3.10% for 3%.

Details: "SC-001 (rate, from the US1 runs)" and "US3" below, and the comparison (SC-007).

## SC-002: a capped group on the four servers

Met for the report, 2026-11-15, one run per server: local servers, 10 bots, layered VP8 up to 720, all asking for 720, receive cap 4, 10 s ramp-up (20 s on mediasoup, 015), 60 s hold. From 30 s, 1200 kbit/s toward the last 3 bots (`runs/plans/slow.json`, reports in `runs/us1/`). All four runs are after R14 (REMB); the Janus and Jitsi runs are also after R15 (NACK limit), the LiveKit and mediasoup runs before it. The US5 comparison repeats all four with both.

| Server | Delivered (mean, highest 10 s) | Dropped by the cap | Added delay (mean) | Capped bots' heights (whole run; the cap starts at 30 s) | Capped bots' freezes | Capped bots' audio score |
|---|---|---|---|---|---|---|
| Janus 1.4.3 | 1200, 1201 kbit/s | 71.6% | 192 ms | 720: 100% | 554 | 2.08 |
| LiveKit 1.13.10 | 1117, 1201 | 19.7% | 94 ms | 180: 64.2%, 360: 4.0%, 720: 31.7% | 113 | 2.64 |
| mediasoup 3.30.1 | 729, 1200 | 35.6% | 44 ms | 180: 51.7%, 360: 3.7%, 720: 44.7% | 132 | 3.64 |
| Jitsi stable-11248 | 1200, 1201 | 50.8% | 191 ms | 180: 23.6%, 360: 23.4%, 720: 53.0% | 29 | 2.17 |

Both report files name the 3 bots and their condition, and give their heights, freezes and score. On the three servers that forward layers by an estimate, the capped bots receive less than 720 for most of the cap. Janus forwards the substream asked for, whatever the receiver's link: its VideoRoom has no downlink estimation (R14), so its capped bots freeze. That is Janus's behaviour, not a failure of the run.

How fast each server lowers them, and how often it tries higher layers again, is US2's measure. The queue stays full on Janus and Jitsi (about 190 ms added). mediasoup settles near what four 180-line streams need.

## SC-001 (rate, from the US1 runs)

Met for the cap: in all four runs the highest 10-second delivered rate is 1200–1201 kbit/s, within 0.1% of the cap (limit +10%). The shaper's own tests check it over 20 simulated seconds, and through real sockets at 400 kbit/s (`TestShaper_*`). Delay and loss, from the US3 runs below: 3% random loss gave 2.93–3.15% dropped per bot on every server (limit ±1 point). The added delay was 46.3–46.7 ms for a planned 40 ± 10 ms before the jitter fix, and 40.1–40.4 ms after it (limit ±5 ms). The fix (research R3, T032): jitter is held for 20 ms, as on a real link. Drawn per packet without reordering, a video frame's packets waited behind each other. The shaper's tests check both cases, sparse packets and 2,000 packets/s (`TestShaper_DelayAndJitter`, `TestShaper_JitterOnBursts`).

## US3: delay, jitter and loss

2026-11-15, one run per server (`runs/plans/lossy.json`, seed 7, reports in `runs/us3/`). The US1 room, with 40 ± 10 ms delay and 3% random loss toward the last 3 bots from 30 s, 60 s hold. Jitsi after a warm-up run.

| Server | Dropped by the condition (per bot) | Video still missing after retransmission | Video latency p50, lossy bots / others | Freezes (lossy) | Audio score (lossy) |
|---|---|---|---|---|---|
| Janus 1.4.3 | 2.93–3.11% | 2.86–3.01% | 47.5 / 1.5 ms | 0 | 4.16 |
| LiveKit 1.13.10 | 2.96–3.08% | 0% | 44.5 / 1.5 ms | 0 | 4.15 |
| mediasoup 3.30.1 | 2.96–3.01% | 0% | 49.5 / 2.5 ms | 0 | 4.10 |
| Jitsi stable-11248 | 2.93–3.15% | 0.00–0.01% | 47.5 / 1.5 ms | 0 | 4.14 |

LiveKit, mediasoup and Jitsi retransmit all but a few lost packets. Janus leaves almost all of them missing. Why was not looked into: the bots' NACKs may go unanswered, or the retransmissions may arrive in a form the bots do not count. No server froze: 3% random loss with retransmission is handled everywhere. The lossy bots' 012 video latency rises by the added delay. These runs were made before the jitter fix (46.5 ms added on average), and the Janus run was repeated after it: 40.2 ms added, latency p50 38.5 ms, same loss.

## SC-003: unconstrained bots keep 720

Met in the same four runs. The 7 bots in no group receive 720 100.0% of their time on Janus, LiveKit and Jitsi, and 98.9% on mediasoup (3 freezes, the 015 start-up). The capped bots do not disturb the others on any server.

## SC-004: reaction and recovery times

Met, 2026-11-15, one run per server (`runs/plans/change.json`, reports in `runs/us2/`, after R14 and R15). Plan: the US1 room (10 bots, layered VP8, all asking for 720, receive cap 4), 1200 kbit/s toward the last 3 bots from 30 s to 50 s, then no limit, 80 s hold. Jitsi after a warm-up run. Both report files give a reaction and a recovery time, or "not recovered", for each capped bot. Resolution 2 s.

| Server | Reaction (median, longest) | Recovered | Recovery (median, longest) | Capped bots' freezes | Their audio score |
|---|---|---|---|---|---|
| Janus 1.4.3 | 2, 2 s (freezes) | 3 of 3 | 4, 4 s | 297 | 3.62 |
| LiveKit 1.13.10 | 2, 2 s | 2 of 3 | 32, 34 s | 37 | 3.88 |
| mediasoup 3.30.1 | 2, 2 s | 2 of 3 | 8, 10 s | 115 | 4.08 |
| Jitsi stable-11248 | 2, 2 s | 2 of 3 | 28, 40 s | 42 | 3.58 |

The bots in no group kept 720 at least 99.4% of the time on every server.

- Every server reacts within one collection (2 s).
- Janus "reacts" only by freezing: it keeps forwarding 720. Its recovery is the fastest because nothing was lowered.
- Of the servers that lower layers, mediasoup gives 720 back in 8–10 s. LiveKit (from the bots' REMB) and Jitsi take about 30 s.
- On LiveKit, mediasoup and Jitsi, one bot in three had not recovered when the run ended, 30 s after the cap.

These are single runs. The US5 comparison repeats them three times.

## SC-005: the same seed drops the same share

Met. Two Janus runs of `lossy.json` with seed 7 dropped, per constrained bot, 2.93/3.11/2.94% and 2.93/3.09/2.94%: within 0.02 point (limit 0.5). A third run, after the jitter fix, gave 2.99/3.07/2.94%: the jitter draws now come from the same generator, so the sequence moved, within the limit. `TestShaper_SeedRepeats` checks it on 60,000 and 61,000 datagrams.

## SC-006: no change without conditions, engine CPU

### Engine CPU at 100 bots (T044)

2026-11-15 15:38–16:01 (UTC 14:38–15:01), the conditions of 015 SC-008: local LiveKit 1.13.10 restarted before each run, 100 bots `audio_video` VP8 single stream, receive cap 10, 20 s ramp-up, 2 minute hold. Runs alternate between "before", a worktree at `70c7b23` (no 016 code), and "after", the 016 tree with no `network` section. `load_machine.engine.cpu_threads.average`:

| Pair | Before (threads) | After (threads) |
|---|---|---|
| 1 | 4.288 | 4.502 |
| 2 | 4.322 | 4.487 |
| 3 | 4.308 | 4.501 |
| Mean | 4.306 (0.0431 per bot) | 4.497 (0.0450 per bot) |

After is 4.4% over before: within 5% (FR-013), and every pair agrees, so it is a real cost. 0 freezes in all six runs.

On LiveKit every bot now runs the receive-side estimate of R14, a parse and a filter step per received packet, because LiveKit asks for REMB. That is the likely cause, but it was not measured apart. On the other servers the REMB estimate stays off.

With conditions, 3 more runs of the same plan, with all 10 of the last bots capped both ways from 30 s (500 kbit/s down, 400 kbit/s up, `runs/cpu/both.json`): 4.510, 4.387, 4.427 threads, mean 4.441. That is 1.2% under the unconditioned runs: the shapers and estimators of 10 bots cost less than the media the caps stop. Those runs count about 8,700 freezes: ten 360-line streams cannot fit in 500 kbit/s. Reports in `runs/cpu/`.

### Layered runs without conditions

The 016 tree with no `network` section, on the 014 T059 plan: 10 bots, layers up to 720, receive cap 4, `180:0.5,720:0.5`, 1 minute, VP8; 20 s ramp-up on mediasoup, 10 s elsewhere, Jitsi after a warm-up. Lowest bot's share at its requested height:

| Server | 016 | Before 016 |
|---|---|---|
| Janus 1.4.3 | 1.000 | 1.000 (015 T051) |
| LiveKit 1.13.10 | 0.989, VP9 1.000 (R14) | 0.989 / 1.000 (015 T051) |
| mediasoup 3.30.1 | 0.987 | 0.986–0.990 with a 20 s ramp-up (015 SC-004) |
| Jitsi stable-11248 | 1.000 | no freeze in 015's corrected runs |

0 freezes in every run, and reception reports are measured where 015 measured them. Met: the bots' new behaviours in every run make no difference without a condition. Those are the REMB answer, which is only used on LiveKit, and the limit of 10 NACKs per packet. Reports in `runs/nochange/`.

## SC-007: comparison of the four servers

Met, 2026-11-15 14:00–15:00, the local servers one at a time, with R14 (REMB), R15 (NACK limit), the held jitter and the sender warm-up. Reports in `runs/compare/`, Jitsi's bridge estimates in `runs/compare/jitsi-estimates.jsonl.xz`; logs in `runs/` are compressed with xz. The room is the one of US1: 10 bots, layered VP8 up to 720, all asking for 720, receive cap 4. The ramp-up is 10 s (20 s on mediasoup, 015). Jitsi had one warm-up run first. Figures are the range over the runs.

**US1: 1200 kbit/s toward the last 3 bots from 30 s to the end (60 s hold), three runs each**

| Server | Capped bots at 720 (whole run; cap from 30 s) | at 180 | Their freezes | Their audio score | Dropped by the cap | Added delay | Lowest unconstrained bot at 720 |
|---|---|---|---|---|---|---|---|
| Janus 1.4.3 | 100% | 0% | 328–562 | 2.09–2.12 | 71–72% | 192 ms | 100.0% |
| LiveKit 1.13.10 | 32% | 65–66% | 38–63 | 2.74–2.79 | 7–12% | 83–90 ms | 99.3–99.7% |
| mediasoup 3.30.1 | 31–33% | 54–58% | 132–163 | 3.79–3.82 | 30–32% | 35–44 ms | 98.1–98.4% |
| Jitsi stable-11248 | 51–52% | 23% | 31–40 | 2.17 | 54–55% | 191–192 ms | 99.3–100.0% |

The servers' estimates toward the capped bots during the cap:
- mediasoup, from its signaling (`server_estimate`): median 295–481 kbit/s, against about 8,500 before.
- Jitsi, read from the bridge (`scripts/read-estimates.py`): 723–984 kbit/s per capped bot, against about 19,800 before and for the other 7 bots.

LiveKit gives none, and Janus does no downlink estimation.

**US2: the same cap from 30 s to 50 s only (80 s hold), three runs each (9 capped bots)**

| Server | Reaction (median, longest) | Recovered before the end | Recovery (median, longest) | Capped bots' freezes per run |
|---|---|---|---|---|
| Janus 1.4.3 | 2 s, 2 s (freezes) | 9 of 9 | 4 s, 4 s | 201–327 |
| LiveKit 1.13.10 | 2 s, 2 s | 8 of 9 | 32 s, 34 s | 36–38 |
| mediasoup 3.30.1 | 2 s, 2 s | 7 of 9 | 6 s, 26 s | 109–141 |
| Jitsi stable-11248 | 2 s, 2 s | 6 of 9 | 38 s, 40 s | 37–44 |

**US3: 40 ± 10 ms and 3% random loss toward 3 bots, one run each**

| Server | Dropped by loss | Added delay (mean) | Video still missing after retransmission | Video latency p50, lossy / others | Freezes | Audio score |
|---|---|---|---|---|---|---|
| Janus 1.4.3 | 2.91–3.08% | 39.8–40.2 ms | 2.78–2.93% | 37.5 / 1.5 ms | 0 | 4.14 |
| LiveKit 1.13.10 | 2.92–3.10% | 40.1–40.5 ms | 0.00% | 37.5 / 1.5 ms | 0 | 4.15 |
| mediasoup 3.30.1 | 2.93–3.09% | 40.3–40.6 ms | 0.00% | 40.5 / 2.5 ms | 0 | 4.11 |
| Jitsi stable-11248 | 2.95–3.07% | 39.9–40.1 ms | 0.01% | 37.5 / 1.5 ms | 0 | 4.15 |

The mediasoup run of US3 fails one live-test check that is not about the condition: a bot in no group at its height 94.9% of its time, against 95%. That is mediasoup's start-up (015).

**US4: 400 kbit/s on the upload of 2 bots from 30 s to 60 s (80 s hold), one run each**

| Server | Senders adapting | Layer dropped before the cap | First drop after the cap began | All layers back after it ended | Unconstrained bots' freezes |
|---|---|---|---|---|---|
| Janus 1.4.3 | 2 of 2 | none | 1.3 s | 5.6 / 5.7 s | 0 |
| LiveKit 1.13.10 | 2 of 2 | none | 1.3 / 1.7 s | 5.6 / 5.8 s | 0 |
| mediasoup 3.30.1 | 2 of 2 | none | 1.3 / 1.4 s | 6.5 / 6.6 s | 0 |
| Jitsi stable-11248 | 2 of 2 | none | 1.4 s | 7.4 s | 0 |

**What the comparison says**
- **Lowering a slow receiver.** LiveKit, mediasoup and Jitsi lower the capped bots within one collection (2 s). Janus does not lower them: it forwards the substream asked for, and its capped bots freeze most.
- **The rest of the room.** No server lets a slow receiver hurt the others: the bots in no group stay at 720 at least 98.1% of the time.
- **Where the queue ends up.** LiveKit fits under the cap best: 7–12% dropped, 83–90 ms added. mediasoup holds the shortest queue (35–44 ms) by going well below the link (its estimate 300–480 kbit/s for 1200). Jitsi keeps the queue full (191 ms) and the capped bots' audio at 2.17.
- **Giving the height back.** mediasoup is fastest (median 6 s) and LiveKit next (32 s, from the bots' REMB). Jitsi is slowest (38 s), and a third of its capped bots had not recovered 30 s after the cap.
- **Loss.** 3% random loss with 40 ms delay costs no freeze anywhere. LiveKit, mediasoup and Jitsi retransmit everything, Janus almost nothing.
- **A slow sender.** All four servers handle it the same way: the sender's top layers stop, and come back 6–7 s after the link recovers. That part is the bots' own behaviour (R7), and the servers do not change it.

## SC-008: an adapting sender

Met on the four servers, 2026-11-15 (`runs/plans/slowup.json`, reports in `runs/us4/`, files `*-us4b`). The US1 room (10 bots, layered VP8, receive cap 4), with 400 kbit/s on the upload of the last 2 bots from 30 s to 60 s, 80 s hold. Jitsi after a warm-up run. Gate T004: all four servers send transport-wide feedback, so every constrained sender adapts (`adapts: true`).

| Server | First layer dropped after the cap began | Layers during the cap | All 3 layers back after the cap ended | Delivered on the capped upload (mean, highest 10 s) |
|---|---|---|---|---|
| Janus 1.4.3 | 1.4 s | 180, tries of 360 | 5.7 / 6.6 s | 326–330, 356–364 kbit/s |
| LiveKit 1.13.10 | 1.5 / 1.8 s | 180, tries of 360 | 5.9 / 7.9 s | 339–340, 375–379 kbit/s |
| mediasoup 3.30.1 | 1.3 / 1.4 s | 180, tries of 360 | 6.6 / 7.6 s | 352–354, 387–391 kbit/s |
| Jitsi stable-11248 | 1.3 / 1.4 s | 180, tries of 360 | 7.5 / 7.5 s | 347–351, 382–384 kbit/s |

Limits: within 5 s and within 10 s. The 180-line layer and audio need about 215 kbit/s; 360 adds 500, more than the cap. Each sender tries 360 again after 2 s, then 4 s, then 8 s: 11 layer changes per sender. The tries, and the retransmissions the servers ask for while they overflow the link, keep the delivered rate near the cap. 45–74% of what the senders offered was dropped by the cap; how much of it was retransmission was not broken down.

A first run on Janus (`*-us4`), before the warm-up of research R7, had both senders drop a layer 1 s after they started, on a clean path. They took it back 2 s later. The warm-up removed it: in the four runs above, no sender drops a layer before the cap.

The receivers' results for the constrained senders' streams apart (US4 scenario 6) are not built (tasks T038): a bot cannot tell which bot a received stream comes from. The 8 unconstrained bots received 720 99.3–100% of the time, with 0 freezes, on every server. With a receive cap of 4 of 9 senders, not every receiver received a constrained sender's stream.

## SC-009: no layer dropped on a clean path

Met. A bot whose group has no upload condition has no estimator and no layer control (`TestPeer_SenderEstimationOnlyWhenAdapting`, `TestLayerControl_OffUnchanged`), so it cannot drop a layer. The constrained senders of the four US4 runs, on a clean path for their first 30 s, dropped no layer before the cap, after the warm-up fix.


## R14: REMB where the server asks for it

Found at T022 (2026-11-15): on its defaults, LiveKit 1.13.10 sizes each receiver's streams from REMB; its offers ask for `goog-remb` with `abs-send-time` and no transport-cc. Before R14 the bots sent no REMB. Under a 500 kbit/s cap toward 3 of 10 bots (layered VP8, all asking for 720, receive cap 4), LiveKit kept forwarding 720 to them: 100% of their video at 720, 295 freezes, audio score 2.14 against 4.37. The user chose to make the bots send REMB, as a browser does (spec clarification, FR-019).

With R14 (receive-side estimate from abs-send-time, REMB once per second and at once on a fall):

| Run (LiveKit, defaults) | Capped bots' heights | Capped bots' freezes | Unconstrained bots at 720 |
|---|---|---|---|
| 500 kbit/s, before R14 | 720: 99.6% | 295 | 100% |
| 500 kbit/s, with REMB | 180: 57.5%, 720: 41.5% (720 was before the cap) | 395 | 99.7% |
| 1200 kbit/s, with REMB | 180: 64.2%, 360: 4.0%, 720: 31.7% (before the cap) | 113 | 100% |

500 kbit/s is below what the lowest layers need with 4 received streams: 4 × 150 kbit/s at 180, plus audio, is about 760 kbit/s. So the bots freeze whatever the server does (spec edge case "a cap lower than the smallest layer"). The plans of this feature use 1200 kbit/s instead, which carries 4 × 180 but not 4 × 360.

No change without conditions (SC-010, second half), the 014 T059 plan (10 bots, layers up to 720, receive cap 4, `180:0.5,720:0.5`, 1 minute), lowest bot's share at its requested height:

| LiveKit 1.13.10 | VP8 | VP9 |
|---|---|---|
| 016 with REMB | 0.989 (014: 0.985) | 1.000 (1.000) |

0 freezes in both. The VP9 run's live test fails only on its known 014 check: no VP9 latency on LiveKit (`frame_not_identified`, LiveKit drops RTP padding).

## R15: a browser's limit on retransmission requests

Found in the US1 Jitsi run (2026-11-15). Each capped bot was offered about 5,500 packets per second, against about 390 delivered: 236,860 packets offered in 43 s, 93% dropped.

Pion's NACK generator asks again for every missing packet of its 512-packet window every 100 ms, with no limit (`maxNacksPerPacket` 0). Under heavy loss the retransmissions are lost too, and the bridge resends them again. libwebrtc stops after 10 requests per packet (`kMaxNackRetries`). The bots now use that limit (`src/execution/pion.go`, `configureNack`). It changes nothing on a clean path, where nothing is asked for twice.

Same plan, Jitsi, before and after:

| | Packets offered to a capped bot | Dropped by the cap | Capped bots' freezes |
|---|---|---|---|
| unlimited requests | 233,716–247,311 | 93–94% | 28 |
| 10 requests per packet | 27,402–38,661 | 40–57% | 29 |

LiveKit and mediasoup, run before R15, did not show it: 20–36% dropped by the cap. They are run again with the limit in the US5 comparison.
