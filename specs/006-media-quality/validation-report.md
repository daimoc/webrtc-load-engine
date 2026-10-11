# Validation Report: Media Quality (006)

**Date**: 2026-10-11 | **Spec**: [spec.md](spec.md) | **Guide**: [quickstart.md](quickstart.md)
| **Detailed records**: [tasks.md](tasks.md) (T025, T027, T028, T032, T039, T041, T057)

## Summary

| Criterion | Target | Result | Status |
|-----------|--------|--------|--------|
| SC-001 receiver quality, 10 bots | frame rate within 10%, sent resolution, no freezes, audio gaps < 1%, median MOS ≥ 4.0 | 30.0 fps, 360 lines 100% of the time, 0 freezes during the hold, 0% concealed, MOS 4.37 | Met, with start-up freezes (see below) |
| SC-002 controlled freeze | 1 s interruption → one freeze of 0.9–1.2 s, no stall | one freeze, no stall (`TestPeer_VideoFreeze`) | Met |
| SC-003 E-model score | within 0.1 of reference values | 5 values computed by hand from the formulas, within 0.01 | Met |
| SC-004 latency | median within 5 ms of a known 50 ms delay; latency for 100% of bot streams locally | 50.5 ms for audio and video; latency on all 10 bots, both kinds | Met |
| SC-005 simulcast | each bot gets its requested height ≥ 95% of the time | every bot gets the lowest layer | **Not met** (open, T057) |
| SC-006 CPU with quality on | ≤ 2 cores and ≤ 20% over the same run without it | 1.72 cores against 1.71 the same day | Met |
| SC-007 summary readable | quality and worst bots from the summary alone | percentiles, totals and worst-10 lists with peer IDs | Met |
| SC-008 simulcast at scale | 360: ≤ 2 cores; 720: measured | not run | **Blocked by SC-005** |

## Environment

As for 005 ([../005-bridge-channel-subscriptions/validation-report.md](../005-bridge-channel-subscriptions/validation-report.md)):
- Host: Intel i5-13600H laptop, 16 threads, running both the bots and Jitsi.
- Jitsi: local docker-jitsi-meet, jitsi-videobridge2 2.3-326, default settings.
- Engine: branch `rework`, Go 1.23, live test `tests/live/jitsi_live_test.go`, same common variables.

CPU is measured as (user + system) / elapsed time of the test binary, run
under `/usr/bin/time -v`. It varies with the host state: the 005 code took
1.05 cores in 005 T022 and 1.71 cores on 2026-10-11. So SC-006 compares
runs made one after the other on the same day.

## Scenario 0: controlled tests and cost per packet

```bash
go test -race -run 'Freeze|EModel|RFactor|MOS|Latency' -v ./src/media/ ./src/execution/
go test -run '^$' -bench Quality ./src/media/
```

- Freeze: one freeze of 0.9–1.2 s, 0 stalls, `webrtc_media_freezes_total` = 1.
- Score: R and MOS within 0.01 of the hand-computed values (for example 5%
  loss with 200 ms delay → R 70.12, MOS 3.603). The best score is 4.41: G.711
  with loss concealment values stand in for Opus (research R4).
- Latency: 50.5 ms median for audio and video through a `vnet` router with a
  50 ms minimum delay.
- Cost: 43 ns per video packet and 46 ns per audio packet, including the
  004 accounting, with 0 allocations (budget 200 ns).

## Scenario 1–2: receiver quality and latency, 10 bots (SC-001, SC-004, SC-007)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 JITSI_HOLD=60s \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

- Frame rate p50 30.0; height 360 for 100% of the received time; 0 switches;
  0 keyframe requests sent.
- Audio concealed 0%; MOS p50 4.37.
- Latency: available for audio and video on every bot, median under 1 ms;
  worst bot p95 1.5 ms. The bridge keeps VP8 RTP timestamps (research R5
  risk), so no fallback was needed.
- **Start-up freezes.** When 10 bots join at once, the bridge sends each
  stream's first keyframe and then holds some streams for 1–27 s, until its
  bandwidth estimate allows them. Each bot receives 9 streams, about
  4.5 Mbit/s.
  - Two runs counted 32 and 18 such freezes, and two later runs 5 and 0.
  - With 4 bots there were none.
  - They are real (a participant sees a frozen first image) and stay in the
    summary. The live test checks freezes over the hold, after video settled.

## Scenario 3: simulcast (SC-005) — not met

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 JITSI_HOLD=60s \
JITSI_SIMULCAST=360 JITSI_REQUESTED_HEIGHTS=360:0.5,90:0.5 \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

What works:
- The bots send three layers. Pion lists them in its answer and sends them
  (`TestPeer_SimulcastEncodingsBound`).
- The `SIM` group reaches the bridge: its `/debug` state shows three
  encodings per bot at 90/180/360 lines and 91/163/515 kbit/s.
- The bots ask for their height in `ReceiverVideoConstraints`.

What fails:
- Every bot receives only encoding 0, both at 360 (all get 90) and at 720 (all get 180).
- The bridge's allocation for a bot asking for 360 has a bandwidth estimate
  of 19.8 Mbit/s and an `ideal` of encoding 1, but its `target` stays at
  encoding 0.
- The allocator labels encoding 1 as 360, as if it assumed Jitsi web's
  180/360/720 ladder.

Tried and reverted:
- The bridge logged our VP8 packets as missing an extended picture ID, with
  no temporal-layer fields, and sent heavy probing padding (6.45× the media
  payload).
- A browser-like VP8 descriptor (15-bit picture ID, TL0PICIDX, TID) removed
  the padding, but the target still stayed at encoding 0.
- The same descriptor degraded single-stream video at 100 bots: freezes went
  from 131 to 30,321 and video latency was no longer found. It was reverted.

Open: why the bridge does not raise the target. Candidates are its
allocation for sources neither on stage nor selected, its assumed layer
heights, and temporal-layer expectations (browsers send VP8 with three
temporal layers).

## Scenario 4: scale, 100 bots, cap 10, 10 min (SC-006)

```bash
go test -c -tags live -o live.test ./tests/live/
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=100 JITSI_MAX_RECEIVED=10 \
JITSI_RAMP_UP=20 JITSI_HOLD=10m \
  /usr/bin/time -v ./live.test -test.run TestJitsiLive -test.v -test.timeout 30m
```

| Same day, one after the other | CPU | Max RSS | Lost (audio / video) |
|-------------------------------|-----|---------|----------------------|
| 005 code (HEAD 651d34c), no quality | 1.71 cores | 827 MB | 1,935 / 4 |
| 006 code, quality on | 1.72 cores | 856 MB | 3,262 / 1 |

- A profile during the hold puts the quality accounting at about 0.05 core.
  The rest is Pion's receive path and the send fan-out, as in 005.
- Summary of the 006 run:
  - Video: frame rate p50 30.0; height 360 for 100% of the time; 131 freezes
    (74 in the start-up minute; freeze seconds p50 0, p95 14.5, p99 87);
    1,797 keyframe requests sent, 487 received; latency p50/p95/p99
    6.5/7.5/8.5 ms.
  - Audio: concealed p95 0.27%; MOS p50 4.35; latency p50 3.5 ms.
  - Worst bots, from the summary alone (SC-007): lowest MOS 4.34; most freeze
    time 90 s; highest latency p95 23.5 ms.
