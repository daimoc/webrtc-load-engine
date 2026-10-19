package main

import (
	"context"
	"flag"
	"fmt"
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
	"webrtc-load-engine/src/platform/catalog"
	"webrtc-load-engine/src/report"
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
	getQualityRegex      = regexp.MustCompile(`^/api/v1/scenarios/([a-zA-Z0-9-]+)/quality$`)
)

func init() {
	// Register metrics with Prometheus's default registry
	prometheus.MustRegister(scenarioCreationRequestsTotal)
	prometheus.MustRegister(scenarioCreationDurationSeconds)
	prometheus.MustRegister(scenarioExecutionRequestsTotal) // Register new metric
}

// MainEngineStarter is an implementation of scenario.ExecutionStarter. It
// also implements scenario.PlatformCatalog over the built-in platforms.
type MainEngineStarter struct {
	logger  *slog.Logger
	catalog *catalog.Catalog
}

var _ scenario.PlatformCatalog = (*MainEngineStarter)(nil)

// NewMainEngineStarter creates a new MainEngineStarter.
func NewMainEngineStarter(logger *slog.Logger) *MainEngineStarter {
	return &MainEngineStarter{logger: logger, catalog: catalog.New(logger)}
}

// StartExecution creates and returns a new execution.Engine. The service
// passes the adapter its catalog built for the scenario's platform; when no
// adapter is supplied, a Jitsi adapter is built from testPlan.PlatformConfig.
func (es *MainEngineStarter) StartExecution(scenarioID string, testPlan scenario.TestPlan, adapter platform.PlatformAdapter, onComplete func(scenarioID string, r scenario.RunResult)) (scenario.ExecutionEngine, error) {
	if adapter == nil {
		a, err := es.catalog.NewAdapter("jitsi", testPlan)
		if err != nil {
			return nil, fmt.Errorf("invalid platform_config: %w", err)
		}
		adapter = a
	}
	return execution.NewEngine(scenarioID, testPlan, adapter, es.logger, onComplete), nil
}

// Platforms implements scenario.PlatformCatalog.
func (es *MainEngineStarter) Platforms() []string { return es.catalog.Platforms() }

// ValidateTestPlan implements scenario.PlatformCatalog.
func (es *MainEngineStarter) ValidateTestPlan(name string, tp scenario.TestPlan) error {
	return es.catalog.ValidateTestPlan(name, tp)
}

// SecretKeys implements scenario.PlatformCatalog.
func (es *MainEngineStarter) SecretKeys(name string) []string { return es.catalog.SecretKeys(name) }

// NewAdapter implements scenario.PlatformCatalog.
func (es *MainEngineStarter) NewAdapter(name string, tp scenario.TestPlan) (platform.PlatformAdapter, error) {
	return es.catalog.NewAdapter(name, tp)
}

func main() {
	reportsFlag := flag.String("reports-dir", "", "Folder of the run reports (default $"+report.EnvDir+", else ./reports)")
	flag.Parse()

	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	engineStarter := NewMainEngineStarter(logger)
	scenarioService := scenario.NewService(logger, engineStarter) // Pass starter to service
	reportsDir, err := report.ResolveDir(*reportsFlag)
	if err != nil {
		logger.Error("Cannot resolve the reports folder", slog.String("error", err.Error()))
		os.Exit(1)
	}
	scenarioService.SetReporter(report.Writer{Dir: reportsDir, Logger: logger})
	logger.Info("Run reports folder", slog.String("dir", reportsDir))

	// Create HandlerMetrics instance
	handlerMetrics := handlers.NewHandlerMetrics(
		scenarioCreationRequestsTotal,
		scenarioCreationDurationSeconds,
		scenarioExecutionRequestsTotal, // Pass new metric
	)
	scenarioHandler := handlers.NewScenarioHandler(scenarioService, logger, handlerMetrics) // Pass logger and metrics to handler

	mux := newRouter(scenarioHandler)

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

// newRouter returns the HTTP API's routes.
func newRouter(scenarioHandler *handlers.ScenarioHandler) *http.ServeMux {
	mux := http.NewServeMux()
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
			if getQualityRegex.MatchString(r.URL.Path) {
				scenarioHandler.GetQuality(w, r)
				return
			}
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
	return mux
}
