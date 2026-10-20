# Use of the platform boundary (no change; research R2, R3, R6)

The adapter implements `platform.PlatformAdapter` and
`platform.PublishSessionAdapter` (`PublishSession() == true`), as LiveKit's
does (specs/008-livekit-platform/contracts/platform-boundary.md). No
method is added to `platform.SignalingHandler`, and `execution.Peer` is not
changed.

| `SignalingHandler` method | Called by the mediasoup adapter |
|---|---|
| `LocalMediaKinds` | before join: decides whether a sending transport and producers are made |
| `ReceiveLimit` | the cap per kind (R6) |
| `OnOffer(offer, trickle)` | once, with the data-channel-only receive offer (R3); `trickle` drops candidates (ICE-lite) |
| `Renegotiate(offer)` | each time a consumer section is added or set inactive; the answer's DTLS fields are unchanged and not resent |
| `CreatePublishOffer(trickle)` | once per sending bot, after `join`; `trickle` drops candidates |
| `ApplyPublishAnswer(answer)` | after every `produce` succeeded |
| `OnSignalingLost(err)` | WebSocket closed or failed after `Connect` returned |
| `OnICECandidate`, `OnPublishICECandidate` | never (all of mediasoup's candidates are in the SDP it builds) |
| `OpenDataChannel` | never |
| `RequestedVideoHeight`, `VideoLayerHeights` | never (rejected at creation, FR-008) |
| `LocalVideoSize` | never (mediasoup needs no announced size) |

All calls that change a session's description are made from the bot's SDP
goroutine, one at a time. `Disconnect` stops that goroutine before it
closes the WebSocket, so no SDP call runs after `Disconnect` returns.
