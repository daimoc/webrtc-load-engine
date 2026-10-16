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
JITSI_APP_SECRET=... go run ./cmd/jitsi-connect --domain meet.example.org --room test --jwt-app-id my_app   # token-auth Jitsi: one signed JWT per bot (or --jwt / $JITSI_JWT for a ready-made token)
```

## Code Style

: Follow standard conventions

## Recent Changes
- 003-xmpp-signaling: Added Go 1.23.0 + `mellium.im/xmpp` (switched from `gosrc.io/xmpp` in 1aed684), `pion/webrtc/v3`
- 003-xmpp-signaling: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]
- 002-scenario-execution-engine: Added Go 1.23.0 + Pion, net/http, `google/uuid`


<!-- MANUAL ADDITIONS START -->

Testing jitsi instance is : 

https://localhost:8443/IntriguingClimatesSupposeProbably

<!-- MANUAL ADDITIONS END -->
