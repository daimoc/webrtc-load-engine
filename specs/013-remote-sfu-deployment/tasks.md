---

description: "Task list for 013 remote SFU deployment"
---

# Tasks: Remote SFU Deployment

**Input**: Design documents from `specs/013-remote-sfu-deployment/`

**Prerequisites**: [plan.md](plan.md), [spec.md](spec.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/sfu-remote-cli.md](contracts/sfu-remote-cli.md), [contracts/settings-file.md](contracts/settings-file.md), [quickstart.md](quickstart.md)

**Tests**: Included. The plan and the constitution require them: write each test first and see it fail. Real-machine tests stay behind `-tags live`.

**Organization**: `src/remote`'s shared parts are foundational (Phase 2). Each story then adds what the operator can do.

**Conventions**:
- **Toolchain**: Go at `/usr/local/go/bin` (`export PATH=/usr/local/go/bin:$PATH`). Code stays go1.23-compatible. `go vet ./... && go test -race ./...` stays green at the end of each phase, and needs no machine.
- **Boundaries** (FR-010, FR-016):
  - no new module in `go.mod`;
  - production changes stay in `cmd/sfu-remote`, `src/remote`, `src/hostload/docker.go`, `src/hostload/sampler.go` and new `docker-compose.remote.yml` files;
  - `src/media`, `src/execution`, `src/platform`, `src/scenario`, `src/report`, test plans and existing compose files are not changed;
  - `src/remote` imports nothing from this module.
- **Names** (used across tasks), in `src/remote`:
  - `type Runner interface { Run(ctx context.Context, cmd string, stdin io.Reader) (stdout []byte, err error) }`; `SSHRunner` runs `ssh -o BatchMode=yes -o ConnectTimeout=5 [-p port] -- [user@]host <cmd>`; `FakeRunner` lives in `src/remote/fake_test.go`, records commands and returns scripted answers;
  - `ParseTarget(s string) (Target, error)`; `Target{User, Host, Port string}`; `(Target).SSHURL() string`;
  - `Server{Name, Folder string; Services, Containers []string; TCPPorts []int; UDPPorts []PortRange; MemoryGB int; Images map[string]string}`; `LookupServer(name string) (Server, bool)`;
  - `RenderEnv(example []byte, values map[string]string) []byte`;
  - `Classify(ipJSON []byte) (Addresses, error)`; `Addresses{Private, Public []netip.Addr}`;
  - `Check(ctx, Runner, Server, Options) (warnings []string, err error)`;
  - `FirewallCommands(Server, []netip.Prefix) []string`; `RemoveFirewallCommands(Server) []string`;
  - `Deploy`, `Status`, `Allow`, `Remove`, each taking `(ctx, Runner, …)`.
- **Secrets** (FR-007, SC-007): secrets go through `Runner.Run`'s `stdin`, never `cmd`. Error messages and logs never contain a `.env` value. Never write the machine's password anywhere: key login only.
- **Live runs**: target `T=fetis@192.168.1.95` ([research R11](research.md)). Compared runs are alternated and taken in the same machine state. Only one server at a time. Stop only what you started. Never `pkill -f`. Containers on this workstation are not touched by `sfu-remote`.
- **A surprise on a real server** (as with loopback ICE on Janus): fix it in that server's overlay or rendered values, add a unit test for the fix, and record it in `validation-report.md`. If it would change a compose file or the spec, stop and tell the user.
- **Commits**: only when the user asks.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story the task belongs to (US1, US2, US3)

---

## Phase 1: Setup

**Purpose**: Check the machine again and open the report.

- [X] T001 Create `specs/013-remote-sfu-deployment/validation-report.md` with one section per success criterion (SC-001 to SC-009), and record `git rev-parse HEAD`, whether the tree holds uncommitted 012 work, and the result of `go vet ./... && go test -race ./...`
- [X] T002 Check the target and record the result in `specs/013-remote-sfu-deployment/validation-report.md`: `ssh -o BatchMode=yes $T` works, `docker ps` works without sudo, and `sysctl -n net.core.rmem_max net.core.wmem_max` both give at least 4194304. If a limit is low, tell the user to run the [quickstart](quickstart.md) command; it only blocks mediasoup's load runs

