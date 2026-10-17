// Package hostload samples the CPU and memory of the media server's
// containers and of the engine's own process during a run, and folds the
// samples into the run's summary (specs/007-server-load-report). It knows
// containers and processes only: no media, no platform.
package hostload

import "github.com/prometheus/client_golang/prometheus"

// Server load metrics (specs/007-server-load-report/contracts/metrics.md),
// updated at each sample and deleted when the scenario's run ends.
var (
	ServerCPUThreads = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "webrtc_server_cpu_threads",
		Help: "CPU of a media server container over the last interval, in threads (1.0 = one thread).",
	}, []string{"scenario_id", "container"})

	ServerMemoryBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "webrtc_server_memory_bytes",
		Help: "Memory of a media server container, as docker stats shows it.",
	}, []string{"scenario_id", "container"})

	EngineCPUThreads = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "webrtc_engine_cpu_threads",
		Help: "CPU of the engine's process over the last interval, in threads.",
	}, []string{"scenario_id"})

	EngineMemoryBytes = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "webrtc_engine_memory_bytes",
		Help: "Resident memory of the engine's process.",
	}, []string{"scenario_id"})
)

func init() {
	prometheus.MustRegister(ServerCPUThreads, ServerMemoryBytes, EngineCPUThreads, EngineMemoryBytes)
}

// deleteSeries removes a scenario's series.
func deleteSeries(scenarioID string, containers []string) {
	for _, c := range containers {
		ServerCPUThreads.DeleteLabelValues(scenarioID, c)
		ServerMemoryBytes.DeleteLabelValues(scenarioID, c)
	}
	EngineCPUThreads.DeleteLabelValues(scenarioID)
	EngineMemoryBytes.DeleteLabelValues(scenarioID)
}
