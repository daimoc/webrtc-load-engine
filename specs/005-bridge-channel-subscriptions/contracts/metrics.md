# Contract: Metrics added by 005 (`/metrics`)

Added to the metrics of 002–004 ([004 metrics](../../004-synthetic-media/contracts/metrics.md)).
Labels are `scenario_id` only; never a peer ID (004 FR-013).

| Name | Type | Labels | Meaning |
|------|------|--------|---------|
| `webrtc_data_channels_open` | gauge | scenario_id | Data channels currently open on the scenario's bots. For Jitsi, one per bot: the bots whose bridge channel is open, so whose subscriptions are in effect (FR-008) |
| `webrtc_transport_bytes_received_total` | counter | scenario_id | RTP and RTCP bytes (decrypted, headers included) the bots received from the server, including streams they drop (FR-007, SC-003) |

Collection: the gauge changes on open/close; the counter is updated every
2 s by the scenario's media collector (004), with the same final collection
when the scenario ends.

Reading it:

- subscriptions in effect: `webrtc_data_channels_open == participants`;
- traffic beyond what the bots keep:
  `rate(webrtc_transport_bytes_received_total[1m]) / (rate(webrtc_media_bytes_received_total{kind="audio"}[1m]) + rate(webrtc_media_bytes_received_total{kind="video"}[1m]))`.
  With subscriptions honored it stays close to the RTP header and RTCP
  overhead (measured in validation; media bytes count payload only); with
  ignored streams it grows with the number of streams the server still
  sends.
