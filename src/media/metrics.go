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

func init() {
	prometheus.MustRegister(PacketsSent, BytesSent, PacketsReceived, BytesReceived, PacketsLost,
		Jitter, RoundTrip, RemoteStreams, Stalls, SendLateness)
}
