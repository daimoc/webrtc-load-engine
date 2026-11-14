package execution

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"webrtc-load-engine/src/media"

	"github.com/pion/interceptor"
	"github.com/pion/logging"
	"github.com/pion/sdp/v3"
	"github.com/pion/transport/v2/packetio"
	"github.com/pion/webrtc/v3"
)

// Buffer sizes of Pion's default SRTP/SRTCP read streams.
const (
	rtpBufferLimit  = 1000 * 1000
	rtcpBufferLimit = 100 * 1000
)

// errNotOffered is returned by reads on a stream whose SSRC the applied
// offer does not list.
var errNotOffered = errors.New("ssrc not offered")

// settingEngineHook, when set (tests only), adjusts the settings of every
// new peer, e.g. to put it on a virtual network.
var settingEngineHook func(*webrtc.SettingEngine)

// newPeerConnection creates the peer's PeerConnection with Pion's default
// codecs and interceptors, plus two settings (research R12):
//   - incoming streams whose SSRC is neither in the applied offer nor one of
//     the peer's own (RTCP about them) are dropped as they arrive, instead of
//     filling a 1 MB buffer each: the media server may send more streams than
//     the peer receives (receive cap, its own sources);
//   - Pion's logs go to the peer's structured logger;
//   - the transport sends RFC 7675 consent checks while it is busy, as
//     browsers do (consent.go).
//
// The publish session (publish true, 008 research R4) receives no media:
// it has neither the receive gate nor transport-wide feedback.
func newPeerConnection(p *Peer, publish bool) (*webrtc.PeerConnection, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	// Pion's default interceptors, except its TWCC generator: the peer
	// acknowledges every arriving packet itself (transportFeedback), with the
	// same header extension and RTCP feedback negotiated.
	registry := &interceptor.Registry{}
	if err := webrtc.ConfigureNack(m, registry); err != nil {
		return nil, err
	}
	if err := webrtc.ConfigureRTCPReports(registry); err != nil {
		return nil, err
	}
	// The dependency descriptor carries AV1's frames and layers, and
	// scalable VP9's layers (014 research R6). Only those runs negotiate
	// it: with it, LiveKit reads VP8 layers from it too, which would change
	// VP8 runs.
	if p.videoCodec == media.CodecAV1 || p.svc {
		if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: media.DependencyDescriptorURI}, webrtc.RTPCodecTypeVideo); err != nil {
			return nil, err
		}
	}
	// Simulcast on a publish session: the servers tell the layers apart by
	// the MID and RID extensions, which the video track writes (014 T054).
	// Jitsi (no publish session) takes the layers by SSRC instead.
	if publish && p.simulcast {
		for _, uri := range []string{sdp.SDESMidURI, sdp.SDESRTPStreamIDURI} {
			if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: uri}, webrtc.RTPCodecTypeVideo); err != nil {
				return nil, err
			}
		}
	}
	if !publish {
		for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
			m.RegisterFeedback(webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBTransportCC}, kind)
			if err := m.RegisterHeaderExtension(webrtc.RTPHeaderExtensionCapability{URI: sdp.TransportCCURI}, kind); err != nil {
				return nil, err
			}
		}
	}

	var se webrtc.SettingEngine
	se.LoggerFactory = slogLoggerFactory{p.logger}
	c, err := newConsent(p.logger)
	if err != nil {
		return nil, err
	}
	se.SetNet(c.net) // A hook may replace it (virtual network): no consent checks then
	if !publish {
		c.net.feedback = p.feedback // Arrivals counted at the socket (015 research R6)
	}
	if publish {
		if settingEngineHook != nil {
			settingEngineHook(&se)
		}
		api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(registry), webrtc.WithSettingEngine(se))
		return c.start(api.NewPeerConnection(webrtc.Configuration{}))
	}
	se.BufferFactory = func(packetType packetio.BufferPacketType, ssrc uint32) io.ReadWriteCloser {
		limit := rtpBufferLimit
		if packetType == packetio.RTCPBufferPacket {
			limit = rtcpBufferLimit
		}
		buf := packetio.NewBuffer()
		buf.SetLimitSize(limit)
		return &gatedBuffer{peer: p, ssrc: ssrc, rtp: packetType == packetio.RTPBufferPacket, buf: buf}
	}
	if settingEngineHook != nil {
		settingEngineHook(&se)
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(registry), webrtc.WithSettingEngine(se))
	return c.start(api.NewPeerConnection(webrtc.Configuration{}))
}

// gatedBuffer is an SRTP/SRTCP read buffer that drops what it is given while
// its SSRC is not allowed, and fails reads at once (so that Pion's probe of
// an unknown stream ends). Once allowed, it is a normal buffer. Every packet
// is counted, and RTP packets recorded for transport feedback, before the
// gate.
type gatedBuffer struct {
	peer *Peer
	ssrc uint32
	rtp  bool // RTP (not RTCP) buffer
	buf  *packetio.Buffer
}

func (b *gatedBuffer) Write(pkt []byte) (int, error) {
	b.peer.receivedBytes.Add(uint64(len(pkt)))
	if b.rtp {
		b.peer.feedback.recordFromBuffer(pkt, time.Now()) // Every arriving packet, kept or not, when the sockets are not seen
	}
	if !b.peer.allowedSSRC(b.ssrc) {
		b.peer.droppedPackets.Add(1)
		return len(pkt), nil
	}
	return b.buf.Write(pkt)
}

func (b *gatedBuffer) Read(pkt []byte) (int, error) {
	if !b.peer.allowedSSRC(b.ssrc) {
		return 0, errNotOffered
	}
	return b.buf.Read(pkt)
}

func (b *gatedBuffer) Close() error {
	return b.buf.Close()
}

// slogLoggerFactory routes Pion's logs to a structured logger: errors as
// warnings, warnings as debug, the rest dropped. The error Pion logs for
// every stream it was not offered is expected (receive cap) and goes to
// debug.
type slogLoggerFactory struct {
	logger *slog.Logger
}

func (f slogLoggerFactory) NewLogger(scope string) logging.LeveledLogger {
	return slogLogger{f.logger.With(slog.String("pion", scope))}
}

type slogLogger struct {
	logger *slog.Logger
}

func (l slogLogger) Trace(string)                  {}
func (l slogLogger) Tracef(string, ...interface{}) {}
func (l slogLogger) Debug(string)                  {}
func (l slogLogger) Debugf(string, ...interface{}) {}
func (l slogLogger) Info(string)                   {}
func (l slogLogger) Infof(string, ...interface{})  {}
func (l slogLogger) Warn(msg string)               { l.logger.Debug(msg) }
func (l slogLogger) Warnf(format string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}
func (l slogLogger) Error(msg string) { l.error(msg) }
func (l slogLogger) Errorf(format string, args ...interface{}) {
	l.error(fmt.Sprintf(format, args...))
}

func (l slogLogger) error(msg string) {
	if strings.HasPrefix(msg, "Incoming unhandled RTP ssrc") {
		l.logger.Debug(msg)
		return
	}
	l.logger.Warn(msg)
}