---

## Phase 2: Foundational (driving a machine)

**Purpose**: Commands reach a machine, servers are known, `.env` files render. All stories depend on this phase.

- [X] T003 [P] Write failing tests in `src/remote/runner_test.go`: `ParseTarget` accepts `host`, `user@host`, `user@host:2222`, an `~/.ssh/config` alias and `[::1]:22`; it refuses an empty value and any value "must not start with `-`"; `SSHURL` gives `ssh://user@host:2222`; `SSHRunner`'s arguments include `BatchMode=yes`, `--` before the host, and `-p` only with a port (test through a fake `ssh` script first in `PATH`, as in T030); stderr appears in the returned error; stdin reaches the command; a cancelled context stops it
- [X] T004 Implement `src/remote/runner.go` with the names above, and `src/remote/fake_test.go`'s `FakeRunner` (research R1)
- [X] T005 [P] Write failing tests in `src/remote/servers_test.go`: `LookupServer` gives the [table](data-model.md) for `jitsi`, `livekit`, `mediasoup`, `janus` (folder `docker-jitsi-meet` for Jitsi; Janus's services are `janus` only, so `janus-demos` is not started); an unknown name gives false; **pins**: each pin in `Images` equals the image in `tests/platforms/<folder>/docker-compose.yml`, read from the file, except Jitsi's, whose compose file uses `${JITSI_IMAGE_VERSION:-unstable}` and whose `stable-11248` comes from the rendered `.env`
- [X] T006 Implement `src/remote/servers.go`: the fixed table of [data-model.md](data-model.md), with Jitsi's containers `sfu-jitsi-jvb-1` and `sfu-jitsi-jicofo-1`, and each server's published signaling port first in `TCPPorts`
- [X] T007 [P] Write failing tests in `src/remote/envfile_test.go`: `RenderEnv` replaces `KEY=old`; replaces a commented `#JVB_ADVERTISE_IPS=192.168.1.10`; appends an absent key; leaves other lines and comments byte-identical; output is stable for one input; each of the four real `env.example` files plus each server's values gives no empty secret
- [X] T008 Implement `src/remote/envfile.go`, and in the same file `GenerateSecret()` (32 hex characters from `crypto/rand`) and per-server `Values(server, address, secrets)`, the "Env applied" table of [research R2](research.md): Jitsi `JVB_ADVERTISE_IPS`, `JICOFO_AUTH_PASSWORD`, `JVB_AUTH_PASSWORD`, `JITSI_IMAGE_VERSION=stable-11248`, `CONFIG=./config`, `HTTP_PORT`/`HTTPS_PORT` as in `env.example`; LiveKit `LIVEKIT_NODE_IP`, `LIVEKIT_API_SECRET`; mediasoup `MEDIASOUP_ANNOUNCED_ADDRESS`, `MEDIASOUP_LISTEN_IP=0.0.0.0`, `HTTP_LISTEN_IP=0.0.0.0`, `MEDIASOUP_DOMAIN`; Janus `JANUS_ICE_ENFORCE_LIST` when the address is on one interface, and `JANUS_NAT_1_1` when it is on none

**Checkpoint**: `go test -race ./src/remote/` green.

---

## Phase 3: User Story 1 - Run a media server on a remote machine (Priority: P1) 🎯 MVP

**Goal**: One command deploys any of the four servers at its pinned version; another shows it; another removes it. No server is left open to the internet.

**Independent Test**: From the workstation, deploy one server, confirm with `status` that it runs, join a room from a browser, remove it, and confirm nothing of it is left running.

### Tests for User Story 1

- [X] T009 [P] [US1] Write failing tests in `src/remote/machine_test.go`. `Classify` with output shaped like the validation machine's `ip -j addr` (loopback, `192.168.1.95/24`, `docker0` at `172.17.0.1/16`, no MAC address in the fixture `src/remote/testdata/ip-addr-private.json`) gives no public address; loopback and link-local are ignored; `100.64.0.0/10` and `fd00::/8` count as private; `203.0.113.7` and a global IPv6 address are public; a VPN plus a public address is public. `Check` reports every failure at once, not the first: not x86_64, `docker` without access, no Compose plugin, under 10 GB of disk, memory under the server's need, `ip` missing, and with `--allow` no passwordless `sudo iptables`. `rmem_max` or `wmem_max` under 4 MiB is a warning for mediasoup and nothing for the others
- [X] T010 [P] [US1] Write failing tests in `src/remote/firewall_test.go`: `FirewallCommands` builds chain `SFU-REMOTE` with an established `RETURN`, one `RETURN` per CIDR, then `DROP`; jumps from `INPUT` for host-network servers' ports and from `DOCKER-USER` with `--ctorigdstport` for Jitsi's published ports; only the server's ports appear, and port 22 never does; `ip6tables` commands appear only with a public IPv6 address; `0.0.0.0/0` and `::/0` are refused; rebuilding the body leaves the jumps alone; `RemoveFirewallCommands` deletes jumps, then flushes and deletes the chain
- [X] T011 [US1] Write failing tests in `src/remote/deploy_test.go` against `FakeRunner`, in the order of the [contract](contracts/sfu-remote-cli.md): a failed prerequisite changes nothing (no write command recorded); a public machine without `--allow` or `--external-firewall` is refused with the contract's message; the `current` server is removed first; the marker is written before the server starts, so a failed deployment can be cleaned up; `.env` and generated secrets appear only in `stdin`, never in any recorded `cmd`, and `.env` is written with mode 0600; the Compose command is `docker compose -p sfu-<server> -f docker-compose.yml -f docker-compose.remote.yml up -d --build <services>`; a second deployment of one server reuses its secrets; a same-named container not of this project fails the deployment and is not replaced; the wait ends with an error naming the container that did not start
- [X] T012 [US1] Add failing tests to `src/remote/deploy_test.go`: the tar sent for each server has no `.env`, no `*.pem`, no `*.key`, even when `tests/platforms/mediasoup/certs/key.pem` exists, and holds `docker-compose.remote.yml`; Jitsi runs `setup.sh` on the machine after `.env` is written; an image that differs from its pin fails the deployment and removes it (spec edge case); `status` reports `none`, `running`, `stopped` and `failed` as in the [data model](data-model.md), and prints no secret; `allow` needs a deployment using `--allow`; `remove` when nothing is deployed succeeds; `remove` touches only project `sfu-<server>`, `~/sfu-remote`, and the rules; `remove` deletes files that containers own through a throwaway container of an image already on the machine, and keeps images
- [X] T013 [P] [US1] Write failing tests in `cmd/sfu-remote/main_test.go`: exit status "0 done, 1 failed, 2 wrong usage"; each sub-command's flags from the [contract](contracts/sfu-remote-cli.md); an unknown server, a missing `--host` and `--allow 0.0.0.0/0` give status 2

### Implementation for User Story 1

- [X] T014 [P] [US1] Create overlays setting `restart: "no"` on every service (research R4): `tests/platforms/docker-jitsi-meet/docker-compose.remote.yml`, `tests/platforms/livekit/docker-compose.remote.yml`, `tests/platforms/mediasoup/docker-compose.remote.yml`, `tests/platforms/janus/docker-compose.remote.yml`. Janus's also sets `command` so that a non-empty `JANUS_NAT_1_1` adds `--nat-1-1`, keeping the Dockerfile's other arguments, with `$` escaped for Compose. Add a test to `src/remote/servers_test.go` that every service in each compose file has the overlay's restart value
- [X] T015 [P] [US1] Implement `src/remote/machine.go`: `Classify` with `net/netip`, and `Check` from one remote command that prints all the facts (research R3, R7)
- [X] T016 [P] [US1] Implement `src/remote/firewall.go` (research R4)
- [X] T017 [US1] Implement `Deploy` in `src/remote/deploy.go`: the contract's steps; the tar stream from `archive/tar` with the exclusions of T012; `~/sfu-remote/current` and `<server>/access`; waiting for the signaling port and containers, timeout 10 minutes plus the first build; the pin check with `docker inspect --format '{{.Config.Image}}'`
- [X] T018 [US1] Implement `Status`, `Allow` and `Remove` in `src/remote/deploy.go`; `Status` probes TCP ports from the workstation, names those unreachable, keeps exit status 0, and says UDP ports cannot be probed
- [X] T019 [US1] Implement `cmd/sfu-remote/main.go` with `flag`: thin, all logic in `src/remote`
- [X] T020 [US1] **Live, first server**: `go run ./cmd/sfu-remote deploy janus --host $T`, then `status` and `remove`, then `ssh $T docker ps -a`. Record in `specs/013-remote-sfu-deployment/validation-report.md` the build time, the deploy time with the image built (SC-001), and that no container remains (SC-003)
- [X] T021 [US1] **Live, the others**: repeat T020 for `livekit`, `mediasoup` and `jitsi`, with one `remove` by way of deploying another server (US1 scenario 3). After Jitsi's `remove`, check that `~/sfu-remote` holds nothing of it. Join a room from a browser on one server per the spec's independent test; ask the user to watch if no browser tool is available. Record results in `specs/013-remote-sfu-deployment/validation-report.md`

**Checkpoint**: All four servers deploy, report and remove on the real machine.

---

## Phase 4: User Story 2 - Run the same test plan against the remote server (Priority: P2)

**Goal**: The existing live tests run against the remote server with only its connection settings changed.

**Independent Test**: With a server deployed, run the 10-bot `audio_video` plan for 1 minute from the workstation: all bots connect, receive audio and video, and report files are written, on each of the four platforms.

### Tests for User Story 2

- [X] T022 [P] [US2] Write failing tests in `src/remote/settings_test.go` for the [contract](contracts/settings-file.md): the path uses `$XDG_CONFIG_HOME`, else `~/.config`, then `webrtc-load-engine/sfu-remote/<host>.env`; mode 0600, also when the file already exists with wider rights; each server's variables are exactly the contract's table, plus `DOCKER_HOST=ssh://<target>`; `JITSI_ROOM_URL` is `http://<address>:8000/loadtest`; `MEDIASOUP_ORIGIN` is `http://<address>:4443`; a value with a space or `$` survives `set -a; . <file>; set +a` in a real `sh`
- [X] T023 [P] [US2] Add failing tests to `src/remote/deploy_test.go`: `Deploy` writes the settings file only after success; `Remove` deletes it; `Status` prints the `settings:` and `url:` lines (FR-011) and the `tls:` line (FR-008b): `not needed (ws://)`, or the variable that skips checks when the URL is `wss://`

### Implementation for User Story 2

- [X] T024 [US2] Implement `src/remote/settings.go`, with the file written by `os.OpenFile` at 0600 and `Chmod` for an existing one
- [X] T025 [US2] Connect the settings file and the new `status` lines in `src/remote/deploy.go`
- [X] T026 [US2] Write `tests/live/remote_live_test.go` (`-tags live`, skipped unless `SFU_REMOTE_HOST` is set): for the server in `SFU_REMOTE_SERVER`, deploy, check `status`, confirm the settings file has mode 0600 and that its secret values appear in neither `status` output nor the machine's `ps`, remove, and confirm that nothing remains. Update the head comments of `tests/live/*_live_test.go` only if a remote run needs a variable they do not name
- [X] T027 [US2] **Live (SC-002, FR-012)**: on each of the four servers, deploy, source the settings file, and run 10 bots for 1 minute with `audio_video` and that server's live test ([quickstart](quickstart.md) step 3). Record bots connected, MOS, latency and the report's server URL in `specs/013-remote-sfu-deployment/validation-report.md`. Note any live test that assumed loopback, and fix it in its test file (FR-012)
- [X] T028 [US2] **Live (FR-013)**: with a run against a server whose media port is blocked or stopped, confirm that bots fail with their reason within the existing timeouts, and record it in `specs/013-remote-sfu-deployment/validation-report.md`

**Checkpoint**: 10-bot runs pass on all four remote servers.

---

## Phase 5: User Story 3 - Server load in remote reports (Priority: P3)

**Goal**: Reports of remote runs give the server's CPU and memory, read over SSH.

**Independent Test**: Run a plan against the remote server with server monitoring on: the report's server section gives CPU and memory for the named containers, and they match the machine's own view within 10%.

### Tests for User Story 3

- [X] T029 [US3] Update `TestDockerClient_Host` in `src/hostload/docker_api_test.go` first: `tcp://` still gives `remote_docker_host`; `ssh://user@host:2222` is accepted and does not; a host starting with `-` gives `docker_unreachable` and starts nothing
- [X] T030 [US3] Write failing tests in `src/hostload/docker_ssh_test.go` with a fake `ssh` script first in `PATH` that records its arguments and serves a canned HTTP stats answer on stdio: `stats` returns the figures and the same parsing as the socket; arguments are `-o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=15`, `-p` when set, `--`, the target, `docker system dial-stdio`; a second request reuses one `ssh` process; eight concurrent containers stay connected between samples; the process stays alive after the dialling request's context ends; `ssh` exiting non-zero gives `docker_unreachable` and the run goes on; a server slower than the timeout gives `docker_unreachable`; the request timeout is 4 s on SSH and unchanged locally; closing idle connections ends the process with no zombie left; DOCKER_HOST is read only from the environment, never the test plan

### Implementation for User Story 3

- [X] T031 [US3] Implement `src/hostload/docker_ssh.go`: a `net.Conn` over `ssh`'s stdin and stdout, started with `exec.Command`, not `CommandContext` (research R5), whose `Close` closes the pipes, then kills and waits for the process; deadlines handled so that a request's timeout returns
- [X] T032 [US3] Change `newDockerClient` in `src/hostload/docker.go` to accept `ssh://`, set `MaxIdleConnsPerHost: 8`, and use 4 s on SSH; update the type's comment. Add `close()` closing idle connections
- [X] T033 [US3] Call the client's `close()` when the sampler finishes in `src/hostload/sampler.go`, and add a test in `src/hostload/sampler_test.go` that no fake `ssh` process remains after `Run` returns
- [X] T034 [US3] Check that local behaviour is unchanged (FR-016): `go test -race ./src/hostload/`, then one short local run with monitoring on, comparing the report's server section with a run before this phase
- [X] T035 [US3] **Live (SC-006, FR-015)**: during a 10-bot run on each server, compare the report with `docker stats` on the machine (within 10%), and measure the `sshd` and `docker` processes' CPU with and without monitoring (at most 1% of one thread). Check that `ps` on the workstation shows no `ssh` after the run. Record in `specs/013-remote-sfu-deployment/validation-report.md`

**Checkpoint**: All three stories work together on the real machine.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Prove the security requirements, compare servers, and document.

- [X] T036 **Live (SC-007)**: take the generated values from the settings file, then search the repository, `reports/`, `status` output, the commands' logs and the machine's `ps` for them, and record the result in `specs/013-remote-sfu-deployment/validation-report.md` without quoting a value. Also search the repository and `reports/` for the machine's password, without writing it into any command: ask the user to run that search with `! grep -rIl` if needed
- [X] T037 **Live (SC-008, SC-009)**: with a server deployed, confirm from the workstation that port 2375 and any Docker API port answer nowhere, and that the server's own ports do. Ask the user to scan from outside the LAN or confirm that the router forwards nothing. Record in `specs/013-remote-sfu-deployment/validation-report.md`
- [X] T038 Record "`--allow` not validated on a real machine" in `specs/013-remote-sfu-deployment/validation-report.md`, and repeat it in the final report: the user chose on 2026-11-06 not to grant passwordless `sudo iptables` on the validation machine (research R11). Its rules keep their unit tests (T010); do not run `--allow` on the machine
- [X] T039 **Live (SC-004, SC-005)**: on each server, step up from 50 bots at cap 10 towards 100, alternated, recording the highest count that holds, send lateness, freezes and server CPU. Repeat one plan on another day, or record SC-005 as pending. Raise `wmem_max` first (T002). Record in `specs/013-remote-sfu-deployment/validation-report.md`, and say which limit stopped each server
- [X] T040 [P] Document in `README.md`: the `sfu-remote` commands, preparing a machine (key login, `docker` group, buffer limits), the [traffic table](research.md), reboots leaving nothing running, that the settings file's `DOCKER_HOST` also redirects that shell's own `docker` commands, how to accept a self-signed certificate for a browser check, and which README "Not done yet" lines this settles
- [X] T041 [P] Update `CLAUDE.md`: `sfu-remote` commands under Commands, and an entry under Recent Changes
- [X] T042 Run `go vet ./... && go test -race ./...` and `go build ./...`, confirm no new line in `go.mod` with `git diff go.mod`, confirm the [boundaries](#tasks-remote-sfu-deployment) with `git status`, and complete `specs/013-remote-sfu-deployment/validation-report.md`, naming every criterion not met or not checked

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: none.
- **Foundational (Phase 2)**: after Setup. Blocks all stories.
- **US1 (Phase 3)**: after Phase 2. The MVP.
- **US2 (Phase 4)**: after US1: a run needs a deployed server. T026–T028 need a deployment.
- **US3 (Phase 5)**: code needs only Phase 2, so T029–T034 may run in parallel with US1. T035 needs US1 and US2.
- **Polish (Phase 6)**: after the stories. T039 needs a raised `wmem_max` for mediasoup.

### Within Each Story

- Tests first, and see them fail for the right reason.
- T011 and T012 share `src/remote/deploy_test.go`; T017, T018 and T025 share `src/remote/deploy.go`: run them in order.
- T020 before T021: the first live deployment finds the shared mistakes cheaply. Janus builds from source, but its image is the smallest test of the whole path.

### Parallel Opportunities

- Phase 2: T003, T005 and T007 (tests), then T004, T006 and T008.
- US1: T009, T010 and T013 (tests), then T014, T015 and T016.
- US2: T022 and T023.
- Polish: T040 and T041.
- Live tasks never run in parallel: one server at a time (FR-004), and compared runs need the same machine state.

```text
# Phase 2 tests, together:
T003 src/remote/runner_test.go
T005 src/remote/servers_test.go
T007 src/remote/envfile_test.go
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Phases 1 and 2.
2. Phase 3 up to T020: Janus alone, deployed and removed on the real machine. **Stop and check**: if the path fails here, fix it before the other three.
3. T021. The operator can now run any server remotely, with no bots yet.

### Incremental Delivery

1. US1: servers run remotely.
2. US2: bots run against them, and every run gives quality figures.
3. US3: reports give server load, which is what the comparison needs.
4. Polish: security proof, then the comparison the feature exists for.

### Known gaps to report, not hide

- `--external-firewall` is a statement: only an outside scan proves it (R3).
- `--allow` is covered by unit tests only: it is not run on a real machine (T038).
- **Live tasks are blocked**: the machine has global IPv6 addresses, so `deploy` refuses it until the user decides (research R11). `--external-firewall` is never passed on the user's behalf.
- This machine's CPU may stop runs before 100 bots: SC-004 is then judged at the highest count that holds.
