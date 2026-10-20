# Feature Specification: mediasoup-demo Platform

**Feature Branch**: `010-mediasoup-platform` (work continues on `rework`)

**Created**: 2026-10-19

**Status**: Draft

**Input**: User description: "010 mediasoup-demo platform: run the same test plan as Jitsi and LiveKit against the official mediasoup-demo server (protoo signaling over WebSocket), so the three media servers can be compared under identical load with one report per server (MOS, server CPU, freezes, media issues). Scope of the first version, as agreed for 008: audio+video with a receive cap only; simulcast and requested heights rejected at test plan creation. Reuse the platform-neutral publish session from 008 (mediasoup send transport = publish session, recv transport = receive session). Register the platform in src/platform/catalog. Provide a local Docker setup of mediasoup-demo in tests/platforms/mediasoup; server CPU/memory read from its local container as in 007; run reports written as in 009. Remote servers out of scope."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a test plan against mediasoup-demo (Priority: P1)

As a load test operator, I want to point an existing test plan at a mediasoup-demo server instead of Jitsi or LiveKit, changing only the platform and its connection settings, so that the bots join a mediasoup-demo room, send audio and video, receive each other's media and leave at the end of the run, as they do on the other two platforms.

**Why this priority**: Without bots that join and exchange media, nothing else about mediasoup can be measured. This story alone already gives a load test of a mediasoup-demo server (traffic, connection and media metrics of 002 to 004).

**Independent Test**: Start the local mediasoup-demo server, create a scenario with platform `mediasoup`, 10 bots, `audio_video`, for 1 minute, and run it: all bots join the same room, every bot receives audio and video from the others, the room is visible with 10 peers when a browser joins it with the demo's web client, and all bots have left once the run ends.

**Acceptance Scenarios**:

