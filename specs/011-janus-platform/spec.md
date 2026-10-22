# Feature Specification: Janus Platform

**Feature Branch**: `011-janus-platform` (work continues on `rework`)

**Created**: 2026-10-22

**Status**: Draft

**Input**: User description: "Could you add tests for janus?", scoped as: "011-janus-platform: Janus WebRTC Server (VideoRoom plugin) as a fourth platform, so the same test plan runs against Jitsi, LiveKit, mediasoup-demo and Janus with one report per server. Scope like 008/010: a Janus adapter registered in the platform catalog, Janus 1.x multistream VideoRoom over the WebSocket transport, audio+video with receive cap only; simulcast and requested heights rejected at test plan creation. Reuses the 008 publish session and 010 consent checks; no new module if possible. A local Janus server from a pinned version in tests/platforms/janus, a live test with JANUS_URL/JANUS_ROOM/JANUS_PARTICIPANTS/JANUS_SERVER_CONTAINERS variables like the mediasoup one, plus unit tests of the adapter against recorded protocol exchanges. Validation: 10 bots pass, then the 50-bot alternated comparison of T029 extended to Janus."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a test plan against Janus (Priority: P1)

As a load test operator, I want to point an existing test plan at a Janus server instead of Jitsi, LiveKit or mediasoup-demo, changing only the platform and its connection settings, so that the bots join a Janus video room, send audio and video, receive each other's media and leave at the end of the run, as they do on the other platforms.

**Why this priority**: Without bots that join and exchange media, nothing else about Janus can be measured. This story alone already gives a load test of a Janus server (traffic, connection and media metrics of 002 to 004).

**Independent Test**: Start the local Janus server, create a scenario with platform `janus`, 10 bots, `audio_video`, for 1 minute, and run it: all bots join the same room, every bot receives audio and video from the others, the room shows 10 participants when a browser joins it with Janus's video room demo page, and all bots have left once the run ends.

**Acceptance Scenarios**:

1. **Given** a running Janus server and a test plan with platform `janus` and its connection settings, **When** the scenario is started, **Then** each bot joins the named room as its own participant, ramped up as the test plan says, and the scenario reports the bots as connected.
2. **Given** joined bots with profile `audio_video` (or `audio`), **When** they run, **Then** each bot publishes its audio (and video) in the room, and each bot receives the media of the other bots.
3. **Given** a receive cap of N streams per kind, **When** more than N other bots publish, **Then** each bot receives at most N streams per kind, and when one of them ends, a waiting one takes its place, as on the other platforms.
4. **Given** profile `receive_only`, **When** the bots run, **Then** they publish nothing and receive the media of other publishers in the room (for example a browser on the demo page).
5. **Given** a running scenario, **When** it reaches its duration or is stopped, **Then** every bot leaves the room and the server shows none of them left behind.
6. **Given** a room that does not exist yet, **When** the first bot joins, **Then** the room is created with room for every bot of the test plan to publish, and the other bots join it.

---

### User Story 2 - The same report for Janus as for the other platforms (Priority: P2)

