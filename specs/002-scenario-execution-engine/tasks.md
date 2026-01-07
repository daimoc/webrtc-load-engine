# Tasks: Scenario Execution Engine

**Input**: Design documents from `/specs/002-scenario-execution-engine/`

## Phase 1: Setup

**Purpose**: Create the directory structure for the new packages.

- [x] T001 Create project directories: `src/execution`, `src/platform`, `src/platform/jitsi`, `tests/unit/execution`, `tests/integration/execution`

---

## Phase 2: Foundational

**Purpose**: Define the core interfaces and placeholder implementations that the execution engine will depend on.

- [x] T002 Define the `PlatformAdapter` interface in a new file `src/platform/adapter.go`. It should include a `Connect(*peer.Peer)` method.
- [x] T003 Create a placeholder implementation of the `PlatformAdapter` for Jitsi in `src/platform/jitsi/adapter.go`. This will be a struct that implements the interface but with no-op methods for now.

---

## Phase 3: User Story 1 - Execute a Load Test Scenario (Priority: P1) 🎯 MVP

**Goal**: Allow a user to trigger the execution of a pre-defined load test scenario via an API call.

**Independent Test**: A user can successfully trigger a scenario execution via a `POST` request to `/api/v1/scenarios/{id}/_execute`. The scenario status should change to `running` and then `completed` (or `failed`). The test will verify these status changes.

### Implementation for User Story 1

- [x] T004 [US1] Define the `Peer` struct in `src/execution/peer.go`. This struct will encapsulate a `pion.PeerConnection` and manage its state.
- [x] T005 [US1] Define the `Engine` struct in `src/execution/engine.go`. This will manage the lifecycle of the scenario execution.
- [x] T006 [US1] Implement the `Engine.Start()` method in `src/execution/engine.go`. This method will contain the main execution loop, including the ticker-based ramp-up logic.
- [x] T007 [US1] Implement the `Engine.Stop()` method in `src/execution/engine.go` for graceful shutdown of all peers.
- [x] T008 [US1] Add a `ExecuteScenario(id string)` method to the `scenario.Service` in `src/scenario/service.go`. This method will retrieve the scenario, create an `Engine`, and start it in a new goroutine. It should also update the scenario status to `running`.
- [x] T009 [US1] Add a `ExecuteScenario` handler method to the `api.ScenarioHandler` in `src/api/handlers/scenario_handler.go` for the `POST /api/v1/scenarios/{id}/_execute` endpoint.
- [x] T010 [US1] Register the new `/api/v1/scenarios/{id}/_execute` route in `src/main.go`.
- [x] T011 [US1] Implement signal handling in `src/main.go` to listen for `SIGINT` and `SIGTERM` and call a `Shutdown()` method on the `scenario.Service` (which will in turn call `Engine.Stop()` for any active engine).

### Tests for User Story 1

- [x] T012 [US1] Write unit tests for the `Engine` in `tests/unit/execution/engine_test.go`. This should test the ramp-up and stop logic.
- [x] T013 [US1] Write an integration test in `tests/integration/execution/scenario_execution_test.go` that creates a scenario, executes it, and polls for the status to change to `completed`.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Must be completed first.
- **Foundational (Phase 2)**: Depends on Setup. Blocks User Story 1.
- **User Story 1 (Phase 3)**: Depends on Foundational.

### Within User Story 1

- The `Peer` and `Engine` structs (`T004`, `T005`) should be defined before implementing the `Engine`'s methods.
- The `Engine` methods (`T006`, `T007`) should be implemented before integrating with the `scenario.Service`.
- The `scenario.Service` (`T008`) should be updated before the `api.ScenarioHandler` (`T009`).
- The handler (`T009`) should be updated before registering the route in `main.go` (`T010`).
- Tests can be developed in parallel with their corresponding implementation.

## Implementation Strategy

### MVP First (User Story 1 Only)

1.  Complete **Phase 1: Setup**.
2.  Complete **Phase 2: Foundational**.
3.  Complete all tasks in **Phase 3: User Story 1**.
4.  **STOP and VALIDATE**:
    -   Run all new unit and integration tests.
    -   Manually test the full flow: create a scenario, then execute it, and check the status.
5.  The MVP for scenario execution is now complete.
