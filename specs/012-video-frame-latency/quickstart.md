# Quickstart: Validating Video Latency on Rewriting Servers (012)

Prefix `PATH` with `/usr/local/go/bin`. Local servers are in
`tests/platforms/`. Read results in `reports/*.md`, line
`Latency p50 (audio / video)`.

## 1. Automated tests

```bash
go vet ./... && go test -race ./...
go test -run '^$' -bench 'QualityVideoPacket' ./src/media/
```

Expected: green, including tests where the path rewrites timestamps, foreign
frames and another scenario's frames; under 200 ns per packet, 0 allocations.

## 2. Janus (SC-001)

```bash
JANUS_URL=ws://localhost:8188 JANUS_ROOM=<new room> JANUS_PARTICIPANTS=10 \
  go test -tags live -count=1 -v -run Janus ./tests/live/
```

Expected: video latency in the report, all 10 bots contributing; audio still
`frame_not_identified`.

## 3. Jitsi with simulcast (SC-002)

```bash
JITSI_ROOM_URL=https://localhost:8443/<room> JITSI_XMPP_DOMAIN=meet.jitsi \
JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
JITSI_MEDIA=audio_video JITSI_SIMULCAST=720 JITSI_MAX_RECEIVED=4 \
JITSI_PARTICIPANTS=10 go test -tags live -count=1 -v -run Jitsi ./tests/live/
```

Expected: video latency in the report, audio latency as before.

## 4. Browser check, first (SC-008, research R5)

While a run holds, open the room on `localhost`, not `127.0.0.1`, in Chrome and
Firefox: Jitsi's web client, and Janus's demo page. Watch for 1 minute.

Expected: moving picture, no artifacts, no decoder errors in the console. If
this fails, stop and revisit the spec.

## 5. No regression (SC-005, SC-006, SC-007)

Alternated runs against a build from before this feature, same machine state
(figures from different days are not comparable):
- 50 bots on Jitsi without simulcast, LiveKit and mediasoup: video latency
  still available, median within 2 ms; freezes, frame rate, MOS unmoved.
- 100 bots: engine CPU +5% at most.
- Lowest layer bitrate +2% at most.

Record results in `validation-report.md`.
