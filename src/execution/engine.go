package execution

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"webrtc-load-engine/src/hostload"
	"webrtc-load-engine/src/media"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

// Engine manages the lifecycle of a scenario execution.
type Engine struct {
	scenarioID string
	testPlan   scenario.TestPlan
	adapter    platform.PlatformAdapter
	logger     *slog.Logger
	publish    bool // The adapter's platform receives the peers' media on a publish session (008)

	peers      map[string]*Peer
	peersMutex sync.RWMutex

	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup // Tracks in-flight peer connect goroutines
	stopOnce sync.Once

	onComplete func(scenarioID string, r scenario.RunResult)

	sources       map[string]*media.Source // By kind; one per sent kind, shared by all peers
	collectorDone chan struct{}            // Closed when the media collector has finished (or never started)
	quality       *scenario.QualitySummary // Built by the media collector before collectorDone is closed

	sampler     *hostload.Sampler // Server and engine load (007); its last sample is taken when ctx is cancelled
	samplerDone chan struct{}     // Closed when the sampler has finished
}

// NewEngine creates a new execution engine.
func NewEngine(scenarioID string, testPlan scenario.TestPlan, adapter platform.PlatformAdapter, logger *slog.Logger, onComplete func(scenarioID string, r scenario.RunResult)) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	pa, ok := adapter.(platform.PublishSessionAdapter)
	return &Engine{
		scenarioID: scenarioID,
		testPlan:   testPlan,
		adapter:    adapter,
		logger:     logger,
		publish:    ok && pa.PublishSession(),
		peers:      make(map[string]*Peer),
		ctx:        ctx,
		cancel:     cancel,
		onComplete: onComplete,

		collectorDone: make(chan struct{}),
		sampler:       hostload.NewSampler(scenarioID, testPlan.ServerMonitoring, logger),
		samplerDone:   make(chan struct{}),
	}
}

// Start begins the scenario execution.
func (e *Engine) Start() {
	e.logger.Info("Starting scenario execution", slog.String("scenario_id", e.scenarioID))

	finalStatus := "completed" // Default status
	result := scenario.RunResult{StartedAt: time.Now().UTC()}

	defer func() {
		e.logger.Info("Scenario execution finished", slog.String("scenario_id", e.scenarioID), slog.String("status", finalStatus))
		// The summary is built after the last collection, during Stop
		// (which another goroutine may be running).
		<-e.collectorDone
		<-e.samplerDone
		if e.quality != nil {
			e.quality.Server = e.sampler.Summary()
			e.quality.LoadMachine = e.sampler.LoadMachine()
		}
		result.Status, result.Quality = finalStatus, e.quality
		result.EndedAt = time.Now().UTC()
		e.onComplete(e.scenarioID, result)
	}()

	go func() {
		defer close(e.samplerDone)
		e.sampler.Run(e.ctx)
	}()

	if err := e.startMedia(); err != nil {
		e.logger.Error("Failed to start media", slog.String("scenario_id", e.scenarioID), slog.String("error", err.Error()))
		finalStatus = "failed"
		result.Error = err.Error()
		close(e.collectorDone)
		e.Stop()
		return
	}
	go e.runMediaCollector()

	// Ramp-up phase: peers are spread evenly over RampUpSeconds.
	rampUpInterval := time.Duration(0)
	if e.testPlan.Participants > 1 && e.testPlan.RampUpSeconds > 0 {
		rampUpInterval = time.Duration(e.testPlan.RampUpSeconds*1000/e.testPlan.Participants) * time.Millisecond
	}

	e.logger.Info("Ramp-up phase starting",
		slog.Int("participants", e.testPlan.Participants),
		slog.Int("ramp_up_seconds", e.testPlan.RampUpSeconds),
		slog.Duration("ramp_up_interval", rampUpInterval),
	)

	for i := 0; i < e.testPlan.Participants; i++ {
		if i > 0 && rampUpInterval > 0 {
			select {
			case <-time.After(rampUpInterval):
			case <-e.ctx.Done():
			}
		}
		if !e.launchPeer(i) {
			e.logger.Info("Engine stopped during ramp-up", slog.String("scenario_id", e.scenarioID))
			finalStatus = "stopped"
			return
		}
	}

	e.logger.Info("Ramp-up phase complete, waiting for duration",
		slog.Int("duration_seconds", e.testPlan.DurationSeconds),
		slog.String("scenario_id", e.scenarioID),
	)

	// Test duration phase
	select {
	case <-time.After(time.Duration(e.testPlan.DurationSeconds) * time.Second):
		e.logger.Info("Test duration completed", slog.String("scenario_id", e.scenarioID))
	case <-e.ctx.Done():
		e.logger.Info("Engine stopped during test duration", slog.String("scenario_id", e.scenarioID))
		finalStatus = "stopped"
		return
	}

	e.Stop()
}

