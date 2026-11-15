package execution

import (
	"log/slog"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/cc"
	"github.com/pion/interceptor/pkg/gcc"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"

	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/scenario"
)

// A sender that adapts to its upload (specs/016-network-conditions US4,
// research R7): from what its estimator says of the path to the server, it
// drops its top layer after a second of congestion, and tries it again
// later, waiting longer after each failed try. Only bots whose group has an
// upload condition adapt.

const (
	senderTick         = 100 * time.Millisecond
	senderHold         = time.Second     // Congestion this long drops a layer
	senderTryHold      = 3 * time.Second // A restored layer kept this long is a success
	senderBackoffStart = 2 * time.Second
	senderBackoffMax   = 8 * time.Second
	senderSample       = time.Second
	// senderWarmUp: no layer is dropped this long after the first feedback.
	// Pion's estimator reports congestion in its first second (T039: both
	// constrained senders dropped a layer 1 s after starting, on a clean
	// path, and took it back 2 s later).
	senderWarmUp = 5 * time.Second
)

// senderAdapter is one sending bot's layer policy.
type senderAdapter struct {
	mu      sync.Mutex
	heights []int // Layer heights, low to high
	signal  func() (estimate int, congested, feedback bool)
	apply   func(layers int) // Send the lowest layers only
	start   time.Time        // When the bot started sending

	sent       int // Layers sent
	congested  time.Time
	nextTry    time.Time
	triedAt    time.Time // When the last restored layer came back; zero: no try under way
	backoff    time.Duration
	changes    []layerChange
	estimates  []estimateAt
	lastSample time.Time
	firstFeed  time.Time // First tick with feedback
}

type layerChange struct {
	at     time.Time
	layers int
}

type estimateAt struct {
	at  time.Time
	bps int
}

func newSenderAdapter(heights []int, signal func() (int, bool, bool), apply func(int), start time.Time) *senderAdapter {
	a := &senderAdapter{heights: heights, signal: signal, apply: apply, start: start, sent: len(heights), backoff: senderBackoffStart}
	a.changes = []layerChange{{at: start, layers: len(heights)}}
	return a
}

// tick runs the policy at now.
func (a *senderAdapter) tick(now time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()
	est, congested, feedback := a.signal()
	if feedback && a.firstFeed.IsZero() {
		a.firstFeed = now
	}
	if a.firstFeed.IsZero() || now.Sub(a.firstFeed) < senderWarmUp {
		congested = false // Not judged before the estimator settles
	}
	if a.lastSample.IsZero() || now.Sub(a.lastSample) >= senderSample {
		a.estimates = append(a.estimates, estimateAt{at: now, bps: est})
		a.lastSample = now
	}
	if len(a.heights) < 2 {
		return // A single stream cannot adapt
	}
	if !congested {
		a.congested = time.Time{}
	} else if a.congested.IsZero() {
		a.congested = now
	}
	switch {
	case !a.congested.IsZero() && now.Sub(a.congested) >= senderHold && a.sent > 1:
		a.set(a.sent-1, now)
		a.congested = time.Time{}
		if !a.triedAt.IsZero() && now.Sub(a.triedAt) < senderTryHold {
			a.backoff = min(2*a.backoff, senderBackoffMax) // The try failed
		}
		a.triedAt = time.Time{}
		a.nextTry = now.Add(a.backoff)
	case !a.triedAt.IsZero() && now.Sub(a.triedAt) >= senderTryHold:
		a.backoff = senderBackoffStart // The try held
		a.triedAt = time.Time{}
		if a.sent < len(a.heights) {
			a.nextTry = now.Add(a.backoff)
		}
	case a.sent < len(a.heights) && a.triedAt.IsZero() && !a.nextTry.IsZero() && !now.Before(a.nextTry) && a.congested.IsZero():
		a.set(a.sent+1, now)
		a.triedAt = now
	}
}

func (a *senderAdapter) set(layers int, now time.Time) {
	a.sent = layers
	a.changes = append(a.changes, layerChange{at: now, layers: layers})
	a.apply(layers)
}

// run ticks the policy until stop is closed.
func (a *senderAdapter) run(stop <-chan struct{}) {
	t := time.NewTicker(senderTick)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case now := <-t.C:
			a.tick(now)
		}
	}
}

// result is the sender's part of the report, with times from runStart.
func (a *senderAdapter) result(runStart time.Time) *scenario.SenderResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	at := func(t time.Time) float64 { return math.Round(t.Sub(runStart).Seconds()*10) / 10 }
	r := &scenario.SenderResult{Adapts: len(a.heights) > 1}
	if !r.Adapts {
		r.Reason = scenario.CannotAdapt
	}
	for _, c := range a.changes {
		r.Layers = append(r.Layers, scenario.LayerChange{AtSeconds: at(c.at), Heights: append([]int(nil), a.heights[:c.layers]...)})
	}
	for _, e := range a.estimates {
		r.EstimateKbps = append(r.EstimateKbps, scenario.EstimateSample{AtSeconds: at(e.at), Kbps: e.bps / 1000})
	}
	return r
}

// peerSender is a constrained sender's estimator and layer policy.
type peerSender struct {
	mu        sync.Mutex
	estimator cc.BandwidthEstimator
	adapter   *senderAdapter
	stop      chan struct{}
	feedbacks atomic.Int64 // Transport-wide feedback packets read about the bot's upload
}

