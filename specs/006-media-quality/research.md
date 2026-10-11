# Research: Media Quality

Decisions for [spec.md](spec.md), based on the code of 002–005 (receive path
in `src/execution/peer.go` `onTrack`, accounting in `src/media/inbound.go`,
one shared `media.Source` per scenario and kind) and on Pion v3.3.6.

## R1. Video frames, keyframes and resolution without decoding

- **Decision**: parse the VP8 RTP payload descriptor (RFC 7741 §4.2) of each
  received video packet: a frame starts at a packet with the S bit set and
  partition index 0; a new RTP timestamp also starts a frame; the marker bit
  ends it. In the first packet of a frame, the VP8 payload header (RFC 6386
  §9.1) gives the frame type (bit 0 of the frame tag, 0 = keyframe) and, for
  keyframes, the 14-bit width and height after the start code
  `9d 01 2a`. The received resolution of a stream is the height of its last
  keyframe.
- **Rationale**: a few byte reads per packet; no decoder (FR-007,
  constitution IV). The bots' content has a keyframe every second (004
  `scripts/gen-media.sh`), and the media server sends a keyframe after each
  layer switch, so the height is known within a second.
- **Alternatives considered**: decoding headers through a VP8 decoder (CPU,
  cgo); relying on the sender's announced resolution (does not show what the
  server forwards, US3).

## R2. Freezes and frame rate

- **Decision**: per received video stream, the frame time is the arrival of
  the first packet of a frame. The usual frame interval is an exponential
  average (1/16) of the intervals that are not freezes. A freeze is an
  interval longer than `max(3 × usual interval, 150 ms)` (FR-002); it is
  counted when the next frame arrives, with its duration capped at the stall
  timeout (004), so a freeze that becomes a stall counts once as a stall
  (004 collector) and once as a freeze of at most the stall timeout. A stream
  that never resumes (sender left, stream replaced) counts no freeze (spec
  edge cases). The frame rate of a stream is its frames in a collection
  period (2 s) divided by the period, sampled for streams that received in
  that period.
- **Rationale**: the W3C `freezeCount` definition uses
  `max(3 × average, average + 150 ms)`; the spec's rule is the same at 30 fps
  (150 ms) and simpler to explain. Counting on resume avoids counting the end
  of a stream as a freeze.
- **Alternatives considered**: frame completion time (marker packet):
  depends on NACK repairs and adds state; first packet is enough to see the
  server's pacing and freezes.

## R3. Audio gaps (concealment)

- **Decision**: per received audio stream, model a receiver with a fixed
  playout delay of 60 ms (spec assumption): a packet's deadline is
  `base + media time + 60 ms`, where media time comes from the RTP timestamp
  and `base` is the minimum of `arrival − media time` seen on the stream (the
  fastest path; reset when the stream restarts). A 20 ms frame is concealed
  when its packet is missing at its deadline (sequence gap) or arrives after
  it; each frame is counted once, a late packet that fills a gap is not
  counted twice. The gap share is concealed time over expected time
  (`(highest − first sequence + 1) × 20 ms`).
- **Rationale**: a listener hears a gap exactly when a frame is not there in
  time; a fixed delay is what a simple jitter buffer does and makes runs
  comparable. Same-host clocks make `base` stable (no drift handling).
- **Alternatives considered**: loss only (misses late packets, which is what
  overload causes first); an adaptive jitter buffer model (closer to a
  browser, but results depend on its tuning and are harder to compare).

## R4. Estimated audio score (E-model)

- **Decision**: ITU-T G.107 with default values except delay and loss:
  `R = 93.2 − Id − Ie,eff`; `Id = 0.024·Ta + 0.11·(Ta − 177.3)·H(Ta − 177.3)`
  (the usual simplification with default echo parameters);
  `Ie,eff = Ie + (95 − Ie)·Ppl / (Ppl/BurstR + Bpl)` with `Ppl` the gap share
  of R3 in percent and `BurstR = 1`; MOS from R with the G.107 Annex B
  conversion `MOS = 1 + 0.035R + R(R − 60)(100 − R)·7·10⁻⁶` (1 below 0, 4.5
  above 100). Codec values: G.113 does not give Opus values for the
  narrowband E-model, so the bots use those of G.711 with packet loss
  concealment (`Ie = 0`, `Bpl = 25.1`, G.113 Appendix I) as a stand-in, as
  common E-model tools do; the maximum score is therefore 4.41. `Ta` (one-way
  mouth-to-ear) is the stream's median end-to-end latency (R5) plus the
  playout delay (60 ms) and one frame (20 ms); without latency, half the
  round trip of the receiving bot's own streams (004) replaces it. One score
  per audio stream per collection period.
