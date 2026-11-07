# Validation Report: Remote SFU Deployment (013)

**Baseline**: commit `c389240a6247459fab0cd4b9c4d0b024c9a31f1f`, clean tree (012 committed). `go vet ./... && go test -race ./...`: 14 packages ok.

**Target**: `fetis@192.168.1.95` ([research R11](research.md)). On 2026-11-06: key login works, `docker ps -a` works without sudo and lists no container, `rmem_max` and `wmem_max` are 4194304, 201 GB free, 19.9 GB of memory.

## Checked without deploying

On 2026-11-06, on the real machine, changing nothing on it:

- `sfu-remote status` gives `server: none`.
- `sfu-remote deploy janus` is refused, naming a public IPv6 address, with exit status 1. No `~/sfu-remote` folder and no container afterwards.
- The Go client read the real Docker API through `DOCKER_HOST=ssh://`: 332 ms for the first request, 2 ms for the next two on one connection, and the right `not_found` reason. No `ssh` process remained.
- Local reading on this workstation (T034): `jmt-jitsi-jvb-1` gives 2.46 GB, and `docker stats` 2.293 GiB, the same figure.

Mutation checks: 13 behaviours broken one at a time (no reaping, ssh left after a run, one idle connection, private keys sent, public IPv6 ignored, public machine accepted, pins unchecked, whole internet allowed, settings mode, foreign container replaced, marker order). 12 were caught by a test. The 13th, ending ssh with its dialling context, changes nothing on this toolchain: Go's transport detaches the dial from the request (`net/http/transport.go`), so it says nothing about the test.

## SC-001: deploy in under 10 minutes

Met on all four. Times from `time sfu-remote deploy`, on 2026-11-07:

| Server | First deploy (build or pull) | Image already there |
|---|---|---|
| janus | 3 min 20 s | 7.5 s |
| livekit | 14 s (pull) | not timed |
| mediasoup | 2 min 54 s | not timed |
| jitsi | 1 min 10 s (pull) | 7.9 s; 15 s once it waits for jicofo |

Each first deploy also removed the previous server, which is US1 scenario 3.

## SC-002: 10-bot run on each server

10 bots, `audio_video`, 60 s hold, over the LAN, on 2026-11-07. All reports give the remote URL.

| Server | Bots connected | Audio MOS | Video freezes (hold / run) | Server CPU avg | Result |
|---|---|---|---|---|---|
| janus | 10/10 | 4.37 | 0 / 0 | 0.79 threads | pass |
| livekit | 10/10 | 4.37 | 0 / 0 | 0.85 threads | pass |
| mediasoup | 10/10 | 4.37 | 0 / 0 | 0.49 threads | pass |
| jitsi, as deployed | 10/10 | 4.27 to 4.37 | 1 / 23, 20 / 49, 14 / 56, 13 / 40 | 1.2 to 1.5 threads | **not met** |
| jitsi, `trust-bwe=false`, warm | 10/10 | 4.37 | 0 / 0 | 1.20 threads | pass, experiment only |
| jitsi, overlay with `JVB_CC_TRUST_BWE=false`, first run after deploy | 10/10 | 4.34 | 0 / 26 | 1.49 threads | **pass** |
| jitsi, same, second run | 10/10 | 4.37 | 0 / 0 | 1.11 threads | **pass** |

No live test assumed loopback (FR-012).

**Why Jitsi fails**: the bridge drops streams by its own bandwidth estimate. Each bot needs 4.6 Mbps. Mid-run, seven bots had estimates of 5.5 to 6.3 Mbps and no suspended stream, and the three below the need (2.9, 3.3, 3.8 Mbps) were exactly the three with suspended streams, on a 1 Gb/s link with 0 packets lost. With `videobridge.cc.trust-bwe = false` a warm run has 0 freezes, so the cause is proven. Why the estimate stays low on a real network, and whether the bots' TWCC feedback plays a part, is not known. On 2026-11-07 the user chose to run Jitsi with `trust-bwe=false`: the overlay sets `JVB_CC_TRUST_BWE: "false"` on `jvb`, and the bridge's debug output shows `trustBwe: false` with no suspended stream, at 50 bots too. Remote Jitsi results therefore show a bridge that forwards every stream, like the other three servers, not Jitsi's default. The local setup keeps the default, and the reports do not record the setting.

