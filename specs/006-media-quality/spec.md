# Feature Specification: Media Quality

**Feature Branch**: `006-media-quality` (work continues on `rework`)

**Created**: 2026-10-11

**Status**: Draft (clarified 2026-10-11: highest resolution with several resolutions is configurable, 360 lines by default; amended 2026-10-11 in planning: FR-009, the send time is identified from the frame rather than carried in it, research R5)

**Input**: User description: "Media quality: measure the quality of the media each bot receives, so a load test shows when the platform degrades and not only how much traffic it carries. Three user stories, in priority order. (1) Per-receiver quality, without changing what the bots send: video frame rate, freezes, received resolution and keyframe requests; audio gaps (concealment); an estimated audio MOS from loss, jitter and delay (E-model); all built on the existing media metrics of 004 (loss, jitter, round trip, stalls). (2) End-to-end latency: time from a bot sending a frame to other bots receiving it through the media server, measurable when the bots share a clock (same machine). (3) Simulcast: bots send several video resolutions and receivers ask for different heights, so the media server's layer selection and switching are part of the load and visible in the quality metrics. Reporting: Prometheus series per scenario (labelled by scenario only, distributions as histograms) plus an end-of-run quality summary per scenario (percentiles, worst bots) available through the API. Must keep 100 bots, audio and video, receive cap 10, within the 2-core budget (1.05 cores measured in 005). Validate on the local docker-jitsi-meet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quality of what each bot receives (Priority: P1)

As a load test operator, I want each scenario to report the quality of the audio and video every bot receives, as a participant would experience it (smooth or frozen video, at what frame rate and resolution, audio with gaps or not, an overall audio score), so that I can tell at which load the platform starts to degrade, and not only how much traffic it carries.

**Why this priority**: Today a run reports volumes, loss, jitter, round trip and stalls (004). A platform can deliver the expected volume while video freezes, frame rate drops or audio is cut, and the operator cannot see it. This story gives the main answer of a quality load test without changing what the bots send.

**Independent Test**: Run 10 sending bots for 1 minute on the local deployment and read the metrics and the end-of-run summary: received frame rate and resolution match what the bots send, no freezes, no audio gaps, and a high audio score; then interrupt one sender's video for a known time and see the freeze reported by every bot receiving it.

**Acceptance Scenarios**:

1. **Given** a running scenario where bots receive video, **When** the operator reads the metrics, **Then** they show, for the scenario, the distribution of received video frame rates, the received resolutions, the number and total duration of freezes, and the number of keyframe requests the bots sent.
2. **Given** a running scenario where bots receive audio, **When** the operator reads the metrics, **Then** they show the share of received audio that a listener would have heard as a gap (missing or too late), and the distribution of an estimated audio quality score (MOS, 1 to 5).
3. **Given** a received video stream that stops for longer than a freeze but shorter than a stall (004), **When** it resumes, **Then** one freeze is counted with its duration, and no stall.
4. **Given** a scenario that has ended, **When** the operator asks the API for its quality summary, **Then** it returns, per media kind, percentiles of each quality measure over all bots and the bots with the worst quality, identified by their peer IDs.
5. **Given** a scenario whose bots receive only audio, or receive nothing, **When** the summary is requested, **Then** it contains only the measures that apply and says so for the others.

---

### User Story 2 - End-to-end latency (Priority: P2)

As a load test operator, I want to know how long media takes from the moment a bot sends it to the moment another bot receives it through the media server, so that I can see the delay the platform adds as load grows, which loss and jitter alone do not show.

**Why this priority**: Delay is a direct part of perceived quality in a conversation and grows when the media server is overloaded, but it needs the sender and the receiver to agree on time, which is only guaranteed when they run on the same machine. It is valuable but comes after the receiver-side measures.

**Independent Test**: Run 10 sending bots on one machine for 1 minute; the latency distribution is reported for audio and video; in a controlled test where the path is delayed by a known amount, the reported latency reflects that delay.

**Acceptance Scenarios**:

1. **Given** sending and receiving bots running on the same machine, **When** a bot receives media from another bot, **Then** the time from sending to receiving is measured and added to the scenario's latency distribution, per media kind.
2. **Given** media the bots receive from a participant that is not a bot of the same machine (a browser, another load machine), **When** it is received, **Then** no latency is reported for it, and the quality measures of User Story 1 still apply.
3. **Given** a scenario that has ended, **When** the operator asks for its quality summary, **Then** it includes latency percentiles per media kind and the bots with the highest latency.
4. **Given** the estimated audio score of User Story 1, **When** end-to-end latency is known for a stream, **Then** the score uses it as its delay; otherwise it uses an estimate from the round trip.

