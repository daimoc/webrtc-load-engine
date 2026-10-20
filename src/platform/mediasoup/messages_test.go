package mediasoup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// golden reads testdata/golden/<name>.json, a protoo frame recorded from
// mediasoup-demo 899109e (mediasoup 3.30.1, 010 T002).
func golden(t *testing.T, name string) map[string]json.RawMessage {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "golden", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]json.RawMessage
	if err := json.Unmarshal(b, &frame); err != nil {
		t.Fatal(err)
	}
	return frame
}

// goldenData decodes a golden frame's data into v.
func goldenData(t *testing.T, name string, v any) {
	t.Helper()
	if err := json.Unmarshal(golden(t, name)["data"], v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
}

func routerCaps(t *testing.T) RtpCapabilities {
	t.Helper()
	var r routerCapsResponse
	goldenData(t, "router_rtp_capabilities", &r)
	return r.RouterRtpCapabilities
}

func consumerTransport(t *testing.T) TransportInfo {
	t.Helper()
	var ti TransportInfo
	goldenData(t, "create_transport_consumer", &ti)
	return ti
}

func TestMessages_Goldens(t *testing.T) {
	caps := routerCaps(t)
	if len(caps.Codecs) != 9 || caps.Codecs[0].MimeType != "audio/opus" || caps.Codecs[0].PreferredPayloadType != 100 || caps.Codecs[0].Channels != 2 {
		t.Errorf("router codecs = %+v", caps.Codecs[:1])
	}
	if caps.Codecs[2].MimeType != "video/rtx" || caps.Codecs[2].Parameters["apt"] != float64(101) {
		t.Errorf("rtx codec = %+v", caps.Codecs[2])
	}
	if len(caps.HeaderExtensions) == 0 || caps.HeaderExtensions[0].Kind != "audio" || caps.HeaderExtensions[0].PreferredID != 1 {
		t.Errorf("header extensions = %+v", caps.HeaderExtensions[:1])
	}

	ti := consumerTransport(t)
	if ti.TransportID == "" || !ti.IceParameters.IceLite || ti.IceParameters.UsernameFragment == "" || ti.IceParameters.Password == "" {
		t.Errorf("transport = %+v", ti)
	}
	if len(ti.IceCandidates) != 2 || ti.IceCandidates[0].Protocol != "udp" || ti.IceCandidates[0].Port != 44444 || ti.IceCandidates[0].Address != "127.0.0.1" || ti.IceCandidates[0].Priority == 0 {
		t.Errorf("candidates = %+v", ti.IceCandidates)
	}
	if len(ti.DtlsParameters.Fingerprints) != 5 || ti.DtlsParameters.Fingerprints[2].Algorithm != "sha-256" {
		t.Errorf("dtls = %+v", ti.DtlsParameters)
	}
	if ti.SctpParameters == nil || ti.SctpParameters.Port != 5000 || ti.SctpParameters.MaxMessageSize != 5000000 {
		t.Errorf("sctp = %+v", ti.SctpParameters)
	}

	var nc NewConsumerRequest
	goldenData(t, "new_consumer_audio", &nc)
	rp := nc.RtpParameters
	if nc.ConsumerID == "" || nc.PeerID != "peerB" || nc.Kind != "audio" || nc.Type != "simple" || rp.MID != "0" || rp.RTCP.CNAME != "probe" {
		t.Errorf("newConsumer = %+v", nc)
	}
	if len(rp.Codecs) != 1 || rp.Codecs[0].PayloadType != 100 || rp.Codecs[0].RtcpFeedback[0].Type != "nack" || len(rp.Encodings) != 1 || rp.Encodings[0].SSRC == 0 {
		t.Errorf("rtpParameters = %+v", rp)
	}

	var closed consumerRef
	goldenData(t, "consumer_closed", &closed)
	var peer peerRef
	goldenData(t, "peer_closed", &peer)
	var v versionNotification
	goldenData(t, "mediasoup_version", &v)
	var pr produceResponse
	goldenData(t, "produce_response", &pr)
	if closed.ConsumerID != nc.ConsumerID || peer.PeerID != "peerB" || v.Version != "3.30.1" || pr.ProducerID != nc.ProducerID {
		t.Errorf("closed %+v peer %+v version %+v produce %+v", closed, peer, v, pr)
	}

	// Re-encoding keeps every field the adapter reads.
	b, err := json.Marshal(nc)
	if err != nil {
		t.Fatal(err)
	}
	var again NewConsumerRequest
	if err := json.Unmarshal(b, &again); err != nil {
		t.Fatal(err)
	}
	if again.RtpParameters.Encodings[0].SSRC != rp.Encodings[0].SSRC || again.RtpParameters.Codecs[0].Parameters["useinbandfec"] != float64(1) || len(again.RtpParameters.HeaderExtensions) != len(rp.HeaderExtensions) {
		t.Errorf("re-encoded = %+v", again)
	}
}
