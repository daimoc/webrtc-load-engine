package janus

import (
	"reflect"
	"testing"
)

const dummyFeed = 99

func pub(id uint64, kinds ...string) Publisher {
	p := Publisher{ID: id, Display: "p"}
	for i, k := range kinds {
		p.Streams = append(p.Streams, PublisherStream{Type: k, MID: string(rune('0' + i))})
	}
	return p
}

func ref(feed uint64, mid string) streamRef { return streamRef{Feed: feed, MID: mid} }

func checkUpdate(t *testing.T, what string, got updateReq, sub, unsub []streamRef) {
	t.Helper()
	if !reflect.DeepEqual(got.Subscribe, sub) || !reflect.DeepEqual(got.Unsubscribe, unsub) {
		t.Errorf("%s: update = subscribe %v unsubscribe %v, want subscribe %v unsubscribe %v", what, got.Subscribe, got.Unsubscribe, sub, unsub)
	}
}

func checkInvariant(t *testing.T, c *receiveCap) {
	t.Helper()
	for kind, k := range c.kinds {
		if c.limit > 0 && len(k.active) > c.limit {
			t.Errorf("%s: %d active > limit %d", kind, len(k.active), c.limit)
		}
	}
}

func TestReceiveCap_ActiveUpToLimitThenWaiting(t *testing.T) {
	c := newReceiveCap(2, dummyFeed)
	u := c.announce([]Publisher{
		{ID: dummyFeed, Dummy: true, Streams: []PublisherStream{{Type: "audio", MID: "0"}}},
		pub(1, "audio", "video"), pub(2, "audio", "video"),
	})
	checkUpdate(t, "first two", u, []streamRef{ref(1, "0"), ref(1, "1"), ref(2, "0"), ref(2, "1")}, nil)
	u = c.announce([]Publisher{pub(3, "audio", "video")})
	checkUpdate(t, "third waits", u, nil, nil)
	if !u.empty() {
		t.Error("update not empty")
	}
	if w := c.kinds["audio"].waiting; !reflect.DeepEqual(w, []streamKey{{3, "0"}}) {
		t.Errorf("audio waiting = %v", w)
	}
	checkInvariant(t, c)
}

func TestReceiveCap_KindsCountedSeparately(t *testing.T) {
	c := newReceiveCap(1, dummyFeed)
	u := c.announce([]Publisher{pub(1, "audio"), pub(2, "video"), pub(3, "audio", "video")})
	checkUpdate(t, "one per kind", u, []streamRef{ref(1, "0"), ref(2, "0")}, nil)
	checkInvariant(t, c)
}

func TestReceiveCap_NoLimit(t *testing.T) {
	c := newReceiveCap(0, dummyFeed)
	var pubs []Publisher
	var want []streamRef
	for id := uint64(1); id <= 20; id++ {
		pubs = append(pubs, pub(id, "audio", "video"))
		want = append(want, ref(id, "0"), ref(id, "1"))
	}
	checkUpdate(t, "all", c.announce(pubs), want, nil)
}

func TestReceiveCap_IgnoredStreams(t *testing.T) {
	c := newReceiveCap(5, dummyFeed)
	p := Publisher{ID: 4, Streams: []PublisherStream{
		{Type: "audio", MID: "0", Disabled: true}, {Type: "data", MID: "1"}, {Type: "video", MID: "2"},
	}}
	u := c.announce([]Publisher{{ID: dummyFeed, Dummy: true, Streams: []PublisherStream{{Type: "audio", MID: "0"}}}, p})
	checkUpdate(t, "dummy, disabled and data skipped", u, []streamRef{ref(4, "2")}, nil)
	// The dummy flag alone is enough, whatever its ID.
	u = c.announce([]Publisher{{ID: 77, Dummy: true, Streams: []PublisherStream{{Type: "audio", MID: "0"}}}})
	checkUpdate(t, "other dummy", u, nil, nil)
	// Announced again: no duplicate m-line.
	u = c.announce([]Publisher{p})
	checkUpdate(t, "again", u, nil, nil)
}

