package platform

import (
	"errors"
	"fmt"
)

// ErrCodecMismatch marks a bot whose negotiated video codec, sent or
// received, is not the test plan's (specs/014 research R11). Adapters
// return a *CodecMismatchError when the server cannot take the plan's
// codec; the bot then fails at the media stage.
var ErrCodecMismatch = errors.New("codec_mismatch")

// CodecMismatchError says which codec was found where the plan's was
// expected.
type CodecMismatchError struct {
	Direction string // "sending" or "receiving"
	Found     string // Codecs found, MIME subtypes: "VP8" or "VP8, H264"
	Want      string // The plan's codec: "av1"
}

func (e *CodecMismatchError) Error() string {
	if e.Direction == "receiving" {
		return fmt.Sprintf("codec_mismatch: receiving %s, plan asks for %s", e.Found, e.Want)
	}
	return fmt.Sprintf("codec_mismatch: sending negotiated %s, plan asks for %s", e.Found, e.Want)
}

// Is makes errors.Is(err, ErrCodecMismatch) true.
func (e *CodecMismatchError) Is(target error) bool { return target == ErrCodecMismatch }