- **Rationale**: the user asked for an E-model score; with these values it
  depends only on what the load changes (loss, lateness, delay). SC-003 is
  checked with values computed by hand from the formulas above.
- **Alternatives considered**: the wideband E-model (G.107.1, R up to 129):
  closer to Opus but its codec values for Opus are not published either, and
  the familiar 1–4.5 scale is what operators read; a perceptual measure
  (POLQA, ViSQOL): needs decoding and a reference signal (constitution IV).

## R5. End-to-end latency

- **Decision**: the send time does not travel in the packets. Every bot of a
  scenario sends the same frames from one shared source per kind (004), with
  the same RTP timestamps; the source records, for each frame, the time it
  starts writing it to the bots' tracks, in a ring of the last 512 frames per
  kind (about 10 s of audio, 17 s of video). A receiver looks up the RTP
  timestamp of the first packet of each received frame (each audio packet)
  in the scenario's ring for that kind: a hit gives `arrival − send time`;
  a miss (media from another engine, or a timestamp the server rewrote) gives
  nothing (FR-009, spec edge case). The sender's identity is not needed
  because all bots send the same frame at the same time.
- **Spec impact**: FR-009 asks that "the send time travel with the media";
  the frame's RTP timestamp, which the media server forwards unchanged for
  each stream, identifies it instead. The spec is amended accordingly.
- **Rationale**: no per-packet extension to write or parse, nothing the media
  server must keep; one lookup per frame. The bias is the time to write a
  frame to all tracks after its start (at most a few ms for 100 bots, R10),
  which is below the 5 ms accuracy of SC-004 for the 10-bot runs.
- **Risks, checked first in implementation**: the Jitsi bridge rewrites the
  sequence numbers and picture IDs of VP8 streams it forwards; it must keep
  their RTP timestamps (or offset them by a constant 0) for video latency to
  be found. Audio is forwarded unchanged. If the live check shows rewritten
  video timestamps, the fallback identifies a video frame by its content
  (frame index in the looped content from the size and first bytes of its
  first packet's payload, which the server does not change), within the loop
  period (10 s).
- **Alternatives considered**: the `abs-capture-time` header extension: not
  in the extensions Jicofo offers (audio level and transport-cc only), so the
  bridge would not forward it, and it costs a header write and parse per
  packet; sender reports' NTP/RTP mapping: the bridge terminates RTCP and
  sends its own reports.

## R6. Several video resolutions (simulcast)

- **Decision**:
  - Content: `scripts/gen-media.sh` generates the layers from the same test
    pattern, 30 fps, keyframes every 30 frames at the same positions, no
    alt-ref: 160×90 at 60 kbit/s, 320×180 at 150 kbit/s, the existing 640×360
    at 500 kbit/s and 1280×720 at 1.5 Mbit/s. A scenario uses 90/180/360
    (default) or 180/360/720 (FR-010).
  - Sending: Pion's RID simulcast API (`RTPSender.AddEncoding` with one
    `TrackLocalStaticRTP` per layer, RIDs `l`, `m`, `h`); each encoding gets
    its own SSRC and its own interceptor chain (NACK, TWCC). Pion lists the
    three SSRCs in the answer (`a=ssrc`), plus `a=rid`/`a=simulcast`, which
    the Jingle conversion drops. The peer adds `a=ssrc-group:SIM` (low to
    high) to the copy of the answer it hands to signaling, as browsers
    announce SSRC simulcast; the Jitsi adapter already carries `ssrc-group`
    lines into Jingle and names all sources of the group with the bot's
    video source name.
  - Pacing: one video source per scenario sends the three layers in lockstep:
    frame i of every layer at the same instant with the same RTP timestamp
    (separate sequence numbers), so latency (R5) and layer switches keep a
    single timeline.
