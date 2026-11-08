package execution

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"webrtc-load-engine/src/media"
)

// fakeBinding is a TrackLocalContext for one negotiated session.
type fakeBinding struct {
	id     string
	codecs []webrtc.RTPCodecParameters
	exts   []webrtc.RTPHeaderExtensionParameter
	ssrc   webrtc.SSRC
	w      *recordingWriter
}

func (f *fakeBinding) CodecParameters() []webrtc.RTPCodecParameters           { return f.codecs }
func (f *fakeBinding) HeaderExtensions() []webrtc.RTPHeaderExtensionParameter { return f.exts }
func (f *fakeBinding) SSRC() webrtc.SSRC                                      { return f.ssrc }
func (f *fakeBinding) WriteStream() webrtc.TrackLocalWriter                   { return f.w }
func (f *fakeBinding) ID() string                                             { return f.id }
func (f *fakeBinding) RTCPReader() interceptor.RTCPReader                     { return nil }

type recordingWriter struct {
	mu   sync.Mutex
	hdrs []rtp.Header
}

func (w *recordingWriter) WriteRTP(h *rtp.Header, payload []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	c := *h
	c.Extensions = append([]rtp.Extension(nil), h.Extensions...)
	w.hdrs = append(w.hdrs, c)
	return len(payload), nil
}
func (w *recordingWriter) Write(b []byte) (int, error) { return len(b), nil }

func codecParams(mime, fmtp string, pt webrtc.PayloadType) webrtc.RTPCodecParameters {
	return webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: mime, ClockRate: 90000, SDPFmtpLine: fmtp}, PayloadType: pt}
}

func TestVideoTrack_BindRewritesSSRCAndPayloadType(t *testing.T) {
	tr := newVideoTrack(media.CodecVP9, "video", "bot", "")
	b := &fakeBinding{id: "a", ssrc: 1111, w: &recordingWriter{},
		codecs: []webrtc.RTPCodecParameters{codecParams("video/VP8", "", 96), codecParams("video/VP9", "profile-id=0", 98)}}
	got, err := tr.Bind(b)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if got.PayloadType != 98 || got.MimeType != "video/VP9" {
		t.Fatalf("bound %+v", got)
	}
	if err := tr.WriteRTP(&rtp.Packet{Header: rtp.Header{SSRC: 1, PayloadType: 1, SequenceNumber: 7}, Payload: []byte{1}}); err != nil {
		t.Fatal(err)
	}
	if h := b.w.hdrs[0]; h.SSRC != 1111 || h.PayloadType != 98 || h.SequenceNumber != 7 {
		t.Fatalf("written header %+v", h)
	}
}

func TestVideoTrack_BindMismatch(t *testing.T) {
	tr := newVideoTrack(media.CodecAV1, "video", "bot", "")
	b := &fakeBinding{id: "a", w: &recordingWriter{}, codecs: []webrtc.RTPCodecParameters{codecParams("video/VP8", "", 96)}}
	_, err := tr.Bind(b)
	if !errors.Is(err, errCodecMismatch) {
		t.Fatalf("Bind: %v, want errCodecMismatch", err)
	}
	if !strings.Contains(err.Error(), "VP8") || !strings.Contains(err.Error(), "av1") {
		t.Fatalf("error %q names neither the codec found nor the plan's", err)
	}
}

func TestVideoTrack_BindChecksFmtp(t *testing.T) {
	cases := []struct {
		codec media.VideoCodec
		offer webrtc.RTPCodecParameters
		ok    bool
	}{
		{media.CodecVP9, codecParams("video/VP9", "profile-id=2", 100), false},
		{media.CodecVP9, codecParams("video/VP9", "", 100), true},
		{media.CodecH264, codecParams("video/H264", "level-asymmetry-allowed=1;packetization-mode=0;profile-level-id=42e01f", 102), false},
		{media.CodecH264, codecParams("video/H264", "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", 102), true},
		{media.CodecH264, codecParams("video/H264", "packetization-mode=1;profile-level-id=4d001f", 102), false},
		{media.CodecH264, codecParams("video/h264", "packetization-mode=1;profile-level-id=42001f", 102), true},
	}
	for _, c := range cases {
		tr := newVideoTrack(c.codec, "video", "bot", "")
		_, err := tr.Bind(&fakeBinding{id: "a", w: &recordingWriter{}, codecs: []webrtc.RTPCodecParameters{c.offer}})
		if (err == nil) != c.ok {
			t.Errorf("%s with %q: err %v, want ok=%v", c.codec, c.offer.SDPFmtpLine, err, c.ok)
		}
	}
}

