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
	times    *SendTimes // When each frame's write started
	video    bool       // Video: frames are numbered (012 research R1)
	trailer  bool       // Video frames carry the 012 trailer, in their codec's carrier (014)

	mu   sync.Mutex
	subs [][]TrackWriter // Guarded by mu; one writer per layer, in subscription order
}

type sourceLayer struct {
	content *Content
	payload rtp.Payloader
	codec   VideoCodec   // Video only
	dd      *DDStructure // AV1 and scalable VP9: the dependency descriptor's structure
	// Scalable content (frames with layer frames): VP9's payloader, which
	// writes the layer indices, and the layer frames' DDL3T3Key templates.
	svc bool
	vp9 *vp9Payloader
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
		video:    kind == KindVideo,
	}
	if s.video {
		s.times = NewVideoSendTimes(scenarioTag(scenarioID))
	}
	for _, c := range layers {
		l := sourceLayer{content: c, payload: &codecs.OpusPayloader{}, codec: c.VideoCodec}
		if l.codec == "" {
			l.codec = CodecVP8
		}
		if c.Kind == KindVideo && len(c.Frames) > 0 && c.Frames[0].Layers != nil {
			l.svc = true
			widths := make([]int, len(c.Heights))
			for i, h := range c.Heights {
				widths[i] = h * 16 / 9
			}
			dd := DDL3T3Key(widths, c.Heights)
			l.dd = &dd
			if l.codec == CodecVP9 {
				l.vp9 = newVP9SVCPayloader(widths, c.Heights)
				l.payload = l.vp9
			} else {
				l.payload = &av1Payloader{}
			}
		} else if c.Kind == KindVideo {
			switch l.codec {
			case CodecVP9:
				l.payload = newVP9Payloader(c.Width, c.Height)
			case CodecAV1:
				l.payload = &av1Payloader{}
				dd := DDSingleLayer(c.Width, c.Height)
				l.dd = &dd
			case CodecH264:
				l.payload = &codecs.H264Payloader{}
			case CodecVP8:
				if len(layers) > 1 {
					l.payload = &vp8LayerPayloader{}
				} else {
					l.payload = &codecs.VP8Payloader{EnablePictureID: true}
				}
			}
		}
		s.layers = append(s.layers, l)
	}
	s.trailer = s.video
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

// initialRTPTimestamp is the RTP timestamp of a source's first frame. It is
// not 0 (RFC 3550 5.1: the initial value should not be predictable) so that
// a server which rebases forwarded timestamps from 0, as Janus does, never
// yields timestamps of recently sent frames, which would give the audio
// latency measure wrong values (011 research R8; video uses frame trailers,
// 012). It is below 2^31 because Jitsi's bridge keeps a video stream's
// timestamps only when they are newer than 0 in RTP's modular order, and
// rebases them otherwise. Fixed, so media stays deterministic.
const initialRTPTimestamp uint32 = 1 << 30

// Run sends the content in a loop until ctx ends.
func (s *Source) Run(ctx context.Context) {
	var (
		seqs  = make([]uint16, len(s.layers))
		ts    = initialRTPTimestamp
		num   uint32 // Video frame number, the same on all layers
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
			if s.video {
				num = (num + 1) & frameNumberMask
				s.times.Record(num, now)
			} else {
				s.times.Record(ts, now)
			}
			for l, layer := range s.layers {
				if layer.svc {
					frame[l] = s.packetizeSVC(layer, i, &seqs[l], ts, num)
				} else {
					frame[l] = s.packetize(layer, layer.content.Frames[i].Data, &seqs[l], ts, num)
				}
			}
			s.fanOut(frame)
			ts += f.Samples
			offset += f.Duration
		}
	}
}

// ddHeaderRoom is the room left in a packet for the dependency descriptor:
// the extension header, and the descriptor with its structure on keyframes.
const ddHeaderRoom = 4 + 2 + 64

