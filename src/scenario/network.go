package scenario

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
)

// The network section of a test plan
// (specs/016-network-conditions/contracts/test-plan-network.md): groups of
// bots, each with a schedule of conditions on its path to the server, in
// each direction. Platform-independent: no adapter reads it.

// Defaults of the network section.
const (
	DefaultSettleSeconds = 10
	DefaultQueueMs       = 200
)

// Limits of the network section.
const (
	maxNetworkGroups = 16
	maxGroupSteps    = 32
	maxSettleSeconds = 300
	minRateKbps      = 16
	maxRateKbps      = 1000000
	minQueueMs       = 10
	maxQueueMs       = 2000
	maxDelayMs       = 5000
)

var groupNameRE = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// NetworkConfig is the network part of a test plan.
type NetworkConfig struct {
	// Seed draws random loss and jitter; nil: drawn by the engine, and
	// reported.
	Seed *uint64 `json:"seed,omitempty"`
	// SettleSeconds is how long after the last join a step still counts as
	// starting while the room starts up; nil: DefaultSettleSeconds.
	SettleSeconds *int       `json:"settle_seconds,omitempty"`
	Groups        []BotGroup `json:"groups"`
}

// BotGroup is a set of bots, given by Count or by Share of the
// participants, with one schedule.
type BotGroup struct {
	Name     string         `json:"name"`
	Count    int            `json:"count,omitempty"`
	Share    float64        `json:"share,omitempty"`
	Schedule []ScheduleStep `json:"schedule"`
}

// ScheduleStep is a condition from StartSeconds (counted from the run's
// first launch) for DurationSeconds, or to the end of the run when nil.
type ScheduleStep struct {
	StartSeconds    int            `json:"start_seconds"`
	DurationSeconds *int           `json:"duration_seconds,omitempty"`
	Downlink        *LinkCondition `json:"downlink,omitempty"` // Server to bot
	Uplink          *LinkCondition `json:"uplink,omitempty"`   // Bot to server
}

// LinkCondition is one direction's condition; zero fields are not applied.
type LinkCondition struct {
	RateKbps int   `json:"rate_kbps,omitempty"`
	QueueMs  int   `json:"queue_ms,omitempty"` // With RateKbps; DefaultQueueMs when 0
	DelayMs  int   `json:"delay_ms,omitempty"`
	JitterMs int   `json:"jitter_ms,omitempty"`
	Loss     *Loss `json:"loss,omitempty"`
}

// Loss drops a Share of the datagrams, in bursts of MeanBurst datagrams on
// average (1: random loss).
type Loss struct {
	Share     float64 `json:"share"`
	MeanBurst float64 `json:"mean_burst,omitempty"`
}

// UnmarshalJSON refuses unknown fields, here and in the nested objects.
func (n *NetworkConfig) UnmarshalJSON(data []byte) error {
	type plain NetworkConfig
	var p plain
	if err := strictUnmarshal(data, &p); err != nil {
		return fmt.Errorf("network: %w", err)
	}
	*n = NetworkConfig(p)
	return nil
}

// Settle returns the settling time in seconds.
func (n *NetworkConfig) Settle() int {
	if n == nil || n.SettleSeconds == nil {
		return DefaultSettleSeconds
	}
	return *n.SettleSeconds
}

// Size returns the number of bots of the group among participants.
func (g BotGroup) Size(participants int) int {
	if g.Count > 0 {
		return g.Count
	}
	return max(1, int(math.Round(g.Share*float64(participants))))
}

// End returns the second at which the step ends in a run of runSeconds.
func (s ScheduleStep) End(runSeconds int) int {
	if s.DurationSeconds == nil {
		return runSeconds
	}
	return s.StartSeconds + *s.DurationSeconds
}

// HasUplink reports whether a step of the group conditions its bots'
// upload.
func (g BotGroup) HasUplink() bool {
	for _, s := range g.Schedule {
		if s.Uplink != nil {
			return true
		}
	}
	return false
}

