package media

import (
	"reflect"
	"testing"
)

func TestDD_SingleLayerRoundTrip(t *testing.T) {
	s := DDSingleLayer(640, 360)
	key := DD{Start: true, End: false, TemplateID: 0, FrameNumber: 1, Structure: &s}
	b := WriteDD(key)
	if len(b) <= 3 {
		t.Fatalf("a descriptor with its structure is %d bytes", len(b))
	}
	got, err := ParseDD(b, nil)
	if err != nil {
		t.Fatalf("ParseDD: %v", err)
	}
	if !got.Start || got.End || got.TemplateID != 0 || got.FrameNumber != 1 || got.Structure == nil {
		t.Fatalf("parsed %+v", got)
	}
	if !reflect.DeepEqual(*got.Structure, s) {
		t.Fatalf("structure\n got %+v\nwant %+v", *got.Structure, s)
	}

	delta := WriteDD(DD{End: true, TemplateID: 1, FrameNumber: 2})
	if len(delta) != 3 {
		t.Fatalf("a descriptor without extended fields is %d bytes, want 3", len(delta))
	}
	d, err := ParseDD(delta, got.Structure)
	if err != nil {
		t.Fatal(err)
	}
	if d.Start || !d.End || d.TemplateID != 1 || d.FrameNumber != 2 {
		t.Fatalf("parsed %+v", d)
	}
	if l := d.Layer(got.Structure); l.SpatialID != 0 || l.TemporalID != 0 {
		t.Fatalf("layer %+v", l)
	}
}

// The mandatory fields are start_of_frame, end_of_frame, a 6-bit template
// ID and a 16-bit frame number, most significant bit first (AV1 RTP
// specification A.8.1).
func TestDD_MandatoryFieldsLayout(t *testing.T) {
	b := WriteDD(DD{Start: true, End: true, TemplateID: 5, FrameNumber: 0x1234})
	want := []byte{0x80 | 0x40 | 5, 0x12, 0x34}
	if !reflect.DeepEqual(b, want) {
		t.Fatalf("% x, want % x", b, want)
	}
}

func TestDD_FrameNumberWraps(t *testing.T) {
	s := DDSingleLayer(160, 90)
	for n := 0; n < 1<<16; n += 4099 {
		b := WriteDD(DD{Start: n%2 == 0, End: n%3 == 0, TemplateID: 1, FrameNumber: uint16(n)})
		d, err := ParseDD(b, &s)
		if err != nil || d.FrameNumber != uint16(n) || d.Start != (n%2 == 0) || d.End != (n%3 == 0) {
			t.Fatalf("frame %d: %+v, %v", n, d, err)
		}
	}
}

func TestDD_NoStructure(t *testing.T) {
	if _, err := ParseDD([]byte{0x80, 0, 1}, nil); err == nil {
		t.Fatal("a descriptor without a structure parsed with none known")
	}
	if _, err := ParseDD([]byte{0x80}, nil); err == nil {
		t.Fatal("a 1-byte descriptor parsed")
	}
}

func TestNonSymmetric(t *testing.T) {
	for n := 1; n <= 33; n++ {
		for v := 0; v < n; v++ {
			var w bitWriter
			w.ns(n, v)
			r := bitReader{b: w.bytes()}
			got, err := r.ns(n)
			if err != nil || got != v {
				t.Fatalf("ns(%d) %d -> %d, %v", n, v, got, err)
			}
		}
	}
}

