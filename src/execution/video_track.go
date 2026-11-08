package execution

import (
	"errors"
	"strings"
	"sync"

	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

// errCodecMismatch marks a bot whose negotiated video codec is not the
// plan's (014 research R11).
var errCodecMismatch = platform.ErrCodecMismatch

type codecMismatchError = platform.CodecMismatchError

// mismatchReport is a mismatch as the run report lists it.
func mismatchReport(e *codecMismatchError, bot string) scenario.CodecMismatch {
	dir := "sent"
	if e.Direction == "receiving" {
		dir = "received"
	}
	found, _, _ := strings.Cut(e.Found, ",")
	return scenario.CodecMismatch{Bot: bot, Direction: dir, Codec: strings.ToLower(strings.TrimSpace(found))}
}

// sendTrack is a local track the scenario's media sources write to.
type sendTrack interface {
	webrtc.TrackLocal
	WriteRTP(p *rtp.Packet) error
}

// videoBinding is one session a video track is sent on.
type videoBinding struct {
	id          string
	ssrc        webrtc.SSRC
	payloadType webrtc.PayloadType
	ddID        uint8 // Negotiated dependency descriptor ID; 0: not negotiated
	midID       uint8 // Negotiated MID and RID extension IDs (simulcast); 0: not negotiated
	ridID       uint8
	writeStream webrtc.TrackLocalWriter
}

// videoTrack sends one video stream (or simulcast layer) in the plan's
// codec. Unlike Pion's TrackLocalStaticRTP it binds only to its own codec,
// failing with errCodecMismatch otherwise, and rewrites the header
// extension IDs each session negotiated (research R7). Packets are shared
// between bindings and never changed.
type videoTrack struct {
	mu                sync.RWMutex
	bindings          []videoBinding
	codec             media.VideoCodec
	capability        webrtc.RTPCodecCapability
	id, rid, streamID string
	mid               func() string // The sender's transceiver MID, for the MID extension; nil: none
}

// setMID gives the track its transceiver's MID, written with its RID where
// the session negotiated those extensions (simulcast on the servers the
// bots publish to, which tell the layers apart by RID).
func (t *videoTrack) setMID(mid func() string) {
	t.mu.Lock()
	t.mid = mid
	t.mu.Unlock()
}

var _ sendTrack = (*videoTrack)(nil)

func newVideoTrack(codec media.VideoCodec, id, streamID, rid string) *videoTrack {
	return &videoTrack{
		codec:      codec,
		capability: webrtc.RTPCodecCapability{MimeType: codec.MimeType(), ClockRate: 90000, SDPFmtpLine: codec.FmtpLine()},
		id:         id,
		rid:        rid,
		streamID:   streamID,
	}
}

// Bind accepts the session when it negotiated the track's codec in a form
// the content can be sent as.
func (t *videoTrack) Bind(c webrtc.TrackLocalContext) (webrtc.RTPCodecParameters, error) {
	codec, ok := matchCodec(t.codec, c.CodecParameters())
	if !ok {
		return webrtc.RTPCodecParameters{}, &codecMismatchError{Direction: "sending", Found: foundCodecs(c.CodecParameters()), Want: string(t.codec)}
	}
	b := videoBinding{id: c.ID(), ssrc: c.SSRC(), payloadType: codec.PayloadType, writeStream: c.WriteStream()}
	for _, e := range c.HeaderExtensions() {
		if e.ID <= 0 || e.ID >= 256 {
			continue
		}
		switch e.URI {
		case media.DependencyDescriptorURI:
			b.ddID = uint8(e.ID)
		case sdp.SDESMidURI:
			b.midID = uint8(e.ID)
		case sdp.SDESRTPStreamIDURI:
			b.ridID = uint8(e.ID)
		}
	}
	t.mu.Lock()
	t.bindings = append(t.bindings, b)
	t.mu.Unlock()
	return codec, nil
}

// matchCodec finds the plan's codec among the negotiated ones: VP9 must be
// profile 0 and H.264 Constrained Baseline in packetization mode 1, which
// is what the content and its payloader are.
func matchCodec(want media.VideoCodec, codecs []webrtc.RTPCodecParameters) (webrtc.RTPCodecParameters, bool) {
	// H.264: Constrained Baseline 42e01f itself first, as servers list it,
	// then any Baseline-compatible profile.
	if want == media.CodecH264 {
		for _, c := range codecs {
			f := fmtpParams(c.SDPFmtpLine)
			if strings.EqualFold(c.MimeType, want.MimeType()) && f["packetization-mode"] == "1" && strings.EqualFold(f["profile-level-id"], "42e01f") {
				return c, true
			}
		}
	}
	for _, c := range codecs {
		if !strings.EqualFold(c.MimeType, want.MimeType()) {
			continue
		}
		f := fmtpParams(c.SDPFmtpLine)
		switch want {
		case media.CodecVP9:
			if p, ok := f["profile-id"]; ok && p != "0" {
				continue
			}
		case media.CodecH264:
			plid := strings.ToLower(f["profile-level-id"])
			if f["packetization-mode"] != "1" || !strings.HasPrefix(plid, "42") {
				continue
			}
		}
		return c, true
	}
	return webrtc.RTPCodecParameters{}, false
}

func fmtpParams(line string) map[string]string {
	out := map[string]string{}
	for _, kv := range strings.Split(line, ";") {
		k, v, _ := strings.Cut(strings.TrimSpace(kv), "=")
		if k != "" {
			out[strings.ToLower(k)] = v
		}
	}
	return out
}

// foundCodecs names the video codecs of a negotiation, for the mismatch
// error: "VP8" or "VP8, H264".
func foundCodecs(codecs []webrtc.RTPCodecParameters) string {
	var names []string
	seen := map[string]bool{}
	for _, c := range codecs {
		name := strings.ToUpper(strings.TrimPrefix(strings.ToLower(c.MimeType), "video/"))
		if !strings.HasPrefix(strings.ToLower(c.MimeType), "video/") || name == "RTX" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	if len(names) == 0 {
		return "no video codec"
	}
	return strings.Join(names, ", ")
}

func (t *videoTrack) Unbind(c webrtc.TrackLocalContext) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.bindings {
		if t.bindings[i].id == c.ID() {
			t.bindings = append(t.bindings[:i], t.bindings[i+1:]...)
			return nil
		}
	}
	return webrtc.ErrUnbindFailed
}

func (t *videoTrack) ID() string                       { return t.id }
func (t *videoTrack) RID() string                      { return t.rid }
func (t *videoTrack) StreamID() string                 { return t.streamID }
func (t *videoTrack) Kind() webrtc.RTPCodecType        { return webrtc.RTPCodecTypeVideo }
func (t *videoTrack) Codec() webrtc.RTPCodecCapability { return t.capability }

// WriteRTP sends a shared packet on every binding with that binding's SSRC,
// payload type and extension IDs. Only a copy of the header is changed.
func (t *videoTrack) WriteRTP(p *rtp.Packet) error {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var errs []error
	for _, b := range t.bindings {
		h := p.Header
		h.SSRC = uint32(b.ssrc)
		h.PayloadType = uint8(b.payloadType)
		if err := t.setExtensions(&h, &p.Header, b); err != nil {
			return err
		}
		if _, err := b.writeStream.WriteRTP(&h, p.Payload); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// setExtensions writes into h, in a new slice (interceptors may keep the
// header, as the NACK responder does, or change it, as TWCC does), the
// source packet's extensions with the binding's IDs, and the layer's MID
// and RID where negotiated.
func (t *videoTrack) setExtensions(h, src *rtp.Header, b videoBinding) error {
	ids := src.GetExtensionIDs()
	h.Extensions = make([]rtp.Extension, 0, len(ids)+2)
	h.Extension = false
	set := func(id uint8, payload []byte) error {
		if err := h.SetExtension(id, payload); err != nil {
			// An ID above 14 or a long payload: the two-byte form holds
			// every ID.
			h.ExtensionProfile = 0x1000
			return h.SetExtension(id, payload)
		}
		return nil
	}
	for _, id := range ids {
		payload := src.GetExtension(id)
		if id == media.SourceDDExtID {
			if b.ddID == 0 {
				continue
			}
			id = b.ddID
		}
		if err := set(id, payload); err != nil {
			return err
		}
	}
	if b.midID != 0 && t.mid != nil {
		if mid := t.mid(); mid != "" {
			if err := set(b.midID, []byte(mid)); err != nil {
				return err
			}
		}
	}
	if b.ridID != 0 && t.rid != "" {
		if err := set(b.ridID, []byte(t.rid)); err != nil {
			return err
		}
	}
	if len(h.Extensions) == 0 {
		h.Extension = false
		h.Extensions = nil
		h.ExtensionProfile = src.ExtensionProfile
	}
	return nil
}

// codecName is a received track's codec as the report names it: "vp9", or
// the lower-case MIME subtype of a codec the plan cannot name.
func codecName(mime string) string {
	if c, ok := media.CodecFromMime(mime); ok {
		return string(c)
	}
	return strings.ToLower(strings.TrimPrefix(strings.ToLower(mime), "video/"))
}
