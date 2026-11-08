# Contract: `media.video_codec` in the test plan (014)

Extends the media part of the test plan
([004](../../004-synthetic-media/contracts/test-plan-media.yaml),
[006](../../006-media-quality/contracts/test-plan-quality.yaml)). Every field
not named here keeps its contract.

```yaml
media:
  profile: audio_video
  video_codec: vp9          # vp8 (default) | vp9 | av1 | h264
  max_received_streams: 10
  simulcast:                # optional: layered video, in every codec
    max_height: 720         # 360 (default) or 720
  requested_heights:        # optional, as 006
    - {height: 180, share: 0.5}
    - {height: 720, share: 0.5}
```

## `video_codec`

| Value | Sent as |
|---|---|
| `vp8` | VP8 (as before 014) |
| `vp9` | VP9 profile 0 (`profile-id=0`) |
| `av1` | AV1 main profile |
| `h264` | H.264 Constrained Baseline (`profile-level-id=42e01f`), `packetization-mode=1` |

Absent: `vp8`. Case-sensitive, lower case.

## `simulcast` with each codec

`simulcast` now means "layered video". Its heights do not depend on the codec:
90/180/360 with `max_height: 360`, 180/360/720 with `max_height: 720`.

| Codec | Layers sent |
|---|---|
| `vp8`, `h264` | three simulcast streams, one per height |
| `vp9`, `av1` | one stream, three spatial and three temporal layers (`L3T3_KEY`) |

`requested_heights` and `max_received_streams` apply to every codec. Each
platform carries them in its own way; on every platform they now work with
`simulcast` (before 014: Jitsi only).

## Refused at creation

The error names the field and, for an unknown value, the four accepted ones.

- `video_codec` not one of `vp8`, `vp9`, `av1`, `h264`;
- `video_codec` with a profile other than `audio_video`;
- `source_file` with `video_codec` `vp9` or `av1` and `simulcast` set (SVC
  cannot be made from a file at run time; research R13);
- the existing 006 refusals (`max_height` not 360 or 720, bad shares).

A server that does not support a codec is not known at creation: it fails at
run time with `codec_mismatch` ([report](report-codec.md)).

## Unchanged

A test plan without `video_codec` is valid and behaves as before 014
(SC-007).