func TestVideoTrack_DependencyDescriptorIDPerBinding(t *testing.T) {
	tr := newVideoTrack(media.CodecAV1, "video", "bot", "")
	av1 := []webrtc.RTPCodecParameters{codecParams("video/AV1", "", 45)}
	withDD := &fakeBinding{id: "a", ssrc: 1, w: &recordingWriter{}, codecs: av1,
		exts: []webrtc.RTPHeaderExtensionParameter{{URI: "urn:ietf:params:rtp-hdrext:sdes:mid", ID: 1}, {URI: media.DependencyDescriptorURI, ID: 11}}}
	otherDD := &fakeBinding{id: "b", ssrc: 2, w: &recordingWriter{}, codecs: av1,
		exts: []webrtc.RTPHeaderExtensionParameter{{URI: media.DependencyDescriptorURI, ID: 7}}}
	noDD := &fakeBinding{id: "c", ssrc: 3, w: &recordingWriter{}, codecs: av1}
	for _, b := range []*fakeBinding{withDD, otherDD, noDD} {
		if _, err := tr.Bind(b); err != nil {
			t.Fatal(err)
		}
	}
	dd := []byte{0x80, 0x01, 0x02}
	p := &rtp.Packet{Header: rtp.Header{Extension: true, ExtensionProfile: 0xBEDE}, Payload: []byte{9}}
	if err := p.Header.SetExtension(media.SourceDDExtID, dd); err != nil {
		t.Fatal(err)
	}
	if err := tr.WriteRTP(p); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		b  *fakeBinding
		id uint8
	}{{withDD, 11}, {otherDD, 7}} {
		h := c.b.w.hdrs[0]
		if got := h.GetExtension(c.id); string(got) != string(dd) {
			t.Errorf("binding %s: extension %d = %x", c.b.id, c.id, got)
		}
		if h.GetExtension(media.SourceDDExtID) != nil {
			t.Errorf("binding %s: the source's placeholder ID was sent", c.b.id)
		}
	}
	if h := noDD.w.hdrs[0]; h.Extension || len(h.Extensions) != 0 {
		t.Errorf("binding without the descriptor got extensions %+v", h.Extensions)
	}
	if ids := p.Header.GetExtensionIDs(); len(ids) != 1 || ids[0] != media.SourceDDExtID || p.Header.SSRC != 0 {
		t.Errorf("the caller's packet changed: %+v", p.Header)
	}
}

func TestVideoTrack_LargeDescriptorUsesTwoByteHeader(t *testing.T) {
	tr := newVideoTrack(media.CodecAV1, "video", "bot", "")
	b := &fakeBinding{id: "a", w: &recordingWriter{}, codecs: []webrtc.RTPCodecParameters{codecParams("video/AV1", "", 45)},
		exts: []webrtc.RTPHeaderExtensionParameter{{URI: media.DependencyDescriptorURI, ID: 11}}}
	if _, err := tr.Bind(b); err != nil {
		t.Fatal(err)
	}
	dd := make([]byte, 40)
	p := &rtp.Packet{Header: rtp.Header{Extension: true, ExtensionProfile: 0x1000}, Payload: []byte{9}}
	if err := p.Header.SetExtension(media.SourceDDExtID, dd); err != nil {
		t.Fatal(err)
	}
	if err := tr.WriteRTP(p); err != nil {
		t.Fatal(err)
	}
	h := b.w.hdrs[0]
	if h.ExtensionProfile != 0x1000 || len(h.GetExtension(11)) != 40 {
		t.Fatalf("profile %#x, extension %d bytes", h.ExtensionProfile, len(h.GetExtension(11)))
	}
	if _, err := h.Marshal(); err != nil {
		t.Fatalf("Marshal: %v", err)
	}
}

