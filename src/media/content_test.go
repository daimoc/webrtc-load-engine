package media

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestContent_Video(t *testing.T) {
	c, err := LoadContent(KindVideo)
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if c.Codec.MimeType != "video/VP8" || c.Codec.ClockRate != 90000 {
		t.Fatalf("unexpected codec %+v", c.Codec)
	}
	if n := len(c.Frames); n < 299 || n > 301 {
		t.Fatalf("expected 300 ±1 frames, got %d", n)
	}
	if !isVP8Keyframe(c.Frames[0].Data) {
		t.Fatal("first frame is not a keyframe")
	}
	if c.LoopDuration < 9900*time.Millisecond || c.LoopDuration > 10100*time.Millisecond {
		t.Fatalf("expected a 10 s loop, got %s", c.LoopDuration)
	}
	if c.Frames[1].Samples != 3000 {
		t.Fatalf("expected 3000 RTP ticks per frame at 30 fps, got %d", c.Frames[1].Samples)
	}
	if c.Width != 640 || c.Height != 360 {
		t.Fatalf("expected 640×360 from the IVF header, got %d×%d", c.Width, c.Height)
	}
}

func TestContent_Audio(t *testing.T) {
	c, err := LoadContent(KindAudio)
	if err != nil {
		t.Fatalf("LoadContent: %v", err)
	}
	if c.Codec.MimeType != "audio/opus" || c.Codec.ClockRate != 48000 || c.Codec.Channels != 2 {
		t.Fatalf("unexpected codec %+v", c.Codec)
	}
	if n := len(c.Frames); n < 499 || n > 501 {
		t.Fatalf("expected 500 ±1 packets, got %d", n)
	}
	for i, f := range c.Frames {
		if f.Duration != 20*time.Millisecond || f.Samples != 960 || len(f.Data) == 0 {
			t.Fatalf("packet %d: expected 20 ms / 960 samples, got %s / %d (%d bytes)", i, f.Duration, f.Samples, len(f.Data))
		}
	}
}

func TestContent_UnknownKind(t *testing.T) {
	if _, err := LoadContent("screen"); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}

func TestLoadLayers(t *testing.T) {
	for maxHeight, want := range map[int][]int{360: {90, 180, 360}, 720: {180, 360, 720}} {
		layers, err := LoadLayers(maxHeight)
		if err != nil {
			t.Fatalf("LoadLayers(%d): %v", maxHeight, err)
		}
		if len(layers) != 3 {
			t.Fatalf("LoadLayers(%d): %d layers", maxHeight, len(layers))
		}
		for i, c := range layers {
			_, h := keyframeSize(c.Frames[0].Data)
			if h != want[i] {
				t.Errorf("LoadLayers(%d) layer %d: height %d, want %d", maxHeight, i, h, want[i])
			}
			for j, f := range c.Frames {
				if f.Samples != layers[0].Frames[j].Samples {
					t.Fatalf("layer %d frame %d: %d ticks, base %d", i, j, f.Samples, layers[0].Frames[j].Samples)
				}
				if key := isVP8Keyframe(f.Data); key != (j%30 == 0) {
					t.Fatalf("layer %d frame %d: keyframe %v", i, j, key)
				}
			}
		}
	}
	if _, err := LoadLayers(480); err == nil {
		t.Fatal("LoadLayers(480) succeeded")
	}
}

func TestLoadVideo_VP8IsTodaysContent(t *testing.T) {
	single, err := LoadVideo(CodecVP8, ArrangementSingle, 0)
	if err != nil {
		t.Fatalf("LoadVideo: %v", err)
	}
	old, err := LoadContent(KindVideo)
	if err != nil {
		t.Fatal(err)
	}
	if len(single) != 1 || single[0] != old {
		t.Fatalf("LoadVideo(vp8, single) is not LoadContent(video)")
	}
	if single[0].VideoCodec != CodecVP8 || len(single[0].Heights) != 1 || single[0].Heights[0] != 360 {
		t.Fatalf("codec %q heights %v", single[0].VideoCodec, single[0].Heights)
	}
	layers, err := LoadVideo(CodecVP8, ArrangementSimulcast, 720)
	if err != nil {
		t.Fatal(err)
	}
	old720, err := LoadLayers(720)
	if err != nil {
		t.Fatal(err)
	}
	for i := range layers {
		if layers[i] != old720[i] {
			t.Fatalf("layer %d differs from LoadLayers(720)", i)
		}
	}
	for i, h := range []int{180, 360, 720} {
		if got := layers[i].Heights; len(got) != 1 || got[0] != h {
			t.Errorf("layer %d heights %v, want [%d]", i, got, h)
		}
	}
}

