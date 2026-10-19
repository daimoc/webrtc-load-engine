package livekit

import (
	"slices"
	"testing"
)

func participant(sid string, state int32, tracks ...trackInfo) participantInfo {
	return participantInfo{SID: sid, Identity: "id-" + sid, State: state, Tracks: tracks}
}

func audio(sid string) trackInfo {
	return trackInfo{SID: sid, Type: trackAudio, Source: sourceMicrophone}
}
func video(sid string) trackInfo { return trackInfo{SID: sid, Type: trackVideo, Source: sourceCamera} }

func wantSubscribe(t *testing.T, what string, got []string, want ...string) {
	t.Helper()
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s: subscribe %v, want %v", what, got, want)
	}
}

func TestSubscriptions_CapPerKindEarliestFirst(t *testing.T) {
	s := newSubscriptions(1, "PA_self")
	add := s.participants([]participantInfo{
		participant("PA_2", stateActive, audio("A2"), video("V2")),
		participant("PA_3", stateActive, audio("A3"), video("V3")),
	})
	wantSubscribe(t, "join", add, "A2", "V2")

	// The subscribed participant leaves: the waiting ones take its place.
	add = s.participants([]participantInfo{participant("PA_2", stateDisconnected)})
	wantSubscribe(t, "PA_2 left", add, "A3", "V3")

	// Nothing waits: a departure subscribes nothing.
	add = s.participants([]participantInfo{participant("PA_3", stateDisconnected)})
	wantSubscribe(t, "PA_3 left", add)
	if n := s.subscribedCount(); n != 0 {
		t.Errorf("subscribed after everyone left = %d", n)
	}
}

func TestSubscriptions_NoLimit(t *testing.T) {
	s := newSubscriptions(0, "PA_self")
	add := s.participants([]participantInfo{
		participant("PA_2", stateActive, audio("A2"), video("V2")),
		participant("PA_3", stateActive, audio("A3")),
	})
	wantSubscribe(t, "no limit", add, "A2", "V2", "A3")
}

func TestSubscriptions_OwnTracksNeverSubscribed(t *testing.T) {
	s := newSubscriptions(0, "PA_self")
	add := s.participants([]participantInfo{
		participant("PA_self", stateActive, audio("A_self"), video("V_self")),
		participant("PA_2", stateActive, audio("A2")),
	})
	wantSubscribe(t, "own tracks", add, "A2")
}

func TestSubscriptions_TrackMissingFromUpdate(t *testing.T) {
	s := newSubscriptions(1, "PA_self")
	s.participants([]participantInfo{
		participant("PA_2", stateActive, audio("A2"), video("V2")),
		participant("PA_3", stateActive, video("V3")),
	})
	// PA_2 unpublishes its video: V3 takes the video slot; A2 stays.
	add := s.participants([]participantInfo{participant("PA_2", stateActive, audio("A2"))})
	wantSubscribe(t, "V2 gone", add, "V3")
	// A repeated update changes nothing.
	add = s.participants([]participantInfo{participant("PA_2", stateActive, audio("A2"))})
	wantSubscribe(t, "same update", add)
}

func TestSubscriptions_TracksAppearLater(t *testing.T) {
	s := newSubscriptions(2, "PA_self")
	add := s.participants([]participantInfo{participant("PA_2", stateJoined)}) // Joined, not publishing yet
	wantSubscribe(t, "no tracks", add)
	add = s.participants([]participantInfo{participant("PA_2", stateActive, audio("A2"), video("V2"))})
	wantSubscribe(t, "tracks published", add, "A2", "V2")
}

func TestSubscriptions_DropOnError(t *testing.T) {
	s := newSubscriptions(1, "PA_self")
	s.participants([]participantInfo{
		participant("PA_2", stateActive, video("V2")),
		participant("PA_3", stateActive, video("V3")),
	})
	wantSubscribe(t, "V2 failed", s.drop("V2"), "V3")
	// The dropped track is not subscribed again by a later update.
	wantSubscribe(t, "update after drop", s.participants([]participantInfo{participant("PA_2", stateActive, video("V2"))}))
}

// The order the real server and the fake use: participants listed at join
// without tracks, their tracks announced one by one as media flows.
func TestSubscriptions_TracksAnnouncedOneByOne(t *testing.T) {
	s := newSubscriptions(1, "PA_self")
	wantSubscribe(t, "join", s.participants([]participantInfo{participant("PA_1", stateActive), participant("PA_4", stateActive)}))
	wantSubscribe(t, "s1 audio", s.participants([]participantInfo{participant("PA_1", stateActive, audio("TR_2"))}), "TR_2")
	wantSubscribe(t, "s1 video", s.participants([]participantInfo{participant("PA_1", stateActive, audio("TR_2"), video("TR_3"))}), "TR_3")
	wantSubscribe(t, "s2 audio", s.participants([]participantInfo{participant("PA_4", stateActive, audio("TR_5"))}))
	wantSubscribe(t, "s2 video", s.participants([]participantInfo{participant("PA_4", stateActive, audio("TR_5"), video("TR_6"))}))
	wantSubscribe(t, "s1 left", s.participants([]participantInfo{participant("PA_1", stateDisconnected)}), "TR_5", "TR_6")
}
