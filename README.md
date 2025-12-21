# WebRTC Load Engine

A modular, platform-agnostic load testing engine for WebRTC signaling and real-time media.

## Vision

**WebRTC Load Engine** is an open‑source tool for **load testing, resilience testing, and performance evaluation of WebRTC platforms**.

The long‑term vision is to provide a **generic, modular, and extensible engine** capable of testing any real‑time media architecture: SFUs, MCUs, SIP–WebRTC gateways, or hybrid solutions.

The tool is designed to be used:

* in **lab environments** (capacity testing, platform comparison),
* in **CI/CD pipelines** (performance regression testing),
* and in **controlled production environments** (planned load tests).

---

## Long‑term goals

* 🧪 Test **real‑world scalability** of WebRTC platforms
* 📡 Measure **media quality** (audio / video)
* 🔁 Validate **signaling robustness**
* 📊 Provide **actionable metrics** for SRE and capacity planning
* 🔌 Remain **platform‑agnostic** (Jitsi, Janus, mediasoup, LiveKit, etc.)
* 🤖 Be **fully automatable** (API, CLI, agents)

---

## Design principles

### 1. Strict separation of concerns

The tool is structured around three independent layers:

1. **Signaling**
2. **Media**
3. **Orchestration & scenarios**

This separation allows:

* adding new platforms without rewriting the core engine,
* replaying the same scenarios across different backends,
* evolving each layer independently.

---

## Target architecture

### 1. Signaling layer

Responsible for establishing and managing WebRTC connections.

Responsibilities:

* Session creation and teardown
* Room and participant management
* SDP / ICE negotiation

Implemented through **platform adapters**:

* Jitsi (XMPP / Colibri)
* Janus
* mediasoup
* LiveKit
* Others to come

Each adapter implements a shared interface.

---

### 2. Media layer

Responsible for sending and receiving real‑time media streams.

Responsibilities:

* Synthetic media generation (audio / video)
* Media reception and analysis
* Support for:

  * audio‑only
  * video
  * simulcast / SVC

Target metrics:

* actual bitrate
* end‑to‑end latency
* jitter
* packet loss
* perceived quality (MOS, long term)

---

### 3. Orchestration & scenario layer

Responsible for driving load test execution.

Responsibilities:

* Scenario definition:

  * number of participants
  * ramp‑up / ramp‑down
  * duration
  * topologies (1→N, N→N, single speaker)
* Load distribution across multiple agents
* Constraint injection:

  * bandwidth limitation
  * network instability (future)

Control interfaces:

* CLI
* HTTP API
* CI/CD integration
* Automated agents (human‑driven or AI‑driven)

---

## Deployment and execution

Planned targets:

* Headless execution (Chrome / native WebRTC)
* Docker containers
* On‑prem or cloud VMs
* Isolated environments for large‑scale tests

Distributed architecture:

* one **control plane**
* multiple **load agents**

---

## Observability

* Metrics export:

  * Prometheus
  * JSON / files
* Structured logs
* Correlation between signaling and media metrics

Goal: enable fine‑grained analysis of bottlenecks (SFU, network, client).

---

## Open‑source philosophy

* Open, modular, and well‑documented code
* Clear contribution interfaces
* Reproducible tests
* Vendor‑neutral and platform‑agnostic

The goal is to become a **shared building block** for WebRTC, SRE, and platform teams.

---

## Project status

🚧 **Phase 0 – Design & prototyping**

Initial priorities:

1. Jitsi adapter
2. Basic load and scalability scenarios
3. Core metrics collection

---

## AI-assisted development

This project may experiment with an **AI agent–guided coding workflow** if it proves effective.

Potential use cases include:

* bootstrapping adapters and boilerplate code,
* generating and refining test scenarios,
* accelerating refactoring and documentation,
* assisting with exploratory performance analysis.

AI agents are considered **supporting tools**, not a replacement for human review, architectural decisions, or code ownership.

Contributions and feedback are welcome from this early stage.

