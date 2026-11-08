package media

import (
	"strings"
	"testing"
)

func TestParseVideoCodec(t *testing.T) {
	for in, want := range map[string]VideoCodec{"": CodecVP8, "vp8": CodecVP8, "vp9": CodecVP9, "av1": CodecAV1, "h264": CodecH264} {
		got, err := ParseVideoCodec(in)
		if err != nil || got != want {
			t.Errorf("ParseVideoCodec(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"VP8", "vp10", "h265", " vp9"} {
		_, err := ParseVideoCodec(in)
		if err == nil {
			t.Errorf("ParseVideoCodec(%q): expected an error", in)
			continue
		}
		if !strings.Contains(err.Error(), "vp8, vp9, av1, h264") {
			t.Errorf("ParseVideoCodec(%q): error %q does not list the accepted codecs", in, err)
		}
	}
}

func TestVideoCodec_MimeAndFmtp(t *testing.T) {
	cases := []struct {
		c          VideoCodec
		mime, fmtp string
	}{
		{CodecVP8, "video/VP8", ""},
		{CodecVP9, "video/VP9", "profile-id=0"},
		{CodecAV1, "video/AV1", ""},
		{CodecH264, "video/H264", "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f"},
	}
	for _, c := range cases {
		if got := c.c.MimeType(); got != c.mime {
			t.Errorf("%s.MimeType() = %q, want %q", c.c, got, c.mime)
		}
		if got := c.c.FmtpLine(); got != c.fmtp {
			t.Errorf("%s.FmtpLine() = %q, want %q", c.c, got, c.fmtp)
		}
	}
}

func TestVideoCodec_Arrangement(t *testing.T) {
	want := map[VideoCodec]Arrangement{CodecVP8: ArrangementSimulcast, CodecH264: ArrangementSimulcast, CodecVP9: ArrangementSVC, CodecAV1: ArrangementSVC}
	for c, layered := range want {
		if got := c.Arrangement(false); got != ArrangementSingle {
			t.Errorf("%s.Arrangement(false) = %q", c, got)
		}
		if got := c.Arrangement(true); got != layered {
			t.Errorf("%s.Arrangement(true) = %q, want %q", c, got, layered)
		}
	}
}

func TestCodecFromMime(t *testing.T) {
	for _, c := range VideoCodecs() {
		got, ok := CodecFromMime(strings.ToLower(c.MimeType()))
		if !ok || got != c {
			t.Errorf("CodecFromMime(%q) = %q, %v", c.MimeType(), got, ok)
		}
	}
	if _, ok := CodecFromMime("video/H265"); ok {
		t.Error("CodecFromMime(video/H265): expected false")
	}
}