// Assign returns the group name of each bot by launch index ("" for no
// group): groups take bots from the end of the launch order, the first
// group first.
func (n *NetworkConfig) Assign(participants int) []string {
	names := make([]string, participants)
	if n == nil {
		return names
	}
	next := participants - 1
	for _, g := range n.Groups {
		for k := g.Size(participants); k > 0 && next >= 0; k-- {
			names[next] = g.Name
			next--
		}
	}
	return names
}

// Group returns the group named name, or nil.
func (n *NetworkConfig) Group(name string) *BotGroup {
	if n == nil {
		return nil
	}
	for i := range n.Groups {
		if n.Groups[i].Name == name {
			return &n.Groups[i]
		}
	}
	return nil
}

// withDefaults validates n against the plan and returns a copy with
// defaults applied; nil stays nil.
func (n *NetworkConfig) withDefaults(plan TestPlan, profile string) (*NetworkConfig, error) {
	if n == nil {
		return nil, nil
	}
	c := *n
	if c.SettleSeconds != nil {
		if s := *c.SettleSeconds; s < 0 || s > maxSettleSeconds {
			return nil, fmt.Errorf("network.settle_seconds must be between 0 and %d, got %d", maxSettleSeconds, s)
		}
		v := *c.SettleSeconds
		c.SettleSeconds = &v
	}
	if c.Seed != nil {
		v := *c.Seed
		c.Seed = &v
	}
	if len(c.Groups) < 1 || len(c.Groups) > maxNetworkGroups {
		return nil, fmt.Errorf("network.groups must list 1 to %d groups, got %d", maxNetworkGroups, len(c.Groups))
	}
	run := plan.RampUpSeconds + plan.DurationSeconds
	seen := map[string]bool{}
	total := 0
	c.Groups = make([]BotGroup, len(n.Groups))
	for i, src := range n.Groups {
		g := src
		at := fmt.Sprintf("network.groups[%d]", i)
		switch {
		case !groupNameRE.MatchString(g.Name):
			return nil, fmt.Errorf("%s.name must match [a-z0-9_-]{1,32}, got %q", at, g.Name)
		case seen[g.Name]:
			return nil, fmt.Errorf("%s.name %q is used by another group", at, g.Name)
		}
		seen[g.Name] = true
		switch {
		case (g.Count != 0) == (g.Share != 0):
			return nil, fmt.Errorf("%s needs exactly one of count or share", at)
		case g.Count < 0 || g.Count > plan.Participants:
			return nil, fmt.Errorf("%s.count must be between 1 and participants (%d), got %d", at, plan.Participants, g.Count)
		case g.Share < 0 || g.Share > 1:
			return nil, fmt.Errorf("%s.share must be over 0 and at most 1, got %v", at, g.Share)
		}
		total += g.Size(plan.Participants)
		if len(g.Schedule) < 1 || len(g.Schedule) > maxGroupSteps {
			return nil, fmt.Errorf("%s.schedule must list 1 to %d steps, got %d", at, maxGroupSteps, len(g.Schedule))
		}
		g.Schedule = make([]ScheduleStep, len(src.Schedule))
		prevEnd := -1
		for j, step := range src.Schedule {
			sat := fmt.Sprintf("%s.schedule[%d]", at, j)
			s, err := step.withDefaults(sat, run, profile)
			if err != nil {
				return nil, err
			}
			if s.StartSeconds < prevEnd {
				return nil, fmt.Errorf("%s.start_seconds %d is before the end of the previous step (%d): steps must be ordered and not overlap", sat, s.StartSeconds, prevEnd)
			}
			prevEnd = s.End(run)
			g.Schedule[j] = s
		}
		c.Groups[i] = g
	}
	if total > plan.Participants {
		return nil, fmt.Errorf("network.groups hold %d bots, more than participants (%d)", total, plan.Participants)
	}
	return &c, nil
}

