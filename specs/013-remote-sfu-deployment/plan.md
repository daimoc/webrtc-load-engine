# Implementation Plan: Remote SFU Deployment

**Branch**: `013-remote-sfu-deployment` (work continues on `rework`) | **Date**: 2026-11-06 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/013-remote-sfu-deployment/spec.md`

## Summary

A new command, `cmd/sfu-remote`, drives a remote Linux machine over the
operator's SSH access: `deploy`, `status`, `allow` and `remove`. It sends the
existing `tests/platforms/<server>` folder, adds a small overlay per server, and
runs Docker Compose there (R1, R2). The setups stay the single source of each
server's configuration.

**Server load** needs almost no new code. The engine already reads the Docker
Engine API through `DOCKER_HOST`; a `ssh://` value now dials
`ssh <host> docker system dial-stdio`, as the Docker CLI does (R5). No port
opens, the test plan does not change, and the report keeps its form.

**Exposure** is decided from the machine's own addresses, not the announced one
(R3): host-network servers listen on every interface, so a VPN address on a
machine that also has a public one protects nothing. Such a machine needs
`--allow`, which installs firewall rules (R4), or `--external-firewall`, where
the operator states that one exists.

**Local settings**: `deploy` writes a private env file on the workstation with
everything a run needs, secrets included (R6), so a remote run is:

```bash
set -a; . ~/.config/webrtc-load-engine/sfu-remote/<host>.env; set +a
go test -tags live -count=1 -v -run Janus ./tests/live/
```

**The main limit depends on where the machine is** (R9). 100 bots at cap 10
receive about 560 Mbps, and 50 bots about 280 Mbps, so over a home connection
bandwidth limits large runs before the server does.

**The validation machine is on the LAN** (R11): `fetis@192.168.1.95`, 4 low-power
cores, at least 824 Mbps each way. There the bots' bandwidth is not the limit;
the server's CPU likely is. It has global IPv6 addresses, so it needs a decision
before `deploy` accepts it (R11); the access prerequisites were checked on it.

## Technical Context

**Language/Version**: Go 1.23.0 (keep 1.23-compatible code)

**Primary Dependencies**: standard library (`os/exec`, `archive/tar`, `net/netip`, `crypto/rand`). No new module. On the workstation: OpenSSH client. On the machine: Docker Engine with the Compose plugin; `sudo iptables` only with `--allow`.

**Storage**: on the machine, `~/sfu-remote/<server>/` and a one-line state file; on the workstation, one 0600 env file per host

**Testing**: `go test -race ./...`: `src/remote` against a fake command runner with recorded commands; `src/hostload` against a fake `ssh` that serves the Docker API on stdio. Live: `-tags live` tests against the validation machine (R11)

**Target Platform**: workstation Linux; target Linux amd64

**Project Type**: Go CLI plus an engine change

**Performance Goals**: deploy under 10 minutes with images built (SC-001); load reading at most 1% of one thread on the machine (FR-015)

**Constraints**: nothing opens the Docker API (FR-008a); no secret in the repository, status output, logs or reports (FR-007); local setups unchanged (FR-016); only one server per machine (FR-004)

**Scale/Scope**: four servers, one machine at a time; about 600 lines in `src/remote`, 40 in `src/hostload`

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Result |
|---|---|
| I. Infrastructure-first | Pass: separates bots from the server under test |
| II. Media engine independence | Pass: `src/media` and `src/execution` are not changed |
| III. Pluggable signaling | Pass: adapters are not changed; `src/remote` knows servers as Compose setups, not signaling |
| IV. CPU efficiency | Pass: no change on the bots; one `ssh` per monitored container |
| V. Lifecycle | Not concerned |
| Language | Pass: Go, in the same module. `ssh` is an external program, as `ffmpeg` already is for media files |
| Test discipline | Pass: tests first, fakes for SSH; real-machine tests behind `-tags live` |
| Incremental complexity | Pass: no agent, no provisioning tool, no new API field |

Re-checked after Phase 1 and after surveying the validation machine: no change.

## Project Structure

### Documentation (this feature)

```text
specs/013-remote-sfu-deployment/
├── plan.md
├── research.md          # R1–R11
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── sfu-remote-cli.md
│   └── settings-file.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
cmd/sfu-remote/          # New: flags and output
src/remote/              # New
├── runner.go            # Runs commands over ssh; fake in tests
├── servers.go           # The four servers: folder, containers, ports, env, needs
├── envfile.go           # .env from env.example, with values applied
├── machine.go           # Prerequisites and address classification
├── firewall.go          # Rules for --allow
├── deploy.go            # deploy, status, remove, allow
└── settings.go          # The workstation's env file
src/hostload/docker.go   # ssh:// DOCKER_HOST
tests/platforms/*/docker-compose.remote.yml  # New: per-server overlay
tests/live/remote_live_test.go               # Deploy, status, remove on a real machine
```

**Structure Decision**: the existing single project. `src/remote` depends on no
other package here.

## Complexity Tracking

No violations.