// signal is the estimator's target and whether it sees congestion: its
// delay detector's overuse, or more than 10% loss.
func (s *peerSender) signal() (int, bool, bool) {
	s.mu.Lock()
	e := s.estimator
	s.mu.Unlock()
	if e == nil {
		return 0, false, false
	}
	stats := e.GetStats()
	usage, _ := stats["usage"].(string)
	loss, _ := stats["averageLoss"].(float64)
	return e.GetTargetBitrate(), usage == "overuse" || loss > 0.1, s.feedbacks.Load() > 0
}

// registerSenderEstimation adds, on the session that sends, the
// transport-wide sequence numbers, the transport-cc feedback a browser
// offers (LiveKit sends none without it, T004), Pion's send-side estimator
// and a counter of the feedback received.
func registerSenderEstimation(s *peerSender, m *webrtc.MediaEngine, registry *interceptor.Registry, initialBps int) error {
	f, err := cc.NewInterceptor(func() (cc.BandwidthEstimator, error) {
		return gcc.NewSendSideBWE(gcc.SendSideBWEInitialBitrate(initialBps), gcc.SendSideBWEPacer(gcc.NewNoOpPacer()))
	})
	if err != nil {
		return err
	}
	f.OnNewPeerConnection(func(_ string, e cc.BandwidthEstimator) {
		s.mu.Lock()
		s.estimator = e
		s.mu.Unlock()
	})
	registry.Add(f)
	registry.Add(feedbackCounterFactory{&s.feedbacks})
	for _, kind := range []webrtc.RTPCodecType{webrtc.RTPCodecTypeVideo, webrtc.RTPCodecTypeAudio} {
		m.RegisterFeedback(webrtc.RTCPFeedback{Type: webrtc.TypeRTCPFBTransportCC}, kind)
	}
	return webrtc.ConfigureTWCCHeaderExtensionSender(m, registry)
}

// feedbackCounterFactory counts the transport-wide feedback a session reads.
type feedbackCounterFactory struct{ n *atomic.Int64 }

func (f feedbackCounterFactory) NewInterceptor(string) (interceptor.Interceptor, error) {
	return &feedbackCounter{n: f.n}, nil
}

type feedbackCounter struct {
	interceptor.NoOp
	n *atomic.Int64
}

func (c *feedbackCounter) BindRTCPReader(r interceptor.RTCPReader) interceptor.RTCPReader {
	return interceptor.RTCPReaderFunc(func(b []byte, a interceptor.Attributes) (int, interceptor.Attributes, error) {
		n, attr, err := r.Read(b, a)
		if err == nil {
			if pkts, perr := rtcp.Unmarshal(b[:n]); perr == nil {
				for _, p := range pkts {
					if _, ok := p.(*rtcp.TransportLayerCC); ok {
						c.n.Add(1)
					}
				}
			}
		}
		return n, attr, err
	})
}

// startSending starts the layer policy on the peer's video tracks.
func (p *Peer) startSending() {
	s := p.sender
	var heights []int
	var apply func(int)
	switch {
	case len(p.layerTracks) > 1: // Simulcast: one track per layer, low to high
		heights = p.layerHeights
		for _, t := range p.layerTracks {
			if vt, ok := t.(*videoTrack); ok {
				vt.enableLayerControl()
			}
		}
		apply = func(n int) {
			for i, t := range p.layerTracks {
				if vt, ok := t.(*videoTrack); ok {
					vt.setLayerEnabled(i < n)
				}
			}
		}
	case p.svc:
		heights = p.layerHeights
		vt, _ := p.localTracks[media.KindVideo].(*videoTrack)
		if vt != nil {
			vt.enableLayerControl()
		}
		apply = func(n int) {
			if vt != nil {
				vt.setMaxSpatial(n - 1)
			}
		}
	default: // A single stream, or no video: nothing to drop
		heights = []int{p.videoHeight}
		apply = func(int) {}
	}
	s.adapter = newSenderAdapter(heights, s.signal, func(n int) {
		apply(n)
		p.logger.Info("Sending fewer or more video layers", slog.Int("layers", n), slog.Any("heights", heights[:n]))
	}, time.Now())
	s.stop = make(chan struct{})
	go s.adapter.run(s.stop)
}

// senderResult is the sender's part of the report, with times from
// runStart; nil for a peer that does not adapt.
func (p *Peer) senderResult(runStart time.Time) *scenario.SenderResult {
	s := p.sender
	if s == nil || s.adapter == nil {
		return nil
	}
	r := s.adapter.result(runStart)
	if r.Adapts && s.feedbacks.Load() == 0 {
		r.Adapts, r.Reason = false, scenario.NoFeedback
	}
	return r
}

// stopSending stops the layer policy.
func (p *Peer) stopSending() {
	if s := p.sender; s != nil && s.stop != nil {
		select {
		case <-s.stop:
		default:
			close(s.stop)
		}
	}
}

// sendingBitrate is what the peer sends at the start, bit/s: its video
// layers (014 content bitrates) and its audio.
func (p *Peer) sendingBitrate() int {
	kbps := 64 // Audio, with headers
	arr := media.ArrangementSingle
	maxHeight := 0
	switch {
	case p.simulcast:
		arr = media.ArrangementSimulcast
	case p.svc:
		arr = media.ArrangementSVC
	}
	if len(p.layerHeights) > 0 {
		maxHeight = p.layerHeights[len(p.layerHeights)-1]
	}
	rates := media.ContentBitrates(p.videoCodec, arr, maxHeight)
	switch arr {
	case media.ArrangementSVC: // Each height's rate includes the layers below
		kbps += rates[maxHeight]
	default:
		for _, r := range rates {
			kbps += r
		}
	}
	return kbps * 1000
}
