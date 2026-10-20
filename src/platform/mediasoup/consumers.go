package mediasoup

import "slices"

// The receive cap of research R6 (data-model "Receive cap"): per kind, at
// most limit consumers are active; the others are declined and wait, FIFO,
// paused on the server, until an active one closes. The state is pure:
// each event returns what the adapter must do.

type capActionKind int

const (
	actionAccept     capActionKind = iota // Add the consumer's section, renegotiate, accept newConsumer
	actionDecline                         // Answer newConsumer with an error; the consumer waits
	actionDeactivate                      // Set the closed consumer's section inactive, renegotiate
	actionPromote                         // Add a waiting consumer's section, renegotiate, resumeConsumer
)

type capAction struct {
	kind       capActionKind
	consumer   NewConsumerRequest // Accept, decline, promote
	consumerID string             // Deactivate
}

type capState struct {
	limit   int                             // 0: no limit
	active  map[string][]string             // Consumer IDs by kind
	waiting map[string][]NewConsumerRequest // By kind, in arrival order
}

func newCapState(limit int) *capState {
	return &capState{limit: limit, active: map[string][]string{}, waiting: map[string][]NewConsumerRequest{}}
}

// offer handles a newConsumer.
func (c *capState) offer(nc NewConsumerRequest) capAction {
	if c.limit > 0 && len(c.active[nc.Kind]) >= c.limit {
		c.waiting[nc.Kind] = append(c.waiting[nc.Kind], nc)
		return capAction{kind: actionDecline, consumer: nc}
	}
	c.active[nc.Kind] = append(c.active[nc.Kind], nc.ConsumerID)
	return capAction{kind: actionAccept, consumer: nc}
}

// closed handles a consumerClosed: an active consumer frees its slot for
// the earliest waiting one of its kind; a waiting one is dropped.
func (c *capState) closed(id string) []capAction {
	for kind, ids := range c.active {
		i := slices.Index(ids, id)
		if i < 0 {
			continue
		}
		c.active[kind] = slices.Delete(ids, i, i+1)
		acts := []capAction{{kind: actionDeactivate, consumerID: id}}
		if w := c.waiting[kind]; len(w) > 0 {
			next := w[0]
			c.waiting[kind] = w[1:]
			c.active[kind] = append(c.active[kind], next.ConsumerID)
			acts = append(acts, capAction{kind: actionPromote, consumer: next})
		}
		return acts
	}
	for kind, w := range c.waiting {
		if i := slices.IndexFunc(w, func(nc NewConsumerRequest) bool { return nc.ConsumerID == id }); i >= 0 {
			c.waiting[kind] = slices.Delete(w, i, i+1)
			return nil
		}
	}
	return nil
}
