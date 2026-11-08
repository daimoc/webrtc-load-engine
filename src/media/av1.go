package media

import (
	"errors"
	"fmt"

	"github.com/pion/rtp"
)

// AV1 OBU types (AV1 specification 6.2.2).
const (
	obuSequenceHeader         = 1
	obuTemporalDelimiter      = 2
	obuFrameHeader            = 3
	obuTileGroup              = 4
	obuMetadata               = 5
	obuFrame                  = 6
	obuRedundantFrameHdr      = 7
	obuTileList               = 8
	obuPadding                = 15
	obuHasSizeField      byte = 0x02
	obuHasExtension      byte = 0x04
)

// obu is one OBU of a temporal unit: its header (1 or 2 bytes, the size
// field flag cleared) and its payload.
type obu struct {
	header  []byte
	payload []byte
}

func (o obu) typ() int { return int(o.header[0]>>3) & 0x0f }

// spatialID and temporalID come from the extension header; 0 without one.
func (o obu) spatialID() int {
	if len(o.header) < 2 {
		return 0
	}
	return int(o.header[1]>>3) & 0x03
}

func (o obu) temporalID() int {
	if len(o.header) < 2 {
		return 0
	}
	return int(o.header[1] >> 5)
}

var errBadOBU = errors.New("malformed OBU")

// readLEB128 reads an unsigned LEB128 value (AV1 specification 4.10.5).
func readLEB128(b []byte) (v uint64, n int, err error) {
	for i := 0; i < 8 && i < len(b); i++ {
		v |= uint64(b[i]&0x7f) << (7 * i)
		if b[i]&0x80 == 0 {
			return v, i + 1, nil
		}
	}
	return 0, 0, errBadOBU
}

func appendLEB128(b []byte, v uint64) []byte {
	for {
		c := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b = append(b, c|0x80)
			continue
		}
		return append(b, c)
	}
}

// splitOBUs splits a temporal unit in the low overhead bitstream format
// (every OBU with its size field, as in IVF) into OBUs.
func splitOBUs(b []byte) ([]obu, error) {
	var out []obu
	for len(b) > 0 {
		h := b[0]
		if h&0x80 != 0 {
			return nil, fmt.Errorf("%w: forbidden bit set", errBadOBU)
		}
		hl := 1
		if h&obuHasExtension != 0 {
			hl = 2
		}
		if len(b) < hl {
			return nil, errBadOBU
		}
		header := append([]byte(nil), b[:hl]...)
		header[0] &^= obuHasSizeField
		rest := b[hl:]
		var size uint64
		if h&obuHasSizeField != 0 {
			v, n, err := readLEB128(rest)
			if err != nil {
				return nil, err
			}
			size, rest = v, rest[n:]
		} else {
			size = uint64(len(rest))
		}
		if size > uint64(len(rest)) {
			return nil, fmt.Errorf("%w: size %d beyond %d bytes", errBadOBU, size, len(rest))
		}
		out = append(out, obu{header: header, payload: rest[:size]})
		b = rest[size:]
	}
	return out, nil
}

// joinOBUs writes OBUs back in the low overhead format, with size fields.
func joinOBUs(obus []obu) []byte {
	var b []byte
	for _, o := range obus {
		h := append([]byte(nil), o.header...)
		h[0] |= obuHasSizeField
		b = append(b, h...)
		b = appendLEB128(b, uint64(len(o.payload)))
		b = append(b, o.payload...)
	}
	return b
}

// obuTypes lists the OBU types of a temporal unit.
func obuTypes(b []byte) ([]int, error) {
	obus, err := splitOBUs(b)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(obus))
	for i, o := range obus {
		out[i] = o.typ()
	}
	return out, nil
}

// isAV1Keyframe reports whether a temporal unit holds a key frame: a frame
// or frame header OBU whose uncompressed header starts with
// show_existing_frame 0 and frame_type KEY_FRAME (0). The content's
// sequence headers have reduced_still_picture_header 0.
func isAV1Keyframe(b []byte) bool {
	obus, err := splitOBUs(b)
	if err != nil {
		return false
	}
	for _, o := range obus {
		if (o.typ() == obuFrame || o.typ() == obuFrameHeader) && len(o.payload) > 0 {
			return o.payload[0]>>5 == 0
		}
	}
	return false
}

