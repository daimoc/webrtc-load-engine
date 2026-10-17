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
	// Server and LoadMachine are the load of the media server's containers
	// and of the engine during the run (007).
	Server      ServerLoad  `json:"server"`
	LoadMachine LoadMachine `json:"load_machine"`
}

// Server load of a run (specs/007-server-load-report/contracts/server-load-summary.yaml).

// Load statuses of a container.
const (
	LoadStatusMeasured    = "measured"
	LoadStatusPartial     = "partial" // Some intervals missed
	LoadStatusUnavailable = "unavailable"
)

// Reasons a load figure is missing.
const (
	ReasonNotRequested      = "not_requested"
	ReasonDockerUnreachable = "docker_unreachable"
	ReasonPermissionDenied  = "permission_denied"
	ReasonRemoteDockerHost  = "remote_docker_host"
	ReasonNotFound          = "not_found"
	ReasonNotRunning        = "not_running"
	ReasonNoSamples         = "no_samples"
)

// Period is the time from the first to the last sample of a run.
type Period struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CPUThreads is CPU in threads: 1.0 is one hardware thread fully busy
// (docker stats % / 100). Never clamped.
type CPUThreads struct {
	Average float64 `json:"average"` // CPU time over the run / wall time over the run
	Peak    float64 `json:"peak"`    // Highest value over one interval
	First   float64 `json:"first"`   // Value over the first interval
}

// MemoryBytes is the peak and first-sample memory.
type MemoryBytes struct {
	Peak  int64 `json:"peak"`
	First int64 `json:"first"`
}

// LoadFigures is the load of one target (a container, all of them, or the
// engine) over a run.
type LoadFigures struct {
	CPUThreads  CPUThreads  `json:"cpu_threads"`
	MemoryBytes MemoryBytes `json:"memory_bytes"`
	Samples     int         `json:"samples"`
	Missed      int         `json:"missed"` // Intervals without a sample (error, timeout, restart)
}

// ContainerLoad is the load of one named container.
type ContainerLoad struct {
	Name   string `json:"name"`
	Status string `json:"status"`           // LoadStatus*
	Reason string `json:"reason,omitempty"` // Reason* when not measured
	// SharedWith lists other scenarios of this engine that sampled the
	// same container during this run.
	SharedWith []string `json:"shared_with,omitempty"`
	LoadFigures
}

// ServerTotal is the load over the measured containers; peaks are of the
// per-sample sum.
type ServerTotal struct {
	ContainersMeasured int `json:"containers_measured"`
	LoadFigures
}

// ServerLoad is the load of the media server's containers.
type ServerLoad struct {
	Available       bool            `json:"available"` // At least one container measured
	Reason          string          `json:"reason,omitempty"`
	IntervalSeconds int             `json:"interval_seconds,omitempty"`
	Period          *Period         `json:"period,omitempty"`
	Containers      []ContainerLoad `json:"containers,omitempty"`
	Total           *ServerTotal    `json:"total,omitempty"`
}

// LoadMachine is the load of the engine's process.
type LoadMachine struct {
	Period          *Period     `json:"period,omitempty"`
	Engine          LoadFigures `json:"engine"`
	HardwareThreads int         `json:"hardware_threads"`
	UsableThreads   int         `json:"usable_threads"` // Threads the engine may run on (CPU affinity)
}
