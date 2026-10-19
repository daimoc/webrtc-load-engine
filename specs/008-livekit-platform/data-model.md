# Data Model: LiveKit Platform (008)

## LiveKit Config (`livekit.Config`, from `platform_config`)

| Field | Type | Rules |
|-------|------|-------|
| `URL` | string | required; scheme ws/wss/http/https; http(s) mapped to ws(s); `/rtc` appended |
| `Room` | string | required, non-empty |
| `APIKey` | string | with `APISecret`, or neither |
| `APISecret` | string | secret: never logged, redacted in answers |
| `Token` | string | secret; exclusive with key/secret |
| `TokenTTL` | duration | default 6 h; > 0; only with key/secret |

Validation messages: [contracts/livekit-config.yaml](contracts/livekit-config.yaml). Unknown keys are rejected.

## Access Token Claims (`livekit.claims`, research R3)

`iss` = APIKey, `sub` = peer ID, `name` = peer ID, `jti` = peer ID, `nbf` = now − 10 s, `exp` = now + TokenTTL, `video` = `{room: Room, roomJoin: true, canPublish: true, canSubscribe: true, canPublishData: false}`. Signed HS256 with APISecret.

## Bot Session (`livekit.session`, one per connected peer)

| Field | Meaning |
|-------|---------|
| `peerID` | the bot's identity |
| `conn` | WebSocket to `/rtc` |
| `handler` | the peer (`platform.SignalingHandler`) |
| `ownSID` | own participant SID from `JoinResponse` |
| `pingInterval`, `pingTimeout` | from `JoinResponse` (defaults 5 s / 15 s) |
| `lastHeard` | time of the last frame read (liveness) |
| `subAnswered` | first subscriber offer answered (`OnOffer` used; later ones `Renegotiate`) |
| `pendingCIDs` | local track IDs waiting for `TrackPublishedResponse` |
| `publishAnswer` | signalled when the publish answer is applied |
| `subs` | receive-cap state (below) |
| `leaving` | set by `Disconnect`; a close after it is not a loss |

**States** (internal; mapped to the peer lifecycle as in research R9):

`dialing → joined (JoinResponse) → publishing (AddTrack sent, offer pending) → ready (publish answer applied and first subscriber offer answered; Connect returns) → leaving → closed`

- A receive-only bot goes from `joined` to `ready` on the first subscriber answer.
- Failure before `ready`: `Connect` returns the error with its stage, and everything is closed.
- Failure after `ready`: `OnSignalingLost`.

## Receive-Cap State (`livekit.subscriptions`, research R7)

Per kind (`audio`, `video`):

| Field | Meaning |
|-------|---------|
| `known` | remote track SIDs in the order first seen, with their participant SID |
| `subscribed` | set of track SIDs subscribed, at most `limit` (0 = all) |
| `limit` | `handler.ReceiveLimit()` |

Operations:
- **Track seen**: add it to `known`, then fill.
- **Track gone** (participant DISCONNECTED, track missing from its participant's update, subscription error): remove it from `known` and `subscribed`, then fill.
- **Fill**: while fewer than `limit` are subscribed, subscribe the earliest known track that isn't subscribed yet. One `UpdateSubscription` per fill, carrying every SID added.

Own tracks (participant SID = `ownSID`) are never added.

## Peer Publish Session (`execution.Peer`, research R4)

New fields:

| Field | Meaning |
|-------|---------|
| `publishSession` | bool, from `PeerMedia.PublishSession` |
| `pub` | second `*webrtc.PeerConnection`; nil without publish session |
| `pubState` | its last connection state |
| `pubAnswered` | its answer applied; remote candidates buffered until then |
| `pubPending` | buffered remote candidates |

Lifecycle: `established` when the answered session is connected and (if `pub != nil`) `pub` is connected; `failed` on either failing. No new `PeerState`.

## Platform Catalog Entry (`catalog.Entry`, research R10)

| Field | Meaning |
|-------|---------|
| `Name` | `jitsi`, `livekit` |
| `Validate(TestPlan) error` | creation-time check; Jitsi: none (config checked at execution, as today); LiveKit: config + simulcast/requested heights rejected |
| `SecretKeys` | Jitsi: `jwt_app_secret`, `jwt`; LiveKit: `api_secret`, `token` |
| `NewAdapter(logger, platform_config)` | builds the adapter for a run |

## Quality Summary Change

`KindQuality.LatencyReason string` (`latency_reason`, omitempty), with the values in [contracts/quality-summary-delta.yaml](contracts/quality-summary-delta.yaml).
