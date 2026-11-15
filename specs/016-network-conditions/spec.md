# Feature Specification: Network Conditions per Bot

**Feature Branch**: `016-network-conditions` (work continues on `rework`)

**Created**: 2026-11-15

**Status**: Draft

**Input**: User description: "Network conditions per bot: apply bandwidth limits and network instability (rate cap, added delay and jitter, packet loss, and changes of these over time) to each bot or group of bots from the test plan, and report how each server (Jitsi, LiveKit, mediasoup, Janus) adapts: the layers it forwards, its estimate where known, freezes, quality score and recovery time after a condition ends."

## Background

Every run so far uses a clean path: loopback, or a loss-free LAN to the remote machine (013). The README lists "bandwidth limits and network instability" under "Not done yet". Real receivers have slow or lossy links, and how a server serves them is a main difference between servers: which layer it forwards to a constrained receiver, how fast it reacts, whether the other receivers in the room are affected, and how fast it comes back when the link recovers.

Until 015, a low estimate could come from the bots themselves. 015 made the bots acknowledge packets like a browser does and added per-bot reception reports, so a constrained run can now be read as describing the server. Two server behaviours found in 015 must be kept apart from the effect of a condition: mediasoup 3.30.1's probe bug when many receivers start together, and Jitsi's start-up suspensions on a LAN.

## Clarifications

### Session 2026-11-15

- Q: Do conditions also apply on the path from a bot to the server, and what does a bot then send? → A: Both directions; a constrained sender lowers what it sends from the server's feedback, like a browser.
- Q: Where are conditions applied? → A: In the engine, on each bot's own packets: per bot, no administrator rights, the same locally and against a remote server.
- Q: LiveKit, on its defaults, sizes what it sends each receiver from REMB (its offer asks for `goog-remb`, not transport-cc); the bots send none, so a capped bot was never lowered. What do the bots do? → A: Like a browser: where a session asks for REMB and not for transport-cc, the bot estimates its download from the arrival times of the packets and sends REMB.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A receiver with a slow link (Priority: P1)

As a load test operator, I want to limit the bandwidth toward some bots in a room, from the test plan, and see in the report which layers the server then forwards to them and whether their video freezes, so that I can compare how each server serves a constrained receiver.

**Why this priority**: A rate cap toward the receiver is the condition every server is built to adapt to, through its estimate and its layers. It alone gives a comparison between the four servers that the clean runs cannot give.

**Independent Test**: On each local server, run 10 bots sending layered VP8, all asking for 720, with a 1200 kbit/s cap toward 3 of them for the whole run (it carries four 180-line streams, not four 360-line ones). The report names the 3 constrained bots and gives, for each bot, the heights received over time, freezes and score; the 7 others keep their results of a run without the cap.

**Acceptance Scenarios**:

1. **Given** a test plan that caps the bandwidth toward a group of bots, **When** the run ends, **Then** both report files give the condition applied to each group and, for each bot, which group it was in.
2. **Given** a capped bot on a server that forwards layers, **When** the run ends, **Then** the report gives the heights the bot received over time, the share of time at each height, its freezes and its score, as for any bot.
3. **Given** a room with capped and uncapped bots, **When** the run ends, **Then** the report gives the results of each group apart, so that an effect on the uncapped bots is visible.
4. **Given** a capped bot, **When** the run ends, **Then** the report gives what the condition did: the rate that reached the bot and the packets the condition dropped, so that a report never shows a condition that was not applied.
5. **Given** a test plan with no condition, **When** it runs, **Then** it behaves and reports as before this feature.

---

### User Story 2 - A link that changes during the run (Priority: P2)

As a load test operator, I want to change a group's condition at stated times during the run (for example 2 Mbit/s, then 300 kbit/s for 20 seconds, then no limit), and see how long the server takes to adapt and to recover, so that I can compare servers on reaction and recovery time, not only on a steady state.

**Why this priority**: Real links change; the time a server needs to drop a layer, and to give it back, is what a user sees as a freeze or a blurry minute. It builds on US1's conditions and reports.

**Independent Test**: On each local server, run 10 bots with layered VP8 asking for 720; toward 3 bots, no limit for 20 s, then 300 kbit/s for 20 s, then no limit for 30 s. The report gives, per bot and per change, the time until the bot received a lower height after the drop, and the time until it received 720 again after the end of the limit.

**Acceptance Scenarios**:

