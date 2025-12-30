package search

import (
	"context"
	"runtime"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/store"
)

// DetailedHealthStatus represents comprehensive health information.
type DetailedHealthStatus struct {
	Status        string                 `json:"status"` // healthy, degraded, unhealthy
	Version       string                 `json:"version"`
	GitCommit     string                 `json:"git_commit,omitempty"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
	Timestamp     time.Time              `json:"timestamp"`
	Checks        map[string]interface{} `json:"checks"`
	System        *SystemInfo            `json:"system,omitempty"`
}

// QdrantCheck represents detailed Qdrant health information.
type QdrantCheck struct {
	Status      string `json:"status"`
	URL         string `json:"url,omitempty"`
	Version     string `json:"version,omitempty"`
	LatencyMs   int64  `json:"latency_ms"`
	Collections int    `json:"collections,omitempty"`
	Error       string `json:"error,omitempty"`
}

// ModelInfo represents information about a loaded model.
type ModelInfo struct {
	Loaded   bool   `json:"loaded"`
	Name     string `json:"name,omitempty"`
	MemoryMB int64  `json:"memory_mb,omitempty"`
}

// ModelsCheck represents detailed ML models health information.
type ModelsCheck struct {
	Status   string     `json:"status"`
	Device   string     `json:"device,omitempty"`
	LoadMode string     `json:"load_mode,omitempty"`
	Embed    *ModelInfo `json:"embed,omitempty"`
	Sparse   *ModelInfo `json:"sparse,omitempty"`
	Rerank   *ModelInfo `json:"rerank,omitempty"`
	Query    *ModelInfo `json:"query,omitempty"`
	Error    string     `json:"error,omitempty"`
}

// EventBusCheck represents event bus health information.
type EventBusCheck struct {
	Status        string `json:"status"`
	Type          string `json:"type"`
	Topics        int    `json:"topics,omitempty"`
	PendingEvents int    `json:"pending_events,omitempty"`
	Error         string `json:"error,omitempty"`
}

// CacheCheck represents cache health information.
type CacheCheck struct {
	Status        string  `json:"status"`
	Type          string  `json:"type"`
	EmbedEntries  int64   `json:"embed_entries,omitempty"`
	EmbedHitRate  float64 `json:"embed_hit_rate,omitempty"`
	SparseEntries int64   `json:"sparse_entries,omitempty"`
	SparseHitRate float64 `json:"sparse_hit_rate,omitempty"`
	RerankEntries int64   `json:"rerank_entries,omitempty"`
	RerankHitRate float64 `json:"rerank_hit_rate,omitempty"`
}

// StoresCheck represents stores health information.
type StoresCheck struct {
	Status      string `json:"status"`
	Count       int    `json:"count"`
	TotalChunks int64  `json:"total_chunks,omitempty"`
	Error       string `json:"error,omitempty"`
}

// SystemInfo represents system resource information.
type SystemInfo struct {
	Goroutines int    `json:"goroutines"`
	HeapMB     int64  `json:"heap_mb"`
	AllocMB    int64  `json:"alloc_mb"`
	SysMB      int64  `json:"sys_mb"`
	NumGC      uint32 `json:"num_gc"`
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	NumCPU     int    `json:"num_cpu"`
}

// DetailedHealthChecker provides comprehensive health checking.
type DetailedHealthChecker struct {
	ml        ml.Service
	qdrant    *qdrant.Client
	bus       bus.Bus
	stores    *store.Service
	qdrantURL string
	startTime time.Time
	version   string
	gitCommit string
}

// DetailedHealthCheckerConfig holds configuration for the checker.
type DetailedHealthCheckerConfig struct {
	MLService ml.Service
	Qdrant    *qdrant.Client
	QdrantURL string
	Bus       bus.Bus
	Stores    *store.Service
	Version   string
	GitCommit string
}

// NewDetailedHealthChecker creates a new detailed health checker.
func NewDetailedHealthChecker(cfg DetailedHealthCheckerConfig) *DetailedHealthChecker {
	return &DetailedHealthChecker{
		ml:        cfg.MLService,
		qdrant:    cfg.Qdrant,
		bus:       cfg.Bus,
		stores:    cfg.Stores,
		qdrantURL: cfg.QdrantURL,
		startTime: time.Now(),
		version:   cfg.Version,
		gitCommit: cfg.GitCommit,
	}
}

// CheckDetailed performs a comprehensive health check.
func (h *DetailedHealthChecker) CheckDetailed(ctx context.Context) DetailedHealthStatus {
	status := DetailedHealthStatus{
		Status:        "healthy",
		Version:       h.version,
		GitCommit:     h.gitCommit,
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		Timestamp:     time.Now(),
		Checks:        make(map[string]interface{}),
		System:        h.getSystemInfo(),
	}

	// Check Qdrant
	qdrantCheck := h.checkQdrantDetailed(ctx)
	status.Checks["qdrant"] = qdrantCheck
	if qdrantCheck.Status == "unhealthy" {
		status.Status = "unhealthy"
	} else if qdrantCheck.Status == "degraded" && status.Status == "healthy" {
		status.Status = "degraded"
	}

	// Check ML models
	modelsCheck := h.checkModelsDetailed()
	status.Checks["models"] = modelsCheck
	if modelsCheck.Status == "unhealthy" {
		status.Status = "unhealthy"
	} else if modelsCheck.Status == "degraded" && status.Status == "healthy" {
		status.Status = "degraded"
	}

	// Check event bus
	busCheck := h.checkEventBus()
	status.Checks["event_bus"] = busCheck
	if busCheck.Status == "unhealthy" && status.Status == "healthy" {
		status.Status = "degraded" // Event bus failure is not critical
	}

	// Check cache (from ML service)
	cacheCheck := h.checkCache()
	status.Checks["cache"] = cacheCheck

	// Check stores
	storesCheck := h.checkStores(ctx)
	status.Checks["stores"] = storesCheck
	if storesCheck.Status == "unhealthy" && status.Status == "healthy" {
		status.Status = "degraded"
	}

	return status
}

// checkQdrantDetailed performs detailed Qdrant health check.
func (h *DetailedHealthChecker) checkQdrantDetailed(ctx context.Context) QdrantCheck {
	check := QdrantCheck{
		URL: h.qdrantURL,
	}

	if h.qdrant == nil {
		check.Status = "unhealthy"
		check.Error = "Qdrant client not configured"
		return check
	}

	start := time.Now()
	err := h.qdrant.HealthCheck(ctx)
	check.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		check.Status = "unhealthy"
		check.Error = err.Error()
		return check
	}

	// Try to get collections count
	collections, err := h.qdrant.ListCollections(ctx)
	if err == nil {
		check.Collections = len(collections)
	}

	// Try to get version info
	version, err := h.qdrant.GetVersion(ctx)
	if err == nil {
		check.Version = version
	}

	check.Status = "healthy"
	return check
}

// checkModelsDetailed performs detailed ML models health check.
func (h *DetailedHealthChecker) checkModelsDetailed() ModelsCheck {
	check := ModelsCheck{}

	if h.ml == nil {
		check.Status = "unhealthy"
		check.Error = "ML service not configured"
		return check
	}

	health := h.ml.Health()
	if !health.Healthy {
		check.Status = "unhealthy"
		check.Error = health.Error
		return check
	}

	// Use actual device if different from requested (fallback scenario)
	if health.DeviceFallback {
		check.Device = health.ActualDevice + " (fallback from " + health.Device + ")"
	} else {
		check.Device = health.Device
	}

	// Determine load mode based on what's loaded
	loadedCount := 0
	for _, loaded := range health.ModelsLoaded {
		if loaded {
			loadedCount++
		}
	}
	switch {
	case loadedCount == len(health.ModelsLoaded):
		check.LoadMode = "all"
	case loadedCount > 0:
		check.LoadMode = "partial"
	default:
		check.LoadMode = "none"
	}

	// Get individual model info from ModelsLoaded map
	if loaded, ok := health.ModelsLoaded["embedder"]; ok {
		check.Embed = &ModelInfo{
			Loaded: loaded,
		}
	}

	if loaded, ok := health.ModelsLoaded["sparse"]; ok {
		check.Sparse = &ModelInfo{
			Loaded: loaded,
		}
	}

	if loaded, ok := health.ModelsLoaded["reranker"]; ok {
		check.Rerank = &ModelInfo{
			Loaded: loaded,
		}
	}

	if loaded, ok := health.ModelsLoaded["query"]; ok {
		check.Query = &ModelInfo{
			Loaded: loaded,
		}
	}

	// Determine overall status
	embedOK := check.Embed != nil && check.Embed.Loaded
	sparseOK := check.Sparse != nil && check.Sparse.Loaded

	if embedOK && sparseOK {
		check.Status = "healthy"
	} else if embedOK || sparseOK {
		check.Status = "degraded"
	} else {
		check.Status = "unhealthy"
	}

	return check
}

// checkEventBus performs event bus health check.
func (h *DetailedHealthChecker) checkEventBus() EventBusCheck {
	check := EventBusCheck{
		Type: "unknown",
	}

	if h.bus == nil {
		check.Status = "healthy" // No bus configured is OK (using defaults)
		check.Type = "none"
		return check
	}

	// Try to determine bus type via type assertion
	switch h.bus.(type) {
	case interface{ IsMemoryBus() bool }:
		check.Type = "memory"
	default:
		// Check if it has a Type() method via reflection-like assertion
		if typed, ok := h.bus.(interface{ Type() string }); ok {
			check.Type = typed.Type()
		} else {
			check.Type = "memory" // Default assumption
		}
	}

	// For memory bus, always healthy (no external dependency)
	if check.Type == "memory" {
		check.Status = "healthy"
		return check
	}

	// For other bus types, try to ping
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if pingable, ok := h.bus.(interface{ Ping(context.Context) error }); ok {
		if err := pingable.Ping(ctx); err != nil {
			check.Status = "unhealthy"
			check.Error = err.Error()
			return check
		}
	}

	check.Status = "healthy"
	return check
}

// checkCache performs cache health check.
func (h *DetailedHealthChecker) checkCache() CacheCheck {
	check := CacheCheck{
		Status: "healthy",
		Type:   "memory", // Default - cache stats not exposed in current ML interface
	}

	// Note: Cache stats would need to be added to ml.HealthStatus to expose here
	// For now, we just indicate cache is operational

	return check
}

// checkStores performs stores health check.
func (h *DetailedHealthChecker) checkStores(ctx context.Context) StoresCheck {
	check := StoresCheck{}

	if h.stores == nil {
		check.Status = "healthy" // No stores configured is OK
		check.Count = 0
		return check
	}

	stores, err := h.stores.ListStores(ctx)
	if err != nil {
		check.Status = "degraded"
		check.Error = err.Error()
		return check
	}

	check.Status = "healthy"
	check.Count = len(stores)

	// Sum up total chunks if we have Qdrant access
	if h.qdrant != nil {
		var totalChunks int64
		for _, s := range stores {
			if count, err := h.qdrant.CountPoints(ctx, s.Name, nil); err == nil {
				totalChunks += int64(count)
			}
		}
		check.TotalChunks = totalChunks
	}

	return check
}

// getSystemInfo returns current system resource information.
func (h *DetailedHealthChecker) getSystemInfo() *SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &SystemInfo{
		Goroutines: runtime.NumGoroutine(),
		HeapMB:     int64(m.HeapAlloc / 1024 / 1024),
		AllocMB:    int64(m.Alloc / 1024 / 1024),
		SysMB:      int64(m.Sys / 1024 / 1024),
		NumGC:      m.NumGC,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		NumCPU:     runtime.NumCPU(),
	}
}
