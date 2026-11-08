# Contract: codec fields in the quality summary and report files (014)

Adds to the quality summary
([006](../../006-media-quality/contracts/quality-summary.yaml)) and the run
report files ([009](../../009-run-report-files/contracts/run-report.schema.yaml),
[Markdown](../../009-run-report-files/contracts/report-markdown.md)). Existing
fields keep their names and meaning. `format_version` is raised by one.

## JSON: `quality.video.codec`

```json
"codec": {
  "sent": "vp9",
  "arrangement": "svc",
  "content_bitrate_kbps": {"180": 95, "360": 260, "720": 800},
  "received": {"vp9": 10},
  "mismatch": [],
  "layers_not_forwarded": false
}
```

| Field | Type | Meaning |
|---|---|---|
| `sent` | string | the plan's `video_codec` (`vp8` when absent) |
| `arrangement` | string | `single`, `simulcast` or `svc` |
| `content_bitrate_kbps` | object | height → bitrate of the content sent, from the embedded manifest |
| `received` | object | codec → number of bots that received video in it |
| `mismatch` | array | `{"bot": "<id>", "direction": "sent" \| "received", "codec": "<found>"}` per bot whose codec was not the plan's |
| `layers_not_forwarded` | boolean | layered run in which no bot received a layer above the lowest |

`quality.video` is present only with `audio_video`, as today; so is `codec`.

## Per bot

Each bot's entry gains `video_codec_received`: the list of codecs of the video
tracks it received, normally one.

## Bot failure reason

`codec_mismatch`, stage `media`: the codec the bot negotiated to send, or the
codec of a video track it received, is not the plan's. The error text names
the codec found and the platform, for example
`codec_mismatch: sending negotiated VP8, plan asks for av1 (platform mediasoup)`.

A run with any `codec_mismatch` does not pass (SC-002), and its figures are
not reported as the plan's codec: the Markdown key figures say so on their
first line.

## Markdown key figures

Two rows after "Outcome":

```markdown
| Video codec | vp9 (svc: 180, 360, 720) · received vp9 by 10 bots |
| Video content bitrate | 180: 95 kbit/s, 360: 260 kbit/s, 720: 800 kbit/s |
```

With a mismatch, the first row reads
`vp9 asked · 3 bots on another codec: see Video` and the Video section lists
them. With `layers_not_forwarded`, the Video section says:
`The server forwarded only the lowest layer: layered result not available.`
