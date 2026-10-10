# Contract: SignalingHandler data channel (change)

`src/platform/adapter.go`. Replaces the 003 method
`OpenDataChannel(label, protocol string) error`. Platform-neutral: no
message format or platform concept crosses it (constitution III, FR-010).

```go
// DataChannel is a data channel opened on the media session.
type DataChannel interface {
	// SendText sends a text message. It fails while the channel is not
	// open; the caller retries on its next OnOpen.
	SendText(msg string) error
}

// DataChannelHandler receives a data channel's events. Methods are called
// from the media stack's goroutines, one at a time per channel, and must not
// block.
type DataChannelHandler interface {
	OnOpen()                // open, or open again
	OnMessage(text string)  // text message (binary messages are dropped)
	OnClose()
}

type SignalingHandler interface {
	// ...
	// OpenDataChannel opens a data channel on the negotiated session (after
	// OnOffer), e.g. the channel a platform's media server expects for its
	// control messages. Events go to h; the returned channel sends.
	OpenDataChannel(label, protocol string, h DataChannelHandler) (DataChannel, error)
	// ...
}
```

## Implementation in `execution.Peer`

- Wraps Pion's `*webrtc.DataChannel`: `SendText` → `dc.SendText`; Pion's
  `OnOpen`, `OnMessage` (text only), `OnClose` → `h`.
- Keeps the incoming message count and its debug log (003 behavior).
- Maintains `webrtc_data_channels_open{scenario_id}`: +1 on open, −1 on close
  or when the peer closes with the channel still open (never counted twice).

## Contract tests

- Peer pair in process: `OpenDataChannel` on one side, the other side
  accepts; `OnOpen` fires, `SendText` reaches the other side, a text from the
  other side reaches `OnMessage`, closing fires `OnClose`; the gauge goes
  0 → 1 → 0.
- `SendText` before open returns an error and does not panic.
- Jitsi fake server: the adapter passes a handler and sends the messages of
  [colibri-messages.md](colibri-messages.md) through a fake `DataChannel`
  (no real SCTP needed).
