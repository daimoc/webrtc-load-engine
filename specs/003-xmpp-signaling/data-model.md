# Data Model: XMPP Signaling

## Entities

### JingleSession
Represents a single media session negotiation state.

| Field | Type | Description |
|-------|------|-------------|
| ID | string | Unique Jingle Session ID (SID) |
| PeerJID | string | The JID of the remote peer (Focus/JVB) |
| State | Enum | `PENDING`, `ACTIVE`, `ENDED` |
| LocalSDP | SessionDescription | The local SDP (from Pion) |
| RemoteSDP | SessionDescription | The remote SDP (from Jingle) |

### XMPPAdapter
The Jitsi-specific implementation of the PlatformAdapter.

| Field | Type | Description |
|-------|------|-------------|
| Client | *xmpp.Client | The underlying XMPP connection |
| Room | string | The MUC room name |
| Sessions | Map<SID, Session> | Active Jingle sessions |
| State | Enum | `DISCONNECTED`, `CONNECTED`, `JOINED` |

## XML Structs (Jingle)

Mapping for `urn:xmpp:jingle:1` and related namespaces.

```go
type JingleIQ struct {
    XMLName xml.Name `xml:"urn:xmpp:jingle:1 jingle"`
    Action  string   `xml:"action,attr"`
    SID     string   `xml:"sid,attr"`
    Initiator string `xml:"initiator,attr,omitempty"`
    Responder string `xml:"responder,attr,omitempty"`
    Content []JingleContent `xml:"content"`
}

type JingleContent struct {
    Creator string `xml:"creator,attr"`
    Name    string `xml:"name,attr"`
    Description JingleDescription `xml:"description"`
    Transport   JingleTransport   `xml:"transport"`
}

// ... Additional structs for ICE-UDP and RTP-Description
```

## State Transitions

### Session Lifecycle
1. **Init**: State `PENDING`. Received `session-initiate`.
2. **Accept**: State `ACTIVE`. Sent `session-accept`.
3. **Terminate**: State `ENDED`. Sent/Received `session-terminate`.