// startMedia starts one media source per kind the bots send; they run until
// the engine stops. Peers subscribe once their transport is established.
func (e *Engine) startMedia() error {
	e.sources = map[string]*media.Source{}
	m := e.testPlan.Media
	file, err := e.loadSourceFile()
	if err != nil {
		return err
	}
	for _, kind := range m.Kinds() {
		var src *media.Source
		switch {
		case file != nil && kind == media.KindAudio:
			src = media.NewSource(e.scenarioID, file.Audio)
		case file != nil:
			src = media.NewLayeredSource(e.scenarioID, file.Video)
		case kind == media.KindVideo && m.Simulcast != nil:
			layers, err := media.LoadLayers(m.Simulcast.MaxHeight)
			if err != nil {
				return err
			}
			src = media.NewLayeredSource(e.scenarioID, layers)
		default:
			content, err := media.LoadContent(kind)
			if err != nil {
				return err
			}
			src = media.NewSource(e.scenarioID, content)
		}
		e.sources[kind] = src
		go src.Run(e.ctx)
	}
	if len(e.sources) > 0 {
		e.logger.Info("Media sources started", slog.String("scenario_id", e.scenarioID), slog.Any("kinds", e.testPlan.Media.Kinds()))
	}
	return nil
}

// loadSourceFile transcodes media.source_file for the kinds the bots send,
// with the simulcast layers when simulcast is on; nil without a file.
func (e *Engine) loadSourceFile() (*media.FileContent, error) {
	m := e.testPlan.Media
	if m == nil || m.SourceFile == "" {
		return nil, nil
	}
	var heights []int
	if slices.Contains(m.Kinds(), media.KindVideo) {
		heights = []int{360}
		if m.Simulcast != nil {
			var err error
			if heights, err = media.LayerHeights(m.Simulcast.MaxHeight); err != nil {
				return nil, err
			}
		}
	}
	start := time.Now()
	file, err := media.LoadFile(e.ctx, m.SourceFile, true, heights)
	if err != nil {
		return nil, err
	}
	e.logger.Info("Media file loaded",
		slog.String("scenario_id", e.scenarioID),
		slog.String("file", m.SourceFile),
		slog.Duration("audio_loop", file.Audio.LoopDuration),
		slog.Any("video_heights", heights),
		slog.Duration("took", time.Since(start)))
	return file, nil
}

// runMediaCollector updates the scenario's media metrics every
// mediaCollectInterval until the engine stops, then once more.
func (e *Engine) runMediaCollector() {
	defer close(e.collectorDone)
	stall := 10 * time.Second
	if m := e.testPlan.Media; m != nil && m.StallTimeoutSeconds > 0 {
		stall = time.Duration(m.StallTimeoutSeconds) * time.Second
	}
	c := newMediaCollector(e.scenarioID, stall, e.logger)
	defer c.finish()

	ticker := time.NewTicker(mediaCollectInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			c.collect(now, e.mediaPeers())
		case <-e.ctx.Done():
			now := time.Now()
			c.collect(now, e.mediaPeers())
			e.quality = c.summary(now)
			return
		}
	}
}

// mediaPeers returns the scenario's peers for the media collector.
func (e *Engine) mediaPeers() []mediaPeer {
	e.peersMutex.RLock()
	defer e.peersMutex.RUnlock()
	out := make([]mediaPeer, 0, len(e.peers))
	for _, p := range e.peers {
		out = append(out, p)
	}
	return out
}

// onPeerState subscribes a peer's tracks to the media sources when its
// transport is established, and unsubscribes them when it ends.
func (e *Engine) onPeerState(p *Peer, to PeerState) {
	if layers := p.LocalLayerTracks(); len(layers) > 0 && e.sources[media.KindVideo] != nil {
		src := e.sources[media.KindVideo]
		switch to {
		case PeerStateEstablished:
			writers := make([]media.TrackWriter, len(layers))
			for i, l := range layers {
				writers[i] = l
			}
			src.SubscribeLayers(writers)
		case PeerStateFailed, PeerStateDisconnected:
			src.Unsubscribe(layers[0])
		}
	}
	for kind, track := range p.LocalTracks() {
		src := e.sources[kind]
		if src == nil {
			continue
		}
		switch to {
		case PeerStateEstablished:
			src.Subscribe(track)
		case PeerStateFailed, PeerStateDisconnected:
			src.Unsubscribe(track)
		}
	}
}

