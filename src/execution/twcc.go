package execution

import (
	"encoding/binary"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor/pkg/twcc"
	"github.com/pion/rtcp"
	"github.com/pion/sdp/v3"
)

// transportFeedbackInterval is how often transport-wide congestion control
// feedback is sent (Pion's default).
const transportFeedbackInterval = 100 * time.Millisecond

// Limits a bot's reception reports are flagged against (015 research R6):
// under ackShareMin of its arriving packets acknowledged, or more than
// lateShareMax of its feedback intervals longer than lateInterval (twice
// the bot's interval; a browser's longest is 250 ms).
const (
	ackShareMin  = 0.99
	lateShareMax = 0.01
	lateInterval = 200 * time.Millisecond
)

// skipRecordEvery and skipRecordPeer make the peer of that ID skip the
// recording for feedback of one packet in skipRecordEvery: a test switch
// for a bot that leaves packets unacknowledged (015 SC-002). 0 in
// production.
var (
	skipRecordEvery int
	skipRecordPeer  string
)

// arrivalRing is the number of socket arrival times kept, by transport
// sequence number, for the stamp lag: about 3 s of packets at 700 per
// second, 16 KiB per bot.
const arrivalRing = 2048

// transportFeedback generates a peer's transport-wide congestion control
// feedback (TWCC) from every RTP packet that reaches its read buffers,
// including the packets of streams the peer drops (research R9 of 005).
// Pion's TWCC interceptor only sees the packets the application reads, so
// the media server would count the others, and its bandwidth probes, as
// lost and lower its estimate.
type transportFeedback struct {
	extID atomic.Uint32 // transport-cc header extension ID of the applied offer; 0: none
	start time.Time

	mu       sync.Mutex
	recorder *twcc.Recorder // Guarded by mu

	runOnce sync.Once
	done    chan struct{}
	stop    sync.Once

	// Reception report quality (015 research R6). counting: the peer's
	// sockets are seen (consentNet), so arrivals are counted.
	counting  atomic.Bool
	closed    atomic.Bool
	arrived   atomic.Uint64              // RTP packets with a transport sequence number read from the socket
	arrivedFb atomic.Uint64              // arrived when the last sent feedback was built
	arrivals  [arrivalRing]atomic.Uint64 // seq<<48 | socket time (µs since start)<<1 | 1, by seq % arrivalRing; for the stamp lag when the buffer records
	now       func() time.Time           // The clock of the feedback timing; time.Now
	skip      uint64                     // Packets seen by the test switch (skipRecordEvery); guarded by mu
	skipEvery uint64                     // Test switch: leave one packet in skipEvery unrecorded; 0 in production
	stats     feedbackStats              // Guarded by mu
}

// feedbackStats are the counters taken under the feedback's lock.
type feedbackStats struct {
	acknowledged uint64
	feedbacks    uint64
	lastSent     time.Time
	intervals    uint64
	intervalSum  time.Duration
	intervalMax  time.Duration
	late         uint64
	lagN         uint64
	lagSum       time.Duration
	lagMax       time.Duration
}

// feedbackSnapshot is a peer's reception reports at one moment.
type feedbackSnapshot struct {
	negotiated   bool   // The applied offer has a transport-cc extension
	counting     bool   // Arrivals were counted at the socket
	arrivedTotal uint64 // Every counted arrival
	arrived      uint64 // Arrivals up to the last sent feedback
	feedbackStats
}

func newTransportFeedback() *transportFeedback {
	return &transportFeedback{
		start:    time.Now(),
		recorder: twcc.NewRecorder(rand.Uint32()),
		done:     make(chan struct{}),
		now:      time.Now,
	}
}

// transportSequence reads the transport-wide sequence number of an RTP
// packet in header extension id, without allocating: arrive runs for every
// datagram the peer's sockets read. ok is false for anything else (STUN,
// DTLS, RTCP, RTP without the extension). RTP headers and their extensions
// are not encrypted by SRTP.
func transportSequence(pkt []byte, id uint8) (seq uint16, ok bool) {
	if id == 0 || len(pkt) < 12 || pkt[0]>>6 != 2 || pkt[0]&0x10 == 0 {
		return 0, false
	}
	if pt := pkt[1]; pt >= 192 && pt <= 223 {
		return 0, false // RTCP (RFC 5761)
	}
	off := 12 + 4*int(pkt[0]&0x0f)
	if len(pkt) < off+4 {
		return 0, false
	}
	profile := binary.BigEndian.Uint16(pkt[off:])
	end := off + 4 + 4*int(binary.BigEndian.Uint16(pkt[off+2:]))
	off += 4
	if end > len(pkt) {
		return 0, false
	}
	switch {
	case profile == 0xBEDE: // One-byte header (RFC 8285)
		for off < end {
			b := pkt[off]
			if b == 0 {
				off++
				continue
			}
			eid, l := b>>4, int(b&0x0f)+1
			if eid == 15 {
				return 0, false
			}
			off++
			if eid == id && l >= 2 && off+2 <= end {
				return binary.BigEndian.Uint16(pkt[off:]), true
			}
			off += l
		}
	case profile&0xfff0 == 0x1000: // Two-byte header
		for off+2 <= end {
			eid := pkt[off]
			if eid == 0 {
				off++
				continue
			}
			l := int(pkt[off+1])
			off += 2
			if eid == id && l >= 2 && off+2 <= end {
				return binary.BigEndian.Uint16(pkt[off:]), true
			}
			off += l
		}
	}
	return 0, false
}