func TestReceiveCap_GonePromotesFIFO(t *testing.T) {
	c := newReceiveCap(1, dummyFeed)
	c.announce([]Publisher{pub(1, "audio", "video")})
	c.announce([]Publisher{pub(2, "audio", "video")})
	c.announce([]Publisher{pub(3, "audio", "video")})
	u := c.gone(1)
	checkUpdate(t, "1 gone", u, []streamRef{ref(2, "0"), ref(2, "1")}, []streamRef{ref(1, "0"), ref(1, "1")})
	// A waiting feed that goes is dropped from the waiting list.
	u = c.gone(3)
	checkUpdate(t, "3 gone while waiting", u, nil, nil)
	if len(c.kinds["audio"].waiting) != 0 || len(c.kinds["video"].waiting) != 0 {
		t.Errorf("waiting not emptied: %+v %+v", c.kinds["audio"], c.kinds["video"])
	}
	u = c.gone(2)
	checkUpdate(t, "2 gone, none waiting", u, nil, []streamRef{ref(2, "0"), ref(2, "1")})
	checkInvariant(t, c)
}

func TestReceiveCap_GoneIdempotent(t *testing.T) {
	c := newReceiveCap(2, dummyFeed)
	c.announce([]Publisher{pub(1, "audio", "video")})
	c.gone(1)
	// Janus sends unpublished twice, then leaving (T002).
	if u := c.gone(1); !u.empty() {
		t.Errorf("second gone = %+v", u)
	}
	if u := c.gone(42); !u.empty() {
		t.Errorf("unknown feed gone = %+v", u)
	}
	// A feed that comes back after it left is subscribed again.
	u := c.announce([]Publisher{pub(1, "audio")})
	checkUpdate(t, "back", u, []streamRef{ref(1, "0")}, nil)
}

func TestReceiveCap_PartialPromotion(t *testing.T) {
	// One feed ends with audio only active (video limit full of others).
	c := newReceiveCap(1, dummyFeed)
	c.announce([]Publisher{pub(1, "video")})
	c.announce([]Publisher{pub(2, "audio", "video")}) // audio active, video waits
	c.announce([]Publisher{pub(3, "audio")})          // waits
	u := c.gone(2)
	checkUpdate(t, "2 gone", u, []streamRef{ref(3, "0")}, []streamRef{ref(2, "0")})
	if len(c.kinds["video"].waiting) != 0 {
		t.Errorf("2's waiting video kept: %v", c.kinds["video"].waiting)
	}
}

func TestUpdateReq_Body(t *testing.T) {
	u := updateReq{Subscribe: []streamRef{ref(1, "0")}}
	b := u.body()
	if b["request"] != "update" || b["unsubscribe"] != nil {
		t.Errorf("body = %v", b)
	}
	if s, ok := b["subscribe"].([]streamRef); !ok || len(s) != 1 {
		t.Errorf("subscribe = %v", b["subscribe"])
	}
}

func TestReceiveCap_ActiveAndDiff(t *testing.T) {
	c := newReceiveCap(1, dummyFeed)
	c.announce([]Publisher{pub(1, "audio", "video"), pub(2, "audio", "video")})
	before := c.active()
	if !reflect.DeepEqual(before, []streamKey{{1, "0"}, {1, "1"}}) {
		t.Fatalf("active = %v", before)
	}
	c.gone(1)
	after := c.active()
	checkUpdate(t, "diff", diff(before, after), []streamRef{ref(2, "0"), ref(2, "1")}, []streamRef{ref(1, "0"), ref(1, "1")})
	if u := diff(after, after); !u.empty() {
		t.Errorf("diff of equal sets = %+v", u)
	}
	// Changes that cancel out while the subscription waits give no update.
	c.announce([]Publisher{pub(3, "audio")})
	c.gone(3)
	if u := diff(after, c.active()); !u.empty() {
		t.Errorf("cancelled change = %+v", u)
	}
}
