package execution

import (
	"context"
	"fmt" // Add fmt import
	"log/slog"
	"sync"
	"time"

	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/scenario"
)

// Engine manages the lifecycle of a scenario execution.
type Engine struct {
	scenarioID string
	testPlan   scenario.TestPlan
	adapter    platform.PlatformAdapter
	logger     *slog.Logger

	peers      map[string]*Peer
	peersMutex sync.RWMutex

	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup // For waiting on all peers to finish
	
	onComplete func(scenarioID string, status string)
}

// NewEngine creates a new execution engine.
func NewEngine(scenarioID string, testPlan scenario.TestPlan, adapter platform.PlatformAdapter, logger *slog.Logger, onComplete func(scenarioID string, status string)) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Engine{
		scenarioID: scenarioID,
		testPlan:   testPlan,
		adapter:    adapter,
		logger:     logger,
		peers:      make(map[string]*Peer),
		ctx:        ctx,
		cancel:     cancel,
		onComplete: onComplete,
	}
}

// Start begins the scenario execution.
func (e *Engine) Start() {
	e.logger.Info("Starting scenario execution", slog.String("scenario_id", e.scenarioID))
	
	finalStatus := "completed" // Default status

	defer func() {
		e.logger.Info("Scenario execution finished", slog.String("scenario_id", e.scenarioID), slog.String("status", finalStatus))
		e.onComplete(e.scenarioID, finalStatus)
	}()

	// Ramp-up phase
	
	rampUpInterval := time.Duration(0)
	if e.testPlan.Participants > 1 && e.testPlan.RampUpSeconds > 0 {
		rampUpInterval = time.Duration(e.testPlan.RampUpSeconds*1000/e.testPlan.Participants) * time.Millisecond
	} else if e.testPlan.Participants > 0 {
		rampUpInterval = time.Duration(0) // Connect immediately if 1 participant or no ramp up
	}

	e.logger.Info("Ramp-up phase starting",
		slog.Int("participants", e.testPlan.Participants),
		slog.Int("ramp_up_seconds", e.testPlan.RampUpSeconds),
		slog.Duration("ramp_up_interval", rampUpInterval),
	)

	for i := 0; i < e.testPlan.Participants; i++ {
		select {
		case <-e.ctx.Done():
			e.logger.Info("Engine context cancelled during ramp-up", slog.String("scenario_id", e.scenarioID))
			finalStatus = "failed"
			return
		default:
			peerID := fmt.Sprintf("peer-%s-%d", e.scenarioID, i)
			// Pass logger to NewPeer
			peer, err := NewPeer(peerID, e.logger)
			if err != nil {
				e.logger.Error("Failed to create peer", slog.String("peer_id", peerID), slog.String("error", err.Error()))
				continue
			}

			e.peersMutex.Lock()
			e.peers[peerID] = peer
			e.peersMutex.Unlock()

			e.wg.Add(1) // Increment for this peer
			go func(p *Peer) {
				defer e.wg.Done()
				e.logger.Info("Connecting peer", slog.String("peer_id", p.ID()))
				// Use p.ID() instead of p itself
				if err := e.adapter.Connect(p.ID()); err != nil {
					e.logger.Error("Failed to connect peer", slog.String("peer_id", p.ID()), slog.String("error", err.Error()))
					p.state = PeerStateFailed // Set peer state to failed
				} else {
					p.state = PeerStateConnected // Set peer state to connected
				}
			}(peer)

			if rampUpInterval > 0 && i < e.testPlan.Participants-1 {
				time.Sleep(rampUpInterval)
			}
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
		e.logger.Info("Engine context cancelled during test duration", slog.String("scenario_id", e.scenarioID))
		finalStatus = "failed"
		return
	}

	// Wait for all peers to disconnect (graceful shutdown handled by Stop)
	e.Stop()
}

// Stop gracefully stops the scenario execution and disconnects all peers.
func (e *Engine) Stop() {
	e.logger.Info("Stopping scenario execution", slog.String("scenario_id", e.scenarioID))
	e.cancel() // Signal all goroutines to stop

	// Disconnect all peers
	e.peersMutex.RLock()
	for _, peer := range e.peers {
		// Use peer.ID() instead of peer itself
		e.logger.Info("Simulating peer disconnect", slog.String("peer_id", peer.ID()))
		e.adapter.Disconnect(peer.ID()) // Call Disconnect with peer ID
		peer.state = PeerStateDisconnected
	}
	e.peersMutex.RUnlock()

	e.wg.Wait() // Wait for all peer goroutines to finish
	e.logger.Info("All peers disconnected, engine stopped", slog.String("scenario_id", e.scenarioID))
}




