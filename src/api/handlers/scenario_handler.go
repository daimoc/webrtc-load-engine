package handlers

import (
	"encoding/json"
	"fmt" // Add fmt import
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
	IncScenarioExecutionRequestsTotal(status string) // New metric
}

// HandlerMetrics implements the Metrics interface
type HandlerMetrics struct {
	scenarioCreationRequestsTotal *prometheus.CounterVec
	scenarioCreationDurationSeconds prometheus.Histogram
	scenarioExecutionRequestsTotal *prometheus.CounterVec // New metric
}

// NewHandlerMetrics creates a new HandlerMetrics instance
func NewHandlerMetrics(
	requestsTotal *prometheus.CounterVec,
	durationSeconds prometheus.Histogram,
	executionRequestsTotal *prometheus.CounterVec, // New metric
) *HandlerMetrics {
	return &HandlerMetrics{
		scenarioCreationRequestsTotal: requestsTotal,
		scenarioCreationDurationSeconds: durationSeconds,
		scenarioExecutionRequestsTotal: executionRequestsTotal, // New metric
	}
}

func (m *HandlerMetrics) IncScenarioCreationRequestsTotal(status string) {
	m.scenarioCreationRequestsTotal.WithLabelValues(status).Inc()
}

func (m *HandlerMetrics) ObserveScenarioCreationDurationSeconds(durationSeconds float64) {
	m.scenarioCreationDurationSeconds.Observe(durationSeconds)
}

func (m *HandlerMetrics) IncScenarioExecutionRequestsTotal(status string) { // New metric method
	m.scenarioExecutionRequestsTotal.WithLabelValues(status).Inc()
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

// ExecuteScenario handles the POST /api/v1/scenarios/{id}/_execute request.
func (h *ScenarioHandler) ExecuteScenario(w http.ResponseWriter, r *http.Request) {
	status := "success"
	defer func() {
		h.metrics.IncScenarioExecutionRequestsTotal(status)
	}()

	h.logger.Info("Received request to execute scenario",
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path))

	if r.Method != http.MethodPost {
		status = "failure"
		h.logger.Warn("Method not allowed", slog.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scenarioID := r.URL.Path[len("/api/v1/scenarios/") : len(r.URL.Path)-len("/_execute")]

	err := h.scenarioService.ExecuteScenario(scenarioID)
	if err != nil {
		status = "failure"
		h.logger.Error("Failed to execute scenario", slog.String("scenario_id", scenarioID), slog.String("error", err.Error()))
		switch err.Error() {
		case fmt.Sprintf("scenario not found: %s", scenarioID):
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		case fmt.Sprintf("scenario is already running: %s", scenarioID):
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		default: // Catch other validation errors like "scenario cannot be executed in current status"
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		}
		return
	}

	h.logger.Info("Scenario execution accepted", slog.String("scenario_id", scenarioID))
	w.WriteHeader(http.StatusAccepted)
}

// GetScenario handles the GET /api/v1/scenarios/{id} request.
func (h *ScenarioHandler) GetScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.logger.Warn("Method not allowed", slog.String("method", r.Method))
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scenarioID := r.URL.Path[len("/api/v1/scenarios/"):]

	scenario, err := h.scenarioService.GetScenario(scenarioID)
	if err != nil {
		h.logger.Error("Failed to get scenario", slog.String("scenario_id", scenarioID), slog.String("error", err.Error()))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(ScenarioResponse{ID: scenario.ID, Status: scenario.Status})
}

