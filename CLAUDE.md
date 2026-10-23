# webrtc-load-engine Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-01-06

## Development workflow

Spec Kit is the source of truth for requirements, specifications,
architecture and implementation tasks.

Superpowers skills may be used for implementation practices such as
TDD, debugging, planning execution and code review.

Do not create a second specification or implementation plan when
Spec Kit already provides one.

When a Spec Kit task is being implemented, follow the existing
Spec Kit task and use Superpowers practices to execute it.


## Active Technologies
- Go 1.23.0 + Pion, net/http, `google/uuid` (002-scenario-execution-engine)
- In-memory (002-scenario-execution-engine)
- Go 1.23.0 + `mellium.im/xmpp`, `nhooyr.io/websocket`, `pion/webrtc/v3` (003-xmpp-signaling)
- N/A (In-memory session state) (003-xmpp-signaling)
- Go 1.23.0 + `pion/webrtc/v3` (TrackLocalStaticRTP, ivf/ogg readers), `pion/rtp`, `pion/rtcp`; media content embedded with `go:embed` (004-synthetic-media)
- Go 1.23.0 + `pion/webrtc/v3` (DataChannel), `pion/interceptor/pkg/twcc` (own TWCC feedback); Jitsi Colibri bridge channel messages (`ReceiverVideoConstraints`, `ReceiverAudioSubscription`) in the Jitsi adapter (005-bridge-channel-subscriptions)
- Go 1.23.0 + `pion/webrtc/v3` (RID simulcast encodings, RTCP PLI), `pion/rtp` (VP8 payload descriptor); E-model score; `pion/transport/v2/vnet` in tests (006-media-quality)
- Go 1.23.0 + Docker Engine API over its Unix socket with `net/http` (no Docker SDK), `syscall.Getrusage`, `/proc/self/statm` (007-server-load-report)
- Go 1.23.0 + `pion/webrtc/v3` (second PeerConnection: publish session the peer offers), `nhooyr.io/websocket`, `google.golang.org/protobuf/encoding/protowire` (hand-written LiveKit signaling codec; no LiveKit SDK/protocol module) (008-livekit-platform)
- Go 1.23.0 (keep 1.23-compatible code) + standard library only (`encoding/json`, `os`, `path/filepath`, `runtime/debug`, `strings`); `google/uuid` (already used) for run IDs. No new module. (009-run-report-files)
- files: two per run in the reports folder (`reports/` by default); the scenario keeps only the paths in memory (009-run-report-files)
- Go 1.23.0 + `nhooyr.io/websocket` (protoo JSON, subprotocol `protoo`), `pion/sdp/v3` (SDP built from mediasoup's ICE/DTLS/RTP parameters, as mediasoup-client does), 008 publish session reused; no new module (010-mediasoup-platform)
- Go 1.23.0 + `nhooyr.io/websocket` (Janus JSON API, subprotocol `janus-protocol`), 008 publish session and 010 consent checks reused; Janus v1.4.3 VideoRoom (multistream, dummy publisher); no new module (011-janus-platform)

- (001-create-scenario-api)

## Project Structure

```text
src/
tests/
```

## Commands

```bash
go build ./...                 # build everything
go vet ./... && go test -race ./...   # lint + tests (keep this green)
go run ./src                   # HTTP API on :8080 (/api/v1/scenarios, /metrics)
go run ./cmd/jitsi-connect --domain meet.example.org --room test --log-xmpp   # manual Jitsi join
go run ./cmd/jitsi-connect --domain meet.example.org --room test --participants 2 --media audio_video   # bots sending audio+video
go run ./cmd/jitsi-connect --domain meet.example.org --room test --participants 2 --media audio_video --media-file clip.mp4   # bots sending a file's audio+video (needs ffmpeg)
go run ./cmd/jitsi-connect --domain meet.example.org --room test --participants 2 --server-containers docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1   # also sample the server containers' CPU/memory (prints server + load_machine at the end)
set -a; . tests/platforms/livekit/.env; set +a; LIVEKIT_URL=ws://localhost:7880 LIVEKIT_ROOM=load008 LIVEKIT_PARTICIPANTS=10 go test -tags live -count=1 -v -run LiveKit ./tests/live/   # LiveKit live test (start tests/platforms/livekit first; variables at the top of tests/live/livekit_live_test.go)
go run ./cmd/jitsi-connect --domain meet.jitsi --muc muc.meet.jitsi --ws ws://localhost:8000/xmpp-websocket --room test --participants 2 --reports-dir /tmp/r   # every run writes <time>-<platform>-<name>-<run>.json/.md to ./reports (or --reports-dir / $REPORTS_DIR)
MEDIASOUP_URL=ws://localhost:4443 MEDIASOUP_ROOM=load010 MEDIASOUP_PARTICIPANTS=10 MEDIASOUP_SERVER_CONTAINERS=mediasoup-demo go test -tags live -count=1 -v -run Mediasoup ./tests/live/   # mediasoup-demo live test (start tests/platforms/mediasoup first; variables at the top of tests/live/mediasoup_live_test.go)
JITSI_APP_SECRET=... go run ./cmd/jitsi-connect --domain meet.example.org --room test --jwt-app-id my_app   # token-auth Jitsi: one signed JWT per bot (or --jwt / $JITSI_JWT for a ready-made token)
JANUS_URL=ws://localhost:8188 JANUS_ROOM=1011 JANUS_PARTICIPANTS=10 JANUS_SERVER_CONTAINERS=janus go test -tags live -count=1 -v -run Janus ./tests/live/   # Janus live test (start tests/platforms/janus first; variables at the top of tests/live/janus_live_test.go; a new JANUS_ROOM when the bot count grows)
```

## Code Style

: Follow standard conventions

## Recent Changes
- 011-janus-platform: Janus VideoRoom adapter (`src/platform/janus`), catalog entry (catalog `Entry.NewAdapter` now gets the test plan), local server built from tag v1.4.3 in `tests/platforms/janus`, live test `tests/live/janus_live_test.go`; media sources start RTP timestamps at 2^30 (`src/media/source.go`; 2^31 makes Jitsi's bridge rebase video)
- 010-mediasoup-platform: mediasoup-demo adapter (`src/platform/mediasoup`), catalog entry, local server built from a pinned commit in `tests/platforms/mediasoup`; RFC 7675 consent checks on every PeerConnection (`src/execution/consent.go`); no publish session for receive-only peers
- 009-run-report-files: JSON + Markdown report of every run in `reports/` (`src/report`), `RunResult` in the engine's completion callback, `report` in the scenario answer; no new module


<!-- MANUAL ADDITIONS START -->

Testing jitsi instance is : 

https://localhost:8443/IntriguingClimatesSupposeProbably

<!-- MANUAL ADDITIONS END -->
