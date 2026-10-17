package hostload

import (
	"slices"
	"sync"
	"time"
)

// overlapRegistry records which scenarios of this engine sample which
// container, so that a summary can say when another run loaded the same
// container at the same time (FR-012, research R9).
type overlapRegistry struct {
	mu      sync.Mutex
	watches map[string][]*watchRecord // Container -> watches, oldest first
}

type watchRecord struct {
	scenarioID string
	start, end time.Time // end is zero while the watch runs
}

var registry = newOverlapRegistry()

func newOverlapRegistry() *overlapRegistry {
	return &overlapRegistry{watches: map[string][]*watchRecord{}}
}

// watch records that scenarioID samples containers from now on. stop ends
// the watch and returns, per container, the other scenarios that sampled it
// at some point during the watch (none: absent).
func (r *overlapRegistry) watch(scenarioID string, containers []string) (stop func() map[string][]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	mine := map[string]*watchRecord{}
	for _, c := range containers {
		r.prune(c)
		w := &watchRecord{scenarioID: scenarioID, start: now}
		r.watches[c] = append(r.watches[c], w)
		mine[c] = w
	}
	return func() map[string][]string {
		r.mu.Lock()
		defer r.mu.Unlock()
		end := time.Now()
		shared := map[string][]string{}
		for c, w := range mine {
			w.end = end
			for _, o := range r.watches[c] {
				// o overlaps [w.start, end] and is another scenario.
				if o == w || o.scenarioID == scenarioID || o.start.After(end) || (!o.end.IsZero() && o.end.Before(w.start)) {
					continue
				}
				if !slices.Contains(shared[c], o.scenarioID) {
					shared[c] = append(shared[c], o.scenarioID)
				}
			}
		}
		return shared
	}
}

// prune drops ended watches of a container that no running watch overlaps:
// no stop can report them any more. Called with mu held.
func (r *overlapRegistry) prune(container string) {
	ws := r.watches[container]
	oldestRunning := time.Time{}
	for _, w := range ws {
		if w.end.IsZero() && (oldestRunning.IsZero() || w.start.Before(oldestRunning)) {
			oldestRunning = w.start
		}
	}
	r.watches[container] = slices.DeleteFunc(ws, func(w *watchRecord) bool {
		return !w.end.IsZero() && (oldestRunning.IsZero() || w.end.Before(oldestRunning))
	})
}
