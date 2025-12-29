package search

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/qdrant"
)

// HealthChecker provides health check capabilities.
type HealthChecker struct {
	ml     ml.Service
	qdrant *qdrant.Client
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(mlSvc ml.Service, qc *qdrant.Client) *HealthChecker {
	return &HealthChecker{
		ml:     mlSvc,
		qdrant: qc,
	}
}

// HealthStatus represents the overall health status.
type HealthStatus struct {
	Status     string               `json:"status"` // healthy, degraded, unhealthy
	Timestamp  time.Time            `json:"timestamp"`
	Version    string               `json:"version,omitempty"`
	Uptime     string               `json:"uptime,omitempty"`
	Components map[string]Component `json:"components"`
}

// Component represents a component's health.
type Component struct {
	Status  string `json:"status"` // healthy, degraded, unhealthy
	Message string `json:"message,omitempty"`
	Latency int64  `json:"latency_ms,omitempty"`
}

// Check performs a full health check.
func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		Status:     "healthy",
		Timestamp:  time.Now(),
		Components: make(map[string]Component),
	}

	// Check ML service
	mlHealth := h.checkML()
	status.Components["ml"] = mlHealth
	if mlHealth.Status != "healthy" {
		status.Status = "degraded"
	}

	// Check Qdrant
	qdrantHealth := h.checkQdrant(ctx)
	status.Components["qdrant"] = qdrantHealth
	if qdrantHealth.Status == "unhealthy" {
		status.Status = "unhealthy"
	} else if qdrantHealth.Status == "degraded" && status.Status == "healthy" {
		status.Status = "degraded"
	}

	return status
}

// checkML checks ML service health.
func (h *HealthChecker) checkML() Component {
	if h.ml == nil {
		return Component{
			Status:  "unhealthy",
			Message: "ML service not configured",
		}
	}

	health := h.ml.Health()
	if !health.Healthy {
		return Component{
			Status:  "unhealthy",
			Message: health.Error,
		}
	}

	// Check which models are loaded
	allLoaded := true
	for model, loaded := range health.ModelsLoaded {
		if !loaded && (model == "embedder" || model == "sparse") {
			allLoaded = false
			break
		}
	}

	if !allLoaded {
		return Component{
			Status:  "degraded",
			Message: "some models not loaded",
		}
	}

	return Component{
		Status:  "healthy",
		Message: "all models loaded",
	}
}

// checkQdrant checks Qdrant connectivity.
func (h *HealthChecker) checkQdrant(ctx context.Context) Component {
	if h.qdrant == nil {
		return Component{
			Status:  "unhealthy",
			Message: "Qdrant client not configured",
		}
	}

	start := time.Now()
	err := h.qdrant.HealthCheck(ctx)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return Component{
			Status:  "unhealthy",
			Message: err.Error(),
			Latency: latency,
		}
	}

	return Component{
		Status:  "healthy",
		Message: "connected",
		Latency: latency,
	}
}

// HealthHandler handles health check HTTP requests.
type HealthHandler struct {
	checker   *HealthChecker
	startTime time.Time
	version   string
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(checker *HealthChecker, version string) *HealthHandler {
	return &HealthHandler{
		checker:   checker,
		startTime: time.Now(),
		version:   version,
	}
}

// HandleHealth handles GET /healthz (simple liveness check).
func (h *HealthHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// HandleReady handles GET /readyz (readiness check).
func (h *HealthHandler) HandleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	status := h.checker.Check(ctx)
	status.Version = h.version
	status.Uptime = time.Since(h.startTime).Round(time.Second).String()

	w.Header().Set("Content-Type", "application/json")

	if status.Status == "healthy" {
		w.WriteHeader(http.StatusOK)
	} else if status.Status == "degraded" {
		w.WriteHeader(http.StatusOK) // Still OK but with warnings
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// HandleVersion handles GET /v1/version.
func (h *HealthHandler) HandleVersion(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":    h.version,
		"uptime":     time.Since(h.startTime).Round(time.Second).String(),
		"go_version": "go1.21+",
	})
}

// HandleDetailedHealth handles GET /v1/health (detailed health).
func (h *HealthHandler) HandleDetailedHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status := h.checker.Check(ctx)
	status.Version = h.version
	status.Uptime = time.Since(h.startTime).Round(time.Second).String()

	w.Header().Set("Content-Type", "application/json")

	if status.Status == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(status)
}

// RegisterRoutes registers health routes with the given mux.
func (h *HealthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", h.HandleHealth)
	mux.HandleFunc("GET /readyz", h.HandleReady)
	mux.HandleFunc("GET /v1/version", h.HandleVersion)
	mux.HandleFunc("GET /v1/health", h.HandleDetailedHealth)
}

// RegisterRoutesLegacy registers routes for older mux.
func (h *HealthHandler) RegisterRoutesLegacy(mux *http.ServeMux) {
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleHealth(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleReady(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleVersion(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.HandleDetailedHealth(w, r)
		} else {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
