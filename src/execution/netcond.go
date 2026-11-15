package execution

import (
	"math/rand/v2"
	"sync"
	"time"

	"webrtc-load-engine/src/scenario"
)

// Network conditions per bot (specs/016-network-conditions, research
// R1–R4). A constrained bot has one shaper, shared by all its sessions'
// sockets: down conditions what its sockets read, up what they write. A
// bot without a condition has no shaper.

// udpIPHeaderBytes is counted with every datagram on a shaped link: IPv4
// and UDP headers.
const udpIPHeaderBytes = 28

// shaper is one bot's emulated link to the server.
type shaper struct {
	down, up *linkShaper
}

func (s *shaper) close() {
	if s == nil {
		return
	}
	s.down.close()
	s.up.close()
}

// stepCounters is what a link did during one step of its schedule.
type stepCounters struct {
	Offered, Delivered       uint64
	OfferedBytes, Delivered8 uint64 // Delivered8: bytes delivered
	DroppedCap, DroppedLoss  uint64
	Bursts                   uint64 // Loss bursts started
	DelaySum, DelayMax       time.Duration
	Start                    time.Time      // When the step was put in force
	DeliveredPerSecond       map[int]uint64 // Bytes by second since Start, at delivery
}

// queued is a datagram waiting on a link.
type queued struct {
	pkt     []byte
	due     time.Time
	deliver func([]byte)
}

// linkShaper is one direction of a bot's link: a serialization at the cap
// rate behind a drop-tail queue (R2). Its datagrams leave in the order they
// came.
type linkShaper struct {
	mu       sync.Mutex
	step     int // Step in force; -1: none
	cond     *scenario.LinkCondition
	linkFree time.Time // When the link finishes sending what it holds
	lastDue  time.Time // Delivery time of the last datagram admitted
	counts   map[int]*stepCounters
	queue    []queued
	wake     chan struct{}
	done     chan struct{}
	closed   bool
	running  bool

	rng      *rand.Rand // Loss and jitter draws, from the link's seed (R4)
	bad      bool       // Gilbert-Elliott state: in a loss burst
	lostLast bool       // The last datagram was lost (burst counting)

	jitter      float64   // ms, in force until jitterUntil
	jitterUntil time.Time // When a new jitter value is drawn
}

// jitterHold is how long one jitter value lasts (R3): a burst of packets
// shares its delay, as on a real link, so that packets do not wait behind
// each other, which drawing per packet without reordering would make them
// do (46.5 ms on average for 40 ± 10 ms on video, at T032).
const jitterHold = 20 * time.Millisecond

func newLinkShaper(seed uint64) *linkShaper {
	return &linkShaper{step: -1, counts: map[int]*stepCounters{}, wake: make(chan struct{}, 1), done: make(chan struct{}),
		rng: rand.New(rand.NewPCG(seed, seed^0x6A09E667F3BCC909))}
}

// lost draws whether the link loses the next datagram (R4): Bernoulli for
// random loss, the two-state Gilbert-Elliott model for bursts, all lost in
// the bad state, leaving it with chance 1/L and entering it with
// p/(L·(1−p)).
func (l *linkShaper) lost(loss *scenario.Loss) bool {
	if loss == nil || loss.Share <= 0 {
		return false
	}
	if loss.Share >= 1 {
		return true
	}
	if loss.MeanBurst <= 1 {
		return l.rng.Float64() < loss.Share
	}
	if l.bad {
		if l.rng.Float64() < 1/loss.MeanBurst {
			l.bad = false
		}
	} else if l.rng.Float64() < loss.Share/(loss.MeanBurst*(1-loss.Share)) {
		l.bad = true
	}
	return l.bad
}

// setCondition puts step's condition in force from now; a nil condition
// lets datagrams through untouched (no counting).
func (l *linkShaper) setCondition(step int, c *scenario.LinkCondition, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if c == nil {
		l.step, l.cond = -1, nil
		return
	}
	l.step, l.cond = step, c
	if _, ok := l.counts[step]; !ok {
		l.counts[step] = &stepCounters{Start: now, DeliveredPerSecond: map[int]uint64{}}
	}
}

