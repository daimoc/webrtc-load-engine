package execution

import "github.com/prometheus/client_golang/prometheus"

// Per-peer lifecycle metrics, labelled by scenario so that signaling, ICE and
// media series of one execution can be correlated. Individual peers are not
// used as labels (unbounded cardinality); per-peer detail is carried by the
// structured logs, which use the same scenario_id and peer_id keys.
var (
	peersByState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webrtc_peers",
			Help: "Number of peers per lifecycle state.",
		},
		[]string{"scenario_id", "state"},
	)

	signalingConnectSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webrtc_peer_signaling_connect_seconds",
			Help:    "Time for a peer to connect to the platform signaling and join the room.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"scenario_id"},
	)

	negotiationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webrtc_peer_negotiation_seconds",
			Help:    "Time from receiving an SDP offer to producing the answer (including ICE gathering).",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"scenario_id"},
	)

	transportConnectSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "webrtc_peer_transport_connect_seconds",
			Help:    "Time from producing the SDP answer to the ICE+DTLS transport being connected.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"scenario_id"},
	)

	dataChannelsOpen = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "webrtc_data_channels_open",
			Help: "Data channels currently open on the scenario's peers (for Jitsi, the bots whose bridge channel is open).",
		},
		[]string{"scenario_id"},
	)

	transportBytesReceived = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_transport_bytes_received_total",
			Help: "RTP and RTCP bytes (decrypted) received by the peers from the media server, including streams the peers drop.",
		},
		[]string{"scenario_id"},
	)

	peerFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "webrtc_peer_failures_total",
			Help: "Peer failures by lifecycle stage (signaling, negotiation, transport, signaling_lost).",
		},
		[]string{"scenario_id", "stage"},
	)
)

// Failure stages used as the "stage" label of webrtc_peer_failures_total.
const (
	stageSignaling     = "signaling"
	stageNegotiation   = "negotiation"
	stageTransport     = "transport"
	stageSignalingLost = "signaling_lost"
)

func init() {
	prometheus.MustRegister(peersByState, signalingConnectSeconds, negotiationSeconds, transportConnectSeconds, peerFailures, dataChannelsOpen, transportBytesReceived)
}
