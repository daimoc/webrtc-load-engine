# Contract: Bridge Channel Messages (Jitsi adapter)

Text messages sent by each bot on the data channel labelled
`JVB data channel` (protocol `http://jitsi.org/protocols/colibri`), opened by
the Jitsi adapter when the session offer contains an SCTP content (003 T056).
Formats checked against the local bridge, `jitsi-videobridge2`
2.3-326-gb92aba83c (research R1, R2). Everything here lives in
`src/platform/jitsi`; no other package sees these formats (FR-010).

## Sent by the bot

### Video constraints (always, FR-001/FR-002)

With a receive cap (`max_received_streams` = N > 0): one entry per received
remote video source (at most N), keyed by source name.

```json
{
  "colibriClass": "ReceiverVideoConstraints",
  "lastN": -1,
  "defaultConstraints": {"maxHeight": 0},
  "constraints": {
    "a1b2c3d4-v0": {"maxHeight": 360},
    "e5f6a7b8-v0": {"maxHeight": 360}
  }
}
```

Without a cap: every video source, current and future.

```json
{"colibriClass": "ReceiverVideoConstraints", "lastN": -1, "defaultConstraints": {"maxHeight": 360}}
```

If any received video source has no name, the no-cap form is sent instead
(research R3).

### Audio subscription (only with a cap, FR-003)

```json
{"colibriClass": "ReceiverAudioSubscription", "mode": "Include", "list": ["a1b2c3d4-a0", "e5f6a7b8-a0"]}
```

The list holds the received remote audio sources (at most N); it may be
empty. Without a cap nothing is sent (the bridge forwards all audio). If any
received audio source has no name, nothing is sent for audio.

### Sending rules (FR-005, FR-006)

| Event | Action |
|-------|--------|
| Channel opens (first time or again) | send both messages for the current state |
| Active sources change (source-add, source-remove, occupant leaves, waiting source promoted) | recompute; send each message that differs from the last one sent |
| Channel closed | store the latest state; send it on the next open |
| Send fails | log once per bot (`Info`); the next change or open retries |

Field order and whitespace are not significant; list order follows the
announcement order of 004 FR-015.

## Received by the bot (FR-009)

Every message is passed to the adapter, which reads only `colibriClass` for a
`Debug` log and otherwise ignores it: `ServerHello`, `ForwardedSources`,
`DominantSpeakerEndpointChangeEvent`, `EndpointConnectivityStatusChangeEvent`,
`SenderSourceConstraints`, `EndpointStats`, `ConnectionStats`,
`VideoSourcesMap`, `AudioSourcesMap`, and any unknown class. Invalid JSON is
ignored the same way. No reply is sent.

## Not acknowledged

The bridge sends no answer to either message (research R5). Whether a
subscription is in effect shows in the metrics:
`webrtc_transport_bytes_received_total` close to
`webrtc_media_bytes_received_total` plus RTP/RTCP/SCTP overhead (see
[metrics.md](metrics.md)).
