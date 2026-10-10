package execution

import (
	"log/slog"
	"time"

	"webrtc-load-engine/src/media"
)

// mediaCollectInterval is how often a scenario's media metrics are updated.
const mediaCollectInterval = 2 * time.Second

// mediaPeer is what the collector reads from a peer.
type mediaPeer interface {
	ID() string
	State() PeerState
	MediaSnapshot(now time.Time) map[string]KindMediaSnapshot
	TransportBytesReceived() uint64
}

// mediaCollector turns the established peers' cumulative media snapshots into
// the scenario's media metrics, and detects stalls: a kind received before,
// then nothing for stallTimeout while the peer is established and still
// offered remote sources of that kind. Stalls are counted and logged, never
// a lifecycle transition (FR-014).
type mediaCollector struct {
	scenarioID   string
	stallTimeout time.Duration
	logger       *slog.Logger

	prev      map[string]map[string]KindMediaSnapshot // Peer ID -> kind -> last snapshot
	stalled   map[string]map[string]bool              // Peer ID -> kind -> stall counted
	transport map[string]uint64                       // Peer ID -> transport bytes received at the last collection
}

func newMediaCollector(scenarioID string, stallTimeout time.Duration, logger *slog.Logger) *mediaCollector {
	return &mediaCollector{
		scenarioID:   scenarioID,
		stallTimeout: stallTimeout,
		logger:       logger,
		prev:         map[string]map[string]KindMediaSnapshot{},
		stalled:      map[string]map[string]bool{},
		transport:    map[string]uint64{},
	}
}

var mediaKinds = []string{media.KindAudio, media.KindVideo}

// collect updates the metrics from the peers at now.
func (c *mediaCollector) collect(now time.Time, peers []mediaPeer) {
	streams := map[string]int{}
	for _, p := range peers {
		if p.State() != PeerStateEstablished {
			continue
		}
		if n := p.TransportBytesReceived(); n > c.transport[p.ID()] {
			transportBytesReceived.WithLabelValues(c.scenarioID).Add(float64(n - c.transport[p.ID()]))
			c.transport[p.ID()] = n
		}
		snap := p.MediaSnapshot(now)
		if c.prev[p.ID()] == nil {
			c.prev[p.ID()] = map[string]KindMediaSnapshot{}
			c.stalled[p.ID()] = map[string]bool{}
		}
		for _, kind := range mediaKinds {
			k := snap[kind]
			prev := c.prev[p.ID()][kind]
			c.prev[p.ID()][kind] = k
			labels := []string{c.scenarioID, kind}
			if k.Packets > prev.Packets {
				media.PacketsReceived.WithLabelValues(labels...).Add(float64(k.Packets - prev.Packets))
			}
			if k.Bytes > prev.Bytes {
				media.BytesReceived.WithLabelValues(labels...).Add(float64(k.Bytes - prev.Bytes))
			}
			if k.Lost > prev.Lost {
				media.PacketsLost.WithLabelValues(labels...).Add(float64(k.Lost - prev.Lost))
			}
			for _, j := range k.Jitters {
				media.Jitter.WithLabelValues(labels...).Observe(j)
			}
			for _, rtt := range k.RoundTrips {
				media.RoundTrip.WithLabelValues(labels...).Observe(rtt.Seconds())
			}
			streams[kind] += k.Offered
			c.checkStall(p.ID(), kind, k, now)
		}
	}
	for _, kind := range mediaKinds {
		media.RemoteStreams.WithLabelValues(c.scenarioID, kind).Set(float64(streams[kind]))
	}
}

func (c *mediaCollector) checkStall(peerID, kind string, k KindMediaSnapshot, now time.Time) {
	silent := !k.LastReceived.IsZero() && now.Sub(k.LastReceived) > c.stallTimeout
	if !silent || k.Offered == 0 {
		c.stalled[peerID][kind] = false
		return
	}
	if c.stalled[peerID][kind] {
		return // Already counted until media resumes
	}
	c.stalled[peerID][kind] = true
	media.Stalls.WithLabelValues(c.scenarioID, kind).Inc()
	c.logger.Warn("Incoming media stalled",
		slog.String("scenario_id", c.scenarioID), slog.String("peer_id", peerID),
		slog.String("kind", kind), slog.Duration("silent_for", now.Sub(k.LastReceived).Round(time.Second)))
}

// finish resets the scenario's gauges once it has ended.
func (c *mediaCollector) finish() {
	for _, kind := range mediaKinds {
		media.RemoteStreams.WithLabelValues(c.scenarioID, kind).Set(0)
	}
}