1. **Given** a test plan with a schedule of conditions for a group, **When** the run ends, **Then** the report gives each step of the schedule with the time it started and ended in the run.
2. **Given** a step that lowers the bandwidth below what a bot's requested height needs, **When** the run ends, **Then** the report gives, per bot, the reaction time: from the start of the step to the first lower height received, or to the first freeze, whichever comes first.
3. **Given** a step that ends a limit, **When** the run ends, **Then** the report gives, per bot, the recovery time: from the end of the step until the bot again receives its requested height and receives it for at least 5 seconds without a freeze; or says the bot did not recover before the run ended.
4. **Given** a schedule whose steps fall outside the run's duration, **When** the test plan is created, **Then** it is refused with a message naming the step.

---

### User Story 3 - Delay, jitter and loss (Priority: P3)

As a load test operator, I want to add delay, jitter and packet loss toward or from groups of bots, alone or with a rate cap, so that I can see how each server's retransmission and recovery handle an unstable network, and what it costs in freezes, score and latency.

**Why this priority**: Loss and jitter are the other half of "network instability", and they test different server features (retransmission, keyframe requests) than a rate cap. They reuse US1's groups and US2's schedule.

**Independent Test**: On each local server, run 10 bots with 3% random loss and 40 ms ± 10 ms delay toward 3 of them. The report gives those bots' loss before and after the server's retransmissions, their freezes, score and latency, beside the 7 others.

**Acceptance Scenarios**:

1. **Given** a group with added delay and jitter, **When** the run ends, **Then** the bots' measured latency of 012 includes the added delay, and the report gives the delay that was applied.
2. **Given** a group with packet loss, **When** the run ends, **Then** the report gives, per bot, the share of packets the condition dropped and the share still missing after the server's retransmissions.
3. **Given** loss in bursts as well as random loss, **When** the test plan states the kind of loss, **Then** the condition drops packets that way, and the report gives the mean length of the dropped bursts.
---

### User Story 4 - A sender with a slow link (Priority: P4)

As a load test operator, I want to put a condition on the path from some bots to the server, and have those bots lower what they send as a browser does, so that I can see how each server and the other receivers handle a sender whose link is short.

**Why this priority**: A constrained sender is the other common case in a real meeting, and it is handled differently by each server (layer switching for the receivers, keyframe requests, estimate on the upload). It needs the bots to adapt, which they never did: without it, a capped sender only loses packets steadily, which no browser does.

**Independent Test**: On each local server, run 10 bots sending layered VP8, all asking for 720, with a 400 kbit/s cap on the path from 2 bots to the server. Those 2 bots stop sending their top layers within a few seconds, the receivers of their streams receive a lower height without lasting freezes, and the report gives, per constrained sender, the layers it sent over time.

**Acceptance Scenarios**:

1. **Given** a condition on the path from a group of bots to the server, **When** the run ends, **Then** the report gives the condition and its measured effect for that direction, as for US1–US3.
2. **Given** a sender that sends layers, **When** the server's feedback shows that its link cannot carry all of them, **Then** the bot stops sending its top layers, one at a time, and the report gives the layers it sent over time and its own estimate.
3. **Given** a sender whose link recovers, **When** the server's feedback shows room again, **Then** the bot sends its layers again, and the report gives the time it needed.
4. **Given** a sender that sends a single stream (no layers), **When** its link is too short, **Then** it keeps sending as today, and the report states that the sender could not adapt.
5. **Given** a run with no condition on any sender's path, **When** it runs, **Then** every bot sends all its layers for the whole run, as before this feature.
6. **Given** the receivers of a constrained sender's streams, **When** the run ends, **Then** the report gives their heights, freezes and score for those streams apart from the others.

---

### User Story 5 - The server's view and comparison across servers (Priority: P5)

As a load test operator, I want, where a server tells it, the server's own estimate for each constrained bot in the report, and a validation report that runs the same conditions on the four servers, so that I can say which server adapts best to each condition.

**Why this priority**: The layers and freezes of US1–US3 are what users see; the server's estimate explains them. The comparison is the goal of the feature, but it is a set of runs and a document once US1–US3 exist.

**Independent Test**: Run the US1 and US2 plans on the four local servers, three times each, and one US3 and one US4 plan once each, and read the validation report: one table per condition with, per server, the share of time at each height for the constrained and the unconstrained bots, freezes, score, reaction and recovery times.

