# LiveKit signaling messages used by the adapter (research R1, R2)

Field numbers are from `livekit/protocol` v1.52.1 (`protobufs/livekit_rtc.proto`,
`protobufs/livekit_models.proto`). The adapter encodes only the "sent" fields,
decodes only the "read" fields, and skips everything else. Wire types:
`string`/`bytes`/message = length-delimited, `bool`/`int32`/`int64`/`uint32`/enum = varint.

## Sent: `SignalRequest` (oneof `message`)

| # | Field | Type | Used for |
|---|-------|------|----------|
| 1 | `offer` | SessionDescription | publish session offer (R5) |
| 2 | `answer` | SessionDescription | subscribe session answer (R6) |
| 3 | `trickle` | TrickleRequest | local candidates, both sessions |
| 4 | `add_track` | AddTrackRequest | announce each local track before the offer |
| 6 | `subscription` | UpdateSubscription | receive cap (R7) |
| 8 | `leave` | LeaveRequest | `Disconnect` (R8) |
| 16 | `ping_req` | Ping | keepalive (R8) |

- `SessionDescription`: 1 `type` ("offer"/"answer"), 2 `sdp`, 3 `id` (uint32; the answer echoes the offer's).
- `TrickleRequest`: 1 `candidateInit` (JSON `{"candidate","sdpMid","sdpMLineIndex"}`), 2 `target` (PUBLISHER = 0, SUBSCRIBER = 1).
- `AddTrackRequest`: 1 `cid`, 2 `name`, 3 `type` (AUDIO = 0, VIDEO = 1), 4 `width`, 5 `height`, 8 `source` (CAMERA = 1, MICROPHONE = 2), 13 `disable_red` (true for audio).
- `UpdateSubscription`: 1 `track_sids` (repeated), 2 `subscribe`.
- `LeaveRequest`: 2 `reason` (CLIENT_INITIATED = 1).
- `Ping`: 1 `timestamp` (ms since epoch), 2 `rtt` (ms, last measured; 0 at first).

## Read: `SignalResponse` (oneof `message`)

| # | Field | Type | Handling |
|---|-------|------|----------|
| 1 | `join` | JoinResponse | join complete; start pings; known tracks |
| 2 | `answer` | SessionDescription | `ApplyPublishAnswer` |
| 3 | `offer` | SessionDescription | `OnOffer` (first) / `Renegotiate` (later) |
| 4 | `trickle` | TrickleRequest | `OnPublishICECandidate` (target 0) / `OnICECandidate` (target 1) |
| 5 | `update` | ParticipantUpdate | known tracks, departures |
| 6 | `track_published` | TrackPublishedResponse | publish step 4 (R5) |
| 8 | `leave` | LeaveRequest | signaling lost (reason logged) |
| 17 | `track_unpublished` | TrackUnpublishedResponse | not used (own tracks only); skipped |
| 18 | `pong` | int64 | liveness |
| 20 | `pong_resp` | Pong | liveness, round trip |
| 21 | `subscription_response` | SubscriptionResponse | drop the track, log once |

Any other field, including `speakers_changed`, `connection_quality`, `refresh_token`, `stream_state_update` and `room_update`, counts only as liveness and is skipped.

- `JoinResponse`: 2 `participant` (ParticipantInfo, own), 3 `other_participants` (repeated), 6 `subscriber_primary`, 7 `alternative_url` (non-empty fails the join), 10 `ping_timeout` (s), 11 `ping_interval` (s), 12 `server_info` (ServerInfo: 2 `version`, 3 `protocol`; logged once).
- `ParticipantInfo`: 1 `sid`, 2 `identity`, 3 `state` (DISCONNECTED = 3), 4 `tracks` (repeated TrackInfo).
- `TrackInfo`: 1 `sid`, 2 `type`, 4 `muted`, 9 `source`.
- `TrackPublishedResponse`: 1 `cid`, 2 `track` (TrackInfo).
- `LeaveRequest`: 1 `can_reconnect`, 2 `reason`, 3 `action`.
- `Pong`: 1 `last_ping_timestamp`, 2 `timestamp`.
- `SubscriptionResponse`: 1 `track_sid`, 2 `err` (enum, logged).

`TrackUnpublishedResponse` (17) concerns the bot's own tracks, so the adapter does not need it. Remote tracks that end are seen through `ParticipantUpdate`.