// TestDD_L3T3Key checks the L3T3_KEY structure against the layer frames
// the SVC content holds, numbered 3p+s for picture p, spatial layer s, a
// keyframe every 20 pictures and temporal layers 0-2-1-2. Each frame's
// template must carry the frame's layers, the frame number differences of
// what it predicts from, its distance to the previous frame of each chain,
// and be present in exactly the decode targets that need it.
func TestDD_L3T3Key(t *testing.T) {
	s := DDL3T3Key([]int{320, 640, 1280}, []int{180, 360, 720})
	if s.DecodeTargets != 9 || s.Chains != 3 || len(s.ProtectedBy) != 9 {
		t.Fatalf("structure: %d decode targets, %d chains, protected by %v", s.DecodeTargets, s.Chains, s.ProtectedBy)
	}
	for dt, c := range s.ProtectedBy {
		if c != dt/3 {
			t.Fatalf("decode target %d protected by chain %d, want %d", dt, c, dt/3)
		}
	}
	b := WriteDD(DD{Start: true, Structure: &s})
	got, err := ParseDD(b, nil)
	if err != nil || !reflect.DeepEqual(*got.Structure, s) {
		t.Fatalf("round trip: %v\n got %+v\nwant %+v", err, got.Structure, s)
	}

	tids := [4]int{0, 2, 1, 2}
	type frame struct {
		n, p, sid, tid int
		key            bool // On a keyframe picture
	}
	var frames []frame
	for p := 0; p < 41; p++ {
		for sid := 0; sid < 3; sid++ {
			frames = append(frames, frame{3*p + sid, p, sid, tids[p%4], p%20 == 0})
		}
	}
	// refs: what a frame predicts from (L3T3_KEY).
	refs := func(f frame) []int {
		switch {
		case f.key && f.sid == 0:
			return nil
		case f.key:
			return []int{f.n - 1} // The layer below, same picture
		case f.tid == 0:
			return []int{f.n - 12} // The T0 of its layer 4 pictures back
		case f.tid == 1:
			return []int{f.n - 6}
		default:
			return []int{f.n - 3} // The picture before
		}
	}
	// inChain: chain c goes through the T0 frames of layer c and, on a
	// keyframe picture, the frames of the layers below it.
	inChain := func(f frame, c int) bool { return f.tid == 0 && (f.sid == c || (f.key && f.sid < c)) }
	// inTarget: decode target (ds, dt) needs a frame of its layer up to
	// temporal layer dt, and on a keyframe picture the layers below.
	inTarget := func(f frame, ds, dt int) bool { return (f.sid == ds && f.tid <= dt) || (f.key && f.sid < ds) }

	for _, f := range frames {
		id := ddL3T3KeyTemplate(f.key, f.p, f.sid)
		tpl := s.Templates[id]
		name := "frame " + itoa(f.n)
		if tpl.SpatialID != f.sid || tpl.TemporalID != f.tid {
			t.Fatalf("%s: template %d of layer S%dT%d, frame is S%dT%d", name, id, tpl.SpatialID, tpl.TemporalID, f.sid, f.tid)
		}
		if !reflect.DeepEqual(tpl.FDiffs, diffsOf(f.n, refs(f))) {
			t.Fatalf("%s: template %d fdiffs %v, want %v", name, id, tpl.FDiffs, diffsOf(f.n, refs(f)))
		}
		for c := 0; c < 3; c++ {
			want := 0 // A keyframe picture's first frame restarts every chain
			if !(f.key && f.sid == 0) {
				for _, g := range frames {
					if g.n < f.n && inChain(g, c) && (g.p >= f.p-f.p%20) {
						want = f.n - g.n
					}
				}
			}
			if tpl.ChainFDiffs[c] != want {
				t.Fatalf("%s: template %d chain %d diff %d, want %d", name, id, c, tpl.ChainFDiffs[c], want)
			}
		}
		for dt := 0; dt < 9; dt++ {
			if present := tpl.DTIs[dt] != dtiNotPresent; present != inTarget(f, dt/3, dt%3) {
				t.Fatalf("%s: template %d DTI %d for decode target %d", name, id, tpl.DTIs[dt], dt)
			}
		}
	}
}

func diffsOf(n int, refs []int) []int {
	var out []int
	for _, r := range refs {
		out = append(out, n-r)
	}
	return out
}
