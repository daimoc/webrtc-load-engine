# Contract: `network` in a test plan (016)

Optional, platform-independent. Absent: no condition, and the run behaves as before 016.

```json
"network": {
  "seed": 42,                       // optional; absent: drawn by the engine and reported
  "settle_seconds": 10,             // optional, default 10, 0–300
  "groups": [
    {
      "name": "slow",               // required, unique, [a-z0-9_-]{1,32}
      "count": 3,                   // exactly one of count (1..participants) or share (0 < share ≤ 1)
      "schedule": [                 // 1–32 steps, ordered, not overlapping
        {
          "start_seconds": 20,      // ≥ 0, < duration of the run (ramp-up + duration_seconds)
          "duration_seconds": 20,   // optional, ≥ 1; absent: until the end of the run
          "downlink": {             // server → bot; optional
            "rate_kbps": 300,       // optional, 16–1000000
            "queue_ms": 200,        // optional, 10–2000, default 200; only with rate_kbps
            "delay_ms": 40,         // optional, 0–5000
            "jitter_ms": 10,        // optional, 0..delay_ms
            "loss": { "share": 0.03, "mean_burst": 1 }   // optional; share 0–1, mean_burst ≥ 1 (1: random)
          },
          "uplink": { }             // bot → server; same fields; optional
        }
      ]
    }
  ]
}
```

## Rules (refused at creation, message names the field)

- At least one group; group names unique; at most 16 groups.
- A step has at least one of `downlink`, `uplink`, and each given direction has at least one field.
- The groups' bots added together do not exceed `participants`. `share` gives `round(share × participants)`, at least 1.
- Steps of a group are ordered by `start_seconds` and do not overlap. A step ends before or at the end of the run (`ramp_up_seconds + duration_seconds`).
- `jitter_ms` ≤ `delay_ms`; `queue_ms` without `rate_kbps` is refused.
- `uplink` on a `receive_only` plan is refused (no sender).
- Unknown fields are refused (strict JSON, as 015).

## Assignment

Bots are assigned from the end of the launch order, the first group first: with 10 bots, `slow` (count 3) gets bots 8, 9 and 10 in launch order. The report lists every bot's group.

## Entry points

- HTTP API: the `test_plan.network` object.
- `cmd/jitsi-connect --network plan.json`: the file holds the `network` object.
- Live tests: `<PLATFORM>_NETWORK=plan.json` (Jitsi, LiveKit, mediasoup, Janus).
