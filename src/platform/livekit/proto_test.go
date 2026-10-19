package livekit

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Golden frames: marshalled with the official livekit/protocol v1.52.1
// types (testdata/golden/README.md shows each message).

func golden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name+".bin"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const goldenCandidate = `{"candidate":"candidate:1 1 udp 2130706431 192.168.1.65 50000 typ host","sdpMid":"0","sdpMLineIndex":0}`

func TestEncodeSignalRequest_Golden(t *testing.T) {
	for name, req := range map[string]signalRequest{
		"req_offer":           {Offer: &sessionDescription{Type: "offer", SDP: "v=0\r\no=- 1 2 IN IP4 127.0.0.1\r\n", ID: 1}},
		"req_answer":          {Answer: &sessionDescription{Type: "answer", SDP: "v=0\r\nanswer\r\n", ID: 7}},
		"req_trickle_pub":     {Trickle: &trickleRequest{CandidateInit: goldenCandidate, Target: targetPublisher}},
		"req_trickle_sub":     {Trickle: &trickleRequest{CandidateInit: goldenCandidate, Target: targetSubscriber}},
		"req_add_track_audio": {AddTrack: &addTrackRequest{CID: "audio-track-id", Name: "audio", Type: trackAudio, Source: sourceMicrophone, DisableRed: true}},
		"req_add_track_video": {AddTrack: &addTrackRequest{CID: "video-track-id", Name: "video", Type: trackVideo, Width: 640, Height: 360, Source: sourceCamera}},
		"req_subscription":    {Subscription: &updateSubscription{TrackSIDs: []string{"TR_a2", "TR_v2"}, Subscribe: true}},
		"req_leave":           {Leave: &leaveRequest{Reason: reasonClientInitiated}},
		"req_ping":            {PingReq: &ping{Timestamp: 1760000000000, RTT: 12}},
	} {
		got, err := encodeSignalRequest(req)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if want := golden(t, name); !bytes.Equal(got, want) {
			t.Errorf("%s:\n got % x\nwant % x", name, got, want)
		}
	}
}

func TestEncodeSignalRequest_NeedsOneMessage(t *testing.T) {
	if _, err := encodeSignalRequest(signalRequest{}); err == nil {
		t.Error("empty request encoded without error")
	}
	if _, err := encodeSignalRequest(signalRequest{Leave: &leaveRequest{}, PingReq: &ping{}}); err == nil {
		t.Error("request with two messages encoded without error")
	}
}

func tracksOf(p string) []trackInfo {
	return []trackInfo{
		{SID: "TR_a" + p, Type: trackAudio, Source: sourceMicrophone},
		{SID: "TR_v" + p, Type: trackVideo, Source: sourceCamera},
	}
}

func TestDecodeSignalResponse_Golden(t *testing.T) {
	for name, want := range map[string]signalResponse{
		"resp_join": {Join: &joinResponse{
			Participant: participantInfo{SID: "PA_self", Identity: "bot-1", State: stateJoined},
			OtherParticipants: []participantInfo{
				{SID: "PA_2", Identity: "bot-2", State: stateActive, Tracks: tracksOf("2")},
				{SID: "PA_3", Identity: "bot-3", State: stateActive, Tracks: tracksOf("3")},
			},
			SubscriberPrimary: true,
			PingTimeout:       15,
			PingInterval:      5,
			ServerVersion:     "1.13.10",
			ServerProtocol:    16,
		}},
		"resp_join_alternative": {Join: &joinResponse{AlternativeURL: "wss://other.example.org"}},
		"resp_answer":           {Answer: &sessionDescription{Type: "answer", SDP: "v=0\r\npublish answer\r\n", ID: 1}},
		"resp_offer":            {Offer: &sessionDescription{Type: "offer", SDP: "v=0\r\nsubscribe offer\r\n", ID: 3}},
		"resp_trickle":          {Trickle: &trickleRequest{CandidateInit: goldenCandidate, Target: targetSubscriber}},
		"resp_trickle_pub":      {Trickle: &trickleRequest{CandidateInit: goldenCandidate, Target: targetPublisher}},
		"resp_update": {Update: &participantUpdate{Participants: []participantInfo{
			{SID: "PA_2", Identity: "bot-2", State: stateDisconnected},
			{SID: "PA_4", Identity: "bot-4", State: stateActive, Tracks: tracksOf("4")[1:]},
		}}},
		"resp_track_published":       {TrackPublished: &trackPublishedResponse{CID: "video-track-id", Track: trackInfo{SID: "TR_vself", Type: trackVideo, Source: sourceCamera, Muted: true}}},
		"resp_leave":                 {Leave: &leaveRequest{CanReconnect: true, Reason: reasonServerShutdown, Action: actionReconnect}},
		"resp_pong":                  {Pong: true},
		"resp_pong_resp":             {Pong: true},
		"resp_subscription_response": {SubscriptionResponse: &subscriptionResponse{TrackSID: "TR_v3", Err: 2}},
		"resp_track_unpublished":     {},
		"resp_speakers_changed":      {},
		"resp_refresh_token":         {},
	} {
		got, err := decodeSignalResponse(golden(t, name))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got %+v\nwant %+v", name, got, want)
		}
	}
}

func TestDecodeSignalResponse_Garbage(t *testing.T) {
	join := golden(t, "resp_join")
	for name, b := range map[string][]byte{
		"truncated":       join[:len(join)/2],
		"bad tag":         {0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x01},
		"length past end": {0x0a, 0x7f, 0x01},
	} {
		if _, err := decodeSignalResponse(b); err == nil {
			t.Errorf("%s: decoded without error", name)
		}
	}
	if got, err := decodeSignalResponse(nil); err != nil || !reflect.DeepEqual(got, signalResponse{}) {
		t.Errorf("empty frame: %+v, %v", got, err)
	}
}

func FuzzDecodeSignalResponse(f *testing.F) {
	for _, name := range []string{"resp_join", "resp_update", "resp_offer", "resp_leave", "resp_speakers_changed"} {
		b, err := os.ReadFile(filepath.Join("testdata", "golden", name+".bin"))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		decodeSignalResponse(b) // Must not panic
	})
}
