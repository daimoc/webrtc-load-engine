package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"webrtc-load-engine/src/api/handlers"
	"webrtc-load-engine/src/execution"
	"webrtc-load-engine/src/platform"
	"webrtc-load-engine/src/platform/jitsi"
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

	// scenarioExecutionRequestsTotal counts the total number of scenario execution requests.
	scenarioExecutionRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "scenario_execution_requests_total",
			Help: "Total number of scenario execution requests.",
		},
		[]string{"status"}, // "status" label for success/failure
	)
)

var (
	executeScenarioRegex = regexp.MustCompile(`^/api/v1/scenarios/([a-zA-Z0-9-]+)/_execute$`)
	getScenarioRegex     = regexp.MustCompile(`^/api/v1/scenarios/([a-zA-Z0-9-]+)$`)
)

func init() {
	// Register metrics with Prometheus's default registry
	prometheus.MustRegister(scenarioCreationRequestsTotal)
	prometheus.MustRegister(scenarioCreationDurationSeconds)
	prometheus.MustRegister(scenarioExecutionRequestsTotal) // Register new metric
}

// MainEngineStarter is an implementation of scenario.ExecutionStarter.
type MainEngineStarter struct {
	logger *slog.Logger
}

// NewMainEngineStarter creates a new MainEngineStarter.
func NewMainEngineStarter(logger *slog.Logger) *MainEngineStarter {
	return &MainEngineStarter{logger: logger}
}

// StartExecution creates and returns a new execution.Engine.
func (es *MainEngineStarter) StartExecution(scenarioID string, testPlan scenario.TestPlan, adapter platform.PlatformAdapter, onComplete func(scenarioID string, status string)) scenario.ExecutionEngine {
	// In a real implementation, the adapter would be dynamically selected based on testPlan.Platform
	// For now, we hardcode jitsi.NewAdapter
	if adapter == nil { // Provide a default if nil is passed
		adapter = jitsi.NewAdapter(es.logger)
	}
	return execution.NewEngine(scenarioID, testPlan, adapter, es.logger, onComplete)
}

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	engineStarter := NewMainEngineStarter(logger)
	scenarioService := scenario.NewService(logger, engineStarter) // Pass starter to service

	// Create HandlerMetrics instance
	handlerMetrics := handlers.NewHandlerMetrics(
		scenarioCreationRequestsTotal,
		scenarioCreationDurationSeconds,
		scenarioExecutionRequestsTotal, // Pass new metric
	)
	scenarioHandler := handlers.NewScenarioHandler(scenarioService, logger, handlerMetrics) // Pass logger and metrics to handler

	mux := http.NewServeMux()

	// Register handlers
	mux.HandleFunc("/api/v1/scenarios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			scenarioHandler.CreateScenario(w, r)
			return
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/api/v1/scenarios/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if executeScenarioRegex.MatchString(r.URL.Path) {
				scenarioHandler.ExecuteScenario(w, r)
				return
			}
		} else if r.Method == http.MethodGet {
			if getScenarioRegex.MatchString(r.URL.Path) {
				scenarioHandler.GetScenario(w, r)
				return
			}
		}
		http.NotFound(w, r)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("WebRTC Load Engine API is running"))
	})
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Create a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// Start HTTP server in a goroutine
	go func() {
		slog.Info("Server starting", slog.String("port", "8080"))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Could not listen on :8080", slog.String("error", err.Error()))
		}
	}()

	// Wait for OS signal
	<-stop

	slog.Info("Shutting down server...")

	// Create a context with a timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down HTTP server
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", slog.String("error", err.Error()))
	} else {
		slog.Info("Server gracefully stopped.")
	}

	// Shut down scenario service (stops all running engines)
	scenarioService.Shutdown()
	slog.Info("Scenario service shutdown complete.")

	slog.Info("Application exited.")
}