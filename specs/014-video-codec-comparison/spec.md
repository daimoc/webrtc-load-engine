# Feature Specification: Video Codec Comparison

**Feature Branch**: `014-video-codec-comparison` (work continues on `rework`)

**Created**: 2026-11-07

**Status**: Draft

**Input**: User description: "Now I want the test run on difference codec vp8, vp9 av1 and h264 to see if there is some impact on sfu"

## Clarifications

### Session 2026-11-07

- Q: How are the four codecs compared? → A: One run per codec; the reports are read side by side, and the validation report gathers them in a table. No comparison runner or new report format.
- Q: Simulcast and requested heights with codecs other than VP8? → A: Every codec has layers: VP8 and H.264 as simulcast, VP9 and AV1 as scalable layers within one stream, as real clients send them.
- Q: Bitrate of each codec? → A: Each codec at its usual bitrate for the same picture quality as the VP8 content, so a codec's lower bitrate counts in its favour; the bitrate of each is stated with the results.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Choose the video codec of a run (Priority: P1)

As a load test operator, I want to say in the test plan which video codec the bots send, VP8, VP9, AV1 or H.264, so that I can run the same load on a media server with each codec and see what the codec changes for that server.

**Why this priority**: Every run today sends VP8 only. Without a codec choice nothing else in this feature can be measured, and one run per codec already answers the question by reading the reports side by side.

**Independent Test**: On the local LiveKit server, run 10 bots with `audio_video` for 1 minute once per codec. Each run passes, and each report says which codec the bots sent and which codec they received.

**Acceptance Scenarios**:

1. **Given** a test plan with `audio_video` and codec VP9, **When** the run ends, **Then** every bot sent VP9, every bot received VP9 from the others, and the report says VP9.
2. **Given** a test plan with no codec, **When** it runs, **Then** the bots send VP8 as today, and the report says VP8.
3. **Given** a test plan with a codec that is not one of the four, **When** it is created, **Then** it is refused, naming the four accepted codecs.
4. **Given** a codec the media server does not accept, or does not forward as sent, **When** the run starts, **Then** the bots fail with a reason naming the codec and the server, and the run never reports results for another codec in its place.

---

### User Story 2 - The same measures for every codec (Priority: P2)

As a load test operator, I want each codec's run to report the same measures as a VP8 run today (audio MOS, video freezes, frame rate, resolution, video latency, server CPU and memory), so that differences between the reports come from the codec and not from a measure that is missing for one codec.

**Why this priority**: A codec run without freezes or latency cannot be compared with the VP8 run. Server CPU alone would come from US1; this story makes the comparison complete.

**Independent Test**: Run 10 bots for 1 minute with each codec on one server, and check that each report gives freezes, frame rate, resolution and video latency, or gives a named reason where a measure cannot be taken, never a zero in its place.

**Acceptance Scenarios**:

1. **Given** a run with any of the four codecs, **When** it ends, **Then** video freezes, frame rate and resolution are measured as for VP8.
2. **Given** a run with any of the four codecs, **When** it ends, **Then** video latency is given, or left out with a reason, under the same rules as VP8 on that server.
3. **Given** the same media file sent as each codec, **When** a browser joins the room, **Then** it shows the video without artefacts, so the bots' media is media a real client could use.

---

### User Story 3 - Layered video in every codec (Priority: P3)

As a load test operator, I want bots to send layered video in every codec, as real clients do: VP8 and H.264 as three simulcast streams, VP9 and AV1 as one stream with scalable layers. Then the server's work of choosing a layer for each receiver is part of the codec comparison.

**Why this priority**: Real meetings send layers, and choosing layers is a large part of a media server's work, so a single-stream comparison leaves part of the codec's effect unseen. It comes after US1 and US2, which already compare the codecs as single streams, and it is the largest piece of the feature.

**Independent Test**: On one server, run 10 bots for 1 minute with layers and a receive cap, once per codec, with some bots asking for a lower height. Each run passes, the bots asking for a lower height receive it, and the report gives the heights received.

**Acceptance Scenarios**:

