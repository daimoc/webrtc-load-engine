package media

import (
	"errors"
	"fmt"
)

// The dependency descriptor RTP header extension (AV1 RTP specification,
// appendix A): the frame's template, number and, on keyframes, the
// template dependency structure that says which layers templates belong to.
// Only what the bots send is written: fixed structures, no custom DTIs,
// frame differences or chains, no active decode targets mask.

// Decode target indications (A.8.3).
const (
	dtiNotPresent  = 0
	dtiDiscardable = 1
	dtiSwitch      = 2
	dtiRequired    = 3
)

// DDTemplate is one frame dependency template.
type DDTemplate struct {
	SpatialID, TemporalID int
	DTIs                  []int // One per decode target
	FDiffs                []int // Frame number differences of the frames it depends on
	ChainFDiffs           []int // One per chain
}

// DDStructure is a template dependency structure.
type DDStructure struct {
	TemplateIDOffset int
	DecodeTargets    int
	Templates        []DDTemplate
	Chains           int
	ProtectedBy      []int    // Chain protecting each decode target, when Chains > 0
	Resolutions      [][2]int // Width and height per spatial layer; nil when absent
}

// DD is one packet's dependency descriptor.
type DD struct {
	Start, End  bool
	TemplateID  int
	FrameNumber uint16
	Structure   *DDStructure // Attached on keyframes' first packet
}

// DDLayer is the layer a descriptor's frame belongs to.
type DDLayer struct{ SpatialID, TemporalID int }

// Layer returns the layer of the frame, from its template in s.
func (d DD) Layer(s *DDStructure) DDLayer {
	if s == nil {
		return DDLayer{}
	}
	i := (d.TemplateID + 64 - s.TemplateIDOffset) % 64
	if i >= len(s.Templates) {
		return DDLayer{}
	}
	t := s.Templates[i]
	return DDLayer{t.SpatialID, t.TemporalID}
}

// DDSingleLayer is the structure of a stream without layers, as libwebrtc
// writes it for L1T1: a keyframe template and a delta frame template
// depending on the previous frame, one decode target and one chain.
func DDSingleLayer(width, height int) DDStructure {
	return DDStructure{
		DecodeTargets: 1,
		Templates: []DDTemplate{
			{DTIs: []int{dtiSwitch}, ChainFDiffs: []int{0}},
			{DTIs: []int{dtiSwitch}, FDiffs: []int{1}, ChainFDiffs: []int{1}},
		},
		Chains:      1,
		ProtectedBy: []int{0},
		Resolutions: [][2]int{{width, height}},
	}
}

// DDL3T3Key is the structure of the scalable content (L3T3_KEY): 3
// spatial and 3 temporal layers (0-2-1-2), each spatial layer predicting
// only from itself except on keyframe pictures, where it predicts from the
// layer below. Decode target 3s+t is spatial layer s up to temporal layer
// t, protected by chain s, which goes through the T0 frames of layer s and,
// on keyframe pictures, of the layers below. Frame numbers go up by one per
// layer frame (3 per picture). Per spatial layer the templates are: the
// keyframe picture's frame, T0, T1, then T2 after a T0 and T2 after a T1
// (they differ in their chain distances). Template order follows
// template_layers: by spatial, then temporal layer.
func DDL3T3Key(widths, heights []int) DDStructure {
	s := DDStructure{DecodeTargets: 9, Chains: 3}
	for dt := 0; dt < 9; dt++ {
		s.ProtectedBy = append(s.ProtectedBy, dt/3)
	}
	// dtis returns the indications of a frame of layer sid, tid on the 9
	// decode targets: L1T3's "SSS", "-DS" or "--D" on its own layer's
	// targets; on a keyframe picture "R" on the upper layers' targets.
	dtis := func(sid, tid int, key bool) []int {
		own := [3][3]int{
			{dtiSwitch, dtiSwitch, dtiSwitch},
			{dtiNotPresent, dtiDiscardable, dtiSwitch},
			{dtiNotPresent, dtiNotPresent, dtiDiscardable},
		}[tid]
		out := make([]int, 9)
		for ds := 0; ds < 3; ds++ {
			for dt := 0; dt < 3; dt++ {
				switch {
				case ds == sid:
					out[3*ds+dt] = own[dt]
				case key && ds > sid:
					out[3*ds+dt] = dtiRequired
				}
			}
		}
		return out
	}
	for sid := 0; sid < 3; sid++ {
		// Chain distances of a T1 or T2 frame: to the latest T0 frame of
		// each layer, in the picture that began back layer frames before
		// this picture; chain c's frame there is layer c's.
		chains := func(back int) []int {
			out := make([]int, 3)
			for c := range out {
				out[c] = back + sid - c
			}
			return out
		}
		key := DDTemplate{SpatialID: sid, DTIs: dtis(sid, 0, true)}
		if sid == 0 {
			key.ChainFDiffs = []int{0, 0, 0}
		} else {
			key.FDiffs = []int{1}
			key.ChainFDiffs = make([]int, 3)
			for c := range key.ChainFDiffs {
				// Chain c's latest frame: the layer below for c >= sid,
				// layer c itself for c < sid.
				key.ChainFDiffs[c] = max(sid-c, 1)
			}
		}
		s.Templates = append(s.Templates,
			key,
			DDTemplate{SpatialID: sid, DTIs: dtis(sid, 0, false), FDiffs: []int{12}, ChainFDiffs: t0Chains(sid)},
			DDTemplate{SpatialID: sid, TemporalID: 1, DTIs: dtis(sid, 1, false), FDiffs: []int{6}, ChainFDiffs: chains(6)},
			DDTemplate{SpatialID: sid, TemporalID: 2, DTIs: dtis(sid, 2, false), FDiffs: []int{3}, ChainFDiffs: chains(3)},
			DDTemplate{SpatialID: sid, TemporalID: 2, DTIs: dtis(sid, 2, false), FDiffs: []int{3}, ChainFDiffs: chains(9)},
		)
	}
	for i := range widths {
		s.Resolutions = append(s.Resolutions, [2]int{widths[i], heights[i]})
	}
	return s
}

