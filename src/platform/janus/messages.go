package janus

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Message is a Janus API message as the server sends it
// (specs/011-janus-platform/contracts/signaling-messages.md). IDs are
// integers up to about 2^52, so they are decoded as uint64.
type Message struct {
	Janus       string      `json:"janus"`
	Transaction string      `json:"transaction,omitempty"`
	SessionID   uint64      `json:"session_id,omitempty"`
	Sender      uint64      `json:"sender,omitempty"`
	Data        *IDData     `json:"data,omitempty"`  // create and attach
	Error       *CoreError  `json:"error,omitempty"` // janus: error
	PluginData  *PluginData `json:"plugindata,omitempty"`
	JSEP        *JSEP       `json:"jsep,omitempty"`
	Reason      string      `json:"reason,omitempty"`    // hangup
	Candidate   *Candidate  `json:"candidate,omitempty"` // trickle (full-trickle servers)
	// media events
	MID       string `json:"mid,omitempty"`
	Type      string `json:"type,omitempty"`
	Receiving *bool  `json:"receiving,omitempty"`
}

// Event returns the VideoRoom data of a plugin message, or nil.
func (m *Message) Event() *RoomEvent {
	if m.PluginData == nil {
		return nil
	}
	return &m.PluginData.Data
}

// IDData is the data of a create or attach reply.
type IDData struct {
	ID uint64 `json:"id"`
}

// CoreError is the error of a janus: error reply.
type CoreError struct {
	Code   int    `json:"code"`
	Reason string `json:"reason"`
}

// PluginData carries a plugin's reply or event.
type PluginData struct {
	Plugin string    `json:"plugin"`
	Data   RoomEvent `json:"data"`
}

// JSEP is an SDP offer or answer.
type JSEP struct {
	Type string `json:"type"`
	SDP  string `json:"sdp"`
}

// Candidate is a trickled ICE candidate, or the end of candidates.
type Candidate struct {
	SDPMid        string `json:"sdpMid,omitempty"`
	SDPMLineIndex *int   `json:"sdpMLineIndex,omitempty"`
	Candidate     string `json:"candidate,omitempty"`
	Completed     bool   `json:"completed,omitempty"`
}

// RoomEvent is the VideoRoom plugin's data (videoroom: joined, attached,
// updated, updating, event, created, destroyed...).
type RoomEvent struct {
	VideoRoom   string      `json:"videoroom"`
	Room        uint64      `json:"room,omitempty"`
	ID          uint64      `json:"id,omitempty"`
	PrivateID   uint64      `json:"private_id,omitempty"`
	Publishers  []Publisher `json:"publishers,omitempty"`
	Streams     []SubStream `json:"streams,omitempty"` // attached, updated
	Unpublished *IDOrOK     `json:"unpublished,omitempty"`
	Leaving     *IDOrOK     `json:"leaving,omitempty"`
	Kicked      uint64      `json:"kicked,omitempty"`
	Configured  string      `json:"configured,omitempty"`
	Started     string      `json:"started,omitempty"`
	ErrorCode   int         `json:"error_code,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// Publisher is an active publisher of a room.
type Publisher struct {
	ID      uint64            `json:"id"`
	Display string            `json:"display,omitempty"`
	Dummy   bool              `json:"dummy,omitempty"`
	Streams []PublisherStream `json:"streams,omitempty"`
}

// PublisherStream is one stream a publisher sends.
type PublisherStream struct {
	Type     string `json:"type"` // audio, video or data
	MID      string `json:"mid"`
	Disabled bool   `json:"disabled,omitempty"`
	Codec    string `json:"codec,omitempty"`
}

// SubStream is one stream of a subscription.
type SubStream struct {
	MID     string `json:"mid"`
	Type    string `json:"type"`
	FeedID  uint64 `json:"feed_id,omitempty"`
	FeedMID string `json:"feed_mid,omitempty"`
	Active  bool   `json:"active,omitempty"`
}

// IDOrOK is the value of unpublished and leaving: a participant ID in
// notifications about others, the string "ok" in replies about oneself.
type IDOrOK struct {
	ID uint64
	OK bool
}

// UnmarshalJSON reads a number or the string "ok".
func (v *IDOrOK) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		if s != "ok" {
			return fmt.Errorf("janus: unexpected value %q", s)
		}
		v.OK = true
		return nil
	}
	return json.Unmarshal(b, &v.ID)
}

// MarshalJSON writes the number or "ok" (for the fake server in tests).
func (v IDOrOK) MarshalJSON() ([]byte, error) {
	if v.OK {
		return []byte(`"ok"`), nil
	}
	return json.Marshal(v.ID)
}
