# Research: Remote SFU Deployment (013)

## R1: How to drive the machine

**Decision**: a Go command, `cmd/sfu-remote`, running the operator's `ssh`.

**Rationale**: `ssh` already works for the operator: keys, agent, `~/.ssh/config`,
jump hosts and known hosts come for free, and no credentials enter this project.
`BatchMode=yes` makes a failure an error rather than a prompt. Go gives
per-server rendering and address logic real unit tests through a fake runner.

**Alternatives considered**:
- *`DOCKER_HOST=ssh://… docker compose`.* Bind mounts name paths on the machine,
  so the setups' config files would not be there. Files must be sent anyway.
- *A shell script.* Env rendering, CIDR logic and rollback are awkward to test.
- *Ansible.* A new dependency on the workstation for four short sequences.

## R2: What runs on the machine

**Decision**: send `tests/platforms/<server>/` as a tar stream over `ssh`, except
`.env`, write a rendered `.env`, and run
`docker compose -p sfu-<server> -f docker-compose.yml -f docker-compose.remote.yml up -d --build`
there, with only the services the server needs.

- `tar` is everywhere; `rsync` might not be.
- Besides `.env`, `*.pem` and `*.key` stay behind: a local certificate's private
  key must not leave the workstation, and `certs/` may hold one.
- Building on the machine uses the same pinned sources (FR-003).
- The Compose project name is what bounds `remove`: it only touches its own
  project (FR-009), and a same-named container that is not ours makes Compose
  fail rather than get replaced.

**Overlay** (`docker-compose.remote.yml`), per server:

| Server | What it changes |
|---|---|
| All | `restart: "no"` (R4) |
| Janus | `command` adds `--nat-1-1` when the announced address is not on an interface; the Dockerfile stays as it is |

**Env applied** per server:

| Server | Values |
|---|---|
| Jitsi | `JVB_ADVERTISE_IPS`, generated `JICOFO_AUTH_PASSWORD`, `JVB_AUTH_PASSWORD`, `JITSI_IMAGE_VERSION=stable-11248`, `CONFIG` inside the deployment folder |
| LiveKit | `LIVEKIT_NODE_IP`, generated `LIVEKIT_API_SECRET` |
| mediasoup | `MEDIASOUP_ANNOUNCED_ADDRESS`, `MEDIASOUP_LISTEN_IP=0.0.0.0`, `HTTP_LISTEN_IP=0.0.0.0`, `MEDIASOUP_DOMAIN` |
| Janus | `JANUS_ICE_ENFORCE_LIST` when the address is on one interface |

Keys in `env.example` may be commented out (`#JVB_ADVERTISE_IPS=`): rendering
replaces the line, commented or not, and appends a key that is absent.

**Found**:
- **Jitsi's containers own files in `CONFIG` as other users**, so `rm -rf` as the
  operator fails and `remove` would leave them. `remove` deletes the folder from a
  throwaway container of an image the deployment already has. `CONFIG` is
  `./config` so that `setup.sh`, run on the machine after `.env` is written, and
  Compose both resolve it inside the deployment folder.
- Pinned versions are checked after start: each container's image must equal the
  server's pin (spec edge case). A test keeps the pins equal to the compose files.
- mediasoup binds `127.0.0.1` by default: without the listen values, nothing is
  reachable. Its origin check also needs `MEDIASOUP_DOMAIN` and the bots' `origin`.
- **Jitsi is not pinned locally**: the compose file defaults to `unstable`. The
  validated stack on this workstation runs `stable-11248`, so that tag is pinned
  for deployments (FR-003). The local default is left as it is (FR-016); pinning
  it too is worth a separate decision.

## R3: Is a machine restricted? (FR-008)

**Decision**: read the machine's addresses (`ip -j addr`) and classify them.

- All addresses private (RFC 1918, 100.64.0.0/10, fd00::/8; loopback and
  link-local ignored): allowed. The private network is the restriction.
- Any public address: deployment refuses unless given `--allow <cidr>…` (R4) or
  `--external-firewall`, where the operator states a cloud or network firewall
  exists. The choice is recorded and shown by `status`.