// av1FrameLayers returns the spatial and temporal layer of a layer frame:
// those of its frame (or frame header) OBU's extension header.
func av1FrameLayers(b []byte) (sid, tid int, err error) {
	obus, err := splitOBUs(b)
	if err != nil {
		return 0, 0, err
	}
	for _, o := range obus {
		if o.typ() == obuFrame || o.typ() == obuFrameHeader {
			if len(o.header) < 2 {
				return 0, 0, fmt.Errorf("AV1 layer frame without an OBU extension header")
			}
			return o.spatialID(), o.temporalID(), nil
		}
	}
	return 0, 0, fmt.Errorf("AV1 layer frame without a frame OBU")
}

// av1ContentFrame turns an IVF temporal unit into the content's frame: its
// OBUs without the temporal delimiter (the RTP payload format drops it).
func av1ContentFrame(tu []byte) ([]byte, error) {
	obus, err := splitOBUs(tu)
	if err != nil {
		return nil, err
	}
	kept := obus[:0]
	for _, o := range obus {
		if o.typ() != obuTemporalDelimiter {
			kept = append(kept, o)
		}
	}
	return joinOBUs(kept), nil
}

// av1Payloader packetizes AV1 temporal units (AV1 RTP specification 4):
// OBUs without their size field, each OBU element preceded by its length
// (W = 0), fragments marked with Z and Y, and N on the first packet of a
// coded video sequence (a temporal unit starting with a sequence header).
type av1Payloader struct{}

// Payload implements rtp.Payloader. frame holds OBUs in the low overhead
// format, without a temporal delimiter.
func (p *av1Payloader) Payload(mtu uint16, frame []byte) [][]byte {
	obus, err := splitOBUs(frame)
	if err != nil || len(obus) == 0 || mtu < 4 {
		return nil
	}
	var (
		out [][]byte
		cur = []byte{0}
	)
	if obus[0].typ() == obuSequenceHeader {
		cur[0] |= 0x08 // N
	}
	for _, o := range obus {
		data := append(append([]byte(nil), o.header...), o.payload...)
		for len(data) > 0 {
			room := int(mtu) - len(cur)
			if room < 2 {
				out = append(out, cur)
				cur = []byte{0}
				continue
			}
			n := min(len(data), room-1)
			for n > 0 && lebLen(n)+n > room {
				n--
			}
			cur = appendLEB128(cur, uint64(n))
			cur = append(cur, data[:n]...)
			data = data[n:]
			if len(data) > 0 {
				cur[0] |= 0x40 // Y: continues in the next packet
				out = append(out, cur)
				cur = []byte{0x80} // Z: continues from the previous packet
			}
		}
	}
	if len(cur) > 1 {
		out = append(out, cur)
	}
	return out
}

func lebLen(v int) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

// av1Parser reads AV1 packets (AV1 RTP specification 4) and, when it was
// negotiated, the dependency descriptor, keeping the last structure it
// carried. A keyframe's first packet starts with a sequence header OBU;
// its size comes from the descriptor's structure, else from the sequence
// header. Without the descriptor nothing marks a frame's start (the
// timestamp change does) nor its layer.
type av1Parser struct {
	ddExtID   uint8
	structure *DDStructure
	// layerHeights are the run's spatial layer heights, for a scalable
	// stream received without the dependency descriptor (Janus does not
	// forward it): the layer then comes from the OBU extension header.
	layerHeights []int
}

func (a *av1Parser) setLayerHeights(h []int) { a.layerHeights = append([]int(nil), h...) }

