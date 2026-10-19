package hostload

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"webrtc-load-engine/src/scenario"
)

// Sampler samples the named containers of a scenario at a regular interval
// for the duration of a run, and builds the server section of its summary
// (research R5). It never fails the run: a container that cannot be read is
// reported with the reason.
type Sampler struct {
	scenarioID string
	containers []string // Test plan order; nil: none requested
	interval   time.Duration
	logger     *slog.Logger

	docker   *dockerClient
	process  func() (cpuNS uint64, rssBytes int64, err error) // The engine's own figures
	registry *overlapRegistry
	now      func() time.Time

	busy       atomic.Bool    // A tick is running
	procLogged atomic.Bool    // Engine sampling failure logged
	inflight   sync.WaitGroup // Ticks run by Run

	mu         sync.Mutex
	acc        []accumulator // Per container
	proc       accumulator   // The engine's process
	logged     []bool        // Unavailability logged for the container
	total      totalFigures
	startAt    time.Time // First tick
	end        time.Time // Last tick
	stopWatch  func() map[string][]string
	sharedWith map[string][]string
}

// totalFigures are the per-tick sums over containers (peak of the sum, not
// sum of peaks).
type totalFigures struct {
	peakCPU float64
	peakMem int64
	samples int // Ticks with at least one container sampled
}

// NewSampler returns a sampler for the containers of cfg (nil: none, the
// server section says not_requested). DOCKER_HOST selects the socket.
func NewSampler(scenarioID string, cfg *scenario.ServerMonitoringConfig, logger *slog.Logger) *Sampler {
	if logger == nil {
		logger = slog.Default()
	}
	interval := scenario.DefaultServerIntervalSeconds
	var containers []string
	if cfg != nil {
		containers = cfg.Containers
		if cfg.IntervalSeconds > 0 {
			interval = cfg.IntervalSeconds
		}
	}
	d := time.Duration(interval) * time.Second
	return &Sampler{
		scenarioID: scenarioID,
		containers: containers,
		interval:   d,
		logger:     logger,
		docker:     newDockerClient(os.Getenv("DOCKER_HOST"), min(d/2, 2*time.Second)),
		process:    processSample,
		registry:   registry,
		now:        time.Now,
		acc:        make([]accumulator, len(containers)),
		logged:     make([]bool, len(containers)),
	}
}

// Run samples at once, then every interval until ctx is cancelled, then a
// last time. It returns once the last sample is folded in.
func (s *Sampler) Run(ctx context.Context) {
	s.start()
	s.tick(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.inflight.Add(1)
			go func() {
				defer s.inflight.Done()
				s.tick(ctx)
			}()
		case <-ctx.Done():
			s.inflight.Wait()
			// The run's context is gone: the end sample has its own timeout.
			s.tick(context.Background())
			s.finish()
			return
		}
	}
}

// start registers the watch of the containers (FR-012).
func (s *Sampler) start() {
	if len(s.containers) > 0 {
		s.stopWatch = s.registry.watch(s.scenarioID, s.containers)
	}
}

// finish ends the watch and deletes the scenario's series.
func (s *Sampler) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopWatch != nil {
		s.sharedWith = s.stopWatch()
		s.stopWatch = nil
	}
	deleteSeries(s.scenarioID, s.containers)
}

// tick samples every container at once. A tick still running when the next
// is due makes the next one a missed interval.
func (s *Sampler) tick(ctx context.Context) {
	if !s.busy.CompareAndSwap(false, true) {
		s.mu.Lock()
		for i := range s.acc {
			s.acc[i].miss()
		}
		s.proc.miss()
		s.mu.Unlock()
		return
	}
	defer s.busy.Store(false)

	at := s.now()
	results := make([]sample, len(s.containers))
	var wg sync.WaitGroup
	for i, name := range s.containers {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			cpu, mem, reason := s.docker.stats(ctx, name)
			results[i] = sample{at: s.now(), cpuNS: cpu, memBytes: mem, err: reason}
		}(i, name)
	}
	// The engine is sampled while the containers' answers are awaited, so
	// both cover the same instants.
	proc := s.sampleProcess()
	wg.Wait()
	if ctx.Err() != nil {
		// The run ended while this tick was in flight: its requests were
		// cancelled, not failed. The end-of-run tick covers the end.
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.startAt.IsZero() {
		s.startAt = at
	}
	s.end = at
	if threads, ok := s.proc.add(proc); ok {
		EngineCPUThreads.WithLabelValues(s.scenarioID).Set(threads)
	}
	if proc.err == "" {
		EngineMemoryBytes.WithLabelValues(s.scenarioID).Set(float64(proc.memBytes))
	}
	var sumCPU float64
	var sumMem int64
	sampled := 0
	for i, r := range results {
		name := s.containers[i]
		threads, ok := s.acc[i].add(r)
		if r.err != "" {
			if !s.logged[i] {
				s.logged[i] = true
				s.logger.Warn("Server container not measured",
					slog.String("scenario_id", s.scenarioID), slog.String("container", name), slog.String("reason", r.err))
			}
			continue
		}
		sampled++
		sumMem += r.memBytes
		ServerMemoryBytes.WithLabelValues(s.scenarioID, name).Set(float64(r.memBytes))
		if ok {
			sumCPU += threads
			ServerCPUThreads.WithLabelValues(s.scenarioID, name).Set(threads)
		}
	}
	if sampled > 0 {
		s.total.samples++
		s.total.peakCPU = max(s.total.peakCPU, sumCPU)
		s.total.peakMem = max(s.total.peakMem, sumMem)
	}
}

// sampleProcess reads the engine's own figures.
func (s *Sampler) sampleProcess() sample {
	cpu, rss, err := s.process()
	if err != nil {
		if !s.procLogged.Swap(true) {
			s.logger.Warn("Engine load not measured", slog.String("scenario_id", s.scenarioID), slog.String("error", err.Error()))
		}
		return sample{err: scenario.ReasonNoSamples}
	}
	return sample{at: s.now(), cpuNS: cpu, memBytes: rss}
}
