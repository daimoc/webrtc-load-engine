package livekit

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/encoding/protowire"
)

// A hand-written codec for the LiveKit signaling messages the adapter uses
// (specs/008-livekit-platform/contracts/signaling-messages.md, research R2).
// Field numbers are those of livekit/protocol v1.52.1
// (protobufs/livekit_rtc.proto, livekit_models.proto). Fields are written in
// field-number order and zero values are left out, as the official
// marshaller does; unknown fields and messages are skipped.

// Enum values used.
const (
	targetPublisher  int32 = 0 // SignalTarget.PUBLISHER
	targetSubscriber int32 = 1 // SignalTarget.SUBSCRIBER

	trackAudio int32 = 0 // TrackType.AUDIO
	trackVideo int32 = 1 // TrackType.VIDEO

	sourceCamera     int32 = 1 // TrackSource.CAMERA
	sourceMicrophone int32 = 2 // TrackSource.MICROPHONE

	stateJoined       int32 = 1 // ParticipantInfo.State.JOINED
	stateActive       int32 = 2 // ParticipantInfo.State.ACTIVE
	stateDisconnected int32 = 3 // ParticipantInfo.State.DISCONNECTED

	reasonClientInitiated int32 = 1 // DisconnectReason.CLIENT_INITIATED
	reasonServerShutdown  int32 = 3 // DisconnectReason.SERVER_SHUTDOWN

	actionReconnect int32 = 2 // LeaveRequest.Action.RECONNECT
)

// sessionDescription is SessionDescription: 1 type, 2 sdp, 3 id.
type sessionDescription struct {
	Type string
	SDP  string
	ID   uint32
}

// trickleRequest is TrickleRequest: 1 candidateInit, 2 target, 3 final.
type trickleRequest struct {
	CandidateInit string
	Target        int32
	Final         bool
}

// addTrackRequest is AddTrackRequest: 1 cid, 2 name, 3 type, 4 width,
// 5 height, 8 source, 13 disable_red.
type addTrackRequest struct {
	CID        string
	Name       string
	Type       int32
	Width      uint32
	Height     uint32
	Source     int32
	DisableRed bool
}

// updateSubscription is UpdateSubscription: 1 track_sids, 2 subscribe.
type updateSubscription struct {
	TrackSIDs []string
	Subscribe bool
}

// leaveRequest is LeaveRequest: 1 can_reconnect, 2 reason, 3 action.
type leaveRequest struct {
	CanReconnect bool
	Reason       int32
	Action       int32
}

// ping is Ping: 1 timestamp, 2 rtt.
type ping struct {
	Timestamp int64
	RTT       int64
}

// signalRequest is SignalRequest; exactly one field is set: 1 offer,
// 2 answer, 3 trickle, 4 add_track, 6 subscription, 8 leave, 16 ping_req.
type signalRequest struct {
	Offer        *sessionDescription
	Answer       *sessionDescription
	Trickle      *trickleRequest
	AddTrack     *addTrackRequest
	Subscription *updateSubscription
	Leave        *leaveRequest
	PingReq      *ping
}

// trackInfo is TrackInfo: 1 sid, 2 type, 4 muted, 9 source.
type trackInfo struct {
	SID    string
	Type   int32
	Muted  bool
	Source int32
}

// participantInfo is ParticipantInfo: 1 sid, 2 identity, 3 state, 4 tracks.
type participantInfo struct {
	SID      string
	Identity string
	State    int32
	Tracks   []trackInfo
}

// joinResponse is JoinResponse: 2 participant, 3 other_participants,
// 6 subscriber_primary, 7 alternative_url, 10 ping_timeout,
// 11 ping_interval, 12 server_info (ServerInfo: 2 version, 3 protocol).
type joinResponse struct {
	Participant       participantInfo
	OtherParticipants []participantInfo
	SubscriberPrimary bool
	AlternativeURL    string
	PingTimeout       int32 // Seconds
	PingInterval      int32 // Seconds
	ServerVersion     string
	ServerProtocol    int32
}

// participantUpdate is ParticipantUpdate: 1 participants.
type participantUpdate struct {
	Participants []participantInfo
}

// trackPublishedResponse is TrackPublishedResponse: 1 cid, 2 track.
type trackPublishedResponse struct {
	CID   string
	Track trackInfo
}

// subscriptionResponse is SubscriptionResponse: 1 track_sid, 2 err.
type subscriptionResponse struct {
	TrackSID string
	Err      int32
}

// signalResponse is the part of SignalResponse the adapter handles: 1 join,
// 2 answer, 3 offer, 4 trickle, 5 update, 6 track_published, 8 leave,
// 18 pong / 20 pong_resp (Pong), 21 subscription_response. Every other
// message leaves all fields zero.
type signalResponse struct {
	Join                 *joinResponse
	Answer               *sessionDescription
	Offer                *sessionDescription
	Trickle              *trickleRequest
	Update               *participantUpdate
	TrackPublished       *trackPublishedResponse
	Leave                *leaveRequest
	Pong                 bool
	SubscriptionResponse *subscriptionResponse
}

// Encoding.

