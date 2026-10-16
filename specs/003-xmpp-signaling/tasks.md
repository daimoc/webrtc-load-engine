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

- [x] T001 Add XMPP dependency to `go.mod` (`mellium.im/xmpp`; replaced the initial `gosrc.io/xmpp` in 1aed684)
- [x] T002 Create directory `src/platform/jitsi` if it doesn't exist (should exist from previous feature)
- [x] T003 Create file `src/platform/jitsi/xmpp_structs.go` with package declaration
- [x] T004 Create file `src/platform/jitsi/signaling.go` with package declaration
- [x] T005 Create file `src/platform/jitsi/jingle.go` with package declaration
- [x] T006 Update `src/platform/jitsi/adapter.go` to wire the XMPP `SignalingClient`

## Phase 2: Foundational
**Goal**: Define core data structures and configuration parsing.

- [x] T007 Define `JingleConfig` struct in `src/platform/jitsi/adapter.go` matching `specs/003-xmpp-signaling/contracts/configuration.yaml`
- [x] T008 [P] Define `JingleIQ`, `JingleContent`, `JingleDescription`, `JingleTransport` structs in `src/platform/jitsi/xmpp_structs.go` per `data-model.md`
- [x] T009 [P] Define `IceUdpTransport` and `RtpDescription` structs in `src/platform/jitsi/xmpp_structs.go` for XEP-0176 and XEP-0167 mapping
- [x] T010 Implement `ParseConfig` method in `src/platform/jitsi/adapter.go` to validate and extract `platform_config` from TestPlan
- [x] T010a Add `TestPlan.PlatformConfig` in `src/scenario/scenario.go` and call `ParseConfig` from `MainEngineStarter` in `src/main.go` so API-driven scenarios configure the adapter

## Phase 3: Connect and Join (User Story 1)
**Goal**: Establish XMPP connection and join MUC.
**Independent Test**: Verify "bot" appears in Jitsi meeting participant list.

- [x] T011 [US1] Implement `Connect` method in `src/platform/jitsi/signaling.go` using `mellium.im/xmpp` to establish WebSocket connection
- [x] T012 [US1] Implement anonymous SASL authentication logic in `Connect` method in `src/platform/jitsi/signaling.go`
- [x] T013 [US1] Implement `JoinMUC` method in `src/platform/jitsi/signaling.go` to send presence stanza to `room@muc.domain`
- [x] T014 [US1] Implement presence handler in `src/platform/jitsi/signaling.go` to detect when MUC join is successful (self-presence)
- [X] T014a [US1] Configure XMPP client keepalive settings in `src/platform/jitsi/signaling.go` (not implemented: no XEP-0199 ping or whitespace keepalive is sent)
- [x] T015 [US1] Update `src/platform/jitsi/adapter.go` to call `Connect` and `JoinMUC` in `Start()`
- [x] T016 [US1] Add log output in `src/platform/jitsi/signaling.go` when joined to MUC (for verification)

## Phase 4: Negotiate Jingle (User Story 2)
**Goal**: Handle Jingle session initiation and SDP exchange.
**Independent Test**: Verify transition to "Established" state in engine and SDP exchange in logs.

- [x] T017 [US2] Implement `JingleSession` struct in `src/platform/jitsi/jingle.go` to track state (PENDING, ACTIVE, ENDED)
- [x] T018 [US2] Create `HandleIQ` dispatcher in `src/platform/jitsi/signaling.go` to route `jingle` action stanzas
- [x] T019 [US2] Implement `handleSessionInitiate` in `src/platform/jitsi/jingle.go` to parse incoming offer and create `JingleSession`
- [x] T020 [US2] Implement helper `convertJingleToSDP` in `src/platform/jitsi/jingle.go` to map XML to `pion/webrtc.SessionDescription` (partial: `rtpmap` lacks clockrate/channels, no BUNDLE/fmtp/rtcp-fb/ssrc)
- [x] T021 [US2] Implement helper `convertSDPToJingle` in `src/platform/jitsi/jingle.go` to map `pion/webrtc.SessionDescription` to XML (partial: emits no payload-types, no `setup` on fingerprint, no rtcp-mux)
- [x] T022 [US2] Implement `SendSessionAccept` in `src/platform/jitsi/jingle.go` to send `session-accept` stanza with local SDP
- [x] T023 [US2] Integrate with `platform.Adapter` interface: Call `peer.SetRemoteDescription` and `peer.CreateAnswer` in `handleSessionInitiate` flow
- [X] T023a [US2] Reply with an IQ `result` to handled `session-initiate` / `transport-info` stanzas in `handleIQ` (Jicofo expects the ack)
- [X] T023b [US2] Send the Jicofo conference request (`http://jitsi.org/protocol/focus`) before joining the MUC so that Jicofo sends `session-initiate`
- [x] T024 [US2] Implement `HandleTransportInfo` in `src/platform/jitsi/jingle.go` to process incoming Trickle ICE candidates (candidates are forwarded, but `execution.Peer.OnICECandidate` still drops them — see T024a)
- [X] T024a [US2] Apply incoming candidates with `pc.AddICECandidate` in `src/execution/peer.go`
- [X] T025 [US2] Implement `SendTransportInfo` in `src/platform/jitsi/jingle.go` to send local ICE candidates (currently a stub returning nil)

