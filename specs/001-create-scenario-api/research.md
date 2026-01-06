# Research & Decisions for `001-create-scenario-api`

This document records the research and decisions made to resolve the "NEEDS CLARIFICATION" items identified in the `plan.md`.

## 1. Go Version

-   **Decision**: Use Go `1.23.0`.
-   **Rationale**: The Prometheus client library (`github.com/prometheus/client_golang`) required Go version >= 1.23.0, leading to an automatic upgrade from the initially estimated Go 1.25.5.
-   **Alternatives considered**: Using an older version (rejected as there is no compelling reason to do so).

## 2. Web Framework

-   **Decision**: Use the standard `net/http` library for the web server.
-   **Rationale**: The current requirement is for a single API endpoint. The `net/http` package is well-documented, performant, and part of the standard library, which aligns with the "Incremental Complexity" principle in the constitution. We can introduce a lightweight framework like Gin or Echo later if routing or middleware needs become more complex.
-   **Alternatives considered**:
    -   **Gin**: A popular, fast, and feature-rich framework. Rejected for now to avoid adding an external dependency for a simple use case.
    -   **Echo**: Similar to Gin, also a good choice but rejected for the same reason.

## 3. Persistence Strategy

-   **Decision**: Store scenarios in-memory for the initial implementation.
-   **Rationale**: The feature specification does not include a requirement for persistence. Adhering to the "No feature is added 'just in case'" principle, in-memory storage is sufficient for the defined scope. If future requirements include listing, retrieving, or re-running saved scenarios, a persistence layer can be added.
-   **Alternatives considered**:
    -   **PostgreSQL**: A powerful relational database. Rejected as overkill for the current scope.
    -   **File-based storage (e.g., YAML files)**: Would align with the constitution's mention of YAML, but adds complexity of file I/O and management not currently needed.

## 4. API Performance Goals

-   **Decision**: The P95 (95th percentile) response time for the `/api/v1/scenarios` endpoint should be less than 200ms.
-   **Rationale**: This provides a concrete, measurable performance target that ensures a responsive user experience for the API.
-   **Alternatives considered**: Not setting a specific goal was rejected as it's better to have a clear target for non-functional requirements.

## 5. Scenario Definition Format (JSON vs. YAML)

-   **Decision**: The API will accept a `JSON` request body as defined in the feature specification.
-   **Rationale**: This is the standard and expected format for a REST API. The constitutional reference to "Scenarios are defined declaratively (YAML)" is interpreted as applying to *static, file-based scenario definitions* that might be loaded from disk, not the interactive creation via an API. The API endpoint will receive JSON and convert it to the internal Go struct representation. This internal representation could later be serialized to YAML for file-based storage or other purposes, thus satisfying both contexts.
-   **Alternatives considered**:
    -   Requiring the API to accept `YAML`: This is less common for REST APIs and would add complexity to the client's request generation and server's request parsing. It was rejected in favor of using the right tool for the job (JSON for APIs, YAML for static config).
