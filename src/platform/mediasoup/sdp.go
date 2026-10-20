package mediasoup

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"

	"github.com/pion/sdp/v3"
)

// The SDP translations of research R2 and R3
// (contracts/sdp-mapping.md): mediasoup hands out JSON parameters, the
// peer speaks SDP, and the adapter writes and reads SDP as mediasoup-client
// does.

const dataChannelMID = "datachannel"

func itoa(i int) string { return strconv.Itoa(i) }

// recvSection is a section of the receive offer (data-model "Receive
// section"): the application section, or one consumer.
type recvSection struct {
	mid        string
	kind       string // "application", "audio" or "video"
	consumerID string
	peerID     string
	rtp        RtpParameters
	active     bool
}

// recvOffer is the offer of the session the bot receives on: built from
// the receiving transport, with the application section first and one
// section per consumer ever accepted, in acceptance order.
type recvOffer struct {
	transport TransportInfo
	sessionID uint64
	version   uint64
	sections  []recvSection
}

func newRecvOffer(t TransportInfo) *recvOffer {
	return &recvOffer{
		transport: t,
		sessionID: rand.Uint64N(1 << 62),
		sections:  []recvSection{{mid: dataChannelMID, kind: "application", active: true}},
	}
}

// add appends the section of an accepted consumer.
func (o *recvOffer) add(c NewConsumerRequest) {
	o.sections = append(o.sections, recvSection{mid: c.RtpParameters.MID, kind: c.Kind, consumerID: c.ConsumerID, peerID: c.PeerID, rtp: c.RtpParameters, active: true})
}

// deactivate sets the section of a closed consumer inactive and reports
// whether the consumer had one.
func (o *recvOffer) deactivate(consumerID string) bool {
	for i := range o.sections {
		if s := &o.sections[i]; s.consumerID == consumerID && s.active {
			s.active = false
			return true
		}
	}
	return false
}

// sdpWriter collects SDP lines.
type sdpWriter struct{ b strings.Builder }

func (w *sdpWriter) line(format string, args ...any) {
	fmt.Fprintf(&w.b, format, args...)
	w.b.WriteString("\r\n")
}

// transportLines are the parameters of a mediasoup transport, written in
// every section: ICE credentials and candidates (UDP only), DTLS setup.
type transportLines struct {
	t     TransportInfo
	setup string // "actpass" (receive offer) or "passive" (publish answer)
}

func (tl transportLines) udp() []IceCandidate {
	var cs []IceCandidate
	for _, c := range tl.t.IceCandidates {
		if strings.EqualFold(c.Protocol, "udp") {
			cs = append(cs, c)
		}
	}
	return cs
}

func (tl transportLines) fingerprint() (DtlsFingerprint, error) {
	for _, f := range tl.t.DtlsParameters.Fingerprints {
		if f.Algorithm == "sha-256" {
			return f, nil
		}
	}
	return DtlsFingerprint{}, errors.New("transport has no sha-256 DTLS fingerprint")
}

// header writes the session part.
func (tl transportLines) header(w *sdpWriter, sessionID, version uint64, mids []string) error {
	fp, err := tl.fingerprint()
	if err != nil {
		return err
	}
	w.line("v=0")
	w.line("o=- %d %d IN IP4 127.0.0.1", sessionID, version)
	w.line("s=-")
	w.line("t=0 0")
	w.line("a=ice-lite")
	w.line("a=group:BUNDLE %s", strings.Join(mids, " "))
	w.line("a=msid-semantic: WMS *")
	w.line("a=fingerprint:%s %s", fp.Algorithm, fp.Value)
	return nil
}

// media writes a section's m=, c= and transport lines.
func (tl transportLines) media(w *sdpWriter, mline, mid string) error {
	cs := tl.udp()
	if len(cs) == 0 {
		return errors.New("transport has no UDP candidate")
	}
	w.line("m=%s", strings.Replace(mline, "{port}", itoa(cs[0].Port), 1))
	w.line("c=IN IP4 %s", cs[0].Address)
	w.line("a=mid:%s", mid)
	w.line("a=ice-ufrag:%s", tl.t.IceParameters.UsernameFragment)
	w.line("a=ice-pwd:%s", tl.t.IceParameters.Password)
	w.line("a=setup:%s", tl.setup)
	for _, c := range cs {
		w.line("a=candidate:%s 1 udp %d %s %d typ %s", c.Foundation, c.Priority, c.Address, c.Port, c.Type)
	}
	w.line("a=end-of-candidates")
	return nil
}

