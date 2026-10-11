# Quickstart: Validate Media Quality

Prerequisites: the local docker-jitsi-meet of 004/005 and the live test
variables (README "Development"):

```bash
export JITSI_ROOM_URL=http://localhost:8000/IntriguingClimatesSupposeProbably \
  JITSI_XMPP_DOMAIN=meet.jitsi JITSI_MUC_DOMAIN=muc.meet.jitsi \
  JITSI_WS_URL=ws://localhost:8000/xmpp-websocket
```

The live test logs the quality summary at the end of each run (the same one
`GET /api/v1/scenarios/{id}/quality` returns,
[contracts/quality-summary.yaml](contracts/quality-summary.yaml)).

## 0. Controlled tests (SC-002, SC-003, SC-004) and cost per packet

```bash
go test -race -run 'Freeze|RFactor|MOS|Latency' -v ./src/media/ ./src/execution/
go test -run '^$' -bench Quality ./src/media/
```

Expected: a 1 s interruption gives one freeze of 0.9–1.2 s and no stall;
scores within 0.1 of the hand-computed values; with a 50 ms delay on the
test network, median latency within 5 ms of 50 ms; the benchmark under
200 ns per packet.

## 1. Receiver quality (US1, SC-001, SC-007)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 JITSI_HOLD=60s \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

Expected in the summary: median frame rate 27–33, height 360 for 100% of
the time, 0 freezes, audio concealed ratio p95 under 1%, median MOS ≥ 4.0;
the worst bots listed with their peer IDs.

## 2. Latency (US2, SC-004)

Same run as step 1. Expected: latency reported for audio and video
(`latency_available: true`), median below 50 ms on the local deployment. If
video latency is missing while audio is present, the bridge rewrote video
timestamps (research R5 risk): apply the fallback and record it.

## 3. Simulcast (US3, SC-005)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 JITSI_HOLD=60s \
JITSI_SIMULCAST=360 JITSI_REQUESTED_HEIGHTS=360:0.5,90:0.5 \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

Expected: the bridge's `/debug` state shows three layers per bot; bots asking
for 360 receive 360 and those asking for 90 receive 90, each at least 95% of
the time (summary `height_share` and the per-bot log); switches counted.
Repeat with `JITSI_SIMULCAST=720 JITSI_REQUESTED_HEIGHTS=720:0.5,180:0.5`.

## 4. Scale (SC-006, SC-008)

As 005 quickstart step 3 (100 bots, `audio_video`, cap 10, ramp-up 20 s,
10 min, binary under `/usr/bin/time -v`), three runs:

1. no simulcast: ≤ 1.26 cores (20% over 005's 1.05) and ≤ 2 cores;
2. `JITSI_SIMULCAST=360`: ≤ 2 cores;
3. `JITSI_SIMULCAST=720`: record CPU, memory and the bridge's CPU (no budget).

Record CPU, RSS, the summary (median MOS, freeze seconds, latency p95) and
the bridge CPU (`docker stats`) in tasks.md and the validation report.