func appendString(b []byte, num protowire.Number, v string) []byte {
	if v == "" {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendString(b, v)
}

func appendVarint(b []byte, num protowire.Number, v uint64) []byte {
	if v == 0 {
		return b
	}
	b = protowire.AppendTag(b, num, protowire.VarintType)
	return protowire.AppendVarint(b, v)
}

func appendBool(b []byte, num protowire.Number, v bool) []byte {
	if !v {
		return b
	}
	return appendVarint(b, num, 1)
}

// appendInt32 encodes an int32 or enum: negative values take ten bytes, as
// in protobuf.
func appendInt32(b []byte, num protowire.Number, v int32) []byte {
	return appendVarint(b, num, uint64(int64(v)))
}

// appendMessage writes a nested message (present even when empty).
func appendMessage(b []byte, num protowire.Number, m []byte) []byte {
	b = protowire.AppendTag(b, num, protowire.BytesType)
	return protowire.AppendBytes(b, m)
}

func (s *sessionDescription) encode() []byte {
	var b []byte
	b = appendString(b, 1, s.Type)
	b = appendString(b, 2, s.SDP)
	return appendVarint(b, 3, uint64(s.ID))
}

func (t *trickleRequest) encode() []byte {
	var b []byte
	b = appendString(b, 1, t.CandidateInit)
	b = appendInt32(b, 2, t.Target)
	return appendBool(b, 3, t.Final)
}

func (a *addTrackRequest) encode() []byte {
	var b []byte
	b = appendString(b, 1, a.CID)
	b = appendString(b, 2, a.Name)
	b = appendInt32(b, 3, a.Type)
	b = appendVarint(b, 4, uint64(a.Width))
	b = appendVarint(b, 5, uint64(a.Height))
	b = appendInt32(b, 8, a.Source)
	return appendBool(b, 13, a.DisableRed)
}

func (u *updateSubscription) encode() []byte {
	var b []byte
	for _, sid := range u.TrackSIDs {
		b = protowire.AppendTag(b, 1, protowire.BytesType)
		b = protowire.AppendString(b, sid)
	}
	return appendBool(b, 2, u.Subscribe)
}

func (l *leaveRequest) encode() []byte {
	var b []byte
	b = appendBool(b, 1, l.CanReconnect)
	b = appendInt32(b, 2, l.Reason)
	return appendInt32(b, 3, l.Action)
}

func (p *ping) encode() []byte {
	var b []byte
	b = appendVarint(b, 1, uint64(p.Timestamp))
	return appendVarint(b, 2, uint64(p.RTT))
}

// encodeSignalRequest encodes r, which must have exactly one field set.
func encodeSignalRequest(r signalRequest) ([]byte, error) {
	var (
		num protowire.Number
		msg []byte
		set int
	)
	pick := func(n protowire.Number, present bool, encode func() []byte) {
		if present {
			num, msg = n, encode()
			set++
		}
	}
	pick(1, r.Offer != nil, func() []byte { return r.Offer.encode() })
	pick(2, r.Answer != nil, func() []byte { return r.Answer.encode() })
	pick(3, r.Trickle != nil, func() []byte { return r.Trickle.encode() })
	pick(4, r.AddTrack != nil, func() []byte { return r.AddTrack.encode() })
	pick(6, r.Subscription != nil, func() []byte { return r.Subscription.encode() })
	pick(8, r.Leave != nil, func() []byte { return r.Leave.encode() })
	pick(16, r.PingReq != nil, func() []byte { return r.PingReq.encode() })
	if set != 1 {
		return nil, fmt.Errorf("signal request needs exactly one message, got %d", set)
	}
	return appendMessage(nil, num, msg), nil
}

// Decoding.

var errMalformed = errors.New("malformed protobuf")

// fields calls f for each field of the message b. f returns the bytes it
// consumed for the value, or 0 to have it skipped.
func fields(b []byte, f func(num protowire.Number, typ protowire.Type, v []byte) (int, error)) error {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return fmt.Errorf("%w: %v", errMalformed, protowire.ParseError(n))
		}
		b = b[n:]
		used, err := f(num, typ, b)
		if err != nil {
			return err
		}
		if used == 0 {
			used = protowire.ConsumeFieldValue(num, typ, b)
			if used < 0 {
				return fmt.Errorf("%w: field %d: %v", errMalformed, num, protowire.ParseError(used))
			}
		}
		b = b[used:]
	}
	return nil
}

// value helpers: each returns the decoded value and the bytes used, or an
// error when the wire type does not match or the value is cut.

func bytesValue(typ protowire.Type, b []byte) ([]byte, int, error) {
	if typ != protowire.BytesType {
		return nil, 0, fmt.Errorf("%w: expected bytes, got wire type %d", errMalformed, typ)
	}
	v, n := protowire.ConsumeBytes(b)
	if n < 0 {
		return nil, 0, fmt.Errorf("%w: %v", errMalformed, protowire.ParseError(n))
	}
	return v, n, nil
}