As a load test operator, I want a Janus run to produce the same end-of-run report as a Jitsi, LiveKit or mediasoup run (media quality, worst bots, server load, the load machine's own load, and the report files of 009), in the same form, so that I can put the reports of the four platforms side by side for the same test plan.

**Why this priority**: Comparing media servers under the same load is the user's goal. The measures already exist (006, 007, 009); this story makes sure they hold on Janus, which forwards media differently.

**Independent Test**: Run the same 10-bot, 1-minute test plan on Janus with server monitoring naming its container, and compare its quality summary and report files with those of a mediasoup run of the same plan: same sections and fields; on an idle machine no freezes, no audio gaps and a high audio score; the Janus server's CPU and memory are reported.

**Acceptance Scenarios**:

1. **Given** a finished Janus scenario, **When** the operator asks the API for its quality summary, **Then** it has the same sections and fields as the other platforms' summaries.
2. **Given** a test plan whose server monitoring names the Janus container, **When** the run ends, **Then** the server section reports that container's CPU and memory, as for the other platforms.
3. **Given** a measure that cannot be taken on Janus, **When** the summary is built, **Then** that measure is left out and the summary says why, rather than reporting zero or a wrong value.
4. **Given** a finished Janus run, **When** its report files are written, **Then** they are named with platform `janus` and have the same layout as the other platforms' report files.
5. **Given** the run's Prometheus series, **When** the operator reads them for a Janus scenario, **Then** they are the same series, with the same labels, as for the other platforms.

---

### User Story 3 - A local Janus server and repeatable tests (Priority: P3)

As a load test operator or developer, I want a Janus setup in the repository that starts with one command on the load machine, a live test that runs bots against it, and automated tests of the Janus support that need no server, so that Janus results can be reproduced and compared with the other platforms' results taken on the same machine, and regressions are caught by the normal test run.

**Why this priority**: The first two stories can be developed against any Janus server; a pinned, documented setup and tests make the results repeatable and the comparison fair. It is needed before the comparison runs.

**Independent Test**: On a machine with Docker, start the setup with the documented command, run the live test with 10 participants against it, open the room in a browser on the demo page, then stop the setup; separately, the project's normal test run covers the Janus support without any server.

**Acceptance Scenarios**:

1. **Given** the repository and Docker, **When** the operator starts the Janus setup as documented, **Then** a Janus server of a pinned version runs locally, reachable by the bots on the same machine, with the demo page to watch a room.
2. **Given** the running setup, **When** the operator runs the documented live test with its server address, room, number of participants and container names, **Then** the bots join, exchange media, and the test passes or fails on the same checks as the mediasoup live test.
3. **Given** a checkout without any Janus server, **When** the project's normal test run is started, **Then** it checks the Janus support against recorded exchanges with a real Janus server (join, publish, receive within the cap, promotion, leave, errors) and passes.
4. **Given** the setup, **When** the 50-bot comparison of the other three platforms (010 T029) is run again with Janus added, alternated, **Then** its runs complete and its results are recorded next to the others.

---

### Edge Cases

- **Options this version does not support on Janus** (simulcast, requested heights): the scenario is rejected at creation, with a message naming the option and the platform.
- **Missing or invalid connection settings** (no server address, no room, a room that is not a number when the server expects one): the scenario is rejected at creation and the message names the setting.
- **Server over TLS with a self-signed certificate**: the operator can say in the settings that the certificate is not checked; checking stays on unless the settings say otherwise.
- **Server protected by an API secret or by tokens, room protected by a PIN or secret**: the settings MAY carry them; the bots present them; a wrong or missing one makes the join fail with the server's reason. They never appear in logs, API answers, metrics or report files.
- **A room that exists with too few publisher places** (Janus rooms limit how many participants publish at once): bots beyond the limit fail to publish and are reported as failed with the server's reason, never counted as connected; the documentation says how to size the room.
- **A room that cannot be created** (the server forbids creating rooms, or asks for an admin key the settings lack): the join fails with the server's reason.
- **Session expiry**: Janus drops a session that stays silent; the bots keep their sessions alive for the whole run, whatever its length.
- **Unreachable server, refused request during the join**: the bot fails to join and is reported as failed with the reason.
- **The server closes a bot's connection or session during the run** (restart, kicked participant, destroyed room): the bot is reported as lost, and the run goes on for the other bots.
- **Two bots with the same participant identity in one room**: surfaced as a failed or lost bot, not hidden.
- **The server offers media the bots did not ask for** (participants that are not bots, data channels, screen sharing): the cap still applies to audio and video, and anything else is ignored without failing the bot.
- **The server lowers the forwarded video under load**: the bots report what they actually receive.
- **A Jitsi, LiveKit or mediasoup test plan**: it runs exactly as before this feature.

## Requirements *(mandatory)*

### Functional Requirements

#### Joining and media (User Story 1)

- **FR-001**: The test plan's platform MUST accept `janus` in addition to `jitsi`, `livekit` and `mediasoup`; any other value MUST still be rejected at creation.
- **FR-002**: The Janus connection settings MUST include the server address and the room, and MAY say that the server's certificate is not checked and give the server's API secret or token, the room's PIN or secret, and the key needed to create rooms. Missing or malformed settings MUST be rejected at creation with a message naming the setting. The secrets MUST never appear in logs, API answers, metrics or report files.
- **FR-003**: Each bot MUST join the room as its own participant, with its peer ID as its display name, and MUST be counted as connected only once it can publish (if its profile sends) and receive media.
- **FR-004**: When the room does not exist, the bots MUST create it, with enough publisher places for every bot of the test plan, and the bots joining at the same time MUST NOT fail because another bot is creating it.
- **FR-005**: Bots MUST send what the test plan's media profile says (`audio`, `audio_video` or nothing for `receive_only`), with the same content, codecs and rates as on the other platforms.
- **FR-006**: Each bot MUST receive the audio and video of other publishers up to the receive cap per kind, and MUST replace an ended stream by a waiting one, as on the other platforms.
- **FR-007**: The bots MUST keep their Janus sessions alive for the whole run, however long it is.
- **FR-008**: At the end of the run, or when it is stopped, every bot MUST leave the room and release its session and everything it opened, within the same time as on the other platforms.
- **FR-009**: A failed join, a lost connection and a disconnection MUST be reported through the same lifecycle states, metrics and logs as for the other platforms, with Janus's reason.
- **FR-010**: Test plans for `janus` that enable simulcast or requested heights MUST be rejected at creation, with a message saying these options are not supported on Janus in this version.
- **FR-011**: Adding Janus MUST NOT change how a Jitsi, LiveKit or mediasoup test plan is validated, run or reported.

#### Report (User Story 2)

- **FR-012**: A Janus run's quality summary, report files and Prometheus series MUST have the same sections, fields and labels as the other platforms'; nothing in them may depend on the platform except their values and the platform's name.
- **FR-013**: Each quality measure of 006 MUST be computed for Janus from what the bots receive; a measure that cannot be computed on Janus MUST be left out with its reason, never reported as zero.
- **FR-014**: Server monitoring (007) MUST work for Janus by naming the Janus containers in the test plan, without any Janus-specific setting.

#### Local setup and tests (User Story 3)

- **FR-015**: The repository MUST contain, under `tests/platforms`, a Janus setup that starts a pinned Janus version locally with one documented command, with a page to watch a room in a browser, and its documentation (platform settings, container names, room sizing, how to stop it). Machine-specific values MUST stay out of version control, with an example file committed instead.
- **FR-016**: The repository MUST contain a live test for Janus, run only on demand, configured by the server address, room, number of participants and server container names, with the same checks as the mediasoup live test.
- **FR-017**: The project's normal test run MUST cover the Janus support without a server, against exchanges recorded from the pinned Janus version: join and room creation, publishing, receiving within the cap, promotion when a stream ends, keeping the session alive, leaving, and the server's errors.

### Key Entities

- **Janus connection settings**: the platform part of a test plan for `janus`: server address, room, whether the server's certificate is checked, optionally the API secret or token, room PIN or secret, and room-creation key (all write-only). Validated at creation.
- **Bot participant**: a bot as Janus sees it: one session per bot, a publisher in the room identified by the bot's peer ID, and one subscription to at most the receive cap of other publishers' streams per kind.
- **Local Janus setup**: the pinned server version, its configuration (transports, announced address, media ports, room defaults), the demo page, its container names for server monitoring.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A test plan that runs on the other platforms runs on Janus by changing only the platform and its connection settings: 10 bots `audio_video` for 1 minute all join, each receives 9 audio and 9 video streams, and all leave within 10 seconds of the end.
- **SC-002**: On the local setup, 50 bots `audio_video` with receive cap 10 run for 10 minutes against Janus with every bot connected for the whole hold, in the same conditions as the 50-bot runs of the other three platforms (010 T029), alternated with them, and their results are recorded side by side.
- **SC-003**: For the 10-bot run on an otherwise idle machine, the Janus summary shows no freezes, audio gaps below 1% and a median audio score of at least 4.0, as the other platforms' runs of the same plan do.
- **SC-004**: The Janus summary and report files have exactly the same set of fields as the other three platforms' for the same test plan.
- **SC-005**: 100% of test plans for Janus with simulcast, requested heights or incomplete settings are rejected at creation, before any bot starts.
- **SC-006**: From a clean checkout with Docker, an operator starts the local Janus setup and runs the 10-bot live test in under 15 minutes, following only the repository's documentation.
- **SC-007**: The project's normal test run passes with the Janus tests included, needs no network or server for them, and stays green with race detection.

## Assumptions

- The server is the open source Janus WebRTC Server, version 1.x (multistream), self-hosted from a pinned version with its VideoRoom plugin, unchanged except for its configuration. Other Janus plugins (AudioBridge, Streaming, SIP), the legacy 0.x API and the HTTP long-poll transport are out of scope; the bots use the WebSocket transport, as Janus's own JavaScript library can.
- The bots use the same signaling as Janus's video room demo: each bot publishes its own streams and receives the others through one subscription that changes as publishers come and go.
- The local setup has no API secret and allows bots to create rooms; the settings for secrets exist for servers that use them.
- The bots and the server run on the same machine, as for 007's server monitoring; a remote server may work for joining, but its load is not measured.
- Bots send the same synthetic content and codecs as on the other platforms (Opus audio, VP8 video at one resolution), which Janus supports; no new media content is needed.
- One report per run and per server; comparing platforms is done by reading the reports, no comparison report (agreed 2026-10-16).
- Simulcast and requested heights on Janus, data channels, screen sharing, recording and Janus's other features are out of scope for this version.
- The 100-bot run is not a success criterion here: on this laptop mediasoup-demo's room did not hold 100 bots (010 SC-002), so the cross-platform comparison is taken at 50 bots, where all three held.
- The platform boundary of 008 (a sending session separate from the receiving one) fits Janus's separate publisher and subscriber connections; this feature reuses it, and the 010 consent checks, and adds no platform-specific path in the engine.
