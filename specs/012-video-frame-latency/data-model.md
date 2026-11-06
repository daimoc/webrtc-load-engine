# Data Model: Video Latency on Servers That Rewrite Timestamps (012)

Everything is in memory, in `src/media`. The API, summary, report files and
metrics do not change (FR-007).

## Frame trailer

The 6 bytes after each frame's data
([contracts/frame-trailer.md](contracts/frame-trailer.md)).

| Field | Size | Rule |
|---|---|---|
| Tag | 3 bytes | first 3 bytes of FNV-1a of the scenario ID; fixed for a scenario |
| Frame number | 3 bytes, big endian | starts at 1, +1 per frame, wraps at 2^24; same on all layers of a frame |

## SendTimes (changed)

One per scenario and kind, as today.

| Field | Change |
|---|---|
| key | Video: frame number. Audio: RTP timestamp, as today |
| tag | New: the scenario tag (`[3]byte`); used for video only |

Memory stays 512 frames (about 17 s of video).

## Source (changed)

| Field | Rule |
|---|---|
| frame number | Video only; +1 per frame in `Run`, before recording the send time |

## VideoQuality (changed)

| Field | Rule |
|---|---|
| last frame number, seen flag | Ignore numbers not newer in 24-bit modular order |
| `lookups` | Unchanged meaning for the summary: +1 per frame received while send times are known |

### Sample rule

For a received packet, a sample is taken when all of these hold:

1. the stream has send times;
2. the packet's last 6 bytes start with the scenario tag;
3. the packet's timestamp is the current frame's;
4. the frame number is newer than the last one seen;
5. `SendTimes` still holds that number;
6. the current frame's arrival is not before the send time.

The sample is `current frame's arrival − send time`, added to the latency
buckets and observed in the existing Prometheus histogram.