A separate effect: the first run after the bridge starts is worse (215 freezes during the hold, 3.77 threads peak, with `trust-bwe=false`), then the next is clean. It looks like the JVM warming up; this was not isolated.

FR-013: with mediasoup announcing the unreachable `192.0.2.1`, signaling worked and 3 of 3 bots failed in about 30 s with `stage=transport`, "ICE/DTLS transport failed", and the report says no bot received media. A first attempt on Janus proved nothing, because the fault did not take: Janus reaches the bots' own addresses, so a wrong announced address does not stop its media.

## SC-003: nothing left after removal

Met, after a fix. On the machine after `remove`, proved on Jitsi, the largest setup, and again after the last run: 0 containers, 0 volumes, 0 project networks, `~/sfu-remote` empty, no settings file. It took 2 s for janus.

Images stay on the machine (7), so that the next deployment is fast. There were no firewall rules to check: `--allow` was not used.

## Browser join (US1 independent test, T021)

Met on 2026-11-07, after a fix. Jitsi deployed on the machine; the user joined one room from 3 Chrome tabs on the workstation over `https://192.168.1.95:8443`, with audio and video both ways. The bridge on the machine showed the conference with 3 endpoints, each sending it about 1.3 MB and receiving about 1.6 MB, so the media went through the machine.

The first attempt passed in the browser but not on the machine: jicofo and the bridge on the machine saw no conference. The page's `config.js` said `wss://localhost:8443/xmpp-websocket`, so the tabs joined the Jitsi running on the workstation. Fixed by setting `PUBLIC_URL` to the machine's address (`src/remote/envfile.go`); the second attempt is the one above.

## SC-004: bots no longer limit the run

Not shown at 100 bots: no server held 75 on this 4-core machine, so the step-up stopped there. What the runs do show is that the workstation was not short of CPU, at 4.66 of 16 threads at most. They do not show which of the server or the network stopped LiveKit at 75.

Cap 10, `audio_video`, ramp-up 10 s, hold 120 s, 2026-11-07, alternated:

| Server | Bots | Result | Audio MOS | Server CPU avg | What stopped it |
|---|---|---|---|---|---|
| janus | 40 | pass | 4.37 | 2.33 threads | |
| janus | 45 | **pass, highest** | 4.36 | 2.72 threads | |
| janus | 50 | fail | 4.32 | 3.01 threads | 1 freeze during the hold; load average 50 |
| livekit | 50 | **pass, highest** | 4.37 | 1.85 threads | |
| livekit | 75 | fail | 4.36, worst 4.11 | 2.43 threads | audio concealed p95 2.1%, limit 1%; 0 freezes |
| mediasoup | 30 | **pass, highest** | 4.37 | 0.71 threads | |
| mediasoup | 40 | fail | 2.18 | 1.09 threads | collapse, one bot lost signaling |
| mediasoup | 50 | fail | 1.87 | 1.34 threads | collapse, one bot lost signaling |
| mediasoup, 2 workers | 50 | fail | 2.77 | 1.45 threads | one bot lost signaling after 29 s |
| jitsi | 20 | pass | 4.36 | 1.78 threads | |
| jitsi | 30 | **pass, highest** | 4.32 | 2.39 threads | 6 freezes before the hold, none during |
| jitsi | 40 | fail | 3.93 | 3.02 threads | 337 freezes during the hold, audio concealed 7.7% |
| jitsi | 50 | fail | 1.00 | 3.88 threads | the bridge saturates (3.96 of 4 cores, 4.1 GB, 1.36 s latency) |

