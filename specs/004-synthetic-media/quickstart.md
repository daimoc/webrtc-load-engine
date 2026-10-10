# Quickstart: Synthetic Media Sending and Media Metrics

Validation guide for [spec.md](spec.md). Configuration:
[contracts/test-plan-media.yaml](contracts/test-plan-media.yaml). Metrics:
[contracts/metrics.md](contracts/metrics.md).

## Prerequisites

- Go at `/usr/local/go/bin` (`export PATH=$PATH:/usr/local/go/bin`).
- The local docker-jitsi-meet running: web at
  `https://localhost:8443/IntriguingClimatesSupposeProbably`, XMPP domain
  `meet.jitsi`, WebSocket `ws://localhost:8000/xmpp-websocket`.
- Only run multi-bot tests against deployments you operate.

Shared environment for the live test (see `specs/003-xmpp-signaling/quickstart.md`):

```bash
export JITSI_ROOM_URL=http://localhost:8000/intriguingclimatessupposeprobably \
       JITSI_XMPP_DOMAIN=meet.jitsi JITSI_MUC_DOMAIN=muc.meet.jitsi \
       JITSI_WS_URL=ws://localhost:8000/xmpp-websocket
```

## 1. Unit and contract tests

```bash
go vet ./... && go test -race ./...
```

Expected: green. Covers profile validation, content loading, the pacer
schedule and fan-out, inbound accounting (loss, jitter, round trip), stall
detection, the SDP expansion and collapse for Jitsi, source announcement and
`source-add` / `source-remove` renegotiation against the fake XMPP server.

## 2. Backward compatibility (FR-002)

```bash
JITSI_PARTICIPANTS=2 go test -tags live -count=1 -v ./tests/live/
```

Expected: as in 003; bots answer `recvonly`, no `webrtc_media_packets_sent_total`
increase.

## 3. Bots seen and heard in a browser (US1, SC-001)

```bash
go run ./cmd/jitsi-connect --domain meet.jitsi --muc muc.meet.jitsi \
  --ws ws://localhost:8000/xmpp-websocket --room intriguingclimatessupposeprobably \
  --participants 2 --media audio_video
```

Then open `https://localhost:8443/IntriguingClimatesSupposeProbably` in a
browser. Expected: two tiles with the moving test pattern, the beeping tone
audible, neither shown as muted, within 5 s of joining. Ctrl+C: both tiles
disappear. Repeat with `--media audio`: no video tiles, audio audible.

## 4. Media metrics (US2, SC-002, SC-005)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_HOLD=30s \
  go test -tags live -count=1 -v ./tests/live/
```

Expected: the test passes and logs, per kind, packets sent and received,
loss, the number of remote streams per bot (9 audio and up to 9 video when
the bridge forwards everything) and zero stalls. Received volume is within
10% of `streams × per-stream rate`.

With `JITSI_METRICS_ADDR=127.0.0.1:9100`, `curl -s 127.0.0.1:9100/metrics |
grep webrtc_media_` shows the series of the contract while the test runs
(and `/debug/pprof/` profiles it). Through the HTTP API, the same series are
on the server's `/metrics` (`"media": {"profile": "audio_video"}` in the test
plan).

Video is only checked with `JITSI_EXPECT_VIDEO=1`: a Jitsi bridge forwards no
video to a receiver that has not sent its receiver constraints over the
bridge channel. The bots open that channel (003 T056) but do not send
constraints yet (feature 005).

Late joiner: during a run, join from a browser with camera on. Expected:
`webrtc_media_remote_streams{kind="video"}` grows by the number of bots
(source-add handled); leaving makes it shrink with no stall counted.

## 5. Scale on one machine (US3, SC-003, SC-004)

`JITSI_MAX_RECEIVED` sets `max_received_streams` (SC-004 uses 10). Without
it, every bot receives all 99 other audio streams (R10, R12).

```bash
JITSI_MEDIA=audio_video JITSI_MAX_RECEIVED=10 JITSI_PARTICIPANTS=100 JITSI_HOLD=600s \
  /usr/bin/time -v go test -tags live -count=1 -timeout 20m -v ./tests/live/
```

Expected: all 100 bots established and none failed; the 99th percentile of
`webrtc_media_send_lateness_seconds` under 20 ms; average CPU of the test
process at most 2 cores (`User time + System time` / elapsed). Record the
received streams per bot next to the CPU figure: if the bridge forwards all
streams and the budget is exceeded, rerun with the bridge's last-N set
(research R10) and report both.
