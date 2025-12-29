// Package web provides the web UI using templ templates and HTMX.
package web

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/connection"
	"github.com/ricesearch/rice-search/internal/metrics"
	"github.com/ricesearch/rice-search/internal/models"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/settings"
)

// GRPCClient interface defines the gRPC methods needed by the web handlers.
type GRPCClient interface {
	Search(ctx context.Context, req *pb.SearchRequest) (*pb.SearchResponse, error)
	ListStores(ctx context.Context, req *pb.ListStoresRequest) (*pb.ListStoresResponse, error)
	CreateStore(ctx context.Context, req *pb.CreateStoreRequest) (*pb.Store, error)
	GetStore(ctx context.Context, req *pb.GetStoreRequest) (*pb.Store, error)
	DeleteStore(ctx context.Context, req *pb.DeleteStoreRequest) (*pb.DeleteStoreResponse, error)
	GetStoreStats(ctx context.Context, req *pb.GetStoreStatsRequest) (*pb.StoreStats, error)
	Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error)
	Version(ctx context.Context, req *pb.VersionRequest) (*pb.VersionResponse, error)
}

// Handler handles all web UI requests.
type Handler struct {
	grpc        GRPCClient
	log         *logger.Logger
	cfg         *config.Config
	modelReg    *models.Registry
	mapperSvc   *models.MapperService
	connSvc     *connection.Service
	settingsSvc *settings.Service
	metrics     *metrics.Metrics
	startTime   time.Time
	qdrantURL   string
}

// NewHandler creates a new web handler.
func NewHandler(
	grpc GRPCClient,
	log *logger.Logger,
	cfg *config.Config,
	modelReg *models.Registry,
	mapperSvc *models.MapperService,
	connSvc *connection.Service,
	settingsSvc *settings.Service,
	metricsInstance *metrics.Metrics,
	qdrantURL string,
) *Handler {
	return &Handler{
		grpc:        grpc,
		log:         log,
		cfg:         cfg,
		modelReg:    modelReg,
		mapperSvc:   mapperSvc,
		connSvc:     connSvc,
		settingsSvc: settingsSvc,
		metrics:     metricsInstance,
		startTime:   time.Now(),
		qdrantURL:   qdrantURL,
	}
}

// RegisterRoutes registers all web routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Main Pages
	mux.HandleFunc("GET /", h.handleDashboard)
	mux.HandleFunc("GET /search", h.handleSearchPage)
	mux.HandleFunc("GET /stores", h.handleStoresPage)
	mux.HandleFunc("GET /stores/{name}", h.handleStoreDetail)
	mux.HandleFunc("GET /stores/{name}/files", h.handleFilesPage)
	mux.HandleFunc("GET /files", h.handleFilesPage)
	mux.HandleFunc("GET /stats", h.handleStatsPage)

	// Admin Pages
	mux.HandleFunc("GET /admin", h.handleAdminPage)
	mux.HandleFunc("GET /admin/models", h.handleModelsPage)
	mux.HandleFunc("GET /admin/mappers", h.handleMappersPage)
	mux.HandleFunc("GET /admin/connections", h.handleConnectionsPage)
	mux.HandleFunc("GET /admin/settings", h.handleSettingsPage)

	// Search API (HTMX)
	mux.HandleFunc("POST /search", h.handleSearch)

	// Store API (HTMX)
	mux.HandleFunc("POST /admin/stores", h.handleCreateStore)
	mux.HandleFunc("DELETE /admin/stores/{name}", h.handleDeleteStore)

	// Models API (HTMX)
	mux.HandleFunc("POST /admin/models/{id}/download", h.handleDownloadModel)
	mux.HandleFunc("POST /admin/models/{id}/default", h.handleSetDefaultModel)
	mux.HandleFunc("POST /admin/models/{id}/gpu", h.handleToggleModelGPU)
	mux.HandleFunc("DELETE /admin/models/{id}", h.handleDeleteModel)

	// Mappers API (HTMX)
	mux.HandleFunc("POST /admin/mappers", h.handleCreateMapper)
	mux.HandleFunc("PUT /admin/mappers/{id}", h.handleUpdateMapper)
	mux.HandleFunc("DELETE /admin/mappers/{id}", h.handleDeleteMapper)
	mux.HandleFunc("GET /admin/mappers/{id}/yaml", h.handleMapperYAML)
	mux.HandleFunc("POST /admin/mappers/generate", h.handleGenerateMapper)

	// Connections API (HTMX)
	mux.HandleFunc("POST /admin/connections/{id}/enable", h.handleEnableConnection)
	mux.HandleFunc("POST /admin/connections/{id}/disable", h.handleDisableConnection)
	mux.HandleFunc("DELETE /admin/connections/{id}", h.handleDeleteConnection)

	// Settings API (HTMX)
	mux.HandleFunc("POST /admin/settings", h.handleSaveSettings)

	// Stats API (HTMX)
	mux.HandleFunc("GET /stats/refresh", h.handleStatsRefresh)

	// Metrics endpoint
	if h.metrics != nil {
		mux.Handle("GET /metrics", h.metrics.Handler())
	}
}