// t0Chains is the chain distances of a T0 frame of layer sid on a delta
// picture: the lower layers' T0 frames of the same picture, its own layer's
// and the upper layers' of four pictures back.
func t0Chains(sid int) []int {
	out := make([]int, 3)
	for c := range out {
		if c < sid {
			out[c] = sid - c
		} else {
			out[c] = 12 + sid - c
		}
	}
	return out
}

// ddL3T3KeyTemplate returns the DDL3T3Key template of the layer frame of
// spatial layer sid of picture p (p counts from a picture whose temporal
// layer is 0; key: a keyframe picture).
func ddL3T3KeyTemplate(key bool, p, sid int) int {
	i := 5 * sid
	switch {
	case key:
	case p%4 == 0:
		i++
	case p%4 == 2:
		i += 2
	case p%4 == 1:
		i += 3
	default:
		i += 4
	}
	return i
}

// WriteDD writes a descriptor: the mandatory fields only, unless it
// carries a structure.
func WriteDD(d DD) []byte {
	var w bitWriter
	w.flag(d.Start)
	w.flag(d.End)
	w.bits(uint64(d.TemplateID), 6)
	w.bits(uint64(d.FrameNumber), 16)
	if d.Structure != nil {
		w.flag(true)  // template_dependency_structure_present_flag
		w.flag(false) // active_decode_targets_present_flag
		w.flag(false) // custom_dtis_flag
		w.flag(false) // custom_fdiffs_flag
		w.flag(false) // custom_chains_flag
		writeStructure(&w, d.Structure)
	}
	return w.bytes()
}

func writeStructure(w *bitWriter, s *DDStructure) {
	w.bits(uint64(s.TemplateIDOffset), 6)
	w.bits(uint64(s.DecodeTargets-1), 5)
	// template_layers: next_layer_idc after each template.
	for i, t := range s.Templates {
		switch {
		case i == len(s.Templates)-1:
			w.bits(3, 2)
		case s.Templates[i+1].SpatialID == t.SpatialID && s.Templates[i+1].TemporalID == t.TemporalID:
			w.bits(0, 2)
		case s.Templates[i+1].SpatialID == t.SpatialID:
			w.bits(1, 2) // Next temporal layer
		default:
			w.bits(2, 2) // Next spatial layer, temporal layer 0
		}
	}
	for _, t := range s.Templates {
		for _, dti := range t.DTIs {
			w.bits(uint64(dti), 2)
		}
	}
	for _, t := range s.Templates {
		for _, f := range t.FDiffs {
			w.flag(true)
			w.bits(uint64(f-1), 4)
		}
		w.flag(false)
	}
	w.ns(s.DecodeTargets+1, s.Chains)
	if s.Chains > 0 {
		for _, c := range s.ProtectedBy {
			w.ns(s.Chains, c)
		}
		for _, t := range s.Templates {
			for _, f := range t.ChainFDiffs {
				w.bits(uint64(f), 4)
			}
		}
	}
	w.flag(s.Resolutions != nil)
	for _, r := range s.Resolutions {
		w.bits(uint64(r[0]-1), 16)
		w.bits(uint64(r[1]-1), 16)
	}
}

var errDD = errors.New("dependency descriptor")

