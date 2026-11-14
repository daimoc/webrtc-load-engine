package execution

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/pion/webrtc/v3"
)

// What the bots tell the server (015 US2, research R6): packets arriving at
// the socket, packets acknowledged, feedback timing and stamp lag.

func negotiatedFeedback(t *testing.T) *transportFeedback {
	t.Helper()
	f := newTransportFeedback()
	var parsed sdp.SessionDescription
	if err := parsed.Unmarshal([]byte(offerWithTWCC)); err != nil {
		t.Fatal(err)
	}
	f.setExtension(&parsed)
	f.counting.Store(true)
	return f
}

func TestTransportFeedback_ArriveCountsRTPWithSequence(t *testing.T) {
	f := newTransportFeedback()
	f.counting.Store(true)
	now := time.Now()
	f.arrive(twccPacket(t, 11, 1, 5), now) // Before the offer: no extension ID yet
	if s := f.snapshot(); s.arrivedTotal != 0 {
		t.Fatalf("counted %d before the offer", s.arrivedTotal)
	}
	f = negotiatedFeedback(t)
	f.arrive(twccPacket(t, 11, 1, 5), now)
	f.arrive(twccPacket(t, 99, 2, 5), now) // Another stream: still counted
	f.arrive(twccPacket(t, 11, 3, 0), now) // No transport-cc extension
	f.arrive(twccPacket(t, 11, 4, 7), now) // Another extension ID
	rr, _ := (&rtcp.ReceiverReport{SSRC: 1}).Marshal()
	f.arrive(rr, now)                                         // RTCP
	f.arrive([]byte{0, 1, 0, 0, 0x21, 0x12, 0xa4, 0x42}, now) // STUN
	f.arrive([]byte{22, 254, 253, 0, 0}, now)                 // DTLS
	f.arrive([]byte{0x80}, now)                               // Too short
	if s := f.snapshot(); s.arrivedTotal != 2 {
		t.Fatalf("arrived %d, want 2", s.arrivedTotal)
	}
}

func TestTransportFeedback_AcknowledgedFromSentFeedback(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false) // The buffer path: arrive counts, record acknowledges
	now := time.Now()
	for seq := uint16(1); seq <= 5; seq++ {
		pkt := twccPacket(t, 11, seq, 5)
		f.arrive(pkt, now)
		if seq != 3 { // Seq 3 reaches the socket and is never recorded
			f.recordFromBuffer(pkt, now)
		}
	}
	failing := func([]rtcp.Packet) error { return errors.New("transport not up") }
	f.send(failing)
	if s := f.snapshot(); s.acknowledged != 0 || s.feedbacks != 0 {
		t.Fatalf("a failed write counted: %+v", s)
	}
	// The failed feedback's packets are gone from the recorder: the next
	// feedback acknowledges only what came after.
	for seq := uint16(6); seq <= 9; seq++ {
		pkt := twccPacket(t, 11, seq, 5)
		f.arrive(pkt, now)
		if seq != 8 {
			f.recordFromBuffer(pkt, now)
		}
	}
	var sent []rtcp.Packet
	f.send(func(p []rtcp.Packet) error { sent = p; return nil })
	s := f.snapshot()
	if s.acknowledged != uint64(acknowledged(t, sent)) || s.acknowledged != 3 {
		t.Fatalf("acknowledged %d, feedback had %d, want 3", s.acknowledged, acknowledged(t, sent))
	}
	if s.feedbacks != 1 || s.arrived != 9 {
		t.Fatalf("feedbacks %d, arrived %d; want 1, 9", s.feedbacks, s.arrived)
	}
}

func TestTransportFeedback_IntervalsAndLate(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false)
	clock := f.start.Add(time.Second)
	f.now = func() time.Time { return clock }
	ok := func([]rtcp.Packet) error { return nil }
	seq := uint16(0)
	tick := func(after time.Duration) {
		clock = clock.Add(after)
		seq++
		pkt := twccPacket(t, 11, seq, 5)
		f.arrive(pkt, clock)
		f.recordFromBuffer(pkt, clock)
		f.send(ok)
	}
	tick(0)
	for _, d := range []time.Duration{100, 100, 100, 450, 100} {
		tick(d * time.Millisecond)
	}
	s := f.snapshot()
	if s.feedbacks != 6 || s.intervals != 5 {
		t.Fatalf("feedbacks %d, intervals %d", s.feedbacks, s.intervals)
	}
	if s.intervalMax != 450*time.Millisecond || s.intervalSum != 850*time.Millisecond || s.late != 1 {
		t.Fatalf("max %s, sum %s, late %d", s.intervalMax, s.intervalSum, s.late)
	}
	// A tick with nothing to acknowledge sends nothing and is no interval.
	clock = clock.Add(100 * time.Millisecond)
	f.send(ok)
	if s := f.snapshot(); s.feedbacks != 6 {
		t.Fatalf("an empty feedback was counted: %d", s.feedbacks)
	}
}

