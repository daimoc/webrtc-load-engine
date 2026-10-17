# Feature Specification: Server Load in the Run Report

**Feature Branch**: `007-server-load-report` (work continues on `rework`)

**Created**: 2026-10-16

**Status**: Draft

**Input**: User description: "Server CPU and memory in the run report (007). During a scenario run, the engine records the CPU and memory of the media server under test, so that runs against different platforms (Jitsi today; LiveKit and mediasoup-demo in later features 008/009) can be compared on server load as well as on media quality. The servers are Docker containers on the same machine as the engine; the test plan names which containers belong to the server (e.g. the Jitsi videobridge and jicofo containers). The engine samples them at a regular interval through the local Docker API for the whole run, and the end-of-run quality summary gains a server section: per container and in total, average and peak CPU (in threads, as docker stats reports it: 100% = one thread) and peak memory, plus the bots' own CPU and memory over the same run; the samples are also exported as Prometheus series. One report per run, in the same form whatever the platform; no cross-platform comparison report. Out of scope: servers on other machines (remote hosts, Kubernetes), JVM heap/GC internals, alerting. If Docker is unreachable or a named container does not exist, the run still proceeds and the summary says server load is unavailable and why. Context: in 006 T058 the bridge CPU was read by hand with docker stats, and bot CPU with /usr/bin/time; this feature makes both part of the report."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Media server load in the report (Priority: P1)

As a load test operator, I want each run's report to say how much CPU and memory the media server used during the run, next to the media quality the bots measured, so that I can compare platforms (Jitsi, and later LiveKit and mediasoup-demo) under the same test plan on what they cost to run as well as on the quality they deliver, without reading container statistics by hand.

**Why this priority**: Server load is the number that differs most between media servers and the one the operator asked for alongside MOS, freezes and media issues. Today it is read by hand with `docker stats` during the run (006 T058), which is easy to forget, hard to line up with the run, and gives no peak. This story alone makes server load part of every report.

**Independent Test**: Run 10 bots for 1 minute against the local docker-jitsi-meet with the bridge and focus containers named in the test plan; the end-of-run summary has a server section with each container's average and peak CPU and peak memory, plus totals, and the values agree with `docker stats` read over the same minute.

**Acceptance Scenarios**:

1. **Given** a test plan naming the containers of the media server under test, **When** the run ends, **Then** the quality summary has a server section giving, for each named container and for all of them together, the average CPU and the peak CPU (in threads: 1.0 is one thread fully busy) and the peak memory over the run, with the period they cover and the number of samples.
2. **Given** a running scenario with named server containers, **When** the operator reads the metrics endpoint, **Then** it shows the current CPU and memory of each named container, labelled by scenario and container.
3. **Given** a test plan that names no server containers, **When** the run ends, **Then** the summary says server load was not requested, and the run is unchanged otherwise.
4. **Given** the same test plan run against two different platforms, **When** the operator reads the two summaries, **Then** the server sections have the same form and units, whatever the platform.

---

### User Story 2 - The bots' own load in the same report (Priority: P2)

As a load test operator, I want the same report to say how much CPU and memory the engine itself used during the run, so that I know whether the load machine had room to spare and can tell a server limit from a load-machine limit, without wrapping the engine in an external timing tool.

**Why this priority**: The bots' CPU is today measured by running the engine under `/usr/bin/time`, which covers start-up and shutdown and cannot be done through the HTTP API. In 006 T058 the bots and the bridge shared one machine, and knowing both loads was what explained the results. It is less central than server load, which is what is compared between platforms.

**Independent Test**: Run 10 bots for 1 minute; the summary has the engine's average and peak CPU and peak memory over the run, agreeing with the process's own CPU time and peak memory measured externally over the same period.

**Acceptance Scenarios**:

1. **Given** any run, with or without named server containers, **When** it ends, **Then** the summary has a load-machine section with the engine's average and peak CPU (in threads) and peak memory over the run, the number of hardware threads of the machine, and the same period as the server section.
2. **Given** a running scenario, **When** the operator reads the metrics endpoint, **Then** it shows the engine's CPU and memory, labelled by scenario.

---

### Edge Cases

- The container engine cannot be reached (not running, no permission): the run proceeds, and the summary's server section says server load is unavailable and why; the load-machine section is still filled.
- A named container does not exist or is not running when the run starts: that container is reported as unavailable with the reason; the other named containers are measured and the total covers only the measured ones, which the summary says.
- A container stops or restarts during the run: samples taken while it ran are kept, the gap is reported (number of missed samples), and measuring resumes if it comes back.
- Two scenarios run at the same time and name the same container: each scenario's report covers the container's whole load during its run, and the summary notes that the server load is not attributable to one scenario alone when runs overlap.
- A run shorter than two sampling intervals: average and peak are still given from the samples taken, with their count; a run with no sample says so instead of reporting zero.
- CPU above the number of hardware threads (container counters are per thread): reported as measured, never clamped.
- Server load exists before the run (other conferences, a bridge left in a bad state): the summary gives the first sample's values, so the operator can see a server that was not idle at the start.

