package mediasoup

import (
	"slices"
	"strings"
	"testing"

	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/media"
)

// audioConsumer returns the golden newConsumer (Opus, mid "0").
func audioConsumer(t *testing.T) NewConsumerRequest {
	t.Helper()
	var nc NewConsumerRequest
	goldenData(t, "new_consumer_audio", &nc)
	return nc
}

// videoConsumer is a VP8 consumer with RTX, as the router of the goldens
// would give one (PT 101, rtx 102).
func videoConsumer() NewConsumerRequest {
	return NewConsumerRequest{
		PeerID: "peerB", ConsumerID: "video-consumer-1", ProducerID: "video-producer-1", Kind: "video", Type: "simple",
		RtpParameters: RtpParameters{
			MID: "1",
			Codecs: []RtpCodecParameters{
				{MimeType: "video/VP8", PayloadType: 101, ClockRate: 90000, Parameters: map[string]any{},
					RtcpFeedback: []RtcpFeedback{{Type: "nack"}, {Type: "nack", Parameter: "pli"}, {Type: "ccm", Parameter: "fir"}, {Type: "transport-cc"}}},
				{MimeType: "video/rtx", PayloadType: 102, ClockRate: 90000, Parameters: map[string]any{"apt": float64(101)}},
			},
			HeaderExtensions: []RtpHeaderExtensionParameters{
				{URI: "urn:ietf:params:rtp-hdrext:sdes:mid", ID: 1},
				{URI: "http://www.webrtc.org/experiments/rtp-hdrext/abs-send-time", ID: 4},
				{URI: "http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01", ID: 5},
			},
			Encodings: []RtpEncodingParameters{{SSRC: 2222, RTX: &RtxSSRC{SSRC: 3333}}},
			RTCP:      RtcpParameters{CNAME: "probe", ReducedSize: true},
		},
	}
}

func parseSDP(t *testing.T, s string) *sdp.SessionDescription {
	t.Helper()
	var d sdp.SessionDescription
	if err := d.Unmarshal([]byte(s)); err != nil {
		t.Fatalf("unparsable SDP: %v\n%s", err, s)
	}
	return &d
}

// offeredByKind counts the sections a peer counts as offered streams: with
// ssrc lines, not inactive nor recvonly (execution.Peer.recordOffered).
func offeredByKind(d *sdp.SessionDescription) map[string]int {
	n := map[string]int{}
	for _, md := range d.MediaDescriptions {
		if _, ok := md.Attribute("ssrc"); !ok {
			continue
		}
		if _, ok := md.Attribute("inactive"); ok {
			continue
		}
		if _, ok := md.Attribute("recvonly"); ok {
			continue
		}
		n[md.MediaName.Media]++
	}
	return n
}

// answerWithPion applies offer to a fresh Pion PeerConnection and returns
// its answer.
func answerWithPion(t *testing.T, pc *webrtc.PeerConnection, offer string) string {
	t.Helper()
	if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer}); err != nil {
		t.Fatalf("SetRemoteDescription: %v\n%s", err, offer)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		t.Fatal(err)
	}
	return answer.SDP
}

