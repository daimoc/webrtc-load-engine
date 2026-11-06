# Contract: Frame Trailer (012)

The only external change is 6 more bytes in sent video. Test plans, the API,
report files, metrics and signaling do not change.

## Layout

The last 6 bytes of a VP8 frame's last RTP packet, after the frame's data:

```text
... VP8 frame data | tag[0] tag[1] tag[2] | n[0] n[1] n[2] |
                   |   scenario tag       | frame number   |
```

- **Tag**: first 3 bytes of the 32-bit FNV-1a of the scenario ID.
- **Frame number**: big endian, from 1, +1 per frame, modulo 2^24.

## Rules for senders

- Every video frame of every layer carries a trailer.
- All layers of one frame carry the same frame number.
- The trailer is never split across packets.
- Audio carries none.

## Rules for receivers

- A packet whose last 6 bytes do not start with the tag has no trailer. That is
  normal: it is mid-frame, or from another publisher.
- Frame numbers compare in 24-bit modular order.
- A receiver never needs to remove the trailer: bots do not decode, and VP8
  decoders are expected not to read it ([research R5](../research.md)).

## Examples

Scenario tag `a1 b2 c3`, frame 258:

```text
a1 b2 c3 00 01 02
```

Frame 16777215 is followed by frame 0.
