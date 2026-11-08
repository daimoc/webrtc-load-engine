package media

import "github.com/pion/rtp"

// isVP9Keyframe reports whether a VP9 frame is a key frame (VP9 bitstream
// specification 6.2): frame_marker, profile bits, show_existing_frame 0,
// then frame_type 0.
func isVP9Keyframe(f []byte) bool {
	if len(f) == 0 || f[0]>>6 != 2 {
		return false
	}
	// The profile is 2 bits, low bit first; profile 3 adds a reserved bit.
	bit := 2
	profile := int(f[0]>>5&1) | int(f[0]>>4&1)<<1
	bit += 2
	if profile == 3 {
		bit++
	}
	if f[0]>>(7-bit)&1 == 1 { // show_existing_frame
		return false
	}
	bit++
	return f[0]>>(7-bit)&1 == 0
}

// vp9Desc is what the VP9 payload descriptor says of one layer frame
// (RFC 9628 4.2, flexible mode).
type vp9Desc struct {
	interPicture bool    // P: predicted from an earlier picture
	layered      bool    // L: layer indices present
	tid, sid     int     // Temporal and spatial layer
	switchUp     bool    // U: switching up point
	interLayer   bool    // D: predicted from the spatial layer below
	notRefUpper  bool    // Z: not a reference for upper spatial layers
	pdiffs       []uint8 // Reference picture ID differences, when P
	ss           bool    // V: the scalability structure follows (keyframes)
}

// vp9Payloader packetizes VP9 frames with the flexible-mode payload
// descriptor and a 15-bit picture ID, the scalability structure on
// keyframes giving each spatial layer's size.
type vp9Payloader struct {
	pictureID       uint16 // 15 bits, one per picture (all its layers)
	widths, heights []int  // Per spatial layer, low to high
}

func newVP9Payloader(width, height int) *vp9Payloader {
	return &vp9Payloader{widths: []int{width}, heights: []int{height}}
}

// Payload implements rtp.Payloader for a single-layer stream: each frame is
// one picture, predicted from the previous one unless it is a keyframe.
func (p *vp9Payloader) Payload(mtu uint16, frame []byte) [][]byte {
	key := isVP9Keyframe(frame)
	d := vp9Desc{interPicture: !key, ss: key}
	if !key {
		d.pdiffs = []uint8{1}
	}
	out := p.layerFrame(int(mtu), frame, d)
	p.nextPicture()
	return out
}

// newVP9SVCPayloader packetizes scalable pictures whose spatial layers have
// these sizes, low to high.
func newVP9SVCPayloader(widths, heights []int) *vp9Payloader {
	return &vp9Payloader{widths: append([]int(nil), widths...), heights: append([]int(nil), heights...)}
}

// vp9PDiffs is the picture distance to the reference of a layer frame of
// each temporal layer, in the content's 0-2-1-2 pattern (L3T3_KEY): T0
// predicts from the T0 four pictures back, T1 from the T0 two back, T2
// from the picture before.
var vp9PDiffs = [3]uint8{4, 2, 1}

// PayloadLayers packetizes one scalable picture (L3T3_KEY): one packet
// list per layer frame, all with the picture's ID. On a keyframe picture
// the lowest layer is intra and each upper layer predicts from the one
// below (D); otherwise each layer predicts from its own earlier frame (P)
// and is no reference for the upper layers (Z). The scalability structure
// is in the first packet of a keyframe picture. U is set on every frame:
// in the 0-2-1-2 pattern no frame depends on an earlier one of a higher
// temporal layer.
func (p *vp9Payloader) PayloadLayers(mtu uint16, layers []LayerFrame) [][][]byte {
	key := len(layers) > 0 && isVP9Keyframe(layers[0].Data)
	top := len(layers) - 1
	out := make([][][]byte, 0, len(layers))
	for s, l := range layers {
		d := vp9Desc{
			layered:      true,
			tid:          l.TemporalID,
			sid:          l.SpatialID,
			switchUp:     true,
			interPicture: !key,
			interLayer:   key && s > 0,
			notRefUpper:  !key || s == top,
			ss:           key && s == 0,
		}
		if !key && l.TemporalID < len(vp9PDiffs) {
			d.pdiffs = []uint8{vp9PDiffs[l.TemporalID]}
		}
		out = append(out, p.layerFrame(int(mtu), l.Data, d))
	}
	p.nextPicture()
	return out
}

func (p *vp9Payloader) nextPicture() { p.pictureID = (p.pictureID + 1) & 0x7fff }

