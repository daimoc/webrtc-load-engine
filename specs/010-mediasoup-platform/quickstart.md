# Quickstart: validating the mediasoup-demo platform

Prerequisites: Docker, the Go toolchain (`export PATH=/usr/local/go/bin:$PATH`),
the repository root as working directory, and no other media server
running (stop Jitsi and LiveKit, except for step 7).

## 1. Start the local server (US3, FR-013, FR-014, SC-006)

```bash
cd tests/platforms/mediasoup
cp env.example .env              # MEDIASOUP_NUM_WORKERS=2
docker compose up -d --build     # first start builds the demo at the pinned commit
docker logs mediasoup-demo 2>&1 | grep -i "listening\|worker"
cd -
```

Expected:
- the container `mediasoup-demo` runs with 2 workers;
- WebSocket on `ws://localhost:4443`;
- the WebRTC server on UDP and TCP 44444 and 44445.

Time the steps from a clean checkout up to the end of step 3 (SC-006:
under 15 minutes).

## 2. Creation-time rules (FR-001, FR-002, FR-008, SC-005)

With the API running (`go run ./src`), each request below gets a 400 with
the message of [contracts/mediasoup-config.yaml](contracts/mediasoup-config.yaml):

- no `room`
- `url: "ftp://x"`
- an unknown key
- `media.simulcast` set
- `media.requested_heights` set
- `insecure_tls: "yes"`

A valid plan with `unlimited_room_secret: "s3cret"` creates. Its answer
and its creation log show `"[redacted]"` and never `s3cret`.

## 3. 10 bots, 1 minute (US1, US2, SC-001, SC-003, SC-004)

```bash
export MEDIASOUP_URL=ws://localhost:4443 MEDIASOUP_ROOM=load010 MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo
MEDIASOUP_PARTICIPANTS=10 MEDIASOUP_MEDIA=audio_video MEDIASOUP_HOLD=60s \
  go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

Expected:
- PASS: 10 bots established;
- each bot is offered 9 audio and 9 video streams and receives packets of
  both kinds;
- all bots left within 10 s of the end;
- the logged summary has no freezes, audio gaps below 1%, and an audio MOS
  p50 ≥ 4.0;
- the server section reports `mediasoup-demo` with CPU and memory;
- one pair of files `reports/<time>-mediasoup-live-mediasoup-<run>.{json,md}`.

Compare the JSON's field set with a LiveKit report of the same plan:

```bash
jq -S 'paths|map(tostring)|join(".")' reports/*mediasoup*.json | sort -u > /tmp/ms
jq -S 'paths|map(tostring)|join(".")' reports/*livekit*.json   | sort -u > /tmp/lk
diff /tmp/ms /tmp/lk
```

There must be no difference other than array indexes and the
`platform_config` keys (SC-004).

## 4. Latency (FR-011, research R8)

In the step 3 summary, `audio.latency` and `video.latency` are present.
If one is absent, its `latency_reason` must be set (`frame_not_identified`
or `not_measured`), and the value must never be 0. Record which case
occurred in the validation report.

## 5. Receive cap and replacement (FR-005, US1 scenarios 3 and 6)

```bash
MEDIASOUP_PARTICIPANTS=12 MEDIASOUP_MAX_RECEIVED=5 MEDIASOUP_LEAVE_ONE=1 MEDIASOUP_HOLD=60s \
  go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

Expected:
- each bot is offered exactly 5 streams per kind for the whole hold;
- after the extra bot leaves halfway, its streams are replaced by waiting
  ones within 2 s;
- the server log has the declined consumers at WARN (`consume() | failed:
  … receive cap`) and no errors.

## 6. Receive-only (US1 scenario 4)

Run a 3-bot `audio_video` plan with a 5-minute hold. While it runs, run
a 2-bot `receive_only` plan in the same room:

```bash
MEDIASOUP_PARTICIPANTS=2 MEDIASOUP_MEDIA=receive_only MEDIASOUP_HOLD=30s \
  go test -tags live -count=1 -v -run Mediasoup ./tests/live/
```

Expected: the receive-only bots are established and receive 3 audio and 3
video streams each. A receive-only bot alone in an empty room is also
established (research R3).

## 7. 100 bots, alternated with LiveKit (US3 scenario 3, SC-002, research R13)

Use the same plan as 009's T027 (100 bots, `audio_video`, cap 10, ramp-up
20 s, 10 min hold), with the binaries built once (`go test -c -tags live`).
Run LiveKit, mediasoup, LiveKit, mediasoup:
- restart the server under test before each run;
- stop the other server during each run;
- keep the screen state the same for all four runs.

Record for each run:
- `/usr/bin/time` engine cores and `load_machine`;
- server CPU average and peak;
- MOS p50, freezes, bots established.

Expected: every mediasoup bot stays established for the whole hold, and the
engine's CPU on mediasoup is within run-to-run noise of LiveKit's. If
the server peaks at one worker's thread (about 1.0 thread for the media
alone), repeat the mediasoup runs with `use_pipe_transports: true` and
record both modes.

## 8. Room time limit (edge case, research R9)

Restart the server with `UNLIMITED_ROOM_SECRET=s` in `.env`, then run a
2-bot plan:
- without the secret and a hold of 11 min: the bots are reported lost at
  about 10 min, with the server's close in the log;
- with `MEDIASOUP_UNLIMITED_ROOM_SECRET=s` in another room: the run
  completes.

Remove the variable afterwards.

## 9. Nothing else changed (FR-009)

```bash
go vet ./... && go vet -tags live ./tests/live/ && go test -race -count=1 ./...
```

Everything must pass. Then rerun the Jitsi and LiveKit 10-bot live tests:
same results as before this feature.

## Stop

```bash
cd tests/platforms/mediasoup && docker compose down
```
