# Research & Decisions for `002-scenario-execution-engine`

This document records the research and decisions made for implementing the scenario execution engine.

## 1. Core Engine Structure

- **Decision**: The execution logic will be encapsulated in an `Engine` struct in `src/execution/engine.go`. This engine will be responsible for managing the entire lifecycle of a scenario execution.
- **Rationale**: This centralizes the control flow of the test, from ramp-up to completion, making it easier to manage and reason about.
- **Alternatives considered**: A more distributed actor-based model was considered but rejected for now to adhere to the "start simple" principle.

## 2. Peer Management

- **Decision**: A `Peer` struct will be created in `src/execution/peer.go`. It will wrap the `pion.PeerConnection` and manage its state machine (e.g., `connecting`, `connected`, `failed`, `disconnected`). The `Engine` will manage a slice or map of these `Peer` objects.
- **Rationale**: Encapsulating the peer logic simplifies the `Engine`'s responsibility to only orchestrating peers, not managing the low-level details of each WebRTC connection.
- **Alternatives considered**: Managing `pion.PeerConnection` objects directly within the `Engine` was rejected as it would lead to a monolithic and less maintainable structure.

## 3. Ramp-up Implementation

- **Decision**: The `Engine` will implement a ticker-based ramp-up. A new `Peer` will be created and initiated at a regular interval (`ramp_up_seconds / participants`) to distribute the connection load evenly over the ramp-up period.
- **Rationale**: A ticker provides a simple and effective way to control the rate of new connections, preventing a thundering herd problem.
- **Alternatives considered**:
    - **Randomized ramp-up**: Rejected for now to favor a more predictable and deterministic load pattern.
    - **Connecting all peers at once**: Rejected as this is unrealistic and would not constitute a "ramp-up".

## 4. Signaling and Platform Adapters

- **Decision**: A `PlatformAdapter` interface will be defined. The initial implementation will be a placeholder for the Jitsi adapter in `src/platform/jitsi/adapter.go`. This adapter will expose methods like `Connect(peer *Peer)` which will handle the XMPP signaling for that peer.
- **Rationale**: This adheres to the "Pluggable Signaling" principle of the constitution. The `Engine` will be completely decoupled from the specifics of Jitsi's XMPP signaling.
- **Alternatives considered**: Implementing signaling logic directly in the `Engine` was rejected as a clear violation of the constitution.

## 5. Graceful Shutdown

- **Decision**: The `Engine` will have a `Stop()` method. This method will be responsible for iterating through all active `Peer` objects and calling a `Disconnect()` method on each. In `main.go`, we will listen for `SIGINT` and `SIGTERM` signals and call the `Engine.Stop()` method to ensure a graceful shutdown. This will also be used at the end of a successful test run.
- **Rationale**: This provides a clean and predictable way to terminate a test, ensuring that resources are released correctly.
- **Alternatives considered**: Letting the OS terminate the process without cleanup was rejected as it could leave scenarios in an inconsistent state.
