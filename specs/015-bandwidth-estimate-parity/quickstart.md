# Quickstart: Bandwidth Estimate Parity with a Browser (015)

How to check the feature end to end. Fields are in
[contracts/test-plan.md](contracts/test-plan.md) and
[contracts/report.md](contracts/report.md); the verdict rules in
[research.md](research.md) R11.

## Prerequisites

- Docker and the local servers of `tests/platforms`.
- Go at `/usr/local/go/bin`.
- Chrome or Chromium on the load machine (`google-chrome --version`).
- For SC-006 and SC-007: the 013 remote machine.

The addresses below marked *(R12)* are fixed by the first live task.

## 1. Facts first (research R12)

Run the checks of R12 and write the results at the top of
`validation-report.md`: the join-by-itself address of each page, the
estimate's source on each server, the feedback each server uses.

## 2. A browser in a run (FR-016 to FR-019)

```bash
MEDIASOUP_URL=ws://localhost:4443 MEDIASOUP_ROOM=b015 MEDIASOUP_PARTICIPANTS=3 MEDIASOUP_MEDIA=audio_video \
MEDIASOUP_BROWSER_URL='<demo address for room b015, R12>' MEDIASOUP_BROWSER_HEIGHT=360 \
go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

Expected: both report files have a browser part with `status: measured`,
height shares, and a share at 360; the bots' figures count 3 bots.

Failure cases, each with the run completing and the bots reported:

| Change | Browser part says |
|---|---|
| `MEDIASOUP_BROWSER_URL=http://localhost:1/` | `page_load_failed` |
| `PATH` without Chrome | `browser_not_found` |
| an address that waits for a click | `no_video_received` |
| no `*_BROWSER_URL` | no browser part; no browser process started |

## 3. Reception reports (US2)

Any run with video:

```bash
JANUS_URL=ws://localhost:8188 JANUS_ROOM=1015 JANUS_PARTICIPANTS=10 JANUS_MEDIA=audio_video \
go test -tags live -count=1 -v -run JanusLive ./tests/live/
```

Expected: `quality.reception_reports.available` is true, with one `per_bot`
entry per bot. An audio-only run gives `available: false`, `reason:
no_video`.

SC-002, a bot that leaves packets unacknowledged: the engine test with the
test-only switch (`go test -race ./src/execution -run ReceptionReports`)
shows the bot in `flagged` and the warning in the Markdown file.

## 4. The comparison (US1)

mediasoup, the 014 conditions (layers up to 720, receive cap 4, half the
bots asking for 720), with the server's TLS and the demo app of R12 fact 3
running (`https://localhost:5555`). mediasoup gives no per-receiver
estimate, so the comparison is on the layer forwarded over time (R7):

```bash
MEDIASOUP_URL=wss://localhost:4443 MEDIASOUP_INSECURE_TLS=1 MEDIASOUP_ROOM=c015m1 MEDIASOUP_PARTICIPANTS=10 \
MEDIASOUP_MEDIA=audio_video MEDIASOUP_SIMULCAST=720 MEDIASOUP_MAX_RECEIVED=4 MEDIASOUP_REQUESTED_HEIGHTS=180:0.5,720:0.5 \
MEDIASOUP_HOLD=60s MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo \
MEDIASOUP_BROWSER_URL='https://localhost:5555/?roomId=c015m1&produce=false&mic=false&webcam=false&displayName=browser' \
MEDIASOUP_BROWSER_HEIGHT=720 MEDIASOUP_BROWSER_INSECURE_TLS=1 REPORTS_DIR=specs/015-bandwidth-estimate-parity/estimates \
go test -tags live -count=1 -v -run MediasoupLive ./tests/live/ > specs/015-bandwidth-estimate-parity/estimates/mediasoup-vp8-1.log 2>&1
python3 -I scripts/estimate-verdict.py mediasoup <report>.json specs/015-bandwidth-estimate-parity/estimates/mediasoup-vp8-1.log
```

Jitsi, default bridge settings, the 014 conditions (10 bots, single
stream, 1 minute), after one warm-up run, VP8 then AV1; the estimates are
read from the bridge during the run:

```bash
python3 -I scripts/read-estimates.py jitsi > specs/015-bandwidth-estimate-parity/estimates/jitsi-vp8-1.jsonl &
JITSI_ROOM_URL=http://localhost:8000/c015j1 JITSI_XMPP_DOMAIN=meet.jitsi JITSI_MUC_DOMAIN=muc.meet.jitsi \
JITSI_WS_URL=ws://localhost:8000/xmpp-websocket JITSI_PARTICIPANTS=10 JITSI_MEDIA=audio_video JITSI_VIDEO_CODEC=vp8 \
JITSI_HOLD=60s JITSI_SERVER_CONTAINERS=docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1 \
JITSI_BROWSER_URL='https://localhost:8443/c015j1#config.prejoinConfig.enabled=false&config.startWithAudioMuted=true&config.startWithVideoMuted=true' \
JITSI_BROWSER_HEIGHT=360 JITSI_BROWSER_INSECURE_TLS=1 REPORTS_DIR=specs/015-bandwidth-estimate-parity/estimates \
go test -tags live -count=1 -v -run JitsiLive ./tests/live/
kill %1
python3 -I scripts/estimate-verdict.py jitsi specs/015-bandwidth-estimate-parity/estimates/jitsi-vp8-1.jsonl
```

Repeat each until a run shows a bot held low, or five show none. For each
run keep the two report files and the estimates file, and write in
`validation-report.md`: the table of `estimate-verdict.py`, the verdict,
and what differed between the browser's streams and a bot's.

A verdict of "the bots" is run once more with the server on the remote
machine (SC-006):

```bash
go run ./cmd/sfu-remote deploy mediasoup --host user@host
set -a; . ~/.config/webrtc-load-engine/sfu-remote/<host>.env; set +a
```

## 5. After a correction (US3)

- The comparison of step 4 again, five runs: every bot's estimate reaches
  its need within 1 s of the browser's (SC-003).
- mediasoup: five 10-bot, 1-minute layered VP8 runs with
  `180:0.5,720:0.5`, then one per other codec: lowest
  `requested_height_share` at 720 is at least 0.95 (SC-004).
- Jitsi, local defaults: one warm-up, then five VP8 and five AV1 10-bot,
  1-minute runs: 0 freezes (SC-005).
- LiveKit and Janus, the 014 layered runs per codec: shares within 0.02 of
  014's table (SC-008).
- Engine CPU per bot at 100 bots, before and after: within 5% (SC-008).

## 6. Server settings (US4)

```bash
go run ./cmd/sfu-remote deploy jitsi --host user@host
set -a; . ~/.config/webrtc-load-engine/sfu-remote/<host>.env; set +a
go run ./cmd/jitsi-connect --domain "$JITSI_XMPP_DOMAIN" --muc "$JITSI_MUC_DOMAIN" --ws "$JITSI_WS_URL" \
  --room s015 --participants 3 --media audio_video
```

Expected: both report files name `JVB_CC_TRUST_BWE=false`, stated by the
deployment (or, once FR-011 is met, "none changed by the deployment"). The
same run on the local server, with no setting given, says "not known to the
run".

## 7. Always

```bash
go build ./... && go vet ./... && go test -race ./...
```
