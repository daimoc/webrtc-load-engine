# Data Model: Remote SFU Deployment (013)

## Server (`src/remote`, fixed table)

| Field | Jitsi | LiveKit | mediasoup | Janus |
|---|---|---|---|---|
| Name | `jitsi` | `livekit` | `mediasoup` | `janus` |
| Folder | `docker-jitsi-meet` | `livekit` | `mediasoup` | `janus` |
| Services | all | all | all | `janus` only |
| Monitored containers | `sfu-jitsi-jvb-1`, `sfu-jitsi-jicofo-1` | `livekit-server` | `mediasoup-demo` | `janus` |
| TCP ports | 8000, 8443 | 7880, 7881 | 4443, 44444–44445 | 8188 |
| UDP ports | 10000 | 50000–60000 | 44444–44445 | 20000–20999 |
| Memory needed | 10 GB | 2 GB | 2 GB | 2 GB |
| Pinned images | `ghcr.io/jitsi/*:stable-11248` | `livekit/livekit-server:v1.13.10` | `webrtc-load-engine/mediasoup-demo:899109e` | `webrtc-load-engine/janus:v1.4.3` |

mediasoup's media ports are 44444 plus one per worker, so they follow
`MEDIASOUP_NUM_WORKERS` (2 by default).

## Target

| Field | Rule |
|---|---|
| Host | `[user@]host[:port]`, or an `~/.ssh/config` alias; must not start with `-` |
| Announced address | `--address`, else the host part when it is an IP, else asked for; a valid IP |
| Access | `private`, `allow` or `external` (research R3) |
| Allowed | With `allow`: 1 or more CIDRs; `0.0.0.0/0` and `::/0` are refused |

## Deployment

On the machine, `~/sfu-remote/`:

| Item | Content |
|---|---|
| `current` | Name of the deployed server |
| `<server>/` | The setup, overlay and rendered `.env` (0600) |
| `<server>/access` | The access mode and allowed CIDRs |

### States

```text
none --deploy--> running --remove--> none
running --deploy other--> running (other)
any --failed deploy--> failed --deploy or remove--> running or none
running --reboot--> stopped (rules gone, containers do not restart) --deploy--> running
```

`status` reports `none`, `running`, `stopped` or `failed`; `failed` means the
marker exists but not all containers run.

## Settings file

Workstation, mode 0600 ([contract](contracts/settings-file.md)). One per host, and
rewritten by each `deploy`.

## Unchanged

Test plans, the quality summary, report files and metrics. The only engine change
is which `DOCKER_HOST` values `src/hostload` accepts.
