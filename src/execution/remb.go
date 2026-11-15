package execution

import (
	"encoding/binary"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/sdp/v3"
)

// Receive-side bandwidth estimation with REMB (specs/016-network-conditions,
// research R14): where a server asks for REMB and not for transport-wide
// feedback (LiveKit on its defaults), a browser estimates its download from
// the packets' abs-send-time and arrival times, and tells the server. The
// estimator follows libwebrtc's RemoteBitrateEstimatorAbsSendTime: send-time
// groups, a trendline filter with an adaptive threshold, and an AIMD rate.

const (
	absSendTimeURI   = "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time"
	absUnitsPerSec   = 1 << 18 // abs-send-time is 6.18 fixed point seconds
	groupSpan        = 5 * time.Millisecond
	rembInterval     = time.Second
	rembFallSendNow  = 0.97 // A fall below this share of the last REMB is sent at once
	incomingWindow   = time.Second
	ssrcForget       = 2 * time.Second
	trendWindow      = 20
	trendGain        = 4.0
	trendSmoothing   = 0.9
	overuseTimeMs    = 10.0
	thresholdStartMs = 12.5
	thresholdUp      = 0.0087
	thresholdDown    = 0.039
	increasePerSec   = 1.08
	decreaseFactor   = 0.85
)

// bandwidthUsage is the trendline's verdict.
type bandwidthUsage int

const (
	usageNormal bandwidthUsage = iota
	usageOveruse
	usageUnderuse
)

// absSendTime reads the abs-send-time extension of an RTP packet.
func absSendTime(pkt []byte, id uint8) (uint32, bool) {
	b, ok := rtpExtension(pkt, id)
	if !ok || len(b) < 3 {
		return 0, false
	}
	return uint32(b[0])<<16 | uint32(b[1])<<8 | uint32(b[2]), true
}

// rtpExtension returns the payload of header extension id of a raw RTP
// packet (in the clear in SRTP).
func rtpExtension(pkt []byte, id uint8) ([]byte, bool) {
	if id == 0 || len(pkt) < 12 || pkt[0]>>6 != 2 || pkt[0]&0x10 == 0 {
		return nil, false
	}
	if pt := pkt[1]; pt >= 192 && pt <= 223 {
		return nil, false // RTCP
	}
	off := 12 + 4*int(pkt[0]&0x0f)
	if len(pkt) < off+4 {
		return nil, false
	}
	profile := binary.BigEndian.Uint16(pkt[off:])
	end := off + 4 + 4*int(binary.BigEndian.Uint16(pkt[off+2:]))
	off += 4
	if end > len(pkt) {
		return nil, false
	}
	switch {
	case profile == 0xBEDE:
		for off < end {
			b := pkt[off]
			if b == 0 {
				off++
				continue
			}
			eid, l := b>>4, int(b&0x0f)+1
			if eid == 15 {
				return nil, false
			}
			off++
			if off+l > end {
				return nil, false
			}
			if eid == id {
				return pkt[off : off+l], true
			}
			off += l
		}
	case profile&0xfff0 == 0x1000:
		for off+2 <= end {
			eid := pkt[off]
			if eid == 0 {
				off++
				continue
			}
			l := int(pkt[off+1])
			off += 2
			if off+l > end {
				return nil, false
			}
			if eid == id {
				return pkt[off : off+l], true
			}
			off += l
		}
	}
	return nil, false
}

// absUnwrapper turns 24-bit abs-send-times, which wrap every 64 s, into a
// growing count.
type absUnwrapper struct {
	last  uint32
	value int64
	set   bool
}

func (u *absUnwrapper) unwrap(v uint32) int64 {
	if !u.set {
		u.set, u.last, u.value = true, v, int64(v)
		return u.value
	}
	d := int64(v) - int64(u.last)
	if d > 1<<23 {
		d -= 1 << 24
	} else if d < -(1 << 23) {
		d += 1 << 24
	}
	u.last = v
	u.value += d
	return u.value
}

// trendline is libwebrtc's TrendlineEstimator with its adaptive threshold.
type trendline struct {
	accumulated, smoothed float64
	first                 float64 // Arrival time of the first group, ms
	samples               [][2]float64
	deltas                int
	threshold             float64
	lastUpdate            float64
	prevTrend             float64
	overTime              float64
	overCount             int
	state                 bandwidthUsage
	started               bool
}

func newTrendline() *trendline {
	return &trendline{threshold: thresholdStartMs, overTime: -1}
}

