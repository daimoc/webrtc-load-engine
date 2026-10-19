package livekit

import "slices"

// subscriptions is a bot's receive cap (research R7, data-model "Receive-Cap
// State"): the remote tracks it knows, per kind in the order first seen,
// and those it subscribed to, at most limit per kind (0: all). The caller
// sends one UpdateSubscription for the SIDs each change returns. Not safe
// for concurrent use: the bot's read loop owns it.
type subscriptions struct {
	limit      int
	own        string                     // Own participant SID: never subscribed
	known      map[int32][]string         // By track type: track SIDs, first seen first
	owner      map[string]string          // Track SID -> participant SID
	tracks     map[string]map[string]bool // Participant SID -> its track SIDs
	subscribed map[string]bool            // Track SIDs subscribed
	dropped    map[string]bool            // Track SIDs the server refused: never again
	kindOf     map[string]int32           // Track SID -> type
}

func newSubscriptions(limit int, ownSID string) *subscriptions {
	return &subscriptions{
		limit:      limit,
		own:        ownSID,
		known:      map[int32][]string{},
		owner:      map[string]string{},
		tracks:     map[string]map[string]bool{},
		subscribed: map[string]bool{},
		dropped:    map[string]bool{},
		kindOf:     map[string]int32{},
	}
}

// participants applies the participants of a JoinResponse or a
// ParticipantUpdate: a disconnected participant's tracks are forgotten,
// otherwise its track list replaces what was known of it. It returns the
// track SIDs to subscribe to now.
func (s *subscriptions) participants(ps []participantInfo) []string {
	for _, p := range ps {
		if p.SID == s.own {
			continue
		}
		if p.State == stateDisconnected {
			for sid := range s.tracks[p.SID] {
				s.forget(sid)
			}
			delete(s.tracks, p.SID)
			continue
		}
		listed := map[string]bool{}
		for _, t := range p.Tracks {
			if t.Type != trackAudio && t.Type != trackVideo {
				continue
			}
			listed[t.SID] = true
			if _, ok := s.owner[t.SID]; ok || s.dropped[t.SID] {
				continue
			}
			s.owner[t.SID] = p.SID
			s.kindOf[t.SID] = t.Type
			s.known[t.Type] = append(s.known[t.Type], t.SID)
		}
		for sid := range s.tracks[p.SID] {
			if !listed[sid] {
				s.forget(sid)
			}
		}
		kept := map[string]bool{}
		for sid := range listed {
			if !s.dropped[sid] {
				kept[sid] = true
			}
		}
		s.tracks[p.SID] = kept
	}
	return s.fill()
}

// drop forgets a track the server refused to subscribe us to, for good,
// and returns the track SIDs to subscribe to in its place.
func (s *subscriptions) drop(trackSID string) []string {
	s.dropped[trackSID] = true
	if p, ok := s.owner[trackSID]; ok {
		delete(s.tracks[p], trackSID)
	}
	s.forget(trackSID)
	return s.fill()
}

// forget removes a track from what is known and subscribed.
func (s *subscriptions) forget(sid string) {
	kind, ok := s.kindOf[sid]
	if !ok {
		return
	}
	s.known[kind] = slices.DeleteFunc(s.known[kind], func(k string) bool { return k == sid })
	delete(s.subscribed, sid)
	delete(s.owner, sid)
	delete(s.kindOf, sid)
}

// fill subscribes, per kind, the earliest known tracks not subscribed yet
// until the limit is reached.
func (s *subscriptions) fill() []string {
	var add []string
	for _, kind := range []int32{trackAudio, trackVideo} {
		n := 0
		for _, sid := range s.known[kind] {
			if s.subscribed[sid] {
				n++
			}
		}
		for _, sid := range s.known[kind] {
			if s.limit > 0 && n >= s.limit {
				break
			}
			if s.subscribed[sid] {
				continue
			}
			s.subscribed[sid] = true
			add = append(add, sid)
			n++
		}
	}
	return add
}

// subscribedCount returns the number of tracks subscribed.
func (s *subscriptions) subscribedCount() int {
	return len(s.subscribed)
}
