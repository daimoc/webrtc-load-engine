# Contract: `platform.SignalingHandler` additions

The media side (`execution.Peer`) and the platform adapters only exchange
SDP, ICE candidates, lifecycle events and, new here, media kinds
(constitution II and III). Two methods are added:

```go
// LocalMediaKinds returns the media kinds ("audio", "video") the peer sends,
// in a stable order. Adapters call it before joining so that they can
// announce the sources (for example in presence). Empty for receive-only.
LocalMediaKinds() []string

// Renegotiate applies an updated remote SDP offer on an already negotiated
// session (remote sources added or removed) and returns the SDP answer.
// It does not trickle candidates or record negotiation metrics. Adapters
// whose platform expects no reply may drop the answer.
Renegotiate(sdp string) (string, error)

// ReceiveLimit returns the maximum number of remote streams the peer
// receives per media kind (0: no limit). Adapters subscribe to (or offer)
// at most that many and replace an ended one by a waiting one.
ReceiveLimit() int
```

Behavior changes of existing methods:

- `OnOffer`: when the peer sends media, its answer is `sendrecv` for the
  offered kinds it sends and carries one `a=ssrc` source per sent kind (no
  RTX source: Pion v3 retransmits on the main SSRC). Offered kinds it
  does not send stay `recvonly`.
- Every remote media section of the offer leads to a received stream that the
  peer reads and accounts for, whatever the profile.

Adapter obligations (Jitsi):

- turn Jicofo's per-kind contents into one SDP section per remote source and
  back (research R5);
- announce the bot's sources the way the platform's clients do (R4);
- call `Renegotiate` on source changes, serialized per session (R6);
- offer at most `ReceiveLimit()` remote sources per kind, promoting waiting
  ones when active ones are removed (R12).

Contract tests: the fake XMPP server of 003 is extended to check the
session-accept sources and the presence `SourceInfo`, and to send
`source-add` / `source-remove`.
