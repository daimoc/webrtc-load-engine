package media

import (
	"bytes"

	"github.com/pion/rtp"
)

// H.264 NAL unit types (ITU-T H.264 table 7-1).
const (
	nalSlice = 1
	nalIDR   = 5
	nalSEI   = 6
	nalSPS   = 7
	nalPPS   = 8
	nalAUD   = 9
)

var annexBStart = []byte{0, 0, 1}

// splitNALs splits an Annex B byte stream into NAL units, without their
// start codes.
func splitNALs(b []byte) [][]byte {
	var out [][]byte
	for {
		i := bytes.Index(b, annexBStart)
		if i < 0 {
			break
		}
		b = b[i+3:]
		j := bytes.Index(b, annexBStart)
		nal := b
		if j >= 0 {
			nal = b[:j]
			// A 4-byte start code leaves its leading zero on the NAL before.
			for len(nal) > 0 && nal[len(nal)-1] == 0 {
				nal = nal[:len(nal)-1]
			}
		}
		if len(nal) > 0 {
			out = append(out, nal)
		}
		if j < 0 {
			break
		}
		b = b[j:]
	}
	return out
}

func nalType(nal []byte) int { return int(nal[0] & 0x1f) }

// nalTypes lists the NAL unit types of an Annex B access unit.
func nalTypes(b []byte) []int {
	nals := splitNALs(b)
	out := make([]int, len(nals))
	for i, n := range nals {
		out[i] = nalType(n)
	}
	return out
}

// isH264Keyframe reports whether an access unit holds an IDR slice.
func isH264Keyframe(b []byte) bool {
	for _, n := range splitNALs(b) {
		if nalType(n) == nalIDR {
			return true
		}
	}
	return false
}

// h264AccessUnits splits the content's Annex B stream into access units at
// each access unit delimiter, which the delimiters are dropped from. Each
// access unit is written back in Annex B with 4-byte start codes.
func h264AccessUnits(stream []byte) [][]byte {
	var (
		out [][]byte
		cur []byte
	)
	for _, n := range splitNALs(stream) {
		if nalType(n) == nalAUD {
			if len(cur) > 0 {
				out = append(out, cur)
			}
			cur = nil
			continue
		}
		cur = append(cur, 0, 0, 0, 1)
		cur = append(cur, n...)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// H.264 RTP packet types (RFC 6184 5.4).
const (
	h264STAPA = 24
	h264FUA   = 28
)

// h264Parser reads H.264 packets in packetization mode 1 (single NAL
// units, STAP-A, FU-A). Every packet of a picture has its timestamp, and
// nothing else marks a picture's first packet (an IDR picture's first slice
// comes after its SPS, PPS and SEI): a picture starts at the first packet of
// a new timestamp. It is a keyframe when that packet holds an SPS or the
// start of an IDR slice; the size comes from the SPS.
type h264Parser struct {
	seen   bool
	lastTS uint32
}

func (hp *h264Parser) Parse(h *rtp.Header, p []byte) (FrameDesc, bool) {
	var d FrameDesc
	if len(p) < 2 {
		return d, false
	}
	look := func(nal []byte) {
		switch nalType(nal) {
		case nalSPS:
			d.Keyframe = true
			d.Width, d.Height = h264SPSSize(nal)
		case nalIDR:
			d.Keyframe = true
		}
	}
	switch t := int(p[0] & 0x1f); {
	case t >= 1 && t <= 23:
		look(p)
	case t == h264STAPA:
		rest := p[1:]
		for len(rest) >= 2 {
			n := int(rest[0])<<8 | int(rest[1])
			if n == 0 || len(rest) < 2+n {
				return d, false
			}
			look(rest[2 : 2+n])
			rest = rest[2+n:]
		}
	case t == h264FUA:
		fu := p[1]
		if fu&0x80 != 0 { // Start of the fragmented NAL unit
			look([]byte{p[0]&0xe0 | fu&0x1f})
		}
		d.End = fu&0x40 != 0
	default:
		return d, false
	}
	if h != nil {
		d.Start = !hp.seen || h.Timestamp != hp.lastTS
		hp.seen, hp.lastTS = true, h.Timestamp
	}
	// A keyframe's later IDR slices are in later packets: the keyframe is
	// told on the picture's first packet only.
	d.Keyframe = d.Keyframe && d.Start
	return d, true
}

// rbsp removes the emulation prevention bytes of a NAL unit's payload.
func rbsp(b []byte) []byte {
	out := make([]byte, 0, len(b))
	zeros := 0
	for _, c := range b {
		if zeros >= 2 && c == 3 {
			zeros = 0
			continue
		}
		if c == 0 {
			zeros++
		} else {
			zeros = 0
		}
		out = append(out, c)
	}
	return out
}

// ue reads an Exp-Golomb unsigned value (H.264 9.1).
func (r *bitReader) ue() int {
	zeros := 0
	for !r.flag() {
		if r.err != nil || zeros > 31 {
			return 0
		}
		zeros++
	}
	return (1<<zeros - 1) + int(r.bitsOr0(zeros))
}

func (r *bitReader) se() int {
	k := r.ue()
	if k%2 == 1 {
		return (k + 1) / 2
	}
	return -k / 2
}

// h264SPSSize reads the picture size of an SPS NAL unit (H.264 7.3.2.1.1),
// cropping applied; 0, 0 when it cannot (scaling matrices).
func h264SPSSize(nal []byte) (int, int) {
	if len(nal) < 4 {
		return 0, 0
	}
	r := bitReader{b: rbsp(nal[1:])}
	profile := int(r.bitsOr0(8))
	r.bitsOr0(16) // constraint flags, level_idc
	r.ue()        // seq_parameter_set_id
	chroma := 1
	switch profile {
	case 100, 110, 122, 244, 44, 83, 86, 118, 128, 138, 139, 134, 135:
		chroma = r.ue()
		if chroma == 3 {
			r.bitsOr0(1)
		}
		r.ue() // bit_depth_luma_minus8
		r.ue() // bit_depth_chroma_minus8
		r.bitsOr0(1)
		if r.flag() { // seq_scaling_matrix_present_flag
			return 0, 0
		}
	}
	r.ue()          // log2_max_frame_num_minus4
	switch r.ue() { // pic_order_cnt_type
	case 0:
		r.ue()
	case 1:
		r.bitsOr0(1)
		r.se()
		r.se()
		n := r.ue()
		for i := 0; i < n && r.err == nil; i++ {
			r.se()
		}
	}
	r.ue()       // max_num_ref_frames
	r.bitsOr0(1) // gaps_in_frame_num_value_allowed_flag
	wmbs := r.ue() + 1
	hmaps := r.ue() + 1
	frameMBsOnly := r.bitsOr0(1)
	if frameMBsOnly == 0 {
		r.bitsOr0(1)
	}
	r.bitsOr0(1) // direct_8x8_inference_flag
	w := wmbs * 16
	h := (2 - int(frameMBsOnly)) * hmaps * 16
	if r.flag() { // frame_cropping_flag
		cx, cy := 1, 2-int(frameMBsOnly)
		if chroma == 1 {
			cx, cy = 2, 2*(2-int(frameMBsOnly))
		} else if chroma == 2 {
			cx = 2
		}
		left, right, top, bottom := r.ue(), r.ue(), r.ue(), r.ue()
		w -= cx * (left + right)
		h -= cy * (top + bottom)
	}
	if r.err != nil {
		return 0, 0
	}
	return w, h
}