// formatParams writes codec parameters as an fmtp value, keys sorted.
func formatParams(p map[string]any) string {
	keys := make([]string, 0, len(p))
	for k := range p {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		var v string
		switch x := p[k].(type) {
		case float64:
			v = strconv.FormatFloat(x, 'f', -1, 64)
		default:
			v = fmt.Sprint(x)
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ";")
}

// rtpmapName is a codec's rtpmap encoding: "opus/48000/2", "VP8/90000".
func rtpmapName(c RtpCodecParameters) string {
	_, name, _ := strings.Cut(c.MimeType, "/")
	s := name + "/" + itoa(c.ClockRate)
	if c.Channels > 1 {
		s += "/" + itoa(c.Channels)
	}
	return s
}

// codecLines writes a section's codecs: rtpmap, fmtp, rtcp-fb.
func codecLines(w *sdpWriter, codecs []RtpCodecParameters) {
	for _, c := range codecs {
		w.line("a=rtpmap:%d %s", c.PayloadType, rtpmapName(c))
		if f := formatParams(c.Parameters); f != "" {
			w.line("a=fmtp:%d %s", c.PayloadType, f)
		}
		for _, fb := range c.RtcpFeedback {
			w.line("a=rtcp-fb:%d %s", c.PayloadType, strings.TrimSpace(fb.Type+" "+fb.Parameter))
		}
	}
}

func payloadTypes(codecs []RtpCodecParameters) string {
	pts := make([]string, len(codecs))
	for i, c := range codecs {
		pts[i] = itoa(c.PayloadType)
	}
	return strings.Join(pts, " ")
}

// SDP returns the offer, with the session version incremented.
func (o *recvOffer) SDP() (string, error) {
	o.version++
	tl := transportLines{t: o.transport, setup: "actpass"}
	mids := make([]string, len(o.sections))
	for i, s := range o.sections {
		mids[i] = s.mid
	}
	var w sdpWriter
	if err := tl.header(&w, o.sessionID, o.version, mids); err != nil {
		return "", err
	}
	for _, s := range o.sections {
		if s.kind == "application" {
			if err := tl.media(&w, "application {port} UDP/DTLS/SCTP webrtc-datachannel", s.mid); err != nil {
				return "", err
			}
			port, size := 5000, 262144
			if sp := o.transport.SctpParameters; sp != nil {
				port, size = sp.Port, sp.MaxMessageSize
			}
			w.line("a=sctp-port:%d", port)
			w.line("a=max-message-size:%d", size)
			continue
		}
		if err := tl.media(&w, s.kind+" {port} UDP/TLS/RTP/SAVPF "+payloadTypes(s.rtp.Codecs), s.mid); err != nil {
			return "", err
		}
		w.line("a=rtcp-mux")
		if s.rtp.RTCP.ReducedSize {
			w.line("a=rtcp-rsize")
		}
		codecLines(&w, s.rtp.Codecs)
		for _, e := range s.rtp.HeaderExtensions {
			w.line("a=extmap:%d %s", e.ID, e.URI)
		}
		if !s.active {
			w.line("a=inactive")
			continue
		}
		w.line("a=sendonly")
		w.line("a=msid:%s %s", s.peerID, s.consumerID)
		if len(s.rtp.Encodings) > 0 {
			enc := s.rtp.Encodings[0]
			if enc.RTX != nil {
				w.line("a=ssrc-group:FID %d %d", enc.SSRC, enc.RTX.SSRC)
			}
			w.line("a=ssrc:%d cname:%s", enc.SSRC, s.rtp.RTCP.CNAME)
			if enc.RTX != nil {
				w.line("a=ssrc:%d cname:%s", enc.RTX.SSRC, s.rtp.RTCP.CNAME)
			}
		}
	}
	return w.b.String(), nil
}

// ProducerSpec is what one section of the publish offer becomes: a
// produce request's kind and RTP parameters.
type ProducerSpec struct {
	Kind          string
	MID           string
	RtpParameters RtpParameters
}

// wantedCodec is the codec the bots send per kind (004): Opus and VP8.
var wantedCodec = map[string]struct {
	mime, rtpmap string
	clock, ch    int
}{
	"audio": {"audio/opus", "opus/48000/2", 48000, 2},
	"video": {"video/VP8", "vp8/90000", 90000, 0},
}

// parseParams reads an fmtp value; integer values become ints.
func parseParams(f string) map[string]any {
	p := map[string]any{}
	for _, kv := range strings.Split(f, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(kv), "=")
		if !ok || k == "" {
			continue
		}
		if n, err := strconv.Atoi(v); err == nil {
			p[k] = n
		} else {
			p[k] = v
		}
	}
	return p
}

// sectionCodecs reads a section's rtpmap, fmtp and rtcp-fb lines by
// payload type.
type sectionCodec struct {
	pt       int
	rtpmap   string
	params   map[string]any
	feedback []RtcpFeedback
}

