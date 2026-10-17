# Quickstart: Server Load in the Run Report (007)

Validation guide. Contracts: [test-plan-server.yaml](contracts/test-plan-server.yaml),
[server-load-summary.yaml](contracts/server-load-summary.yaml),
[metrics.md](contracts/metrics.md).

## Prerequisites

- The local docker-jitsi-meet from `tests/platforms/docker-jitsi-meet` up
  (`docker compose up -d`), the bridge restarted before scale runs (006 T058).
- The user running the engine can use Docker (`docker ps` works without
  sudo).
- `export PATH=/usr/local/go/bin:$PATH`.

Common live variables (as 006):

```bash
export JITSI_ROOM_URL=http://localhost:8000/load007 JITSI_XMPP_DOMAIN=meet.jitsi \
  JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket
export JITSI_SERVER_CONTAINERS=docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1
```

## 1. Unit and fake-Docker tests

```bash
go vet ./... && go test -race ./...
go test -race -v ./src/hostload/
```

Expected: arithmetic on recorded cgroup v1/v2 bodies, missing, stopped,
slow and restarted containers, and the unreachable socket all pass.

## 2. Server section against `docker stats` (US1, SC-001)

```bash
go test -c -tags live -o live.test ./tests/live/
( while sleep 5; do docker stats --no-stream --format '{{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}' \
    docker-jitsi-meet-jvb-1; done ) > stats.tsv &
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_HOLD=60s ./live.test -test.v
kill %1
```

Expected: the logged summary has `server.available: true`, both containers
`measured`, `missed` 0; the bridge's `cpu_threads.average` within 10% of
the mean of `stats.tsv`'s CPU column / 100 over the hold, and
`memory_bytes.peak` within 5% of its highest memory reading.

## 3. Engine section against `/usr/bin/time` (US2, SC-002)

```bash
JITSI_MEDIA=audio_video JITSI_PARTICIPANTS=10 JITSI_HOLD=60s /usr/bin/time -v ./live.test -test.v
```

Expected: `load_machine.engine.cpu_threads.average` within 5% of
(user + system) / elapsed, the run's start-up and shutdown excepted (compare
over the summary's `period`); `hardware_threads` 16 on the reference
laptop, `usable_threads` as `nproc`.

## 4. Failures do not touch the run (SC-003)

```bash
JITSI_SERVER_CONTAINERS=docker-jitsi-meet-jvb-1,no-such-container JITSI_PARTICIPANTS=2 ./live.test -test.v
DOCKER_HOST=unix:///nonexistent.sock JITSI_PARTICIPANTS=2 ./live.test -test.v
```

Expected: both runs pass and take the same time as without monitoring; the
first reports `no-such-container` `unavailable`/`not_found` and the bridge
`measured`; the second `server.available: false`, reason
`docker_unreachable`; one WARN log line per container.

## 5. Cost at scale (SC-004)

As 006 quickstart step 4 (100 bots, `audio_video`, cap 10, ramp-up 20 s,
10 min, `/usr/bin/time -v`), twice on the same day, the bridge restarted
before each: once with `JITSI_SERVER_CONTAINERS` set to three containers
(jvb, jicofo, prosody), once unset.

Expected: at most 0.05 core more with monitoring. Record both figures, the
bridge's average and peak CPU from the summary, and the engine's figures in
tasks.md and the validation report.

## 6. API

`go test -race -run ServerLoadThroughAPI -v ./tests/integration/execution/`
does this in process (the API server's fixed `:8080` is taken by the local
bridge's Colibri port while docker-jitsi-meet runs). By hand, with the
bridge stopped: create a scenario with `server_monitoring`, run it, then
`GET /api/v1/scenarios/{id}/quality`: the `server` and `load_machine`
sections are present; a test plan with 17 containers, a duplicate or
`interval_seconds: 61` is rejected with 400 (0 means the 5 s default).
