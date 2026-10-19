# Reports folder configuration (feature 009, FR-003, research R8)

| Entry point | Setting | Default |
|-------------|---------|---------|
| HTTP API (`go run ./src`) | `--reports-dir <dir>` | `$REPORTS_DIR`, else `reports` in the working directory |
| `cmd/jitsi-connect` | `--reports-dir <dir>` | `$REPORTS_DIR`, else `reports` in the working directory |
| Live tests (`go test -tags live ./tests/live/`) | `REPORTS_DIR=<dir>` | `reports` in the module root (folder of `go.mod`) |

- A relative folder is resolved against the working directory when the
  process starts, so the paths in the API and in the logs are absolute.
- The folder is created if missing, at each write.
- The CLI and the live tests print, at the end of the run:
  `report written json=<path> markdown=<path>` (one log line), or one
  warning per file not written, with the reason (FR-015).
- `/reports/` is ignored by git (research R11).
