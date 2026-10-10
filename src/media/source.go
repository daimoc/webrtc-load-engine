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

// Source loops one Content and writes it to every subscribed track: each
// frame is packetized once and the same packets go to all tracks, on a
// fixed schedule (frame i of loop k is due at start + k·loop + offset(i)).
// One Source serves all the bots of a scenario for one media kind.
type Source struct {
	content  *Content
	payload  rtp.Payloader
	clock    clock
	lateness prometheus.Observer
	sent     prometheus.Counter
	sentB    prometheus.Counter

	mu     sync.Mutex
	tracks []TrackWriter // Guarded by mu; in subscription order
}

// NewSource creates a Source for the scenario's bots. Run starts sending.
func NewSource(scenarioID string, c *Content) *Source {
	var payload rtp.Payloader = &codecs.OpusPayloader{}
	if c.Kind == KindVideo {
		payload = &codecs.VP8Payloader{EnablePictureID: true}
	}
	return &Source{
		content:  c,
		payload:  payload,
		clock:    realClock{},
		lateness: SendLateness.WithLabelValues(scenarioID, c.Kind),
		sent:     PacketsSent.WithLabelValues(scenarioID, c.Kind),
		sentB:    BytesSent.WithLabelValues(scenarioID, c.Kind),
	}
}

// Kind returns the media kind the source sends.
func (s *Source) Kind() string { return s.content.Kind }

// Subscribe adds a track; it receives packets from the next frame on.
func (s *Source) Subscribe(w TrackWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.tracks {
		if t == w {
			return
		}
	}
	s.tracks = append(s.tracks, w)
}

// Unsubscribe removes a track. It is safe to call for unknown tracks.
func (s *Source) Unsubscribe(w TrackWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tracks {
		if t == w {
			s.tracks = append(s.tracks[:i:i], s.tracks[i+1:]...)
			return
		}
	}
}

// Run sends the content in a loop until ctx ends.
func (s *Source) Run(ctx context.Context) {
	var (
		seq   uint16
		ts    uint32
		start = s.clock.Now()
	)
	for loop := time.Duration(0); ; loop++ {
		offset := time.Duration(0)
		for _, f := range s.content.Frames {
			due := start.Add(loop*s.content.LoopDuration + offset)
			if !s.clock.SleepUntil(ctx, due) {
				return
			}
			s.lateness.Observe(s.clock.Now().Sub(due).Seconds())
			packets := s.packetize(f.Data, &seq, ts)
			s.fanOut(packets)
			ts += f.Samples
			offset += f.Duration
		}
	}
}

// packetize splits a frame into RTP packets with consecutive sequence
// numbers, the frame's timestamp and the marker on the last packet. SSRC and
// payload type are set per track by the writer.
func (s *Source) packetize(frame []byte, seq *uint16, ts uint32) []*rtp.Packet {
	payloads := s.payload.Payload(rtpPayloadMTU, frame)
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

// fanOut writes the packets to every subscribed track. A failing track
// (e.g. a closed connection) does not affect the others.
func (s *Source) fanOut(packets []*rtp.Packet) {
	s.mu.Lock()
	tracks := append([]TrackWriter(nil), s.tracks...)
	s.mu.Unlock()

	var sent, bytes int
	for _, t := range tracks {
		for _, p := range packets {
			if err := t.WriteRTP(p); err != nil {
				break // The rest of this frame would fail too
			}
			sent++
			bytes += len(p.Payload)
		}
	}
	if sent > 0 {
		s.sent.Add(float64(sent))
		s.sentB.Add(float64(bytes))
	}
}
