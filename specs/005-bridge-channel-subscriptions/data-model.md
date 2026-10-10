# Data Model: Bridge Channel Subscriptions

In-memory state only. Platform-neutral types live in `src/platform` and
`src/execution`; Jitsi types in `src/platform/jitsi`.

## Receive Preferences (Jitsi adapter, per session)

Derived from `sessionSources` (004), never stored separately from it.

| Field | Type | Source | Rule |
|-------|------|--------|------|
| `capped` | bool | `sessionSources.limit > 0` | selects the message forms of [colibri-messages.md](contracts/colibri-messages.md) |
| `audio` | []string | names of the primary sources of active audio sections, in section order | at most `limit`; empty is valid |
| `video` | []string | same for video | at most `limit` |
| `unnamed` | per kind bool | an active section whose primary source has no `Name` | that kind falls back to the no-cap form (research R3) |
| `maxHeight` | int | constant 360 | 004 R1 (height the bots send) |

Changes when: a source is added to or removed from an active section,
an occupant leaves (`removeOwner`), a waiting source is promoted
(`removeSection`). Not changed by sources that only enter or leave the
waiting list.

## Bridge Channel State (Jitsi adapter, per session: `bridgeChannel`)

| Field | Type | Meaning |
|-------|------|---------|
| `dc` | `platform.DataChannel` | send side; nil when the offer had no SCTP content |
| `open` | bool | between `OnOpen` and `OnClose` |
| `lastSent` | map class → string | last message sent per `colibriClass`, to skip identical resends; cleared on close so that a reopen resends everything |
| `logged` | set of reasons | fallback reasons already logged for this bot (research R8) |
| `openTimer` | `*time.Timer` | logs "not open after 10 s" once; stopped on open or session end |
| `mu` | mutex | serializes sends and state changes |

State transitions:

```text
none ──(offer has SCTP, OpenDataChannel)──▶ opening
opening ──OnOpen──▶ open           (send current preferences)
opening ──10 s──▶ opening          (log once, keep waiting)
open ──preferences change──▶ open  (send messages that differ)
open ──OnClose──▶ closed           (log once; clear lastSent)
closed ──OnOpen──▶ open            (send current preferences)
any ──session terminated / peer gone──▶ done (timer stopped, nothing sent)
none: offer without SCTP → log once, 004 behavior for the whole session
```

## Data channel (execution, per peer)

`execution.dataChannel` wraps Pion's `*webrtc.DataChannel` and implements
`platform.DataChannel`; it records whether it counted itself in
`webrtc_data_channels_open` so that close and peer teardown decrement the
gauge exactly once.

## Transport counter (execution, per peer)

The media collector reads the RTP and RTCP bytes counted by the peer's
read buffers (`gatedBuffer.Write`, before the SSRC gate) and adds the delta since the last collection to
`webrtc_transport_bytes_received_total{scenario_id}`, as it does for the
004 counters (`mediaPeer` gains a `TransportBytesReceived() uint64`
method).
