package main

import (
	"log"
	"log/slog" // Import slog
	"net/http"
	"os" // Import os for default logger output

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/scenario"
)

// Define Prometheus metrics
var (
	// scenarioCreationRequestsTotal counts the total number of scenario creation requests.
	scenarioCreationRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scenario_creation_requests_total",
			Help: "Total number of scenario creation requests.",
		},
		[]string{"status"}, // "status" label for success/failure
	)

	// scenarioCreationDurationSeconds measures the duration of scenario creation requests.
	scenarioCreationDurationSeconds = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "scenario_creation_duration_seconds",
			Help: "Histogram of scenario creation request latencies in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)
)

func init() {
	// Register metrics with Prometheus's default registry
	prometheus.MustRegister(scenarioCreationRequestsTotal)
	prometheus.MustRegister(scenarioCreationDurationSeconds)
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	scenarioService := scenario.NewService(logger) // Pass logger to service

	// Create HandlerMetrics instance
	handlerMetrics := handlers.NewHandlerMetrics(
		scenarioCreationRequestsTotal,
		scenarioCreationDurationSeconds,
	)
	scenarioHandler := handlers.NewScenarioHandler(scenarioService, logger, handlerMetrics) // Pass logger and metrics to handler

	// Register handlers
	http.HandleFunc("/api/v1/scenarios", scenarioHandler.CreateScenario)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("WebRTC Load Engine API is running"))
	})
	// Expose Prometheus metrics
	http.Handle("/metrics", promhttp.Handler())

	slog.Info("Server starting", slog.String("port", "8080"))
	log.Fatal(http.ListenAndServe(":8080", nil)) // Use log.Fatal for compatibility
}
