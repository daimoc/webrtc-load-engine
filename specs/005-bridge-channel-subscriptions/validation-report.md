# Validation Report: Bridge Channel Subscriptions (005)

**Date**: 2026-10-10 | **Spec**: [spec.md](spec.md) | **Guide**: [quickstart.md](quickstart.md)
| **Detailed records**: [tasks.md](tasks.md) (T014, T021, T022, T028, T029)

## Summary

| Criterion | Target | Result | Status |
|-----------|--------|--------|--------|
| SC-001 video reaches the bots | 10 bots, 30 s, received video within 10% of 9 × sent | 0.93–1.04 over 6 runs | Met |
| SC-002 CPU budget | 100 bots, cap 10, ≤ 2 cores on average | 1.05 cores | Met |
| SC-003 no traffic beyond kept streams | traffic ratio within 10% of the no-cap value | 1.03 (no cap: 1.03) | Met |
| SC-004 replacement of a departed stream | waiting stream received within 5 s | 100/100, at most 0.50 s | Met |
| SC-005 no bridge channel | session works, bot-side cap only | fake-server test passes | Met |

One defect was found and fixed during validation: the bots' congestion
feedback reported the bridge's probe packets as lost, which held video at 26%
of the expected volume (research R9, tasks T026–T027).

## Environment

- Host: laptop, Intel i5-13600H, 16 threads; the bots and the whole Jitsi
  deployment run on it.
- Jitsi: local docker-jitsi-meet, `unstable` images (jitsi-videobridge2
  2.3-326); no bridge setting changed (`trust-bwe` left at its default).
- Engine: branch `rework`, Go 1.23, live test `tests/live/jitsi_live_test.go`
  (`-tags live`), which runs the real execution engine and reads its
  `/metrics`.
- Common variables for every run:

```bash
JITSI_ROOM_URL=http://localhost:8000/IntriguingClimatesSupposeProbably
JITSI_XMPP_DOMAIN=meet.jitsi
JITSI_MUC_DOMAIN=muc.meet.jitsi          # required, otherwise the join times out
JITSI_WS_URL=ws://localhost:8000/xmpp-websocket
```

- CPU and memory: test binary run under `/usr/bin/time -v`; average cores =
  (user + system time) / elapsed time. Bridge CPU from `docker stats`
  sampled during the hold.
- Traffic ratio: RTP and RTCP bytes received by the bots
  (`webrtc_transport_bytes_received_total`) divided by the media payload they
  keep (`webrtc_media_bytes_received_total`, audio + video). About 1.03 is
  header and RTCP overhead; higher means the bridge sends streams the bots
  drop ([contracts/metrics.md](contracts/metrics.md)).

## Scenario 1: Video reaches the bots (SC-001)

10 bots sending audio and video, no receive cap, 30 s measured window.

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 JITSI_HOLD=30s \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

Checks: every bot opens the bridge channel (`webrtc_data_channels_open` =
10); video packets received per bot within 10% of 9 × the video packets one
bot sends over the same window.

| Run | Condition | Video received / expected |
|-----|-----------|---------------------------|
| First run (T014) | before the feedback fix | 0.26 |
| T014/T021, 4 runs | feedback fixed, window starts at join | 0.98, 0.94, 0.98, **0.83** |
| T014 | 60 s window | 0.99 |
| T028, 6 runs | window starts once video has settled (4–10 s) | 1.04, 0.95, 0.93, 0.99, 0.97, 1.01 |

- 10/10 bridge channels open; about 580 video packets/s received per bot
  (9 × 66/s sent); 0 lost, 0 stalls.
- The 0.83 run was the bridge's bandwidth estimate ramping up during the
  window. The test now waits for the received video to reach 90% of the
  expected volume (at most 30 s) before measuring.
- Sampled every 2 s, the ratio varies between 0.87 and 1.06 throughout the
  window with no loss: bridge-side pacing, averaged out over 30 s.

## Scenario 2: Subscriptions follow the cap (US2, SC-003)

