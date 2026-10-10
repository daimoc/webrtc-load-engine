# Feature Specification: Bridge Channel Subscriptions

**Feature Branch**: `005-bridge-channel-subscriptions`

**Created**: 2026-10-10

**Status**: Draft (amended 2026-10-10 in planning: FR-007 and US2 scenario 4, subscriptions are not acknowledged by the server, research R5)

**Input**: User description: "Bots talk to the media server over its bridge channel (Jitsi Colibri messages over the "JVB data channel", now opened by every bot since 003 T056): (1) send receiver video constraints so the bridge forwards video to the bots (video is currently never forwarded to them, so SC-002 of 004 cannot be checked for video); (2) subscribe only to the audio and video streams a bot receives under the receive cap of 004 (FR-015), so the bridge stops sending the streams the bot ignores, bringing 100 bots under the 2-core budget of 004 SC-004 (2.60 cores measured with the cap, most of it spent on packets the bridge sends and the bots drop). Validate on the local docker-jitsi-meet."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Bots receive video (Priority: P1)

As a load test operator, I want the bots to ask the media server for the video of the participants they receive, as real clients do, so that the server forwards video to them and a test measures video forwarding, not only audio.

**Why this priority**: Today the media server forwards no video at all to the bots, because they never tell it what video they want. A video conference load test that loads only half of the server's forwarding work, and cannot check video reception, misses its main purpose.

**Independent Test**: Run 10 sending bots for 30 seconds; each bot receives the video of the other bots it is offered, and the received video volume matches what they send within 10%.

**Acceptance Scenarios**:

1. **Given** a running scenario where other participants send video, **When** a bot's connection to the media server is up, **Then** it receives video from each remote video stream it is offered.
2. **Given** a bot receiving video, **When** a new participant starts sending video, **Then** the bot receives it too, within 5 seconds.
3. **Given** a scenario whose bots receive at most N streams per kind (004 receive cap), **When** more participants send video, **Then** each bot receives video only from its N streams.
4. **Given** a media server that does not offer the bridge channel, **When** the scenario runs, **Then** bots behave as before this feature (no video received, everything else unchanged) and the situation is reported once per bot in the logs.

---

### User Story 2 - The server sends only what the bots receive (Priority: P1)

As a load test operator, I want the media server to send each bot only the streams that bot receives under the receive cap, so that a single load machine can run 100 bots in one large room within its budget, and the server's forwarding load reflects real clients that do not receive everyone.

**Why this priority**: With the cap alone, the server still sends each bot all the other participants' audio and the bots drop it: 100 bots cost 2.60 cores, over the 2-core budget, and the server does work that real clients would not ask for.

**Independent Test**: Run 100 sending bots with a receive cap of 10 for 10 minutes; the traffic the server sends to each bot matches 10 audio and 10 video streams, and the load machine stays within 2 cores.

**Acceptance Scenarios**:

1. **Given** bots with a receive cap of N, **When** more than N participants send audio, **Then** the server sends each bot no more than N audio streams.
2. **Given** a bot receiving N streams, **When** one of them stops and a waiting stream takes its place (004 FR-015), **Then** the bot's subscription follows, and the server sends the new stream instead of the old one.
3. **Given** bots without a receive cap, **When** the scenario runs, **Then** they subscribe to every stream, as before.
4. **Given** a media server that cannot restrict what it sends to a client, **When** bots run with a cap, **Then** they still drop the extra streams themselves (004 behavior), and the metrics show the traffic the server sends beyond the received streams.

---

### Edge Cases

