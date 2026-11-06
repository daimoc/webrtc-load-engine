# Quickstart: Validating Remote SFU Deployment (013)

Needs a Linux amd64 machine reachable over SSH with key authentication, with
Docker and its Compose plugin, and the user in the `docker` group. Prefix `PATH`
with `/usr/local/go/bin`. `$T` is the target: `fetis@192.168.1.95` for the
validation machine ([research R11](research.md)).

Before mediasoup, raise the socket buffer limits on the machine, or `deploy` warns
and mediasoup's buffers stay capped:

```bash
printf 'net.core.rmem_max=4194304\nnet.core.wmem_max=4194304\n' | sudo tee /etc/sysctl.d/90-sfu-remote.conf
sudo sysctl --system
```

**Bandwidth**: 100 bots at cap 10 download about 560 Mbps, or about 250 GB an
hour ([research R9](research.md)). The validation LAN has room for that; over the
internet, start with 10 bots.

## 1. Automated tests

```bash
go vet ./... && go test -race ./...
```

Expected: green, with no machine needed.

## 2. Deploy (SC-001)

```bash
go run ./cmd/sfu-remote deploy janus --host $T
go run ./cmd/sfu-remote status --host $T
```

Expected: the server running, its signaling port reachable, no secret printed. On
a machine with a public address, add `--allow $(curl -s ifconfig.me)/32`.

## 3. Run (SC-002)

```bash
set -a; . ~/.config/webrtc-load-engine/sfu-remote/<host>.env; set +a
JANUS_ROOM=2013 JANUS_PARTICIPANTS=10 JANUS_HOLD=60s \
  go test -tags live -count=1 -v -run Janus ./tests/live/
```

Expected: the test passes, MOS at least 4.0, the report gives server CPU and
memory. Repeat steps 2–3 for `jitsi`, `livekit` and `mediasoup`, with the matching
live test.

## 4. Server load (SC-006)

During a run, compare with `docker stats` on the machine. Expected: within 10%.
Also compare the `sshd` and `docker` processes' CPU with and without monitoring:
at most 1% of one thread (FR-015), which matters on these low-power cores.

## 5. Exposure (SC-008, SC-009)

From a machine that is not allowed (a phone hotspot will do), scan the server's
[ports](data-model.md). Expected: none answers; from the allowed address, or from
the LAN on the validation machine, TCP ports do. Check that port 2375 answers from
nowhere.

## 6. Remove (SC-003)

```bash
go run ./cmd/sfu-remote remove --host $T
ssh $T docker ps; ssh $T sudo iptables -S | grep SFU-REMOTE
```

Expected: none of the server's containers, no rule, no settings file. The scan of
step 5 finds nothing from any address.

## 7. Comparison (SC-004, SC-005)

With a bot machine that has the bandwidth: 100 bots, cap 10, on each server,
alternated, then the same again on another day. Expected: no send lateness or
freezes from the bots' own CPU, and server CPU within 15% between the days. On the
validation machine, step up from 50 bots: if the server's CPU saturates first,
record the highest count that holds, and judge SC-004 and SC-005 there.

Record results in `validation-report.md`.