// peerMedia returns what peer number i of the scenario sends and receives.
func (e *Engine) peerMedia(i int) PeerMedia {
	m := PeerMedia{Kinds: e.testPlan.Media.Kinds(), PublishSession: e.publish}
	if mc := e.testPlan.Media; mc != nil {
		m.ReceiveLimit = mc.MaxReceivedStreams
		m.StallTimeout = time.Duration(mc.StallTimeoutSeconds) * time.Second
		if mc.Simulcast != nil && slices.Contains(m.Kinds, media.KindVideo) {
			m.Simulcast = true
			m.LayerHeights, _ = media.LayerHeights(mc.Simulcast.MaxHeight) // Validated by the scenario
		}
		m.RequestedHeight = mc.HeightFor(i, e.testPlan.Participants)
	}
	for kind, src := range e.sources {
		if m.SendTimes == nil {
			m.SendTimes = map[string]*media.SendTimes{}
		}
		m.SendTimes[kind] = src.SendTimes()
	}
	return m
}

// launchPeer creates peer number i and starts connecting it. It reports false
// if the engine is stopping.
func (e *Engine) launchPeer(i int) bool {
	peerID := fmt.Sprintf("peer-%s-%d", e.scenarioID, i)

	// Registering the peer and wg.Add under peersMutex, after checking ctx,
	// guarantees that Stop (which cancels ctx and then takes the lock) never
	// races a new peer against wg.Wait.
	e.peersMutex.Lock()
	defer e.peersMutex.Unlock()
	if e.ctx.Err() != nil {
		return false
	}

	peer, err := NewPeer(peerID, e.scenarioID, e.peerMedia(i), e.logger)
	if err != nil {
		e.logger.Error("Failed to create peer", slog.String("peer_id", peerID), slog.String("error", err.Error()))
		return true
	}
	peer.OnStateChange(func(to PeerState) { e.onPeerState(peer, to) })
	e.peers[peerID] = peer
	e.wg.Add(1)
	go e.connectPeer(peer)
	return true
}

func (e *Engine) connectPeer(p *Peer) {
	defer e.wg.Done()

	e.logger.Info("Connecting peer", slog.String("peer_id", p.ID()))
	e.mustTransition(p, PeerStateConnecting)

	start := time.Now()
	if err := e.adapter.Connect(e.ctx, p.ID(), p); err != nil {
		if e.ctx.Err() != nil {
			e.logger.Info("Peer connect cancelled", slog.String("peer_id", p.ID()))
			e.mustTransition(p, PeerStateFailed)
			return
		}
		p.fail(stageSignaling, err)
		return
	}
	signalingConnectSeconds.WithLabelValues(e.scenarioID).Observe(time.Since(start).Seconds())

	// ICE may already have moved the peer to Established (or Failed).
	if _, err := p.transitionIf(PeerStateConnecting, PeerStateConnected); err != nil {
		e.logger.Error("Peer lifecycle error", slog.String("peer_id", p.ID()), slog.String("error", err.Error()))
	}
}

// mustTransition applies a lifecycle transition, surfacing illegal ones as errors.
func (e *Engine) mustTransition(p *Peer, to PeerState) {
	if err := p.transition(to); err != nil {
		e.logger.Error("Peer lifecycle error", slog.String("peer_id", p.ID()), slog.String("error", err.Error()))
	}
}

// Stop gracefully stops the scenario execution and disconnects all peers.
// It is idempotent and safe to call concurrently.
func (e *Engine) Stop() {
	e.stopOnce.Do(e.stop)
}

func (e *Engine) stop() {
	e.logger.Info("Stopping scenario execution", slog.String("scenario_id", e.scenarioID))
	e.cancel() // Abort ramp-up and in-flight connects

	// Barrier: after this no new peer can be launched (see launchPeer).
	e.peersMutex.Lock()
	e.peersMutex.Unlock()

	e.wg.Wait()       // In-flight connects return promptly once ctx is cancelled
	<-e.collectorDone // Last media collection while the peers are still up

	e.peersMutex.RLock()
	var disconnects sync.WaitGroup
	for _, peer := range e.peers {
		disconnects.Add(1)
		go func(p *Peer) {
			defer disconnects.Done()
			if err := e.adapter.Disconnect(p.ID()); err != nil {
				e.logger.Warn("Failed to disconnect peer", slog.String("peer_id", p.ID()), slog.String("error", err.Error()))
			}
			if err := p.Close(); err != nil {
				e.logger.Warn("Failed to close peer connection", slog.String("peer_id", p.ID()), slog.String("error", err.Error()))
			}
			e.mustTransition(p, PeerStateDisconnected)
		}(peer)
	}
	disconnects.Wait()
	e.peersMutex.RUnlock()
	e.logger.Info("All peers disconnected, engine stopped", slog.String("scenario_id", e.scenarioID))
}
