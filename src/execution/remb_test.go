package execution

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

const testAbsID = 3

// absPacket is an RTP packet of size bytes with an abs-send-time of at.
func absPacket(t *testing.T, ssrc uint32, seq uint16, at time.Duration, size int) []byte {
	t.Helper()
	p := rtp.Packet{Header: rtp.Header{Version: 2, PayloadType: 96, SSRC: ssrc, SequenceNumber: seq}}
	v := uint32(at.Seconds()*(1<<18)) & 0xFFFFFF
	if err := p.Header.SetExtension(testAbsID, []byte{byte(v >> 16), byte(v >> 8), byte(v)}); err != nil {
		t.Fatal(err)
	}
	head, err := p.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if size < len(head) {
		size = len(head)
	}
	return append(head, make([]byte, size-len(head))...)
}

func TestAbsSendTime(t *testing.T) {
	pkt := absPacket(t, 1, 1, 1500*time.Millisecond, 100)
	v, ok := absSendTime(pkt, testAbsID)
	if !ok || math.Abs(float64(v)/(1<<18)-1.5) > 1e-5 {
		t.Errorf("abs-send-time %d (%v), want 1.5 s", v, ok)
	}
	if _, ok := absSendTime(pkt, 4); ok {
		t.Error("found an extension that is not there")
	}
	// Unwrapping across the 64 s wrap.
	var u absUnwrapper
	a := u.unwrap(0xFFFFF0)
	b := u.unwrap(0x000010)
	if d := b - a; d != 0x20 {
		t.Errorf("unwrapped delta %d, want 32", d)
	}
}

// feed sends packets of 1200 bytes at kbps for seconds, each arriving
// after a delay that grows by growMsPerS per second of send time, and
// returns the REMBs sent.
func feed(t *testing.T, e *rembEstimator, start time.Time, from, seconds float64, kbps int, growMsPerS float64) {
	t.Helper()
	interval := time.Duration(float64(time.Second) * 1200 * 8 / float64(kbps*1000))
	seq := uint16(from * 1000)
	for s := time.Duration(from * float64(time.Second)); s < time.Duration((from+seconds)*float64(time.Second)); s += interval {
		delay := 20*time.Millisecond + time.Duration(growMsPerS*s.Seconds()*float64(time.Millisecond))
		if growMsPerS != 0 {
			delay = 20*time.Millisecond + time.Duration(growMsPerS*(s.Seconds()-from)*float64(time.Millisecond))
		}
		seq++
		e.arrive(absPacket(t, 7, seq, s, 1200), start.Add(s+delay))
	}
}

func TestREMB_FlatDelayGrows(t *testing.T) {
	var sent []*rtcp.ReceiverEstimatedMaximumBitrate
	e := newREMBEstimator(func(r *rtcp.ReceiverEstimatedMaximumBitrate) { sent = append(sent, r) })
	e.setExtension(testAbsID)
	start := time.Unix(1000, 0)
	feed(t, e, start, 0, 20, 1000, 0)
	if len(sent) < 15 {
		t.Fatalf("%d REMBs in 20 s, want about one per second", len(sent))
	}
	last := sent[len(sent)-1]
	if last.Bitrate < 1000e3 || last.Bitrate > 1.6e6 { // 1.5 × the incoming rate, headers counted, plus 10 kbit/s
		t.Errorf("estimate %.0f after 20 s at 1 Mbit/s with a flat delay, want from the incoming rate up to 1.5 times it", last.Bitrate)
	}
	if len(last.SSRCs) != 1 || last.SSRCs[0] != 7 {
		t.Errorf("REMB names %v, want [7]", last.SSRCs)
	}
}

