package media

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
)

// connectedTrack returns a VP8 track on a PeerConnection connected (ICE+DTLS)
// to a receiving peer that drains what it gets, so writes go through SRTP.
func connectedTrack(b *testing.B) *webrtc.TrackLocalStaticRTP {
	b.Helper()
	send, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		b.Fatal(err)
	}
	recv, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { send.Close(); recv.Close() })

	track, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "bench")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := send.AddTrack(track); err != nil {
		b.Fatal(err)
	}
	recv.OnTrack(func(t *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
		buf := make([]byte, 1500)
		for {
			if _, _, err := t.Read(buf); err != nil {
				return
			}
		}
	})
	connected := make(chan struct{})
	send.OnConnectionStateChange(func(s webrtc.PeerConnectionState) {
		if s == webrtc.PeerConnectionStateConnected {
			close(connected)
		}
	})

	offer, _ := send.CreateOffer(nil)
	gathered := webrtc.GatheringCompletePromise(send)
	if err := send.SetLocalDescription(offer); err != nil {
		b.Fatal(err)
	}
	<-gathered
	if err := recv.SetRemoteDescription(*send.LocalDescription()); err != nil {
		b.Fatal(err)
	}
	answer, _ := recv.CreateAnswer(nil)
	gathered = webrtc.GatheringCompletePromise(recv)
	if err := recv.SetLocalDescription(answer); err != nil {
		b.Fatal(err)
	}
	<-gathered
	if err := send.SetRemoteDescription(*recv.LocalDescription()); err != nil {
		b.Fatal(err)
	}
	select {
	case <-connected:
	case <-time.After(10 * time.Second):
		b.Fatal("peers did not connect")
	}
	return track
}

// BenchmarkSourceFanOut measures one video frame of the built-in content
// packetized once and written to 100 connected tracks (one per bot).
func BenchmarkSourceFanOut(b *testing.B) {
	c, err := LoadContent(KindVideo)
	if err != nil {
		b.Fatal(err)
	}
	s := NewSource("bench", c)
	for i := 0; i < 100; i++ {
		s.Subscribe(connectedTrack(b))
	}
	var seq uint16
	var ts uint32
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		f := c.Frames[i%len(c.Frames)]
		s.fanOut([][]*rtp.Packet{s.packetize(s.layers[0], f.Data, &seq, ts, uint32(i+1))})
		ts += f.Samples
	}
}

// benchArrangements are the arrangements benchmarked per codec (014 T062):
// single stream, and layered up to 720 (simulcast or SVC).
func benchArrangements(c VideoCodec) []Arrangement {
	return []Arrangement{ArrangementSingle, c.Arrangement(true)}
}

// BenchmarkSourcePacketize measures packetizing one picture of each codec's
// content, trailer included, every layer of it (what a source does per
// frame before fanning out; per codec, 014 T062).
func BenchmarkSourcePacketize(b *testing.B) {
	for _, c := range VideoCodecs() {
		for _, arr := range benchArrangements(c) {
			b.Run(string(c)+"/"+string(arr), func(b *testing.B) {
				layers, err := LoadVideo(c, arr, 720)
				if err != nil {
					b.Fatal(err)
				}
				s := NewLayeredSource("bench", layers)
				seqs := make([]uint16, len(s.layers))
				frames := len(s.layers[0].content.Frames)
				var ts uint32
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					f := i % frames
					for l, layer := range s.layers {
						if layer.svc {
							s.packetizeSVC(layer, f, &seqs[l], ts, uint32(i+1))
						} else {
							s.packetize(layer, layer.content.Frames[f].Data, &seqs[l], ts, uint32(i+1))
						}
					}
					ts += 3000
				}
			})
		}
	}
}