func readCodecs(md *sdp.MediaDescription) map[int]*sectionCodec {
	codecs := map[int]*sectionCodec{}
	get := func(value string) (*sectionCodec, string) {
		ptStr, rest, _ := strings.Cut(value, " ")
		pt, err := strconv.Atoi(ptStr)
		if err != nil {
			return nil, ""
		}
		c := codecs[pt]
		if c == nil {
			c = &sectionCodec{pt: pt, params: map[string]any{}}
			codecs[pt] = c
		}
		return c, strings.TrimSpace(rest)
	}
	for _, a := range md.Attributes {
		switch a.Key {
		case "rtpmap":
			if c, rest := get(a.Value); c != nil {
				c.rtpmap = strings.ToLower(rest)
			}
		case "fmtp":
			if c, rest := get(a.Value); c != nil {
				c.params = parseParams(rest)
			}
		case "rtcp-fb":
			if c, rest := get(a.Value); c != nil {
				typ, param, _ := strings.Cut(rest, " ")
				fb := RtcpFeedback{Type: typ, Parameter: strings.TrimSpace(param)}
				if !slices.Contains(c.feedback, fb) {
					c.feedback = append(c.feedback, fb)
				}
			}
		}
	}
	return codecs
}

// producerParams turns the publish offer's sections into produce requests
// (contracts/sdp-mapping.md "Publish offer"): the bot's codec with its
// payload type and feedback, its RTX, the header extensions the router
// supports, the section's SSRCs and CNAME.
func producerParams(offer string, caps RtpCapabilities) ([]ProducerSpec, error) {
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(offer)); err != nil {
		return nil, fmt.Errorf("publish offer: %w", err)
	}
	var specs []ProducerSpec
	for _, md := range d.MediaDescriptions {
		kind := md.MediaName.Media
		want, ok := wantedCodec[kind]
		if !ok {
			continue
		}
		if !slices.ContainsFunc(caps.Codecs, func(c RtpCodecCapability) bool { return strings.EqualFold(c.MimeType, want.mime) }) {
			return nil, fmt.Errorf("the router has no %s codec", want.mime)
		}
		mid, _ := md.Attribute("mid")
		codecs := readCodecs(md)
		var main *sectionCodec
		for _, f := range md.MediaName.Formats { // Offer order: preference
			pt, err := strconv.Atoi(f)
			if err != nil {
				continue
			}
			if c := codecs[pt]; c != nil && c.rtpmap == want.rtpmap {
				main = c
				break
			}
		}
		if main == nil {
			return nil, fmt.Errorf("publish offer: %s section %q has no %s codec", kind, mid, want.mime)
		}
		rp := RtpParameters{MID: mid}
		rp.Codecs = append(rp.Codecs, RtpCodecParameters{
			MimeType: want.mime, PayloadType: main.pt, ClockRate: want.clock, Channels: want.ch,
			Parameters: main.params, RtcpFeedback: nonNil(main.feedback),
		})
		for _, c := range codecs {
			if c.rtpmap == "rtx/90000" && c.params["apt"] == main.pt {
				rp.Codecs = append(rp.Codecs, RtpCodecParameters{
					MimeType: kind + "/rtx", PayloadType: c.pt, ClockRate: 90000,
					Parameters: map[string]any{"apt": main.pt}, RtcpFeedback: []RtcpFeedback{},
				})
				break
			}
		}
		rp.HeaderExtensions = []RtpHeaderExtensionParameters{}
		for _, a := range md.Attributes {
			if a.Key != "extmap" {
				continue
			}
			idDir, uri, _ := strings.Cut(a.Value, " ")
			idStr, _, _ := strings.Cut(idDir, "/")
			id, err := strconv.Atoi(idStr)
			if err != nil {
				continue
			}
			uri = strings.Fields(uri)[0]
			if slices.ContainsFunc(caps.HeaderExtensions, func(h RtpHeaderExtension) bool { return h.Kind == kind && h.URI == uri }) {
				rp.HeaderExtensions = append(rp.HeaderExtensions, RtpHeaderExtensionParameters{URI: uri, ID: id})
			}
		}
		enc, cname, err := sectionSSRCs(md)
		if err != nil {
			return nil, fmt.Errorf("publish offer: %s section %q: %w", kind, mid, err)
		}
		rp.Encodings = []RtpEncodingParameters{enc}
		_, rsize := md.Attribute("rtcp-rsize")
		rp.RTCP = RtcpParameters{CNAME: cname, ReducedSize: rsize}
		specs = append(specs, ProducerSpec{Kind: kind, MID: mid, RtpParameters: rp})
	}
	if len(specs) == 0 {
		return nil, errors.New("publish offer has no audio or video section")
	}
	return specs, nil
}

func nonNil(fb []RtcpFeedback) []RtcpFeedback {
	if fb == nil {
		return []RtcpFeedback{}
	}
	return fb
}