func TestLoadVideo_Errors(t *testing.T) {
	if _, err := LoadVideo("vp10", ArrangementSingle, 0); err == nil || !strings.Contains(err.Error(), "vp10") {
		t.Fatalf("unknown codec: %v", err)
	}
	if _, err := LoadVideo(CodecVP8, ArrangementSimulcast, 480); err == nil {
		t.Fatal("simulcast 480 accepted")
	}
	if _, err := LoadVideo(CodecVP8, ArrangementSVC, 720); err == nil || !strings.Contains(err.Error(), "svc") {
		t.Fatalf("VP8 SVC: %v", err)
	}
}

func TestLoadVideo_EveryCodec(t *testing.T) {
	vp8, err := LoadVideo(CodecVP8, ArrangementSimulcast, 720)
	if err != nil {
		t.Fatal(err)
	}
	ref := vp8[0]
	for _, c := range []VideoCodec{CodecVP9, CodecAV1, CodecH264} {
		for _, maxHeight := range []int{360, 720} {
			layers, err := LoadVideo(c, ArrangementSimulcast, maxHeight)
			if err != nil {
				t.Fatalf("%s simulcast %d: %v", c, maxHeight, err)
			}
			for _, v := range layers {
				name := string(c) + " " + strconv.Itoa(v.Heights[0])
				if v.VideoCodec != c || !strings.EqualFold(v.Codec.MimeType, c.MimeType()) || v.Codec.SDPFmtpLine != c.FmtpLine() {
					t.Fatalf("%s: codec %q %+v", name, v.VideoCodec, v.Codec)
				}
				if len(v.Frames) != len(ref.Frames) || v.LoopDuration != ref.LoopDuration {
					t.Fatalf("%s: %d frames over %s, VP8 %d over %s", name, len(v.Frames), v.LoopDuration, len(ref.Frames), ref.LoopDuration)
				}
				if v.Height != v.Heights[0] || v.Width != v.Height*16/9 {
					t.Errorf("%s: size %dx%d", name, v.Width, v.Height)
				}
				for i, f := range v.Frames {
					if f.Samples != ref.Frames[i].Samples || f.Duration != ref.Frames[i].Duration {
						t.Fatalf("%s frame %d: %d ticks / %s, VP8 %d / %s", name, i, f.Samples, f.Duration, ref.Frames[i].Samples, ref.Frames[i].Duration)
					}
					if key := isKeyframe(c, f.Data); key != (i%30 == 0) {
						t.Fatalf("%s frame %d: keyframe %v", name, i, key)
					}
					checkFrame(t, name, i, c, f.Data)
				}
			}
		}
		single, err := LoadVideo(c, ArrangementSingle, 0)
		if err != nil || len(single) != 1 || single[0].Heights[0] != 360 {
			t.Fatalf("%s single: %v", c, err)
		}
	}
}

// checkFrame checks a frame's form per codec.
func checkFrame(t *testing.T, name string, i int, c VideoCodec, data []byte) {
	t.Helper()
	switch c {
	case CodecAV1:
		types, err := obuTypes(data)
		if err != nil {
			t.Fatalf("%s frame %d: %v", name, i, err)
		}
		for _, ty := range types {
			if ty == obuTemporalDelimiter {
				t.Fatalf("%s frame %d: temporal delimiter kept", name, i)
			}
		}
		if i%30 == 0 && types[0] != obuSequenceHeader {
			t.Fatalf("%s frame %d: keyframe OBUs %v, want a sequence header first", name, i, types)
		}
	case CodecH264:
		types := nalTypes(data)
		for _, ty := range types {
			if ty == nalAUD {
				t.Fatalf("%s frame %d: access unit delimiter kept", name, i)
			}
		}
		if i%30 == 0 && (len(types) < 3 || types[0] != nalSPS || types[1] != nalPPS) {
			t.Fatalf("%s frame %d: keyframe NALs %v, want SPS, PPS first", name, i, types)
		}
	}
}

// svcTemporalIDs is L3T3's temporal pattern, picture i having
// svcTemporalIDs[i%4].
var svcTemporalIDs = [4]int{0, 2, 1, 2}

