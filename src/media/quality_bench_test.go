package media

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
)

// The quality accounting of one received packet must stay under 200 ns
// without allocating (research R10).

func BenchmarkQualityVideoPacket(b *testing.B) {
	c, err := LoadContent(KindVideo)
	if err != nil {
		b.Fatal(err)
	}
	// 256 frames numbered 65536 apart: the numbers go round modulo 2^24, so
	// the loop never ends, and every frame stays in the send times.
	const frames, step = 256, 1 << 16
	tag := [3]byte{1, 2, 3}
	st := NewVideoSendTimes(tag)
	now := time.Unix(1000, 0)
	payloader := &codecs.VP8Payloader{EnablePictureID: true}
	type packet struct {
		payload []byte
		ts      uint32
	}
	var packets []packet
	var ts uint32
	for i, f := range c.Frames[:frames] {
		n := uint32(i) * step
		st.Record(n, now)
		payloads := payloader.Payload(1200-frameTrailerSize, f.Data)
		payloads[len(payloads)-1] = appendFrameTrailer(payloads[len(payloads)-1], tag, n)
		for _, p := range payloads {
			packets = append(packets, packet{p, ts})
		}
		ts += f.Samples
	}
	q := NewVideoQuality(10 * time.Second)
	q.SetLatency(st, Latency.WithLabelValues("bench-video-quality", KindVideo))
	stats := NewInboundStats(90000)
	// One header for the stream, as the bots' receive loop reuses it
	// (execution.Peer.onTrack): UpdatePacket's own header would escape.
	var h rtp.Header
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := packets[i%len(packets)]
		loop := uint32(i / len(packets))
		h.SequenceNumber = uint16(i)
		h.Timestamp = p.ts + loop*ts
		at := now.Add(time.Duration(i) * time.Millisecond)
		q.UpdateRTP(&h, p.payload, at)
		stats.Update(h.SequenceNumber, h.Timestamp, len(p.payload), at)
	}
	b.StopTimer()
	if b.N >= len(packets) && q.Snapshot(false).Latency.N == 0 {
		b.Fatal("no latency sample: the benchmark misses the lookup")
	}
}

func BenchmarkQualityAudioPacket(b *testing.B) {
	q := NewAudioQuality(48000, 20*time.Millisecond)
	stats := NewInboundStats(48000)
	now := time.Unix(1000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		at := now.Add(time.Duration(i) * 20 * time.Millisecond)
		q.Update(uint16(i), uint32(i)*960, at)
		stats.Update(uint16(i), uint32(i)*960, 80, at)
	}
}

// BenchmarkQualityVideoPacketCodec measures the quality accounting of one
// received packet per codec, as a bot receives it: one simulcast layer
// (the highest), every layer of an SVC stream, latency included (014
// T062). Packets come from the source, with their header extensions and
// padding.
func BenchmarkQualityVideoPacketCodec(b *testing.B) {
	for _, c := range VideoCodecs() {
		for _, arr := range benchArrangements(c) {
			b.Run(string(c)+"/"+string(arr), func(b *testing.B) {
				layers, err := LoadVideo(c, arr, 720)
				if err != nil {
					b.Fatal(err)
				}
				s := NewLayeredSource("bench", layers)
				layer := s.layers[len(s.layers)-1]
				st := s.SendTimes()
				now := time.Unix(1000, 0)
				// Frames numbered 65536 apart, as BenchmarkQualityVideoPacket.
				const frames, step = 256, 1 << 16
				var packets []*rtp.Packet
				var seq uint16
				var ts uint32
				for i := 0; i < frames && i < len(layer.content.Frames); i++ {
					n := uint32(i) * step & frameNumberMask
					st.Record(n, now)
					if layer.svc {
						packets = append(packets, s.packetizeSVC(layer, i, &seq, ts, n)...)
					} else {
						packets = append(packets, s.packetize(layer, layer.content.Frames[i].Data, &seq, ts, n)...)
					}
					ts += 3000
				}
				q := NewVideoQualityFor(c, SourceDDExtID, 10*time.Second)
				q.SetLatency(st, Latency.WithLabelValues("bench-video-quality", KindVideo))
				stats := NewInboundStats(90000)
				h := make([]rtp.Header, len(packets))
				for i, p := range packets {
					h[i] = p.Header
				}
				var hd rtp.Header // Reused, as the receive loop does
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					k := i % len(packets)
					hd = h[k]
					hd.SequenceNumber = uint16(i)
					hd.Timestamp += uint32(i/len(packets)) * ts
					at := now.Add(time.Duration(i) * time.Millisecond)
					q.UpdateRTP(&hd, packets[k].Payload, at)
					stats.Update(hd.SequenceNumber, hd.Timestamp, len(packets[k].Payload), at)
				}
				b.StopTimer()
				if b.N >= len(packets) && q.Snapshot(false).Latency.N == 0 {
					b.Fatal("no latency sample: the benchmark misses the lookup")
				}
			})
		}
	}
}
