# Feature Specification: Video Latency on Servers That Rewrite Timestamps

**Feature Branch**: `012-video-frame-latency` (work continues on `rework`)

**Created**: 2026-11-06

**Status**: Draft

**Input**: User description: "I want video latency on jitsi simulcast and janus", scoped as: "012 content-based video latency: measure video end-to-end latency on platforms that rewrite RTP timestamps, specifically Jitsi with simulcast and Janus VideoRoom, where reports today say latency_reason frame_not_identified. Frames must be identified by something that survives the server's timestamp rewriting (for example a marker carried in the media content or RTP payload) instead of the RTP timestamp alone. Keep reporting latency_available false with a reason rather than a wrong value when frames cannot be identified. Existing RTP-timestamp matching on Jitsi without simulcast, LiveKit and mediasoup must not regress, and measurement must stay deterministic and not add meaningful engine CPU at 100 bots. Janus audio latency is also frame_not_identified today; the user asked for video, so audio stays out unless the same mechanism covers it cheaply."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Video latency on Janus (Priority: P1)

As a load test operator, I want a Janus run to report video latency, so that the four media servers can be compared on latency under the same load, not only on freezes, frame rate and CPU.

**Why this priority**: Janus restarts the timestamps of every forwarded stream, so it never reports latency today, in any test plan. It is the only platform with no video latency at all, and comparing the servers is the project's goal.

**Independent Test**: Start the local Janus server, run 10 bots with `audio_video` for 1 minute, and read the quality summary: video latency is available with its percentiles, and no reason is given.

**Acceptance Scenarios**:

1. **Given** a Janus run with profile `audio_video`, **When** it ends, **Then** the quality summary and report files give video latency percentiles for the bots that received video.
2. **Given** a Janus run with a receive cap, **When** a bot receives a stream promoted during the run, **Then** that stream's frames count towards the bot's video latency too.
3. **Given** a Janus run, **When** it ends, **Then** audio latency is still left out with reason `frame_not_identified`, as today.

---

### User Story 2 - Video latency on Jitsi with simulcast (Priority: P2)

As a load test operator, I want a Jitsi run with simulcast to report video latency, so that a test of the bridge's most realistic mode does not lose a measure that the same run has without simulcast.

**Why this priority**: The measure already exists on Jitsi without simulcast; this story extends it to the mode closest to real meetings, where the bridge chooses which layer each bot receives.

**Independent Test**: Run 10 Jitsi bots with simulcast at 720p and a receive cap for 1 minute, and read the quality summary: video latency is available, and it covers the layers the bots received.

**Acceptance Scenarios**:

1. **Given** a Jitsi run with simulcast, **When** it ends, **Then** the quality summary and report files give video latency percentiles for the bots that received video.
2. **Given** a bot whose received layer changes during the run, **When** frames arrive from each layer, **Then** frames of every layer count towards its latency, with no jump caused by the switch itself.
3. **Given** a Jitsi run with simulcast, **When** it ends, **Then** audio latency is measured as before.

---

### User Story 3 - Trustworthy figures everywhere (Priority: P3)

As a load test operator, I want the video latency to stay correct on every platform, and to be left out with its reason whenever it cannot be measured, so that I can put the four reports side by side without wondering which figures to believe.

**Why this priority**: A wrong latency is worse than none. The new measure must not disturb what already works, nor the media the servers and any human viewers receive.

**Independent Test**: Run the same 10-bot plan on the four platforms, and compare with runs from before this feature: the other quality measures do not move, and a browser in the room sees normal video. Separately, automated tests with a known added delay check the value, and tests with unidentifiable frames check that latency is left out.

**Acceptance Scenarios**:

1. **Given** a network path with a known added delay, **When** a run measures video latency, **Then** the median matches that delay, whichever way the server treats timestamps.
2. **Given** frames that cannot be identified, **When** the summary is built, **Then** video latency is left out with reason `frame_not_identified`, never reported as zero or as a guess.
3. **Given** a person watching the bots in a browser on the platform's own web client, **When** the bots send video, **Then** the picture looks as it did before this feature.
4. **Given** a Jitsi run without simulcast, a LiveKit run or a mediasoup run, **When** it ends, **Then** freezes, frame rate, resolution and audio measures are unchanged, and video latency is still available.

---

### Edge Cases