// update takes the arrival and send time differences between two groups
// and the later group's arrival time, all in ms.
func (t *trendline) update(arrivalDelta, sendDelta, arrivalMs float64) bandwidthUsage {
	if !t.started {
		t.started, t.first, t.lastUpdate = true, arrivalMs, arrivalMs
	}
	t.deltas = min(t.deltas+1, 1000)
	t.accumulated += arrivalDelta - sendDelta
	t.smoothed = trendSmoothing*t.smoothed + (1-trendSmoothing)*t.accumulated
	t.samples = append(t.samples, [2]float64{arrivalMs - t.first, t.smoothed})
	if len(t.samples) > trendWindow {
		t.samples = t.samples[1:]
	}
	trend := t.prevTrend
	if len(t.samples) == trendWindow {
		trend = slope(t.samples)
	}
	t.detect(trend, sendDelta, arrivalMs)
	return t.state
}

func slope(points [][2]float64) float64 {
	var sx, sy float64
	for _, p := range points {
		sx += p[0]
		sy += p[1]
	}
	mx, my := sx/float64(len(points)), sy/float64(len(points))
	var num, den float64
	for _, p := range points {
		num += (p[0] - mx) * (p[1] - my)
		den += (p[0] - mx) * (p[0] - mx)
	}
	if den == 0 {
		return 0
	}
	return num / den
}

func (t *trendline) detect(trend, sendDelta, now float64) {
	if t.deltas < 2 {
		t.state = usageNormal
		return
	}
	modified := float64(min(t.deltas, 60)) * trend * trendGain
	switch {
	case modified > t.threshold:
		if t.overTime < 0 {
			t.overTime = sendDelta / 2
		} else {
			t.overTime += sendDelta
		}
		t.overCount++
		if t.overTime > overuseTimeMs && t.overCount > 1 && trend >= t.prevTrend {
			t.overTime, t.overCount = 0, 0
			t.state = usageOveruse
		}
	case modified < -t.threshold:
		t.overTime = -1
		t.overCount = 0
		t.state = usageUnderuse
	default:
		t.overTime = -1
		t.overCount = 0
		t.state = usageNormal
	}
	t.prevTrend = trend
	t.adapt(modified, now)
}

func (t *trendline) adapt(modified, now float64) {
	dt := math.Min(now-t.lastUpdate, 100)
	t.lastUpdate = now
	if math.Abs(modified) > t.threshold+15 {
		return // A spike: not a reason to move the threshold
	}
	k := thresholdUp
	if math.Abs(modified) < t.threshold {
		k = thresholdDown
	}
	t.threshold += k * (math.Abs(modified) - t.threshold) * dt
	t.threshold = math.Max(6, math.Min(600, t.threshold))
}

// rembEstimator estimates a receive session's download and sends REMB.
type rembEstimator struct {
	send func(*rtcp.ReceiverEstimatedMaximumBitrate)

	mu      sync.Mutex
	extID   uint8
	unwrap  absUnwrapper
	trend   *trendline
	inGroup bool
	group   struct {
		firstSend, lastSend int64
		lastArrival         time.Time
	}
	prev struct {
		lastSend    int64
		lastArrival time.Time
		set         bool
	}
	start   time.Time   // First packet
	window  []timedSize // Incoming bytes over the last second
	ssrcs   map[uint32]time.Time
	rate    float64 // Estimate, bit/s; 0 before the first second
	holding bool    // After a decrease, until the delay is normal again
	updated time.Time
	sentAt  time.Time
	sent    float64
}

type timedSize struct {
	at    time.Time
	bytes int
}

func newREMBEstimator(send func(*rtcp.ReceiverEstimatedMaximumBitrate)) *rembEstimator {
	return &rembEstimator{send: send, trend: newTrendline(), ssrcs: map[uint32]time.Time{}}
}

// setExtension turns the estimator on with the negotiated abs-send-time ID.
func (e *rembEstimator) setExtension(id uint8) {
	e.mu.Lock()
	e.extID = id
	e.mu.Unlock()
}

// enabled reports whether the session asked for REMB.
func (e *rembEstimator) enabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.extID != 0
}

// arrive takes a datagram at its arrival time.
func (e *rembEstimator) arrive(pkt []byte, at time.Time) {
	e.mu.Lock()
	if e.extID == 0 {
		e.mu.Unlock()
		return
	}
	v, ok := absSendTime(pkt, e.extID)
	if !ok {
		e.mu.Unlock()
		return
	}
	remb := e.arriveLocked(binary.BigEndian.Uint32(pkt[8:12]), e.unwrap.unwrap(v), len(pkt)+udpIPHeaderBytes, at)
	e.mu.Unlock()
	if remb != nil {
		e.send(remb)
	}
}