// packetize splits a frame into RTP packets with consecutive sequence
// numbers, the frame's timestamp and the marker on the last packet. Video
// frames end with the trailer of frame num, always whole in the last packet
// (VP9: as its RTP padding, the payload then ending with the padding and
// the padding bit set, as WriteRTP(header, payload) sends it).
// SSRC and payload type are set per track by the writer.
func (s *Source) packetize(layer sourceLayer, frame []byte, seq *uint16, ts, num uint32) []*rtp.Packet {
	var payloads [][]byte
	mtu := uint16(rtpPayloadMTU)
	if layer.dd != nil {
		mtu -= ddHeaderRoom
	}
	if s.trailer {
		payloads = layer.payload.Payload(mtu-uint16(trailerRoom(layer.codec)), frame)
		payloads = addTrailer(layer.codec, payloads, s.times.Tag(), num)
	} else {
		payloads = layer.payload.Payload(mtu, frame)
	}
	key := layer.dd != nil && isKeyframe(layer.codec, frame)
	packets := make([]*rtp.Packet, len(payloads))
	for i, p := range payloads {
		packets[i] = &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Padding:        s.trailer && trailerIsPadding(layer.codec) && i == len(payloads)-1,
				Marker:         i == len(payloads)-1,
				SequenceNumber: *seq,
				Timestamp:      ts,
			},
			Payload: p,
		}
		if layer.dd != nil {
			d := DD{Start: i == 0, End: i == len(payloads)-1, TemplateID: 1, FrameNumber: uint16(num)}
			if key {
				d.TemplateID = 0
				if i == 0 {
					d.Structure = layer.dd
				}
			}
			_ = packets[i].Header.SetExtension(SourceDDExtID, WriteDD(d))
		}
		*seq++
	}
	return packets
}

// packetizeSVC packetizes picture i of scalable content: its layer frames
// low to high, under one timestamp, with consecutive sequence numbers and
// the marker on the picture's last packet. Each layer frame ends with the
// trailer of picture num (research R9) and is one frame of the dependency
// descriptor, numbered 3·num + its spatial layer, whose template gives its
// layers; the structure is in a keyframe picture's first packet.
func (s *Source) packetizeSVC(layer sourceLayer, i int, seq *uint16, ts, num uint32) []*rtp.Packet {
	f := layer.content.Frames[i]
	key := isKeyframe(layer.codec, f.Layers[0].Data)
	mtu := uint16(rtpPayloadMTU - ddHeaderRoom)
	if s.trailer {
		mtu -= uint16(trailerRoom(layer.codec))
	}
	var frames [][][]byte
	if layer.vp9 != nil {
		frames = layer.vp9.PayloadLayers(mtu, f.Layers)
	} else {
		for _, l := range f.Layers {
			frames = append(frames, layer.payload.Payload(mtu, l.Data))
		}
	}
	var packets []*rtp.Packet
	for sid, payloads := range frames {
		if s.trailer {
			payloads = addTrailer(layer.codec, payloads, s.times.Tag(), num)
		}
		for j, p := range payloads {
			pkt := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Padding:        s.trailer && trailerIsPadding(layer.codec) && j == len(payloads)-1,
					SequenceNumber: *seq,
					Timestamp:      ts,
				},
				Payload: p,
			}
			d := DD{
				Start:       j == 0,
				End:         j == len(payloads)-1,
				TemplateID:  ddL3T3KeyTemplate(key, i, sid),
				FrameNumber: uint16(3*num + uint32(sid)),
			}
			if key && sid == 0 && j == 0 {
				d.Structure = layer.dd
			}
			_ = pkt.Header.SetExtension(SourceDDExtID, WriteDD(d))
			packets = append(packets, pkt)
			*seq++
		}
	}
	if len(packets) > 0 {
		packets[len(packets)-1].Marker = true
	}
	return packets
}

// fanOut writes each layer's packets to every subscription's track for that
// layer, highest layer first. A failing track (e.g. a closed connection) does not affect the
// others.
func (s *Source) fanOut(layers [][]*rtp.Packet) {
	s.mu.Lock()
	subs := append([][]TrackWriter(nil), s.subs...)
	s.mu.Unlock()

	var sent, bytes int
	for _, sub := range subs {
		// Highest layer first: the layers' keyframes come together, and the
		// Jitsi bridge switches to the first keyframe of such a group, then
		// up to the layer it forwards; low first, every receiver would get
		// the lower keyframes too (006 T057).
		for l := len(sub) - 1; l >= 0; l-- {
			t := sub[l]
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
