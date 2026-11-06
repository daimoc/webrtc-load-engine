# Contract: `sfu-remote` command (013)

```text
sfu-remote deploy <jitsi|livekit|mediasoup|janus> --host <target> [flags]
sfu-remote status --host <target>
sfu-remote allow  --host <target> --allow <cidr> [--allow <cidr>…]
sfu-remote remove --host <target>
```

| Flag | Meaning |
|---|---|
| `--host` | `[user@]host[:port]`, or an `~/.ssh/config` alias |
| `--address` | Address the server announces, when it differs from the host |
| `--allow` | Repeatable. Only these CIDRs reach the server's ports; installs firewall rules |
| `--external-firewall` | The operator states that a firewall outside the machine restricts access |
| `--platforms-dir` | Default `tests/platforms` |

Exit status: 0 done, 1 failed, 2 wrong usage.

## `deploy`

1. Checks prerequisites, reporting every failure, before changing anything.
2. Classifies the machine. Refuses when any address is public and neither
   `--allow` nor `--external-firewall` is given.
3. Removes the server in `current`, if any.
4. Sends the setup, writes `.env`, installs rules, starts the server.
5. Waits for its signaling port, and for all its containers.
6. Checks that each container's image is the server's pin. A mismatch fails the
   deployment and removes it.
7. Writes the settings file, then prints what `status` prints.

Refusal message for an unrestricted machine:

```text
203.0.113.7 is a public address, and the servers listen on every interface.
Give --allow <your-cidr>, or --external-firewall if a firewall already restricts it.
```

## `status`

```text
host:      deploy@203.0.113.7
server:    janus (running)
access:    allow 198.51.100.4/32
address:   203.0.113.7
url:       ws://203.0.113.7:8188
settings:  /home/me/.config/webrtc-load-engine/sfu-remote/203.0.113.7.env
tls:       not needed (ws://)
reachable: 8188/tcp ok
           UDP ports cannot be probed: a bot run shows them
```

Never prints a secret. Unreachable TCP ports are named, and exit status stays 0:
the command reports, it does not judge.

## `allow`

Replaces the allowed CIDRs. Needs a deployment using `--allow`; refused for
`0.0.0.0/0` and `::/0`.

## `remove`

Stops the project, deletes its folder, `current`, the rules, and the settings file.
Removing when nothing is deployed succeeds. Images stay, for the next deployment.
Files that containers wrote as other users are removed too.