## Phase 5: Disconnect (User Story 3)
**Goal**: Cleanly terminate sessions and connection.
**Independent Test**: Verify participant disappears immediately after test ends.

- [x] T026 [US3] Implement `SendSessionTerminate` in `src/platform/jitsi/jingle.go`
- [x] T027 [US3] Implement `Disconnect` method in `src/platform/jitsi/signaling.go` to close WebSocket connection
- [x] T028 [US3] Update `Stop()` in `src/platform/jitsi/adapter.go` to trigger `SendSessionTerminate` and then `Disconnect`

## Phase 6: Polish
**Goal**: Error handling, logging, and code cleanup.

- [x] T029 Add structured logging (slog) to all XMPP stanza handlers in `src/platform/jitsi/signaling.go`
- [X] T030 Implement error handling for connection failures and MUC kick events (join is latched on any presence; self-presence 110 and kick 307/301 status codes are not checked)
- [x] T031 Run `go mod tidy` to clean up dependencies
- [X] T032 Verify compliance with `SC-004` (Standard Jitsi signaling patterns) by reviewing logs against `research.md` flows (verified 2026-10-08 on rendez-vous.renater.fr with JWT auth: conference request → MUC join → session-initiate/accept → ICE+DTLS established → session-terminate/leave; rerun with `JITSI_ROOM_URL=https://<domain>/<room> JITSI_LOG_XMPP=1 go test -tags live -v ./tests/live/`, a `jwt` query parameter or `JITSI_TOKEN` where auth is required)

## Implementation Strategy

1. **MVP (Phase 1-3)**: Focus solely on getting the bot to appear in the meeting (Join MUC). This proves connectivity.
2. **Negotiation (Phase 4)**: Add the Jingle layer to establish the "call". This proves we can talk to Jicofo/JVB.
3. **Cleanup (Phase 5-6)**: Ensure we don't leak resources.

## Parallel Execution Examples

- **Data Structures**: `T008` (Jingle structs) and `T009` (ICE/RTP structs) can be done in parallel with `T007` (Config).
- **Handlers**: `T019` (Session Initiate) and `T024` (Transport Info) are independent handlers.

## Phase 7: Convergence