1. **Given** a running mediasoup-demo server and a test plan with platform `mediasoup` and its connection settings, **When** the scenario is started, **Then** each bot joins the named room as its own peer, ramped up as the test plan says, and the scenario reports the bots as connected.
2. **Given** joined bots with profile `audio_video` (or `audio`), **When** they run, **Then** each bot sends its audio (and video) to the room, and each bot receives the media of the other bots.
3. **Given** a receive cap of N streams per kind, **When** more than N other bots send, **Then** each bot receives at most N streams per kind, and when one of them ends, a waiting one takes its place, as on Jitsi and LiveKit.
4. **Given** profile `receive_only`, **When** the bots run, **Then** they send nothing and receive the media of other peers in the room (for example a browser on the demo's web client).
5. **Given** a running scenario, **When** it reaches its duration or is stopped, **Then** every bot leaves the room and the server shows none of them left behind.
6. **Given** a server that offers media to a bot only once the bot accepts it, **When** a bot is offered more streams than its receive cap, **Then** the streams beyond the cap are not received (or not forwarded) until a received one ends.

---

### User Story 2 - The same report for mediasoup as for Jitsi and LiveKit (Priority: P2)

As a load test operator, I want a mediasoup run to produce the same end-of-run report as a Jitsi or LiveKit run (media quality, worst bots, server load, the load machine's own load, and the report files of 009), in the same form, so that I can put the reports of the three platforms side by side for the same test plan and compare MOS, freezes, media issues and server CPU.

**Why this priority**: Comparing the three media servers under the same load is the user's goal. The measures already exist (006, 007, 009); this story makes sure they hold on mediasoup, which forwards media differently from the Jitsi bridge and LiveKit.

**Independent Test**: Run the same 10-bot, 1-minute test plan on mediasoup-demo, with server monitoring naming its container, and compare its quality summary and report files with those of a LiveKit run of the same plan: same sections and fields; on an idle machine no freezes, no audio gaps and a high audio score; the mediasoup server's CPU and memory are reported.

**Acceptance Scenarios**:

1. **Given** a finished mediasoup scenario, **When** the operator asks the API for its quality summary, **Then** it has the same sections and fields as a Jitsi or LiveKit summary: received frame rate, resolution, freezes, keyframe requests, audio gaps, audio score, latency, worst bots, server load and load machine.
2. **Given** a test plan whose server monitoring names the mediasoup-demo container, **When** the run ends, **Then** the server section reports that container's CPU and memory, all its media worker processes included, as for the other platforms.
3. **Given** a measure that cannot be taken on mediasoup (for example latency, if the server changed what identifies a frame), **When** the summary is built, **Then** that measure is left out and the summary says why, rather than reporting zero or a wrong value; the other measures are still reported.
4. **Given** a finished mediasoup run, **When** its report files are written, **Then** they are named with platform `mediasoup` and have the same layout as the Jitsi and LiveKit report files.
5. **Given** the run's Prometheus series, **When** the operator reads them for a mediasoup scenario, **Then** they are the same series, with the same labels, as for a Jitsi or LiveKit scenario.

---

### User Story 3 - A local mediasoup-demo server to test against (Priority: P3)

As a load test operator or developer, I want a mediasoup-demo setup in the repository that starts with one command on the load machine, as the Jitsi and LiveKit ones do, so that any mediasoup result can be reproduced and compared with the other platforms' results taken on the same machine.

**Why this priority**: The first two stories can be developed against any mediasoup-demo server; a pinned, documented local setup makes the results repeatable and the comparison fair. It is needed before the scale runs.

**Independent Test**: On a machine with Docker, start the setup with the documented command, run the 10-bot scenario of User Story 1 against it with the documented settings, open the room in a browser with the demo's web client, then stop the setup; nothing else needs installing.

**Acceptance Scenarios**:

1. **Given** the repository and Docker, **When** the operator starts the mediasoup-demo setup as documented, **Then** a mediasoup-demo server of a pinned version runs locally, reachable by the bots on the same machine.
2. **Given** the running setup, **When** the operator reads its documentation, **Then** it gives the platform settings and the server-monitoring container names to put in a test plan, and the number of media workers the server runs.
3. **Given** the setup, **When** a 100-bot scenario (`audio_video`, receive cap 10) runs against it, **Then** the run completes and its report is recorded next to the Jitsi and LiveKit reports of the same test plan.

---

### Edge Cases

- **Options this version does not support on mediasoup** (simulcast, requested heights): the scenario is rejected when it is created, with a message naming the option and the platform; it never starts and silently ignores them.
- **Missing or invalid connection settings** (no server address, no room): the scenario is rejected at creation and the message names the setting.
- **Server over TLS with a self-signed certificate** (common for the demo over HTTPS; without a certificate the demo serves plain WebSocket): the operator can say in the settings that the certificate is not checked; certificate checking stays on unless the settings say otherwise.
- **Room time limit**: the demo closes a room after 10 minutes unless the room is unlimited (always, when the server has no unlimited-room secret; otherwise when the bots present it). The settings MAY carry that secret; a room closed by the server makes its bots lost, as for any closed connection, never a silent stop.
- **Unreachable server, refused request during the join**: the bot fails to join and is reported as failed with the reason, as a failed Jitsi or LiveKit join is.
- **The server closes a bot's connection during the run** (server restart, peer removed): the bot is reported as lost, as on the other platforms, and the run goes on for the other bots.
- **The room does not exist yet**: the first bot's join creates it; the bots do not need it created beforehand.
- **Two bots with the same peer ID in one room** (two scenarios using the same room and IDs): the server keeps only one; this is surfaced as a lost bot, not hidden. Peer IDs are already unique within a scenario.
- **The server offers media the bots did not ask for** (peers that are not bots, more streams than the cap, data channels, the demo's bot peer): the cap still applies to audio and video, and anything else is declined without failing the bot.
- **The server lowers the received video quality under load**: the bots report what they actually receive, as for the other platforms.
- **A Jitsi or LiveKit test plan**: it runs exactly as before this feature.

## Requirements *(mandatory)*

### Functional Requirements

#### Joining and media (User Story 1)

- **FR-001**: The test plan's platform MUST accept `mediasoup` in addition to `jitsi` and `livekit`; any other value MUST still be rejected at creation.
- **FR-002**: The mediasoup connection settings MUST include the server address and the room name, and MAY say that the server's certificate is not checked and give the server's unlimited-room secret. Missing or malformed settings MUST be rejected at creation with a message naming the setting. The unlimited-room secret MUST never appear in logs, API answers, metrics or report files.
- **FR-003**: Each bot MUST join the room as its own peer, with its peer ID as its identity, and MUST be counted as connected only once it can send (if its profile sends) and receive media.
- **FR-004**: Bots MUST send what the test plan's media profile says (`audio`, `audio_video` or nothing for `receive_only`), with the same content, codecs and rates as on Jitsi and LiveKit.
- **FR-005**: Each bot MUST receive the audio and video of other peers up to the receive cap per kind, and MUST replace an ended stream by a waiting one, as on the other platforms.
- **FR-006**: At the end of the run, or when it is stopped, every bot MUST leave the room and release everything it opened, within the same time as on the other platforms.
- **FR-007**: A failed join, a lost connection and a disconnection MUST be reported through the same lifecycle states, metrics and logs as for the other platforms, with the platform's reason.
- **FR-008**: Test plans for `mediasoup` that enable simulcast or requested heights MUST be rejected at creation, with a message saying these options are not supported on mediasoup in this version.
- **FR-009**: Adding mediasoup MUST NOT change how a Jitsi or LiveKit test plan is validated, run or reported.

#### Report (User Story 2)

- **FR-010**: A mediasoup run's quality summary, report files and Prometheus series MUST have the same sections, fields and labels as a Jitsi or LiveKit run's; nothing in them may depend on the platform except their values and the platform's name.
- **FR-011**: Each quality measure of 006 MUST be computed for mediasoup from what the bots receive, as for the other platforms; a measure that cannot be computed on mediasoup MUST be left out with its reason, never reported as zero.
- **FR-012**: Server monitoring (007) MUST work for mediasoup by naming the mediasoup-demo containers in the test plan, without any mediasoup-specific setting, and MUST count the CPU of all the server's media workers.

#### Local setup (User Story 3)

- **FR-013**: The repository MUST contain, under `tests/platforms`, a mediasoup-demo setup that starts a pinned mediasoup-demo version locally with one documented command, and its documentation (platform settings, container names, number of media workers, how to stop it).
- **FR-014**: The setup MUST make the server reachable by bots on the load machine without changing the demo's code beyond its configuration; any local configuration that holds machine-specific values (address, certificate) MUST stay out of version control, with an example file committed instead.

### Key Entities

- **mediasoup connection settings**: the platform part of a test plan for `mediasoup`: server address, room name, whether the server's certificate is checked, optionally the unlimited-room secret (write-only). Validated at creation.
- **Bot peer**: a bot as mediasoup-demo sees it: one peer per bot in the room, identified by the bot's peer ID, producing the bot's tracks on its sending side and consuming at most the receive cap of other peers' tracks per kind on its receiving side.
- **Local mediasoup-demo setup**: the pinned server version, its configuration (listening address, announced address, media ports, number of workers), its certificate, its container names for server monitoring.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A test plan that runs on Jitsi or LiveKit runs on mediasoup by changing only the platform and its connection settings: 10 bots `audio_video` for 1 minute all join, each receives 9 audio and 9 video streams, and all leave within 10 seconds of the end.
- **SC-002**: On the local setup, 100 bots `audio_video` with receive cap 10 run for 10 minutes against mediasoup with every bot connected for the whole hold, and the load machine's engine stays within the CPU budget it has on LiveKit for the same plan (within run-to-run noise).
- **SC-003**: For the 10-bot run on an otherwise idle machine, the mediasoup summary shows no freezes, audio gaps below 1% and a median audio score of at least 4.0, as the Jitsi and LiveKit runs of the same plan do.
- **SC-004**: The mediasoup, LiveKit and Jitsi summaries and report files of the same test plan have exactly the same set of fields, so the three reports can be read side by side without any conversion.
- **SC-005**: 100% of test plans for mediasoup with simulcast, requested heights or incomplete settings are rejected at creation, before any bot starts.
- **SC-006**: From a clean checkout with Docker, an operator starts the local mediasoup-demo setup and runs the 10-bot scenario in under 15 minutes, following only the repository's documentation (the first start may build the demo).

## Assumptions

- The server is the official open source mediasoup-demo (its server part), self-hosted from a pinned version, unchanged except for its configuration; other mediasoup applications, custom signaling, and the demo's HTTP "broadcaster" API are out of scope. The bots use the same signaling as the demo's web client.
- mediasoup-demo has no authentication: any peer that knows the address and room joins. No token is needed; the only secret is the optional unlimited-room secret of a server that sets one (the local setup does not, so its rooms have no time limit).
- The bots and the server run on the same machine, as for 007's server monitoring; a server on another machine may work for joining, but its server load is not measured (007's scope).
- The server's load is that of its container: the signaling process and its media worker processes together; the number of workers is part of the setup and is documented, since it changes the server's CPU use.
- Bots send the same synthetic content and codecs as on Jitsi and LiveKit (Opus audio, VP8 video at one resolution), which mediasoup-demo supports; no new media content is needed.
- One report per run and per server, as for the other platforms; comparing platforms is done by reading the reports, and no comparison report is produced (agreed 2026-10-16).
- Simulcast and requested heights on mediasoup, data channels, screen sharing and the demo's other features (chat, stats, recording) are out of scope for this version and may come in a later feature.
- The latency measure of 006 identifies a frame by its RTP timestamp; if mediasoup rewrites it, leaving latency out with its reason (FR-011) is acceptable for this version.
- The platform boundary of 008 (a sending session separate from the receiving one) fits mediasoup's separate sending and receiving transports; this feature reuses it and does not add a platform-specific path in the engine.
