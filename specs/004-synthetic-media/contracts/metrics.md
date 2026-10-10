# Contract: Media Metrics (`/metrics`)

Added to the existing per-scenario metrics of 002/003. Labels are
`scenario_id` and `kind` (`audio` | `video`) only; never a peer ID (FR-013).
Per-peer detail is in the structured logs, with the same `scenario_id` and
`peer_id` keys (FR-011).

| Name | Type | Labels | Meaning |
|------|------|--------|---------|
| `webrtc_media_packets_sent_total` | counter | scenario_id, kind | RTP packets sent by the scenario's bots |
| `webrtc_media_bytes_sent_total` | counter | scenario_id, kind | RTP payload bytes sent |
| `webrtc_media_packets_received_total` | counter | scenario_id, kind | RTP packets received by the bots |
| `webrtc_media_bytes_received_total` | counter | scenario_id, kind | RTP payload bytes received |
| `webrtc_media_packets_lost_total` | counter | scenario_id, kind | Packets the bots expected but did not receive (sequence gaps, RFC 3550) |
| `webrtc_media_jitter_seconds` | histogram | scenario_id, kind | Interarrival jitter of received streams, one sample per stream per collection |
| `webrtc_media_round_trip_seconds` | histogram | scenario_id, kind | Round-trip time from the media server's receiver reports on the bots' streams |
| `webrtc_media_remote_streams` | gauge | scenario_id, kind | Remote streams the bots are offered (one per remote participant source, at most `max_received_streams` per bot), summed over the established bots; what they can expect to receive |
| `webrtc_media_stalls_total` | counter | scenario_id, kind | Incoming media stalls (see `stall_timeout_seconds`) |
| `webrtc_media_send_lateness_seconds` | histogram | scenario_id, kind | Delay between a frame's scheduled send time and its actual write (SC-003) |

Collection: counters, the gauge and the jitter/round-trip histograms are
updated every 2 s by the scenario's collector; send lateness is observed by
the pacer for each frame. Series of a scenario stop changing when it ends.

Health at a glance (SC-005):

- media flowing: `rate(webrtc_media_packets_received_total[30s]) > 0` per kind;
- expected forwarding: `webrtc_media_remote_streams / participants` close to
  the number of other senders (or the bridge's last-N);
- loss: `rate(webrtc_media_packets_lost_total[30s]) / rate(webrtc_media_packets_received_total[30s])`;
- stalls: `increase(webrtc_media_stalls_total[30s]) == 0`.