// ParseDD reads a descriptor. known is the structure in force (the last one
// received); a descriptor carrying its own structure does not need one.
func ParseDD(b []byte, known *DDStructure) (DD, error) {
	if len(b) < 3 {
		return DD{}, fmt.Errorf("%w: %d bytes", errDD, len(b))
	}
	r := bitReader{b: b}
	var d DD
	d.Start = r.flag()
	d.End = r.flag()
	d.TemplateID = int(r.bitsOr0(6))
	d.FrameNumber = uint16(r.bitsOr0(16))
	s := known
	if len(b) > 3 {
		present := r.flag()
		activePresent := r.flag()
		customDTIs, customFDiffs, customChains := r.flag(), r.flag(), r.flag()
		if present {
			parsed, err := readStructure(&r)
			if err != nil {
				return DD{}, err
			}
			d.Structure = parsed
			s = parsed
		}
		if s == nil {
			return DD{}, fmt.Errorf("%w: no structure", errDD)
		}
		if activePresent {
			r.bitsOr0(s.DecodeTargets)
		}
		// Custom fields are not sent by the bots; a frame using them is
		// still placed by its template.
		_ = customDTIs || customFDiffs || customChains
	}
	if s == nil {
		return DD{}, fmt.Errorf("%w: no structure", errDD)
	}
	if r.err != nil {
		return DD{}, r.err
	}
	return d, nil
}

func readStructure(r *bitReader) (*DDStructure, error) {
	s := &DDStructure{}
	s.TemplateIDOffset = int(r.bitsOr0(6))
	s.DecodeTargets = int(r.bitsOr0(5)) + 1
	sid, tid := 0, 0
	for {
		s.Templates = append(s.Templates, DDTemplate{SpatialID: sid, TemporalID: tid})
		idc := r.bitsOr0(2)
		if r.err != nil || len(s.Templates) > 64 {
			return nil, fmt.Errorf("%w: template layers", errDD)
		}
		if idc == 3 {
			break
		}
		switch idc {
		case 1:
			tid++
		case 2:
			tid, sid = 0, sid+1
		}
	}
	for i := range s.Templates {
		s.Templates[i].DTIs = make([]int, s.DecodeTargets)
		for dt := range s.Templates[i].DTIs {
			s.Templates[i].DTIs[dt] = int(r.bitsOr0(2))
		}
	}
	for i := range s.Templates {
		for r.flag() {
			s.Templates[i].FDiffs = append(s.Templates[i].FDiffs, int(r.bitsOr0(4))+1)
		}
	}
	chains, err := r.ns(s.DecodeTargets + 1)
	if err != nil {
		return nil, err
	}
	s.Chains = chains
	if chains > 0 {
		s.ProtectedBy = make([]int, s.DecodeTargets)
		for dt := range s.ProtectedBy {
			if s.ProtectedBy[dt], err = r.ns(chains); err != nil {
				return nil, err
			}
		}
		for i := range s.Templates {
			s.Templates[i].ChainFDiffs = make([]int, chains)
			for c := range s.Templates[i].ChainFDiffs {
				s.Templates[i].ChainFDiffs[c] = int(r.bitsOr0(4))
			}
		}
	}
	if r.flag() {
		spatial := s.Templates[len(s.Templates)-1].SpatialID + 1
		for i := 0; i < spatial; i++ {
			w := int(r.bitsOr0(16)) + 1
			h := int(r.bitsOr0(16)) + 1
			s.Resolutions = append(s.Resolutions, [2]int{w, h})
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return s, nil
}

// bitWriter writes bits most significant first.
type bitWriter struct {
	b []byte
	n int // Bits written
}

func (w *bitWriter) bits(v uint64, n int) {
	for i := n - 1; i >= 0; i-- {
		if w.n%8 == 0 {
			w.b = append(w.b, 0)
		}
		if v>>uint(i)&1 == 1 {
			w.b[len(w.b)-1] |= 0x80 >> uint(w.n%8)
		}
		w.n++
	}
}

func (w *bitWriter) flag(v bool) {
	if v {
		w.bits(1, 1)
	} else {
		w.bits(0, 1)
	}
}

// ns writes v (0 <= v < n) in the non-symmetric unsigned encoding (A.3).
func (w *bitWriter) ns(n, v int) {
	width := 0
	for x := n; x != 0; x >>= 1 {
		width++
	}
	m := (1 << width) - n
	if v < m {
		w.bits(uint64(v), width-1)
		return
	}
	w.bits(uint64((v+m)>>1), width-1)
	w.bits(uint64((v+m)&1), 1)
}

func (w *bitWriter) bytes() []byte { return w.b } // Zero padded to a byte

// bitReader reads bits most significant first; past the end it sets err.
type bitReader struct {
	b   []byte
	n   int
	err error
}

func (r *bitReader) bitsOr0(n int) uint64 {
	var v uint64
	for i := 0; i < n; i++ {
		if r.n >= len(r.b)*8 {
			if r.err == nil {
				r.err = fmt.Errorf("%w: truncated", errDD)
			}
			return 0
		}
		v = v<<1 | uint64(r.b[r.n/8]>>(7-uint(r.n%8))&1)
		r.n++
	}
	return v
}

func (r *bitReader) flag() bool { return r.bitsOr0(1) == 1 }

func (r *bitReader) ns(n int) (int, error) {
	width := 0
	for x := n; x != 0; x >>= 1 {
		width++
	}
	m := (1 << width) - n
	v := int(r.bitsOr0(width - 1))
	if v >= m {
		v = (v << 1) - m + int(r.bitsOr0(1))
	}
	return v, r.err
}
