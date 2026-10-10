# Research: Synthetic Media Sending and Media Metrics

Decisions taken for [spec.md](spec.md), from the current code (Pion v3.3.6,
pion/interceptor v0.1.29, the Jitsi adapter of 003) and from how
lib-jitsi-meet and Jicofo behave.

## R1. Media content: pre-encoded files embedded in the binary

- **Decision**: ship two pre-encoded files, embedded with `go:embed` in a new
  `src/media` package: a VP8 video in IVF (640x360, 30 fps, about 500 kbit/s,
  a keyframe every 30 frames, about 10 s, test pattern with a moving element)
  and an Opus audio in Ogg (48 kHz mono, 20 ms frames, about 32 kbit/s, a
  tone with periodic beeps, same duration). They are generated once by
  `scripts/gen-media.sh` (ffmpeg through its Docker image, since ffmpeg is
  not installed locally) and committed. At startup they are parsed once with
  Pion's `ivfreader`/`oggreader` into in-memory frames.
- **Rationale**: FR-004 (no real-time encoding, constitution IV), FR-005 (no
  operator files), single static binary (constitution, Technical
  Constraints). Total size is under 1 MB. A keyframe every second lets a
  receiver that joins later, or the bridge after a loss, get a picture within
  1 s without the bot having to answer keyframe requests (SC-001). A tone
  that is not silence keeps the audio forwarded by bridges that drop silence.