// =============================================================================
// Layout Data Helper
// =============================================================================

func (h *Handler) getLayoutData(ctx context.Context, title, currentPath string) LayoutData {
	healthStatus := "healthy"
	version := "unknown"

	// Get health status
	healthResp, err := h.grpc.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		healthStatus = "unhealthy"
	} else if healthResp != nil {
		switch healthResp.Status {
		case pb.HealthStatus_HEALTH_STATUS_DEGRADED:
			healthStatus = "degraded"
		case pb.HealthStatus_HEALTH_STATUS_UNHEALTHY:
			healthStatus = "unhealthy"
		}
	}

	// Get version
	verResp, err := h.grpc.Version(ctx, &pb.VersionRequest{})
	if err == nil && verResp != nil {
		version = verResp.Version
	}

	return LayoutData{
		Title:        title,
		CurrentPath:  currentPath,
		HealthStatus: healthStatus,
		Version:      version,
		QdrantURL:    h.qdrantURL,
	}
}

// =============================================================================
// Dashboard
// =============================================================================

func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := DashboardData{
		Layout: h.getLayoutData(ctx, "Dashboard", "/"),
	}

	// Health summary
	healthResp, err := h.grpc.Health(ctx, &pb.HealthRequest{})
	if err == nil && healthResp != nil {
		data.Health.Overall = "healthy"
		for name, comp := range healthResp.Components {
			status := HealthStatus{
				Status:  "healthy",
				Message: comp.Message,
			}
			if comp.Status == pb.HealthStatus_HEALTH_STATUS_DEGRADED {
				status.Status = "degraded"
				data.Health.Overall = "degraded"
			} else if comp.Status == pb.HealthStatus_HEALTH_STATUS_UNHEALTHY {
				status.Status = "unhealthy"
				data.Health.Overall = "unhealthy"
			}
			if comp.Latency != nil {
				status.Latency = comp.Latency.AsDuration().Milliseconds()
			}
			switch name {
			case "ml", "onnx":
				data.Health.ML = status
			case "qdrant":
				data.Health.Qdrant = status
			default:
				data.Health.System = status
			}
		}
	} else {
		data.Health.Overall = "unhealthy"
		data.Health.System = HealthStatus{Status: "unhealthy", Message: "Cannot connect to server"}
	}

	// Quick stats from stores
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err == nil && storesResp != nil {
		data.QuickStats.TotalStores = len(storesResp.Stores)
		for _, s := range storesResp.Stores {
			if s.Stats != nil {
				data.QuickStats.TotalFiles += s.Stats.DocumentCount
				data.QuickStats.TotalChunks += s.Stats.ChunkCount
			}
		}
	}

	// Active connections
	if h.connSvc != nil {
		conns, _ := h.connSvc.ListAllConnections(ctx)
		activeCount := 0
		for _, c := range conns {
			if c.IsActive {
				activeCount++
			}
		}
		data.QuickStats.ActiveConnections = activeCount
	}

	// System info
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	data.System.MemoryUsedMB = int64(memStats.Alloc / 1024 / 1024)
	data.System.MemoryTotalMB = int64(memStats.Sys / 1024 / 1024)
	data.System.Goroutines = runtime.NumGoroutine()
	data.System.Uptime = formatDuration(time.Since(h.startTime))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := DashboardPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render dashboard", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// Search Page
// =============================================================================

