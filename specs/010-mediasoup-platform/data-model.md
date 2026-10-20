# Data Model: mediasoup-demo Platform

All state is in memory, per scenario run. Nothing is added to the
scenario, quality summary or report models: they are reused as they are
(FR-010).

## Config (`mediasoup.Config`)

The `platform_config` of a `mediasoup` test plan
([contracts/mediasoup-config.yaml](contracts/mediasoup-config.yaml)).

| Field | Type | Rule |
|---|---|---|
| URL | string | required. `ws://`, `wss://`, `http://` or `https://`; http(s) becomes ws(s); no query |
| Room | string | required, non-empty; becomes `roomId` |
| Origin | string | optional; `http://` or `https://` URL; default `http(s)://<url host>` from the URL's scheme |
| InsecureTLS | bool | default false; only meaningful with `wss://` |
| UnlimitedRoomSecret | string | optional; secret (never logged, redacted in answers and reports) |
| UsePipeTransports | bool | default false |

- Validated at creation by `Validate(TestPlan)`: the config, plus no
  `media.simulcast` and no `media.requested_heights`.
- Unknown keys are rejected.
- `LogValue` and `String` show `has_unlimited_room_secret` instead of the
  value.

## Bot session (`adapter.session`, one per connected bot)

| Field | Meaning |
|---|---|
| peerID | the bot's peer ID; `peerId` and `displayName` |
| conn | the protoo client (WebSocket, pending requests by ID, handlers) |
| routerCaps | the router's RTP capabilities (`getRouterRtpCapabilities`) |
| recvCaps | the bot's RTP capabilities sent in `join` (R5) |
| recvTransport | ID, ICE parameters, UDP candidates, DTLS parameters, SCTP parameters |
| sendTransport | the same, for sending bots only |
| recvSections | the receive offer's sections, in order: section 0 = application, then one per consumer ever accepted |
| producers | producer ID by kind |
| cap | the receive cap state (below) |
| sdpQueue | one goroutine running the bot's SDP work in order |

### Receive section

| Field | Meaning |
|---|---|
| mid | the consumer's mid (`rtpParameters.mid`, `"0"`, `"1"`… from the server); `"datachannel"` for the application section; never reused |
| kind | audio, video or application |
| consumerID | the consumer it carries |
| rtpParameters | the consumer's codecs, extensions, encodings and RTCP |
| active | true: `sendonly` with SSRC lines; false: `inactive`, no SSRC lines |

## Receive cap (`consumers.go`), per kind

| Field | Meaning |
|---|---|
| limit | `ReceiveLimit()`; 0 = no limit |
| active | consumer IDs with an active section |
| waiting | FIFO of declined consumers: ID, producer ID, peer ID, rtpParameters, arrival time |

State of one consumer, from the bot's side:

```text
            newConsumer, slot free            consumerClosed
 (offered) ───────────────────────▶ active ───────────────────▶ closed (section inactive)
     │                                ▲
     │ newConsumer, cap reached       │ slot freed, earliest waiting:
     ▼                                │ section added + resumeConsumer
  waiting ────────────────────────────┘
     │ consumerClosed
     ▼
  dropped
```

Invariants:

- `len(active) ≤ limit` per kind, when `limit > 0`.
- A waiting consumer is never in the receive offer.
- An active consumer's section is in the bot's current remote description
  before the server resumes it: the accept is sent, or `resumeConsumer` is
  notified, only after `Renegotiate` returned.
- When a slot frees, the earliest waiting consumer of that kind is
  promoted first.

## Peer lifecycle mapping (no new state)

| mediasoup event | Peer effect |
|---|---|
| join sequence complete (R4) | `Connect` returns → `connected` |
| receiving (and sending, if any) transport connected | `established` (existing rule) |
| request rejected / WebSocket refused during join | `Connect` error → `failed` (signaling stage), with method and reason |
| WebSocket closed after join (server gone, room time limit, peer replaced) | `OnSignalingLost` |
| `Disconnect` | WebSocket closed; server closes the bot's transports |
