# Feature Specification: Synthetic Media Sending and Media Metrics

**Feature Branch**: `004-synthetic-media`

**Created**: 2026-10-10

**Status**: Draft (amended 2026-10-10: receive cap, US3 / FR-015, after the scale measurements of T038/T039)

**Input**: User description: "Make the bots send audio and video, and report media-quality metrics, so that the media server carries real load (follow-up to 003, where bots only receive)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Bots publish audio and video (Priority: P1)

As a load test operator, I want each simulated participant to send an audio stream and a video stream into the conference, so that the media server has to receive, route and forward real media, as it would for human participants.

**Why this priority**: Today the bots only join and receive, so a test measures signaling and connection setup but not media routing, which is the main load on the media server. Without this, the engine cannot answer "how many participants can this deployment carry".

**Independent Test**: Run a scenario with 2 sending bots against a test deployment, join the same room from a browser and check that both bots appear with a moving video and an audible sound. Can be verified without any new metric.

**Acceptance Scenarios**:

1. **Given** a scenario whose media profile is audio and video, **When** a bot joins the room, **Then** it starts sending one audio stream and one video stream once its media connection is established.
2. **Given** a sending bot in a room, **When** a human participant joins the same room from a browser, **Then** the human sees the bot's video and hears its audio within 5 seconds.
3. **Given** a scenario whose media profile is audio only, **When** the bots join, **Then** they send audio and no video.
4. **Given** a scenario with no media profile set, **When** it runs, **Then** the bots behave as before this feature (receive only), so existing scenarios are unchanged.
5. **Given** sending bots, **When** the scenario ends or is stopped, **Then** their streams stop and disappear from the other participants' view along with the bots.

---

### User Story 2 - Measure media received by bots (Priority: P2)

As a load test operator, I want each scenario to report how much media the bots send and receive and with what quality (loss, jitter, round-trip time), so that I can tell whether the media server keeps up as the number of participants grows.

**Why this priority**: Sending media creates the load; measuring what comes back is what turns a run into a result. It depends on Story 1 for meaningful numbers but also has value for receive-only runs with human or other senders.

**Independent Test**: Run a scenario with sending bots and read the metrics endpoint: sent and received volumes, packet loss, jitter and round-trip time are present for the scenario and move during the run.

**Acceptance Scenarios**:

1. **Given** a running scenario, **When** the operator reads the metrics endpoint, **Then** it shows, for that scenario, the media sent and received (packets and bytes) split by audio and video.
2. **Given** a running scenario, **When** the operator reads the metrics endpoint, **Then** it shows the packet loss, jitter and round-trip time observed by the bots for that scenario.
3. **Given** a scenario on a loss-free local network, **When** N bots send media in one room, **Then** the reported received media matches what the media server is expected to forward (each bot receives the streams of the others it is subscribed to), within 10%.
4. **Given** a bot whose incoming media stops for longer than a configurable timeout while its connection is still up, **When** the timeout elapses, **Then** the stall is counted and logged with the scenario and peer identifiers.

---

### User Story 3 - Sending at scale on one machine (Priority: P3)

As a load test operator, I want a single load machine to run hundreds of sending bots, so that I can load a deployment without a fleet of load generators.

**Why this priority**: The engine's purpose is load generation; this story guards the efficiency that makes the first two useful at scale.

**Independent Test**: Run 100 sending bots (audio and video) against the local test deployment for 10 minutes and observe the load machine's CPU and memory, and the bots' send regularity.

**Acceptance Scenarios**:

1. **Given** 100 bots with audio and video, **When** they run for 10 minutes, **Then** all of them keep sending on schedule (see SC-003) and none fails because of the load machine.
2. **Given** the same run, **When** the operator compares the load machine's CPU with a receive-only run of the same size, **Then** the increase stays within the budget in SC-004.
3. **Given** a scenario that caps each bot at N received streams per media kind, **When** more than N other participants send that kind, **Then** each bot receives N of them, ignores the others, and receives the next waiting stream when one of its N stops.

In a large room the media server forwards every participant's audio to every bot (up to N−1 streams each), and processing them all on one load machine exceeds its budget (measured: 3.85 cores for 100 bots receiving 99 audio streams each). Real clients in large conferences also receive only part of the streams. The cap keeps the load machine's work bounded while the bots still send everything.

---

### Edge Cases

