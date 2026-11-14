# Feature Specification: Bandwidth Estimate Parity with a Browser

**Feature Branch**: `015-bandwidth-estimate-parity` (work continues on `rework`)

**Created**: 2026-11-13

**Status**: Draft

**Input**: User description: "Settle whether the bots cause the low bandwidth estimates seen on mediasoup and Jitsi (014 validation report, 'Not met or not checked'): compare the server's estimate for a browser and for the bots in the same room, fix the bots if they are the cause, and stop depending on Jitsi's estimate being turned off."

## Background

Two results of 014 are open, and both come from the server's estimate of how much each receiver can take:

- **mediasoup**: bots asking for 720 lines receive them only part of the time (0.53 to 0.86 of a 1-minute run at 10 bots; SC-008 of 014 asks for 0.95). The server holds them on a lower layer while its estimate for them is low. The cause "between mediasoup's ramp-up and the bots' feedback" was not settled, because no browser was compared.
- **Jitsi, local setup**: with the bridge's default settings, some VP8 and AV1 runs at 10 bots freeze (7 to 41 freezes, no packet loss): the bridge stops forwarding some streams. The 014 tables, and the remote deployment of 013, only pass with the bridge's use of its estimate turned off.

The path is loss-free in both cases, and the two servers estimate bandwidth in different ways. What they share is the bots. An earlier case had the same shape (005): the bots did not acknowledge the bridge's probe packets, and the bridge suspended streams. Until this is settled, a layered result can describe the bots as much as the server.

## Clarifications

### Session 2026-11-13

- Q: Must the reference browser of the comparison runs be opened by hand? → A: No. For a test case with one browser, the tool can start a headless one.
- Q: Which part of the project starts the headless browser? → A: The engine: a test plan can ask for one headless browser participant, and the run report carries its results.
- Q: What does the run report give for the browser? → A: What the browser measures itself: the heights it received over time, freezes, received bitrate, and the share of time at the requested height, as for the bots. The server's estimates stay in the validation report.
- Q: On which servers can a test plan ask for the headless browser? → A: Any server: the test plan gives the page's address, and the engine only opens it and reads the browser's measures, without joining steps of its own.
- Q: What does a run do when the browser cannot be started, or its page receives no video? → A: The run always goes on: the bots' results are reported, and the browser's part carries a named reason in both cases.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - A browser beside the bots (Priority: P1)

As a load test operator, I want to see, for one room, the server's bandwidth estimate for a browser and for the bots over the same minute, so that I know whether a low estimate comes from the server or from the bots.

**Why this priority**: Nothing can be fixed, or documented as a server limit, before the cause is known. This story alone settles the open question of 014, whatever its answer.

**Independent Test**: On the local mediasoup, run 10 bots sending layered VP8 with half of them asking for 720, with a test plan that also asks for one headless browser, which the engine starts on the load machine and joins to the same room, asking for the same streams. The validation report gives the server's estimate for the browser and for each bot over time, and states the verdict for mediasoup.

**Acceptance Scenarios**:

