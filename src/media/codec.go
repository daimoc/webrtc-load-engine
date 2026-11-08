package media

import (
	"fmt"
	"strings"
)

// VideoCodec is the video codec a run sends (media.video_codec, specs/014).
type VideoCodec string

// The video codecs a test plan can name.
const (
	CodecVP8  VideoCodec = "vp8"
	CodecVP9  VideoCodec = "vp9"
	CodecAV1  VideoCodec = "av1"
	CodecH264 VideoCodec = "h264"
)

// VideoCodecs returns the four codecs, in the order error messages list them.
func VideoCodecs() []VideoCodec { return []VideoCodec{CodecVP8, CodecVP9, CodecAV1, CodecH264} }

// ParseVideoCodec reads a test plan's video_codec. Empty is VP8, so plans
// written before 014 keep sending VP8.
func ParseVideoCodec(s string) (VideoCodec, error) {
	if s == "" {
		return CodecVP8, nil
	}
	for _, c := range VideoCodecs() {
		if s == string(c) {
			return c, nil
		}
	}
	return "", fmt.Errorf("unknown video codec %q (accepted: vp8, vp9, av1, h264)", s)
}

// MimeType returns the codec's WebRTC MIME type.
func (c VideoCodec) MimeType() string {
	switch c {
	case CodecVP9:
		return "video/VP9"
	case CodecAV1:
		return "video/AV1"
	case CodecH264:
		return "video/H264"
	default:
		return "video/VP8"
	}
}

// FmtpLine returns the format parameters the bots send the codec with: VP9
// profile 0 and H.264 Constrained Baseline in packetization mode 1
// (contracts/test-plan-codec.md).
func (c VideoCodec) FmtpLine() string {
	switch c {
	case CodecVP9:
		return "profile-id=0"
	case CodecH264:
		return "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"
	default:
		return ""
	}
}

// CodecFromMime returns the codec of a MIME type, case-insensitively.
func CodecFromMime(mime string) (VideoCodec, bool) {
	for _, c := range VideoCodecs() {
		if strings.EqualFold(mime, c.MimeType()) {
			return c, true
		}
	}
	return "", false
}

// Arrangement is how a run's video is layered.
type Arrangement string

// SimulcastRIDs are the RIDs of a simulcast track's layers, low to high:
// what the bots send, and what adapters announce to servers that need them.
var SimulcastRIDs = []string{"l", "m", "h"}

// Video arrangements (data-model.md).
const (
	ArrangementSingle    Arrangement = "single"
	ArrangementSimulcast Arrangement = "simulcast"
	ArrangementSVC       Arrangement = "svc"
)

// Arrangement returns how the codec sends layered video: simulcast for VP8
// and H.264, scalable layers in one stream for VP9 and AV1.
func (c VideoCodec) Arrangement(layered bool) Arrangement {
	switch {
	case !layered:
		return ArrangementSingle
	case c == CodecVP9 || c == CodecAV1:
		return ArrangementSVC
	default:
		return ArrangementSimulcast
	}
}

// DependencyDescriptorURI is the dependency descriptor RTP header extension
// (AV1 RTP specification, appendix A), which servers read AV1 frames and
// every codec's scalable layers from.
const DependencyDescriptorURI = "https://aomediacodec.github.io/av1-rtp-spec/#dependency-descriptor-rtp-header-extension"

// SourceDDExtID is the header extension ID sources put the dependency
// descriptor under. It is not sent: the sending track replaces it with the
// ID each session negotiated, or drops it when the session has none.
const SourceDDExtID uint8 = 14
