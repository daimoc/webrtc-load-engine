# Janus messages the adapter sends and handles (011)

Janus v1.4.3, WebSocket subprotocol `janus-protocol`, plugin
`janus.plugin.videoroom`.

Every request has `"transaction": "<random>"`, plus `"apisecret"` and
`"token"` when configured. Handle requests also have `session_id` and
`handle_id`; session requests have `session_id`. Golden copies of each
reply are recorded in T002 (`src/platform/janus/testdata/golden/`).

## Core

| Sent | Reply | Notes |
|---|---|---|
| `{"janus":"create"}` | `success`, `data.id` = session ID | |
| `{"janus":"attach","plugin":"janus.plugin.videoroom"}` | `success`, `data.id` = handle ID | twice: publisher, subscriber |
| `{"janus":"keepalive"}` | `ack` | every 25 s (R7) |
| `{"janus":"trickle","candidate":{"sdpMid","sdpMLineIndex","candidate"}}` | `ack` | on the handle of the candidate's session; then `{"completed":true}` |
| `{"janus":"message","body":{...},"jsep":{...}?}` | `ack`, then `event` with the same transaction | asynchronous plugin requests |
| `{"janus":"message","body":{...}}` | `success` with `plugindata.data` | synchronous plugin requests (`create`) |
| `{"janus":"destroy"}` | `success` | `Disconnect` (R7) |

Core error: `{"janus":"error","error":{"code":N,"reason":"..."}}`. For example
403 is an unauthorized request (bad API secret or token), and 458 is an
unknown session.

## VideoRoom requests (`body`)

| Request | When | Expected reply |
|---|---|---|
| `{"request":"create","room":R,"permanent":false,"publishers":P+1,"audiocodec":"opus","videocodec":"vp8","bitrate":0,"dummy_publisher":true,"dummy_streams":[{"codec":"opus"}],"pin"?,"admin_key"?}` | step 2 | `videoroom: "created"`; error 427 = exists, treated as success |
| `{"request":"join","ptype":"publisher","room":R,"display":"<peer ID>","pin"?}` | step 3 | `videoroom: "joined"`: `id`, `private_id`, `publishers[]` (each `id`, `display`, `dummy`, `streams[]` with `type`, `mid`, `disabled`) |
| `{"request":"publish","audiocodec":"opus","videocodec":"vp8"}` + `jsep` offer | step 4, bots that send | `event` `configured: "ok"` + `jsep` answer |
| `{"request":"join","ptype":"subscriber","room":R,"private_id":X,"use_msid":false,"autoupdate":false,"streams":[{"feed":D}],"pin"?}` | step 5 | `videoroom: "attached"`, `streams[]` + `jsep` offer |
| `{"request":"start"}` + `jsep` answer | after each subscriber offer | `event` `started: "ok"` (first time) or nothing more |
| `{"request":"update","subscribe":[{"feed","mid"}],"unsubscribe":[{"feed","mid"}]}` | cap changes (R6); one list may be absent | `updated` + `jsep` offer, or `updating` (queued), or `updated` without `streams` (no SDP change) |

VideoRoom error: `{"videoroom":"event","error_code":N,"error":"..."}`.

| Code | Meaning | Adapter |
|---|---|---|
| 426 | no such room | join fails |
| 427 | room exists | success on `create` |
| 432 | publishers full | join fails |
| 433 | unauthorized (PIN, admin key) | join fails |
| 428 | no such feed | logged; the stream is dropped from the cap state |
| others | | the request fails with the code and reason |

## Unsolicited messages (no transaction, routed by `sender`)

| Message | Handle | Adapter |
|---|---|---|
| `event` `publishers: [...]` | publisher | announce to the cap (R6) |
| `event` `unpublished: <id>` | publisher | feed gone (R6) |
| `event` `leaving: <id>` | publisher | feed gone; `leaving: "ok"` is our own leave |
| `event` `kicked: <id>` (own id) | publisher | lost |
| `event` `videoroom: "destroyed"` | either | lost |
| `event` `updated` + `jsep` | subscriber | `Renegotiate`, then `start` |
| `webrtcup`, `media`, `slowlink` | either | logged at debug |
| `hangup`, `detached` | either | lost (after `Connect`) |
| `timeout` | session | lost |
| `trickle` (full-trickle servers only) | either | `OnICECandidate` / `OnPublishICECandidate` |
