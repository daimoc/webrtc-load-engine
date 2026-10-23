# Quickstart: validating the Janus platform

Prerequisites:
- Docker and the Go toolchain (`export PATH=/usr/local/go/bin:$PATH`);
- the repository root as working directory;
- no other media server running, except during step 8.

## 1. Start the local server (US3, FR-015, SC-006)

```bash
cd tests/platforms/janus
cp env.example .env
docker compose up -d --build     # the first start builds Janus v1.4.3
docker logs janus 2>&1 | grep -i "websockets\|videoroom\|rtp port"
cd -
```

Expected:
- the containers `janus` and `janus-demos` run;
- WebSocket on `ws://localhost:8188`;
- RTP ports 20000–20999;
- the demos on `http://localhost:8090`.

Time the steps from a clean checkout up to the end of step 4 (SC-006:
under 15 minutes).

## 2. Unit and contract tests without a server (FR-017, SC-007)

```bash
go vet ./... && go test -race ./src/platform/janus/ ./src/platform/catalog/ ./src/...
```

Expected: PASS, with no network access beyond loopback. The fake Janus
server covers:
- join, and the room-creation race (427);
- publish;
- the receive cap: a publisher leaves and a waiting stream is promoted;
- keepalives;
- a rejected request, a closed WebSocket, and a session `timeout`.

## 3. Creation-time rules (FR-001, FR-002, FR-010, SC-005)

With the API running (`go run ./src`), each request below gets a 400 with
the message of [contracts/janus-config.yaml](contracts/janus-config.yaml):

- no `room`
- `room: "abc"`
- `room: 0`
- `url: "ftp://x"`
- an unknown key
- `media.simulcast` set
- `media.requested_heights` set
- `insecure_tls: "yes"`

A valid plan with `api_secret: "s3cret"` and `pin: "1234"` is created. Its
answer and its creation log show `"[redacted]"` and never the values.

## 4. 10 bots, 1 minute (US1, US2, SC-001, SC-003, SC-004)

```bash
export JANUS_URL=ws://localhost:8188 JANUS_ROOM=1011 JANUS_SERVER_CONTAINERS=janus
JANUS_PARTICIPANTS=10 JANUS_MEDIA=audio_video JANUS_HOLD=60s \
  go test -tags live -count=1 -v -run Janus ./tests/live/
```

Expected:
- PASS, with 10 bots established;
- each bot receives 9 audio and 9 video streams, plus the dummy's idle
  section, which yields no track;
- all bots left within 10 s of the end;
- the summary has no freezes, audio gaps below 1%, and a median audio MOS
  of at least 4.0;
- the server section reports `janus` with CPU and memory;
- one pair of files `reports/<time>-janus-live-janus-<run>.{json,md}`.

During the hold, open `http://localhost:8090/demos/videoroom.html?room=1011`: the
page shows the 10 bots' videos.

Compare the JSON's field set with a mediasoup report of the same plan, as
in 010 quickstart step 3. There must be no difference other than array
indexes and the `platform_config` keys (SC-004).

## 5. Latency (FR-013, research R8)

In the step 4 summary, latency is either:
- present, with p50 values of a few ms as on the other platforms;
- or absent, with a `latency_reason`.

Record which case applies in `validation-report.md`.

## 6. Receive cap and replacement (FR-006)

```bash
JANUS_PARTICIPANTS=6 JANUS_MAX_RECEIVED=3 JANUS_LEAVE_ONE=1 JANUS_HOLD=60s \
  go test -tags live -count=1 -v -run Janus ./tests/live/
```

Expected:
- each bot receives at most 3 audio and 3 video streams;
- when the extra bot leaves halfway through, the bots that received it get
  a waiting stream within a few seconds (one `update` and one
  renegotiation per bot in the log);
- the test passes.

## 7. Receive-only and session lifetime (US1 scenario 4, FR-007)

- `JANUS_MEDIA=receive_only JANUS_PARTICIPANTS=1`, alone in a new room: the
  bot is established through the dummy subscription (research R3) and
  passes.
- `JANUS_PARTICIPANTS=2 JANUS_HOLD=3m`: hold longer than Janus's 60 s
  session timeout. No `timeout` event appears, and both bots pass.

## 8. 50 bots, alternated with the other three servers (US3 scenario 4, SC-002, research R12)

Run 010 T029's plan in the same machine state:
- 50 bots `audio_video`, cap 10, `JANUS_RAMP_UP=10`, `JANUS_HOLD=10m`;
- order: Jitsi, LiveKit, mediasoup, Janus, then again in the same order;
- each server alone, started just before its run;
- screen unlocked (`loginctl show-session $XDG_SESSION_ID -p LockedHint`).

Expected:
- every Janus run keeps 50/50 bots established for the whole hold;
- the T029 table in `validation-report.md` gains a Janus column (MOS, audio
  concealed, frame rate, freezes, latency, server CPU and memory, engine
  CPU).

## 9. Nothing else changed (FR-011)

```bash
go test -race ./...
```

Expected:
- PASS;
- the Jitsi, LiveKit and mediasoup catalog and service tests are
  unchanged;
- a 2-bot mediasoup live run still passes.

## Stop

```bash
cd tests/platforms/janus && docker compose down && cd -
```
