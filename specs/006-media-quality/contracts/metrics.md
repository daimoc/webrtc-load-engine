# Contract: Metrics added by 006 (`/metrics`)

Added to the metrics of 002–005
([004 metrics](../../004-synthetic-media/contracts/metrics.md),
[005 metrics](../../005-bridge-channel-subscriptions/contracts/metrics.md)).
Labels: `scenario_id`, plus `kind` where noted and `height` for the time per
resolution; never a peer ID (FR-014).

| Name | Type | Labels | Meaning |
|------|------|--------|---------|
| `webrtc_media_frame_rate` | histogram | scenario_id | Frames per second of each received video stream, one sample per stream per collection period (2 s). Buckets 5, 10, 15, 20, 24, 27, 29, 31, 40 |
| `webrtc_media_freezes_total` | counter | scenario_id | Video freezes (FR-002) |
| `webrtc_media_freeze_seconds_total` | counter | scenario_id | Total freeze duration |
| `webrtc_media_frame_height` | histogram | scenario_id | Height of each received video stream (last keyframe), one sample per stream per period. Buckets 90, 180, 270, 360, 540, 720, 1080 |
| `webrtc_media_height_seconds_total` | counter | scenario_id, height | Time received video spent at each height |
| `webrtc_media_resolution_switches_total` | counter | scenario_id | Height changes of received video streams |
| `webrtc_media_keyframe_requests_sent_total` | counter | scenario_id | Keyframe requests (PLI) the bots sent as receivers |
| `webrtc_media_keyframe_requests_received_total` | counter | scenario_id | Keyframe requests (PLI, FIR) the bots received as senders |
| `webrtc_media_audio_concealed_seconds_total` | counter | scenario_id | Received audio a listener would not hear in time (missing or late, 60 ms playout delay) |
| `webrtc_media_audio_expected_seconds_total` | counter | scenario_id | Received audio expected (from sequence numbers) |
| `webrtc_media_audio_mos` | histogram | scenario_id | Estimated listening score (E-model), one sample per audio stream per period. Buckets 1, 2, 3, 3.5, 3.8, 4, 4.1, 4.2, 4.3, 4.4, 4.5 |
| `webrtc_media_latency_seconds` | histogram | scenario_id, kind | Send-to-receive time of frames between bots of the same engine, one sample per video frame and per audio packet. Buckets .005, .01, .02, .03, .05, .075, .1, .15, .2, .3, .5, 1 |

Reading them:

- audio gap share: `rate(webrtc_media_audio_concealed_seconds_total[1m]) / rate(webrtc_media_audio_expected_seconds_total[1m])`;
- freeze share: `rate(webrtc_media_freeze_seconds_total[1m]) / sum(webrtc_media_remote_streams{kind="video"})` approximates the fraction of time a received video is frozen;
- median latency: `histogram_quantile(0.5, rate(webrtc_media_latency_seconds_bucket[1m]))`.
