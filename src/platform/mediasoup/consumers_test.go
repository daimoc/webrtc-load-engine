package mediasoup

import (
	"fmt"
	"testing"
)

func consumer(id, kind string) NewConsumerRequest {
	return NewConsumerRequest{ConsumerID: id, Kind: kind, PeerID: "p-" + id}
}

// checkInvariants checks data-model "Receive cap": len(active) ≤ limit per
// kind, and no waiting consumer is active.
func checkInvariants(t *testing.T, c *capState) {
	t.Helper()
	for kind, ids := range c.active {
		if c.limit > 0 && len(ids) > c.limit {
			t.Errorf("%s: %d active, limit %d", kind, len(ids), c.limit)
		}
		for _, w := range c.waiting[kind] {
			for _, id := range ids {
				if w.ConsumerID == id {
					t.Errorf("%s is both active and waiting", id)
				}
			}
		}
	}
}

func TestCap_AcceptUpToLimitPerKind(t *testing.T) {
	c := newCapState(2)
	for i, want := range []capActionKind{actionAccept, actionAccept, actionDecline, actionDecline} {
		a := c.offer(consumer(fmt.Sprintf("a%d", i), "audio"))
		if a.kind != want {
			t.Errorf("audio %d: %v, want %v", i, a.kind, want)
		}
		checkInvariants(t, c)
	}
	// Video is counted on its own.
	for i := range 2 {
		if a := c.offer(consumer(fmt.Sprintf("v%d", i), "video")); a.kind != actionAccept {
			t.Errorf("video %d: %v, want accept", i, a.kind)
		}
	}
	if a := c.offer(consumer("v2", "video")); a.kind != actionDecline {
		t.Errorf("video 2: %v, want decline", a.kind)
	}
	checkInvariants(t, c)
}

func TestCap_NoLimit(t *testing.T) {
	c := newCapState(0)
	for i := range 50 {
		if a := c.offer(consumer(fmt.Sprint(i), "video")); a.kind != actionAccept {
			t.Fatalf("consumer %d: %v, want accept", i, a.kind)
		}
	}
}

func TestCap_CloseActivePromotesEarliestWaiting(t *testing.T) {
	c := newCapState(1)
	c.offer(consumer("a1", "audio"))
	c.offer(consumer("a2", "audio")) // Waiting, first
	c.offer(consumer("a3", "audio")) // Waiting, second
	c.offer(consumer("v1", "video"))
	c.offer(consumer("v2", "video")) // Waiting

	acts := c.closed("a1")
	if len(acts) != 2 || acts[0].kind != actionDeactivate || acts[0].consumerID != "a1" || acts[1].kind != actionPromote || acts[1].consumer.ConsumerID != "a2" {
		t.Fatalf("close a1: %+v, want deactivate a1 then promote a2", acts)
	}
	checkInvariants(t, c)
	acts = c.closed("a2")
	if len(acts) != 2 || acts[1].consumer.ConsumerID != "a3" {
		t.Fatalf("close a2: %+v, want promote a3", acts)
	}
	acts = c.closed("a3")
	if len(acts) != 1 || acts[0].kind != actionDeactivate {
		t.Fatalf("close a3 with none waiting: %+v, want deactivate only", acts)
	}
	// The video waiting list was not touched.
	if acts := c.closed("v1"); len(acts) != 2 || acts[1].consumer.ConsumerID != "v2" {
		t.Fatalf("close v1: %+v, want promote v2", acts)
	}
	checkInvariants(t, c)
}

func TestCap_CloseWaitingDropsIt(t *testing.T) {
	c := newCapState(1)
	c.offer(consumer("a1", "audio"))
	c.offer(consumer("a2", "audio"))
	c.offer(consumer("a3", "audio"))
	if acts := c.closed("a2"); len(acts) != 0 {
		t.Fatalf("close waiting a2: %+v, want nothing", acts)
	}
	if acts := c.closed("a1"); len(acts) != 2 || acts[1].consumer.ConsumerID != "a3" {
		t.Fatalf("close a1: %+v, want promote a3 (a2 dropped)", acts)
	}
	if acts := c.closed("unknown"); len(acts) != 0 {
		t.Errorf("close unknown: %+v", acts)
	}
	if acts := c.closed("a1"); len(acts) != 0 {
		t.Errorf("close a1 twice: %+v", acts)
	}
	checkInvariants(t, c)
}