func (a *av1Parser) Parse(h *rtp.Header, p []byte) (FrameDesc, bool) {
	var d FrameDesc
	if len(p) < 2 {
		return d, false
	}
	z := p[0]&0x80 != 0
	if !z {
		if obu, ok := firstOBUElement(p); ok && len(obu) > 0 && int(obu[0]>>3)&0x0f == obuSequenceHeader {
			d.Keyframe = true
			d.Width, d.Height = av1SequenceSize(obu)
		}
	}
	if a.ddExtID != 0 && h != nil {
		if ext := h.GetExtension(a.ddExtID); ext != nil {
			dd, err := ParseDD(ext, a.structure)
			if err == nil {
				if dd.Structure != nil {
					a.structure = dd.Structure
				}
				l := dd.Layer(a.structure)
				d.SpatialID, d.TemporalID = l.SpatialID, l.TemporalID
				d.Start = dd.Start && l.SpatialID == 0
				d.End = dd.End
				if a.structure != nil && l.SpatialID < len(a.structure.Resolutions) {
					d.LayerHeight = a.structure.Resolutions[l.SpatialID][1]
				}
				if d.Keyframe && a.structure != nil && l.SpatialID < len(a.structure.Resolutions) {
					top := a.structure.Resolutions[len(a.structure.Resolutions)-1]
					d.Width, d.Height = top[0], top[1]
				}
			}
		}
	}
	if !d.Start {
		d.Keyframe = d.Keyframe && (a.ddExtID == 0 || h == nil || h.GetExtension(a.ddExtID) == nil)
	}
	if len(a.layerHeights) > 0 && a.structure == nil {
		// Without the descriptor: the layer frame's spatial layer from the
		// first OBU's extension header, when the packet starts one. The
		// sequence header's size is the top layer's, not the picture's.
		d.Width, d.Height = 0, 0
		if obu, ok := firstOBUElement(p); !z && ok && len(obu) > 1 && obu[0]&0x04 != 0 {
			sid := int(obu[1]>>3) & 0x03
			d.SpatialID, d.TemporalID = sid, int(obu[1]>>5)
			if sid < len(a.layerHeights) {
				d.LayerHeight = a.layerHeights[sid]
			}
		}
	}
	return d, true
}

// firstOBUElement returns the first OBU element of a packet (without its
// length field), or false when the packet has none.
func firstOBUElement(p []byte) ([]byte, bool) {
	w := int(p[0]>>4) & 0x03
	rest := p[1:]
	if w == 1 {
		return rest, true
	}
	n, l, err := readLEB128(rest)
	if err != nil || uint64(len(rest)-l) < n {
		return nil, false
	}
	return rest[l : l+int(n)], true
}

// av1SequenceSize reads max_frame_width and max_frame_height of a sequence
// header OBU (AV1 specification 5.5), or 0, 0 for forms the bots do not
// send (timing info, reduced still pictures).
func av1SequenceSize(obu []byte) (int, int) {
	hl := 1
	if obu[0]&obuHasExtension != 0 {
		hl = 2
	}
	if obu[0]&obuHasSizeField != 0 {
		_, n, err := readLEB128(obu[hl:])
		if err != nil {
			return 0, 0
		}
		hl += n
	}
	if len(obu) <= hl {
		return 0, 0
	}
	r := bitReader{b: obu[hl:]}
	r.bitsOr0(3)  // seq_profile
	r.bitsOr0(1)  // still_picture
	if r.flag() { // reduced_still_picture_header
		return 0, 0
	}
	if r.flag() { // timing_info_present_flag
		return 0, 0
	}
	initialDisplayDelay := r.flag()
	ops := int(r.bitsOr0(5)) + 1
	for i := 0; i < ops; i++ {
		r.bitsOr0(12) // operating_point_idc
		if r.bitsOr0(5) > 7 {
			r.bitsOr0(1) // seq_tier
		}
		if initialDisplayDelay && r.flag() {
			r.bitsOr0(4)
		}
	}
	wb := int(r.bitsOr0(4)) + 1
	hb := int(r.bitsOr0(4)) + 1
	w := int(r.bitsOr0(wb)) + 1
	h := int(r.bitsOr0(hb)) + 1
	if r.err != nil {
		return 0, 0
	}
	return w, h
}
