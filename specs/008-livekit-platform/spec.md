# Feature Specification: LiveKit Platform

**Feature Branch**: `008-livekit-platform` (work continues on `rework`)

**Created**: 2026-10-17

**Status**: Draft

**Input**: User description: empty `/speckit-specify`; the description is the scope agreed on 2026-10-16 for the second platform: "LiveKit support (008). Run the same test plan that runs against Jitsi against a LiveKit server, and get the same report: media quality (MOS, freezes, frame rate, audio gaps, latency) from 006 and server load from 007. First version: audio and video with the receive cap; simulcast and requested heights are rejected when the test plan is created. The LiveKit server runs as a local Docker container; its setup is added under tests/platforms. One report per run and per server; no cross-platform comparison report. Order: 007 server load, 008 LiveKit, 009 mediasoup-demo."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a test plan against LiveKit (Priority: P1)

As a load test operator, I want to point an existing test plan at a LiveKit server instead of Jitsi, changing only the platform and its connection settings, so that the bots join a LiveKit room, send audio and video, receive each other's media and leave at the end of the run, as they do on Jitsi.

**Why this priority**: Without bots that join and exchange media, nothing else about LiveKit can be measured. This story alone already gives a load test of a LiveKit server (traffic, connection and media metrics of 002 to 004).

**Independent Test**: Start the local LiveKit server, create a scenario with platform `livekit`, 10 bots, `audio_video`, for 1 minute, and run it: all bots join the same room, every bot receives audio and video from the others, the room is visible with 10 participants on the server side, and all bots have left once the run ends.

**Acceptance Scenarios**:

1. **Given** a running LiveKit server and a test plan with platform `livekit` and its connection settings, **When** the scenario is started, **Then** each bot joins the named room as its own participant, ramped up as the test plan says, and the scenario reports the bots as connected.
2. **Given** joined bots with profile `audio_video` (or `audio`), **When** they run, **Then** each bot sends its audio (and video) to the room, and each bot receives the media of the other bots.
3. **Given** a receive cap of N streams per kind, **When** more than N other bots send, **Then** each bot receives at most N streams per kind, and when one of them ends, a waiting one takes its place, as on Jitsi.
4. **Given** profile `receive_only`, **When** the bots run, **Then** they send nothing and receive the media of other participants in the room (for example a browser).
5. **Given** a running scenario, **When** it reaches its duration or is stopped, **Then** every bot leaves the room and the server shows none of them left behind.
6. **Given** a LiveKit server that requires authentication (it always does), **When** bots join, **Then** each bot presents its own access token, with its peer ID as identity, made by the engine from the key and secret in the connection settings, or a ready-made token given in the settings.

---

### User Story 2 - The same report for LiveKit as for Jitsi (Priority: P2)