func TestTransportFeedback_StampLag(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false) // Stamp lag exists only on the buffer path
	at := f.start.Add(time.Second)
	a, b := twccPacket(t, 11, 1, 5), twccPacket(t, 11, 2, 5)
	f.arrive(a, at)
	f.arrive(b, at)
	f.recordFromBuffer(a, at.Add(3*time.Millisecond))
	f.recordFromBuffer(b, at.Add(1*time.Millisecond))
	s := f.snapshot()
	if s.lagN != 2 || s.lagMax != 3*time.Millisecond || s.lagSum != 4*time.Millisecond {
		t.Fatalf("lag n %d, max %s, sum %s", s.lagN, s.lagMax, s.lagSum)
	}
	// A packet recorded without a socket time (network not counted) adds no lag.
	f.recordFromBuffer(twccPacket(t, 11, 3, 5), at)
	if s := f.snapshot(); s.lagN != 2 {
		t.Fatalf("lag n %d after a packet with no socket time", s.lagN)
	}
}

func TestTransportFeedback_NotCountedAfterClose(t *testing.T) {
	f := negotiatedFeedback(t)
	f.arrive(twccPacket(t, 11, 1, 5), time.Now())
	f.close()
	f.arrive(twccPacket(t, 11, 2, 5), time.Now())
	if s := f.snapshot(); s.arrivedTotal != 1 {
		t.Fatalf("arrived %d, want 1", s.arrivedTotal)
	}
}

// Counting allocates nothing; recording at the socket costs what Pion's
// recorder costs, as it did at the read buffer (measured at 100 bots).
func TestTransportFeedback_ArriveAllocatesNothing(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false)
	pkt := twccPacket(t, 11, 1, 5)
	now := time.Now()
	if n := testing.AllocsPerRun(1000, func() { f.arrive(pkt, now) }); n != 0 {
		t.Fatalf("arrive allocates %v times per packet", n)
	}
}

func TestTransportFeedback_SnapshotReasons(t *testing.T) {
	f := newTransportFeedback()
	if s := f.snapshot(); s.negotiated || s.counting {
		t.Fatalf("fresh feedback: %+v", s)
	}
	f = negotiatedFeedback(t)
	if s := f.snapshot(); !s.negotiated || !s.counting {
		t.Fatalf("negotiated feedback: %+v", s)
	}
}

// SC-002: a bot made to leave packets unacknowledged is named. Two real
// receivers fed over loopback by Pion senders (which add the transport-cc
// extension); one skips the recording of 1 packet in 20.
func TestReceptionReports_UnacknowledgedBotNamed(t *testing.T) {
	oldEvery, oldPeer := skipRecordEvery, skipRecordPeer
	skipRecordEvery, skipRecordPeer = 20, "peer-sc002-0"
	t.Cleanup(func() { skipRecordEvery, skipRecordPeer = oldEvery, oldPeer })

	id := "sc002-" + uuid.NewString()
	var peers []mediaPeer
	var tracks []*webrtc.TrackLocalStaticRTP
	for i := 0; i < 2; i++ {
		offerer, track := twccVideoOfferer(t)
		p, err := NewPeer(fmt.Sprintf("peer-sc002-%d", i), id, PeerMedia{}, quietLogger)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { p.Close() })
		p.transition(PeerStateConnecting)
		negotiate(t, offerer, p)
		waitForState(t, p, PeerStateEstablished, 10*time.Second)
		peers = append(peers, p)
		tracks = append(tracks, track)
	}
	stop := make(chan struct{})
	go pump(stop, tracks...)
	c := newMediaCollector(id, 10*time.Second, quietLogger)
	deadline := time.Now().Add(10 * time.Second)
	for {
		time.Sleep(500 * time.Millisecond)
		c.collect(time.Now(), peers)
		if peers[0].ReceptionReports().arrived > 300 && peers[1].ReceptionReports().arrived > 300 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("too few packets: %+v %+v", peers[0].ReceptionReports(), peers[1].ReceptionReports())
		}
	}
	close(stop)
	r := c.summary(time.Now()).ReceptionReports
	if !r.Available || len(r.PerBot) != 2 {
		t.Fatalf("reception reports %+v", r)
	}
	skipping, normal := r.PerBot[0], r.PerBot[1]
	if skipping.AcknowledgedShare < 0.93 || skipping.AcknowledgedShare > 0.97 {
		t.Errorf("skipping bot acknowledged %.3f, want about 0.95", skipping.AcknowledgedShare)
	}
	if normal.AcknowledgedShare < ackShareMin {
		t.Errorf("normal bot acknowledged %.3f", normal.AcknowledgedShare)
	}
	if !slices.Equal(r.Flagged, []string{"peer-sc002-0"}) {
		t.Errorf("flagged %v, want only peer-sc002-0", r.Flagged)
	}
}