// layerFrame packetizes one layer frame of the current picture.
func (p *vp9Payloader) layerFrame(mtu int, frame []byte, d vp9Desc) [][]byte {
	var out [][]byte
	for first := true; first || len(frame) > 0; first = false {
		h := p.descriptor(d, first)
		room := mtu - len(h)
		if room <= 0 {
			return nil
		}
		n := min(room, len(frame))
		b := make([]byte, len(h), len(h)+n)
		copy(b, h)
		b = append(b, frame[:n]...)
		frame = frame[n:]
		if len(frame) == 0 {
			b[0] |= 0x04 // E
		}
		out = append(out, b)
	}
	return out
}

// descriptor writes the payload descriptor of one packet; E is set by the
// caller on the last one.
func (p *vp9Payloader) descriptor(d vp9Desc, first bool) []byte {
	b0 := byte(0x80 | 0x10) // I, F
	if d.interPicture {
		b0 |= 0x40
	}
	if d.layered {
		b0 |= 0x20
	}
	if first {
		b0 |= 0x08 // B
	}
	ss := d.ss && first
	if ss {
		b0 |= 0x02
	}
	if d.notRefUpper {
		b0 |= 0x01
	}
	b := []byte{b0, 0x80 | byte(p.pictureID>>8), byte(p.pictureID)}
	if d.layered {
		l := byte(d.tid&7)<<5 | byte(d.sid&7)<<1
		if d.switchUp {
			l |= 0x10
		}
		if d.interLayer {
			l |= 0x01
		}
		b = append(b, l)
	}
	if d.interPicture {
		for i, diff := range d.pdiffs {
			v := diff << 1
			if i < len(d.pdiffs)-1 {
				v |= 1 // N: another follows
			}
			b = append(b, v)
		}
	}
	if ss {
		n := len(p.widths)
		b = append(b, byte(n-1)<<5|0x10) // N_S, Y; no picture group
		for i := 0; i < n; i++ {
			b = append(b, byte(p.widths[i]>>8), byte(p.widths[i]), byte(p.heights[i]>>8), byte(p.heights[i]))
		}
	}
	return b
}

// vp9Parser keeps the spatial layers' heights from the last scalability
// structure.
type vp9Parser struct {
	heights []int
}

// Parse reads the VP9 payload descriptor (RFC 9628 4.2). A picture starts
// at the beginning of its lowest spatial layer's frame; it is a keyframe
// when that frame is not predicted from an earlier picture. The size is
// the highest layer's in the scalability structure.
func (v *vp9Parser) Parse(_ *rtp.Header, p []byte) (FrameDesc, bool) {
	var d FrameDesc
	if len(p) < 2 {
		return d, false
	}
	b0 := p[0]
	n := 1
	if b0&0x80 != 0 { // I: picture ID
		if p[1]&0x80 != 0 {
			n += 2
		} else {
			n++
		}
	}
	flexible := b0&0x10 != 0
	if b0&0x20 != 0 { // L: layer indices
		if len(p) <= n {
			return d, false
		}
		d.TemporalID = int(p[n] >> 5)
		d.SpatialID = int(p[n]>>1) & 0x07
		n++
		if !flexible {
			n++ // TL0PICIDX
		}
	}
	if flexible && b0&0x40 != 0 { // P_DIFF list
		for {
			if len(p) <= n {
				return d, false
			}
			more := p[n]&0x01 != 0
			n++
			if !more {
				break
			}
		}
	}
	if b0&0x02 != 0 { // V: scalability structure
		if len(p) <= n {
			return d, false
		}
		ns := int(p[n]>>5) + 1
		y, g := p[n]&0x10 != 0, p[n]&0x08 != 0
		n++
		if y {
			if len(p) < n+4*ns {
				return d, false
			}
			last := n + 4*(ns-1)
			d.Width = int(p[last])<<8 | int(p[last+1])
			d.Height = int(p[last+2])<<8 | int(p[last+3])
			v.heights = v.heights[:0]
			for i := 0; i < ns; i++ {
				v.heights = append(v.heights, int(p[n+4*i+2])<<8|int(p[n+4*i+3]))
			}
			n += 4 * ns
		}
		if g {
			if len(p) <= n {
				return d, false
			}
			groups := int(p[n])
			n++
			for i := 0; i < groups; i++ {
				if len(p) <= n {
					return d, false
				}
				r := int(p[n]>>2) & 0x03
				n += 1 + r
			}
		}
	}
	if len(p) < n {
		return d, false
	}
	if d.SpatialID < len(v.heights) {
		d.LayerHeight = v.heights[d.SpatialID]
	}
	begin := b0&0x08 != 0
	d.End = b0&0x04 != 0
	d.Start = begin && d.SpatialID == 0
	d.Keyframe = d.Start && b0&0x40 == 0
	if !d.Keyframe {
		d.Width, d.Height = 0, 0
	}
	return d, true
}