1. **Given** a run with layers in VP8 or H.264, **When** it ends, **Then** every bot sent three simulcast streams, as VP8 bots do today.
2. **Given** a run with layers in VP9 or AV1, **When** it ends, **Then** every bot sent one stream with spatial and temporal layers, and the server forwarded to each receiver the layers it chose.
3. **Given** a layered run where some bots ask for a lower height, **When** it ends, **Then** those bots received that height, whatever the codec.
4. **Given** a server that cannot forward a codec's layers, **When** the run starts or ends, **Then** the report says so, naming the codec and the server, instead of reporting a layered result.
5. **Given** a layered run in any codec, **When** a received layer changes, **Then** freezes, frame rate, resolution and latency keep being measured across the change, as for VP8 simulcast today.

---

### User Story 4 - The codecs side by side (Priority: P4)

As a load test operator, I want the four codecs' results on each server gathered in the validation report, so that I see whether a codec changes the server's CPU, memory or the media quality without opening sixteen reports.

**Why this priority**: The runs of US1 to US3 already answer the question one report at a time; this story only gathers them, with the conditions that make them comparable.

**Independent Test**: For one server, the validation report gives, per codec at 10 bots, server CPU and memory, MOS, freezes, latency and the video bitrate, with the bot count, duration and receive cap of the runs.

**Acceptance Scenarios**:

1. **Given** the four codec runs on one server, **When** the validation report is read, **Then** each codec's server CPU and memory, MOS, freezes, latency and bitrate are given for the same bot count, duration and receive cap.
2. **Given** runs from different days, machines or conditions, **When** they are gathered, **Then** the difference is stated and they are not presented as like for like.

---

### Edge Cases

- **A server negotiates another codec** than the one asked for (it falls back to VP8, for example): the run fails with a reason, rather than measuring the wrong codec.
- **A server accepts a codec but some bots receive another** (it transcodes, or a receiver negotiates another codec): the bot reports the codec it received, and the run fails naming the bots concerned.
- **H.264 profile**: servers and browsers support different H.264 profiles. The bots send the profile most widely supported by servers and browsers (Assumptions); a server refusing it is the case above.
- **Layers and requested heights in each codec**: VP8 and H.264 send simulcast, VP9 and AV1 send scalable layers (US3). A server that forwards a codec but not its layers is reported as such, not run as a single stream in its place.
- **A layered stream loses its upper layers on the way** (the server or a receive cap drops them): the receiver's height drops, and this is measured as a layer change, not as a freeze.
- **A media source file** (`source_file`): it is prepared in the run's codec, as it is in VP8 today.
- **One room, one codec**: every bot of a run sends the same codec. Mixing codecs in one room is out of scope.
- **A codec costs the bots more** than VP8 (larger content or more work per packet): the engine's CPU per bot is reported, so a server difference is not mistaken for a bot limit.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A test plan MUST be able to name the video codec the bots send: VP8, VP9, AV1 or H.264. With no codec named, the bots MUST send VP8, so that existing test plans and their results do not change.
- **FR-002**: Test plan creation MUST refuse a codec outside these four, naming the accepted ones, and MUST refuse a codec with a profile that sends no video.
- **FR-003**: Every bot MUST send video in the codec named, on each of the four platforms (Jitsi, LiveKit, mediasoup, Janus), using the platform's normal way to choose the codec.
- **FR-004**: Each bot MUST check that the codec it sends and the codec it receives are the codec named. When they are not, the bot MUST fail with a reason naming the codec found, and the run MUST NOT report results as the named codec.
- **FR-005**: The built-in video content MUST exist in each codec, made from the same source, at the same resolution, frame rate and duration as the VP8 content, and at the same picture quality as the VP8 content, judged by one objective quality score for all codecs, within a stated tolerance. Each codec's bitrate is then what that quality costs it, and MUST be recorded with the content.
- **FR-006**: Bots MUST NOT encode video while a run is under way: the content in each codec is prepared before the run, as VP8 is today, so that adding codecs does not change how many bots one machine can run.
- **FR-007**: Video freezes, frame rate, resolution and video latency MUST be measured for every codec, as they are for VP8. Where a measure cannot be taken for a codec on a server, the report MUST leave it out with a named reason, never a zero.
- **FR-008**: The frame identification used for video latency (012) MUST keep the video decodable by a browser in every codec.
- **FR-009**: The run report MUST say which codec the bots sent, and which codec they received, in both the JSON and Markdown files.
- **FR-010**: A media source file (`source_file`) MUST be prepared in the run's codec.
- **FR-011**: A VP8 run MUST give the same results as before this feature, within the run-to-run variation seen today.
- **FR-012**: The engine's CPU per bot MUST stay within 10% of VP8's for every codec at 100 bots, or the difference MUST be measured and stated in the documentation.
- **FR-013**: Bots MUST be able to send layered video in every codec: VP8 and H.264 as three simulcast streams, as VP8 does today; VP9 and AV1 as one stream with spatial and temporal layers. Each layer's content MUST be prepared before the run (FR-006), at the same quality rule as FR-005.
- **FR-014**: Requested heights MUST work with every codec's layers, and the receive cap MUST apply to layered streams of every codec as it does to VP8 simulcast.
- **FR-015**: Freezes, frame rate, resolution and video latency MUST be measured across layer changes for every codec, as for VP8 simulcast (006, 012).
- **FR-016**: A layered run MUST report, per bot, the heights it received, and a server that does not forward a codec's layers MUST be named with the codec, not reported as a layered result.
- **FR-017**: The local setups in `tests/platforms` and the remote deployments (013) MUST accept the four codecs, or the documentation MUST name the codec a server does not support and why.

