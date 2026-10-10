# webrtc-load-engine Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-01-06

## Active Technologies
- Go 1.23.0 + Pion, net/http, `google/uuid` (002-scenario-execution-engine)
- In-memory (002-scenario-execution-engine)
- Go 1.23.0 + `mellium.im/xmpp`, `nhooyr.io/websocket`, `pion/webrtc/v3` (003-xmpp-signaling)
- N/A (In-memory session state) (003-xmpp-signaling)

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
```

## Code Style

: Follow standard conventions

## Recent Changes
- 003-xmpp-signaling: Added Go 1.23.0 + `mellium.im/xmpp` (switched from `gosrc.io/xmpp` in 1aed684), `pion/webrtc/v3`
- 003-xmpp-signaling: Added [if applicable, e.g., PostgreSQL, CoreData, files or N/A]
- 002-scenario-execution-engine: Added Go 1.23.0 + Pion, net/http, `google/uuid`


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