func TestVideoTrack_Unbind(t *testing.T) {
	tr := newVideoTrack(media.CodecVP8, "video", "bot", "l")
	b := &fakeBinding{id: "a", w: &recordingWriter{}, codecs: []webrtc.RTPCodecParameters{codecParams("video/VP8", "", 96)}}
	if _, err := tr.Bind(b); err != nil {
		t.Fatal(err)
	}
	if err := tr.Unbind(b); err != nil {
		t.Fatal(err)
	}
	if err := tr.WriteRTP(&rtp.Packet{}); err != nil || len(b.w.hdrs) != 0 {
		t.Fatalf("wrote after Unbind: %v, %d", err, len(b.w.hdrs))
	}
	if tr.RID() != "l" || tr.ID() != "video" || tr.StreamID() != "bot" || tr.Kind() != webrtc.RTPCodecTypeVideo {
		t.Fatalf("identity %s %s %s %s", tr.RID(), tr.ID(), tr.StreamID(), tr.Kind())
	}
	if tr.Codec().MimeType != "video/VP8" {
		t.Fatalf("codec %+v", tr.Codec())
	}
}

// vp8OnlyOfferer is a media server that offers VP8 video only.
func vp8OnlyOfferer(t *testing.T) *webrtc.PeerConnection {
	t.Helper()
	m := &webrtc.MediaEngine{}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, PayloadType: 96}, webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatal(err)
	}
	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	if _, err := pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatal(err)
	}
	return pc
}

func TestPeer_CodecMismatchFailsAtMediaStage(t *testing.T) {
	scenarioID := "scenario-mismatch-" + t.Name()
	p, err := NewPeer("peer-av1", scenarioID, PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: media.CodecAV1}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	offerer := vp8OnlyOfferer(t)
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := offerer.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	_, err = p.OnOffer(offerer.LocalDescription().SDP, func(string, string) {})
	if !errors.Is(err, errCodecMismatch) {
		t.Fatalf("OnOffer: %v, want a codec mismatch", err)
	}
	if !strings.Contains(err.Error(), "codec_mismatch: sending negotiated VP8, plan asks for av1") {
		t.Fatalf("error %q", err)
	}
	p.fail(stageSignaling, err)
	if got := testutil.ToFloat64(peerFailures.WithLabelValues(scenarioID, stageMedia)); got != 1 {
		t.Fatalf("media-stage failures = %v, want 1", got)
	}
}

func TestPeer_DependencyDescriptorOnlyForAV1(t *testing.T) {
	for codec, want := range map[media.VideoCodec]bool{media.CodecAV1: true, media.CodecVP8: false, media.CodecH264: false} {
		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			t.Fatal(err)
		}
		if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: media.DependencyDescriptorURI}, webrtc.RTPCodecTypeVideo); err != nil {
			t.Fatal(err)
		}
		offerer, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { offerer.Close() })
		if _, err := offerer.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
			t.Fatal(err)
		}
		p, err := NewPeer("peer-dd", "scenario-dd", PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: codec}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		p.transition(PeerStateConnecting)
		answer := negotiate(t, offerer, p)
		if got := strings.Contains(answer, media.DependencyDescriptorURI); got != want {
			t.Errorf("%s: descriptor in the answer %v, want %v", codec, got, want)
		}
	}
}

func TestPeer_ReceivedCodecMismatchFails(t *testing.T) {
	scenarioID := "scenario-recv-mismatch-" + t.Name()
	offerer, tracks := sendingOfferer(t) // Sends VP8
	p, err := NewPeer("peer-recv-av1", scenarioID, PeerMedia{VideoCodec: media.CodecAV1}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	negotiate(t, offerer, p)
	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, tracks...)
	waitForState(t, p, PeerStateFailed, 10*time.Second)
	if got := testutil.ToFloat64(peerFailures.WithLabelValues(scenarioID, stageMedia)); got != 1 {
		t.Fatalf("media-stage failures = %v, want 1", got)
	}
	received, mismatch := p.CodecReport()
	if len(received) != 1 || received[0] != "vp8" {
		t.Fatalf("received %v, want [vp8]", received)
	}
	if mismatch == nil || mismatch.Direction != "received" || mismatch.Codec != "vp8" {
		t.Fatalf("mismatch %+v", mismatch)
	}
}

