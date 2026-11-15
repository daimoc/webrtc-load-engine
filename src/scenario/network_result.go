package scenario

// The network part of a run's result and report
// (specs/016-network-conditions/contracts/report-network.md).

// Reasons in the network result, where a measure or behaviour is absent.
const (
	NotApplied            = "not_applied"              // No bot of the group connected during the step
	NotAtRequestedHeight  = "not_at_requested_height"  // No stream at its height when the step began
	NoVideo               = "no_video"                 // The bot received no video during the step
	NotRecoveredBeforeEnd = "not_recovered_before_end" // The run ended first
	NotLowered            = "not_lowered"              // Neither a lower height nor a freeze during the step
	NoUplinkCondition     = "no_uplink_condition"      // The bot's upload has no condition: it sends as before
	CannotAdapt           = "cannot_adapt"             // A single-stream sender
	NoFeedback            = "no_feedback"              // No transport-wide feedback from the server
	ReceiveOnly           = "receive_only"             // The bot sends nothing
	NotGivenByServer      = "not_given_by_server"      // The adapter cannot read the server's estimate
)

// Seed sources.
const (
	SeedFromTestPlan = "test_plan"
	SeedDrawn        = "drawn"
)

// NetworkResult is the network section of a run.
type NetworkResult struct {
	Seed                      uint64          `json:"seed"`
	SeedSource                string          `json:"seed_source"`
	SettleSeconds             int             `json:"settle_seconds"`
	TimelineResolutionSeconds int             `json:"timeline_resolution_seconds"`
	Groups                    []GroupResult   `json:"groups"`
	UnconstrainedQuality      *QualitySummary `json:"unconstrained_quality"`
	Bots                      []BotNetwork    `json:"bots"`
}

// GroupResult is one group: its bots, its steps as applied, its quality.
type GroupResult struct {
	Name    string          `json:"name"`
	Bots    []string        `json:"bots"`
	Steps   []AppliedStep   `json:"steps"`
	Quality *QualitySummary `json:"quality"`
}

// AppliedStep is a step of a group's schedule as it ran.
type AppliedStep struct {
	Index   int          `json:"index"`
	Planned ScheduleStep `json:"planned"`
	// StartedAtSeconds and EndedAtSeconds are counted from the run's first
	// launch; nil: the step never started (the run ended first).
	StartedAtSeconds *float64 `json:"started_at_seconds"`
	EndedAtSeconds   *float64 `json:"ended_at_seconds"`
	DuringStartUp    bool     `json:"during_start_up"`
}

// BotNetwork is one constrained bot's network results.
type BotNetwork struct {
	Bot             string          `json:"bot"`
	Group           string          `json:"group"`
	RequestedHeight int             `json:"requested_height"`
	Steps           []BotStep       `json:"steps"`
	Sender          *SenderResult   `json:"sender"`
	ServerEstimate  *ServerEstimate `json:"server_estimate"`
}

// BotStep is what one step did to one bot, and how the bot's video fared.
type BotStep struct {
	Index    int               `json:"index"`
	Downlink *DirectionMeasure `json:"downlink"`
	Uplink   *DirectionMeasure `json:"uplink"`
	// MissingAfterRetransmissionShare is the received video still missing
	// after the server's retransmissions during the step; nil with no video.
	MissingAfterRetransmissionShare *float64 `json:"missing_after_retransmission_share"`
	ReactionSeconds                 *float64 `json:"reaction_seconds"`
	RecoverySeconds                 *float64 `json:"recovery_seconds"`
	Recovered                       *bool    `json:"recovered"`
	Reason                          string   `json:"reason,omitempty"`
	// ConstrainedSenders, for an uplink step of other bots, names the
	// constrained senders whose streams this bot received.
	ConstrainedSenders []string `json:"constrained_senders,omitempty"`
}

// DirectionMeasure is what one direction of the link measurably did during
// a step; never copied from the plan.
type DirectionMeasure struct {
	OfferedPackets     uint64   `json:"offered_packets"`
	DeliveredPackets   uint64   `json:"delivered_packets"`
	DeliveredKbps      KbpsMean `json:"delivered_kbps"`
	DroppedByCapShare  *float64 `json:"dropped_by_cap_share"`
	DroppedByLossShare *float64 `json:"dropped_by_loss_share"`
	MeanBurst          *float64 `json:"mean_burst"` // Datagrams per loss burst; nil without loss
	AddedDelayMs       MeanMax  `json:"added_delay_ms"`
	Reason             string   `json:"reason,omitempty"`
}

// KbpsMean is a delivered rate: over the step, and the highest 10-second
// window.
type KbpsMean struct {
	Mean   float64 `json:"mean"`
	Max10s float64 `json:"max_10s"`
}

// SenderResult is how a bot's sending adapted to its upload.
type SenderResult struct {
	Adapts       bool             `json:"adapts"`
	Reason       string           `json:"reason,omitempty"`
	Layers       []LayerChange    `json:"layers,omitempty"`
	EstimateKbps []EstimateSample `json:"estimate_kbps,omitempty"`
}

// LayerChange is the set of layers a sender sent from AtSeconds on.
type LayerChange struct {
	AtSeconds float64 `json:"at_seconds"`
	Heights   []int   `json:"heights"`
}

// EstimateSample is a bandwidth estimate at a time of the run.
type EstimateSample struct {
	AtSeconds float64 `json:"at_seconds"`
	Kbps      int     `json:"kbps"`
}

// ServerEstimate is the server's estimate toward a bot, as its signaling
// gives it, or the reason it is absent.
type ServerEstimate struct {
	Reason string           `json:"reason,omitempty"`
	Kbps   []EstimateSample `json:"kbps,omitempty"`
}
