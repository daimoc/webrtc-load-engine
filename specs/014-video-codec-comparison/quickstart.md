# Quickstart: Video Codec Comparison (014)

How to check the feature end to end. Field details are in
[contracts/test-plan-codec.md](contracts/test-plan-codec.md) and
[contracts/report-codec.md](contracts/report-codec.md).

## Prerequisites

- Docker (content generation and the local servers).
- Go at `/usr/local/go/bin`.
- For SC-004: Chrome and Firefox on the workstation.
- For server CPU comparisons: the 013 remote machine, one server deployed at a
  time with `sfu-remote`.

## 1. Content (once; the files are committed)

```bash
scripts/gen-media.sh            # VP8 unchanged; VP9, AV1, H.264 single and simulcast heights; content.json
scripts/gen-media.sh svc        # builds scripts/svc-encoders, writes VP9 and AV1 L3T3_KEY content
```

Expected: `src/media/assets/content.json` lists every codec and height with
`abs(vmaf - vmaf_reference) <= 1.0`, and the VP8 files are byte-identical to
before (`git diff --stat src/media/assets/*.ivf` shows none of them).

## 2. Browser gate (before anything builds on the content)

Start the local Janus (`tests/platforms/janus`), run 2 bots per codec with a
long hold, and join the room from Chrome and Firefox. Each codec shows the
moving test pattern without artefacts, single stream and with `simulcast`.
If one does not, stop: its trailer carrier (research R9) needs revising.

## 3. One codec per run (US1)

```bash
set -a; . tests/platforms/livekit/.env; set +a
for c in vp8 vp9 av1 h264; do
  LIVEKIT_URL=ws://localhost:7880 LIVEKIT_ROOM=c014$c LIVEKIT_PARTICIPANTS=10 LIVEKIT_HOLD=60s \
  LIVEKIT_VIDEO_CODEC=$c go test -tags live -count=1 -v -run LiveKit ./tests/live/
done
```

Expected: four passes; each report's key figures say the codec sent and
"received <codec> by 10 bots". The same with `JITSI_`, `MEDIASOUP_` and
`JANUS_VIDEO_CODEC` on the other servers; `jitsi-connect --video-codec <c>`
for a manual Jitsi run.

Refusals: `video_codec: vp10` is refused naming the four codecs; `video_codec`
with profile `audio` is refused.

Mismatch: on a server set up without a codec (for example AV1 removed from
`config.mjs` for one run), the run fails with `codec_mismatch` naming the
codec found, and no figures are reported as AV1.

## 4. Same measures (US2)

For each codec run above, the report gives video freezes, frame rate,
resolution and latency, or a named reason for each missing one; none is
missing where the VP8 run on the same server has it.

## 5. Layers (US3)

```bash
JANUS_URL=ws://localhost:8188 JANUS_ROOM=$((8000+RANDOM%900)) JANUS_PARTICIPANTS=10 JANUS_HOLD=60s \
JANUS_VIDEO_CODEC=vp9 JANUS_SIMULCAST=720 JANUS_MAX_RECEIVED=4 \
JANUS_REQUESTED_HEIGHTS=180:0.5,720:0.5 go test -tags live -count=1 -v -run Janus ./tests/live/
```

Expected per codec and server: the bots asking for 180 receive 180 for at
least 95% of their measured time (SC-008), the others 720; the report says
`svc` (VP9, AV1) or `simulcast` (VP8, H.264). A server that forwards only the
lowest layer gives `layers_not_forwarded`, recorded in the validation report.

## 6. Comparison (US4, on the remote machine)

On the 013 machine, one server at a time: two VP8 runs (run-to-run
variation), then one run per codec, 10 bots, 1 minute, single stream; then
the layered runs. `validation-report.md` gets one table per server: codec,
content bitrate, server CPU and memory, MOS, freezes, latency, with the bot
count, duration and receive cap.

## 7. Engine cost and regressions

```bash
go test -run '^$' -bench 'Source|Quality' ./src/media/
go vet ./... && go test -race ./...
```

Per codec at 100 bots, engine CPU per bot within 10% of VP8's, or the
difference recorded (SC-006). Existing VP8 live tests give their usual
results (SC-007).