// arrive counts a datagram read from the peer's socket, before Pion
// handles it: every packet that reaches the bot (FR-004).
func (f *transportFeedback) arrive(pkt []byte, at time.Time) {
	if f.closed.Load() {
		return
	}
	seq, ok := transportSequence(pkt, uint8(f.extID.Load()))
	if !ok {
		return
	}
	f.arrived.Add(1)
	if f.counting.Load() {
		// The feedback reports every packet that reached the socket, at the
		// time it was read: Pion may drop it afterwards (the SRTP replay
		// check refuses the copies a bridge sends with their own transport
		// sequence number, 015 verdict for Jitsi).
		f.recordSequence(binary.BigEndian.Uint32(pkt[8:12]), seq, at.Sub(f.start).Microseconds())
		return
	}
	// The low bit marks a stored time, so 0 is "none"; 47 bits of µs.
	us := uint64(at.Sub(f.start).Microseconds()) & (1<<47 - 1)
	f.arrivals[int(seq)%arrivalRing].Store(uint64(seq)<<48 | us<<1 | 1)
}

// setExtension records the transport-cc extension ID of an applied offer.
func (f *transportFeedback) setExtension(parsed *sdp.SessionDescription) {
	for _, md := range parsed.MediaDescriptions {
		for _, a := range md.Attributes {
			if a.Key != "extmap" || !strings.HasSuffix(a.Value, sdp.TransportCCURI) {
				continue
			}
			id, _, _ := strings.Cut(a.Value, " ")
			id, _, _ = strings.Cut(id, "/") // "5/sendrecv"
			if n, err := strconv.ParseUint(id, 10, 8); err == nil && n > 0 {
				f.extID.Store(uint32(n))
				return
			}
		}
	}
}

// recordFromBuffer notes an RTP packet written to a read buffer, for
// feedback when the peer's sockets are not seen (a test replaced the
// network); with them, arrive has already recorded it at the socket.
// The stamp lag, from the socket read to here, exists only on this path.
func (f *transportFeedback) recordFromBuffer(pkt []byte, arrival time.Time) {
	if f.counting.Load() {
		return
	}
	seq, ok := transportSequence(pkt, uint8(f.extID.Load()))
	if !ok {
		return
	}
	us := arrival.Sub(f.start).Microseconds()
	if v := f.arrivals[int(seq)%arrivalRing].Load(); v != 0 && uint16(v>>48) == seq {
		lag := max(0, time.Duration(us-int64(v&(1<<48-1)>>1))*time.Microsecond)
		f.mu.Lock()
		f.stats.lagN++
		f.stats.lagSum += lag
		f.stats.lagMax = max(f.stats.lagMax, lag)
		f.mu.Unlock()
	}
	f.recordSequence(binary.BigEndian.Uint32(pkt[8:12]), seq, us)
}

// recordSequence adds one packet to the next feedback.
func (f *transportFeedback) recordSequence(ssrc uint32, seq uint16, us int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.skipEvery > 0 {
		f.skip++
		if f.skip%f.skipEvery == 0 {
			return // Test switch: this packet is left unacknowledged
		}
	}
	f.recorder.Record(ssrc, seq, us)
}

// build returns the feedback for the packets recorded since the last call.
func (f *transportFeedback) build() []rtcp.Packet {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recorder.BuildFeedbackPacket()
}

// send builds the feedback of the packets recorded since the last one and
// writes it; a written feedback's acknowledged packets, and the time since
// the previous one, are counted.
func (f *transportFeedback) send(write func([]rtcp.Packet) error) {
	arrived := f.arrived.Load()
	pkts := f.build()
	if len(pkts) == 0 {
		return
	}
	if err := write(pkts); err != nil {
		return // Fails until the transport is up
	}
	var acked uint64
	for _, p := range pkts {
		if cc, ok := p.(*rtcp.TransportLayerCC); ok {
			acked += uint64(len(cc.RecvDeltas))
		}
	}
	now := f.now()
	f.arrivedFb.Store(arrived)
	f.mu.Lock()
	defer f.mu.Unlock()
	st := &f.stats
	st.acknowledged += acked
	st.feedbacks++
	if !st.lastSent.IsZero() {
		d := now.Sub(st.lastSent)
		st.intervals++
		st.intervalSum += d
		st.intervalMax = max(st.intervalMax, d)
		if d > lateInterval {
			st.late++
		}
	}
	st.lastSent = now
}

// snapshot returns the peer's reception reports so far.
func (f *transportFeedback) snapshot() feedbackSnapshot {
	f.mu.Lock()
	st := f.stats
	f.mu.Unlock()
	return feedbackSnapshot{
		negotiated:    f.extID.Load() != 0,
		counting:      f.counting.Load(),
		arrivedTotal:  f.arrived.Load(),
		arrived:       f.arrivedFb.Load(),
		feedbackStats: st,
	}
}

// run starts sending the feedback with write, once, until close.
func (f *transportFeedback) run(write func([]rtcp.Packet) error) {
	f.runOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(transportFeedbackInterval)
			defer ticker.Stop()
			for {
				select {
				case <-f.done:
					return
				case <-ticker.C:
					f.send(write)
				}
			}
		}()
	})
}

func (f *transportFeedback) close() {
	f.stop.Do(func() {
		f.closed.Store(true)
		close(f.done)
	})
}

// ReceptionReports returns how the peer reported its reception so far
// (015 research R6).
func (p *Peer) ReceptionReports() feedbackSnapshot { return p.feedback.snapshot() }
