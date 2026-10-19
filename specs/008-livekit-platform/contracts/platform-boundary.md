# Platform boundary changes (research R4, R10)

All additions are platform-neutral: they speak of a *publish session the peer
offers*, not of LiveKit. Jitsi's adapter does not call them and is unchanged.

## `platform.SignalingHandler`: new methods

```go
// CreatePublishOffer creates the publish session's offer: one sendonly
// section per local track. Candidates gathered before it returns are in
// the SDP; later ones go to trickle. Fails if the peer was not built with a
// publish session, or sends nothing.
CreatePublishOffer(trickle TrickleFunc) (string, error)
// ApplyPublishAnswer applies the remote answer to the publish offer.
ApplyPublishAnswer(sdp string) error
// OnPublishICECandidate applies a remote candidate for the publish session.
// Candidates received before the answer are buffered.
OnPublishICECandidate(mid, candidate string) error
// LocalVideoSize returns the width and height of the video the peer sends
// (0, 0 without video).
LocalVideoSize() (width, height int)
```

The existing `OnOffer`, `Renegotiate` and `OnICECandidate` keep their meaning
for the session the remote side offers. With a publish session, that session
carries no local track (recvonly).

## `platform.PublishSessionAdapter`: new optional interface

```go
// PublishSessionAdapter is implemented by adapters whose platform receives
// the peer's media on a session the peer offers, separate from the session
// the platform offers for the media the peer receives.
type PublishSessionAdapter interface {
	PublishSession() bool
}
```

The engine checks it once per scenario and sets `PeerMedia.PublishSession`.

## Behaviour required of `execution.Peer`

| | Without publish session (Jitsi) | With publish session (LiveKit) |
|---|---|---|
| Local tracks | on the answered session (sendrecv) | on the publish session (sendonly) |
| Answered session | sends and receives | receives only |
| `established` | answered session connected | every open session connected |
| `failed` (transport) | answered session failed | either session failed |
| `negotiation_seconds` | one observation | one per session |
| `transport_connect_seconds` | from answer | from first SDP handed out, to last session connected |
| Receive gate, TWCC feedback, quality | answered session | answered session (publish session receives no media) |
| `Close` | one PeerConnection | both |

## `scenario.PlatformCatalog`: new optional interface of the execution starter

```go
// PlatformCatalog validates test plans per platform at creation and names
// the platform_config keys to redact in answers.
type PlatformCatalog interface {
	Platforms() []string
	ValidateTestPlan(platform string, tp TestPlan) error
	SecretKeys(platform string) []string
	NewAdapter(platform string, tp TestPlan) (platform.PlatformAdapter, error)
}
```

`Service.CreateScenario` rejects platforms not in `Platforms()` (same message
as today: `unsupported platform adapter: %s`) and returns `ValidateTestPlan`'s
error as a 400. Snapshots returned to callers, and the creation log line,
replace the values of `SecretKeys` in `platform_config` with `"[redacted]"`.
`ExecuteScenario` passes `NewAdapter`'s adapter to `StartExecution` (whose
signature is unchanged). Without a catalog (unit tests with a mock starter),
today's `jitsi`-only behaviour stays.
