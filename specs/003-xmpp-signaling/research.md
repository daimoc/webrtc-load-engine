# Research: XMPP Signaling for Jitsi

## Technical Approach

### 1. XMPP Library Selection
**Decision**: Use `gosrc.io/xmpp` (Fluux).
**Rationale**: 
- It is the modern, actively maintained successor to generic Go XMPP libraries.
- It provides good support for Component and Client modes.
- It allows low-level stanza manipulation which is necessary for Jingle (since no library has built-in high-level Jingle support).
**Alternatives**:
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
- Use `gosrc.io/xmpp`'s router/handler mechanism to intercept `iq` stanzas of type `set` with `jingle` payload.
- Map the Jingle XML to `pion/webrtc` SessionDescription objects.

### 4. Authentication
**Decision**: Support **Anonymous SASL** by default.
**Rationale**: Standard for public Jitsi instances and easiest for load testing without pre-provisioning accounts.

### 5. Open Questions (Resolved)
- *Does Jitsi require extensions?* Yes, `colibri` namespace is used for bridge communication by Jicofo, but the client mainly speaks standard Jingle + ICE-UDP to Jicofo.
- *Who initiates?* Jicofo initiates.

## Dependencies

- `gosrc.io/xmpp`: Main signaling.
- `pion/webrtc/v3`: For generating the local SDP and handling ICE (already in project).
- `encoding/xml`: Standard Go lib for stanza marshaling.
