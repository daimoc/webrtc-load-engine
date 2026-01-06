---

description: "Task list for implementing the Create Scenario API"
---

# Tasks: Create Scenario API

**Input**: Design documents from `/specs/001-create-scenario-api/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1)
- Include exact file paths in descriptions

## Path Conventions

- Paths assume a single project structure: `src/`, `tests/` at the repository root.

---

## Phase 1: Setup

**Purpose**: Project initialization and basic structure.

- [x] T001 Create project directories: `src/api/handlers`, `src/scenario`, `tests/unit/scenario`, `tests/integration/api`
- [x] T002 Initialize Go module with `go mod init webrtc-load-engine` in the root directory.

---

## Phase 2: Foundational

**Purpose**: This feature is self-contained, so no foundational tasks that block other features are required at this time.

---

## Phase 3: User Story 1 - Create a new load test scenario (Priority: P1) 🎯 MVP

**Goal**: Allow a user to submit a valid scenario definition via a POST request to `/api/v1/scenarios` and receive a `201 Created` response with the new scenario's ID.

**Independent Test**: A user can successfully create a scenario using a `curl` command as defined in `quickstart.md` and receive the expected successful response. Error cases for invalid platform and invalid test plan should return a `400 Bad Request`.

### Implementation for User Story 1

- [x] T003 [US1] Define `Scenario` and `TestPlan` structs in `src/scenario/scenario.go` based on `data-model.md`.
- [x] T004 [US1] Implement a simple in-memory storage solution for scenarios (e.g., a map) within the `scenario` package, in `src/scenario/service.go`.
- [x] T005 [US1] Implement the `CreateScenario` function in `src/scenario/service.go`. This function should handle the core business logic, including input validation.
- [x] T006 [US1] In the `CreateScenario` function, add validation logic to handle the happy path where the input is valid.
- [x] T007 [US1] In the `CreateScenario` function, add validation logic to return an error for an unsupported platform adapter.
- [x] T008 [US1] In the `CreateScenario` function, add validation logic to return an error for an invalid test plan (e.g., negative participants).
- [x] T009 [US1] Implement the HTTP handler for the `POST /api/v1/scenarios` endpoint in `src/api/handlers/scenario_handler.go`. This handler will parse the request, call the `CreateScenario` service, and write the HTTP response.
- [x] T010 [US1] In `src/main.go`, set up the HTTP server, register the `/api/v1/scenarios` route, and link it to the handler.

### Tests for User Story 1

- [x] T011 [US1] Write unit tests for the `CreateScenario` service in `tests/unit/scenario/service_test.go`. The tests should cover the happy path and the defined error cases (invalid platform, invalid test plan).
- [x] T012 [US1] Write an integration test in `tests/integration/api/scenario_api_test.go` that starts the server and sends a real HTTP request to the `POST /api/v1/scenarios` endpoint to verify the end-to-end flow.

**Checkpoint**: User Story 1 is fully functional and can be tested independently.

---

## Phase 4: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories.

- [x] T013 [P] Integrate a structured logging library (e.g., `slog`) into the application, adding logs in the handler and service layers.
- [x] T014 [P] Add basic Prometheus metrics for the API endpoint (e.g., request count, latency) to provide observability.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Must be completed first.
- **User Story 1 (Phase 3)**: Depends on Setup completion.
- **Polish (Phase 4)**: Can be done after User Story 1 is complete.

### Within User Story 1

- Models (`T003`) should be created before the service (`T005`).
- The service (`T005-T008`) should be implemented before the handler (`T009`).
- Unit tests (`T011`) can be written in parallel with the service implementation.
- The `main.go` setup (`T010`) and integration test (`T012`) are the final steps to make the service runnable and testable end-to-end.

### Parallel Opportunities

- `T013` and `T014` in the Polish phase can be implemented in parallel.
- Unit tests (`T011`) can be developed alongside the service logic (`T005-T008`).

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1.  Complete **Phase 1: Setup**.
2.  Complete all tasks in **Phase 3: User Story 1**.
3.  **STOP and VALIDATE**:
    -   Run all tests to ensure they pass.
    -   Manually test the endpoint using `curl` as described in `quickstart.md`.
4.  The MVP is now complete and delivers the core functionality.

---