**Rationale**: the servers use host networking, or publish on all interfaces, so
every interface is exposed. Looking only at the announced address would pass a
VPS with a VPN address and an open public one.

**Limit**: `--external-firewall` is a statement, not a check: a machine cannot see
a firewall in front of it. SC-008's outside scan is what proves it.

**Alternatives considered**: binding each server to one address. Each server
does this differently, and Jitsi publishes through Docker.

## R4: Firewall rules (`--allow`)

**Decision**: a chain `SFU-REMOTE` with one `RETURN` per allowed CIDR, an
established-connections `RETURN`, then `DROP`. It is entered only for the running
server's ports, from `INPUT` (host-network servers) and from `DOCKER-USER`
(Jitsi's published ports, matched with `--ctorigdstport` because DNAT has already
changed the port). The same goes for `ip6tables` when the machine has a public
IPv6 address.

- Only the server's ports are matched, so SSH is never at risk.
- `allow` rebuilds the chain's body without touching the jumps (spec edge case:
  the operator's address changes).
- `remove` deletes the jumps and the chain (FR-008c).

**Reboots**: rules do not survive one, and `restart: unless-stopped` would bring
a server back unprotected. So the overlay sets `restart: "no"`: after a reboot,
nothing runs, which is the safe side. Saving rules across reboots would touch the
distribution's own firewall setup.

## R5: Reading server load over SSH (FR-014, FR-015)

**Decision**: `DOCKER_HOST=ssh://[user@]host[:port]` makes `dockerClient` dial by
starting `ssh -o BatchMode=yes -o ConnectTimeout=5 -o ServerAliveInterval=15
[-p port] -- [user@]host docker system dial-stdio`, and using its stdin and stdout
as the connection. The Docker CLI does the same.

- Requests, parsing and reasons do not change. `tcp://` stays
  `remote_docker_host`.
- The Docker API stays on its socket: nothing opens (FR-008a, FR-015).
- HTTP keep-alive reuses one `ssh` per concurrent request, so handshakes happen
  once. `MaxIdleConnsPerHost` goes from the default 2 to 8: Jitsi can name more
  containers than that.
- A request's timeout rises to 4 s on SSH, within a 5 s interval: the first
  sample includes a handshake. It stays 2 s locally.
- A failed `ssh` gives the existing `docker_unreachable`, and the run goes on.
- The `ssh` process outlives the request that dialled it, so it is not tied to the
  request's context. It ends when its connection closes, and the sampler closes
  idle connections when it stops, so no `ssh` outlives a run.

**Why an environment variable and not a test plan field**: the engine would
otherwise start `ssh` to a host named by an API caller. `DOCKER_HOST` is set by
whoever runs the engine, as today, and the test plan does not change (FR-010).

## R6: Secrets and settings (FR-007, FR-011)

**Decision**: generated at deployment with `crypto/rand`, kept in the `.env` on
the machine (mode 0600) and in one workstation file (mode 0600): `sfu-remote/<host>.env`,
in the user's config folder ([contract](contracts/settings-file.md)).

Secrets travel on `ssh`'s stdin, never on a command line, where the machine's
`ps` would show them. `status` prints addresses and the file's path, never its
secrets. Reports already
redact secrets (`[redacted]`), and the URL they keep says which server a run
targeted (US2 scenario 2), so reports need no change.

**Not encrypted**: signaling stays `ws://` as in the local setups, with `wss://`
where a server already has it. On a private network or VPN that is fine. With
`--allow` over the internet, a LiveKit token could be read on the path; it is
short-lived and useless without being on an allowed address. This is the
accepted cost of Q2's restricted access, which keeps self-signed setups simple.

## R7: Prerequisites (FR-005)

**Decision**: checked before anything changes, and all failures reported at once:
Linux on x86_64; `docker` usable without sudo; Compose plugin; free disk (10 GB;
Janus and mediasoup build from source); memory (Jitsi 10 GB for its 8 GB bridge
heap, others 2 GB); `ip`; with `--allow`, passwordless `sudo iptables`.

For mediasoup, `net.core.rmem_max` or `net.core.wmem_max` under 4 MiB gives a
warning: they silently cap its socket buffers (010 T028). Changing a kernel
setting is left to the operator.