- **Rationale**: real Jitsi clients use SSRC simulcast with a SIM group; the
  answerer cannot use `a=simulcast` (Jicofo's offer has none). Pre-encoded
  layers keep the CPU cost to packetizing and encrypting (constitution IV).
- **Risks, checked first in implementation**: Pion's sender with RID
  encodings when the remote description has no RIDs (it must still bind
  every encoding and send each SSRC); the bridge's handling of VP8 layers
  without temporal-layer indices (it should treat each frame as base layer).
  Fallback if Pion does not bind the extra encodings: a custom track that
  writes the three SSRCs through the base encoding's binding.
- **Alternatives considered**: three video sections (not how Jitsi clients
  send; the bridge would see three cameras); encoding layers live (CPU).

## R7. Requested resolution

- **Decision**: the test plan sets the height each bot asks for (one value,
  or shares of bots per height, assigned in bot order); default: the top
  layer sent (360). `platform.SignalingHandler` gains
  `RequestedVideoHeight() int`; the Jitsi adapter uses it as the `maxHeight`
  of every received source in `ReceiverVideoConstraints` (005), instead of
  the constant 360.
- **Rationale**: platform-neutral request (constitution III), mapped by each
  adapter to its mechanism (FR-012).
- **Alternatives considered**: a per-source height (needs a selection policy
  per bot, not needed for SC-005).

## R8. Keyframe requests

- **Decision**: receiving bots send a picture loss indication (PLI) for a
  stream when its first frame, or the first frame after a sequence gap that
  was not repaired before the next frame started, is not a keyframe; at most
  one per stream per second. Sending bots count PLI and FIR received about
  their video (the RTCP read loop of 004). Neither changes what the shared
  source sends: the looped content has a keyframe every second.
- **Rationale**: real receivers ask for a keyframe when they cannot decode;
  counting both sides shows how often the server and the bots need one,
  which grows with loss and layer switches (FR-004).
- **Alternatives considered**: answering PLI with a keyframe (the source is
  shared by all bots, R5; jumping to a keyframe for everyone would distort
  the other bots' streams).

## R9. Collection and summary

- **Decision**:
  - Per received stream, a `media.QualityTracker` (one per stream, updated by
    the stream's read loop under its own mutex like `InboundStats`) keeps
    cumulative counters and per-period values (R1–R5).
  - The scenario's media collector (004, every 2 s) folds them into the
    Prometheus series ([contracts/metrics.md](contracts/metrics.md)) and
    into one per-bot record kept until the scenario ends, also for bots that
    left early.
  - Latency is observed into the Prometheus histogram at each lookup (one
    atomic operation per frame) and into a per-bot bucket array for the
    summary's per-bot percentiles.
  - At the last collection, the engine builds the quality summary
    ([contracts/quality-summary.yaml](contracts/quality-summary.yaml)) and
    hands it to the scenario service with the final status; `GET
    /api/v1/scenarios/{id}/quality` returns it.
- **Rationale**: same pattern and period as 004's metrics; the summary
  needs per-bot values, which Prometheus series (scenario labels only) do not
  keep.
- **Alternatives considered**: computing the summary from Prometheus
  (loses per-bot detail, FR-015).

## R10. CPU cost

- **Decision**: per kept packet, the quality accounting adds a header parse
  already done, a few byte reads (video), a sequence/deadline update
  (audio) and, per frame, a ring lookup and a histogram observation. A
  benchmark (`media` package) bounds it: under 200 ns per packet on the
  validation laptop. With cap 10, 100 bots keep about 115,000 packets/s, so
  under 0.03 core. Simulcast at 360 adds the 90 and 180 layers (about 40%
  more video packets sent by every bot), measured in SC-008.
- **Rationale**: SC-006 allows at most 20% over 005's 1.05 cores
  (0.21 core).

## R11. Controlled tests

- **Decision**: SC-002 (freeze of 1 s) with an in-process pair of Pion peers
  where the test stops writing video for 1 s; SC-004 (known delay) with the
  same pair connected through `pion/transport/v2/vnet` with a router whose
  `MinDelay` is the known delay (50 ms), comparing the median latency to it;
  SC-003 with unit tests of the score against values computed by hand from
  R4's formulas. Live runs on the local Jitsi cover SC-001, SC-005, SC-006,
  SC-008.
- **Rationale**: the controlled tests need exact interruptions and delays,
  which the live deployment cannot give; network impairment stays out of
  scope (spec assumption).
