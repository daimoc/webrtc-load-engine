package media

import (
	"context"
	"sync"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
	"github.com/prometheus/client_golang/prometheus"
)

// rtpPayloadMTU bounds the RTP payload of a packet, leaving room for the RTP
// header, header extensions and SRTP authentication within a 1280-byte path.
const rtpPayloadMTU = 1200

// TrackWriter receives the packets of a Source. webrtc.TrackLocalStaticRTP
// implements it, rewriting the SSRC and payload type for each binding (the
// packet itself is not modified).
type TrackWriter interface {
	WriteRTP(p *rtp.Packet) error
}

// clock abstracts time for tests.
type clock interface {
	Now() time.Time
	// SleepUntil waits until t; it reports false if ctx ended first.
	SleepUntil(ctx context.Context, t time.Time) bool
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) SleepUntil(ctx context.Context, t time.Time) bool {
	d := time.Until(t)
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// Source loops one Content, or several simulcast layers of it, and writes
// it to every subscribed track: each frame is packetized once and the same
// packets go to all tracks, on a fixed schedule (frame i of loop k is due
// at start + k·loop + offset(i)). Layers are sent in lockstep: frame i of
// every layer at the same time with the same RTP timestamp, each layer
// with its own sequence numbers (006 research R6). One Source serves all
// the bots of a scenario for one media kind.
type Source struct {
	layers   []sourceLayer // Low to high; one without simulcast
	clock    clock
	lateness prometheus.Observer
	sent     prometheus.Counter
	sentB    prometheus.Counter
	times    *SendTimes // When each frame's write started, by RTP timestamp

	mu   sync.Mutex
	subs [][]TrackWriter // Guarded by mu; one writer per layer, in subscription order
}

type sourceLayer struct {
	content *Content
	payload rtp.Payloader
}

// NewSource creates a Source for the scenario's bots. Run starts sending.
func NewSource(scenarioID string, c *Content) *Source {
	return NewLayeredSource(scenarioID, []*Content{c})
}

// NewLayeredSource creates a Source sending the layers (low to high, same
// kind, same frame timing, see LoadLayers) in lockstep.
func NewLayeredSource(scenarioID string, layers []*Content) *Source {
	kind := layers[0].Kind
	s := &Source{
		clock:    realClock{},
		lateness: SendLateness.WithLabelValues(scenarioID, kind),
		sent:     PacketsSent.WithLabelValues(scenarioID, kind),
		sentB:    BytesSent.WithLabelValues(scenarioID, kind),
		times:    NewSendTimes(),
	}
	for _, c := range layers {
		var payload rtp.Payloader = &codecs.OpusPayloader{}
		if c.Kind == KindVideo {
			payload = &codecs.VP8Payloader{EnablePictureID: true}
		}
		s.layers = append(s.layers, sourceLayer{content: c, payload: payload})
	}
	return s
}

// SendTimes returns the send time of the source's recent frames, for the
// receivers' latency (research R5).
func (s *Source) SendTimes() *SendTimes { return s.times }

// Kind returns the media kind the source sends.
func (s *Source) Kind() string { return s.layers[0].content.Kind }

// Layers returns the number of layers the source sends.
func (s *Source) Layers() int { return len(s.layers) }

// Subscribe adds a track to a single-layer source; it receives packets from
// the next frame on.
func (s *Source) Subscribe(w TrackWriter) {
	s.SubscribeLayers([]TrackWriter{w})
}

// SubscribeLayers adds one track per layer, low to high; they receive
// packets from the next frame on. A subscription with the wrong number of
// tracks is ignored.
func (s *Source) SubscribeLayers(ws []TrackWriter) {
	if len(ws) != len(s.layers) {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sub := range s.subs {
		if sub[0] == ws[0] {
			return
		}
	}
	s.subs = append(s.subs, append([]TrackWriter(nil), ws...))
}

// Unsubscribe removes the subscription whose first (or only) track is w.
// It is safe to call for unknown tracks.
func (s *Source) Unsubscribe(w TrackWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, sub := range s.subs {
		if sub[0] == w {
			s.subs = append(s.subs[:i:i], s.subs[i+1:]...)
			return
		}
	}
}

// Run sends the content in a loop until ctx ends.
func (s *Source) Run(ctx context.Context) {
	var (
		seqs  = make([]uint16, len(s.layers))
		ts    uint32
		start = s.clock.Now()
		base  = s.layers[0].content
		frame = make([][]*rtp.Packet, len(s.layers))
	)
	for loop := time.Duration(0); ; loop++ {
		offset := time.Duration(0)
		for i, f := range base.Frames {
			due := start.Add(loop*base.LoopDuration + offset)
			if !s.clock.SleepUntil(ctx, due) {
				return
			}
			now := s.clock.Now()
			s.lateness.Observe(now.Sub(due).Seconds())
			s.times.Record(ts, now)
			for l, layer := range s.layers {
				frame[l] = s.packetize(layer, layer.content.Frames[i].Data, &seqs[l], ts)
			}
			s.fanOut(frame)
			ts += f.Samples
			offset += f.Duration
		}
	}
}

// packetize splits a frame into RTP packets with consecutive sequence
// numbers, the frame's timestamp and the marker on the last packet. SSRC and
// payload type are set per track by the writer.
func (s *Source) packetize(layer sourceLayer, frame []byte, seq *uint16, ts uint32) []*rtp.Packet {
	payloads := layer.payload.Payload(rtpPayloadMTU, frame)
	packets := make([]*rtp.Packet, len(payloads))
	for i, p := range payloads {
		packets[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         i == len(payloads)-1,
				SequenceNumber: *seq,
				Timestamp:      ts,
			},
			Payload: p,
		}
		*seq++
	}
	return packets
}

// fanOut writes each layer's packets to every subscription's track for that
// layer. A failing track (e.g. a closed connection) does not affect the
// others.
func (s *Source) fanOut(layers [][]*rtp.Packet) {
	s.mu.Lock()
	subs := append([][]TrackWriter(nil), s.subs...)
	s.mu.Unlock()

	var sent, bytes int
	for _, sub := range subs {
		for l, t := range sub {
			for _, p := range layers[l] {
				if err := t.WriteRTP(p); err != nil {
					break // The rest of this frame would fail too
				}
				sent++
				bytes += len(p.Payload)
			}
		}
	}
	if sent > 0 {
		s.sent.Add(float64(sent))
		s.sentB.Add(float64(bytes))
	}
}