func TestPeer_ReceivedCodecMatches(t *testing.T) {
	offerer, tracks := sendingOfferer(t)
	p, err := NewPeer("peer-recv-vp8", "scenario-recv-ok", PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	negotiate(t, offerer, p)
	stop := make(chan struct{})
	defer close(stop)
	go pump(stop, tracks...)
	waitForState(t, p, PeerStateEstablished, 10*time.Second)
	deadline := time.Now().Add(5 * time.Second)
	for {
		received, mismatch := p.CodecReport()
		if mismatch != nil {
			t.Fatalf("mismatch %+v", mismatch)
		}
		if len(received) == 1 && received[0] == "vp8" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("received %v", received)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestPeer_SendingMismatchIsReported(t *testing.T) {
	p, err := NewPeer("peer-send-av1", "scenario-send-mismatch", PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: media.CodecAV1}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	p.transition(PeerStateConnecting)
	offerer := vp8OnlyOfferer(t)
	offer, _ := offerer.CreateOffer(nil)
	_ = offerer.SetLocalDescription(offer)
	_, err = p.OnOffer(offerer.LocalDescription().SDP, func(string, string) {})
	p.fail(stageSignaling, err)
	_, mismatch := p.CodecReport()
	if mismatch == nil || mismatch.Direction != "sent" || mismatch.Codec != "vp8" {
		t.Fatalf("mismatch %+v", mismatch)
	}
}

// firstVideoCodec returns the MIME subtype of the first payload type of an
// offer's video section.
func firstVideoCodec(t *testing.T, sdp string) string {
	t.Helper()
	sec := mediaSection(sdp, "m=video")
	fields := strings.Fields(strings.SplitN(sec, "\r\n", 2)[0])
	if len(fields) < 4 {
		t.Fatalf("video section %q", sec)
	}
	pt := fields[3]
	for _, l := range strings.Split(sec, "\r\n") {
		if strings.HasPrefix(l, "a=rtpmap:"+pt+" ") {
			return strings.SplitN(strings.Fields(l)[1], "/", 2)[0]
		}
	}
	t.Fatalf("no rtpmap for %s", pt)
	return ""
}

func TestPeer_PublishOfferPutsPlanCodecFirst(t *testing.T) {
	var vp8Offer string
	for codec, want := range map[media.VideoCodec]string{media.CodecVP8: "VP8", media.CodecVP9: "VP9", media.CodecAV1: "AV1", media.CodecH264: "H264"} {
		p, err := NewPeer("peer-pref-"+string(codec), "scenario-pref", PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: codec, PublishSession: true}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		offer, err := p.CreatePublishOffer(func(string, string) {})
		if err != nil {
			t.Fatal(err)
		}
		if got := firstVideoCodec(t, offer); got != want {
			t.Errorf("%s: first video codec %s", codec, got)
		}
		if codec == media.CodecH264 && !strings.Contains(mediaSection(offer, "m=video"), "profile-level-id=42e01f") {
			t.Errorf("H.264 offer without Constrained Baseline")
		}
		if codec == media.CodecVP8 {
			vp8Offer = mediaSection(offer, "m=video")
		}
	}
	// VP8's offer keeps Pion's default order: every codec, VP8 first.
	for _, c := range []string{"VP8/90000", "VP9/90000", "H264/90000", "AV1/90000"} {
		if !strings.Contains(vp8Offer, c) {
			t.Errorf("VP8 offer lacks %s", c)
		}
	}
}

// mediasoup's first receive offer has only the data channel section. Pion
// takes a packet of an unknown SSRC (mediasoup's bandwidth probe) for that
// section's and adds a sendrecv video transceiver of its own, sending a
// sample track in Pion's first video codec (VP8), which the next video
// section then reuses: binding VP8 fails when the consumer is VP9 (014
// T030). The receiving session of a bot that publishes apart never sends.
func TestPeer_ReceivingSessionDropsPionSenders(t *testing.T) {
	p, err := NewPeer("bot-recv", "scenario-stray", PeerMedia{Kinds: []string{media.KindAudio, media.KindVideo}, PublishSession: true, VideoCodec: media.CodecVP9}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	// What Pion's undeclared SSRC handling does (peerconnection.go).
	if _, err := p.pc.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendrecv}); err != nil {
		t.Fatal(err)
	}
	sender, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sender.Close() })
	tr, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/VP9", ClockRate: 90000, SDPFmtpLine: "profile-id=0"}, "v", "remote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sender.AddTransceiverFromTrack(tr, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		t.Fatal(err)
	}
	offer, err := sender.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	answer, err := p.Renegotiate(sender.LocalDescription().SDP)
	if err != nil {
		t.Fatalf("Renegotiate: %v", err)
	}
	if sec := mediaSection(answer, "m=video"); !strings.Contains(sec, "a=recvonly") {
		t.Fatalf("video answered:\n%s", sec)
	}
}