func (h *Handler) handleSearchPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get list of stores
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	var stores []string
	if err == nil {
		for _, s := range storesResp.Stores {
			stores = append(stores, s.Name)
		}
	}
	if len(stores) == 0 {
		stores = []string{"default"}
	}

	// Get connections for filter
	var connections []ConnectionOption
	if h.connSvc != nil {
		conns, _ := h.connSvc.ListAllConnections(ctx)
		for _, c := range conns {
			connections = append(connections, ConnectionOption{
				ID:   c.ID,
				Name: c.Name,
			})
		}
	}

	data := SearchPageData{
		Layout:      h.getLayoutData(ctx, "Search", "/search"),
		Store:       r.URL.Query().Get("store"),
		Stores:      stores,
		Connections: connections,
		Options: SearchOptions{
			TopK:            20,
			EnableReranking: true,
			RerankTopK:      50,
			SparseWeight:    0.5,
			DenseWeight:     0.5,
			EnableDedup:     true,
			DedupThreshold:  0.85,
			EnableDiversity: true,
			DiversityLambda: 0.7,
			IncludeContent:  true,
		},
	}

	if data.Store == "" {
		data.Store = "default"
	}

	// Check for query parameter (from dashboard quick search)
	if query := r.URL.Query().Get("query"); query != "" {
		data.Query = query
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SearchPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render search page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderSearchResults(w, r, SearchPageData{Error: "Invalid form data"})
		return
	}

	query := strings.TrimSpace(r.FormValue("query"))
	store := r.FormValue("store")
	if store == "" {
		store = "default"
	}

	// Parse all options
	topK := parseIntOr(r.FormValue("top_k"), 20)
	enableRerank := r.FormValue("rerank") == "on" || r.FormValue("rerank") == "true"
	includeContent := r.FormValue("content") == "on" || r.FormValue("content") == "true"

	// Get stores for the dropdown
	storesResp, _ := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	var stores []string
	if storesResp != nil {
		for _, s := range storesResp.Stores {
			stores = append(stores, s.Name)
		}
	}
	if len(stores) == 0 {
		stores = []string{"default"}
	}

	// If no query, just render empty results
	if query == "" {
		h.renderSearchResults(w, r, SearchPageData{
			Query:  "",
			Store:  store,
			Stores: stores,
		})
		return
	}

	// Execute search
	searchReq := &pb.SearchRequest{
		Query:           query,
		Store:           store,
		TopK:            int32(topK),
		EnableReranking: &enableRerank,
		IncludeContent:  includeContent,
	}

	resp, err := h.grpc.Search(ctx, searchReq)
	if err != nil {
		h.renderSearchResults(w, r, SearchPageData{
			Query:  query,
			Store:  store,
			Stores: stores,
			Error:  err.Error(),
		})
		return
	}

	// Convert results
	results := make([]SearchResult, len(resp.Results))
	for i, res := range resp.Results {
		sr := SearchResult{
			ID:        res.ID,
			Path:      res.Path,
			Language:  res.Language,
			StartLine: int(res.StartLine),
			EndLine:   int(res.EndLine),
			Content:   res.Content,
			Score:     res.Score,
		}
		if res.RerankScore != nil {
			sr.RerankScore = *res.RerankScore
		}
		results[i] = sr
	}

	var searchTime int64
	if resp.Metadata != nil {
		searchTime = resp.Metadata.SearchTimeMs
	}

	h.renderSearchResults(w, r, SearchPageData{
		Query:      query,
		Store:      store,
		Stores:     stores,
		Results:    results,
		Total:      int(resp.Total),
		SearchTime: searchTime,
	})
}

