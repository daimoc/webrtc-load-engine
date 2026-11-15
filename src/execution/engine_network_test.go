package execution

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"webrtc-load-engine/src/scenario"
)

// A plan with a network section gives the run a network result: the
// groups, the bots of each, their requested heights; the mock adapter
// connects no transport, so nothing was offered and every measure says
// not_applied (016 T015).
func TestEngine_NetworkResult(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	seed := uint64(9)
	plan := scenario.TestPlan{Participants: 4, DurationSeconds: 1,
		Network: &scenario.NetworkConfig{Seed: &seed, Groups: []scenario.BotGroup{{Name: "slow", Count: 2,
			Schedule: []scenario.ScheduleStep{{Downlink: &scenario.LinkCondition{RateKbps: 300, QueueMs: 200}}}}}}}
	r := runEngine(t, plan, adapter, 0)
	n := r.Network
	if n == nil {
		t.Fatal("no network result")
	}
	if n.Seed != 9 || n.SeedSource != scenario.SeedFromTestPlan {
		t.Errorf("seed %d from %s", n.Seed, n.SeedSource)
	}
	if len(n.Groups) != 1 || len(n.Groups[0].Bots) != 2 || n.Groups[0].Quality == nil || n.UnconstrainedQuality == nil {
		t.Fatalf("groups %+v, unconstrained %v", n.Groups, n.UnconstrainedQuality)
	}
	if st := n.Groups[0].Steps[0]; st.StartedAtSeconds == nil || st.EndedAtSeconds == nil {
		t.Errorf("step times %+v", st)
	}
	if len(n.Bots) != 2 || n.Bots[0].Bot != n.Groups[0].Bots[0] || n.Bots[0].RequestedHeight != 360 {
		t.Fatalf("bots %+v", n.Bots)
	}
	if d := n.Bots[0].Steps[0].Downlink; d == nil || d.Reason != scenario.NotApplied {
		t.Errorf("downlink %+v, want not_applied", d)
	}
}

func TestEngine_NoNetworkWithoutSection(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	r := runEngine(t, scenario.TestPlan{Participants: 2, DurationSeconds: 1}, adapter, 0)
	if r.Network != nil {
		t.Errorf("network result %+v for a plan without network", r.Network)
	}
}

func TestEngine_DrawnSeed(t *testing.T) {
	adapter := &MockPlatformAdapter{t: t}
	plan := scenario.TestPlan{Participants: 1, DurationSeconds: 1, Network: &scenario.NetworkConfig{Groups: []scenario.BotGroup{{Name: "g", Count: 1,
		Schedule: []scenario.ScheduleStep{{Downlink: &scenario.LinkCondition{DelayMs: 10}}}}}}}
	r := runEngine(t, plan, adapter, 0)
	if r.Network == nil || r.Network.SeedSource != scenario.SeedDrawn {
		t.Fatalf("network %+v", r.Network)
	}
}

// estimatingAdapter is a mock adapter whose platform gives its estimate.
type estimatingAdapter struct {
	*MockPlatformAdapter
}

func (estimatingAdapter) ReceiverEstimate(_ context.Context, peerID string) (int, bool, error) {
	return 900, true, nil
}

// The server's estimate is sampled for the constrained bots only, when the
// adapter can read it; otherwise the reason says it is not given (T042).
func TestEngine_ServerEstimate(t *testing.T) {
	old := estimateInterval
	estimateInterval = 100 * time.Millisecond
	t.Cleanup(func() { estimateInterval = old })
	plan := scenario.TestPlan{Participants: 3, DurationSeconds: 1,
		Network: &scenario.NetworkConfig{Groups: []scenario.BotGroup{{Name: "g", Count: 1,
			Schedule: []scenario.ScheduleStep{{Downlink: &scenario.LinkCondition{RateKbps: 300}}}}}}}
	done := make(chan scenario.RunResult, 1)
	e := NewEngine("est-"+uuid.NewString(), plan, estimatingAdapter{&MockPlatformAdapter{t: t}}, quietLogger, func(_ string, r scenario.RunResult) { done <- r })
	go e.Start()
	r := <-done
	if len(r.Network.Bots) != 1 {
		t.Fatalf("bots %+v", r.Network.Bots)
	}
	se := r.Network.Bots[0].ServerEstimate
	if se == nil || se.Reason != "" || len(se.Kbps) < 5 || se.Kbps[0].Kbps != 900 {
		t.Errorf("server estimate %+v, want samples of 900 kbit/s", se)
	}

	r = runEngine(t, plan, &MockPlatformAdapter{t: t}, 0)
	if se := r.Network.Bots[0].ServerEstimate; se == nil || se.Reason != scenario.NotGivenByServer {
		t.Errorf("without an estimating adapter: %+v, want not_given_by_server", se)
	}
}
