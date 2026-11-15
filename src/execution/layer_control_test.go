package execution

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/media"
)

// packetRecorder keeps copies of the packets a source writes to it.
type packetRecorder struct {
	mu      sync.Mutex
	packets []*rtp.Packet
}

func (r *packetRecorder) WriteRTP(p *rtp.Packet) error {
	c := p.Clone()
	r.mu.Lock()
	r.packets = append(r.packets, c)
	r.mu.Unlock()
	return nil
}

// layerPackets runs the plan's layered content (layers up to 720) for a
// bit over a second, two keyframes, and returns each layer's packets.
func layerPackets(t *testing.T, c media.VideoCodec, arr media.Arrangement) [][]*rtp.Packet {
	t.Helper()
	contents, err := media.LoadVideo(c, arr, 720)
	if err != nil {
		t.Fatal(err)
	}
	src := media.NewLayeredSource("layers-"+t.Name(), contents)
	recs := make([]*packetRecorder, src.Layers())
	ws := make([]media.TrackWriter, len(recs))
	for i := range recs {
		recs[i] = &packetRecorder{}
		ws[i] = recs[i]
	}
	src.SubscribeLayers(ws)
	ctx, cancel := context.WithTimeout(context.Background(), 1300*time.Millisecond)
	defer cancel()
	src.Run(ctx)
	out := make([][]*rtp.Packet, len(recs))
	for i, r := range recs {
		out[i] = r.packets
	}
	return out
}

// payloadWriter keeps the headers and payloads written to a binding.
type payloadWriter struct {
	hdrs     []rtp.Header
	payloads [][]byte
}

func (w *payloadWriter) WriteRTP(h *rtp.Header, payload []byte) (int, error) {
	w.hdrs = append(w.hdrs, h.Clone())
	w.payloads = append(w.payloads, append([]byte(nil), payload...))
	return len(payload), nil
}
func (w *payloadWriter) Write(b []byte) (int, error) { return len(b), nil }

// payloadBinding is a fakeBinding whose writer keeps payloads.
type payloadBinding struct {
	fakeBinding
	pw *payloadWriter
}

func (b *payloadBinding) WriteStream() webrtc.TrackLocalWriter { return b.pw }

func boundTrack(t *testing.T, c media.VideoCodec, dd bool) (*videoTrack, *payloadWriter) {
	t.Helper()
	tr := newVideoTrack(c, "v", "s", "")
	w := &payloadWriter{}
	b := &payloadBinding{fakeBinding: fakeBinding{id: "b", ssrc: 1, codecs: []webrtc.RTPCodecParameters{codecParams(c.MimeType(), c.FmtpLine(), 96)}}, pw: w}
	if dd {
		b.exts = []webrtc.RTPHeaderExtensionParameter{{URI: media.DependencyDescriptorURI, ID: 5}}
	}
	if _, err := tr.Bind(b); err != nil {
		t.Fatal(err)
	}
	return tr, w
}

// contiguous checks that the written sequence numbers follow each other.
func contiguous(t *testing.T, hdrs []rtp.Header) {
	t.Helper()
	for i := 1; i < len(hdrs); i++ {
		if hdrs[i].SequenceNumber != hdrs[i-1].SequenceNumber+1 {
			t.Fatalf("sequence numbers %d then %d at packet %d", hdrs[i-1].SequenceNumber, hdrs[i].SequenceNumber, i)
		}
	}
}

// A simulcast layer stops at once and starts again at its next keyframe,
// with no gap in what the server sees (016 T033).
func TestLayerControl_Simulcast(t *testing.T) {
	layers := layerPackets(t, media.CodecVP8, media.ArrangementSimulcast)
	top := layers[len(layers)-1]
	tr, w := boundTrack(t, media.CodecVP8, false)
	tr.enableLayerControl()
	third := len(top) / 3
	for i, p := range top {
		switch i {
		case third / 2:
			tr.setLayerEnabled(false)
		case third:
			tr.setLayerEnabled(true)
		}
		if err := tr.WriteRTP(p); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.hdrs) == 0 || len(w.hdrs) >= len(top) {
		t.Fatalf("%d of %d packets written", len(w.hdrs), len(top))
	}
	contiguous(t, w.hdrs)
	// The first packet written after the pause is a keyframe's first.
	parser := media.NewFrameParser(media.CodecVP8, 0)
	resumed := third / 2 // Packets written before the pause
	d, ok := parser.Parse(&w.hdrs[resumed], w.payloads[resumed])
	if !ok || !d.Start || !d.Keyframe {
		t.Errorf("resumed on %+v, want a keyframe's first packet", d)
	}
}

// A scalable stream drops its top spatial layer at once and takes it back
// at a keyframe; the end of the top layer still sent carries the marker
// (016 T033).
func TestLayerControl_SVC(t *testing.T) {
	layers := layerPackets(t, media.CodecAV1, media.ArrangementSVC)
	all := layers[0]
	tr, w := boundTrack(t, media.CodecAV1, true)
	tr.enableLayerControl()
	tr.setMaxSpatial(1)
	quarter := len(all) / 4 // Before the second keyframe, at about 1 s
	writtenBefore := 0
	for i, p := range all {
		if i == quarter {
			writtenBefore = len(w.hdrs)
			tr.setMaxSpatial(2)
		}
		if err := tr.WriteRTP(p); err != nil {
			t.Fatal(err)
		}
	}
	contiguous(t, w.hdrs)
	parser := media.NewFrameParser(media.CodecAV1, 5)
	descs := make([]media.FrameDesc, len(w.hdrs))
	for i := range w.hdrs {
		descs[i], _ = parser.Parse(&w.hdrs[i], w.payloads[i])
	}
	firstTop, markers := -1, 0
	for i, d := range descs {
		if d.SpatialID == 2 && firstTop < 0 {
			firstTop = i
		}
		if w.hdrs[i].Marker && firstTop < 0 {
			markers++
		}
	}
	if firstTop < 0 {
		t.Fatal("the top layer never came back")
	}
	if firstTop < writtenBefore {
		t.Fatalf("spatial layer 2 written at packet %d while limited to 1", firstTop)
	}
	// The picture the top layer comes back in starts with a keyframe.
	for j := firstTop; j >= 0; j-- {
		if descs[j].Start {
			if !descs[j].Keyframe {
				t.Errorf("the top layer came back in a picture that is not a keyframe (packet %d)", j)
			}
			break
		}
	}
	if markers == 0 {
		t.Error("no marker while the top layer was dropped")
	}
}

// Without layer control a track writes every packet with its own sequence
// number, as before 016.
func TestLayerControl_OffUnchanged(t *testing.T) {
	layers := layerPackets(t, media.CodecVP8, media.ArrangementSimulcast)
	tr, w := boundTrack(t, media.CodecVP8, false)
	for _, p := range layers[0] {
		if err := tr.WriteRTP(p); err != nil {
			t.Fatal(err)
		}
	}
	if len(w.hdrs) != len(layers[0]) {
		t.Fatalf("%d of %d written", len(w.hdrs), len(layers[0]))
	}
	for i, h := range w.hdrs {
		if h.SequenceNumber != layers[0][i].SequenceNumber {
			t.Fatalf("sequence number changed at %d", i)
		}
	}
}