// Scalable VP9 carries its layers in the dependency descriptor too, so the
// descriptor is negotiated for it; single-stream VP9 goes without.
func TestPeer_DependencyDescriptorForSVC(t *testing.T) {
	for svc, want := range map[bool]bool{true: true, false: false} {
		m := &webrtc.MediaEngine{}
		if err := m.RegisterDefaultCodecs(); err != nil {
			t.Fatal(err)
		}
		if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: media.DependencyDescriptorURI}, webrtc.RTPCodecTypeVideo); err != nil {
			t.Fatal(err)
		}
		offerer, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { offerer.Close() })
		if _, err := offerer.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
			t.Fatal(err)
		}
		p, err := NewPeer("peer-dd-svc", "scenario-dd", PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: media.CodecVP9, SVC: svc}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		p.transition(PeerStateConnecting)
		if got := strings.Contains(negotiate(t, offerer, p), media.DependencyDescriptorURI); got != want {
			t.Errorf("VP9 svc=%v: descriptor in the answer %v, want %v", svc, got, want)
		}
	}
}

// A simulcast layer names itself in every packet where the session
// negotiated the MID and RID extensions: the servers the bots publish to
// tell the layers apart by RID (014 T054).
func TestVideoTrack_WritesMIDAndRID(t *testing.T) {
	tr := newVideoTrack(media.CodecH264, "video", "bot", "m")
	tr.setMID(func() string { return "1" })
	h264 := []webrtc.RTPCodecParameters{codecParams("video/H264", "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f", 102)}
	with := &fakeBinding{id: "a", ssrc: 1, w: &recordingWriter{}, codecs: h264, exts: []webrtc.RTPHeaderExtensionParameter{
		{URI: "urn:ietf:params:rtp-hdrext:sdes:mid", ID: 4}, {URI: "urn:ietf:params:rtp-hdrext:sdes:rtp-stream-id", ID: 10},
	}}
	without := &fakeBinding{id: "b", ssrc: 2, w: &recordingWriter{}, codecs: h264}
	for _, b := range []*fakeBinding{with, without} {
		if _, err := tr.Bind(b); err != nil {
			t.Fatal(err)
		}
	}
	p := &rtp.Packet{Header: rtp.Header{Version: 2}, Payload: []byte{9}}
	if err := tr.WriteRTP(p); err != nil {
		t.Fatal(err)
	}
	h := with.w.hdrs[0]
	if string(h.GetExtension(4)) != "1" || string(h.GetExtension(10)) != "m" {
		t.Errorf("MID %q, RID %q", h.GetExtension(4), h.GetExtension(10))
	}
	if h := without.w.hdrs[0]; h.Extension || len(h.Extensions) != 0 {
		t.Errorf("binding without the extensions got %+v", h.Extensions)
	}
	if p.Header.Extension {
		t.Error("the caller's packet changed")
	}
}

// A simulcast bot's publish offer negotiates the MID and RID extensions and
// names its three layers; a single-stream bot's offer does neither.
func TestPeer_PublishOfferSimulcast(t *testing.T) {
	for _, simulcast := range []bool{true, false} {
		p, err := NewPeer("peer-pub-sim", "scenario-pub-sim", PeerMedia{Kinds: []string{media.KindVideo}, VideoCodec: media.CodecH264, Simulcast: simulcast, LayerHeights: []int{180, 360, 720}, PublishSession: true}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		offer, err := p.CreatePublishOffer(func(string, string) {})
		if err != nil {
			t.Fatal(err)
		}
		video := mediaSection(offer, "m=video")
		for _, want := range []string{"sdes:rtp-stream-id", "sdes:mid", "a=simulcast:send l;m;h", "a=rid:h send"} {
			if strings.Contains(video, want) != simulcast {
				t.Errorf("simulcast %v: %q in the offer %v", simulcast, want, !simulcast)
			}
		}
	}
}