// twccVideoOfferer is videoOfferer whose packets carry a transport-wide
// sequence number, as media servers send them (Pion's default interceptors
// only generate feedback).
func twccVideoOfferer(t *testing.T) (*webrtc.PeerConnection, *webrtc.TrackLocalStaticRTP) {
	t.Helper()
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		t.Fatal(err)
	}
	registry := &interceptor.Registry{}
	if err := webrtc.RegisterDefaultInterceptors(m, registry); err != nil {
		t.Fatal(err)
	}
	if err := webrtc.ConfigureTWCCHeaderExtensionSender(m, registry); err != nil {
		t.Fatal(err)
	}
	pc, err := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(registry)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	tr, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8, ClockRate: 90000}, "video", "remote")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pc.AddTrack(tr); err != nil {
		t.Fatal(err)
	}
	return pc, tr
}

// Correction of the Jitsi verdict (015 T041): the feedback records every
// packet read from the socket, at the time it is read, so a packet Pion
// drops afterwards (a duplicate the SRTP replay check refuses, a stream it
// never opens) is still acknowledged.
func TestTransportFeedback_RecordsAtTheSocket(t *testing.T) {
	f := negotiatedFeedback(t)
	at := f.start.Add(time.Second)
	f.arrive(twccPacketSeq(t, 11, 1, 1), at)
	// The same RTP packet again, with its own transport sequence number, as
	// the bridge sends it; then a stream Pion never opens.
	f.arrive(twccPacketSeq(t, 11, 1, 2), at.Add(time.Millisecond))
	f.arrive(twccPacketSeq(t, 99, 7, 3), at)
	var sent []rtcp.Packet
	f.send(func(p []rtcp.Packet) error { sent = p; return nil })
	if n := acknowledged(t, sent); n != 3 {
		t.Fatalf("acknowledged %d, want 3", n)
	}
	// The buffer path does not record when the sockets are seen: a packet
	// written to the read buffer after its arrival is not reported twice.
	f.recordFromBuffer(twccPacketSeq(t, 11, 2, 4), at)
	sent = nil
	f.send(func(p []rtcp.Packet) error { sent = p; return nil })
	if n := acknowledged(t, sent); n != 0 {
		t.Fatalf("the read buffer recorded %d packets although the sockets are seen", n)
	}
	if s := f.snapshot(); s.arrived != 3 || s.acknowledged != 3 {
		t.Fatalf("arrived %d acknowledged %d, want 3 and 3", s.arrived, s.acknowledged)
	}
}

func TestTransportFeedback_BufferRecordsWithoutSockets(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false) // A test replaced the network
	f.recordFromBuffer(twccPacket(t, 11, 1, 5), f.start.Add(time.Second))
	var sent []rtcp.Packet
	f.send(func(p []rtcp.Packet) error { sent = p; return nil })
	if n := acknowledged(t, sent); n != 1 {
		t.Fatalf("acknowledged %d without counted sockets, want 1", n)
	}
}

func TestTransportFeedback_ArrivalTimeIsTheSocketTime(t *testing.T) {
	f := negotiatedFeedback(t)
	at := f.start.Add(time.Second)
	f.arrive(twccPacket(t, 11, 1, 5), at)
	f.arrive(twccPacket(t, 11, 2, 5), at.Add(20*time.Millisecond))
	var sent []rtcp.Packet
	f.send(func(p []rtcp.Packet) error { sent = p; return nil })
	cc := sent[0].(*rtcp.TransportLayerCC)
	if len(cc.RecvDeltas) != 2 || cc.RecvDeltas[1].Delta != 20000 {
		t.Fatalf("deltas %+v, want the second 20 ms (20000 µs) after the first", cc.RecvDeltas)
	}
}

// twccPacketSeq is twccPacket with a transport sequence number apart from
// the RTP one.
func twccPacketSeq(t *testing.T, ssrc uint32, rtpSeq, transportSeq uint16) []byte {
	t.Helper()
	p := rtp.Packet{Header: rtp.Header{Version: 2, PayloadType: 111, SSRC: ssrc, SequenceNumber: rtpSeq}, Payload: []byte{1, 2, 3}}
	ext, err := (&rtp.TransportCCExtension{TransportSequence: transportSeq}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Header.SetExtension(5, ext); err != nil {
		t.Fatal(err)
	}
	b, err := p.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return b
}
