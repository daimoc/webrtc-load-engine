# Feature Specification: Remote SFU Deployment

**Feature Branch**: `013-remote-sfu-deployment` (work continues on `rework`)

**Created**: 2026-11-06

**Status**: Draft

## Clarifications

### Session 2026-11-06

- Q: What is the remote server? → A: An existing Linux machine the operator can already reach over SSH; this feature does not create machines.
- Q: How is it reached? → A: From the operator's own addresses only: a private network or VPN, or a firewall allowing only stated IP addresses. Self-signed certificates are acceptable.
- Q: Should remote reports give server CPU and memory? → A: Yes, read from the remote machine over the same secure access used to deploy (this lifts 007's limit to local servers).

**Input**: User description: "Deploy Run the sfu infrastucture on remote server with docker"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Run a media server on a remote machine (Priority: P1)

As a load test operator, I want to start any of the four media servers (Jitsi, LiveKit, mediasoup-demo, Janus) on a remote machine with one documented command from my workstation, in the same pinned versions as the local setups, and to stop it the same way, so that the bots and the server stop competing for one laptop's CPU.

**Why this priority**: Every result so far comes from one laptop running both sides. Its CPU limits the load, and figures from different days cannot be compared. Separating the two is the point of this feature, and nothing else in it works without a running remote server.

**Independent Test**: From the workstation, deploy one server to the remote machine, confirm with the documented status command that it runs, join a room from a browser, then remove it and confirm nothing of it is left running.

**Acceptance Scenarios**:

1. **Given** a prepared remote machine and the operator's access to it, **When** the operator deploys a named server, **Then** that server runs on the machine, at the same pinned version as its local setup.
2. **Given** a deployed server, **When** the operator asks for its status, **Then** the answer says whether it runs and gives the address and connection settings the bots need, without showing secrets.
3. **Given** a deployed server, **When** the operator deploys another one, **Then** the first is stopped first, so that only one server runs on the machine at a time.
4. **Given** a deployed server, **When** the operator removes it, **Then** none of its containers remain running, and the machine can take another server.
5. **Given** a deployment that fails part way, **When** the command ends, **Then** it reports what failed, and running it again after the cause is fixed succeeds.

---

### User Story 2 - Run the same test plan against the remote server (Priority: P2)

As a load test operator, I want to run an existing test plan from my workstation against the remote server, changing only its connection settings, so that bots really exchange media with it across the network and I get the usual report.

**Why this priority**: A deployed server is only useful once bots reach it. Media across a real network is what differs from the local setups: servers must announce the right address, and the media ports must be open.

**Independent Test**: With a server deployed, run the 10-bot `audio_video` plan for 1 minute against it from the workstation: all bots connect, receive audio and video, and the report files are written, on each of the four platforms.

**Acceptance Scenarios**:

1. **Given** a deployed server, **When** a test plan runs against the connection settings the status command gave, **Then** every bot joins, publishes and receives media, as on the local setups.
2. **Given** a remote run, **When** its report is written, **Then** it has the same sections and fields as a local run's, and says which server it ran against.
3. **Given** bots and server on different machines, **When** the report gives latency, **Then** it is the true send-to-receive time, because the bots still both send and receive.
4. **Given** the server's media ports are blocked, **When** a run starts, **Then** the bots are reported as failed to establish media, with a reason that points at connectivity, and the run does not hang.

---

### User Story 3 - Server load in remote reports (Priority: P3)

As a load test operator, I want the report of a remote run to give the server's CPU and memory, as local runs do, so that the servers can be compared on the machine they really run on.

**Why this priority**: Server load is one of the four measures the project compares servers on, and until now the laptop made it unreliable. It comes after the first two stories because a remote run is already useful for media quality without it.

**Independent Test**: Run a test plan against the remote server with server monitoring on: the report's server section gives CPU and memory for the named containers, and they match the remote machine's own view within 10%.

**Acceptance Scenarios**:

1. **Given** a remote run with server monitoring, **When** it ends, **Then** the report gives the CPU and memory of the named containers, in the same form as a local run.
2. **Given** the load cannot be read during a run, **When** the report is written, **Then** that section is left out with its reason, and the run is not failed.

---

### Edge Cases

- **The machine lacks what the setup needs** (no container engine, too little memory or disk): the deployment says so before changing anything.
- **Its media or signaling ports are closed by a firewall**: the status check says which ones it could not reach.
- **The machine has several addresses, or sits behind NAT**: the operator can state the address servers announce.
- **Another server of this project already runs there**: it is stopped first (US1 scenario 3). Containers that this project did not start are never touched.
- **The operator's connection drops during a deployment**: running it again finishes the work.
- **Secrets** (server API secrets, passwords): generated for each deployment, never in the repository, logs, status output or report files.
- **A self-signed certificate in a browser**: secure pages are needed for media, so the documentation says how to accept the certificate for a browser check.
- **A machine with a public address and no firewall in place**: deployment stops before starting the server (FR-008).
- **The operator's address changes** (a home connection): the allowed addresses can be updated without redeploying; a run from the old rule fails with a connectivity reason.
- **A forgotten deployment**: the status command makes running servers easy to see.
- **Mismatch between versions**: a version that differs from the pinned one is refused rather than run silently.
- **A remote server and bots in different regions**: network delay is real and appears in latency; it is not corrected.

## Requirements *(mandatory)*

### Functional Requirements

#### Deployment (User Story 1)

- **FR-001**: The operator MUST be able to deploy, get the status of, and remove each of the four servers from the workstation with documented commands, naming the machine and the server.
- **FR-002**: The target MUST be an existing Linux machine that the operator can already reach over SSH. Creating machines or accounts at a provider is out of scope.
- **FR-003**: Deployed servers MUST use the same pinned versions and the same media configuration as the local setups in `tests/platforms/`, apart from addresses, ports and secrets, so that remote and local results differ only by the network and the machine.
- **FR-004**: Only one server of this project MUST run on a machine at a time, so that each is measured alone.
- **FR-005**: Deployment MUST check the machine's prerequisites before changing anything, and MUST be safe to run again.
- **FR-006**: Servers MUST announce an address the bots can reach, found automatically or stated by the operator.
- **FR-007**: Secrets MUST be generated per deployment and stay out of the repository, logs, status output and report files, apart from the operator's own local settings.
- **FR-008**: Deployed servers MUST be reachable only from the operator's own addresses: over a private network or VPN, or through a firewall that allows only addresses the operator states. Deployment MUST NOT leave a server open to the whole internet, and MUST stop and say so if it cannot confirm this.
- **FR-008a**: The container engine's API MUST NOT be reachable from any network: the workstation uses the existing secure access only.
- **FR-008b**: Self-signed certificates MUST be acceptable, for the servers and transports that need one. The status command MUST say when the bots need the setting that skips certificate checks.
- **FR-008c**: Removal MUST also undo any network access the deployment opened.
- **FR-009**: Removal MUST stop everything the deployment started, and MUST NOT touch anything else on the machine.

#### Running against it (User Story 2)

- **FR-010**: Existing test plans MUST run against a remote server with only their connection settings changed; no new test plan option is needed for media.
- **FR-011**: The status command MUST give the connection settings for that server in a form the operator can use directly.
- **FR-012**: The live tests and run commands MUST work against a remote address, including the ones that assume a loopback address today.
- **FR-013**: Failing to reach a server's media MUST be reported as a failed bot with its reason, within the existing connection timeouts.

#### Server load (User Story 3)

- **FR-014**: With server monitoring on, a remote run's report MUST give the named containers' CPU and memory as a local run does; when this cannot be read, the section MUST be left out with its reason.
- **FR-015**: Server load MUST be read over the same secure access as deployment, with no new port open to the internet, and MUST NOT disturb the server: at most 1% of one CPU thread on the remote machine.

#### Compatibility

- **FR-016**: Local setups, local test plans and their reports MUST work exactly as before.

### Key Entities

- **Target machine**: where a server runs; its address, how the operator reaches it, and its announced address.
- **Deployment**: one server on one machine; its server name, pinned version, state, connection settings and secrets.
- **Connection settings**: what a test plan needs to reach a deployment.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator with a prepared machine deploys any of the four servers in under 10 minutes, images already built, using only the documentation.
- **SC-002**: A 10-bot, 1-minute `audio_video` run from the workstation passes on each of the four servers: all bots connected, video and audio received, MOS at least 4.0 on a network with no loss.
- **SC-003**: After removal, none of the feature's containers run on the machine, in 100% of removals.
- **SC-004**: At 100 bots, cap 10, with the server on its own machine and bots on a machine with enough upload bandwidth, the bots' machine no longer limits the run: send lateness and freezes do not rise because of the bots' own CPU.
- **SC-005**: Two runs of one plan on different days give server CPU within 15% of each other, against up to 2.9x on the laptop.
- **SC-006**: Reported server CPU and memory match the machine's own tools within 10%.
- **SC-007**: No secret appears in the repository, status output, logs or report files, checked by searching for the values.
- **SC-008**: A port scan from an address that is not allowed finds none of the server's ports, while the allowed address reaches all of them; after removal, neither finds any.
- **SC-009**: The container engine's API answers from no network address, checked from the workstation.

## Assumptions

- The bots stay on one machine in one process: the operator's workstation, or any machine with enough upload bandwidth for the run; a home connection may limit large runs. Distributing bots across several machines stays out of scope (README "Not done yet").
- Linux, on the amd64 architecture, as for the local setups, on a machine with a container engine or one that can have it installed.
- One server at a time on a machine: the project compares servers measured alone. Several machines can each hold one.
- Images are built from the same pinned sources as the local setups; a machine needs no copy of the source code beyond that.
- This is load test infrastructure, not production hosting: no high availability, autoscaling, backups or updates.
- The operator has the right to run load against the machine and its network. A provider's traffic costs and limits are the operator's concern, and the documentation says how much traffic a run uses.
- The operator has the rights over the machine's firewall or private network. No domain name is needed.
- Depends on the local setups of 008, 010, 011 and docker-jitsi-meet, on 007 for server load, and on 012 for latency across machines.
