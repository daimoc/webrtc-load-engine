# protoo messages used by the adapter (research R1, R4, R6, R7)

Source: `server/src/signaling/protooMessages.ts` and `server/src/Peer.ts`
at mediasoup-demo `899109e`. Fields the adapter ignores are not listed.

## Connection

`GET <url>/?roomId=<room>&peerId=<peer ID>[&unlimitedRoomSecret=<s>][&usePipeTransports=true]`,
WebSocket subprotocol `protoo`, header `Origin: <origin setting, or http(s)://<url host>>`. The URL with its secret is never logged:
the log shows the URL without query.

- Refused handshakes: 403 without the subprotocol or with an `Origin` not matching the server's domain; 400 for a missing room or peer ID; 403 or 500 for a
  room blocked after its time limit, or one that cannot be created (the
  join fails with the status and body).
- The server closes a peer that has not sent `join` within 10 s.

## Framing

```json
{"request":true,"id":12,"method":"join","data":{}}
{"response":true,"id":12,"ok":true,"data":{}}
{"response":true,"id":12,"errorCode":500,"errorReason":"Peer already joined"}
{"notification":true,"method":"consumerClosed","data":{"consumerId":"…"}}
```

A response without `"ok": true` is an error (the server omits `ok` on errors).
Request IDs are unique per connection: random start, then incremented.
Every request has a timeout of 15 s; a timeout fails the join, or, after
the join, it is logged and counted as a signaling error.

## Bot → server requests

| Method | data | response data | When |
|---|---|---|---|
| `getRouterRtpCapabilities` | – | `routerRtpCapabilities` | join step 2 |
| `createWebRtcTransport` | `{forceTcp:false, appData:{direction:"consumer"\|"producer"}}` | `transportId, iceParameters{usernameFragment,password,iceLite}, iceCandidates[{foundation,priority,address,protocol,port,type}], dtlsParameters{role,fingerprints[{algorithm,value}]}, sctpParameters{port,OS,MIS,maxMessageSize}` | join steps 3–4 |
| `connectWebRtcTransport` | `{transportId, dtlsParameters:{role, fingerprints:[{algorithm:"sha-256",value}]}}`; role from the bot's SDP: `server` on the receiving transport (Pion answers `passive`), `client` on the sending one | – | after the bot's first SDP of each transport |
| `join` | `{displayName:<peer ID>, device:{flag:"unknown",name:"webrtc-load-engine",version}, rtpCapabilities}` | `peers[]` (logged as a count) | join step 5 |
| `produce` | `{transportId, kind, rtpParameters, appData:{source:"mic"\|"webcam"}}` | `producerId` | join step 6, per kind |

## Bot → server notifications

| Method | data | When |
|---|---|---|
| `resumeConsumer` | `{consumerId}` | a waiting consumer is promoted (R6), after its section was negotiated |

## Server → bot requests (must be answered)

| Method | data used | Answer |
|---|---|---|
| `newConsumer` | `peerId, consumerId, producerId, kind, rtpParameters, type, producerPaused` | `ok:true` once its section is negotiated if a slot is free; else `ok:false, errorCode:403, errorReason:"receive cap"`, and it is kept as waiting |
| `newDataConsumer` | – | `ok:false, errorCode:403, errorReason:"data channels not used"` |
| any other | – | `ok:false, errorCode:405` (logged once per method) |

## Server → bot notifications

| Method | Effect |
|---|---|
| `consumerClosed {consumerId}` | active: section inactive and promote the next waiting; waiting: dropped |
| `peerClosed {peerId}` | logged at debug; its consumers are closed by `consumerClosed` |
| `mediasoupVersion {version}` | logged once per run (in the report's log context) |
| `newPeer`, `producerScore`, `consumerScore`, `consumerPaused`, `consumerResumed`, `consumerLayersChanged`, `activeSpeaker`, `speakingPeers`, `peerDisplayNameChanged`, `dataConsumerClosed` | ignored |