func varintValue(typ protowire.Type, b []byte) (uint64, int, error) {
	if typ != protowire.VarintType {
		return 0, 0, fmt.Errorf("%w: expected varint, got wire type %d", errMalformed, typ)
	}
	v, n := protowire.ConsumeVarint(b)
	if n < 0 {
		return 0, 0, fmt.Errorf("%w: %v", errMalformed, protowire.ParseError(n))
	}
	return v, n, nil
}

// decoder fills the fields of one message; each setter consumes a value.
type decoder map[protowire.Number]func(typ protowire.Type, b []byte) (int, error)

func (d decoder) decode(b []byte) error {
	return fields(b, func(num protowire.Number, typ protowire.Type, v []byte) (int, error) {
		if set, ok := d[num]; ok {
			return set(typ, v)
		}
		return 0, nil
	})
}

func str(dst *string) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := bytesValue(typ, b)
		*dst = string(v)
		return n, err
	}
}

func i32(dst *int32) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := varintValue(typ, b)
		*dst = int32(v)
		return n, err
	}
}

func u32(dst *uint32) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := varintValue(typ, b)
		*dst = uint32(v)
		return n, err
	}
}

func boolean(dst *bool) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := varintValue(typ, b)
		*dst = v != 0
		return n, err
	}
}

// message decodes a nested message with decode.
func message(decode func([]byte) error) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := bytesValue(typ, b)
		if err != nil {
			return 0, err
		}
		return n, decode(v)
	}
}

func (s *sessionDescription) decode(b []byte) error {
	return decoder{1: str(&s.Type), 2: str(&s.SDP), 3: u32(&s.ID)}.decode(b)
}

func (t *trickleRequest) decode(b []byte) error {
	return decoder{1: str(&t.CandidateInit), 2: i32(&t.Target), 3: boolean(&t.Final)}.decode(b)
}

func (t *trackInfo) decode(b []byte) error {
	return decoder{1: str(&t.SID), 2: i32(&t.Type), 4: boolean(&t.Muted), 9: i32(&t.Source)}.decode(b)
}

func (p *participantInfo) decode(b []byte) error {
	return decoder{
		1: str(&p.SID),
		2: str(&p.Identity),
		3: i32(&p.State),
		4: message(func(v []byte) error {
			var t trackInfo
			if err := t.decode(v); err != nil {
				return err
			}
			p.Tracks = append(p.Tracks, t)
			return nil
		}),
	}.decode(b)
}

func appendParticipant(dst *[]participantInfo) func(protowire.Type, []byte) (int, error) {
	return message(func(v []byte) error {
		var p participantInfo
		if err := p.decode(v); err != nil {
			return err
		}
		*dst = append(*dst, p)
		return nil
	})
}

func (j *joinResponse) decode(b []byte) error {
	return decoder{
		2:  message(j.Participant.decode),
		3:  appendParticipant(&j.OtherParticipants),
		6:  boolean(&j.SubscriberPrimary),
		7:  str(&j.AlternativeURL),
		10: i32(&j.PingTimeout),
		11: i32(&j.PingInterval),
		12: message(func(v []byte) error {
			return decoder{2: str(&j.ServerVersion), 3: i32(&j.ServerProtocol)}.decode(v)
		}),
	}.decode(b)
}

func (l *leaveRequest) decode(b []byte) error {
	return decoder{1: boolean(&l.CanReconnect), 2: i32(&l.Reason), 3: i32(&l.Action)}.decode(b)
}

// decodeSignalResponse decodes a SignalResponse frame. A message the adapter
// does not handle gives a zero signalResponse and no error.
func decodeSignalResponse(b []byte) (signalResponse, error) {
	var r signalResponse
	err := decoder{
		1: message(func(v []byte) error { r.Join = &joinResponse{}; return r.Join.decode(v) }),
		2: message(func(v []byte) error { r.Answer = &sessionDescription{}; return r.Answer.decode(v) }),
		3: message(func(v []byte) error { r.Offer = &sessionDescription{}; return r.Offer.decode(v) }),
		4: message(func(v []byte) error { r.Trickle = &trickleRequest{}; return r.Trickle.decode(v) }),
		5: message(func(v []byte) error {
			r.Update = &participantUpdate{}
			return decoder{1: appendParticipant(&r.Update.Participants)}.decode(v)
		}),
		6: message(func(v []byte) error {
			r.TrackPublished = &trackPublishedResponse{}
			return decoder{1: str(&r.TrackPublished.CID), 2: message(r.TrackPublished.Track.decode)}.decode(v)
		}),
		8: message(func(v []byte) error { r.Leave = &leaveRequest{}; return r.Leave.decode(v) }),
		18: func(typ protowire.Type, b []byte) (int, error) {
			_, n, err := varintValue(typ, b)
			r.Pong = err == nil
			return n, err
		},
		20: message(func([]byte) error { r.Pong = true; return nil }),
		21: message(func(v []byte) error {
			r.SubscriptionResponse = &subscriptionResponse{}
			return decoder{1: str(&r.SubscriptionResponse.TrackSID), 2: i32(&r.SubscriptionResponse.Err)}.decode(v)
		}),
	}.decode(b)
	if err != nil {
		return signalResponse{}, err
	}
	return r, nil
}