## R8: One server at a time (FR-004)

**Decision**: `~/sfu-remote/current` names the deployed server. `deploy` removes
that one first, and is safe to run again (FR-005): Compose and the rules converge.
A deployment that failed keeps its marker, so `remove` still cleans it up.

Secrets are reused when the same server is deployed again, so a retry does not
invalidate the workstation's settings.

## R9: Bandwidth

**Decision**: documented in the README and shown in the quickstart, not enforced.

From the bundled content, one bot sends 0.50 Mbps of video and 0.06 Mbps of audio,
before packet headers. At cap N a bot receives N streams of each kind.

| Bots | Cap | Bots' upload | Bots' download | Downloaded per hour |
|---|---|---|---|---|
| 10 | none (9) | 6 Mbps | 50 Mbps | about 25 GB |
| 50 | 10 | 28 Mbps | 280 Mbps | about 125 GB |
| 100 | 10 | 56 Mbps | 560 Mbps | about 250 GB |

So SC-004 needs a well-connected bot machine, and a metered provider can cost real
money. A bot machine in the same data centre is the usual answer, and `sfu-remote`
does not care where the bots run. The validation machine's LAN has that bandwidth
(R11). Latency over a real path includes that path: it
is not comparable with local figures, only between servers on the same path.

## R10: Testing

**Decision**: fakes in `go test`, and real-machine tests behind `-tags live`
against the validation machine (R11).

The unit tests prove what is sent, not that it works, so `go test` needs no
machine and the live tests carry the proof. The first real deployment will likely
find per-server surprises, as each local setup's first run did (loopback ICE on
Janus, the port mux on LiveKit).

The address classification's tests use the shape of this machine's real
`ip -j addr` output (loopback, a private LAN address, `docker0`), with its MAC
address removed.

**Alternative**: an sshd and Docker-in-Docker container as a stand-in. Dropped: a
real machine is available, and the stand-in's networking would not match it.

## R11: The validation machine

**Decision**: validate on `fetis@192.168.1.95`, surveyed on 2026-11-06.

| Item | Found | Effect |
|---|---|---|
| CPU | AMD PRO A10-8770E, 4 cores, 4 threads, 2.8 GHz max; AES, AVX2 | Probably the limit of a run: results compare servers on this machine, not with local bot counts |
| Memory, disk | 18 GiB, 201 GB free | Passes R7, Jitsi included |
| Software | Linux Mint 22.3, Docker 29.9.3, Compose v5.7.0, `ip -j` works | Passes R7 |
| Access | Key login, `docker` group; `sudo` asks for a password | Passes R7; only `--allow` needs passwordless `sudo` |
| Addresses | One private IPv4 address; no public or global IPv6 address | Restricted under R3, so no `--allow` |
| Docker over SSH | Docker CLI through `ssh://`: 0.34–0.41 s for a fresh connection | R5's 4 s timeout holds; the Go client is still to be written |
| Network | 1000 Mb/s at both ends; 871 Mbps down, 824 Mbps up, through SSH over TCP | Enough for 100 bots (560 Mbps plus headers); UDP loss is not measured |
| `ufw` | `ENABLED=no` | Nothing visible blocks media; `iptables` itself needs root, so the first run proves it |
| Power | Desktop session; suspend on AC is off (timeout 0); no suspend logged | A run will not be cut by sleep |
| `net.core.rmem_max`, `wmem_max` | 212992, under R7's 4 MiB | **R7's warning fires.** The operator raises both before mediasoup runs |
| Address assignment | DHCP | The address can change. A reservation on the router avoids re-deploying |

**`--allow` is not proved on a real machine**: its rules need passwordless `sudo
iptables`, which this machine does not grant, and the operator chose not to add
it. They keep their unit tests, which prove what is sent, not that it blocks.
Run it on a machine first before relying on it.

**Limits**: this machine's private address means every device on the LAN can reach
a deployed server. Q2 accepts a private network as restricted access, and no
secret protects media beyond the local setups'. The router must not forward these
ports. The time to build Janus and mediasoup on these cores is unknown; SC-001
counts deployment with images already built.