## Requirements *(mandatory)*

### Functional Requirements

#### What is measured

- **FR-001**: A test plan MUST be able to name the containers that make up the media server under test, independently of the platform section, so that the same naming works for Jitsi, LiveKit and mediasoup-demo.
- **FR-002**: For each named container, the engine MUST sample CPU use and memory use at a regular interval from the start of the scenario's execution to its end. The interval MUST default to 5 seconds and be configurable in the test plan between 1 and 60 seconds.
- **FR-003**: CPU MUST be expressed in threads (1.0 = one hardware thread fully busy over the interval), the unit of the container engine's own statistics divided by 100, so that values compare across machines and with what operators read today.
- **FR-004**: Memory MUST be the container's memory use as the container engine reports it (the figure `docker stats` shows), in bytes.
- **FR-005**: The engine MUST sample its own process the same way and at the same times: CPU in threads and resident memory, plus the number of hardware threads of the load machine.

#### Reporting

- **FR-006**: The end-of-run quality summary (006) MUST gain a server section and a load-machine section. Per named container and in total over the named containers: average CPU, peak CPU, peak memory, first sample's CPU and memory, number of samples and of missed samples. For the engine: average CPU, peak CPU, peak memory, number of samples, and the machine's hardware threads. Both sections give the period they cover.
- **FR-007**: The summary MUST be in the same form and units whatever the platform; there is no cross-platform comparison report.
- **FR-008**: The samples MUST be exported as Prometheus series: CPU and memory per named container, labelled by scenario and container name, and the engine's CPU and memory, labelled by scenario.
- **FR-009**: The summary's server and load-machine sections MUST stay available with the rest of the summary, as long as the scenario is kept (006 FR-016).

#### Failures

- **FR-010**: Failure to reach the container engine or to find a named container MUST NOT fail or delay the scenario; the summary MUST state, per container, whether it was measured and, if not, why (unreachable, not found, not running, permission denied).
- **FR-011**: Unavailable measurement MUST be logged once per container per run with the scenario identifier and the reason, not once per sample.
- **FR-012**: When the server section covers a period in which another scenario of the same engine was running against the same containers, the summary MUST say so.

#### Cost

- **FR-013**: Measuring MUST add no per-bot work: its cost depends on the number of named containers and the interval only, and MUST stay within the CPU budget of the 100-bot run (2 cores).

### Key Entities

- **Server Containers**: per test plan: the names of the containers that make up the media server under test, and the sampling interval.
- **Load Sample**: per container (or the engine) and sampling time: CPU in threads and memory in bytes; folded into the scenario's metrics and summary, not kept one by one.
- **Server Load Summary**: per scenario, built at the end of the run: per container and total, average and peak CPU, peak memory, first sample, sample counts, availability and reason; the engine's own figures and the machine's hardware threads.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: With 10 bots for 1 minute on the local docker-jitsi-meet, the reported average CPU of the bridge container is within 10% of the average of `docker stats` readings taken over the same minute, and its peak memory within 5% of the highest reading.
- **SC-002**: The reported average CPU of the engine over a run is within 5% of its CPU time (user + system) measured externally over the same period, divided by that period.
- **SC-003**: With the container engine stopped, or a misspelt container name, a run completes in the same time as without server measurement, and the summary says which container was not measured and why.
- **SC-004**: The 100-bot run (audio and video, receive cap 10, 10 minutes) with three named containers at the default interval uses no more than 0.05 core more than the same run without them.
- **SC-005**: An operator can read, from the summary alone and within 1 minute of the run's end, the media server's average and peak CPU and peak memory next to the run's media quality, in the same form for any platform.

## Assumptions

- The media server runs as containers on the same machine as the engine, and the engine can reach the local container engine (Docker); servers on other machines, Kubernetes and cloud monitoring are out of scope.
- Containers are named as the container engine lists them (for docker-jitsi-meet, for example `docker-jitsi-meet-jvb-1`, `docker-jitsi-meet-jicofo-1`); choosing which containers belong to the server is the operator's job, through the test plan.
- Only container-level CPU and memory are measured; server internals (JVM heap and garbage collection, Node.js event loop, per-conference figures) are out of scope.
- Averages, peaks and first-sample values are enough for comparing runs; the full time series is available through the Prometheus series for operators who scrape them, and is not kept in the summary.
- Thresholds that pass or fail a run on server load, and alerting, are out of scope; the summary gives the values and the operator decides (as 006).
- When the engine and the server share a machine, their loads affect each other (006 T058); this feature reports both, it does not separate them.
