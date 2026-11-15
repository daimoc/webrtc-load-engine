# Quickstart: Network Conditions per Bot (016)

How to check the feature end to end. Shapes: contracts/test-plan-network.md and contracts/report-network.md.

## Prerequisites

- Go at `/usr/local/go/bin`. One local server started from `tests/platforms/<server>` (README of each). Stop the other servers.
- mediasoup: use `ramp_up_seconds` 20 for 10 bots (015 probe bug). Jitsi: make one warm-up run first after the bridge starts.

## 1. The shaper alone (SC-001, SC-005)

```bash
go test -race -run 'Shaper' ./src/execution/
```

Expected: the rate, delay and loss tests pass within SC-001's limits. The same seed gives the same dropped share within 0.5 point.

## 2. A receiver with a slow link (US1, SC-002, SC-003)

`slow.json`:

```json
{ "groups": [ { "name": "slow", "count": 3,
  "schedule": [ { "start_seconds": 30, "downlink": { "rate_kbps": 1200 } } ] } ] }
```

```bash
JANUS_URL=ws://localhost:8188 JANUS_ROOM=1016 JANUS_PARTICIPANTS=10 JANUS_RAMP_UP=10 JANUS_MEDIA=audio_video \
JANUS_SIMULCAST=720 JANUS_REQUESTED_HEIGHTS=720:1 JANUS_MAX_RECEIVED=4 JANUS_HOLD=60s \
JANUS_NETWORK=$PWD/slow.json \
  go test -tags live -count=1 -v -run JanusLive ./tests/live/
```

Give `<PLATFORM>_NETWORK` an absolute path: the live tests run in `tests/live`. With a network section, the live tests check the bots in no group only, and report the constrained bots' freezes without failing on them. The plans used for the validation are in `runs/plans/`, and `runs/` holds their reports.

Expected in `reports/…-janus-….md`:
- "Network conditions" names the 3 bots of `slow`, with about 1200 kbit/s delivered and a mean added delay up to the 200 ms queue;
- the `slow` bots' height share is mostly below 720;
- the unconstrained row keeps 720 at least 95% of the time.

Repeat with `LIVEKIT_*`, `MEDIASOUP_*` (`MEDIASOUP_RAMP_UP=20`, 015) and `JITSI_*` (the Jitsi live test; one warm-up run first), or `go run ./cmd/jitsi-connect … --network slow.json`. Expected on Janus: the capped bots stay at 720 and freeze (no downlink estimation). On the others, they receive lower layers.

## 3. A link that changes (US2, SC-004)

```json
{ "groups": [ { "name": "slow", "count": 3, "schedule": [
  { "start_seconds": 30, "duration_seconds": 20, "downlink": { "rate_kbps": 1200 } } ] } ] }
```

Hold 80 s. Expected: per `slow` bot, a reaction time and a recovery time, or "not recovered", in both files.

## 4. Delay, jitter and loss (US3)

```json
{ "seed": 7, "groups": [ { "name": "lossy", "count": 3, "schedule": [
  { "start_seconds": 30, "downlink": { "delay_ms": 40, "jitter_ms": 10, "loss": { "share": 0.03 } } } ] } ] }
```

Expected:
- the dropped share is about 0.03;
- the share missing after retransmission is smaller;
- the `lossy` bots' video latency is about 40 ms higher than the others'.

Run it twice with seed 7: the dropped shares agree within 0.5 point.

## 5. A sender with a slow link (US4, SC-008, SC-009)

```json
{ "groups": [ { "name": "slowup", "count": 2, "schedule": [
  { "start_seconds": 30, "duration_seconds": 30, "uplink": { "rate_kbps": 400 } } ] } ] }
```

Expected:
- `sender.adapts` is true for the 2 bots, with the top layers dropped within 5 s of 30 s and all layers back within 10 s of 60 s;
- or `no_feedback` on a server that sends no transport-wide feedback to publishers;
- receivers of their streams show lower heights during the step.

Run the same plan without `network`: no bot's sender section, and `go test` of the engine shows no layer dropped.

## 6. CPU (SC-006)

At 100 bots, VP8, cap 10, 60 s, compare the engine CPU per bot of this build and of the commit before 016: within 5%. Then run with 10 bots constrained both ways and record the difference.

## 7. Comparison (US5, SC-007)

Plans 2 and 3 on the four servers, three runs each, plus plans 4 and 5 once. The tables are gathered in `specs/016-network-conditions/validation-report.md`.