Each pass is one run at that count. mediasoup's collapse fits one worker on a slower CPU than the laptop, but the two-worker run failed too at under 2 threads, so that limit is not fully explained. In all three failures the bot was dropped by the server ("closed by protoo-server", code 4000). Send lateness is not in the report's key figures. The live tests' send rates during the hold (50 audio and 66 video packets per second per bot, the source rates, at 40 Jitsi bots too) show the bots kept pace.

## SC-005: server CPU repeatable across days

Pending: all runs are from one day.

## SC-006: reported load matches the machine

Met on all four. Report average against `docker stats` samples during the hold:

| Server | Report CPU | `docker stats` | Report memory peak | `docker stats` |
|---|---|---|---|---|
| janus | 0.788 threads | 0.79 | 15.7 MiB | 15 MiB |
| livekit | 0.85 | 0.82 | 84.4 MiB | 80.4 MiB |
| mediasoup | 0.49 | 0.45 | 54.8 MiB | 54.0 MiB |
| jitsi (jvb + jicofo), 10 bots | 1.11 | 1.13 | 808.7 MiB | |
| jitsi, 20 bots | 1.78 | 1.77 | | |
| jitsi, 30 bots | 2.39 | 2.63 | | |
| jitsi, 40 bots | 3.02 | 3.16 | | |

For Jitsi, `docker stats` is averaged over 12 samples from 15 s into each run, since one sample against a run average was 11% off. The 30-bot run is at the 10% limit. The first run after a deploy is 1.49 against 1.23, because the report's average includes the bridge warming up at join.

Cost of monitoring: CPU of `sshd` and `docker` on the machine over a 10-bot run (60 s hold) with monitoring and the same run without. The run without still counts the samples' own ssh and dockerd's background work, so the difference is the cost:

| Server | With monitoring | Without | Cost |
|---|---|---|---|
| livekit | 0.167% of a thread | 0.167% | 0.00% |
| jitsi | 0.547% | 0.514% | 0.03% |
| janus | 0.451% | 0.131% | 0.32% |
| mediasoup | 0.528% | 0.309% | 0.22% |

Under the 1% limit on all four, and so is every total. No `ssh` or `dial-stdio` remained on the workstation after any run. The Jitsi run with monitoring, the first after its deploy, had 46 freezes before its hold, as the earlier first runs did (SC-002); the run without had none.

## SC-007: no secret leaks

LiveKit's generated secret (34 characters, not quoted) was searched for: 0 hits in the repository, git history, `reports/`, this session's logs, `status` output, the machine's and the workstation's process lists, and the machine's shell history. Both `.env` files have mode 600.

Jitsi's two generated passwords (32 characters each, not quoted), searched on 2026-11-07 during a deployment: 0 hits in the repository, git history, the scratch logs, this session's logs, `status` output, the machine's and the workstation's process lists, and the machine's shell history. The machine's `.env` and the settings file have mode 600. The machine's login password: on 2026-11-07 the user searched the repository (`reports/` included) in their own terminal, typing the password at a hidden prompt, with `grep -rIlF`: no file holds it (exit 1). Not searched for it: git history and this session's logs, where it was typed when the work began.

## SC-008: port scan

From the workstation with janus deployed, every TCP port: only 22 and 8188 open. After the last removal, on 2026-11-07, every server port (4443, 7880, 7881, 8000, 8188, 8443) and 2375/2376 are closed; only 22 answers. Not a scan from outside the LAN: on 2026-11-07 the user confirmed that the router forwards no port to the machine. UDP was not scanned. IPv6 is off on the machine's interface, so nothing listens on a global IPv6 address.

## SC-009: Docker API unreachable

Met. Ports 2375 and 2376 are closed from the workstation, `dockerd` runs with `-H fd://` only, and nothing listens on 2375 on the machine's loopback.

## Surprises on real servers