As a load test operator, I want a LiveKit run to produce the same end-of-run report as a Jitsi run (media quality, worst bots, server load and the load machine's own load), in the same form, so that I can put the reports of the two platforms side by side for the same test plan and compare MOS, freezes, media issues and server CPU.

**Why this priority**: Comparing platforms under the same load is the user's goal. The measures already exist (006, 007); this story makes sure they hold on LiveKit, where the server may change the media it forwards differently from the Jitsi bridge.

**Independent Test**: Run the same 10-bot, 1-minute test plan on Jitsi and on LiveKit, with server monitoring naming each platform's containers, and read both quality summaries: both have the same sections and fields; on an idle machine both show no freezes, no audio gaps and a high audio score, and the LiveKit server's CPU and memory are reported.

**Acceptance Scenarios**:

1. **Given** a finished LiveKit scenario, **When** the operator asks the API for its quality summary, **Then** it has the same sections and fields as a Jitsi summary: received frame rate, resolution, freezes, keyframe requests, audio gaps, audio score, latency, worst bots, server load and load machine.
2. **Given** a test plan whose server monitoring names the LiveKit server's container, **When** the run ends, **Then** the server section reports that container's CPU and memory as for Jitsi.
3. **Given** a measure that cannot be taken on LiveKit (for example latency, if the server changed what identifies a frame), **When** the summary is built, **Then** that measure is left out and the summary says why, rather than reporting zero or a wrong value; the other measures are still reported.
4. **Given** the run's Prometheus series, **When** the operator reads them for a LiveKit scenario, **Then** they are the same series, with the same labels, as for a Jitsi scenario.

---

### User Story 3 - A local LiveKit server to test against (Priority: P3)

As a load test operator or developer, I want a LiveKit server setup in the repository that starts with one command on the load machine, as the Jitsi one does, so that any LiveKit result can be reproduced and compared with the Jitsi results taken on the same machine.

**Why this priority**: The first two stories can be developed against any LiveKit server; a pinned, documented local setup makes the results repeatable and the comparison fair. It is small but needed before the scale runs.

**Independent Test**: On a machine with Docker, start the setup with the documented command, run the 10-bot scenario of User Story 1 against it with the documented settings, then stop the setup; nothing else needs installing.

**Acceptance Scenarios**:

1. **Given** the repository and Docker, **When** the operator starts the LiveKit setup as documented, **Then** a LiveKit server of a pinned version runs locally, reachable by the bots on the same machine, with its key and secret coming from a local file that is not committed.
2. **Given** the running setup, **When** the operator reads its documentation, **Then** it gives the platform settings and the server-monitoring container names to put in a test plan.
3. **Given** the setup, **When** a 100-bot scenario (`audio_video`, receive cap 10) runs against it, **Then** the run completes and its report is recorded next to the Jitsi report of the same test plan.

---

### Edge Cases

- **Options this version does not support on LiveKit** (simulcast, requested heights): the scenario is rejected when it is created, with a message naming the option and the platform; it never starts and silently ignores them.
- **Missing or invalid connection settings** (no server address, no room, neither a key and secret nor a token, a key without a secret): the scenario is rejected at creation and the message names the setting.
- **Wrong key or secret, expired token, unreachable server**: the bot fails to join and is reported as failed with the reason, as a failed Jitsi join is; the secret never appears in logs or in the API.
- **The server closes a bot's connection during the run** (server restart, participant removed): the bot is reported as lost, as on Jitsi, and the run goes on for the other bots.
- **The room does not exist yet**: the first bot's join creates it; the bots do not need it created beforehand.
- **Two bots with the same peer ID in one room** (two scenarios using the same room and IDs): the server keeps only one; this is surfaced as a lost bot, not hidden. Peer IDs are already unique within a scenario.
- **The server sends media the bots did not ask for** (participants that are not bots, more streams than the cap): the cap still applies to what each bot receives.
- **The server lowers the received video quality under load**: the bots report what they actually receive, as for Jitsi.
- **A Jitsi test plan without LiveKit settings**: it runs on Jitsi exactly as before this feature.

## Requirements *(mandatory)*

### Functional Requirements

#### Joining and media (User Story 1)

- **FR-001**: The test plan's platform MUST accept `livekit` in addition to `jitsi`; any other value MUST still be rejected at creation.
- **FR-002**: The LiveKit connection settings MUST include the server address and the room name, and either a key and secret from which the engine makes one access token per bot, or a ready-made token shared by all bots. Missing, conflicting or malformed settings MUST be rejected at creation with a message naming the setting.
- **FR-003**: Each bot MUST join the room as its own participant, with its peer ID as its identity, and MUST be counted as connected only once it can send and receive media.
- **FR-004**: Bots MUST send what the test plan's media profile says (`audio`, `audio_video` or nothing for `receive_only`), with the same content, codecs and rates as on Jitsi.
- **FR-005**: Each bot MUST receive the audio and video of other participants up to the receive cap per kind, and MUST replace an ended stream by a waiting one, as on Jitsi.
- **FR-006**: At the end of the run, or when it is stopped, every bot MUST leave the room and release everything it opened, within the same time as on Jitsi.
- **FR-007**: A failed join, a lost connection and a disconnection MUST be reported through the same lifecycle states, metrics and logs as for Jitsi, with the platform's reason.
- **FR-008**: Test plans for `livekit` that enable simulcast or requested heights MUST be rejected at creation, with a message saying these options are not supported on LiveKit in this version.
- **FR-009**: The secret and the tokens MUST never appear in logs, in API answers or in metrics.

#### Report (User Story 2)

- **FR-010**: A LiveKit run's quality summary and Prometheus series MUST have the same sections, fields and labels as a Jitsi run's; nothing in them may depend on the platform except their values.
- **FR-011**: Each quality measure of 006 MUST be computed for LiveKit from what the bots receive, as for Jitsi; a measure that cannot be computed on LiveKit MUST be left out with its reason, never reported as zero.
- **FR-012**: Server monitoring (007) MUST work for LiveKit by naming the LiveKit server's containers in the test plan, without any LiveKit-specific setting.

#### Local setup (User Story 3)

- **FR-013**: The repository MUST contain, under `tests/platforms`, a LiveKit setup that starts a pinned LiveKit server version locally with one documented command, and its documentation (platform settings, container names, how to stop it).
- **FR-014**: The setup's key and secret MUST come from a local file that is excluded from version control, with an example file committed instead.

### Key Entities

- **LiveKit connection settings**: the platform part of a test plan for `livekit`: server address, room name, and either key and secret or a ready-made token; optionally the token's lifetime. Validated at creation; the secret is write-only.
- **Bot participant**: a bot as LiveKit sees it: one participant per bot in the room, identified by the bot's peer ID, publishing the bot's tracks and subscribed to at most the receive cap of other participants' tracks per kind.
- **Local LiveKit setup**: the pinned server version, its configuration, its secrets file (not committed), its container names for server monitoring.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A test plan that runs on Jitsi runs on LiveKit by changing only the platform and its connection settings: 10 bots `audio_video` for 1 minute all join, each receives 9 audio and 9 video streams, and all leave within 10 seconds of the end.
- **SC-002**: On the local setup, 100 bots `audio_video` with receive cap 10 run for 10 minutes against LiveKit with every bot connected for the whole hold, and the load machine's engine stays within the 2-core budget set for Jitsi in 005 and 006.
- **SC-003**: For the 10-bot run on an otherwise idle machine, the LiveKit summary shows no freezes, audio gaps below 1% and a median audio score of at least 4.0, as the Jitsi run of the same plan does.
- **SC-004**: The LiveKit and Jitsi summaries of the same test plan have exactly the same set of fields, so the two reports can be read side by side without any conversion.
- **SC-005**: 100% of test plans for LiveKit with simulcast, requested heights or incomplete settings are rejected at creation, before any bot starts.
- **SC-006**: From a clean checkout with Docker, an operator starts the local LiveKit setup and runs the 10-bot scenario in under 10 minutes, following only the repository's documentation.

## Assumptions

- The LiveKit server is open source LiveKit, self-hosted, run from its official Docker image with one server process and no external database (single-node mode); multi-node, LiveKit Cloud and its egress/ingress services are out of scope.
- The bots and the server run on the same machine, as for 007's server monitoring; a server on another machine may work for joining, but its server load is not measured (007's scope).
- LiveKit always requires an access token; making one per bot from the key and secret is the default, as for Jitsi's JWT support (003), with a ready-made token as the alternative.
- Bots send the same synthetic content and codecs as on Jitsi (Opus audio, VP8 video at one resolution), which LiveKit supports; no new media content is needed.
- One report per run, as for Jitsi; comparing the two platforms is done by reading the two reports, and no comparison report is produced (agreed 2026-10-16).
- Simulcast and requested heights on LiveKit, LiveKit's data messages, screen sharing and other track types are out of scope for this version and may come in a later feature.
- The latency measure of 006 identifies a frame by its RTP timestamp; if LiveKit rewrites video timestamps, the fallback described in 006 research R5 or leaving video latency out with its reason (FR-011) is acceptable for this version.
- mediasoup-demo support is feature 009 and is not part of this feature, but nothing in this feature should make it harder (the platform boundary stays generic).