func (e *rembEstimator) arriveLocked(ssrc uint32, send int64, size int, at time.Time) *rtcp.ReceiverEstimatedMaximumBitrate {
	if e.start.IsZero() {
		e.start = at
	}
	e.ssrcs[ssrc] = at
	e.window = append(e.window, timedSize{at, size})
	for len(e.window) > 0 && at.Sub(e.window[0].at) > incomingWindow {
		e.window = e.window[1:]
	}
	span := int64(groupSpan.Seconds() * absUnitsPerSec)
	switch {
	case !e.inGroup:
		e.inGroup = true
		e.group.firstSend, e.group.lastSend, e.group.lastArrival = send, send, at
	case send-e.group.firstSend <= span:
		e.group.lastSend = max(e.group.lastSend, send)
		e.group.lastArrival = at
	default:
		// The group is complete: compare it with the previous one.
		if e.prev.set {
			sendDelta := float64(e.group.lastSend-e.prev.lastSend) * 1000 / absUnitsPerSec
			arrivalDelta := float64(e.group.lastArrival.Sub(e.prev.lastArrival)) / float64(time.Millisecond)
			usage := e.trend.update(arrivalDelta, sendDelta, float64(e.group.lastArrival.Sub(e.start))/float64(time.Millisecond))
			e.updateRate(usage, at)
		}
		e.prev.lastSend, e.prev.lastArrival, e.prev.set = e.group.lastSend, e.group.lastArrival, true
		e.group.firstSend, e.group.lastSend, e.group.lastArrival = send, send, at
	}
	return e.maybeREMB(at)
}

// incoming is the received rate over the last second, bit/s.
func (e *rembEstimator) incoming() float64 {
	if len(e.window) < 2 {
		return 0
	}
	bytes := 0
	for _, w := range e.window {
		bytes += w.bytes
	}
	return float64(bytes*8) / incomingWindow.Seconds()
}

// updateRate is the AIMD controller.
func (e *rembEstimator) updateRate(usage bandwidthUsage, now time.Time) {
	in := e.incoming()
	if now.Sub(e.start) < incomingWindow {
		return // The incoming rate is not known yet
	}
	if e.rate == 0 {
		e.rate, e.updated = in, now
		return
	}
	dt := now.Sub(e.updated).Seconds()
	e.updated = now
	switch usage {
	case usageOveruse:
		if !e.holding || decreaseFactor*in < e.rate {
			e.rate = decreaseFactor * in
		}
		e.holding = true
	case usageUnderuse:
		// Hold: queues are draining.
	default:
		if e.holding {
			e.holding = false
			return
		}
		e.rate = math.Min(e.rate*math.Pow(increasePerSec, math.Min(dt, 1)), 1.5*in+10000)
	}
}

// maybeREMB returns the REMB to send now, if any: once per interval, or
// at once after a fall.
func (e *rembEstimator) maybeREMB(now time.Time) *rtcp.ReceiverEstimatedMaximumBitrate {
	if e.rate == 0 {
		return nil
	}
	if now.Sub(e.sentAt) < rembInterval && e.rate >= rembFallSendNow*e.sent {
		return nil
	}
	var ssrcs []uint32
	for s, seen := range e.ssrcs {
		if now.Sub(seen) > ssrcForget {
			delete(e.ssrcs, s)
			continue
		}
		ssrcs = append(ssrcs, s)
	}
	sort.Slice(ssrcs, func(i, j int) bool { return ssrcs[i] < ssrcs[j] })
	e.sentAt, e.sent = now, e.rate
	return &rtcp.ReceiverEstimatedMaximumBitrate{Bitrate: float32(e.rate), SSRCs: ssrcs}
}

// rembNegotiated returns the abs-send-time extension ID of an applied
// offer when it asks for REMB and not for transport-wide feedback, as
// LiveKit does on its defaults (research R14); 0 otherwise.
func rembNegotiated(parsed *sdp.SessionDescription) uint8 {
	var remb, twcc bool
	var abs uint8
	for _, md := range parsed.MediaDescriptions {
		for _, a := range md.Attributes {
			switch a.Key {
			case "rtcp-fb":
				remb = remb || strings.HasSuffix(a.Value, " goog-remb")
				twcc = twcc || strings.HasSuffix(a.Value, " transport-cc")
			case "extmap":
				if strings.HasSuffix(a.Value, absSendTimeURI) {
					id, _, _ := strings.Cut(a.Value, " ")
					id, _, _ = strings.Cut(id, "/")
					if n, err := strconv.ParseUint(id, 10, 8); err == nil {
						abs = uint8(n)
					}
				}
				twcc = twcc || strings.HasSuffix(a.Value, sdp.TransportCCURI)
			}
		}
	}
	if !remb || twcc {
		return 0
	}
	return abs
}
