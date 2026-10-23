package janus

import "slices"

// streamKey is one publisher stream: its feed and its mid.
type streamKey struct {
	feed uint64
	mid  string
}

// streamRef is a stream in an update request.
type streamRef struct {
	Feed uint64 `json:"feed"`
	MID  string `json:"mid,omitempty"`
}

// updateReq is a subscription change, sent as one VideoRoom update
// (research R6).
type updateReq struct {
	Subscribe   []streamRef `json:"subscribe,omitempty"`
	Unsubscribe []streamRef `json:"unsubscribe,omitempty"`
}

func (u updateReq) empty() bool { return len(u.Subscribe) == 0 && len(u.Unsubscribe) == 0 }

// body is the update request's body; an empty list is left out (Janus
// rejects empty lists).
func (u updateReq) body() map[string]any {
	b := map[string]any{"request": "update"}
	if len(u.Subscribe) > 0 {
		b["subscribe"] = u.Subscribe
	}
	if len(u.Unsubscribe) > 0 {
		b["unsubscribe"] = u.Unsubscribe
	}
	return b
}

// kindStreams is the receive cap state of one media kind.
type kindStreams struct {
	active  []streamKey // Subscribed, in subscription order
	waiting []streamKey // Announced, not subscribed: FIFO
}

// receiveCap applies the receive cap to the room's publisher streams
// (data-model "Receive cap"): per kind, at most limit are subscribed and
// the others wait, the earliest first. It is pure data: methods return the
// update for the adapter to send.
type receiveCap struct {
	limit int    // 0: no limit
	dummy uint64 // The dummy publisher, never counted
	kinds map[string]*kindStreams
}

func newReceiveCap(limit int, dummy uint64) *receiveCap {
	return &receiveCap{limit: limit, dummy: dummy, kinds: map[string]*kindStreams{
		"audio": {}, "video": {},
	}}
}

func (c *receiveCap) known(k streamKey) bool {
	for _, ks := range c.kinds {
		if slices.Contains(ks.active, k) || slices.Contains(ks.waiting, k) {
			return true
		}
	}
	return false
}

// announce adds the audio and video streams of publishers, from joined or
// a publishers event. The dummy, disabled streams, data streams and
// streams already known are ignored.
func (c *receiveCap) announce(pubs []Publisher) updateReq {
	var u updateReq
	for _, p := range pubs {
		if p.Dummy || p.ID == c.dummy {
			continue
		}
		for _, s := range p.Streams {
			ks := c.kinds[s.Type]
			k := streamKey{p.ID, s.MID}
			if ks == nil || s.Disabled || c.known(k) {
				continue
			}
			if c.limit == 0 || len(ks.active) < c.limit {
				ks.active = append(ks.active, k)
				u.Subscribe = append(u.Subscribe, streamRef{Feed: k.feed, MID: k.mid})
			} else {
				ks.waiting = append(ks.waiting, k)
			}
		}
	}
	return u
}

// gone drops a feed that unpublished or left: its active streams are
// unsubscribed, each replaced by the earliest waiting stream of its kind.
// Unknown feeds and repeated calls change nothing.
func (c *receiveCap) gone(feed uint64) updateReq {
	var u updateReq
	for _, kind := range []string{"audio", "video"} {
		ks := c.kinds[kind]
		ks.waiting = slices.DeleteFunc(ks.waiting, func(k streamKey) bool { return k.feed == feed })
		for _, k := range ks.active {
			if k.feed == feed {
				u.Unsubscribe = append(u.Unsubscribe, streamRef{Feed: k.feed, MID: k.mid})
			}
		}
		ks.active = slices.DeleteFunc(ks.active, func(k streamKey) bool { return k.feed == feed })
		for (c.limit == 0 || len(ks.active) < c.limit) && len(ks.waiting) > 0 {
			k := ks.waiting[0]
			ks.waiting = ks.waiting[1:]
			ks.active = append(ks.active, k)
			u.Subscribe = append(u.Subscribe, streamRef{Feed: k.feed, MID: k.mid})
		}
	}
	return u
}

// active lists the streams that should be subscribed: audio then video,
// each in subscription order.
func (c *receiveCap) active() []streamKey {
	return slices.Concat(c.kinds["audio"].active, c.kinds["video"].active)
}

// diff is the update that turns the subscription from into to.
func diff(from, to []streamKey) updateReq {
	var u updateReq
	for _, k := range from {
		if !slices.Contains(to, k) {
			u.Unsubscribe = append(u.Unsubscribe, streamRef{Feed: k.feed, MID: k.mid})
		}
	}
	for _, k := range to {
		if !slices.Contains(from, k) {
			u.Subscribe = append(u.Subscribe, streamRef{Feed: k.feed, MID: k.mid})
		}
	}
	return u
}
