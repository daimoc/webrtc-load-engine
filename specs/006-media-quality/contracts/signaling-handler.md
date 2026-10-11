# Contract: Signaling handler changes (006)

Changes to `platform.SignalingHandler` (`src/platform/adapter.go`), after
[004](../../004-synthetic-media/contracts/signaling-handler.md) and
[005](../../005-bridge-channel-subscriptions/contracts/signaling-handler.md).

## New method

```go
// RequestedVideoHeight returns the video height, in lines, the peer wants
// to receive for each remote video stream. Adapters ask the media server
// for it with their platform's mechanism; the server may send less.
RequestedVideoHeight() int
```

- Implemented by `execution.Peer` from the test plan (default: highest
  height sent, 360 without simulcast; research R7).
- Jitsi: `maxHeight` of every received source in `ReceiverVideoConstraints`,
  and of `defaultConstraints` in the no-cap form (005
  [colibri-messages.md](../../005-bridge-channel-subscriptions/contracts/colibri-messages.md));
  replaces the constant 360. Adapter test: the message carries the handler's
  height.
- Test fakes (`pionHandler`, engine test adapters) return 360.

## Answer SDP with simulcast

When the peer sends several video layers, the answer returned by `OnOffer`
and `Renegotiate` lists every layer's SSRC in the video section and an
`a=ssrc-group:SIM <low> <mid> <high>` line (research R6). This is the SDP
browsers produce for SSRC simulcast; adapters translate it like any other
SSRC group (Jitsi: `ssrc-group` with `semantics="SIM"` in Jingle, all
sources named with the bot's video source name, 003). Without simulcast the
answer is unchanged.
