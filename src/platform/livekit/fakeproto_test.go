package livekit

import (
	"bytes"
	"reflect"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

// The fake server's side of the codec: it encodes SignalResponses and
// decodes SignalRequests, with the same field numbers as proto.go.

func (t *trackInfo) encode() []byte {
	var b []byte
	b = appendString(b, 1, t.SID)
	b = appendInt32(b, 2, t.Type)
	b = appendBool(b, 4, t.Muted)
	return appendInt32(b, 9, t.Source)
}

func (p *participantInfo) encode() []byte {
	var b []byte
	b = appendString(b, 1, p.SID)
	b = appendString(b, 2, p.Identity)
	b = appendInt32(b, 3, p.State)
	for i := range p.Tracks {
		b = appendMessage(b, 4, p.Tracks[i].encode())
	}
	return b
}

func (j *joinResponse) encode() []byte {
	var b []byte
	b = appendMessage(b, 2, j.Participant.encode())
	for i := range j.OtherParticipants {
		b = appendMessage(b, 3, j.OtherParticipants[i].encode())
	}
	b = appendBool(b, 6, j.SubscriberPrimary)
	b = appendString(b, 7, j.AlternativeURL)
	b = appendInt32(b, 10, j.PingTimeout)
	b = appendInt32(b, 11, j.PingInterval)
	var info []byte
	info = appendString(info, 2, j.ServerVersion)
	info = appendInt32(info, 3, j.ServerProtocol)
	return appendMessage(b, 12, info)
}

func encodeSignalResponse(r signalResponse) []byte {
	switch {
	case r.Join != nil:
		return appendMessage(nil, 1, r.Join.encode())
	case r.Answer != nil:
		return appendMessage(nil, 2, r.Answer.encode())
	case r.Offer != nil:
		return appendMessage(nil, 3, r.Offer.encode())
	case r.Trickle != nil:
		return appendMessage(nil, 4, r.Trickle.encode())
	case r.Update != nil:
		var b []byte
		for i := range r.Update.Participants {
			b = appendMessage(b, 1, r.Update.Participants[i].encode())
		}
		return appendMessage(nil, 5, b)
	case r.TrackPublished != nil:
		var b []byte
		b = appendString(b, 1, r.TrackPublished.CID)
		b = appendMessage(b, 2, r.TrackPublished.Track.encode())
		return appendMessage(nil, 6, b)
	case r.Leave != nil:
		return appendMessage(nil, 8, r.Leave.encode())
	case r.Pong:
		return appendMessage(nil, 20, nil)
	case r.SubscriptionResponse != nil:
		var b []byte
		b = appendString(b, 1, r.SubscriptionResponse.TrackSID)
		b = appendInt32(b, 2, r.SubscriptionResponse.Err)
		return appendMessage(nil, 21, b)
	}
	return nil
}

func i64(dst *int64) func(protowire.Type, []byte) (int, error) {
	return func(typ protowire.Type, b []byte) (int, error) {
		v, n, err := varintValue(typ, b)
		*dst = int64(v)
		return n, err
	}
}

func decodeSignalRequest(b []byte) (signalRequest, error) {
	var r signalRequest
	err := decoder{
		1: message(func(v []byte) error { r.Offer = &sessionDescription{}; return r.Offer.decode(v) }),
		2: message(func(v []byte) error { r.Answer = &sessionDescription{}; return r.Answer.decode(v) }),
		3: message(func(v []byte) error { r.Trickle = &trickleRequest{}; return r.Trickle.decode(v) }),
		4: message(func(v []byte) error {
			r.AddTrack = &addTrackRequest{}
			a := r.AddTrack
			return decoder{1: str(&a.CID), 2: str(&a.Name), 3: i32(&a.Type), 4: u32(&a.Width), 5: u32(&a.Height), 8: i32(&a.Source), 13: boolean(&a.DisableRed)}.decode(v)
		}),
		6: message(func(v []byte) error {
			r.Subscription = &updateSubscription{}
			u := r.Subscription
			return decoder{
				1: func(typ protowire.Type, b []byte) (int, error) {
					s, n, err := bytesValue(typ, b)
					u.TrackSIDs = append(u.TrackSIDs, string(s))
					return n, err
				},
				2: boolean(&u.Subscribe),
			}.decode(v)
		}),
		8: message(func(v []byte) error { r.Leave = &leaveRequest{}; return r.Leave.decode(v) }),
		16: message(func(v []byte) error {
			r.PingReq = &ping{}
			return decoder{1: i64(&r.PingReq.Timestamp), 2: i64(&r.PingReq.RTT)}.decode(v)
		}),
	}.decode(b)
	return r, err
}

// The fake's codec must agree with the official encoding too.
func TestFakeCodec_RequestsDecodeGolden(t *testing.T) {
	for _, name := range []string{"req_offer", "req_answer", "req_trickle_pub", "req_trickle_sub", "req_add_track_audio", "req_add_track_video", "req_subscription", "req_leave", "req_ping"} {
		b := golden(t, name)
		r, err := decodeSignalRequest(b)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		again, err := encodeSignalRequest(r)
		if err != nil || !bytes.Equal(again, b) {
			t.Errorf("%s: decode+encode differs (%v)", name, err)
		}
	}
}

func TestFakeCodec_ResponsesRoundTrip(t *testing.T) {
	for i, r := range []signalResponse{
		{Join: &joinResponse{Participant: participantInfo{SID: "PA_1", Identity: "b1", State: stateJoined}, OtherParticipants: []participantInfo{{SID: "PA_2", Identity: "b2", State: stateActive, Tracks: tracksOf("2")}}, SubscriberPrimary: true, PingTimeout: 3, PingInterval: 1, ServerVersion: "fake", ServerProtocol: 17}},
		{Offer: &sessionDescription{Type: "offer", SDP: "v=0", ID: 4}},
		{Update: &participantUpdate{Participants: []participantInfo{{SID: "PA_2", State: stateDisconnected}}}},
		{TrackPublished: &trackPublishedResponse{CID: "audio", Track: trackInfo{SID: "TR_1", Type: trackAudio, Source: sourceMicrophone}}},
		{Leave: &leaveRequest{Reason: reasonServerShutdown}},
		{Pong: true},
		{SubscriptionResponse: &subscriptionResponse{TrackSID: "TR_x", Err: 2}},
	} {
		got, err := decodeSignalResponse(encodeSignalResponse(r))
		if err != nil || !reflect.DeepEqual(got, r) {
			t.Errorf("case %d: got %+v, %v; want %+v", i, got, err, r)
		}
	}
}
