package media

import "hash/fnv"

// A frame trailer identifies a sent video frame in a way that survives
// servers rewriting timestamps, sequence numbers and picture IDs (012
// research R1). It is the last 6 bytes of a frame's last RTP packet, after
// the VP8 data: a tag of the scenario, then the frame number, big endian.
const (
	frameTrailerSize = 6
	frameNumberMask  = 1<<24 - 1
)

// scenarioTag returns the tag of a scenario: the first 3 bytes of the
// 32-bit FNV-1a of its ID.
func scenarioTag(id string) [3]byte {
	h := fnv.New32a()
	h.Write([]byte(id))
	v := h.Sum32()
	return [3]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8)}
}

// appendFrameTrailer appends the trailer of frame n to p.
func appendFrameTrailer(p []byte, tag [3]byte, n uint32) []byte {
	return append(p, tag[0], tag[1], tag[2], byte(n>>16), byte(n>>8), byte(n))
}

// readFrameTrailer returns the frame number at the end of p, or false when
// p does not end with a trailer of this tag: mid-frame packets, and media
// from other publishers.
func readFrameTrailer(p []byte, tag [3]byte) (n uint32, ok bool) {
	if len(p) < frameTrailerSize {
		return 0, false
	}
	t := p[len(p)-frameTrailerSize:]
	if t[0] != tag[0] || t[1] != tag[1] || t[2] != tag[2] {
		return 0, false
	}
	return uint32(t[3])<<16 | uint32(t[4])<<8 | uint32(t[5]), true
}

// newerFrame reports whether frame n comes after frame last, in 24-bit
// modular order.
func newerFrame(n, last uint32) bool {
	d := (n - last) & frameNumberMask
	return d != 0 && d < 1<<23
}
