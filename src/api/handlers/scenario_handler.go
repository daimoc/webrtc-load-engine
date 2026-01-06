package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time" // Import time for Prometheus duration tracking

	"github.com/prometheus/client_golang/prometheus" // Import prometheus
	
	"webrtc-load-engine/src/scenario"
)

// ScenarioRequest represents the request body for creating a scenario.
type ScenarioRequest struct {
	Name     string             `json:"name"`
	Platform string             `json:"platform"`
	TestPlan scenario.TestPlan `json:"test_plan"`
}

// ScenarioResponse represents the response body for a created scenario.
type ScenarioResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ErrorResponse represents the error response body.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Metrics interface to avoid direct dependency on global Prometheus variables
type Metrics interface {
	IncScenarioCreationRequestsTotal(status string)
	ObserveScenarioCreationDurationSeconds(durationSeconds float64)
}

// HandlerMetrics implements the Metrics interface
type HandlerMetrics struct {
	scenarioCreationRequestsTotal *prometheus.CounterVec
	scenarioCreationDurationSeconds prometheus.Histogram
}

// NewHandlerMetrics creates a new HandlerMetrics instance
func NewHandlerMetrics(
	requestsTotal *prometheus.CounterVec,
	durationSeconds prometheus.Histogram,
) *HandlerMetrics {
	return &HandlerMetrics{
		scenarioCreationRequestsTotal: requestsTotal,
		scenarioCreationDurationSeconds: durationSeconds,
	}
}

func (m *HandlerMetrics) IncScenarioCreationRequestsTotal(status string) {
	m.scenarioCreationRequestsTotal.WithLabelValues(status).Inc()
}

func (m *HandlerMetrics) ObserveScenarioCreationDurationSeconds(durationSeconds float64) {
	m.scenarioCreationDurationSeconds.Observe(durationSeconds)
}

// ScenarioHandler handles HTTP requests for scenarios.
type ScenarioHandler struct {
	scenarioService *scenario.Service
	logger          *slog.Logger
	metrics         Metrics // Use the interface
}

// NewScenarioHandler creates a new ScenarioHandler.
func NewScenarioHandler(service *scenario.Service, logger *slog.Logger, metrics Metrics) *ScenarioHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &ScenarioHandler{
		scenarioService: service,
		logger:          logger,
		metrics:         metrics,
	}
}

// CreateScenario handles the POST /api/v1/scenarios request.
func (h *ScenarioHandler) CreateScenario(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	status := "success" // Default status

	defer func() {
		h.metrics.IncScenarioCreationRequestsTotal(status)
		h.metrics.ObserveScenarioCreationDurationSeconds(time.Since(startTime).Seconds())
	}()

	h.logger.Info("Received request to create scenario",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path))

	if r.Method != http.MethodPost {
		status = "failure"
		h.logger.Warn("Method not allowed", slog.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		status = "failure"
		h.logger.Error("Invalid request body", slog.String("error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: "Invalid request body"})
		return
	}

	newScenario, err := h.scenarioService.CreateScenario(req.Name, req.Platform, req.TestPlan)
	if err != nil {
		status = "failure"
		h.logger.Error("Failed to create scenario", slog.String("error", err.Error()))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	h.logger.Info("Scenario created successfully", slog.String("scenario_id", newScenario.ID))
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ScenarioResponse{ID: newScenario.ID, Status: newScenario.Status})
}
