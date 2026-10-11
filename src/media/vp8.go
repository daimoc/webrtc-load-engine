package media

// vp8Info is what a received VP8 packet tells without decoding (research R1).
type vp8Info struct {
	frameStart bool // First packet of a frame: S bit set, partition index 0
	keyframe   bool // Set on a frame start only
	width      int  // Keyframes only; 0 if the header is incomplete
	height     int
}

// parseVP8 reads the VP8 payload descriptor (RFC 7741 4.2) of an RTP payload
// and, at a frame start, the frame header (RFC 6386 9.1). It reports false
// for a payload too short to hold a descriptor and some frame data.
func parseVP8(p []byte) (vp8Info, bool) {
	var info vp8Info
	if len(p) == 0 {
		return info, false
	}
	b := p[0]
	n := 1
	if b&0x80 != 0 { // X: extension byte
		if len(p) < 2 {
			return info, false
		}
		x := p[1]
		n = 2
		if x&0x80 != 0 { // I: picture ID, 7 or 15 bits
			if len(p) <= n {
				return info, false
			}
			if p[n]&0x80 != 0 {
				n += 2
			} else {
				n++
			}
		}
		if x&0x40 != 0 { // L: TL0PICIDX
			n++
		}
		if x&0x30 != 0 { // T or K: TID/Y/KEYIDX
			n++
		}
	}
	if len(p) <= n {
		return info, false
	}
	info.frameStart = b&0x10 != 0 && b&0x07 == 0
	if !info.frameStart {
		return info, true
	}
	h := p[n:]
	info.keyframe = h[0]&0x01 == 0
	if info.keyframe {
		info.width, info.height = keyframeSize(h)
	}
	return info, true
}

// keyframeSize returns the size of a VP8 keyframe from its header, or 0, 0
// if the header is incomplete or has no start code.
func keyframeSize(h []byte) (width, height int) {
	if len(h) < 10 || h[3] != 0x9d || h[4] != 0x01 || h[5] != 0x2a {
		return 0, 0
	}
	return int(h[6]) | int(h[7]&0x3f)<<8, int(h[8]) | int(h[9]&0x3f)<<8
}