func (h *Handler) renderSearchResults(w http.ResponseWriter, r *http.Request, data SearchPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SearchResults(data).Render(r.Context(), w); err != nil {
		h.log.Error("Failed to render search results", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// Stores Pages
// =============================================================================

func (h *Handler) handleStoresPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := StoresPageData{
		Layout: h.getLayoutData(ctx, "Stores", "/stores"),
	}

	// Get all stores with connections
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err != nil {
		data.Error = err.Error()
	} else {
		for _, s := range storesResp.Stores {
			store := StoreWithConnections{
				Store: StoreInfo{
					Name:        s.Name,
					DisplayName: s.DisplayName,
					Description: s.Description,
				},
			}
			if s.Stats != nil {
				store.FileCount = s.Stats.DocumentCount
				store.ChunkCount = s.Stats.ChunkCount
				store.Store.TotalSize = s.Stats.TotalSize
			}
			if s.CreatedAt != nil {
				store.Store.CreatedAt = s.CreatedAt.AsTime()
			}

			// Get connections for this store
			if h.connSvc != nil {
				conns, _ := h.connSvc.GetConnectionsForStore(ctx, s.Name)
				for _, c := range conns {
					store.Connections = append(store.Connections, ConnectionSummary{
						ID:       c.ID,
						Name:     c.Name,
						LastSeen: c.LastSeenAt,
					})
				}
			}

			data.Stores = append(data.Stores, store)
		}
		data.TotalStores = len(data.Stores)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StoresPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stores page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleStoreDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")

	// For now, redirect to stores page (detail page can be implemented later)
	http.Redirect(w, r, "/stores", http.StatusSeeOther)
	_ = ctx
	_ = name
}

// =============================================================================
// Files Page
// =============================================================================

func (h *Handler) handleFilesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	store := r.PathValue("name")
	if store == "" {
		store = r.URL.Query().Get("store")
	}

	data := FilesPageData{
		Layout:   h.getLayoutData(ctx, "Files", "/files"),
		Store:    store,
		Page:     parseIntOr(r.URL.Query().Get("page"), 1),
		PageSize: parseIntOr(r.URL.Query().Get("page_size"), 50),
		Filters: FileFilters{
			Store:      store,
			PathPrefix: r.URL.Query().Get("path"),
			Language:   r.URL.Query().Get("language"),
			SortBy:     r.URL.Query().Get("sort_by"),
			SortOrder:  r.URL.Query().Get("sort_order"),
		},
	}

	// TODO: Implement file listing from store
	// For now, just render empty
	data.TotalPages = 1

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := FilesPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render files page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// Admin Page (Legacy - redirects to stores)
// =============================================================================

func (h *Handler) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/stores", http.StatusSeeOther)
}

// =============================================================================
// Models Page
// =============================================================================

func (h *Handler) handleModelsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := ModelsPageData{
		Layout: h.getLayoutData(ctx, "Model Management", "/admin/models"),
	}

	if h.modelReg != nil {
		// Get all models
		allModels := h.modelReg.ListAllModels()
		for _, m := range allModels {
			data.Models = append(data.Models, ModelInfoDisplay{
				ID:          m.ID,
				Type:        string(m.Type),
				DisplayName: m.DisplayName,
				Description: m.Description,
				OutputDim:   m.OutputDim,
				MaxTokens:   m.MaxTokens,
				Downloaded:  m.Downloaded,
				IsDefault:   m.IsDefault,
				GPUEnabled:  m.GPUEnabled,
				Status:      getModelStatus(m),
			})
		}

		// Get type configs
		for _, t := range []models.ModelType{models.ModelTypeEmbed, models.ModelTypeRerank, models.ModelTypeQueryUnderstand} {
			cfg := h.modelReg.GetTypeConfig(t)
			if cfg != nil {
				displayName, description := getModelTypeInfo(t)
				data.TypeConfigs = append(data.TypeConfigs, ModelTypeConfigDisplay{
					Type:         string(cfg.Type),
					DisplayName:  displayName,
					DefaultModel: cfg.DefaultModel,
					GPUEnabled:   cfg.GPUEnabled,
					Description:  description,
				})
			}
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminModelsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render models page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func getModelStatus(m *models.ModelInfo) string {
	if m.Downloaded {
		return "ready"
	}
	return "not_downloaded"
}

// getModelTypeInfo returns display name and description for a model type
func getModelTypeInfo(t models.ModelType) (displayName, description string) {
	switch t {
	case models.ModelTypeEmbed:
		return "Embeddings", "Dense vector embeddings for semantic search"
	case models.ModelTypeRerank:
		return "Reranking", "Neural reranking for result quality"
	case models.ModelTypeQueryUnderstand:
		return "Query Understanding", "Intent classification and query expansion"
	default:
		return string(t), ""
	}
}

func (h *Handler) handleDownloadModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelID := r.PathValue("id")

	if modelID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.modelReg == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model registry not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Start download (returns progress channel)
	progressChan, err := h.modelReg.DownloadModel(ctx, modelID)
	if err != nil {
		h.log.Error("Failed to start model download", "model", modelID, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to download model: %s", err.Error())).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Monitor progress in background (simplified - real implementation would use SSE or WebSockets)
	go func() {
		for progress := range progressChan {
			if progress.Error != "" {
				h.log.Error("Model download failed", "model", modelID, "error", progress.Error)
			} else if progress.Complete {
				h.log.Info("Model download complete", "model", modelID)
			}
		}
	}()

	// Return success message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Download started for model: %s", modelID)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleSetDefaultModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelID := r.PathValue("id")

	if modelID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.modelReg == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model registry not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Get model to determine its type
	model, err := h.modelReg.GetModel(ctx, modelID)
	if err != nil {
		h.log.Error("Failed to get model", "model", modelID, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Model not found: %s", modelID)).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Set as default for its type
	err = h.modelReg.SetDefaultModel(ctx, model.Type, modelID)
	if err != nil {
		h.log.Error("Failed to set default model", "model", modelID, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to set default: %s", err.Error())).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Return success message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Set %s as default for %s", model.DisplayName, model.Type)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleToggleModelGPU(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelID := r.PathValue("id")

	if modelID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.modelReg == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model registry not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Parse form to get enabled status (HTMX sends this in hx-vals)
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Invalid form data").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	enabled := r.FormValue("enabled") == "true"

	// Toggle GPU
	err := h.modelReg.ToggleGPU(ctx, modelID, enabled)
	if err != nil {
		h.log.Error("Failed to toggle GPU", "model", modelID, "enabled", enabled, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to toggle GPU: %s", err.Error())).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Return success message
	status := "disabled"
	if enabled {
		status = "enabled"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("GPU %s for model: %s", status, modelID)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	modelID := r.PathValue("id")

	if modelID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.modelReg == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Model registry not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Delete model
	err := h.modelReg.DeleteModel(ctx, modelID)
	if err != nil {
		h.log.Error("Failed to delete model", "model", modelID, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to delete model: %s", err.Error())).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Return success message
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Deleted model: %s", modelID)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

// =============================================================================
// Mappers Page
// =============================================================================

func (h *Handler) handleMappersPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := MappersPageData{
		Layout: h.getLayoutData(ctx, "Model Mappers", "/admin/mappers"),
	}

	if h.mapperSvc != nil {
		mappers, _ := h.mapperSvc.ListMappers(ctx)
		for _, m := range mappers {
			data.Mappers = append(data.Mappers, ModelMapperDisplay{
				ID:             m.ID,
				Name:           m.Name,
				ModelID:        m.ModelID,
				Type:           string(m.Type),
				InputMapping:   m.InputMapping,
				OutputMapping:  m.OutputMapping,
				PromptTemplate: m.PromptTemplate,
			})
		}
	}

	// Get models for the editor
	if h.modelReg != nil {
		for _, m := range h.modelReg.ListAllModels() {
			data.Models = append(data.Models, ModelInfoDisplay{
				ID:          m.ID,
				Type:        string(m.Type),
				DisplayName: m.DisplayName,
			})
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminMappersPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render mappers page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleCreateMapper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.mapperSvc == nil {
		h.renderMappersListWithError(w, r, "Mapper service not available")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderMappersListWithError(w, r, "Invalid form data")
		return
	}

	// Parse basic fields
	name := strings.TrimSpace(r.FormValue("name"))
	modelID := strings.TrimSpace(r.FormValue("model_id"))
	mapperType := strings.TrimSpace(r.FormValue("type"))
	promptTemplate := strings.TrimSpace(r.FormValue("prompt_template"))

	if name == "" || modelID == "" || mapperType == "" {
		h.renderMappersListWithError(w, r, "Name, model ID, and type are required")
		return
	}

	// Parse input mappings
	inputMapping := make(map[string]string)
	inputKeys := r.Form["input_mapping_key[]"]
	inputValues := r.Form["input_mapping_value[]"]
	for i := 0; i < len(inputKeys) && i < len(inputValues); i++ {
		key := strings.TrimSpace(inputKeys[i])
		value := strings.TrimSpace(inputValues[i])
		if key != "" && value != "" {
			inputMapping[key] = value
		}
	}

	// Parse output mappings
	outputMapping := make(map[string]string)
	outputKeys := r.Form["output_mapping_key[]"]
	outputValues := r.Form["output_mapping_value[]"]
	for i := 0; i < len(outputKeys) && i < len(outputValues); i++ {
		key := strings.TrimSpace(outputKeys[i])
		value := strings.TrimSpace(outputValues[i])
		if key != "" && value != "" {
			outputMapping[key] = value
		}
	}

	// Validate mappings exist
	if len(inputMapping) == 0 || len(outputMapping) == 0 {
		h.renderMappersListWithError(w, r, "Input and output mappings are required")
		return
	}

	// Generate mapper ID from name
	mapperID := strings.ToLower(strings.ReplaceAll(name, " ", "-"))

	// Create mapper
	mapper := &models.ModelMapper{
		ID:             mapperID,
		Name:           name,
		ModelID:        modelID,
		Type:           models.ModelType(mapperType),
		PromptTemplate: promptTemplate,
		InputMapping:   inputMapping,
		OutputMapping:  outputMapping,
	}

	if err := h.mapperSvc.CreateMapper(ctx, mapper); err != nil {
		h.renderMappersListWithError(w, r, err.Error())
		return
	}

	h.refreshMappersList(w, r)
}

func (h *Handler) handleUpdateMapper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.mapperSvc == nil {
		h.renderMappersListWithError(w, r, "Mapper service not available")
		return
	}

	mapperID := r.PathValue("id")
	if mapperID == "" {
		h.renderMappersListWithError(w, r, "Mapper ID is required")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.renderMappersListWithError(w, r, "Invalid form data")
		return
	}

	// Parse basic fields
	name := strings.TrimSpace(r.FormValue("name"))
	modelID := strings.TrimSpace(r.FormValue("model_id"))
	mapperType := strings.TrimSpace(r.FormValue("type"))
	promptTemplate := strings.TrimSpace(r.FormValue("prompt_template"))

	if name == "" || modelID == "" || mapperType == "" {
		h.renderMappersListWithError(w, r, "Name, model ID, and type are required")
		return
	}

	// Parse input mappings
	inputMapping := make(map[string]string)
	inputKeys := r.Form["input_mapping_key[]"]
	inputValues := r.Form["input_mapping_value[]"]
	for i := 0; i < len(inputKeys) && i < len(inputValues); i++ {
		key := strings.TrimSpace(inputKeys[i])
		value := strings.TrimSpace(inputValues[i])
		if key != "" && value != "" {
			inputMapping[key] = value
		}
	}

	// Parse output mappings
	outputMapping := make(map[string]string)
	outputKeys := r.Form["output_mapping_key[]"]
	outputValues := r.Form["output_mapping_value[]"]
	for i := 0; i < len(outputKeys) && i < len(outputValues); i++ {
		key := strings.TrimSpace(outputKeys[i])
		value := strings.TrimSpace(outputValues[i])
		if key != "" && value != "" {
			outputMapping[key] = value
		}
	}

	// Validate mappings exist
	if len(inputMapping) == 0 || len(outputMapping) == 0 {
		h.renderMappersListWithError(w, r, "Input and output mappings are required")
		return
	}

	// Update mapper
	mapper := &models.ModelMapper{
		ID:             mapperID,
		Name:           name,
		ModelID:        modelID,
		Type:           models.ModelType(mapperType),
		PromptTemplate: promptTemplate,
		InputMapping:   inputMapping,
		OutputMapping:  outputMapping,
	}

	if err := h.mapperSvc.UpdateMapper(ctx, mapper); err != nil {
		h.renderMappersListWithError(w, r, err.Error())
		return
	}

	h.refreshMappersList(w, r)
}

func (h *Handler) handleDeleteMapper(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.mapperSvc == nil {
		h.renderMappersListWithError(w, r, "Mapper service not available")
		return
	}

	mapperID := r.PathValue("id")
	if mapperID == "" {
		h.renderMappersListWithError(w, r, "Mapper ID is required")
		return
	}

	if err := h.mapperSvc.DeleteMapper(ctx, mapperID); err != nil {
		h.renderMappersListWithError(w, r, err.Error())
		return
	}

	h.refreshMappersList(w, r)
}

func (h *Handler) handleMapperYAML(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.mapperSvc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Mapper service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	mapperID := r.PathValue("id")
	if mapperID == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Mapper ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Get mapper
	mapper, err := h.mapperSvc.GetMapper(ctx, mapperID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(err.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Convert to YAML
	yamlData, err := yaml.Marshal(mapper)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Failed to generate YAML").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Render YAML content
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := YAMLContent(string(yamlData)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render YAML content", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleGenerateMapper(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement generate mapper
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// refreshMappersList refreshes the mappers list after an operation.
func (h *Handler) refreshMappersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var mappers []ModelMapperDisplay
	if h.mapperSvc != nil {
		mappersList, _ := h.mapperSvc.ListMappers(ctx)
		for _, m := range mappersList {
			mappers = append(mappers, ModelMapperDisplay{
				ID:             m.ID,
				Name:           m.Name,
				ModelID:        m.ModelID,
				Type:           string(m.Type),
				InputMapping:   m.InputMapping,
				OutputMapping:  m.OutputMapping,
				PromptTemplate: m.PromptTemplate,
			})
		}
	}

	h.renderMappersList(w, r, mappers, "")
}

// renderMappersListWithError renders the mappers list with an error message.
func (h *Handler) renderMappersListWithError(w http.ResponseWriter, r *http.Request, errMsg string) {
	ctx := r.Context()

	var mappers []ModelMapperDisplay
	if h.mapperSvc != nil {
		mappersList, _ := h.mapperSvc.ListMappers(ctx)
		for _, m := range mappersList {
			mappers = append(mappers, ModelMapperDisplay{
				ID:             m.ID,
				Name:           m.Name,
				ModelID:        m.ModelID,
				Type:           string(m.Type),
				InputMapping:   m.InputMapping,
				OutputMapping:  m.OutputMapping,
				PromptTemplate: m.PromptTemplate,
			})
		}
	}

	h.renderMappersList(w, r, mappers, errMsg)
}

// renderMappersList renders the mappers list component.
func (h *Handler) renderMappersList(w http.ResponseWriter, r *http.Request, mappers []ModelMapperDisplay, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if errMsg != "" {
		if err := ErrorMessage(errMsg).Render(r.Context(), w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
	}

	if err := MappersList(mappers).Render(r.Context(), w); err != nil {
		h.log.Error("Failed to render mappers list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// Connections Page
// =============================================================================

func (h *Handler) handleConnectionsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := ConnectionsPageData{
		Layout: h.getLayoutData(ctx, "Connections", "/admin/connections"),
	}

	if h.connSvc != nil {
		conns, err := h.connSvc.ListAllConnections(ctx)
		if err != nil {
			data.Error = err.Error()
		} else {
			for _, c := range conns {
				conn := ConnectionDisplay{
					ID:        c.ID,
					Name:      c.Name,
					IsActive:  c.IsActive,
					CreatedAt: c.CreatedAt,
					LastSeen:  c.LastSeenAt,
				}
				if c.PCInfo.Hostname != "" {
					conn.Hostname = c.PCInfo.Hostname
					conn.OS = c.PCInfo.OS
					conn.Arch = c.PCInfo.Arch
				}
				data.Connections = append(data.Connections, conn)
				if c.IsActive {
					data.ActiveCount++
				}
			}
			data.Total = len(conns)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminConnectionsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render connections page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleEnableConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.connSvc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := h.connSvc.SetActive(ctx, id, true); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to enable connection: %v", err)).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage("Connection enabled successfully").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleDisableConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.connSvc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := h.connSvc.SetActive(ctx, id, false); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to disable connection: %v", err)).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage("Connection disabled successfully").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleDeleteConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.connSvc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := h.connSvc.DeleteConnection(ctx, id); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to delete connection: %v", err)).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage("Connection deleted successfully").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

// handleRenameConnection handles renaming a connection (if route is added in the future).
func (h *Handler) handleRenameConnection(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	if id == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection ID is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if h.connSvc == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Connection service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Parse form to get new name
	if err := r.ParseForm(); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("Invalid form data").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	newName := strings.TrimSpace(r.FormValue("name"))
	if newName == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage("New name is required").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := h.connSvc.RenameConnection(ctx, id, newName); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := ErrorMessage(fmt.Sprintf("Failed to rename connection: %v", err)).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Connection renamed to '%s' successfully", newName)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

// =============================================================================
// Settings Page
// =============================================================================

func (h *Handler) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := SettingsPageData{
		Layout: h.getLayoutData(ctx, "Settings", "/admin/settings"),
	}

	if h.cfg != nil {
		data.Config = RuntimeConfig{
			ServerHost:        h.cfg.Host,
			ServerPort:        h.cfg.Port,
			LogLevel:          h.cfg.Log.Level,
			LogFormat:         h.cfg.Log.Format,
			QdrantURL:         h.cfg.Qdrant.URL,
			QdrantCollection:  h.cfg.Qdrant.CollectionPrefix,
			QdrantTimeout:     30000, // 30 seconds default
			DefaultTopK:       20,
			DefaultRerank:     true,
			DefaultDedup:      true,
			DefaultDiversity:  true,
			SparseWeight:      0.5,
			DenseWeight:       0.5,
			DedupThreshold:    0.85,
			DiversityLambda:   0.7,
			RerankCandidates:  50,
			MaxChunksPerFile:  5,
			ConnectionEnabled: h.cfg.Connection.Enabled,
			MaxInactiveHours:  h.cfg.Connection.MaxInactive * 24, // days to hours
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminSettingsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render settings page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := r.ParseForm(); err != nil {
		if err := ErrorMessage("Failed to parse form: "+err.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error message", "error", err)
		}
		return
	}

	// Build RuntimeConfig from form values
	cfg := settings.RuntimeConfig{
		// Server
		ServerHost: r.FormValue("server_host"),
		ServerPort: parseIntOr(r.FormValue("server_port"), 8080),
		LogLevel:   r.FormValue("log_level"),
		LogFormat:  r.FormValue("log_format"),

		// ML
		EmbedModel:    r.FormValue("embed_model"),
		RerankModel:   r.FormValue("rerank_model"),
		QueryModel:    r.FormValue("query_model"),
		EmbedGPU:      r.FormValue("embed_gpu") == "on" || r.FormValue("embed_gpu") == "true",
		RerankGPU:     r.FormValue("rerank_gpu") == "on" || r.FormValue("rerank_gpu") == "true",
		QueryGPU:      r.FormValue("query_gpu") == "on" || r.FormValue("query_gpu") == "true",
		QueryEnabled:  r.FormValue("query_enabled") == "on" || r.FormValue("query_enabled") == "true",
		BatchSize:     parseIntOr(r.FormValue("batch_size"), 32),
		MaxConcurrent: parseIntOr(r.FormValue("max_concurrent"), 4),

		// Qdrant
		QdrantURL:        r.FormValue("qdrant_url"),
		QdrantCollection: r.FormValue("qdrant_collection"),
		QdrantTimeout:    parseIntOr(r.FormValue("qdrant_timeout"), 30000),

		// Search
		DefaultTopK:      parseIntOr(r.FormValue("default_top_k"), 20),
		DefaultRerank:    r.FormValue("default_rerank") == "on" || r.FormValue("default_rerank") == "true",
		DefaultDedup:     r.FormValue("default_dedup") == "on" || r.FormValue("default_dedup") == "true",
		DefaultDiversity: r.FormValue("default_diversity") == "on" || r.FormValue("default_diversity") == "true",
		DedupThreshold:   parseFloatOr(r.FormValue("dedup_threshold"), 0.85),
		DiversityLambda:  parseFloatOr(r.FormValue("diversity_lambda"), 0.7),
		RerankCandidates: parseIntOr(r.FormValue("rerank_candidates"), 50),
		MaxChunksPerFile: parseIntOr(r.FormValue("max_chunks_per_file"), 5),
		SparseWeight:     parseFloatOr(r.FormValue("sparse_weight"), 0.5),
		DenseWeight:      parseFloatOr(r.FormValue("dense_weight"), 0.5),

		// Index
		ChunkSize:       parseIntOr(r.FormValue("chunk_size"), 512),
		ChunkOverlap:    parseIntOr(r.FormValue("chunk_overlap"), 128),
		MaxFileSize:     int64(parseIntOr(r.FormValue("max_file_size"), 10)) * 1024 * 1024, // MB to bytes
		ExcludePatterns: r.FormValue("exclude_patterns"),
		SupportedLangs:  r.FormValue("supported_langs"),

		// Connection
		ConnectionEnabled: r.FormValue("connection_enabled") == "on" || r.FormValue("connection_enabled") == "true",
		MaxInactiveHours:  parseIntOr(r.FormValue("max_inactive_hours"), 168),
	}

	// Save settings
	if h.settingsSvc != nil {
		if err := h.settingsSvc.Update(ctx, cfg, "admin"); err != nil {
			if err := ErrorMessage("Failed to save settings: "+err.Error()).Render(ctx, w); err != nil {
				h.log.Error("Failed to render error message", "error", err)
			}
			return
		}
	}

	if err := SuccessMessage("Settings saved successfully").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

// parseIntOr parses an integer or returns a default value.
func parseIntOr(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// parseFloatOr parses a float32 or returns a default value.
func parseFloatOr(s string, defaultVal float32) float32 {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.ParseFloat(s, 32)
	if err != nil {
		return defaultVal
	}
	return float32(v)
}

// =============================================================================
// Stats Page
// =============================================================================

func (h *Handler) handleStatsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := h.getStatsData(ctx)
	data.Layout = h.getLayoutData(ctx, "Stats", "/stats")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleStatsRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := h.getStatsData(ctx)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsContent(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats content", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) getStatsData(ctx context.Context) StatsPageData {
	data := StatsPageData{
		Components: make(map[string]HealthStatus),
	}

	// Get version
	verResp, err := h.grpc.Version(ctx, &pb.VersionRequest{})
	if err == nil {
		data.Version = verResp.Version
		data.GoVersion = verResp.GoVersion
		data.Commit = verResp.Commit
		data.BuildDate = verResp.BuildDate
	} else {
		data.Version = "unknown"
		data.GoVersion = runtime.Version()
	}

	// Get health
	healthResp, err := h.grpc.Health(ctx, &pb.HealthRequest{})
	if err == nil {
		for name, comp := range healthResp.Components {
			status := "healthy"
			switch comp.Status {
			case pb.HealthStatus_HEALTH_STATUS_DEGRADED:
				status = "degraded"
			case pb.HealthStatus_HEALTH_STATUS_UNHEALTHY:
				status = "unhealthy"
			}
			var latency int64
			if comp.Latency != nil {
				latency = comp.Latency.AsDuration().Milliseconds()
			}
			data.Components[name] = HealthStatus{
				Status:  status,
				Message: comp.Message,
				Latency: latency,
			}
		}
	} else {
		data.Components["server"] = HealthStatus{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	}

	// Get stores stats
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err == nil {
		data.TotalStores = len(storesResp.Stores)
		for _, s := range storesResp.Stores {
			if s.Stats != nil {
				data.TotalFiles += s.Stats.DocumentCount
				data.TotalChunks += s.Stats.ChunkCount
			}
		}
	}

	// Get connections count
	if h.connSvc != nil {
		conns, _ := h.connSvc.ListAllConnections(ctx)
		data.TotalConnections = len(conns)
	}

	return data
}

// =============================================================================
// Store CRUD Handlers
// =============================================================================

func (h *Handler) handleCreateStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderStoresGrid(w, r, nil, "Invalid form data")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		h.renderStoresGrid(w, r, nil, "Name is required")
		return
	}

	// Create store
	_, err := h.grpc.CreateStore(ctx, &pb.CreateStoreRequest{
		Name:        name,
		DisplayName: displayName,
		Description: description,
	})
	if err != nil {
		h.renderStoresGrid(w, r, nil, err.Error())
		return
	}

	// Refresh store list
	h.refreshStoresGrid(w, r)
}

func (h *Handler) handleDeleteStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		h.renderStoresGrid(w, r, nil, "Store name is required")
		return
	}

	if name == "default" {
		h.renderStoresGrid(w, r, nil, "Cannot delete default store")
		return
	}

	_, err := h.grpc.DeleteStore(ctx, &pb.DeleteStoreRequest{Name: name})
	if err != nil {
		h.renderStoresGrid(w, r, nil, err.Error())
		return
	}

	h.refreshStoresGrid(w, r)
}

func (h *Handler) refreshStoresGrid(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err != nil {
		h.renderStoresGrid(w, r, nil, err.Error())
		return
	}

	var stores []StoreWithConnections
	for _, s := range storesResp.Stores {
		store := StoreWithConnections{
			Store: StoreInfo{
				Name:        s.Name,
				DisplayName: s.DisplayName,
				Description: s.Description,
			},
		}
		if s.Stats != nil {
			store.FileCount = s.Stats.DocumentCount
			store.ChunkCount = s.Stats.ChunkCount
			store.Store.TotalSize = s.Stats.TotalSize
		}
		stores = append(stores, store)
	}

	h.renderStoresGrid(w, r, stores, "")
}

func (h *Handler) renderStoresGrid(w http.ResponseWriter, r *http.Request, stores []StoreWithConnections, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if errMsg != "" {
		if err := ErrorMessage(errMsg).Render(r.Context(), w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
	}

	if err := StoresGrid(stores).Render(r.Context(), w); err != nil {
		h.log.Error("Failed to render stores grid", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// Middleware
// =============================================================================

// TimeoutMiddleware adds a timeout to requests.
func TimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// =============================================================================
// Helper Functions
// =============================================================================

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

// getOrDefault returns val if non-zero, otherwise returns defaultVal
func getOrDefault(val int, defaultVal int) int {
	if val == 0 {
		return defaultVal
	}
	return val
}

// getOrDefaultFloat returns val if non-zero, otherwise returns defaultVal
func getOrDefaultFloat(val float32, defaultVal float32) float32 {
	if val == 0 {
		return defaultVal
	}
	return val
}