func (s ScheduleStep) withDefaults(at string, run int, profile string) (ScheduleStep, error) {
	switch {
	case s.StartSeconds < 0 || s.StartSeconds >= run:
		return s, fmt.Errorf("%s.start_seconds must be from 0 to before the end of the run (%d s), got %d", at, run, s.StartSeconds)
	case s.DurationSeconds != nil && *s.DurationSeconds < 1:
		return s, fmt.Errorf("%s.duration_seconds must be at least 1, got %d", at, *s.DurationSeconds)
	case s.DurationSeconds != nil && s.End(run) > run:
		return s, fmt.Errorf("%s.duration_seconds ends the step at %d s, after the end of the run (%d s)", at, s.End(run), run)
	case s.Downlink == nil && s.Uplink == nil:
		return s, fmt.Errorf("%s needs a downlink or an uplink condition", at)
	case s.Uplink != nil && profile == MediaProfileReceiveOnly:
		return s, fmt.Errorf("%s.uplink: a receive_only bot sends nothing to condition", at)
	}
	if s.DurationSeconds != nil {
		v := *s.DurationSeconds
		s.DurationSeconds = &v
	}
	var err error
	if s.Downlink, err = s.Downlink.withDefaults(at + ".downlink"); err != nil {
		return s, err
	}
	if s.Uplink, err = s.Uplink.withDefaults(at + ".uplink"); err != nil {
		return s, err
	}
	return s, nil
}

func (l *LinkCondition) withDefaults(at string) (*LinkCondition, error) {
	if l == nil {
		return nil, nil
	}
	c := *l
	switch {
	case c == (LinkCondition{}):
		return nil, fmt.Errorf("%s sets no condition", at)
	case c.RateKbps != 0 && (c.RateKbps < minRateKbps || c.RateKbps > maxRateKbps):
		return nil, fmt.Errorf("%s.rate_kbps must be between %d and %d, got %d", at, minRateKbps, maxRateKbps, c.RateKbps)
	case c.QueueMs != 0 && c.RateKbps == 0:
		return nil, fmt.Errorf("%s.queue_ms needs rate_kbps", at)
	case c.QueueMs != 0 && (c.QueueMs < minQueueMs || c.QueueMs > maxQueueMs):
		return nil, fmt.Errorf("%s.queue_ms must be between %d and %d, got %d", at, minQueueMs, maxQueueMs, c.QueueMs)
	case c.DelayMs < 0 || c.DelayMs > maxDelayMs:
		return nil, fmt.Errorf("%s.delay_ms must be between 0 and %d, got %d", at, maxDelayMs, c.DelayMs)
	case c.JitterMs < 0 || c.JitterMs > c.DelayMs:
		return nil, fmt.Errorf("%s.jitter_ms must be between 0 and delay_ms (%d), got %d", at, c.DelayMs, c.JitterMs)
	}
	if c.RateKbps != 0 && c.QueueMs == 0 {
		c.QueueMs = DefaultQueueMs
	}
	if c.Loss != nil {
		loss := *c.Loss
		switch {
		case loss.Share < 0 || loss.Share > 1:
			return nil, fmt.Errorf("%s.loss.share must be between 0 and 1, got %v", at, loss.Share)
		case loss.MeanBurst != 0 && loss.MeanBurst < 1:
			return nil, fmt.Errorf("%s.loss.mean_burst must be at least 1, got %v", at, loss.MeanBurst)
		}
		if loss.MeanBurst == 0 {
			loss.MeanBurst = 1
		}
		c.Loss = &loss
	}
	return &c, nil
}

// ReadNetworkFile reads a network section from a JSON file (the
// `--network` flag and `<PLATFORM>_NETWORK` of the live tests); unknown
// fields are refused. It is validated with the rest of the plan.
func ReadNetworkFile(path string) (*NetworkConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("network file: %w", err)
	}
	var n NetworkConfig
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("network file %s: %w", path, err)
	}
	return &n, nil
}
