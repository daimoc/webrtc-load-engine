package scenario

import "time"

// Quality summary of a run (specs/006-media-quality/contracts/quality-summary.yaml).
// The engine builds it at the end of a run; the service keeps it with the
// scenario until the next run.

// Percentiles are the 50th, 95th and 99th percentiles of a measure over the
// bots.
type Percentiles struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
}

// KindQuality is what both media kinds report.
type KindQuality struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"` // Why the measures are missing when Available is false
	Bots      int    `json:"bots"`             // Bots that received this kind
	// Latency is over bots, of each bot's median latency; nil when no latency
	// was measured.
	Latency          *Percentiles `json:"latency_seconds,omitempty"`
	LatencyAvailable bool         `json:"latency_available"`
}

// VideoQuality is the video part of the summary.
type VideoQuality struct {
	KindQuality
	FrameRate                     *Percentiles       `json:"frame_rate,omitempty"`
	FreezeSeconds                 *Percentiles       `json:"freeze_seconds,omitempty"`
	FreezesTotal                  int64              `json:"freezes_total"`
	ResolutionSwitchesTotal       int64              `json:"resolution_switches_total"`
	KeyframeRequestsSentTotal     int64              `json:"keyframe_requests_sent_total"`
	KeyframeRequestsReceivedTotal int64              `json:"keyframe_requests_received_total"`
	HeightShare                   map[string]float64 `json:"height_share,omitempty"` // Share of received video time per height
	// RequestedHeightShare is over bots, of the share of each bot's
	// received video time at the height it asked for.
	RequestedHeightShare *Percentiles `json:"requested_height_share,omitempty"`
}

// AudioQuality is the audio part of the summary.
type AudioQuality struct {
	KindQuality
	ConcealedRatio *Percentiles `json:"concealed_ratio,omitempty"`
	MOS            *Percentiles `json:"mos,omitempty"`
}

// BotValue is one bot of a ranking.
type BotValue struct {
	PeerID string  `json:"peer_id"`
	Value  float64 `json:"value"`
}

// WorstBots lists, per ranking, up to 10 bots, worst first.
type WorstBots struct {
	LowestMOS                []BotValue `json:"lowest_mos"`
	MostFreezeSeconds        []BotValue `json:"most_freeze_seconds"`
	HighestLatencyP95Seconds []BotValue `json:"highest_latency_p95_seconds"`
	LowestRequestedHeight    []BotValue `json:"lowest_requested_height_share"`
}

// QualitySummary is the quality of what the bots received during a run.
type QualitySummary struct {
	ScenarioID  string       `json:"scenario_id"`
	GeneratedAt time.Time    `json:"generated_at"`
	Bots        int          `json:"bots"`
	Video       VideoQuality `json:"video"`
	Audio       AudioQuality `json:"audio"`
	Worst       WorstBots    `json:"worst"`
}