func TestREMB_GrowingDelayDecreases(t *testing.T) {
	var sent []*rtcp.ReceiverEstimatedMaximumBitrate
	e := newREMBEstimator(func(r *rtcp.ReceiverEstimatedMaximumBitrate) { sent = append(sent, r) })
	e.setExtension(testAbsID)
	start := time.Unix(1000, 0)
	feed(t, e, start, 0, 10, 1000, 0)
	before := sent[len(sent)-1].Bitrate
	n := len(sent)
	// A queue building up: 100 ms more delay per second.
	feed(t, e, start, 10, 3, 1000, 100)
	low := before
	for _, r := range sent[n:] {
		low = min(low, r.Bitrate)
	}
	if low > 0.9*before || low > 0.9e6 {
		t.Errorf("estimate %.0f under a growing delay (from %.0f), want at most 0.85 times the incoming rate", low, before)
	}
	// A fall is sent at once, not at the next second.
	if len(sent) == n {
		t.Error("no REMB after the overuse")
	}
}

func TestREMB_NotEnabledWithoutExtension(t *testing.T) {
	var sent int
	e := newREMBEstimator(func(*rtcp.ReceiverEstimatedMaximumBitrate) { sent++ })
	feed(t, e, time.Unix(1000, 0), 0, 3, 1000, 0)
	if sent != 0 {
		t.Errorf("%d REMBs without the extension negotiated", sent)
	}
}

func TestTrendline(t *testing.T) {
	flat, growing := newTrendline(), newTrendline()
	var flatOver, growOver bool
	at := 0.0
	for i := 0; i < 100; i++ {
		at += 10
		if flat.update(10, 10, at) == usageOveruse {
			flatOver = true
		}
		if growing.update(12, 10, at) == usageOveruse { // 2 ms more per 10 ms group
			growOver = true
		}
	}
	if flatOver {
		t.Error("overuse on a flat delay")
	}
	if !growOver {
		t.Error("no overuse on a growing delay")
	}
}

func TestREMBNegotiated(t *testing.T) {
	const head = "v=0\r\no=- 1 1 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 96\r\nc=IN IP4 0.0.0.0\r\na=rtpmap:96 VP8/90000\r\n"
	cases := []struct {
		name, lines string
		want        uint8
	}{
		{"livekit: remb, abs-send-time, no transport-cc", "a=rtcp-fb:96 goog-remb\r\na=extmap:3 " + absSendTimeURI + "\r\n", 3},
		{"jitsi: transport-cc too", "a=rtcp-fb:96 goog-remb\r\na=rtcp-fb:96 transport-cc\r\na=extmap:3 " + absSendTimeURI + "\r\na=extmap:5 " + sdp.TransportCCURI + "\r\n", 0},
		{"transport-cc extension only", "a=rtcp-fb:96 goog-remb\r\na=extmap:3 " + absSendTimeURI + "\r\na=extmap:5 " + sdp.TransportCCURI + "\r\n", 0},
		{"no remb", "a=extmap:3 " + absSendTimeURI + "\r\n", 0},
		{"remb without abs-send-time", "a=rtcp-fb:96 goog-remb\r\n", 0},
	}
	for _, c := range cases {
		var d sdp.SessionDescription
		if err := d.Unmarshal([]byte(head + c.lines)); err != nil {
			t.Fatal(err)
		}
		if got := rembNegotiated(&d); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

// A peer's receive session answers with abs-send-time and goog-remb, as a
// browser does, so a server that asks for REMB gets the extension it
// needs.
func TestPeer_AnswersREMB(t *testing.T) {
	p, err := NewPeer("peer-remb", "sc", PeerMedia{}, quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer p.closeConnections()
	// A LiveKit-like offerer: default codecs (with goog-remb) and
	// abs-send-time.
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: absSendTimeURI}, webrtc.RTPCodecTypeVideo); err != nil {
		t.Fatal(err)
	}
	offerer, err := webrtc.NewAPI(webrtc.WithMediaEngine(m)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	defer offerer.Close()
	if _, err := offerer.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo, webrtc.RTPTransceiverInit{Direction: webrtc.RTPTransceiverDirectionSendonly}); err != nil {
		t.Fatal(err)
	}
	offer, err := offerer.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.pc.SetRemoteDescription(offer); err != nil {
		t.Fatal(err)
	}
	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(answer.SDP, absSendTimeURI) || !strings.Contains(answer.SDP, "goog-remb") {
		t.Errorf("answer lacks abs-send-time or goog-remb:\n%s", answer.SDP)
	}
}