---

### User Story 3 - Several video resolutions (simulcast) (Priority: P3)

As a load test operator, I want the bots to send their video in several resolutions at once and to ask for different resolutions as receivers, as real clients do in large meetings, so that the media server's work of choosing and switching resolutions per receiver is part of the load, and its effect on quality is visible.

**Why this priority**: Real conference clients send several resolutions and the server picks one per receiver; with a single resolution, a test misses this part of the server's work and cannot show resolution drops under load. It changes what the bots send and costs more, so it comes last.

**Independent Test**: Run 10 bots sending several resolutions, half of them asking for the highest and half for the lowest; each bot receives the resolution it asked for, and the metrics show the resolutions received and the switches between them.

**Acceptance Scenarios**:

1. **Given** a scenario with several video resolutions enabled, **When** bots send video, **Then** each sends every configured resolution at once.
2. **Given** receivers configured to ask for different resolutions, **When** they receive video, **Then** each receives the resolution it asked for, or a lower one if the media server decides so, and the metrics show the resolutions received.
3. **Given** the media server lowers or raises the resolution it sends to a receiver, **When** that happens, **Then** the switch is counted, and the time spent at each resolution appears in the summary.
4. **Given** a scenario without several resolutions enabled, **When** it runs, **Then** bots send and request a single resolution, as before this feature.

---

### Edge Cases

- **A stream starts in the middle of the run or is replaced** (receive cap, 004): its measures start with its first frame; the time before it is not counted as a freeze or a gap.
- **A freeze long enough to be a stall** (004): it is counted as a stall, and as a freeze from its start until the stall threshold, so totals stay consistent and nothing is counted twice as two freezes.
- **A sender leaves**: its stream's measures end with its last frame; the end is neither a freeze nor a gap.
- **Packets arrive out of order or duplicated**: they do not count as gaps or freezes when they arrive in time to be played.
- **A bot has no received stream of a kind** (cap 0 for that kind, a single participant): no score is produced for that kind, rather than a perfect or a zero score.
- **Clocks cannot be compared** (media from another machine, or a sender whose send time is unknown): latency is left out for that stream, never reported as zero or negative.
- **The media server ignores the requested resolution** (no several-resolution support, or limited bandwidth): the bots report what they actually receive; the request is not treated as an error.
- **Very large scenarios**: the summary lists a bounded number of worst bots (default 10) so that it stays readable at hundreds of bots.

## Requirements *(mandatory)*

### Functional Requirements

#### Receiver quality (User Story 1)

- **FR-001**: The engine MUST measure, for each received video stream, the frames received per second and report the distribution over the scenario's received streams.
- **FR-002**: The engine MUST detect video freezes: a gap between two received frames longer than three times the stream's usual frame interval and at least 150 ms. It MUST count freezes and their total duration per scenario.
- **FR-003**: The engine MUST report the resolution (height in lines) of the video frames each bot receives, as a distribution per scenario.
- **FR-004**: The engine MUST count, per scenario, the requests for a new full picture (keyframe) that the bots send to the media server, and those the bots receive as senders.
- **FR-005**: The engine MUST estimate, for each received audio stream, the share of audio a listener would not hear in time (missing packets, or packets arriving too late for a fixed playout delay), and report it per scenario.
- **FR-006**: The engine MUST compute, for each received audio stream at a regular interval, an estimated listening quality score (MOS, scale 1 to 5) from its loss, jitter and delay, following the published E-model method, and report the distribution per scenario.
- **FR-007**: Quality measures MUST be derived from what the bots receive (packet arrival, packet headers and the frame information the media carries) without decoding audio or rendering video.

#### End-to-end latency (User Story 2)

- **FR-008**: When the sender and the receiver are bots sharing a clock, the engine MUST measure the time between sending a media frame and receiving it, per media kind, and report the distribution per scenario.
- **FR-009**: The engine MUST identify each received frame's send time from what the media server forwards unchanged to a real client (for example the frame's media timestamp), without adding data the server would have to keep or forward; media whose send time cannot be identified (another clock, or an identity the server rewrote) MUST be left out of latency.

#### Several resolutions (User Story 3)

