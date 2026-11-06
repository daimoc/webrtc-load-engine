# Research: Video Latency on Servers That Rewrite Timestamps (012)

## R1: How a frame is identified

**Decision**: a 6-byte trailer after each frame's VP8 data, in the frame's last
RTP packet: a 3-byte scenario tag, then a 3-byte frame number
([contracts/frame-trailer.md](contracts/frame-trailer.md)).

**Rationale**:
- Servers rewrite what surrounds a frame (timestamp, sequence number, picture
  ID, TL0PICIDX), but an SFU has no reason to touch the frame data (R2).
- VP8's last partition runs to the end of the frame, so nothing has to change
  in the frame's headers.
- It needs no decoding: the receiver reads a packet's last bytes.

**Size**: bundled content, 10 s loops of 300 frames:

| Layer | Bitrate | 4 bytes | 6 bytes | 8 bytes |
|---|---|---|---|---|
| 90p | 79 kbps | 1.2% | **1.8%** | 2.4% |
| 180p | 166 kbps | 0.6% | 0.9% | 1.2% |
| 360p | 497 kbps | 0.2% | 0.3% | 0.4% |
| 720p | 1491 kbps | 0.1% | 0.1% | 0.1% |

SC-007 allows 2% on the lowest layer, so 8 bytes is too many. With 6:
- A 24-bit frame number wraps after about 6 days at 30 fps, far beyond the send
  times' memory of 512 frames.
- A 24-bit tag on foreign media (a browser in the room) matches about once in
  16 million packets, and the number must then also be among 512 recent frames
  out of 16 million: about 2 in 10^12 per packet. 4 bytes (16-bit tag) would be
  about 1 in 10^7 per packet, which an hour of a browser's video could hit: a
  wrong value, which FR-008 forbids.

**Alternatives considered**:
- *Hash of the frame's data, nothing added to the stream.* The loop is 10 s
  and the send times remember about 17 s, so every frame has two candidates,
  and another run in the same room sends identical frames (spec edge cases,
  FR-006). Rejected.
- *Picture ID.* Rewritten by the Jitsi bridge, and only 15 bits. Rejected.
- *RTP header extension (`abs-capture-time`).* Made for this, but it has to be
  negotiated per platform (touching adapters, against FR-002) and SFUs drop or
  rewrite extensions. Rejected.
- *Learning each stream's timestamp offset.* It needs a known reference frame,
  so it would assume the latency it measures, and it changes on layer switches.
  Rejected.

## R2: Do the servers keep the trailer? (spec assumption)

**Decision**: yes on both target servers; design confirmed.

**Probe** (2026-11-06, throwaway worktree, 8-byte trailer, 4 bots, 30 s hold):

| Server | Video latency p50 | Audio latency p50 | Freezes | Frame rate | MOS |
|---|---|---|---|---|---|
| Janus v1.4.3 | 0.5 ms | `frame_not_identified` (expected) | 0 | 30.0 fps | 4.37 |
| Jitsi, simulcast 720, cap 4 | 1.5 ms | 0.5 ms | 0 | 30.1 fps | 4.37 |

Reports: `20261106T163403Z-janus-live-janus-c0a6920b`,
`20261106T163439Z-jitsi-live-38b95474` (scratch, not kept in the repository).
The Jitsi bots received 360p 97% of the time and 720p 3%, so they received two
layers; the probe did not break samples down by layer, so the automated tests
must cover a layer switch (FR-004).

LiveKit and mediasoup were not probed. Both are in the validation runs
(SC-005); a server that drops the trailer would show as `frame_not_identified`.

## R3: Reading it

**Decision**: compare the last 6 bytes of every video packet with the tag.

- Every packet, not only those with the RTP marker bit: one less thing that
  depends on what a server keeps, and the compare costs a few nanoseconds.
- The latency is `first packet's arrival − send time`, as today. The arrival is
  the one `VideoQuality` already keeps for the current frame, and the sample is
  taken only if the packet's timestamp is still the current frame's, so a
  reordered last packet cannot borrow the next frame's arrival.
- A frame number at or before the last one seen on the stream is ignored
  (24-bit modular order), so a retransmitted last packet counts once.
- A frame whose last packet was lost has no sample. Bots stay measured by their
  other frames.

## R4: One path or a fallback

**Decision**: one path. Video send times are keyed by frame number on every
platform, and the timestamp is no longer used for video latency.

**Rationale**: a fallback would need rules on which path won, and it keeps the
risk of a false match on a rewritten timestamp. If a server dropped the trailer
but kept timestamps, latency would go from measured to left out with its
reason. That is visible in a report, and no such server is known.

**Consequence**: `lookups` still counts frames received with send times known,
so the summary's reasons do not change: frames but no sample gives
`frame_not_identified`, and no send times gives `not_measured`.

## R5: Browsers and decoders

**Result**: harmless, verified on 2026-11-06
([validation report](validation-report.md)). Every bundled frame decodes
pixel-identical with and without the trailer in Chrome 156 and Firefox 158
(software decoders, headless), and Chrome decoded a steady 30 fps from bots
through Jitsi and Janus.

**Why**: libvpx's encoder flushes its boolean coder when it ends a partition, so
decoding stops depending on later bytes, and the last partition's size is
implicit.

**Not tested**: hardware VP8 decoders.

## R6: Audio stays out

**Decision**: audio is not changed, as the spec says.

Opus frames are small and come 50 times a second, so the trailer would cost
more than on video. Decoders can also be affected: Opus uses the packet length
to find its frame sizes, so trailing bytes are not ignored. Keeping
`frame_not_identified` for Janus audio is the honest result.

`initialRTPTimestamp` stays at 2^30: Janus audio still depends on it to avoid
false matches (011 R8).

## R7: Writing it

**Decision**: packetize video with `rtpPayloadMTU − 6`, then append the trailer
to the last payload.

- Room is reserved by the payloader call, so the trailer can never split across
  two packets, whatever the frame size and payloader (with or without simulcast).
- The frame number is the same across layers (they are sent in lockstep), so
  all layers share one send time and a layer switch changes nothing (FR-004).
- Frames stay packetized once per layer for all bots, as in 006. The append adds
  one small copy per frame and layer, not per bot.

## R8: The tag

**Decision**: the first 3 bytes of the FNV-1a hash of the scenario ID. It is
held by `SendTimes`, so `VideoQuality.SetLatency` keeps its signature and
`src/execution` does not change.

Two runs in the same room therefore have different tags and do not match each
other's frames (FR-006), unlike today's shared timestamps. Scenario IDs are
UUIDs; a hash collision between two runs in one room is about 1 in 16 million.
