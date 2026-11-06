package media

import (
	"testing"
	"time"

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
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := packets[i%len(packets)]
		loop := uint32(i / len(packets))
		seq := uint16(i)
		at := now.Add(time.Duration(i) * time.Millisecond)
		pts := p.ts + loop*ts
		q.UpdatePacket(seq, pts, p.payload, at)
		stats.Update(seq, pts, len(p.payload), at)
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