- **FR-010**: A scenario MUST be able to enable sending several video resolutions at once, as three resolutions whose highest is chosen per scenario: 360 lines (90, 180 and 360; the default) or 720 lines (180, 360 and 720, as the Jitsi web client sends).
- **FR-011**: A scenario MUST be able to set the resolution receivers ask for, either one value for all bots or a share of bots per resolution.
- **FR-012**: Bots MUST ask the media server for their configured resolution through the platform's usual mechanism (on Jitsi, the bridge channel of 005), and MUST keep receiving if the server sends another one.
- **FR-013**: The engine MUST count resolution switches of received streams and the time spent at each resolution, per scenario.

#### Reporting

- **FR-014**: All quality measures MUST be exported as Prometheus series labelled by scenario only (never by bot), distributions as histograms, next to the media metrics of 004.
- **FR-015**: At the end of a scenario, the engine MUST produce a quality summary, available through the HTTP API for that scenario: per media kind, percentiles (median, 95th, 99th) over the bots of each measure, totals (freezes, gaps, keyframe requests, switches) and the worst bots by score, freezes and latency (default 10, with their peer IDs).
- **FR-016**: The summary MUST stay available after the scenario ends, as long as the scenario itself is kept by the engine.
- **FR-017**: Quality problems MUST NOT fail a scenario or a bot on their own (as 004 FR-014); they are reported, and freezes are logged with the scenario and peer identifiers at most once per stream per minute.

#### Cost

- **FR-018**: Quality measurement MUST be on by default and MUST NOT require extra per-bot work proportional to the media content (no decoding, no image analysis), so that the 100-bot budget holds.

### Key Entities

- **Received Stream Quality**: per bot and received stream: frame rate, resolution, freezes, audio gaps, score, latency, resolution switches; updated during the run, folded into the scenario's metrics.
- **Quality Summary**: per scenario, built at the end of the run: percentiles per measure and media kind, totals, worst bots with peer IDs; read through the API.
- **Resolution Preference**: per scenario: whether bots send several resolutions, the highest one (360 or 720 lines), and the resolution each bot asks for (one value or shares).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With 10 bots sending audio and video for 1 minute on the local deployment, the summary reports a median received frame rate within 10% of the sent frame rate, the sent resolution, no freezes, an audio gap share under 1%, and a median audio score of at least 4.0.
- **SC-002**: When one sender's video is interrupted for 1 second in a controlled test, every bot receiving it reports exactly one freeze lasting between 0.9 and 1.2 seconds, and no stall.
- **SC-003**: For reference loss, jitter and delay values published with the E-model, the estimated score is within 0.1 of the reference score.
- **SC-004**: When the media path between bots is delayed by a known amount in a controlled test, the reported median latency differs from that amount by at most 5 ms; on the local deployment with 10 bots, latency is reported for 100% of the streams between bots.
- **SC-005**: With several resolutions enabled and 10 bots, each bot receives the resolution it asked for at least 95% of the time in a 1-minute run on the local deployment, and the summary shows the time spent per resolution.
- **SC-006**: 100 bots sending audio and video with a receive cap of 10, run for 10 minutes with quality measurement on, use no more than 2 CPU cores on average on the load machine, and no more than 20% more than the same run without it (1.05 cores in 005).
- **SC-008**: The same 100-bot run with several resolutions at the default highest resolution (360 lines) stays within 2 CPU cores on average; with 720 lines, the CPU used is measured and recorded (no budget).
- **SC-007**: An operator can find, from the summary alone and within 1 minute of the run's end, whether the run's quality was acceptable and which bots had the worst quality.

## Assumptions

- Quality is estimated from packet arrival and from the information each frame carries (frame boundaries, timestamps, keyframe headers with the picture size), not by decoding: decoding 1,000 streams would break the CPU budget (constitution IV). Audio gaps assume a fixed playout delay (default 60 ms), a common receiver setting.
- The audio score uses the E-model (ITU-T G.107) with the impairment factors published for the audio codec the bots send; it estimates listening quality and is not a perceptual measurement.
- Latency is only reported between bots of the same engine process, which share a clock; latency between load machines (shared clock across machines) is out of scope.
- Several resolutions use pre-encoded media like the single resolution of 004; producing the new embedded media (layers up to 360 and up to 720 lines) is part of this feature, and makes the binary larger.
- Network impairment injection (bandwidth limits, loss) is out of scope; the controlled tests of SC-002 and SC-004 use a test path inside the engine, not the network.
- The summary is kept in memory with the scenario (as scenarios are today); persistent storage is out of scope.
- Quality thresholds that pass or fail a run are out of scope; the summary gives the values and the operator decides.
- Validation runs on the local docker-jitsi-meet, as for 004 and 005; other platforms reuse the receiver measures without change, and the resolution requests through their own adapters.
