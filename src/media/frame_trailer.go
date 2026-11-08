package media

import (
	"bytes"
	"hash/fnv"
)

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

// Carriers of the trailer per codec (specs/014 research R9). VP8 appends
// it to the frame's last packet. VP9 sends it as the RTP padding of the
// frame's last packet (RFC 3550 5.1: the trailer, then the padding length):
// libvpx, the browsers' VP9 decoder, takes any nonzero byte after a frame
// for the next frame, so nothing may follow the frame data (014 T051);
// receivers strip padding before decoding. AV1 sends it as an OBU_PADDING element, last in
// the last packet. H.264 sends it as an SEI message (user data
// unregistered, the project's UUID) in its own last packet: raw bytes after
// slice data could form a start code.
const (
	av1PaddingHeader = obuPadding << 3 // No extension, no size field
	seiUserDataUnreg = 5
)

// trailerUUID marks the project's SEI messages.
var trailerUUID = [16]byte{'w', 'e', 'b', 'r', 't', 'c', '-', 'l', 'o', 'a', 'd', '-', 'e', 'n', 'g', '1'}

// trailerRoom is what the trailer adds to a frame's last packet in codec c.
func trailerRoom(c VideoCodec) int {
	switch c {
	case CodecVP9:
		return frameTrailerSize + 1
	case CodecAV1:
		return frameTrailerSize + 2 // OBU element length and OBU header
	case CodecH264:
		return 0 // A packet of its own
	default:
		return frameTrailerSize
	}
}

// addTrailer adds the trailer of frame n to a frame's packets in codec c.
func addTrailer(c VideoCodec, payloads [][]byte, tag [3]byte, n uint32) [][]byte {
	if len(payloads) == 0 {
		return payloads
	}
	last := len(payloads) - 1
	switch c {
	case CodecVP9:
		payloads[last] = append(appendFrameTrailer(payloads[last], tag, n), frameTrailerSize+1)
	case CodecAV1:
		payloads[last] = appendFrameTrailer(append(payloads[last], frameTrailerSize+1, av1PaddingHeader), tag, n)
	case CodecH264:
		payloads = append(payloads, trailerSEI(tag, n))
	default:
		payloads[last] = appendFrameTrailer(payloads[last], tag, n)
	}
	return payloads
}

// trailerSEI is the SEI NAL unit carrying the trailer of frame n.
func trailerSEI(tag [3]byte, n uint32) []byte {
	payload := append(trailerUUID[:], appendFrameTrailer(nil, tag, n)...)
	raw := append([]byte{seiUserDataUnreg, byte(len(payload))}, payload...)
	raw = append(raw, 0x80) // rbsp_trailing_bits
	out := []byte{nalSEI}   // nal_ref_idc 0
	zeros := 0
	for _, b := range raw {
		if zeros >= 2 && b <= 3 {
			out = append(out, 3) // Emulation prevention
			zeros = 0
		}
		out = append(out, b)
		if b == 0 {
			zeros++
		} else {
			zeros = 0
		}
	}
	return out
}

// trailerIsPadding reports whether codec c carries the trailer as the RTP
// padding of the frame's last packet: the packet has the padding bit set
// and its payload, as addTrailer returns it, ends with the padding.
func trailerIsPadding(c VideoCodec) bool { return c == CodecVP9 }

// readTrailer returns the frame number of a frame's last packet in codec
// c, from its payload p without padding and its RTP padding pad (nil
// without), or false when the packet carries no trailer of this tag.
func readTrailer(c VideoCodec, p, pad []byte, tag [3]byte) (uint32, bool) {
	switch c {
	case CodecVP9:
		if len(pad) != frameTrailerSize+1 || pad[frameTrailerSize] != frameTrailerSize+1 {
			return 0, false
		}
		return readFrameTrailer(pad[:frameTrailerSize], tag)
	case CodecAV1:
		if len(p) < frameTrailerSize+2 || p[len(p)-frameTrailerSize-2] != frameTrailerSize+1 || p[len(p)-frameTrailerSize-1] != av1PaddingHeader {
			return 0, false
		}
		return readFrameTrailer(p, tag)
	case CodecH264:
		if len(p) < 2 || nalType(p) != nalSEI {
			return 0, false
		}
		b := rbsp(p[1:])
		if len(b) < 2+16+frameTrailerSize || b[0] != seiUserDataUnreg || int(b[1]) != 16+frameTrailerSize || !bytes.Equal(b[2:18], trailerUUID[:]) {
			return 0, false
		}
		return readFrameTrailer(b[2:18+frameTrailerSize], tag)
	default:
		return readFrameTrailer(p, tag)
	}
}