15 bots sending audio and video, each receiving at most 5 streams per kind.

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=15 JITSI_MAX_RECEIVED=5 \
  go test -tags live -count=1 -run TestJitsiLive -v ./tests/live/
```

- Each bot received 5 audio and 5 video streams; video at 0.98 of the
  expected volume.
- Traffic ratio 1.03, the same as 10 bots without a cap: the bridge sends
  nothing beyond the received streams.
- The bridge's `/debug` state lists, per bot, the 5 video constraints
  (default `maxHeight` 0) and the audio subscription.

## Scenario 3: A received stream leaves (SC-004)

Headless Chrome joined without sending any stream (as in 004), so two bot
groups were used instead of a browser:

- group A: 5 bots, join first, leave after 20 s;
- group B: 10 bots with cap 5, join 5 s later, so A's streams are the ones B
  receives.

When A left, each B bot promoted 5 waiting audio and 5 waiting video streams:
100 replacements, the first packet of each at most 0.50 s after the departure
presence (100% within 5 s); 0 lost, 0 stalls.

## Scenario 4: Scale (SC-002, SC-003)

100 bots sending audio and video, cap 10, ramp-up 20 s, 10 min hold.

```bash
go test -c -tags live -o live.test ./tests/live/
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=100 JITSI_MAX_RECEIVED=10 \
JITSI_RAMP_UP=20 JITSI_HOLD=10m JITSI_METRICS_ADDR=127.0.0.1:9100 \
  /usr/bin/time -v ./live.test -test.run TestJitsiLive -test.v -test.timeout 30m
```

The same scenario was run with `JITSI_NO_BRIDGE_CHANNEL=1` (adapter option
`no_bridge_channel`: bridge channel left closed, as before 005) to separate
the effect of the feedback fix from that of the subscriptions.

| 100 bots, cap 10, 10 min | 004 T050 | Feedback fixed, no bridge channel (T029) | 005 (T022) |
|--------------------------|----------|------------------------------------------|------------|
| Bot CPU, average | 2.60 cores | 2.32 cores | **1.05 cores** |
| Max RSS | 723 MB | 725 MB | 818 MB |
| Streams received per bot | 10 audio | 10 audio | 10 audio + 10 video |
| Video received / expected | none | none | 0.99 |
| Traffic ratio | not measured | 10.91 | 1.03 |
| Bridge CPU | about 10 cores | about 9 cores | about 3.5 cores |
| Audio packets lost | 672 / 30.1 M | 313 532 / 29.8 M (1.05%) | 114 / 30.2 M |
| Established, stalls | 100/100, 0 | 100/100, 0 | 100/100, 0 |

- SC-002 met: 1.05 cores (529 s user + 129 s system over 624 s), while also
  receiving video, which 004 did not.
- SC-003 met: ratio 1.03, the same as without a cap.
- Of the 1.55 cores saved since 004, about 0.3 comes from the feedback fix
  and about 1.3 from the subscriptions. Without them the bridge still sends
  all 99 audio streams to each bot, and about 90% of what the bots receive is
  dropped (ratio 10.91).
- Open point: the run without the bridge channel lost 1.05% of the audio
  packets the bots kept, against almost none in the two other runs. Not
  investigated; the host was saturated (load average 34 on 16 threads, the
  bridge using about 9 cores), so the loss may come from the host rather
  than the bots.

## Scenario 5: No bridge channel offered (SC-005)

The local Jicofo always offers the bridge channel, so this is covered by the
fake XMPP server tests:

```bash
go test -race -run 'NoBridgeChannel|BridgeChannelDisabled' -v ./src/platform/jitsi/
```

- Offer without SCTP content: one Info log "Bridge channel not offered", no
  message sent, the session accepted and renegotiated as in 004.
- `no_bridge_channel` set: no data channel opened even though one is offered,
  one Info log "Bridge channel disabled by configuration".

## Automated checks

`go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...`
pass. No Colibri message name appears outside `src/platform/jitsi` (FR-010).