**Acceptance Scenarios**:

1. **Given** a server that gives the bots its estimate for them through its usual signaling, **When** a run with a condition ends, **Then** the report gives that estimate per constrained bot over time.
2. **Given** a server that does not give it, **When** the report is written, **Then** the estimate is left out with a named reason, never a zero.
3. **Given** the comparison runs, **When** the validation report is written, **Then** it gives the results per server and per condition, with the run-to-run spread of three runs, and names the server behaviours of 015 where they affected a run.

---

### Edge Cases

- **The condition is the machine's**: with 100 bots on one laptop, the engine itself can drop or delay packets. The report gives the packets the condition dropped apart from those lost elsewhere, and a constrained run states the engine's CPU.
- **A cap lower than the smallest layer**: below what the lowest layer needs, every server must freeze or stop the video. The report shows it as such and does not count it as a server failure.
- **A server without layers for that codec**: H.264 on Jitsi forwards one layer only (014); a capped bot then can only freeze or receive the full stream. The report shows the forwarded layers as today's `layers_not_forwarded`.
- **Audio under a cap**: audio shares the capped link. Its concealment is reported for the constrained bots as for the others.
- **Start-up behaviours of 015**: mediasoup's probe bug and Jitsi's start-up suspensions happen without any condition. A condition starts after a stated settling time, or the report warns that a condition began while the room was starting.
- **The schedule and the ramp-up**: a step that begins before all bots have joined applies to the bots of the group that have joined; later bots get the step in force when they join.
- **A group larger than the room**: a group given as a count larger than the number of bots is refused when the test plan is created.
- **Remote server**: conditions apply between the bots and the server wherever the server runs (local or on a remote machine, 013); the LAN's own conditions add to them and are not removed.
- **The reference browser of 015**: it is never put under a condition; it stays the clean reference in the room.
- **Repeatability**: random loss and jitter must give comparable runs. The test plan can state a seed; the report gives the seed used.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A test plan MUST be able to define groups of bots, each by a count or a share of the participants, and give each group a network condition; bots in no group have no condition.
- **FR-002**: A condition MUST be able to combine a rate cap, an added delay with jitter, and packet loss (random, or in bursts with a stated mean burst length), each optional, separately for each direction of a bot's path: from the server to the bot, and from the bot to the server. Conditions MUST be applied by the engine itself, on each bot's own packets, so that they are truly per bot and need no administrator rights or change on the load machine.
- **FR-003**: A condition MUST be able to change at stated times: a group's condition is a schedule of steps, each with a start time in the run, a duration and its condition; no step means no condition.
- **FR-004**: A test plan whose conditions are invalid (a negative or zero rate, loss outside 0–100%, a step outside the run, overlapping steps for one group, a group larger than the room) MUST be refused at creation with a message naming the field.
- **FR-005**: Both report files MUST give each group's schedule as applied, with the actual start and end time of each step, and each bot's group.
- **FR-006**: The report MUST give, per constrained bot and per step, what the condition did: the rate that reached the bot, the packets dropped by the condition, and the added delay, measured, not copied from the test plan.
- **FR-007**: The report MUST give the existing per-bot results (heights over time, share at each height, freezes, score, latency, loss) for every bot, and a summary per group, so that constrained and unconstrained bots can be compared within a run.
- **FR-008**: For each step that lowers the bandwidth, the report MUST give per bot the reaction time (to the first lower height or first freeze); for each step that ends or raises a limit, the recovery time (until the requested height is received again for at least 5 seconds without a freeze), or "not recovered" before the end of the run.
- **FR-009**: For loss, the report MUST give per bot the share dropped by the condition and the share still missing after the server's retransmissions.
- **FR-010**: Where the server gives the bots its estimate for them through its signaling, the report MUST give it per constrained bot over time; elsewhere it MUST be left out with a named reason.
- **FR-011**: A condition that starts while bots are still joining, or within a stated settling time after the last join, MUST be flagged in the report.
- **FR-012**: Random loss and jitter MUST be drawn from a seed that the test plan can state; the report MUST give the seed used, so that a run can be repeated with the same draws.
- **FR-013**: A run with no condition MUST behave as before this feature, and the engine's CPU per bot at 100 bots without conditions MUST stay within 5% of its value before this feature. The CPU cost of conditions MUST be measured and stated.
- **FR-014**: Conditions MUST work the same on the four platforms and against local and remote servers (013), with no platform-specific setting in the test plan.
- **FR-015**: The validation report MUST compare the four servers on the US1 and US2 conditions, three runs each, and on at least one US3 and one US4 condition.
- **FR-016**: A bot that sends layers MUST estimate the bandwidth of its path to the server from the server's feedback, and MUST stop sending its top layers when the estimate is below what they need, and send them again when it is above, one layer at a time, so that its sending stays within its link as a browser's does.
- **FR-017**: The report MUST give, per sending bot, the layers it sent over time and its own estimate; for a bot that sends a single stream, it MUST state that the bot could not adapt.
- **FR-019**: Where a receiving session asks for REMB and not for transport-wide feedback (LiveKit on its defaults), every bot MUST estimate its download from the packets' send and arrival times and send REMB about the streams it receives, as a browser does; runs on such servers without conditions MUST keep their results within today's run-to-run variation (SC-006).
- **FR-018**: A bot whose path has no condition MUST NOT drop layers on a clean path: in runs without conditions, every bot sends all its layers for at least 99% of the run, the start-up excepted.