1. **Given** a room with 10 bots and one browser on the same machine, **When** the run ends, **Then** the server's estimate for the browser and for each bot is known over the whole run, read from the server itself.
2. **Given** these estimates, **When** the browser's estimate reaches what its streams need and some bots' estimates do not, in the same run, **Then** the verdict for that server is "the bots", and the report says which bots and for how long.
3. **Given** these estimates, **When** the browser's estimate is held low in the same way as the bots', **Then** the verdict for that server is "the server", and the report gives the time the browser needed.
4. **Given** a problem that shows in some runs only, **When** the comparison is made, **Then** it is repeated until at least one run shows the problem on a bot, or five runs show none, and the report gives the count.
5. **Given** the browser and a bot do not receive the same streams (the browser's page chooses its own heights), **When** the results are gathered, **Then** the difference is stated, and the comparison is made on the estimate against what each one needs, not on the estimates alone.

---

### User Story 2 - What the bots tell the server (Priority: P2)

As a load test operator, I want each run to say how completely and how regularly the bots reported their reception to the server, so that a low estimate can be traced to the bots' reports without capturing packets by hand.

**Why this priority**: When the verdict of US1 is "the bots", this measure is what finds the fault, and after the fix it is what shows the fault is gone. When the verdict is "the server", it is the evidence that the bots are not at fault.

**Independent Test**: Run 10 bots for 1 minute on one server. The report gives, per bot, the share of arriving packets the bot acknowledged to the server and how often it reported; a run made with a bot that leaves packets unacknowledged shows a lower share for that bot.

**Acceptance Scenarios**:

1. **Given** any run with video, **When** it ends, **Then** the report gives, per bot and for the run, the share of the packets that arrived which the bot acknowledged to the server, the server's probe packets included.
2. **Given** any run with video, **When** it ends, **Then** the report gives how often the bots reported, and how late the latest reports were, so that reports sent in bursts or late are visible.
3. **Given** a bot whose reports were incomplete or late beyond a stated limit, **When** the report is written, **Then** the bot is named with the measure, and layered results of that run carry a warning.
4. **Given** a server or a run where the bots send no such reports (audio only, or a server that does not ask for them), **When** the report is written, **Then** the measure is left out with a named reason, never a zero.

---

### User Story 3 - Bots estimated like a browser (Priority: P3)

As a load test operator, I want the server to estimate a bot's bandwidth as it estimates a browser's on the same path, so that layered runs pass on the servers' default settings and their results describe the server.

**Why this priority**: This is the result the feature is for, but it depends on the verdict of US1. If the verdict is "the server" for a server, this story is met there by documenting the limit (US4), with no change to the bots.

**Independent Test**: On the local Jitsi with its default settings, run 10 bots sending VP8 for 1 minute, five times after a warm-up run: no run freezes. On the local mediasoup, run 10 bots with layers and half asking for 720, five times: the bots asking for 720 receive it at least 95% of the time in every run.

**Acceptance Scenarios**:

1. **Given** a server whose verdict is "the bots", **When** the bots are corrected and the comparison of US1 is run again, **Then** the bots' estimates reach what their streams need no later than 1 second after the browser's does, and none stays below for the rest of the run.
2. **Given** the local mediasoup and a layered run at 10 bots, **When** bots ask for 720, **Then** they receive it at least 95% of their measured time, in every codec the server forwards with layers.
3. **Given** the local Jitsi with the bridge's default settings, **When** 10 bots send VP8 or AV1 for 1 minute, **Then** no stream is suspended and no freeze is counted, the first run after a bridge start set aside.
4. **Given** the remote Jitsi deployment (013), **When** it is deployed after this feature, **Then** it keeps the bridge's default use of its estimate, or the documentation says why the setting stays off.
5. **Given** a run on LiveKit or Janus, where 014 met its criteria, **When** it is run after this feature, **Then** its results are the same as before, within today's run-to-run variation.

---

### User Story 4 - Server limits and lab settings stated (Priority: P4)

As a load test operator, I want what remains a server behaviour written down, and the server settings of each run recorded in its report, so that I do not compare a run with the estimate turned off with a run on default settings.

**Why this priority**: It closes what US1 and US3 leave: a limit that is the server's own, and the gap noted in 014 ("the reports do not record the setting"). The runs are right without it, but they can be misread.

**Independent Test**: Deploy Jitsi on a remote machine with the estimate setting off, run 3 bots, and read the report: it names the setting and its value. Read the documentation: for each server whose verdict is "the server", it gives the behaviour, how long it lasts for a browser, and how to run a test that is not affected.

**Acceptance Scenarios**:

1. **Given** a server deployed with a setting that differs from the server's default and changes results, **When** a run against it ends, **Then** both report files name the setting and its value.
2. **Given** a test plan in which the operator states such a setting for a server deployed by other means, **When** the run ends, **Then** the report gives it as stated by the operator.
3. **Given** a run with no such setting known, **When** the report is written, **Then** it says that the server's settings are not known to the run, and does not say "defaults".
4. **Given** a server whose verdict is "the server", **When** the documentation is read, **Then** it gives the behaviour, the time a browser needs before it receives the requested height, and the run conditions (ramp-up, hold) under which the layered criteria can be met.

---

### Edge Cases

- **The browser changes the room**: it is one more participant, and it may send its own camera. It joins without sending, or its source is counted and stated, so that the bots receive the same streams as in the 014 runs.
- **The browser does not ask for what a bot asks**: a browser's page picks heights from its layout, and a headless browser has no screen: it is given a stated window size. The comparison uses the same room size and the closest request the page allows, and states what differs (US1, scenario 5).
- **The browser and the bots do not share a path**: the browser runs on the load machine, so that both reach the server over the same network. A browser elsewhere is not a valid comparison.
- **The problem does not show**: it appeared on "one bot in a few" in 014. Five runs without it, with a browser present, are reported as such; the verdict is then "not reproduced", not "the server".
- **Both causes at once**: the browser is also held low for a time, and the bots for longer. The verdict gives both, and US3 applies to the part that is the bots'.
- **A cold bridge**: the first Jitsi run after a start is worse for reasons unrelated to the estimate (013). It is set aside by a warm-up run, and never used for a verdict.
- **The load machine is the limit**: with the bots and the server on one laptop, late reports can come from a busy machine. The engine's CPU is given with each comparison, and a verdict of "the bots" is confirmed once with the server on another machine (013).
- **The server gives no estimate to read**: where a server does not expose its estimate per receiver, the comparison uses what it does expose (the layer forwarded over time), and the report says so.
- **The page does not join by itself**: the engine opens the address and nothing more. A page that waits for a click or a name never receives video; the run goes on, the report says the browser received nothing, and no comparison is made from that run.
- **The browser cannot be started**: no browser on the load machine, or it fails to start. The run goes on with the bots, and the report gives the reason in the browser's part.
- **The browser costs CPU**: a browser decodes and renders, which the bots avoid. It is limited to one per run, its CPU is part of the load machine's figure, and the report says a browser was running.
- **The fix costs CPU**: reporting more often or more precisely costs the bots time per packet. The engine's CPU per bot is measured before and after.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: For mediasoup and for Jitsi, the server's bandwidth estimate for a browser and for the bots in the same room MUST be compared over the same run, with the browser on the load machine, in the conditions of the 014 runs that did not meet their criteria (10 bots, layers up to 720, receive cap 4, half of the bots asking for 720 on mediasoup; 10 bots, VP8 and AV1, default bridge settings on Jitsi).
- **FR-002**: The estimates MUST be read from the server, per receiver, at least once per second, and kept with the validation report, with the method used for each server.
- **FR-003**: The validation report MUST state one verdict per server: "the bots", "the server", both, or "not reproduced", with the runs that support it, under the rules of US1.
- **FR-004**: Every run with video MUST report, per bot, the share of arriving packets the bot acknowledged to the server, counting every packet that reaches the bot, the server's probe packets and streams the bot does not use included.
- **FR-005**: Every run with video MUST report how often the bots reported their reception to the server and how late their latest reports were.
- **FR-006**: A bot whose reports were incomplete or late beyond a stated limit MUST be named in the report, and the layered results of that run MUST carry a warning in both report files.
- **FR-007**: Where these measures cannot be taken, the report MUST leave them out with a named reason, never a zero.
- **FR-008**: Where the verdict is "the bots", the bots MUST be corrected so that the server's estimate for a bot reaches what its streams need no later than 1 second after it does for a browser in the same room, and stays there.
- **FR-009**: After the correction, a layered run on mediasoup MUST give the bots asking for 720 that height at least 95% of their measured time at 10 bots, or the part that is the server's MUST be documented under FR-013.
- **FR-010**: After the correction, 10-bot VP8 and AV1 runs on the local Jitsi MUST pass with the bridge's default settings, a warm-up run set aside, or the part that is the server's MUST be documented under FR-013.
- **FR-011**: The remote Jitsi deployment (013) MUST return to the bridge's default use of its estimate once FR-010 is met on a remote machine, or the documentation MUST say why the setting stays off.
- **FR-012**: The run report MUST name the server settings that differ from the server's defaults and change results, when the run knows them: those set by the remote deployment (013), and those the operator states in the test plan. When none is known, the report MUST say the settings are not known.
- **FR-013**: For each server whose verdict includes "the server", the documentation MUST give the behaviour, the time a browser needs to receive the requested height, and the run conditions under which the layered criteria can be met.
- **FR-014**: Runs on LiveKit and Janus, and runs without layers on every server, MUST give the same results as before this feature, within the run-to-run variation seen today.
- **FR-015**: The engine's CPU per bot at 100 bots MUST stay within 5% of its value before this feature, or the difference MUST be measured and stated in the documentation.
- **FR-016**: A test plan MUST be able to ask for one headless browser as a reference participant: the test plan gives the address of a web page, on any server; the engine starts the browser on the load machine, opens that address, reads the browser's measures, and stops the browser at the end of the run. The engine performs no joining step in the page: the address must be one at which the page joins the room by itself. At most one per run.
- **FR-017**: The run report MUST give, apart from the bots' results and in both report files, what the browser measured itself: the heights it received over time, its freezes, its received bitrate, and its share of time at the height the test plan states for it, computed as for the bots (left out with a named reason when no height is stated). The browser is never counted as a bot, and the bots' results are computed without it. The server's estimates of FR-002 are not in the run report: they stay with the validation report.
- **FR-018**: A run whose test plan does not ask for a browser MUST NOT need a browser on the load machine, and MUST behave as before this feature.
- **FR-019**: A browser that cannot be started, or whose page receives no video, MUST NOT stop or fail the run: the bots' results are reported as usual, and the browser's part of the report carries a named reason in place of measures, never a zero.

### Key Entities

- **Receiver estimate**: the server's estimate of the bandwidth available toward one receiver (a bot or the browser), over time, as the server itself gives it; set against what the receiver's requested streams need.
- **Verdict**: per server, where a low estimate comes from: the bots, the server, both, or not reproduced; with the runs behind it.
- **Reception report quality**: per bot and per run, the share of arriving packets acknowledged to the server, how often the bot reported, and how late.
- **Reference browser**: at most one headless browser per run, asked for in the test plan, started and stopped by the engine; a participant of the room that is not a bot, with its own measures in the report (heights received over time, freezes, received bitrate, share of time at the requested height).
- **Server settings**: the settings of the server under test that differ from its defaults and change results (for example, whether the bridge allocates video within its estimate), with where the run learned them: the deployment or the operator.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: For mediasoup and for Jitsi, the validation report gives the server's estimate for one browser and for every bot of a 10-bot room, at least once per second over at least 1 minute, and a verdict supported by at least one run showing the problem or by five runs without it.
- **SC-002**: In 100% of runs with video, the report gives per bot the share of arriving packets acknowledged and the timing of its reports, or a named reason; a bot made to leave packets unacknowledged is named in its run's report.
- **SC-003**: Where the verdict is "the bots": after the correction, in five runs out of five with a browser in the room, every bot's estimate reaches what its streams need within 1 second of the browser's, and no bot stays below it.
- **SC-004**: On the local mediasoup, bots asking for 720 receive it at least 95% of their measured time in five 1-minute, 10-bot layered VP8 runs out of five, and in one run of each other codec the server forwards with layers; or the remaining gap is shown to be the same for a browser.
- **SC-005**: On the local Jitsi with the bridge's default settings, five 1-minute, 10-bot runs out of five in VP8, and five in AV1, end with 0 freezes and no suspended stream, a warm-up run set aside; or the remaining suspensions are shown to happen to a browser too.
- **SC-006**: A verdict of "the bots" is confirmed by one comparison with the server on another machine.
- **SC-007**: In 100% of runs against a server deployed with `sfu-remote`, the report names the settings the deployment changed from the server's defaults; a run with no known setting says so.
- **SC-008**: LiveKit and Janus runs of 014 (10 bots, layered, each codec) give the same shares of time at the requested height as in 014, within 0.02, and the engine's CPU per bot at 100 bots is within 5% of before, or the difference is documented.
- **SC-009**: The open items of 014 on mediasoup heights (T053, T059, T060) and on the local Jitsi estimate (T035, T044, T064) each end as corrected, or as a documented server behaviour with a browser measurement.

## Assumptions

- The two servers in question are mediasoup and Jitsi, where 014 left results open. LiveKit and Janus met their criteria in 014 and are only checked for no change.
- The reference browser is a current Chrome on the load machine, on the page whose address the test plan gives; for the comparison runs, each server's own page (the mediasoup demo page, the Jitsi Meet page), at an address that joins the room directly. It is a headless one that the engine starts when the test plan asks for it; one the operator opens stays valid for a comparison, without results in the run report. One browser is enough; Firefox is not compared.
- "What its streams need" is the sum of the bitrates of the streams a receiver asked for, from the content's recorded bitrates (014), plus the margin the server applies; the exact figure per server is a planning matter.
- The bots do not estimate bandwidth themselves and do not adapt what they send: this feature is about what the bots report to the server and how the server estimates from it. Limiting bandwidth and simulating an unstable network stay out of scope (README, "Not done yet").
- The comparison runs are part of validation. Their one browser is asked for in the test plan, so that a comparison can be repeated without the operator.
- Constitution, principle I: browser clients "may be added later". One reference browser per run is such an addition; it is not a way to generate load, and the plan must justify it against principles I and IV.
- Servers are not modified: only their configuration is touched, and only to return to defaults.
- If the verdict is "the server" for both, no change is made to how the bots report: US2 and US4 are still delivered, and US3 is met by documentation.
- The 95% share and the freeze rule are those of 006 and 014, unchanged.
- The 1-second margin of FR-008 allows for the browser and the bots joining at slightly different times; it is not a claim about the servers' estimators.