### Key Entities

- **Video codec**: the codec named in the test plan, one of VP8, VP9, AV1 and H.264; it decides which content the bots send and which codec they negotiate.
- **Video content**: the pre-encoded clip each bot sends, one per codec and per layer arrangement (single stream, simulcast, or scalable layers), all made from the same source with the same resolution, frame rate and duration, and the same picture quality; each records its bitrate.
- **Codec result**: what a run reports about codecs: the codec sent, the codec each bot received, and the bots whose codec was not the named one.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A 10-bot, 1-minute `audio_video` run passes for each of the four codecs on each of the four servers, or the documentation names the server and codec that cannot run and the server's reason (16 combinations, each run or explained).
- **SC-002**: In 100% of runs, the report names the codec sent and received, and a run whose negotiated codec differs from the named one fails rather than passing.
- **SC-003**: For every codec, the report gives freezes, frame rate, resolution and video latency, or a named reason for each one missing; no measure is missing for a codec where it is available for VP8 on the same server without a reason given.
- **SC-004**: A browser joining a room shows each codec's video with no visible artefact.
- **SC-005**: For each server, the validation report gives the four codecs' results at 10 bots, single stream and layered, with each codec's bitrate, and says whether each codec changes server CPU by more than run-to-run variation (measured by two VP8 runs on the same day).
- **SC-006**: The engine's CPU per bot at 100 bots is within 10% of VP8's for each codec, or its difference is measured and documented.
- **SC-007**: Existing VP8 test plans run unchanged and give the same results as before, within today's run-to-run variation.
- **SC-008**: With layers and requested heights, bots asking for a lower height receive it in at least 95% of their measured time, in every codec on every server that forwards the codec's layers.
- **SC-009**: Each codec's content scores within the stated tolerance of the VP8 content on the one objective quality score, at every layer.

## Assumptions

- The four servers are the ones already supported: Jitsi, LiveKit, mediasoup and Janus, local in `tests/platforms` or deployed with `sfu-remote` (013). Comparisons across days or machines keep the existing rule: server CPU is only compared within one set of runs on one machine.
- H.264 is sent in the profile most widely accepted by servers and browsers (Constrained Baseline). Other profiles are out of scope.
- Layered VP9 and AV1 use three spatial layers matching today's simulcast heights (90, 180 and 360, or 180, 360 and 720 with a 720 maximum) and temporal layers, the arrangement common in browsers; the exact arrangement is a planning choice.
- Equal picture quality is judged on the built-in content only. A `source_file` is prepared at each codec's quality-matched settings for the built-in content, without a new quality measurement per file.
- The comparison is one run per codec, gathered in the validation report (Clarifications): no new comparison runner or report format.
- Resolution, frame rate and duration of the content stay those of today's VP8 content (640x360 for a single stream), so that only the codec changes between runs.
- Audio stays Opus for every run.
- A server that cannot forward a codec is reported, not worked around: the feature does not change servers' code, only their configuration where a codec must be enabled.
- One codec per room: mixed-codec rooms, transcoding by the server and codec switching during a run are out of scope.