- **A frame arrives after the send time memory has moved on** (more than 10 s late): it is not counted, as today.
- **The content loops**: the same picture sent again later must never match its earlier send time.
- **Media from a publisher that is not a bot of this scenario** (a browser in the room, another run in the same room, Janus's idle placeholder publisher): it never produces a latency value; a bot receiving only such media reports the same reason as today.
- **A frame lost in part**: a frame whose identifying information did not arrive is not counted. It does not fail the bot.
- **Frames sent from another machine**: latency stays left out with reason `not_measured`, as today.
- **A server that alters frames' content**, or a transcoding server: frames are not identified, and latency is left out with its reason.
- **File-based media (`--media-file`)**: video latency works as with the built-in content.
- **Video that a server slows or thins under load**: only the frames received are counted, and latency reflects what they went through.
- **Very long runs**: frame identification keeps working for the whole run, with no wrong match when counters wrap.

## Requirements *(mandatory)*

### Functional Requirements

#### Measuring (User Stories 1 and 2)

- **FR-001**: The system MUST identify each received video frame in a way that survives a server rewriting RTP timestamps, sequence numbers and packetization metadata, and MUST measure its latency from that frame's recorded send time.
- **FR-002**: Identification MUST NOT depend on the platform: the media layer MUST NOT need to know which server it is talking to, and no signaling adapter may be involved.
- **FR-003**: Video latency MUST be available on Janus and on Jitsi with simulcast for bots that received identifiable video.
- **FR-004**: With simulcast, frames of every layer MUST be identifiable, and a bot's latency MUST cover all the layers it received.
- **FR-005**: A platform that also rewrites timestamps (LiveKit video, in some runs) MUST gain the same measure, with no platform-specific work.
- **FR-006**: The identity of a frame MUST stay unique for at least as long as send times are remembered, across content loops and for the whole run, and MUST NOT match frames from another scenario.

#### Reporting (User Story 3)

- **FR-007**: Video latency MUST keep the same fields in the quality summary, report files and Prometheus series; no field may be added, renamed or removed.
- **FR-008**: When no received frame could be identified, video latency MUST be left out with reason `frame_not_identified`, and with `not_measured` when there are no send times, as today. It MUST NOT be reported as zero or estimated.

#### No side effects (User Story 3)

- **FR-009**: The sent video MUST stay valid for the servers and for browsers: bots MUST keep being forwarded, and viewers MUST see no visible change.
- **FR-010**: Media MUST stay deterministic: the same test plan MUST send the same content, apart from what identifies frames.
- **FR-011**: Measuring MUST NOT need real-time encoding or decoding of video.
- **FR-012**: Audio latency MUST stay as it is on every platform. Measuring audio on Janus is out of scope.
- **FR-013**: Test plans, their validation and the options they accept MUST NOT change; the measure is always on, as latency is today.

### Key Entities

- **Frame identity**: what lets a receiving bot tell which sent video frame it is holding; unique among a scenario's recent frames, and readable without decoding the picture.
- **Send time record**: the existing per-scenario memory of when each recent frame was sent, now reachable through the frame identity.
- **Video latency**: the existing per-kind quality measure (percentiles, availability, reason); its form does not change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: In a 10-bot, `audio_video` run on the local Janus server, every bot that received video contributes to video latency, and the summary gives percentiles.
- **SC-002**: In a 10-bot run on the local Jitsi server with simulcast at 720p, every bot that received video contributes to video latency, and the summary gives percentiles.
- **SC-003**: With a known 50 ms added delay, the median video latency is within 5 ms of that delay, as for 006 (SC-004), including when the path rewrites timestamps.
- **SC-004**: In automated tests, frames that cannot be identified, looped content and frames from another scenario produce no latency value in 100% of cases.
- **SC-005**: On Jitsi without simulcast, LiveKit and mediasoup at 50 bots, video latency stays available wherever it was, and median latency stays within 2 ms of an alternated run from before this feature; freezes, frame rate and MOS stay within run-to-run variation.
- **SC-006**: At 100 bots, the engine's CPU grows by no more than 5% against an alternated run from before this feature, taken in the same machine state.
- **SC-007**: The bitrate of the lowest simulcast layer grows by no more than 2%.
- **SC-008**: A person watching for 1 minute in a browser on Jitsi and on Janus sees no artifacts or errors that were not there before this feature.

## Assumptions

- Jitsi's bridge and Janus forward the video content of each frame unchanged, while they rewrite what surrounds it. The plan must verify this on both servers before the design is settled; if one of them does not, the spec is revisited.
- The bots' send and receive sides share one clock, as for the 006 latency: all bots of a run are on one machine. Distribution across machines stays out of scope.
- LiveKit and mediasoup gaining or keeping the measure comes from the same mechanism; they are verified, but are not the reason for this feature.
- Janus simulcast stays out of scope: the 011 rule that rejects it at test plan creation does not change.
- Audio latency on Janus stays out of scope: the user asked for video, and audio frames are too small and too frequent for the same approach to be obviously cheap.
- Browser checks use `localhost`, and the local servers in `tests/platforms/`.
- Depends on the latency measure of 006, the quality report of 009 and the local setups of 008, 010 and 011.
