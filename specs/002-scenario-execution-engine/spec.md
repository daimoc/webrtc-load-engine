# Feature Specification: Scenario Execution Engine

**Feature Branch**: `002-scenario-execution-engine`  
**Created**: 2026-01-06
**Status**: Draft  
**Input**: User description: "Implement the scenario execution engine"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Execute a Load Test Scenario (Priority: P1)

As a user, I want to trigger the execution of a pre-defined load test scenario via an API call so that I can generate load against my WebRTC infrastructure.

**Why this priority**: This is the core functionality of the application. Without it, the service cannot perform its primary function of load testing.

**Independent Test**: Can be fully tested by sending a POST request to a new API endpoint. A successful test will result in the scenario's status changing to `running` and eventually `completed`, and will generate actual WebRTC traffic.

**Acceptance Scenarios**:

1. **Given** a scenario with ID `scenario-123` exists and has a status of `created`,
   **When** I send a `POST` request to `/api/v1/scenarios/scenario-123/_execute`,
   **Then** the system should return a `202 Accepted` status, and the scenario's status should be updated to `running`.
2. **Given** a scenario with ID `scenario-123` is already `running`,
   **When** I send a `POST` request to `/api/v1/scenarios/scenario-123/_execute`,
   **Then** the system should return a `409 Conflict` error with a message indicating the scenario is already running.
3. **Given** no scenario exists with ID `scenario-999`,
   **When** I send a `POST` request to `/api/v1/scenarios/scenario-999/_execute`,
   **Then** the system should return a `404 Not Found` error.

---

### Edge Cases

- **Scenario Execution Failure**: What happens if the platform adapter fails to connect or if media setup fails for a significant number of peers? The scenario status should be updated to `failed`. Only the `failed` status will be updated; a specific reason for the failure will not be stored or accessible via the API in this iteration.
- **Invalid Test Plan during Execution**: What if the TestPlan, which was valid on creation, becomes invalid due to external changes (e.g., platform adapter updated and no longer supports a feature)? The execution should fail gracefully, and the status updated to `failed`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST expose an API endpoint to trigger the execution of a scenario.
- **FR-002**: The system MUST validate that the scenario ID in the execution request corresponds to an existing, non-running scenario.
- **FR-003**: The system MUST update the scenario's status to `running` upon successful initiation of the execution.
- **FR-004**: The execution engine MUST use the `platform` adapter specified in the scenario definition.
- **FR-005**: The engine MUST simulate the number of `participants` over the `ramp_up_seconds` period as defined in the `TestPlan`.
- **FR-006**: The test MUST run for the `duration_seconds` after all participants have joined.
- **FR-007**: Upon completion of the test duration, the engine MUST gracefully disconnect all simulated participants.
- **FR-008**: The system MUST update the scenario's status to `completed` on successful completion, or `failed` if it terminates unexpectedly.
- **FR-009**: The execution status and progress can be monitored by polling the `GET /api/v1/scenarios/{id}` endpoint to retrieve the scenario's current status. No active real-time monitoring is planned for this iteration.
- **FR-010**: The engine should handle graceful shutdown by trapping `SIGINT`/`SIGTERM` signals to attempt to update the scenario status to `failed` for any active executions.

### Key Entities *(include if feature involves data)*

- **Scenario**: The existing entity will be used. Its `status` field will be updated to reflect the execution lifecycle (`running`, `completed`, `failed`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Scenario execution can be successfully initiated via a single `POST` API call.
- **SC-002**: The system can execute a scenario with at least 100 participants, as defined in the `TestPlan`, on a standard cloud VM instance.
- **SC-003**: The scenario execution status can be retrieved via an API call, providing visibility into the current state.
- **SC-004**: The p99 latency for the execute API call must be under 500ms.

## Out of Scope

- Real-time reporting of granular execution metrics (e.g., peer connection times, media quality).
- A UI for triggering or monitoring execution.
- Storing and retrieving historical execution results or detailed logs.

## Assumptions

- A scenario has already been created via the `POST /api/v1/scenarios` endpoint and is available in the system.
- The platform adapter for the specified `platform` (e.g., "jitsi") is implemented and available to the execution engine.
- The execution process is asynchronous. The API call to trigger execution will return a response immediately without waiting for the entire test to complete.