# Quickstart: LiveKit Platform (008)

Validation guide. Contracts: [livekit-config.yaml](contracts/livekit-config.yaml),
[platform-boundary.md](contracts/platform-boundary.md),
[signaling-messages.md](contracts/signaling-messages.md),
[quality-summary-delta.yaml](contracts/quality-summary-delta.yaml).

## Prerequisites

- Docker; `docker ps` works without sudo (007 server monitoring).
- docker-jitsi-meet stopped while LiveKit runs, so the two servers don't share the CPU.
- `export PATH=/usr/local/go/bin:$PATH`.

## 1. Start the local LiveKit server (US3, SC-006)

```bash
cd tests/platforms/livekit
cp env.example .env        # then set LIVEKIT_API_SECRET (32+ characters)
docker compose up -d
docker logs livekit-server | grep -i "starting livekit"
```

Expected: `livekit-server` running, listening on 7880 (signal), 7881/tcp, and UDP ports from 50000 as connections open.

Common live variables:

```bash
set -a; . tests/platforms/livekit/.env; set +a
export LIVEKIT_URL=ws://localhost:7880 LIVEKIT_ROOM=load008
export LIVEKIT_SERVER_CONTAINERS=livekit-server
```

## 2. Unit, codec and fake-server tests

```bash
go vet ./... && go test -race ./...
go test -race -v ./src/platform/livekit/ ./src/platform/catalog/
```

Expected:
- the golden frames encode and decode byte for byte;
- the fake server covers join, publish, offers, subscriptions with cap and replacement, pings, the server's leave, and a 401;
- both peer modes pass the engine tests.

## 3. Creation-time rules (FR-002, FR-008, SC-005)

`go test -race -run 'LiveKit' -v ./tests/integration/api/`

Expected: 400s for:
- a missing `url` or `room`;
- a key without a secret;
- a token together with a key;
- `media.simulcast` or `media.requested_heights` on `livekit`.

A valid LiveKit plan is 201. Neither its answers nor the server's logs contain `api_secret`, and the same holds for a Jitsi plan with `jwt_app_secret`.

## 4. Ten bots (US1, US2, SC-001, SC-003, SC-004)

```bash
go test -c -tags live -o live.test ./tests/live/
LIVEKIT_MEDIA=audio_video LIVEKIT_PARTICIPANTS=10 LIVEKIT_HOLD=60s \
  ./live.test -test.v -test.run LiveKit
```

Expected:
- all 10 bots are `established`;
- each received 9 audio and 9 video streams;
- all bots leave within 10 s of the end (the room is empty in `docker logs livekit-server`);
- summary: `video.freezes_total` 0, audio `concealed_ratio.p50` < 0.01, `mos.p50` ≥ 4.0;
- `server.containers[0]` `livekit-server` `measured`;
- `latency_available` true, or false with `latency_reason` (record which, research R11).

Run the Jitsi live test with the same knobs, then compare the two summaries' key sets: `jq -S 'paths|join(".")'` must give the same list, apart from the optional keys absent in one of them.

## 5. Receive cap (FR-005)

`LIVEKIT_PARTICIPANTS=6 LIVEKIT_MAX_RECEIVED=2 ./live.test -test.v -test.run LiveKit`

Expected: each bot receives 2 audio and 2 video streams. With `LIVEKIT_LEAVE_ONE=1` (one bot leaves halfway), the bots that received it get a waiting stream within a few seconds.

## 6. Scale (SC-002)

As 006 quickstart step 4 (100 bots, `audio_video`, cap 10, ramp-up 20 s, 10 min, `/usr/bin/time -v`), against LiveKit:

Expected: every bot is connected for the whole hold, and `load_machine.engine.cpu_threads.average` is ≤ 2.0. Record the summary next to the same-day Jitsi run in tasks.md and validation-report.md.

## 7. Failures

- Wrong secret: `LIVEKIT_API_SECRET=wrong ./live.test -test.run LiveKit`. Every bot fails at `signaling`, reason `unauthorized`, and no secret or token appears in the output (`grep -c "$LIVEKIT_API_SECRET"` is 0).
- Server stopped mid-run (`docker stop livekit-server`): bots are reported as lost (`signaling_lost`), and the run ends normally.