func TestLoadVideo_SVC(t *testing.T) {
	vp8, err := LoadVideo(CodecVP8, ArrangementSimulcast, 720)
	if err != nil {
		t.Fatal(err)
	}
	ref := vp8[0]
	for _, c := range []VideoCodec{CodecVP9, CodecAV1} {
		for _, maxHeight := range []int{360, 720} {
			name := string(c) + " svc " + strconv.Itoa(maxHeight)
			out, err := LoadVideo(c, ArrangementSVC, maxHeight)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if len(out) != 1 {
				t.Fatalf("%s: %d contents, want one stream", name, len(out))
			}
			v := out[0]
			want, _ := LayerHeights(maxHeight)
			if fmt.Sprint(v.Heights) != fmt.Sprint(want) || v.Height != maxHeight || v.Width != maxHeight*16/9 {
				t.Fatalf("%s: heights %v, size %dx%d", name, v.Heights, v.Width, v.Height)
			}
			if v.VideoCodec != c || len(v.Frames) != len(ref.Frames) || v.LoopDuration != ref.LoopDuration {
				t.Fatalf("%s: %s, %d pictures over %s, VP8 %d over %s", name, v.VideoCodec, len(v.Frames), v.LoopDuration, len(ref.Frames), ref.LoopDuration)
			}
			for i, f := range v.Frames {
				if f.Samples != ref.Frames[i].Samples || f.Duration != ref.Frames[i].Duration {
					t.Fatalf("%s picture %d: %d ticks / %s", name, i, f.Samples, f.Duration)
				}
				if len(f.Layers) != 3 {
					t.Fatalf("%s picture %d: %d layer frames", name, i, len(f.Layers))
				}
				if !bytes.Equal(f.Data, f.Layers[0].Data) {
					t.Fatalf("%s picture %d: Data is not the lowest layer frame", name, i)
				}
				for s, l := range f.Layers {
					if l.SpatialID != s || l.TemporalID != svcTemporalIDs[i%4] {
						t.Fatalf("%s picture %d layer %d: spatial %d temporal %d", name, i, s, l.SpatialID, l.TemporalID)
					}
					// L3T3_KEY: only the lowest layer's frame of a keyframe
					// picture is a keyframe; the others predict from it. A
					// keyframe every 20 pictures, so that each is in
					// temporal layer 0 (scripts/gen-media.sh svc_encode).
					if key := isKeyframe(c, l.Data); key != (s == 0 && i%20 == 0) {
						t.Fatalf("%s picture %d layer %d: keyframe %v", name, i, s, key)
					}
					if c == CodecAV1 {
						obus, err := splitOBUs(l.Data)
						if err != nil {
							t.Fatal(err)
						}
						// No temporal delimiter; a keyframe picture's lowest
						// layer frame starts with the sequence header.
						if (obus[0].typ() == obuSequenceHeader) != (s == 0 && i%20 == 0) {
							t.Fatalf("%s picture %d layer %d: first OBU type %d", name, i, s, obus[0].typ())
						}
						for _, o := range obus {
							if o.typ() == obuTemporalDelimiter {
								t.Fatalf("%s picture %d layer %d: temporal delimiter kept", name, i, s)
							}
							if (o.typ() == obuFrame || o.typ() == obuFrameHeader) && (o.spatialID() != s || o.temporalID() != l.TemporalID) {
								t.Fatalf("%s picture %d layer %d: OBU of spatial %d temporal %d", name, i, s, o.spatialID(), o.temporalID())
							}
						}
					}
				}
			}
		}
	}
}

func TestVP9Superframe(t *testing.T) {
	a, b := []byte{0x82, 1, 2}, []byte{0x86, 3, 4, 5, 6}
	// Index: marker 0b110 | bytes per size - 1 (0) << 3 | frames - 1 (1),
	// the sizes, the marker again.
	marker := byte(0xc0 | 0<<3 | 1)
	sf := append(append(append([]byte{}, a...), b...), marker, byte(len(a)), byte(len(b)), marker)
	got, err := splitVP9Superframe(sf)
	if err != nil || len(got) != 2 || !bytes.Equal(got[0], a) || !bytes.Equal(got[1], b) {
		t.Fatalf("split: %x, %v", got, err)
	}
	if got, err := splitVP9Superframe(a); err != nil || len(got) != 1 || !bytes.Equal(got[0], a) {
		t.Fatalf("plain frame: %x, %v", got, err)
	}
	bad := append(append([]byte{}, a...), marker, 9, 9, marker)
	if _, err := splitVP9Superframe(bad); err == nil {
		t.Fatal("sizes beyond the data accepted")
	}
}
