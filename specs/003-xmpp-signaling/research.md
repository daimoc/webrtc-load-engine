# Research: XMPP Signaling for Jitsi

## Technical Approach

### 1. XMPP Library Selection
**Decision**: Use `mellium.im/xmpp` (revised in commit 1aed684; the original choice was `gosrc.io/xmpp`).
**Rationale**: 
- Token-stream (`xmlstream`) design gives low-level stanza control, which Jingle needs (no Go library has high-level Jingle support).
- Built-in SASL ANONYMOUS and a WebSocket negotiator (`mellium.im/xmpp/websocket`), both required by Jitsi guest access.
- `mux` package routes IQ/presence by type and payload namespace.
**Alternatives**:
- `gosrc.io/xmpp` (Fluux): initial choice; SASL ANONYMOUS over WebSocket was not workable in v0.5.x, which is why it was replaced.
- `mattn/go-xmpp`: Older, less flexible for custom stanza handling.
- `beevik/etree`: Just an XML parser, would require writing all XMPP logic from scratch.

### 2. Jingle Signaling Flow
**Context**: Jitsi Meet uses a specific signaling flow where the conference focus (Jicofo) allocates the bridge and initiates the session to the participant.
**Flow Decision**: The "bot" will act as a **Responder**.
1.  **Join MUC**: Send presence to `room@conference.domain`.
2.  **Wait for Offer**: Receive `session-initiate` IQ from Jicofo (Focus JID).
3.  **Process Offer**: Extract JVB ICE candidates and ufrag/pwd.
4.  **Send Answer**: Send `session-accept` IQ with local ICE candidates (gathered from Pion).
5.  **Transport Info**: Exchange `transport-info` for trickle ICE (if enabled) or include in accept.

### 3. Jingle Implementation
**Approach**: 
- Define Go structs for Jingle XML namespaces (`urn:xmpp:jingle:1`, `urn:xmpp:jingle:transports:ice-udp:1`, `urn:xmpp:jingle:apps:rtp:1`).
- Use `mellium.im/xmpp/mux` to intercept `iq` stanzas of type `set` with a `jingle` payload.
- Map the Jingle XML to `pion/webrtc` SessionDescription objects.

### 4. Authentication
**Decision**: Support **Anonymous SASL** by default.
**Rationale**: Standard for public Jitsi instances and easiest for load testing without pre-provisioning accounts.

### 5. Open Questions (Resolved)
- *Does Jitsi require extensions?* Yes, `colibri` namespace is used for bridge communication by Jicofo, but the client mainly speaks standard Jingle + ICE-UDP to Jicofo.
- *Who initiates?* Jicofo initiates.

## Source signaling format (T055/T056, 2026-10-10)

Jicofo sends remote sources either as XML (`<source>`/`<ssrc-group>` in the
RTP descriptions, XEP-0339) or, to clients advertising
`http://jitsi.org/json-encoded-sources`, as a compact JSON document in
`<json-message xmlns="http://jitsi.org/jitmeet">`. In Jicofo 1.0-1211 the
choice also affects the contents: on the XML path the offer's contents are
rebuilt with audio and video only, so the `data` (SCTP) content, and with
it the bridge channel, is only offered to JSON clients (verified on the
local deployment: a bot advertising the feature receives the `data` content
with its `sctpmap` and opens the channel). The adapter therefore advertises
JSON sources and converts them to the same internal Jingle sources as XML.

## Dependencies

- `mellium.im/xmpp` + `nhooyr.io/websocket`: Main signaling.
- `pion/webrtc/v3`: For generating the local SDP and handling ICE (already in project).
- `encoding/xml`: Standard Go lib for stanza marshaling.
