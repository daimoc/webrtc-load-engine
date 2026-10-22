# Data Model: Janus Platform (011)

Everything is in memory, inside `src/platform/janus`. Nothing new outside it
except the catalog entry.

## Config (`janus.Config`)

The `platform_config` of a `janus` test plan
([contracts/janus-config.yaml](contracts/janus-config.yaml)).

| Field | Type | Rule |
|---|---|---|
| URL | string | required; `ws://` or `wss://` (`http(s)://` turned into `ws(s)://`), no query |
| Room | uint64 | required; > 0; from a JSON number or a string of digits |
| InsecureTLS | bool | default false |
| APISecret | string | optional, write-only (core `apisecret`) |
| Token | string | optional, write-only (core `token`) |
| PIN | string | optional, write-only (VideoRoom `pin`) |
| AdminKey | string | optional, write-only (VideoRoom `admin_key` on room `create`) |
| Participants | int | not in `platform_config`: the test plan's bot count, used to size the room (R4) |

- An unknown key is rejected:
  `platform_config: unknown key %q for platform janus`.
- `String` and `LogValue` show the secrets only as `has_<name>: true|false`.
- The catalog `SecretKeys` are `api_secret`, `token`, `pin` and `admin_key`.

## Bot session (`adapter.session`, one per connected bot)

| Field | Meaning |
|---|---|
| peerID | the engine's peer ID, sent as `display` |
| conn | the bot's WebSocket (`janus-protocol`) |
| client | the JSON client: pending transactions by ID, read loop, `sender` routing |
| sessionID | Janus session |
| pubHandle, subHandle | the publisher and subscriber plugin handles |
| participantID, privateID | from `joined` |
| dummyFeed | the room's dummy publisher ID (R3) |
| cap | the receive cap state (below) |
| work | channel to the bot's single goroutine for subscription updates and their `start` answers (R6) |
| keepalive | ticker, 25 s (R7) |
| handler | the engine's `SignalingHandler` |
| lost | `sync.Once` around `OnSignalingLost` |

The adapter keeps `map[peerID]*session` behind a mutex; `Disconnect`
removes the entry.

## Receive cap (`streams.go`), per kind

| Field | Meaning |
|---|---|
| limit | `handler.ReceiveLimit()` (0: no limit) |
| active | `[]streamKey` subscribed, in subscription order |
| waiting | `[]streamKey` announced but not subscribed, FIFO |

- `streamKey` is `{feed uint64, mid string}`.
- The publishers' `display` values are kept for logs.

Transitions, each returning the `update` to send (`subscribe`,
`unsubscribe` lists; empty means no request):

| Event | Effect |
|---|---|
| publisher announced (`joined`, `event.publishers`) | each non-disabled audio/video stream: active if `len(active) < limit` or no limit, else appended to waiting; dummy and `data` streams ignored |
| feed gone (`unpublished`, `leaving`) | its waiting streams removed; its active streams unsubscribed, and for each one the earliest waiting stream of that kind subscribed |
| announced again (same feed and mid) | ignored (no duplicate m-line: Janus would add one) |

## Peer lifecycle mapping (no new state)

| Janus | Lifecycle |
|---|---|
| R4 steps 1–6 succeed | `Connect` returns, peer `connected` |
| subscriber and publish transports connected (the subscriber alone for `receive_only`) | `established` (engine rule, unchanged) |
| error during R4 | `Connect` fails: `janus: <request>: <code> <reason>` |
| WebSocket closed, `timeout`, `hangup`/`detached` on a handle, `kicked`, room `destroyed` | `OnSignalingLost(reason)`, peer `failed` at stage `signaling_lost` |
| `Disconnect` | `destroy` sent, socket closed, peer `disconnected` (engine) |