// admit decides the fate of a datagram of size bytes offered at now: its
// delivery time, or false when the link drops it.
func (l *linkShaper) admit(now time.Time, size int) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.admitLocked(now, size)
}

func (l *linkShaper) admitLocked(now time.Time, size int) (time.Time, bool) {
	if l.cond == nil {
		due := now
		if l.lastDue.After(due) {
			due = l.lastDue // Behind what is still queued
		}
		l.lastDue = due
		return due, true
	}
	c := l.counts[l.step]
	wire := size + udpIPHeaderBytes
	c.Offered++
	c.OfferedBytes += uint64(wire)
	if l.lost(l.cond.Loss) {
		c.DroppedLoss++
		if !l.lostLast {
			c.Bursts++
		}
		l.lostLast = true
		return time.Time{}, false
	}
	l.lostLast = false
	depart := now
	if l.cond.RateKbps > 0 {
		if l.linkFree.Before(now) {
			l.linkFree = now
		}
		queueLimit := time.Duration(l.cond.QueueMs) * time.Millisecond
		if l.linkFree.Sub(now) > queueLimit {
			c.DroppedCap++
			return time.Time{}, false
		}
		l.linkFree = l.linkFree.Add(time.Duration(float64(time.Second) * float64(wire*8) / float64(l.cond.RateKbps*1000)))
		depart = l.linkFree
	}
	due := depart
	if d := l.cond.DelayMs; d > 0 {
		jitter := 0.0
		if j := l.cond.JitterMs; j > 0 {
			if !now.Before(l.jitterUntil) {
				l.jitter, l.jitterUntil = (2*l.rng.Float64()-1)*float64(j), now.Add(jitterHold)
			}
			jitter = l.jitter
		}
		due = due.Add(time.Duration((float64(d) + jitter) * float64(time.Millisecond)))
	}
	if l.lastDue.After(due) {
		due = l.lastDue // Jitter never reorders (R3)
	}
	l.lastDue = due
	delay := due.Sub(now)
	c.Delivered++
	c.Delivered8 += uint64(wire)
	c.DelaySum += delay
	c.DelayMax = max(c.DelayMax, delay)
	c.DeliveredPerSecond[int(due.Sub(c.Start)/time.Second)] += uint64(wire)
	return due, true
}

// offer sends pkt through the link: deliver is called with it when it
// leaves, from the link's goroutine, or at once when nothing delays it.
// A dropped datagram is forgotten.
func (l *linkShaper) offer(pkt []byte, deliver func([]byte)) {
	now := time.Now()
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	due, ok := l.admitLocked(now, len(pkt))
	if !ok {
		l.mu.Unlock()
		return
	}
	if len(l.queue) == 0 && !due.After(now) {
		l.mu.Unlock()
		deliver(pkt)
		return
	}
	l.queue = append(l.queue, queued{pkt: pkt, due: due, deliver: deliver})
	if !l.running {
		l.running = true
		go l.run()
	}
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// run delivers the queued datagrams when they are due.
func (l *linkShaper) run() {
	timer := time.NewTimer(time.Hour)
	defer timer.Stop()
	for {
		l.mu.Lock()
		now := time.Now()
		var ready []queued
		for len(l.queue) > 0 && !l.queue[0].due.After(now) {
			ready = append(ready, l.queue[0])
			l.queue[0] = queued{}
			l.queue = l.queue[1:]
		}
		wait := time.Hour
		if len(l.queue) > 0 {
			wait = l.queue[0].due.Sub(now)
		}
		l.mu.Unlock()
		for _, q := range ready {
			q.deliver(q.pkt)
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(wait)
		select {
		case <-l.done:
			return
		case <-l.wake:
		case <-timer.C:
		}
	}
}

// counters returns a copy of the counters of each step that was in force.
func (l *linkShaper) counters() map[int]stepCounters {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[int]stepCounters, len(l.counts))
	for k, c := range l.counts {
		cp := *c
		cp.DeliveredPerSecond = make(map[int]uint64, len(c.DeliveredPerSecond))
		for s, b := range c.DeliveredPerSecond {
			cp.DeliveredPerSecond[s] = b
		}
		out[k] = cp
	}
	return out
}

// close stops the link; queued datagrams are dropped.
func (l *linkShaper) close() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	l.queue = nil
	close(l.done)
}