### Key Entities

- **Bot group**: a named set of bots of a run, given by a count or a share of the participants, with one schedule of conditions.
- **Network condition**: rate cap, delay and jitter, loss (random or bursts), each optional, on a stated direction of a bot's path.
- **Schedule step**: a condition with a start time and duration in the run; a group's schedule is its ordered steps.
- **Applied condition**: per bot and per step, what was measured: rate reaching the bot, packets dropped, delay added, and the actual times of the step.
- **Adaptation result**: per bot and per step, reaction time, recovery time or "not recovered", heights received, freezes, and the server's estimate where given.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Under a rate cap, the rate measured reaching a constrained bot over a 10-second window never exceeds the cap by more than 10%; added delay is within 5 ms of the stated value on average; random loss is within 1 percentage point of the stated share over a 1-minute run.
- **SC-002**: On each of the four servers, a 10-bot layered VP8 run with a 1200 kbit/s cap toward 3 bots, each receiving 4 streams, gives a report that names the 3 bots and their condition, and their heights, freezes and score; on servers that forward layers, the constrained bots receive a lower height than 720 for most of the run.
- **SC-003**: In the same runs, the 7 unconstrained bots receive 720 at least 95% of their measured time, or the report shows the effect of the constrained bots on them.
- **SC-004**: In the US2 schedule, every constrained bot has a reaction time and a recovery time (or "not recovered") per step, in both report files, on the four servers.
- **SC-005**: Two runs with the same plan and the same seed drop the same share of packets per bot within 0.5 percentage point.
- **SC-006**: Runs without conditions give the same results as before this feature, within today's run-to-run variation, and the engine's CPU per bot at 100 bots stays within 5%.
- **SC-007**: The validation report has, for the US1 and US2 conditions and one US3 and one US4 condition, a table per condition comparing the four servers over three runs each.
- **SC-008**: Under a 400 kbit/s cap on its upload, a sender of layered VP8 stops sending the layers its link cannot carry within 5 seconds, and its sending then stays within the cap; within 10 seconds of the cap's end, it sends all its layers again.
- **SC-009**: In runs without conditions, no bot drops a layer after the first 10 seconds.
- **SC-010**: On LiveKit with its default settings, under the US1 cap, the constrained bots receive a lower height than 720 for most of the step, from the bots' REMB; and LiveKit runs without conditions keep their 014 shares at the requested height within 0.02.

## Assumptions

- The constrained participants are bots; the 015 reference browser is never constrained.
- Bots that send layers now estimate their upload from the server's feedback and drop or restore whole layers (US4). The content stays pre-encoded (constitution, principle IV): a bot cannot lower the bitrate within a layer, and a single-stream sender does not adapt.
- Receiving bots estimate their download only where the server asks for REMB (FR-019); elsewhere the server estimates from their transport-wide feedback (015). The server decides what they receive.
- "What a requested height needs" uses the content's recorded bitrates (014), as in 015.
- The 5-second "recovered" window, the 95% share and the freeze rule follow 006 and 014.
- Conditions are applied only between the bots and the server; the server's machine and its configuration are not modified (servers stay on their defaults, 015).
- Load distribution over several agents and other topologies stay out of scope (README "Not done yet").
- The first comparison is made on the local servers; one confirmation on the remote machine (013) is enough.