- **The validation machine has two global IPv6 addresses**, missed by the first survey, which said it had only a private address. T009's real-address fixture showed it, and `deploy` refused the machine as designed. On 2026-11-07 the user turned IPv6 off on `enp2s0f0` (`/etc/sysctl.d/91-sfu-remote-noipv6.conf`); `deploy` then accepted the machine on its own check, with no flag.
- **Janus listened on 127.0.0.1 only.** The deployed `conf/` binds the WebSocket to loopback, so port 8188 was closed from outside. `status` named it. Fixed with `tests/platforms/janus/remote/`, mounted by the overlay; the local setup stays on loopback.
- **Janus refuses `ws_ip = "0.0.0.0"`** ("No interface associated with 0.0.0.0?") and exits. The remote file leaves `ws_ip` out.
- **`deploy` waited 10 minutes for a container that had exited.** With `restart: "no"` it cannot come back. `deploy` now stops at once, and names `docker logs` rather than quoting logs, which could hold a secret.
- **`remove` left an anonymous volume per Jitsi deployment.** It now runs `down -v`.
- **`deploy` called Jitsi running before a bot could join.** Its ports open before jicofo has a bridge, and the first bot then got "conference focus unavailable". `deploy` now waits until jicofo's `/debug` shows an operational bridge (`Server.Ready`).
- **A browser on the deployed Jitsi joined the workstation's own Jitsi.** Without `PUBLIC_URL`, the web image tells the browser to connect to `localhost`; bots do not read that page, so no run showed it. `PUBLIC_URL` is now `https://<address>:8443`.
- **Deploying the same server again left a volume behind.** Redeploying Jitsi with new settings recreated its web container, and the old container's anonymous volume stayed, out of `remove`'s reach. `deploy` now runs `down -v` on the same server before starting it, keeping its folder and secrets. Checked on the machine: two deploys in a row leave 1 volume, and `remove` then leaves 0 containers, 0 volumes, 0 networks and an empty folder.

`--allow` not validated on a real machine: the user chose on 2026-11-06 not to grant passwordless `sudo iptables` (research R11). Its rules keep their unit tests.

## Not met or not checked (T042)

Final checks on 2026-11-07: `go vet ./...`, `go vet -tags live ./tests/live/` and `go test -race ./...` pass (16 packages ok), `go build ./...` passes, `go.mod` and `go.sum` unchanged. `git status`: production changes only in `cmd/sfu-remote`, `src/remote`, `src/hostload` (`docker.go`, `sampler.go` and T031's `docker_ssh.go`) and new `docker-compose.remote.yml` files; no change in `src/media`, `src/execution`, `src/platform`, `src/scenario`, `src/report`, test plans or existing compose files; `src/remote` imports nothing from this module. `.gitignore` now keeps the Jitsi overlay, which it hid: without that, a commit would leave it out.

- **SC-004, not met as written**: no server held 100 bots on this 4-core machine, so the 100-bot run was not made. The workstation was not the limit in any run (4.66 of 16 threads at most).
- **SC-005, not checked**: every run is from 2026-11-07. It needs the same runs on another day.
- **SC-008, partly checked**: no scan from an address that is not allowed. Without `--allow` there is no such address on the LAN: every LAN host reaches the server's ports, and the user's router keeps the internet out (the user's confirmation, not a scan from outside). UDP was not scanned. After removal, nothing but 22 answers.
- **`--allow`, not validated on a real machine**: the user declined passwordless `sudo iptables`. Its firewall rules have unit tests only.
- **SC-007, partly checked for the machine's login password**: searched for in the repository and `reports/` only. Not in git history (nothing was committed since it was given, but not searched), and it is in this session's logs, where it was typed when the work began. The user should change it. The generated secrets of LiveKit and Jitsi: 0 hits everywhere searched.
- **Jitsi's figures are not its default behaviour**: the remote overlay sets `JVB_CC_TRUST_BWE=false` (SC-002); the reports do not record that setting.

