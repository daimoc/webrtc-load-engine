# Quickstart: Run Report Files

Validation guide for feature 009. Go at `/usr/local/go/bin`
(`export PATH=/usr/local/go/bin:$PATH`). Live steps use the local Jitsi of
`tests/platforms/docker-jitsi-meet` (`./setup.sh && docker compose up -d`)
with LiveKit stopped, or the reverse for step 6.

## 1. Unit and integration tests

```bash
go vet ./... && go test -race ./...
```

Covers: file names and no overwrite (R3), JSON = API summary (SC-002), Markdown
key figures = JSON values (SC-003), "not measured" lines (FR-009), secrets
masked (SC-006), unwritable folder (SC-005), report status in
`GET /api/v1/scenarios/{id}` ([contracts/api-scenario-report.yaml](contracts/api-scenario-report.yaml)),
shutdown writes the report of a running scenario.

## 2. HTTP API (US1, SC-001, SC-002)

The API server cannot run next to docker-jitsi-meet (both use port 8080,
007), so run this step against LiveKit (`tests/platforms/livekit`), with
`plan.json` the LiveKit test plan of its README set to 2 participants and
30 s.

```bash
go run ./src --reports-dir /tmp/r009 &
curl -s -XPOST localhost:8080/api/v1/scenarios -d @plan.json
curl -s -XPOST localhost:8080/api/v1/scenarios/<id>/_execute
sleep 35; curl -s localhost:8080/api/v1/scenarios/<id>                 # report.json.path, report.markdown.path
diff <(jq -S .quality "$(curl -s localhost:8080/api/v1/scenarios/<id> | jq -r .report.json.path)") \
     <(curl -s localhost:8080/api/v1/scenarios/<id>/quality | jq -S .)  # no output
```

Expected: two files in `/tmp/r009` within 2 s of the end; `diff` prints
nothing.

## 3. Command-line tool (US1, US2, FR-015)

```bash
go run ./cmd/jitsi-connect --domain meet.jitsi --muc muc.meet.jitsi \
  --ws ws://localhost:8000/xmpp-websocket --room r009 --participants 10 \
  --media audio_video --duration 60 \
  --server-containers docker-jitsi-meet-jvb-1,docker-jitsi-meet-jicofo-1
ls reports/
```

Expected: the last log line gives both paths; `reports/` holds
`<time>-jitsi-jitsi-connect-<run>.json` and `.md`. The Markdown starts with
the key figures ([contracts/report-markdown.md](contracts/report-markdown.md));
each figure equals the JSON value (SC-003). Ctrl+C before the end: the run
is `stopped` and both files are still written.

## 4. Live test and default folder (US3)

```bash
JITSI_ROOM_URL=http://localhost:8000/r009 JITSI_XMPP_DOMAIN=meet.jitsi \
JITSI_MUC_DOMAIN=muc.meet.jitsi JITSI_WS_URL=ws://localhost:8000/xmpp-websocket \
JITSI_PARTICIPANTS=2 go test -tags live -count=1 -v -run Jitsi ./tests/live/
ls reports/ tests/live/reports 2>&1
```

Expected: the new report is in the repository's `reports/`, and
`tests/live/reports` does not exist. With `REPORTS_DIR=/tmp/r009-live` it is
written there instead.

## 5. Failures (SC-005, SC-006)

```bash
mkdir -p /tmp/ro && chmod 555 /tmp/ro
go run ./cmd/jitsi-connect ... --duration 10 --reports-dir /tmp/ro/reports
```

Expected: the run ends `completed` in the same time as step 3's shape; one
warning per file with `permission denied`. With `--jwt-app-id my_app` and
`JITSI_APP_SECRET=s3cr3t-009` against a token-auth Jitsi:
`grep -r s3cr3t-009 reports/` finds nothing.

## 6. LiveKit and the 100-bot run (FR-011, SC-008)

Run the LiveKit live test of `tests/platforms/livekit/README.md` with
`LIVEKIT_PARTICIPANTS=100 LIVEKIT_HOLD=600s`: one pair of files named
`…-livekit-…`, same sections as Jitsi's; reports written within 2 s of the
end; CPU during the run unchanged from 008's validation report.

## 7. Uniqueness (SC-004)

Covered by a unit test writing 50 reports with the same end time, platform,
name and run ID into one folder (so the names collide and the `-2`, `-3`…
suffixes of R3 are exercised): 50 distinct pairs, none overwritten.
