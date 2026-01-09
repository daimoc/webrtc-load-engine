# Tasks: XMPP Signaling Logic

**Branch**: `003-xmpp-signaling`
**Spec**: [specs/003-xmpp-signaling/spec.md](spec.md)
**Status**: Draft

## Dependencies

1. **Phase 1: Setup**
2. **Phase 2: Foundational** (Prerequisite for all stories)
3. **Phase 3: Connect and Join** (User Story 1)
4. **Phase 4: Negotiate Jingle** (User Story 2)
5. **Phase 5: Disconnect** (User Story 3)
6. **Phase 6: Polish**

---

## Phase 1: Setup
**Goal**: Initialize project dependencies and structure for XMPP signaling.

- [x] T001 Add `gosrc.io/xmpp` dependency to `go.mod`
- [x] T002 Create directory `src/platform/jitsi` if it doesn't exist (should exist from previous feature)
- [x] T003 Create file `src/platform/jitsi/xmpp_structs.go` with package declaration
- [x] T004 Create file `src/platform/jitsi/signaling.go` with package declaration
- [x] T005 Create file `src/platform/jitsi/jingle.go` with package declaration
- [x] T006 Update `src/platform/jitsi/adapter.go` to import `gosrc.io/xmpp`

## Phase 2: Foundational
**Goal**: Define core data structures and configuration parsing.

- [x] T007 Define `JingleConfig` struct in `src/platform/jitsi/adapter.go` matching `specs/003-xmpp-signaling/contracts/configuration.yaml`
- [x] T008 [P] Define `JingleIQ`, `JingleContent`, `JingleDescription`, `JingleTransport` structs in `src/platform/jitsi/xmpp_structs.go` per `data-model.md`
- [x] T009 [P] Define `IceUdpTransport` and `RtpDescription` structs in `src/platform/jitsi/xmpp_structs.go` for XEP-0176 and XEP-0167 mapping
- [x] T010 Implement `ParseConfig` method in `src/platform/jitsi/adapter.go` to validate and extract `platform_config` from TestPlan

## Phase 3: Connect and Join (User Story 1)
**Goal**: Establish XMPP connection and join MUC.
**Independent Test**: Verify "bot" appears in Jitsi meeting participant list.

- [x] T011 [US1] Implement `Connect` method in `src/platform/jitsi/signaling.go` using `gosrc.io/xmpp` to establish WebSocket connection
- [x] T012 [US1] Implement anonymous SASL authentication logic in `Connect` method in `src/platform/jitsi/signaling.go`
- [x] T013 [US1] Implement `JoinMUC` method in `src/platform/jitsi/signaling.go` to send presence stanza to `room@muc.domain`
- [x] T014 [US1] Implement presence handler in `src/platform/jitsi/signaling.go` to detect when MUC join is successful (self-presence)
- [x] T014a [US1] Configure XMPP client keepalive settings in `src/platform/jitsi/signaling.go`
- [x] T015 [US1] Update `src/platform/jitsi/adapter.go` to call `Connect` and `JoinMUC` in `Start()`
- [x] T016 [US1] Add log output in `src/platform/jitsi/signaling.go` when joined to MUC (for verification)

## Phase 4: Negotiate Jingle (User Story 2)
**Goal**: Handle Jingle session initiation and SDP exchange.
**Independent Test**: Verify transition to "Established" state in engine and SDP exchange in logs.

- [x] T017 [US2] Implement `JingleSession` struct in `src/platform/jitsi/jingle.go` to track state (PENDING, ACTIVE, ENDED)
- [x] T018 [US2] Create `HandleIQ` dispatcher in `src/platform/jitsi/signaling.go` to route `jingle` action stanzas
- [x] T019 [US2] Implement `handleSessionInitiate` in `src/platform/jitsi/jingle.go` to parse incoming offer and create `JingleSession`
- [x] T020 [US2] Implement helper `convertJingleToSDP` in `src/platform/jitsi/jingle.go` to map XML to `pion/webrtc.SessionDescription`
- [x] T021 [US2] Implement helper `convertSDPToJingle` in `src/platform/jitsi/jingle.go` to map `pion/webrtc.SessionDescription` to XML
- [x] T022 [US2] Implement `SendSessionAccept` in `src/platform/jitsi/jingle.go` to send `session-accept` stanza with local SDP
- [x] T023 [US2] Integrate with `platform.Adapter` interface: Call `peer.SetRemoteDescription` and `peer.CreateAnswer` in `handleSessionInitiate` flow
- [x] T024 [US2] Implement `HandleTransportInfo` in `src/platform/jitsi/jingle.go` to process incoming Trickle ICE candidates
- [x] T025 [US2] Implement `SendTransportInfo` in `src/platform/jitsi/jingle.go` to send local ICE candidates

## Phase 5: Disconnect (User Story 3)
**Goal**: Cleanly terminate sessions and connection.
**Independent Test**: Verify participant disappears immediately after test ends.

- [x] T026 [US3] Implement `SendSessionTerminate` in `src/platform/jitsi/jingle.go`
- [x] T027 [US3] Implement `Disconnect` method in `src/platform/jitsi/signaling.go` to close WebSocket connection
- [x] T028 [US3] Update `Stop()` in `src/platform/jitsi/adapter.go` to trigger `SendSessionTerminate` and then `Disconnect`

## Phase 6: Polish
**Goal**: Error handling, logging, and code cleanup.

- [x] T029 Add structured logging (slog) to all XMPP stanza handlers in `src/platform/jitsi/signaling.go`
- [x] T030 Implement error handling for connection failures and MUC kick events
- [x] T031 Run `go mod tidy` to clean up dependencies
- [x] T032 Verify compliance with `SC-004` (Standard Jitsi signaling patterns) by reviewing logs against `research.md` flows

## Implementation Strategy

1. **MVP (Phase 1-3)**: Focus solely on getting the bot to appear in the meeting (Join MUC). This proves connectivity.
2. **Negotiation (Phase 4)**: Add the Jingle layer to establish the "call". This proves we can talk to Jicofo/JVB.
3. **Cleanup (Phase 5-6)**: Ensure we don't leak resources.

## Parallel Execution Examples

- **Data Structures**: `T008` (Jingle structs) and `T009` (ICE/RTP structs) can be done in parallel with `T007` (Config).
- **Handlers**: `T019` (Session Initiate) and `T024` (Transport Info) are independent handlers.
