# WebRTC Load Engine Constitution

## Core Principles

### I. Infrastructure-First, Not Browser Emulation

The project focuses on **infrastructure-level WebRTC load testing**, not browser behavior emulation.

* Synthetic peers are first-class citizens.
* The goal is to stress **SFUs, MCUs, signaling planes, and networks**, not frontend UIs.
* Browser-based clients MAY be added later, but are explicitly out of scope for the initial architecture.

---

### II. Media Engine Independence

The media engine MUST remain fully **independent from signaling and platform logic**.

* The engine consumes only:

  * SDP offers/answers
  * ICE candidates
  * lifecycle events
* No platform-specific signaling concepts (XMPP, JSON schemas, REST endpoints) are allowed inside the media layer.
* Media behavior MUST be reproducible and deterministic.

---

### III. Pluggable Signaling (NON-NEGOTIABLE)

All signaling MUST be implemented through **pluggable adapters**.

* Each platform (Jitsi, Janus, mediasoup, LiveKit, etc.) implements a shared signaling contract.
* Adapters encapsulate all protocol-specific complexity.
* Scenarios and media code MUST NOT depend on signaling internals.

Breaking this principle is considered an architectural violation.

---

### IV. CPU Efficiency as a First-Class Constraint

CPU efficiency is a **core design requirement**, not an optimization.

* The default execution model MUST allow hundreds of peers on a single machine.
* Real-time encoding and rendering SHOULD be avoided.
* Pre-encoded or synthetic media is preferred.
* Any feature significantly increasing CPU usage MUST be explicitly justified.

---

### V. Deterministic Peer Lifecycle

All peers MUST follow a **shared, explicit lifecycle state machine**.

* Platform-specific events MUST be mapped to the common lifecycle.
* Illegal or unexpected transitions MUST be surfaced as errors.
* Lifecycle state is observable and exportable for diagnostics.

---

## Technical Constraints

### Language and Runtime

* **Go** is the reference implementation language.
* The WebRTC stack is based on **Pion**.
* The project SHOULD build as a **single static binary**.

---

### Signaling Support

The architecture MUST support heterogeneous signaling mechanisms:

* XMPP (Jitsi)
* WebSocket / REST (Janus, mediasoup)
* gRPC / WebSocket (LiveKit)

Signaling diversity MUST NOT leak outside adapter boundaries.

---

### Scenario Definition

* Scenarios are defined declaratively (YAML).
* Scenarios describe *what* to test, not *how* signaling works.
* Scenario formats MUST be stable and versioned.

---

### Observability

* Structured logging is REQUIRED.
* Metrics MUST be exportable (Prometheus-compatible).
* Signaling, ICE, and media metrics MUST be correlatable.

---

## Development Workflow

### Test Discipline

* Core components (media engine, signaling interface, lifecycle) MUST be test-covered.
* Signaling adapters MUST include contract tests.
* Performance-sensitive paths SHOULD include benchmarks.

---

### Incremental Complexity

* Start simple.
* No feature is added "just in case".
* Every abstraction must justify its cost.

---

### AI-Assisted Development

The project MAY use **AI agent–assisted coding workflows**.

* AI-generated code is subject to the same review standards as human-written code.
* Architectural decisions remain human-owned.
* The constitution always takes precedence over AI output.

---

## Governance

* This constitution supersedes all informal practices.
* All pull requests MUST be reviewed for constitutional compliance.
* Violations require explicit discussion and documented justification.
* Amendments require:

  * a written proposal,
  * rationale and trade-offs,
  * migration or refactoring plan.

---

**Version**: 0.1.0
**Ratified**: 2025-01-24
**Last Amended**: 2025-01-24