// sectionSSRCs reads a section's first SSRC, its FID (RTX) SSRC and the
// CNAME.
func sectionSSRCs(md *sdp.MediaDescription) (RtpEncodingParameters, string, error) {
	var enc RtpEncodingParameters
	var cname string
	for _, a := range md.Attributes {
		switch a.Key {
		case "ssrc-group":
			f := strings.Fields(a.Value)
			if len(f) == 3 && f[0] == "FID" {
				if rtx, err := strconv.ParseUint(f[2], 10, 32); err == nil {
					enc.RTX = &RtxSSRC{SSRC: uint32(rtx)}
				}
			}
		case "ssrc":
			idStr, rest, _ := strings.Cut(a.Value, " ")
			id, err := strconv.ParseUint(idStr, 10, 32)
			if err != nil {
				continue
			}
			if enc.SSRC == 0 {
				enc.SSRC = uint32(id)
			}
			if v, ok := strings.CutPrefix(rest, "cname:"); ok && uint32(id) == enc.SSRC {
				cname = v
			}
		}
	}
	if enc.SSRC == 0 {
		return enc, "", errors.New("no ssrc")
	}
	return enc, cname, nil
}

// publishAnswer is the remote answer to the publish offer
// (contracts/sdp-mapping.md "Publish answer"): the sending transport's
// parameters with setup:passive, and per section the produced codec, its
// RTX and the kept header extensions, recvonly.
func publishAnswer(offer string, t TransportInfo, specs []ProducerSpec) (string, error) {
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(offer)); err != nil {
		return "", fmt.Errorf("publish offer: %w", err)
	}
	byMID := map[string]ProducerSpec{}
	for _, s := range specs {
		byMID[s.MID] = s
	}
	tl := transportLines{t: t, setup: "passive"}
	var mids []string
	for _, md := range d.MediaDescriptions {
		mid, _ := md.Attribute("mid")
		mids = append(mids, mid)
	}
	var w sdpWriter
	if err := tl.header(&w, rand.Uint64N(1<<62), 1, mids); err != nil {
		return "", err
	}
	for _, md := range d.MediaDescriptions {
		mid, _ := md.Attribute("mid")
		spec, ok := byMID[mid]
		if !ok {
			return "", fmt.Errorf("publish offer section %q (%s) has no producer", mid, md.MediaName.Media)
		}
		rp := spec.RtpParameters
		if err := tl.media(&w, spec.Kind+" {port} UDP/TLS/RTP/SAVPF "+payloadTypes(rp.Codecs), mid); err != nil {
			return "", err
		}
		w.line("a=rtcp-mux")
		if rp.RTCP.ReducedSize {
			w.line("a=rtcp-rsize")
		}
		codecLines(&w, rp.Codecs)
		for _, e := range rp.HeaderExtensions {
			w.line("a=extmap:%d %s", e.ID, e.URI)
		}
		w.line("a=recvonly")
	}
	return w.b.String(), nil
}

// localFingerprint reads the bot's sha-256 DTLS fingerprint from its SDP,
// at session or media level.
func localFingerprint(s string) (DtlsFingerprint, error) {
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(s)); err != nil {
		return DtlsFingerprint{}, err
	}
	parse := func(v string) (DtlsFingerprint, bool) {
		alg, val, ok := strings.Cut(v, " ")
		if !ok || !strings.EqualFold(alg, "sha-256") {
			return DtlsFingerprint{}, false
		}
		return DtlsFingerprint{Algorithm: "sha-256", Value: strings.TrimSpace(val)}, true
	}
	for _, a := range d.Attributes {
		if a.Key == "fingerprint" {
			if fp, ok := parse(a.Value); ok {
				return fp, nil
			}
		}
	}
	for _, md := range d.MediaDescriptions {
		for _, a := range md.Attributes {
			if a.Key == "fingerprint" {
				if fp, ok := parse(a.Value); ok {
					return fp, nil
				}
			}
		}
	}
	return DtlsFingerprint{}, errors.New("SDP has no sha-256 fingerprint")
}

// localDtlsRole is the bot's DTLS role from its own SDP, as mediasoup's
// connectWebRtcTransport wants it: "server" for setup:passive, "client"
// for active or actpass (the publish offer, which the adapter answers with
// passive).
func localDtlsRole(s string) (string, error) {
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(s)); err != nil {
		return "", err
	}
	setup, ok := d.Attribute("setup")
	if !ok {
		for _, md := range d.MediaDescriptions {
			if setup, ok = md.Attribute("setup"); ok {
				break
			}
		}
	}
	switch setup {
	case "passive":
		return "server", nil
	case "active", "actpass":
		return "client", nil
	}
	return "", fmt.Errorf("SDP has no usable setup attribute (%q)", setup)
}