- **Alternatives considered**: generating frames in Go (needs a VP8 encoder,
  i.e. real-time encoding, rejected by FR-004); reading files from disk at
  runtime (breaks FR-005 and the single binary); H.264 (Jitsi prefers VP8,
  and Pion's H.264 path needs Annex-B parsing for no gain).

## R2. Sending: one shared pacer per scenario and media kind

- **Decision**: a `media.Source` per scenario and media kind runs one
  goroutine. It packetizes each frame once (Pion `rtp.Packetizer`, with
  sequence numbers and timestamps that keep growing across loops), then
  writes the same packets to every subscribed track. Each bot has one
  `webrtc.TrackLocalStaticRTP` per kind, added to its PeerConnection before
  the offer is answered; Pion rewrites the SSRC and payload type per binding.
  Bots subscribe when their transport is established and unsubscribe when
  they leave. The pacer measures how late each frame is written against its
  schedule (`webrtc_media_send_lateness_seconds`, for SC-003).
- **Rationale**: one timer per kind instead of one per bot, and one
  packetization per frame instead of one per bot (constitution IV). Every bot
  sends exactly the same payloads on the same schedule (FR-006, constitution
  II). Late joiners start mid-loop, which is fine since keyframes repeat
  every second.
- **Alternatives considered**: `TrackLocalStaticSample` with one goroutine
  per bot (N timers and N packetizations, non-deterministic start offsets);
  per-bot files (no benefit, more memory).

## R3. Answering with local tracks

- **Decision**: in `Peer`, add the local tracks for the scenario's profile
  before `SetRemoteDescription`. Pion then reuses those transceivers for the
  offered audio/video sections and answers `sendrecv`, with `a=ssrc` (cname,
  msid) lines. Pion v3.3.6 does not send RTX for local tracks (no RTX SSRC
  or `FID` group, verified in T011); it answers NACKs by retransmitting on
  the main SSRC, which the bridge accepts. If the offer has no
  section for a kind, the track stays unused and is logged (spec edge case).
  For receive-only scenarios nothing changes (FR-002).
- **Rationale**: the existing `convertSDPToJingle` already turns `a=ssrc`
  and `a=ssrc-group` lines into Jingle `<source>` and `<ssrc-group>` elements,
  so the session-accept carries the sources with no new translation.
- **Alternatives considered**: answering recvonly first and sending
  `source-add` later (what lib-jitsi-meet does on unmute). It needs one more
  round trip and gives the same result.

## R4. Announcing sources to Jitsi

- **Decision**: the Jitsi adapter adds what Jicofo and lib-jitsi-meet expect
  for each local source:
  - in the session-accept: a source name `<nick>-a0` / `<nick>-v0` (we
    advertise `http://jitsi.org/source-name`), `videoType="camera"` on the
    video source, and `<ssrc-info xmlns="http://jitsi.org/jitmeet" owner="<room>/<nick>"/>`;
  - in the MUC join presence: `<SourceInfo>` with the JSON map
    `{"<nick>-a0":{"muted":false},"<nick>-v0":{"muted":false,"videoType":"camera"}}`,
    plus the legacy `<audiomuted>false</audiomuted>` / `<videomuted>false</videomuted>`.
  The adapter learns which kinds the bot sends from a new
  `SignalingHandler.LocalMediaKinds()` method, called before the join.
- **Rationale**: lib-jitsi-meet takes a remote track's mute state from the
  `SourceInfo` presence of its owner; without it, browsers show the bot as
  muted (SC-001). The source naming and owner follow lib-jitsi-meet so that
  Jicofo's source validation accepts them. FR-007/FR-008: all of this stays
  in the adapter; the media layer only exposes media kinds.
- **Alternatives considered**: letting the media layer name sources (leaks
  Jitsi conventions into it, constitution II/III).
- **To verify on the local deployment**: whether Jicofo rejects the
  session-accept for any source attribute (the quickstart covers it).

## R5. Receiving several remote sources: one SDP section per remote source

- **Decision**: in `convertJingleToSDP`, keep the first audio and video
  sections (named after the Jingle contents, carrying our own track), and
  add one `recvonly` section per remote source, together with its RTX
  source and `FID` group, with a mid derived from the content name and a
  per-session counter (`audio-1`, `video-2`, ...). All sections are in the
  BUNDLE group. `convertSDPToJingle` collapses the answer back: only
  sections whose mid is a Jingle content name become contents, and the
  BUNDLE group is filtered the same way. The adapter keeps the mapping
  from source to mid for the whole session.
- **Rationale**: Jicofo sends Plan-B style contents (all remote sources in
  one audio and one video content). Pion follows Unified Plan and starts one
  receiver per media section; extra SSRCs in a section are dropped, so
  today's bots receive at most one stream per kind. lib-jitsi-meet performs
  the same expansion on its side. The expansion is Jitsi-specific and stays
  in the adapter (constitution III); the media layer still gets a standard
  SDP offer.
- **Alternatives considered**: relying on Pion's undeclared-SSRC handling
  (works only for a single unsignalled SSRC per kind); a lower-level RTP
  read (bypasses Pion's receivers and interceptors, much more code).
- **Implementation notes** (T030/T031): the bridge's own sources (owner
  `jvb`, or named `jvb-a0`/`jvb-v0`) get no section: they carry no
  participant media, and Pion drops packets on SSRCs it was not offered
  (Jitsi offers have several sections). For simulcast (`SIM` group) only the
  first layer and its RTX are offered, as the bridge forwards the selected
  layer on it. Without this split, a bot joining a room where 2+
  participants already send was rejected by Pion ("Expected UnifiedPlan,
  but RemoteDescription is PlanB"), found in T022.

## R6. source-add / source-remove: local renegotiation

- **Decision**: the adapter keeps the session's current remote description
  (the Jingle contents and their sources). On `source-add` or
  `source-remove` from the focus, it updates that description, regenerates
  the SDP offer (same ICE credentials, `o=` version incremented, removed
  sources keep their section as `inactive` without SSRC; sections are not
  reused, as Pion v3 does not rebind a receiver to a new SSRC) and passes it to a new
  `SignalingHandler.Renegotiate(sdp) (answer, error)`. The answer is not
  sent; Jitsi only expects the IQ result, as for lib-jitsi-meet.
  Renegotiations of one session are serialized. A source-add received before
  the session is accepted is applied after it.
- **Participants leaving**: Jicofo sends no `source-remove` for a
  participant who leaves (verified on the local deployment); like
  lib-jitsi-meet, the adapter drops the sources whose `ssrc-info` owner is
  the occupant of an unavailable MUC presence.
- **Rationale**: participants who join after a bot are announced by
  `source-add`; without it, received volumes stay flat and SC-002 cannot be
  met. `Renegotiate` keeps the answer, so it also works for platforms that
  must send it back. `OnOffer` keeps its first-offer duties (trickle, timing
  metric).
- **Alternatives considered**: reusing `OnOffer` (would re-arm trickle and
  count a second negotiation in the metrics); ignoring source changes (the
  current behavior; reception would only cover sources present at join).

## R7. Receive accounting and quality metrics

- **Decision**: the peer reads every remote track (`OnTrack` → read loop) and
  every sender's RTCP. A small `media.InboundStats` per remote stream keeps
  packets, bytes, extended highest sequence number (losses) and the RFC 3550
  interarrival jitter. The RTCP loop takes receiver reports about our streams
  and computes the round-trip time from LSR/DLSR. A per-scenario collector in
  the engine wakes every 2 s, adds each peer's deltas to the scenario's
  counters and records jitter and round-trip samples in histograms. Labels
  are `scenario_id` and `kind` only (FR-011, FR-013).
- **Rationale**: incoming RTP must be read anyway, or Pion's buffers fill and
  its interceptors never see the packets. Computing loss and jitter inline
  costs a few arithmetic operations per packet. Pion's stats interceptor
  would give the same values but runs one goroutine and one channel send per
  packet for each SSRC (about 20,000 goroutines at 100 bots in one room),
  against the CPU budget (SC-004).
- **Alternatives considered**: `pion/interceptor/pkg/stats` (see above);
  `PeerConnection.GetStats()` (returns no inbound RTP stats unless the stats
  interceptor is registered).

## R8. Stall detection

- **Decision**: per peer and kind, the read loops store the time of the last
  received packet. The collector counts a stall (`webrtc_media_stalls_total`)
  and logs it with `scenario_id` and `peer_id` when the peer is established,
  has received that kind before, and has received nothing of it for longer
  than `stall_timeout_seconds` (default 10). A stall is counted once until
  media resumes, and does not fail the peer (FR-014).
- **Rationale**: FR-012 without false positives in empty rooms (nothing
  received yet is not a stall).
- **Remote senders leaving**: with R6, the peer knows how many remote
  sources of each kind the current offer has; the collector skips kinds with
  none, so a bot left alone is not counted as stalled.

## R9. Scenario configuration

- **Decision**: a `media` object in the test plan:
  `{"profile": "receive_only" | "audio" | "audio_video", "stall_timeout_seconds": 10}`.
  Missing object or missing profile means `receive_only` (FR-002). Unknown
  profiles and non-positive timeouts are rejected when the scenario is
  created (400), like other test plan fields.
- **Rationale**: the profile is platform-independent (FR-008), so it belongs
  in the test plan, not in `platform_config`.

## R10. CPU budget and the bridge's forwarding (risk)

- **Facts**: sending costs one SRTP encryption per packet and per bot (about
  95 packets/s per bot with audio and video). Receiving dominates: with full
  forwarding in a room of N bots, each bot receives up to N-1 audio and N-1
  video streams. For 100 bots that is up to about 900,000 packets/s to
  decrypt and read. The bridge bounds what it forwards to each receiver with
  its bandwidth estimate (Pion sends transport-cc feedback) and its last-N
  setting; without a bridge channel, the bots cannot send receiver
  constraints on the local deployment (see 003 T054).
- **Decision**: no bot-side limit in this feature. Measure the 100-bot run
  (quickstart step 5) and report CPU together with the number of streams
  received per bot (`webrtc_media_remote_streams`). If SC-004 is exceeded,
  the options are, in order: run the measurement with the deployment's
  last-N set (`videobridge.cc.jvb-last-n`), as production deployments do;
  then add an optional bot-side cap on received video sources (a later
  feature, after a spec change).
- **Rationale**: receiving what the bridge forwards is the realistic
  behavior; a cap chosen now would be a guess.
- **Measured (T038/T039)**: with 100 bots in one room on the local
  deployment, each bot receives the 99 other audio streams (the bridge
  forwards all audio; last-N only applies to video), 495,000 packets/s in
  total, which costs 3.85 cores (about 7.8 µs per packet, almost all in
  Pion's receive path: system calls, SRTP, buffering). Dropping the TWCC
  and NACK-generator interceptors saves 2%. SC-004 is therefore not met for
  a single 100-participant room with all audio forwarded. Open decision:
  measure with loudest-only audio routing on the bridge, keep the bridge
  off the load machine, or bound reception on the bot side (spec change).

## R11. Validation target

- **Decision**: the local docker-jitsi-meet from CLAUDE.md
  (`https://localhost:8443/IntriguingClimatesSupposeProbably`, XMPP domain
  `meet.jitsi`, WebSocket at `ws://localhost:8000/xmpp-websocket`). The live
  test gets a `JITSI_MEDIA` profile and checks the media metrics. A browser
  joining that room checks SC-001 by eye.
- **Note** (corrected 2026-10-10): the bots first got no bridge channel
  because Jicofo only offers the SCTP content to clients advertising
  JSON-encoded sources; fixed in 003 T055/T056. It never blocked RTP.

## R12. Receive cap (FR-015, added after T038/T039)

- **Decision**: a test plan field `media.max_received_streams` (per media
  kind, 0 = no cap, default 0). The peer exposes it to the adapters through
  `SignalingHandler.ReceiveLimit()`. The Jitsi adapter gives an SDP section
  to the first N remote sources of each kind, in announcement order, and
  keeps the others waiting; when an active source is removed (source-remove
  or its owner leaving), the oldest waiting one of that kind gets a new
  section. The bridge's own sources are not counted.
- **Dropping the ignored streams early**: the bridge keeps sending them, and
  Pion v3 keeps a stream for every SSRC it sees, with a 1 MB buffer, and
  logs an error for each (`Incoming unhandled RTP ssrc`). The peer therefore
  sets Pion's `BufferFactory`: for an SSRC that is not in the last applied
  offer, the buffer drops written packets and its reads fail at once (so
  Pion's probe of the stream ends immediately); once the SSRC is offered
  (a promoted source), the same buffer behaves as a normal 1 MB buffer.
  Pion's logs go to the structured logger (errors as warnings, the
  expected "unhandled SSRC" error as debug), as the constitution requires
  structured logging.
- **What remains per ignored packet**: the receive system call and SRTP
  decryption, which happen before Pion knows the SSRC is unwanted. The cap
  bounds the processing, not the network traffic.
- **Alternatives considered**: asking the bridge not to send (Jitsi's
  receiver constraints and audio subscription messages need the bridge
  channel, which the bots only got later with 003 T056, and last-N does
  not apply to audio; see feature 005); capping in the media layer by rewriting the offer
  (subscription is platform-specific: other platforms subscribe through
  their own signaling, so the adapter owns it).
- **Measured (T050)**: 100 bots, cap 10, 10 min: 2.60 cores (from 3.85),
  723 MB (from 3.25 GB), send lateness within SC-003 (99.9% under 20 ms).
  The rest of the cost is per packet the bridge sends, before it can be
  dropped (system calls, ICE/mux, SRTP); SC-004's 2 cores needs the bridge
  to send fewer streams (subscription over the bridge channel: available
  since 003 T056, the Colibri messages are feature 005).
