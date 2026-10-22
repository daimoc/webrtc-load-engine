# How the Janus adapter drives the 008 boundary (no change)

`src/platform/adapter.go` is used as it is. `janus.Adapter` implements
`PlatformAdapter` and `PublishSessionAdapter` (`PublishSession() == true`).

| Boundary call | Janus |
|---|---|
| `LocalMediaKinds()` | empty: skip `publish` (step 4) and build no publish session |
| `CreatePublishOffer(trickle)` | jsep offer of `publish`; the `trickle` func sends core `trickle` on the publisher handle |
| `ApplyPublishAnswer(sdp)` | jsep answer of the `configured` event |
| `OnPublishICECandidate` | only if Janus trickles (full_trickle); normally unused |
| `OnOffer(sdp, trickle)` | jsep offer of `attached`; answer sent with `start`; `trickle` sends on the subscriber handle |
| `Renegotiate(sdp)` | jsep offer of each `updated`; answer sent with `start` |
| `OnICECandidate` | only if Janus trickles; normally unused |
| `ReceiveLimit()` | per-kind cap of the subscription (R6) |
| `RequestedVideoHeight()`, `VideoLayerHeights()` | not used: rejected at creation (FR-010) |
| `OpenDataChannel` | not used |
| `OnSignalingLost(err)` | R7 losses |

Ordering per bot:
- `OnOffer` and every `Renegotiate` run on the bot's single subscription
  goroutine;
- no `update` is sent while an offer is unanswered (R6);
- `CreatePublishOffer` and `ApplyPublishAnswer` run in `Connect`, before
  the subscriber handle is attached.
