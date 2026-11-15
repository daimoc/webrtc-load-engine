# Contract: `network` in the run report (016)

Report FormatVersion 4. `network` is absent when the test plan has none.

```json
"network": {
  "seed": 42,
  "seed_source": "test_plan",                 // or "drawn"
  "settle_seconds": 10,
  "timeline_resolution_seconds": 2,       // the media collection interval
  "groups": [
    {
      "name": "slow",
      "bots": ["3f2a...", "..."],
      "steps": [
        {
          "index": 0,
          "planned": { "start_seconds": 20, "duration_seconds": 20, "downlink": { "rate_kbps": 300 } },
          "started_at_seconds": 20.0, "ended_at_seconds": 40.0,
          "during_start_up": false               // FR-011
        }
      ],
      "quality": { }                             // QualitySummary over the group's bots (FR-007)
    }
  ],
  "unconstrained_quality": { },                  // QualitySummary over bots in no group
  "bots": [
    {
      "bot": "3f2a...",
      "group": "slow",
      "requested_height": 720,
      "steps": [
        {
          "index": 0,
          "downlink": {                          // measured (FR-006); absent if the step has no downlink
            "offered_packets": 52011, "delivered_packets": 50112,
            "delivered_kbps": { "mean": 297, "max_10s": 301 },
            "dropped_by_cap_share": 0.031, "dropped_by_loss_share": 0.0, "mean_burst": null,
            "added_delay_ms": { "mean": 152.3, "max": 201.0 }
          },
          "uplink": null,
          "missing_after_retransmission_share": 0.004,   // FR-009, video received
          "reaction_seconds": 2,                   // null + reason when no stream was at its requested height
          "recovery_seconds": 6,                   // null when not recovered
          "recovered": true,
          "reason": null                           // e.g. "not_at_requested_height", "no_video", "not_recovered_before_end"
        }
      ],
      "sender": {                                // FR-016/017
        "adapts": true,                          // false: reason says why
        "reason": null,                          // "no_uplink_condition", "cannot_adapt" (single stream), "no_feedback", "receive_only"
        "layers": [ { "at_seconds": 0, "heights": [180, 360, 720] }, { "at_seconds": 23, "heights": [180, 360] } ],
        "estimate_kbps": [ { "at_seconds": 0, "kbps": 2150 } ]    // one sample per second while adapting
      },
      "server_estimate": {                       // FR-010
        "reason": "not_given_by_server",         // or null with samples
        "kbps": [ ]
      }
    }
  ]
}
```

## Rules

- Measures are never copied from the test plan: a step that applied nothing (no bot of the group connected during it) gives `"reason": "not_applied"` for that bot.
- A share with no denominator is `null` with a reason, never 0 (as 015).
- `uplink` steps give, under each receiving bot, results for the streams of the constrained senders apart: `constrained_senders` (list of bot ids) and that bot's `reaction_seconds`/`recovery_seconds` for those streams.
- Markdown: a "Network conditions" section. It has a table of groups and steps (planned vs measured rate, drops, delay), a table per step of reaction and recovery per bot (median and maximum per group), the senders' layers over time as a compact list, and the group quality rows beside the unconstrained rows.