func TestRecvOffer_ApplicationOnly(t *testing.T) {
	o := newRecvOffer(consumerTransport(t))
	s, err := o.SDP()
	if err != nil {
		t.Fatal(err)
	}
	d := parseSDP(t, s)
	if len(d.MediaDescriptions) != 1 || d.MediaDescriptions[0].MediaName.Media != "application" {
		t.Fatalf("sections:\n%s", s)
	}
	for _, want := range []string{"a=ice-lite", "a=group:BUNDLE datachannel", "a=mid:datachannel", "a=sctp-port:5000", "a=max-message-size:5000000",
		"a=setup:actpass", "a=fingerprint:sha-256 EB:AE:F8", "a=ice-ufrag:kasxoor7z12vtd2qfrv2i4jo4h4eup7z", "127.0.0.1 44444 typ host"} {
		if !strings.Contains(s, want) {
			t.Errorf("offer lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, " tcp ") {
		t.Errorf("offer has a TCP candidate:\n%s", s)
	}
	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	answer := answerWithPion(t, pc, s)
	// Pion answers an ICE-lite offer with setup:passive (DTLS server).
	if role, err := localDtlsRole(answer); err != nil || role != "server" {
		t.Errorf("localDtlsRole(answer) = %q, %v, want server:\n%s", role, err, answer)
	}
	s2, _ := o.SDP()
	if o1, o2 := parseSDP(t, s).Origin.SessionVersion, parseSDP(t, s2).Origin.SessionVersion; o2 <= o1 {
		t.Errorf("session version %d then %d, want increasing", o1, o2)
	}
}

func TestRecvOffer_Consumers(t *testing.T) {
	o := newRecvOffer(consumerTransport(t))
	a, v := audioConsumer(t), videoConsumer()
	o.add(a)
	o.add(v)
	s, err := o.SDP()
	if err != nil {
		t.Fatal(err)
	}
	d := parseSDP(t, s)
	var mids []string
	for _, md := range d.MediaDescriptions {
		mid, _ := md.Attribute("mid")
		mids = append(mids, mid)
	}
	if !slices.Equal(mids, []string{"datachannel", "0", "1"}) {
		t.Errorf("mids = %v", mids)
	}
	for _, want := range []string{"a=group:BUNDLE datachannel 0 1", "a=rtpmap:100 opus/48000/2", "a=fmtp:100 minptime=10;useinbandfec=1", "a=rtcp-fb:100 nack",
		"a=rtpmap:101 VP8/90000", "a=rtcp-fb:101 nack pli", "a=rtcp-fb:101 transport-cc", "a=rtpmap:102 rtx/90000", "a=fmtp:102 apt=101",
		"a=extmap:5 http://www.ietf.org/id/draft-holmer-rmcat-transport-wide-cc-extensions-01", "a=ssrc-group:FID 2222 3333", "a=ssrc:2222 cname:probe",
		"a=ssrc:3333 cname:probe", "a=msid:peerB video-consumer-1", "a=sendonly"} {
		if !strings.Contains(s, want) {
			t.Errorf("offer lacks %q:\n%s", want, s)
		}
	}
	if got := offeredByKind(d); got["audio"] != 1 || got["video"] != 1 {
		t.Errorf("offered = %v, want 1 audio and 1 video", got)
	}

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	answerWithPion(t, pc, s)

	// Closing the audio consumer leaves its section, inactive, without SSRC.
	if !o.deactivate(a.ConsumerID) {
		t.Fatal("deactivate: consumer not found")
	}
	s, err = o.SDP()
	if err != nil {
		t.Fatal(err)
	}
	d = parseSDP(t, s)
	if got := offeredByKind(d); got["audio"] != 0 || got["video"] != 1 {
		t.Errorf("after close, offered = %v, want 0 audio and 1 video", got)
	}
	if len(d.MediaDescriptions) != 3 {
		t.Errorf("after close, %d sections, want 3", len(d.MediaDescriptions))
	}
	audio := d.MediaDescriptions[1]
	if _, ok := audio.Attribute("inactive"); !ok {
		t.Errorf("closed section not inactive:\n%s", s)
	}
	if _, ok := audio.Attribute("ssrc"); ok {
		t.Errorf("closed section keeps its ssrc:\n%s", s)
	}
	answerWithPion(t, pc, s)
	if o.deactivate("unknown") {
		t.Error("deactivate(unknown) = true")
	}
}

// publishOffer is a real publish offer of the engine's peer (audio and
// video, Pion's default codecs).
func publishOffer(t *testing.T, kinds []string) (*execution.Peer, string) {
	t.Helper()
	p, err := execution.NewPeer("bot-1", "sdp-test", execution.PeerMedia{Kinds: kinds, PublishSession: true}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })
	offer, err := p.CreatePublishOffer(func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	return p, offer
}

func producerTransport(t *testing.T) TransportInfo {
	t.Helper()
	var ti TransportInfo
	goldenData(t, "create_transport_producer", &ti)
	return ti
}

func TestProducerParams(t *testing.T) {
	_, offer := publishOffer(t, []string{media.KindAudio, media.KindVideo})
	d := parseSDP(t, offer)
	specs, err := producerParams(offer, routerCaps(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[0].Kind != "audio" || specs[1].Kind != "video" {
		t.Fatalf("specs = %+v", specs)
	}
	for i, spec := range specs {
		md := d.MediaDescriptions[i]
		mid, _ := md.Attribute("mid")
		rp := spec.RtpParameters
		if spec.MID != mid || rp.MID != mid {
			t.Errorf("%s: mid %q/%q, want %q", spec.Kind, spec.MID, rp.MID, mid)
		}
		if len(rp.Encodings) != 1 || rp.Encodings[0].SSRC == 0 || !strings.Contains(offer, "a=ssrc:"+itoa(int(rp.Encodings[0].SSRC))+" cname:bot-1") {
			t.Errorf("%s: encodings %+v not the offer's ssrc", spec.Kind, rp.Encodings)
		}
		if rp.RTCP.CNAME != "bot-1" || !rp.RTCP.ReducedSize {
			t.Errorf("%s: rtcp = %+v", spec.Kind, rp.RTCP)
		}
		for _, ext := range rp.HeaderExtensions {
			if !slices.ContainsFunc(routerCaps(t).HeaderExtensions, func(h RtpHeaderExtension) bool { return h.Kind == spec.Kind && h.URI == ext.URI }) {
				t.Errorf("%s: extension %s not supported by the router", spec.Kind, ext.URI)
			}
		}
	}
	a, v := specs[0].RtpParameters, specs[1].RtpParameters
	if len(a.Codecs) != 1 || a.Codecs[0].MimeType != "audio/opus" || a.Codecs[0].PayloadType != 111 || a.Codecs[0].Channels != 2 || a.Codecs[0].Parameters["useinbandfec"] != 1 {
		t.Errorf("audio codecs = %+v", a.Codecs)
	}
	if len(v.Codecs) != 2 || v.Codecs[0].MimeType != "video/VP8" || v.Codecs[0].PayloadType != 96 || v.Codecs[1].MimeType != "video/rtx" || v.Codecs[1].PayloadType != 97 || v.Codecs[1].Parameters["apt"] != 96 {
		t.Errorf("video codecs = %+v", v.Codecs)
	}
	var fb []string
	for _, f := range v.Codecs[0].RtcpFeedback {
		fb = append(fb, strings.TrimSpace(f.Type+" "+f.Parameter))
	}
	slices.Sort(fb)
	if !slices.Equal(fb, []string{"ccm fir", "goog-remb", "nack", "nack pli"}) {
		t.Errorf("VP8 feedback = %v (duplicates dropped)", fb)
	}
}

func TestProducerParams_MissingCodec(t *testing.T) {
	_, offer := publishOffer(t, []string{media.KindAudio})
	noOpus := strings.ReplaceAll(offer, "opus/48000/2", "speex/48000/2")
	if _, err := producerParams(noOpus, routerCaps(t)); err == nil || !strings.Contains(err.Error(), "opus") {
		t.Errorf("err = %v, want one naming opus", err)
	}
	caps := routerCaps(t)
	caps.Codecs = slices.DeleteFunc(caps.Codecs, func(c RtpCodecCapability) bool { return c.MimeType == "audio/opus" })
	if _, err := producerParams(offer, caps); err == nil || !strings.Contains(err.Error(), "opus") {
		t.Errorf("err = %v, want one naming opus (router)", err)
	}
}

func TestPublishAnswer(t *testing.T) {
	p, offer := publishOffer(t, []string{media.KindAudio, media.KindVideo})
	specs, err := producerParams(offer, routerCaps(t))
	if err != nil {
		t.Fatal(err)
	}
	answer, err := publishAnswer(offer, producerTransport(t), specs)
	if err != nil {
		t.Fatal(err)
	}
	d := parseSDP(t, answer)
	if len(d.MediaDescriptions) != 2 {
		t.Fatalf("answer sections:\n%s", answer)
	}
	for _, want := range []string{"a=setup:passive", "a=recvonly", "a=ice-lite", "a=mid:0", "a=mid:1", "a=rtpmap:111 opus/48000/2", "a=rtpmap:96 VP8/90000", "a=fmtp:97 apt=96"} {
		if !strings.Contains(answer, want) {
			t.Errorf("answer lacks %q:\n%s", want, answer)
		}
	}
	if strings.Contains(answer, "a=ssrc") {
		t.Errorf("answer has ssrc lines:\n%s", answer)
	}
	if err := p.ApplyPublishAnswer(answer); err != nil {
		t.Fatalf("ApplyPublishAnswer: %v\n%s", err, answer)
	}
}

func TestLocalFingerprint(t *testing.T) {
	_, offer := publishOffer(t, []string{media.KindAudio})
	fp, err := localFingerprint(offer)
	if err != nil {
		t.Fatal(err)
	}
	if fp.Algorithm != "sha-256" || len(fp.Value) != 95 {
		t.Errorf("fingerprint = %+v", fp)
	}
	// At media level only.
	media := strings.Replace(offer, "a=fingerprint:"+fp.Algorithm+" "+fp.Value+"\r\n", "", 1)
	media = strings.Replace(media, "a=mid:0\r\n", "a=mid:0\r\na=fingerprint:sha-256 "+fp.Value+"\r\n", 1)
	if got, err := localFingerprint(media); err != nil || got != fp {
		t.Errorf("media-level fingerprint = %+v, %v", got, err)
	}
	if _, err := localFingerprint("v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\n"); err == nil {
		t.Error("no fingerprint: no error")
	}
}

func TestLocalDtlsRole(t *testing.T) {
	_, offer := publishOffer(t, []string{media.KindAudio})
	// An offer's actpass: mediasoup's answer says passive, the bot is client.
	if role, err := localDtlsRole(offer); err != nil || role != "client" {
		t.Errorf("offer role = %q, %v, want client", role, err)
	}
	active := strings.Replace(offer, "a=setup:actpass", "a=setup:active", -1)
	if role, _ := localDtlsRole(active); role != "client" {
		t.Errorf("active role = %q, want client", role)
	}
	if _, err := localDtlsRole("v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\n"); err == nil {
		t.Error("no setup: no error")
	}
}
