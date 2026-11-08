package media

import "github.com/pion/rtp"

// FrameDesc is what one received video packet tells about its frame,
// without decoding (specs/014 research R8). VideoQuality reads it for every
// codec.
type FrameDesc struct {
	Start    bool // First packet of a picture (its lowest layer's first packet)
	End      bool // Last packet of a layer frame, when the codec says so
	Keyframe bool // Set on a picture's first packet only
	// Width and Height are the picture's size when the packet tells it
	// (keyframe header, SPS, scalability structure, dependency descriptor
	// structure); 0 otherwise.
	Width, Height         int
	SpatialID, TemporalID int // 0 when the codec carries none
	// LayerHeight is the height of the packet's spatial layer, on every
	// packet once the stream described its layers (VP9 scalability
	// structure, dependency descriptor structure); 0 otherwise. A
	// scalable picture's height is its highest layer's.
	LayerHeight int
}

// FrameParser reads one codec's packets of one received stream. It may
// keep state between packets (the dependency descriptor's structure).
type FrameParser interface {
	Parse(h *rtp.Header, payload []byte) (FrameDesc, bool)
}

// NewFrameParser returns the parser of codec c. ddExtID is the dependency
// descriptor's negotiated extension ID, 0 when it was not negotiated.
func NewFrameParser(c VideoCodec, ddExtID uint8) FrameParser {
	switch c {
	case CodecVP9:
		return &vp9Parser{}
	case CodecAV1:
		return &av1Parser{ddExtID: ddExtID}
	case CodecH264:
		return &h264Parser{}
	default:
		return vp8Parser{}
	}
}

type vp8Parser struct{}

func (vp8Parser) Parse(_ *rtp.Header, p []byte) (FrameDesc, bool) {
	info, ok := parseVP8(p)
	return info.desc(), ok
}

func (i vp8Info) desc() FrameDesc {
	return FrameDesc{Start: i.frameStart, Keyframe: i.keyframe, Width: i.width, Height: i.height}
}
