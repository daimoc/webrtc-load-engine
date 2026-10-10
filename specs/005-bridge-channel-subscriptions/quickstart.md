# Quickstart: validating Bridge Channel Subscriptions

Run against the local docker-jitsi-meet
(`https://localhost:8443/IntriguingClimatesSupposeProbably`, XMPP domain
`meet.jitsi`, WebSocket `ws://localhost:8000/xmpp-websocket`), as in
[004's quickstart](../004-synthetic-media/quickstart.md). Go is at
`/usr/local/go/bin`.

## 0. Unit and contract tests

```bash
export PATH=$PATH:/usr/local/go/bin
go vet ./... && go test -race ./...
```

Expected: green, including the data channel contract tests and the fake
server tests of [contracts/signaling-handler.md](contracts/signaling-handler.md)
and [contracts/colibri-messages.md](contracts/colibri-messages.md).

## 1. Video reaches the bots (US1, SC-001)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_EXPECT_VIDEO=1 \
  go test -tags live -run TestJitsiLive -v ./tests/live/
```

Expected: every bot reports `webrtc_data_channels_open` = 10; received video
packets per bot within 10% of 9 × the video packets one bot sends (the live
test's video check, now enabled by `JITSI_EXPECT_VIDEO`). With
`JITSI_LOG_XMPP=1` (debug logs) the adapter logs the `ReceiverVideoConstraints` sent on
open.

## 2. Subscriptions follow the cap (US2 scenarios 1 and 2)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=15 JITSI_MAX_RECEIVED=5 \
  go test -tags live -run TestJitsiLive -v ./tests/live/
```

Expected: each bot receives 5 audio and 5 video streams; the ratio of
[metrics.md](contracts/metrics.md) stays near the overhead value measured in
step 1 (no extra streams sent by the bridge). Then, from a browser in the
room, join and leave: a waiting stream replaces the browser's within 5 s
(SC-004), seen in the bots' debug logs (new message sent) and in the
received-stream counts.

Cross-check on the server side: the bridge's `/debug` endpoint (port 8080
in the `jvb` container) lists each endpoint's audio subscription and video
constraints; `docker stats docker-jitsi-meet-jvb-1` NET I/O should drop
compared with the same run on 004.

## 3. Scale (US2, SC-002, SC-003)

As 004 quickstart step 5, 100 bots, audio and video, cap 10, 10 minutes,
with `JITSI_METRICS_ADDR` set to read `/metrics` during the run.

Expected: average CPU of the test process ≤ 2 cores; every bot established
and `webrtc_data_channels_open` = 100; `webrtc_media_remote_streams` =
100 × 10 per kind; the traffic ratio of [metrics.md](contracts/metrics.md)
within 10% of step 1's (SC-003). Record CPU, memory, ratio and the bridge's
NET I/O in tasks.md, next to the 004 numbers (2.60 cores, 723 MB).

Not needed on the local deployment once the bots acknowledge every packet
(research R9). If video stays below the requested streams after the first
minute (research R6), add `videobridge.cc.trust-bwe = false` to the bridge's
`custom-jvb.conf` (jvb config volume), restart the `jvb` container, rerun
and record it.

## 4. No bridge channel (SC-005)

Covered by the fake-server test that offers no SCTP content (the local
Jicofo always offers it to the bots since 003 T056):

```bash
go test -race -run 'NoBridgeChannel' -v ./src/platform/jitsi/
```

Expected: one `Info` log "bridge channel not offered", no message sent, the
session accepted and renegotiated as in 004 (bot-side cap only, no
failure).