- **Media server rejects or ignores a stream** (e.g. a codec it does not accept): the bot reports it in the logs and the metrics (nothing received by others); the bot stays connected and the scenario continues.
- **Offer without a matching media kind** (e.g. the server offers no video): the bot sends only the kinds the offer allows and logs which ones were skipped.
- **Ramp-up**: bots that join later start sending when their own connection is established, without disturbing those already sending.
- **Server limits the number of forwarded streams** (e.g. last-N): received volumes are lower than N×(N-1); this is expected and visible in the metrics, not an error.
- **Connection drops while sending**: sending stops, the peer follows the existing lifecycle (failed / disconnected) and its media counters stop growing.
- **Receive cap reached**: streams beyond the cap are ignored without errors. Before feature 005 the server still sent them (the cap bounded the load machine's processing, not the network traffic or the server's work); since 005, bots on a platform with a bridge channel subscribe to their received streams only, and the server stops sending the others.
- **Very short scenarios**: a bot stopped before its media connection is established sends nothing and reports zero, without errors.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: A scenario MUST let the operator choose a media profile for its bots: receive only, audio only, or audio and video.
- **FR-002**: When no media profile is set, the bots MUST behave as receive only, so existing scenario definitions keep their current behavior.
- **FR-003**: A sending bot MUST send its streams continuously from the moment its media connection is established until it leaves, at the content's nominal rate.
- **FR-004**: Media content MUST be pre-encoded or synthetic and looped; the engine MUST NOT encode media in real time.
- **FR-005**: The engine MUST ship with default audio and video content so that a sending scenario needs no media files from the operator.
- **FR-006**: Every bot MUST send the same content on a repeatable schedule, so that two runs of the same scenario produce the same sent media.
- **FR-007**: A bot's streams MUST be announced to the conference through the platform's signaling, so that other participants (bots and humans) receive them as they would from a regular participant.
- **FR-008**: The media sending MUST stay independent of the platform: the same media profile works for any platform adapter, and only the adapter knows how streams are announced.
- **FR-009**: The engine MUST export, per scenario and per media kind (audio, video), the packets and bytes sent and received by its bots.
- **FR-010**: The engine MUST export, per scenario, the packet loss, jitter and round-trip time observed by its bots.
- **FR-011**: Media metrics MUST use the same scenario identifier as the existing signaling and connection metrics, so they can be correlated.
- **FR-012**: The engine MUST detect a bot whose incoming media stalls while its connection is up, count it per scenario and log it with the scenario and peer identifiers. The stall timeout MUST be configurable, with a default of 10 seconds.
- **FR-013**: Media metrics MUST NOT use individual peers as labels; per-peer detail goes to the structured logs.
- **FR-014**: A media problem on one bot (a rejected stream, a stall) MUST NOT fail the scenario on its own; only lifecycle failures (connection, negotiation, transport) do, as today.
- **FR-015**: A scenario MUST let the operator cap the number of remote streams each bot receives per media kind (default: no cap). Beyond the cap, a bot ignores the extra streams: they are not processed, not counted as received or lost, and not counted as remote streams. Streams are received in the order the platform announces them; when a received stream ends, the oldest waiting one takes its place.

### Key Entities

- **Media Profile**: The scenario-level choice of what each bot sends: receive only, audio only, audio and video. Part of the test plan; defaults to receive only.
- **Media Source**: The looped content a bot sends for one media kind, with its nominal rate. Shared by all bots of a scenario.
- **Media Statistics**: Per-scenario counters and measures of sent and received media (packets, bytes, loss, jitter, round-trip time, stalls), aggregated from the bots.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With 2 sending bots in a room, a human participant joining from a browser sees both videos and hears both audio streams within 5 seconds of joining, in 100% of attempts against the test deployment.
- **SC-002**: On a loss-free local network, the received media reported for a scenario of N bots (N between 2 and 50) is within 10% of what the media server is expected to forward.
- **SC-003**: For 100 bots sending audio and video for 10 minutes on one machine, at least 99% of media packets are sent within 20 ms of their scheduled time.
- **SC-004**: The same 100-bot run, with each bot receiving at most 10 streams per media kind, uses no more than 2 CPU cores on average on the load machine and completes with every bot established and no bot failed.
- **SC-005**: An operator can tell from the metrics alone, within 30 seconds, whether a run's media is healthy (volumes sent and received, loss, stalls), without reading logs.

## Assumptions

- The receive cap is enforced by the bots, not negotiated with the media server, in this feature: the server keeps forwarding every stream and the bots drop the ones beyond the cap as early as possible. Feature 005 adds the platform's subscription messages over its bridge channel, so the server stops sending the extra streams; the bot-side enforcement stays as a safety net.

- The test deployment is the local Jitsi Meet instance (`https://localhost:8443/IntriguingClimatesSupposeProbably`); multi-bot runs only target deployments the operator controls.
- Jitsi is the only platform adapter today; FR-007 is implemented for it, and later adapters provide their own announcement.
- Default content uses codecs that common media servers accept without transcoding (an Opus audio stream and a VP8 video stream at a modest resolution and bitrate, around 30 kbit/s audio and 500 kbit/s video). Operator-supplied media files, simulcast, multiple quality layers and bandwidth adaptation are out of scope for this feature.
- Bots do not decode or render received media; reception quality is measured from transport statistics only (constitution principle IV).
- Media metrics are aggregated per scenario, as the existing metrics are (FR-013); per-peer analysis uses the logs.
- Mute, unmute, screen sharing and dynamic join/leave patterns during a run are out of scope.
- Configuring the stall timeout lives in the test plan next to the media profile.