- [X] T033 CRITICAL: Enforce the peer lifecycle state machine in `src/execution/peer.go` (reject illegal transitions such as `disconnected`→`connected`, return/log them as errors) per Constitution V (contradicts)
- [X] T034 CRITICAL: Add Jitsi adapter contract tests in `src/platform/jitsi/` covering `convertJingleToSDP`, `convertSDPToJingle`, `handleIQ` dispatch, `ParseConfig` defaults/validation and adapter Connect/Disconnect against a fake XMPP stream per Constitution (Test Discipline) (missing)
- [X] T035 CRITICAL: Export per-peer signaling metrics (XMPP connect, MUC join, Jingle negotiation duration and failures) labelled with scenario and peer IDs so signaling and ICE events are correlatable per Constitution (Observability) (missing)
- [X] T036 Complete `convertJingleToSDP` in `src/platform/jitsi/jingle.go`: rtpmap clockrate/channels, `a=group:BUNDLE`, fmtp, rtcp-fb, ssrc/ssrc-group, `setup` from fingerprint, correct candidate foundation/generation, so Pion accepts the offer per FR-006 (partial)
- [X] T037 Complete `convertSDPToJingle` in `src/platform/jitsi/jingle.go`: payload-types with parameters, rtcp-mux, fingerprint `setup`, `creator`/`responder` set correctly, session-level attributes read once per FR-006 (partial)
- [X] T038 Wait for ICE gathering (or gather with timeout) before building the answer and include local candidates in `session-accept` per US2/AC1 (partial)
- [X] T039 Report Jingle session establishment (session-accept sent + ICE/DTLS connected) to the engine and map it to the peer lifecycle instead of marking peers `connected` on MUC join per US2/AC3 (partial)
- [X] T040 Make `Adapter.Connect`/`Disconnect` in `src/platform/jitsi/adapter.go` race-free and leak-free: register the client before join, close it when `JoinMUC`/`WaitJoined` fail, honour engine cancellation via a caller context per US3/AC2 (contradicts)
- [X] T041 Propagate XMPP stream loss from `serve()` in `src/platform/jitsi/signaling.go` to the peer (fail with a clear error or reconnect) per Edge Case: Network Flaps / Connection Failure (partial)
- [X] T042 On failed `session-initiate` processing, reply with an IQ error / `session-terminate`, remove the `PENDING` session, and guard `RemoteSDP` writes with the mutex per data-model Session Lifecycle (partial)
- [X] T043 Add an explicit `DISCONNECTED`/`CONNECTED`/`JOINED` state to `SignalingClient` per data-model XMPPAdapter.State (partial)
- [X] T044 Use the configured `connect_timeout` instead of hard-coded 20s/10s timeouts in `src/platform/jitsi/adapter.go` and `signaling.go` per contracts/configuration.yaml (partial)
- [X] T045 Close the peer's `PeerConnection` on disconnect in `src/execution/peer.go` so media resources are released per US3 (partial)
- [X] T046 Reconcile the `log_xmpp` config field with the contract (map it to `debug` or add it to `contracts/configuration.yaml`) per contracts/configuration.yaml (unrequested)
- [X] T047 Send one complete stanza per WebSocket message (and `<close/>` instead of `</stream:stream>`) per RFC 7395, as Prosody drops the connection on fragmented stanzas such as session-accept, in `src/platform/jitsi/framing.go` (found by T032)
- [X] T048 Decline P2P `session-initiate` offers not sent by the focus occupant (`<room>/focus`) with `session-terminate` reason `decline`, in `src/platform/jitsi/jingle.go` (found by T032)
- [X] T049 Add a live smoke test against a real Jitsi deployment in `tests/live/jitsi_live_test.go` (build tag `live`), checking the established state through the `webrtc_peers` metric
- [X] T050 Join the MUC with the occupant nick Jitsi's Prosody accepts (`mod_muc_resource_validate`: first 8 characters of the anonymous JID, as lib-jitsi-meet) and carry the peer ID as XEP-0172 display name, in `src/platform/jitsi/signaling.go` (found on the local docker-jitsi-meet: "Invalid resource")
- [X] T051 Retry the focus conference request with backoff while Jicofo answers `ready="false"` instead of joining a room that does not exist yet, in `src/platform/jitsi/signaling.go` (found on the local docker-jitsi-meet: "Room creation is restricted")
- [X] T052 Answer disco#info (XEP-0030) and advertise entity capabilities (XEP-0115) in the join presence, so that Jicofo selects the offer from the bots' real features, in `src/platform/jitsi/disco.go`
- [X] T053 Open the "JVB data channel" (protocol `http://jitsi.org/protocols/colibri`) when the offer includes SCTP, as lib-jitsi-meet does, via `SignalingHandler.OpenDataChannel` in `src/platform/adapter.go`, `src/platform/jitsi/jingle.go` and `src/execution/peer.go`
- [X] T054 Extend the live test to N bots (`JITSI_PARTICIPANTS`, `JITSI_RAMP_UP`, `JITSI_HOLD`, `JITSI_XMPP_DOMAIN`) and run it against the local docker-jitsi-meet (2/10/50/100/200 bots established and held, no errors)
- [X] T055 Parse Jicofo's JSON-encoded sources (`<json-message xmlns="http://jitsi.org/jitmeet">` in `session-initiate`, `source-add`, `source-remove`; compact format `{"sources":{"<owner>":[[video sources],[video groups],[audio sources],[audio groups]?]}}`, source `{"s":ssrc,"n":name,"m":msid,"v":"d"?}`, group `["f"|"s",ssrc,...]`) into the Jingle sources and SSRC groups of the `audio`/`video` contents, with the owner as `ssrc-info`, in `src/platform/jitsi/json_sources.go` with tests in `src/platform/jitsi/json_sources_test.go`; applied in `handleIQ` (`src/platform/jitsi/signaling.go`) before dispatch
- [X] T056 Advertise `http://jitsi.org/json-encoded-sources` in disco#info/caps (`src/platform/jitsi/disco.go`), as browsers do: Jicofo 1.0-1211 only keeps the offer's `data` (SCTP) content for clients with JSON sources (its XML path, `toContents`, rebuilds audio and video only), so this is what gives bots the bridge channel; adapter contract test with a JSON `session-initiate` and `source-add` in `src/platform/jitsi/adapter_test.go`
- [X] T057 Validate against the local docker-jitsi-meet: bridge channel opened by every bot, remote streams and received media unchanged from 004 (live test with `JITSI_MEDIA=audio_video`)
  - 2026-10-10: 10 `audio_video` bots, 30 s: all 10 opened the "JVB data channel" (JVB no longer logs `EndpointMessageTransport ... not connected`); 9 remote audio + 9 video streams offered per bot, 449 audio packets/s received per bot, 0 lost, 0 stalls, as with XML sources. Browser late joiner (JSON source-add with RTX): remote streams 6 → 9 → 6.
- [X] T058 Support token-authenticated Jitsi deployments (Prosody `mod_auth_token`): `jwt` (static token) or `jwt_app_id`/`jwt_app_secret` (one HS256 token signed per bot, `jwt_audience`, `jwt_subject`, `jwt_ttl`, `jwt_moderator`) in contracts/configuration.yaml; the token and room are sent as WebSocket URL query parameters, as lib-jitsi-meet does, in `src/platform/jitsi/jwt.go`, `adapter.go` and `signaling.go`; CLI flags `--jwt`, `--jwt-app-id`, `--jwt-app-secret`, `--jwt-moderator` in `cmd/jitsi-connect/main.go`
