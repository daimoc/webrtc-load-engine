# Data Model: Network Conditions per Bot (016)

## Test plan side (`src/scenario/network.go`)

**NetworkConfig**: `seed *uint64`, `settle_seconds int` (default 10, 0–300), `groups []BotGroup` (1–16).

**BotGroup**: `name string` (unique, `[a-z0-9_-]{1,32}`), exactly one of `count int` (≥ 1) or `share float64` (0 < share ≤ 1), and `schedule []ScheduleStep` (1–32, ordered, not overlapping).

**ScheduleStep**: `start_seconds int` (≥ 0), `duration_seconds *int` (≥ 1; nil: until the end of the run), `downlink *LinkCondition`, `uplink *LinkCondition` (at least one).

**LinkCondition**: `rate_kbps int` (16–1 000 000, 0: no cap), `queue_ms int` (10–2000, default 200, only with a rate), `delay_ms int` (0–5000), `jitter_ms int` (0..delay_ms), `loss *Loss`. At least one field is set.

**Loss**: `share float64` (0–1), `mean_burst float64` (≥ 1, default 1).

The full validation rules are in contracts/test-plan-network.md. `NewScenario` resolves the groups to bot counts and refuses invalid plans.

## Engine side (`src/execution`)

**shaper** (one per constrained bot): two `linkShaper`s (down, up), each nil when no step of the group uses that direction. It holds the step in force (set by the schedule timer), and per-step counters.

**linkShaper**: a token bucket (rate, burst = one MTU), a byte-bounded FIFO, the delay/jitter draw, the Gilbert-Elliott state, a PCG generator, a delivery goroutine and timer. **States**: `idle` (no condition in force: datagrams pass straight through), `shaping` (condition in force), `closed`. A step change swaps the condition. Datagrams already queued keep their delivery time.

**stepCounters**: offered/delivered packets and bytes, dropped by cap, dropped by loss, burst count, delay sum and maximum, delivered bytes per second (for the 10-second windows).

**senderAdapter** (constrained senders only): Pion `SendSideBWE`, the list of layers with content bitrates, the sent set, the drop timer and the restore backoff (2 s, then 4 s, then 8 s at most). **Transitions**: `all_layers` → `dropping` (estimate below the total for 1 s) → `fewer_layers` → `trying` (restore attempt; waits for the next keyframe) → `fewer_layers` (failed within 3 s, backoff doubles) or `all_layers`/`fewer_layers+1` (success, backoff reset). It records layer changes and the estimate once per second.

**videoTrack** (existing): gains an enable flag per layer (simulcast) or a maximum spatial id (SVC), and its own outgoing sequence numbers, so dropped packets leave no gap.

**botTimeline** (media collector): per received video stream, samples `{at, height, froze}` at each collection, with the stream's sender (bot id) where known.

## Report side (`src/scenario/result.go`, `src/report`)

**NetworkResult** in `RunResult.Network` (nil without `network`): seed and its source, settle time, resolution, `[]GroupResult`, `UnconstrainedQuality`, `[]BotNetwork`.

**GroupResult**: name, bot ids, `[]AppliedStep` (planned step, actual start and end, `during_start_up`), `Quality *QualitySummary`.

**BotNetwork**: bot, group, requested height, `[]BotStep` (direction measures, missing after retransmission, reaction, recovery, recovered, reason), `Sender` (adapts, reason, layer changes, estimate samples), `ServerEstimate` (reason or samples).

`report.FormatVersion` becomes 4. The shapes are in contracts/report-network.md.

## Platform side (`src/platform`)

**ReceiverEstimator** (optional, implemented by an adapter): `ReceiverEstimate(ctx, botID) (kbps int, ok bool, err error)`. Implemented by mediasoup (`getTransportStats` on the receive transport).
