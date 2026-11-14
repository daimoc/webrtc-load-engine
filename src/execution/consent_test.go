package execution

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pion/ice/v2"
	"github.com/pion/stun"
	"github.com/pion/webrtc/v3"
)

// liteServer is an ICE-lite PeerConnection, as SFU transports often are,
// that counts the Binding requests it receives once connected.
func liteServer(t *testing.T) (*webrtc.PeerConnection, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	var se webrtc.SettingEngine
	se.SetLite(true)
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
	se.SetICEBindingRequestHandler(func(*stun.Message, ice.Candidate, ice.Candidate, *ice.CandidatePair) bool {
		requests.Add(1)
		return false // Pion answers it as usual
	})
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	return pc, &requests
}

// TestConsent_ChecksWhileTrafficFlows checks RFC 7675 consent freshness
// (010 T016): a peer keeps sending Binding requests on its selected pair
// although the transport is busy, where Pion's keepalive would stay quiet.
func TestConsent_ChecksWhileTrafficFlows(t *testing.T) {
	old := consentInterval
	consentInterval = 200 * time.Millisecond
	t.Cleanup(func() { consentInterval = old })

	var logs lockedBuffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	p, err := NewPeer("bot", uuid.NewString(), PeerMedia{}, logger)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { p.Close() })

	server, requests := liteServer(t)
	dc, err := server.CreateDataChannel("busy", nil)
	if err != nil {
		t.Fatal(err)
	}
	offer, err := server.CreateOffer(nil)
	if err != nil {
		t.Fatal(err)
	}
	gathered := webrtc.GatheringCompletePromise(server)
	if err := server.SetLocalDescription(offer); err != nil {
		t.Fatal(err)
	}
	<-gathered
	answer, err := p.OnOffer(server.LocalDescription().SDP, func(string, string) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer}); err != nil {
		t.Fatal(err)
	}
	open := make(chan struct{})
	dc.OnOpen(func() { close(open) })
	select {
	case <-open:
	case <-time.After(10 * time.Second):
		t.Fatal("transport not connected")
	}

	// Keep the transport busy both ways, so Pion's keepalive stays quiet.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				dc.SendText("x")
			}
		}
	}()
	before := requests.Load()
	time.Sleep(2 * time.Second)
	if got := requests.Load() - before; got < 5 {
		t.Errorf("%d Binding requests in 2 s with a 200 ms consent interval, want at least 5", got)
	}
	if s := logs.String(); strings.Contains(s, "nknown TransactionID") {
		t.Errorf("consent responses reached Pion as unknown transactions:\n%s", s)
	}
}

// lockedBuffer is a log sink written from Pion's goroutines.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// The receiving connection's sockets count every RTP packet with a
// transport sequence number as it is read (015 T029); answers to consent
// checks and other datagrams are not counted.
func TestConsentNet_CountsArrivals(t *testing.T) {
	f := negotiatedFeedback(t)
	f.counting.Store(false)
	c, err := newConsent(quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	c.net.feedback = f
	conn, err := c.net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if !f.counting.Load() {
		t.Fatal("a socket opened through the net does not mark the peer counted")
	}
	sender, err := net.DialUDP("udp4", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	answer := stun.MustBuild(stun.TransactionID, stun.BindingSuccess)
	c.net.expect(answer.TransactionID)
	for _, pkt := range [][]byte{twccPacket(t, 11, 1, 5), answer.Raw, twccPacket(t, 11, 2, 0), twccPacket(t, 11, 3, 5)} {
		if _, err := sender.Write(pkt); err != nil {
			t.Fatal(err)
		}
	}
	buf := make([]byte, 1500)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 3; i++ { // The consent answer is skipped by the read
		if _, _, err := conn.ReadFrom(buf); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.snapshot().arrivedTotal; got != 2 {
		t.Fatalf("arrived %d, want 2", got)
	}
}

func TestConsentNet_PublishSessionNotCounted(t *testing.T) {
	c, err := newConsent(quietLogger)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := c.net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	conn.Close() // No feedback: nothing to count, and no panic
}
