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
	payloader := &codecs.VP8Payloader{EnablePictureID: true}
	type packet struct {
		payload []byte
		ts      uint32
	}
	var packets []packet
	var ts uint32
	for _, f := range c.Frames {
		for _, p := range payloader.Payload(1200, f.Data) {
			packets = append(packets, packet{p, ts})
		}
		ts += f.Samples
	}
	q := NewVideoQuality(10 * time.Second)
	stats := NewInboundStats(90000)
	now := time.Unix(1000, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := packets[i%len(packets)]
		loop := uint32(i / len(packets))
		seq := uint16(i)
		at := now.Add(time.Duration(i) * time.Millisecond)
		pts := p.ts + loop*900000
		q.UpdatePacket(seq, pts, p.payload, at)
		stats.Update(seq, pts, len(p.payload), at)
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
