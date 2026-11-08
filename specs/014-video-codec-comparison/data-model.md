# Data Model: Video Codec Comparison (014)

## VideoCodec (test plan)

`media.video_codec` ([contract](contracts/test-plan-codec.md)).

| Value | Codec | Layered arrangement (`media.simulcast` set) |
|---|---|---|
| `vp8` (default) | VP8 | simulcast, 3 streams |
| `vp9` | VP9 profile 0 | SVC `L3T3_KEY`, 1 stream |
| `av1` | AV1 main profile | SVC `L3T3_KEY`, 1 stream |
| `h264` | H.264 Constrained Baseline `42e01f`, packetization mode 1 | simulcast, 3 streams |

Validation at creation:

- one of the four values, else refused naming them (FR-002);
- set with a profile other than `audio_video`: refused (FR-002);
- `source_file` with `vp9` or `av1` and `simulcast`: refused (research R13);
- `simulcast.max_height` stays 360 or 720 for every codec.

## Arrangement

Derived, not set: `single` without `simulcast`; with it, `simulcast` for VP8
and H.264, `svc` for VP9 and AV1. Heights per arrangement: 360 for `single`;
90/180/360 or 180/360/720 for the other two.

## Content (embedded)

One per codec and height for `single`/`simulcast`, one per codec and maximum
height for `svc`.

| Field | Meaning |
|---|---|
| codec | one of the four |
| arrangement | `single`/`simulcast` (one file per height) or `svc` (one file holding all layers) |
| height(s) | 90, 180, 360, 720; an SVC file lists its three |
| frames | in-memory layer frames, 300 per layer (10 s at 30 fps), every one shown |
| keyframes | every 30 frames, at the same frames in every layer and codec |
| bitrate | per height, kbit/s, from `content.json` |
| vmaf, vmaf_reference | per height; `abs(vmaf - vmaf_reference) <= 1.0` (SC-009) |

`content.json` is the manifest written by `scripts/gen-media.sh` and embedded
with the files. The VP8 entries describe today's files, which do not change.

## Layer frame (send side)

What the source packetizes, once per frame per layer (research R5):

| Field | Meaning |
|---|---|
| data | the codec's frame bytes for this layer |
| spatial_id, temporal_id | 0 for single stream; simulcast layers are separate streams with spatial 0 |
| keyframe | the frame starts a group (all spatial layers of an SVC keyframe) |
| frame_number | 012's number, the same in every layer of one picture |

## Frame description (receive side)

Produced per packet by the codec's parser, read by `VideoQuality` (research R8):

| Field | Meaning |
|---|---|
| frame_start, frame_end | first and last packet of a layer frame |
| keyframe | set on a frame start only |
| width, height | when the packet tells them (keyframe header, SPS, scalability structure, descriptor structure); else 0 |
| spatial_id, temporal_id | 0 when the codec carries none |

The received picture's height is the highest spatial layer's height seen for
its timestamp.

## Frame trailer carrier

Per codec (research R9). The trailer itself is 012's: 3-byte scenario tag,
3-byte frame number.

| Codec | Carried as | Read as |
|---|---|---|
| VP8 | appended to the frame data | last 6 bytes of the last packet |
| VP9 | RTP padding of the layer frame's last packet: the trailer, then the padding length (7) | the padding's first 6 bytes |
| AV1 | an `OBU_PADDING` element, last in the last packet | last 6 bytes of the last packet |
| H.264 | SEI user data unregistered with the project UUID, its own last packet | SEI parsed, emulation prevention removed, UUID checked |

## Codec result (report)

([contract](contracts/report-codec.md))

| Field | Meaning |
|---|---|
| video_codec | the plan's codec |
| arrangement | `single`, `simulcast` or `svc` |
| content_bitrate_kbps | per height sent |
| per bot: codec_received | codecs of the video tracks it received; normally the plan's only |
| codec_mismatch | bots that sent or received another codec, with the codec found |
| layers_not_forwarded | set when a layered run received only the lowest layer everywhere |

## Bot failure reasons (added)

| Reason | When |
|---|---|
| `codec_mismatch` | the negotiated sending codec, or a received track's codec, is not the plan's; the error names the codec found |

`layers_not_forwarded` is a run result, not a bot failure: the bots did
receive media.
