package media

import "github.com/prometheus/client_golang/prometheus"

// Media metrics (specs/004-synthetic-media/contracts/metrics.md). Labels are
// the scenario and the media kind only; per-peer detail goes to the logs.
var (
	PacketsSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_packets_sent_total",
		Help: "RTP packets sent by the scenario's bots.",
	}, []string{"scenario_id", "kind"})

	BytesSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_bytes_sent_total",
		Help: "RTP payload bytes sent by the scenario's bots.",
	}, []string{"scenario_id", "kind"})

	PacketsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_packets_received_total",
		Help: "RTP packets received by the scenario's bots.",
	}, []string{"scenario_id", "kind"})

	BytesReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_bytes_received_total",
		Help: "RTP payload bytes received by the scenario's bots.",
	}, []string{"scenario_id", "kind"})

	PacketsLost = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_packets_lost_total",
		Help: "RTP packets the bots expected but did not receive (sequence gaps).",
	}, []string{"scenario_id", "kind"})

	Jitter = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_jitter_seconds",
		Help:    "Interarrival jitter of received streams, one sample per stream per collection.",
		Buckets: []float64{.0005, .001, .002, .005, .01, .02, .05, .1, .2},
	}, []string{"scenario_id", "kind"})

	RoundTrip = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_round_trip_seconds",
		Help:    "Round-trip time from the media server's receiver reports on the bots' streams.",
		Buckets: []float64{.001, .005, .01, .02, .05, .1, .2, .5, 1},
	}, []string{"scenario_id", "kind"})

	RemoteStreams = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "webrtc_media_remote_streams",
		Help: "Remote streams the bots are offered (one per remote participant source), summed over the established bots.",
	}, []string{"scenario_id", "kind"})

	Stalls = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_stalls_total",
		Help: "Incoming media stalls: a kind received before, then nothing for the stall timeout.",
	}, []string{"scenario_id", "kind"})

	SendLateness = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_send_lateness_seconds",
		Help:    "Delay between a frame's scheduled send time and its write to the bots' tracks.",
		Buckets: []float64{.0005, .001, .002, .005, .01, .02, .05, .1, .25},
	}, []string{"scenario_id", "kind"})
)

// Quality metrics (specs/006-media-quality/contracts/metrics.md). Labels are
// the scenario, plus the kind for latency and the height for the time per
// resolution; never a bot.
var (
	FrameRate = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_frame_rate",
		Help:    "Frames per second of each received video stream, one sample per stream per collection period.",
		Buckets: []float64{5, 10, 15, 20, 24, 27, 29, 31, 40},
	}, []string{"scenario_id"})

	Freezes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_freezes_total",
		Help: "Video freezes: a gap between received frames longer than max(3 x the usual interval, 150 ms).",
	}, []string{"scenario_id"})

	FreezeSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_freeze_seconds_total",
		Help: "Total duration of video freezes.",
	}, []string{"scenario_id"})

	FrameHeight = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_frame_height",
		Help:    "Height of each received video stream (last keyframe), one sample per stream per collection period.",
		Buckets: []float64{90, 180, 270, 360, 540, 720, 1080},
	}, []string{"scenario_id"})

	HeightSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_height_seconds_total",
		Help: "Time received video spent at each height.",
	}, []string{"scenario_id", "height"})

	ResolutionSwitches = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_resolution_switches_total",
		Help: "Height changes of received video streams.",
	}, []string{"scenario_id"})

	KeyframeRequestsSent = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_keyframe_requests_sent_total",
		Help: "Keyframe requests (PLI) the bots sent as receivers.",
	}, []string{"scenario_id"})

	KeyframeRequestsReceived = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_keyframe_requests_received_total",
		Help: "Keyframe requests (PLI, FIR) the bots received as senders.",
	}, []string{"scenario_id"})

	AudioConcealedSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_audio_concealed_seconds_total",
		Help: "Received audio a listener would not hear in time (missing or late, 60 ms playout delay).",
	}, []string{"scenario_id"})

	AudioExpectedSeconds = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "webrtc_media_audio_expected_seconds_total",
		Help: "Received audio expected (from sequence numbers).",
	}, []string{"scenario_id"})

	AudioMOS = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_audio_mos",
		Help:    "Estimated listening score (E-model), one sample per audio stream per collection period.",
		Buckets: []float64{1, 2, 3, 3.5, 3.8, 4, 4.1, 4.2, 4.3, 4.4, 4.5},
	}, []string{"scenario_id"})

	Latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "webrtc_media_latency_seconds",
		Help:    "Send-to-receive time of frames between bots of the same engine, one sample per video frame and per audio packet.",
		Buckets: LatencyBounds,
	}, []string{"scenario_id", "kind"})
)

// LatencyBounds are the buckets of webrtc_media_latency_seconds, also used
// for the per-bot latency counts of the quality summary.
var LatencyBounds = []float64{.005, .01, .02, .03, .05, .075, .1, .15, .2, .3, .5, 1}

func init() {
	prometheus.MustRegister(PacketsSent, BytesSent, PacketsReceived, BytesReceived, PacketsLost,
		Jitter, RoundTrip, RemoteStreams, Stalls, SendLateness,
		FrameRate, Freezes, FreezeSeconds, FrameHeight, HeightSeconds, ResolutionSwitches,
		KeyframeRequestsSent, KeyframeRequestsReceived, AudioConcealedSeconds, AudioExpectedSeconds,
		AudioMOS, Latency)
}