- **Bridge channel opens late or reopens**: the bots' preferences are sent as soon as it is open, and again after it reopens; nothing is lost if it opens after media started.
- **Preferences change faster than the channel can carry them** (bursts of participants joining): the server ends up with the latest preferences; intermediate ones may be skipped.
- **The server answers with an error or ignores a message**: the bot keeps running, logs it once, and the metrics show what it actually receives.
- **Participants who send only audio, or only video**: subscriptions cover what each participant sends; nothing is requested for media a participant does not send.
- **Messages the server sends to the bots** (speaker changes, statistics, forwarded sources): they are accepted without errors; using them is out of scope except where needed for the requirements below.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Each bot MUST tell the media server, over the bridge channel when it is open, which video it wants to receive, so that the server forwards video to it.
- **FR-002**: Bots MUST request video of each remote video stream they receive, up to the resolution the bots themselves send (360 lines), and none of the streams they do not receive.
- **FR-003**: When a receive cap is set (004 FR-015), each bot MUST subscribe, for audio and for video, to the streams it receives under the cap only, so that the server does not send it the others.
- **FR-004**: Bots without a receive cap MUST subscribe to every remote stream (unchanged forwarding for audio; video as in FR-002).
- **FR-005**: A bot's subscriptions MUST follow the streams it receives: when a stream is added, removed or replaced by a waiting one, the server is told within 2 seconds.
- **FR-006**: Preferences MUST be (re)sent when the bridge channel opens or reopens, and only the latest preferences matter.
- **FR-007**: Bots MUST keep the 004 bot-side cap in every case, so that a server that ignores subscriptions changes nothing else. When the bridge channel is not offered, does not open or fails, bots MUST keep working with the 004 behavior (no video requested) and log the reason once per bot. Since the server does not acknowledge subscriptions, whether it honors them MUST be visible in the metrics: the engine exports, per scenario, the traffic the bots receive from the server in total, next to the media they keep.
- **FR-008**: The engine MUST export, per scenario, how many bots have an open bridge channel, so that an operator can tell whether subscriptions are in effect.
- **FR-009**: Messages received from the server over the bridge channel MUST NOT cause errors or failures; unknown messages are ignored.
- **FR-010**: The bridge channel and its message formats MUST stay inside the platform adapter: the media and execution layers only express which streams a bot receives (constitution III).

### Key Entities

- **Receive Preferences**: per bot, the remote streams it receives (per kind) and the video quality it asks for. Derived from the streams it is offered and the receive cap; owned by the platform adapter, which turns them into the platform's messages.
- **Bridge Channel State**: per bot, whether the channel to the media server is open, and whether the server accepted each kind of subscription.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With 10 sending bots in one room for 30 seconds, each bot receives video from the 9 others, and the received video volume is within 10% of what they send (completes 004 SC-002 for video).
- **SC-002**: With 100 sending bots and a receive cap of 10 for 10 minutes on one machine, the load machine uses no more than 2 CPU cores on average (004 SC-004), every bot stays established, and each bot receives 10 audio and 10 video streams.
- **SC-003**: In the same run, the media traffic the server sends to each bot corresponds to its 10 audio and 10 video streams, within 10% (the server no longer sends the ignored streams).
- **SC-004**: When a received stream ends and a waiting one replaces it, the bot receives the new stream within 5 seconds, in 100% of replacements observed in a run.
- **SC-005**: On a deployment without the bridge channel, a scenario runs with the same results as before this feature (no regression).

## Assumptions

- The validation deployment is the local docker-jitsi-meet (`https://localhost:8443/IntriguingClimatesSupposeProbably`), whose Jicofo offers the bridge channel to the bots since 003 T056.
- The bots send a single 360-line video stream (004), so asking for more than 360 lines brings nothing; the requested video quality is fixed at 360 lines in this feature rather than configurable.
- Restricting audio per receiver depends on the media server version; if the local server does not support it, US2 can only be met for video and the gap is documented (FR-007).
- The order in which streams are received under the cap is the one defined by 004 FR-015 (announcement order, oldest waiting stream replaces an ended one); this feature does not add speaker-based selection.
- Using the server's own messages (dominant speaker, statistics) for anything other than the requirements above is out of scope.
- The CPU budget is measured with the media server on the same machine as the bots, as in 004; the server's own load is reported but not budgeted.
