// Package web provides the web UI using templ templates and HTMX.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
	"github.com/ricesearch/rice-search/internal/bus"
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
	ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error)
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
	eventLogger *bus.EventLogger
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
	eventLogger *bus.EventLogger,
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
		eventLogger: eventLogger,
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
	mux.HandleFunc("GET /stores/{name}/files/{path...}", h.handleFileDetail)
	mux.HandleFunc("GET /files", h.handleFilesPage)
	mux.HandleFunc("GET /stats", h.handleStatsPage)

	// File Operations API (HTMX)
	mux.HandleFunc("POST /files/{path...}/reindex", h.handleReindexFile)
	mux.HandleFunc("DELETE /files/{path...}", h.handleDeleteFile)
	mux.HandleFunc("GET /stores/{name}/files/export", h.handleExportFiles)

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
	mux.HandleFunc("POST /admin/connections/{id}/rename", h.handleRenameConnection)
	mux.HandleFunc("DELETE /admin/connections/{id}", h.handleDeleteConnection)

	// Settings API (HTMX)
	mux.HandleFunc("POST /admin/settings", h.handleSaveSettings)
	mux.HandleFunc("GET /admin/settings/export", h.handleExportSettings)
	mux.HandleFunc("POST /admin/settings/import", h.handleImportSettings)
	mux.HandleFunc("POST /admin/settings/reset", h.handleResetSettings)

	// Settings REST API (JSON)
	mux.HandleFunc("GET /api/v1/settings", h.handleAPIGetSettings)
	mux.HandleFunc("PUT /api/v1/settings", h.handleAPIPutSettings)
	mux.HandleFunc("GET /api/v1/settings/history", h.handleAPIGetSettingsHistory)
	mux.HandleFunc("GET /api/v1/settings/audit", h.handleAPIGetSettingsAudit)
	mux.HandleFunc("POST /api/v1/settings/rollback/{version}", h.handleAPIRollbackSettings)

	// Stats JSON API (for Grafana/monitoring)
	mux.HandleFunc("GET /api/v1/stats/overview", h.handleAPIStatsOverview)
	mux.HandleFunc("GET /api/v1/stats/search-timeseries", h.handleAPIStatsSearchTimeseries)
	mux.HandleFunc("GET /api/v1/stats/index-timeseries", h.handleAPIStatsIndexTimeseries)
	mux.HandleFunc("GET /api/v1/stats/latency-timeseries", h.handleAPIStatsLatencyTimeseries)
	mux.HandleFunc("GET /api/v1/stats/stores", h.handleAPIStatsStores)
	mux.HandleFunc("GET /api/v1/stats/languages", h.handleAPIStatsLanguages)
	mux.HandleFunc("GET /api/v1/stats/connections", h.handleAPIStatsConnections)

	// Event Logging API (for debugging)
	mux.HandleFunc("GET /api/v1/events", h.handleAPIGetEvents)

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
			// Extract device info for ML component
			// TODO: Uncomment after regenerating protobuf with DeviceInfo
			// if comp.DeviceInfo != nil {
			// 	status.Device = comp.DeviceInfo.Device
			// 	status.ActualDevice = comp.DeviceInfo.ActualDevice
			// 	status.DeviceFallback = comp.DeviceInfo.DeviceFallback
			// 	status.RuntimeAvail = comp.DeviceInfo.RuntimeAvailable
			// }
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

	// Recent files (get from default store, sorted by indexed_at desc)
	filesResp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
		Store:     "default",
		Page:      1,
		PageSize:  5,
		SortBy:    "indexed_at",
		SortOrder: "desc",
	})
	if err == nil && filesResp != nil {
		for _, f := range filesResp.Files {
			data.RecentFiles = append(data.RecentFiles, RecentFile{
				Path:      f.Path,
				Language:  f.Language,
				Chunks:    int(f.ChunkCount),
				IndexedAt: f.GetIndexedAt().AsTime(),
			})
		}
	}

	// Recent searches from metrics (if we have time-series data)
	if h.metrics != nil && h.metrics.TimeSeries != nil {
		// Get recent search count per time bucket as pseudo "recent searches"
		history := h.metrics.TimeSeries.SearchRate.GetHistoryWithCurrent()
		for i := len(history) - 1; i >= 0 && len(data.RecentSearches) < 5; i-- {
			if history[i].Value > 0 {
				data.RecentSearches = append(data.RecentSearches, RecentSearch{
					Query:     fmt.Sprintf("%.0f searches", history[i].Value),
					Store:     "all",
					Results:   int(history[i].Value),
					Timestamp: history[i].Timestamp,
				})
			}
		}
	}

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
			ID:        res.Id,
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

	data := StoreDetailPageData{
		Layout: h.getLayoutData(ctx, name+" Store", "/stores"),
	}

	// Get store details
	storeResp, err := h.grpc.GetStore(ctx, &pb.GetStoreRequest{Name: name})
	if err != nil {
		data.Error = fmt.Sprintf("Failed to get store: %v", err)
	} else if storeResp != nil {
		data.Store = StoreInfo{
			Name:        storeResp.Name,
			DisplayName: storeResp.DisplayName,
			Description: storeResp.Description,
		}
		if storeResp.CreatedAt != nil {
			data.Store.CreatedAt = storeResp.CreatedAt.AsTime()
		}
	}

	// Get store stats
	statsResp, err := h.grpc.GetStoreStats(ctx, &pb.GetStoreStatsRequest{Name: name})
	if err == nil && statsResp != nil {
		data.Store.DocumentCount = statsResp.DocumentCount
		data.Store.ChunkCount = statsResp.ChunkCount
		data.Store.TotalSize = statsResp.TotalSize
		if statsResp.LastIndexed != nil {
			data.Store.LastIndexed = statsResp.LastIndexed.AsTime()
		}
	}

	// Get connections for this store
	if h.connSvc != nil {
		conns, err := h.connSvc.GetConnectionsForStore(ctx, name)
		if err == nil {
			for _, c := range conns {
				conn := ConnectionDetail{
					ID:           c.ID,
					Name:         c.Name,
					FilesIndexed: c.IndexedFiles,
					SearchCount:  c.SearchCount,
					LastSeen:     c.LastSeenAt,
					LastIP:       c.LastIP,
					CreatedAt:    c.CreatedAt,
					PCInfo: PCInfoDisplay{
						Hostname: c.PCInfo.Hostname,
						OS:       c.PCInfo.OS,
						Arch:     c.PCInfo.Arch,
						Username: c.PCInfo.Username,
					},
				}
				data.Connections = append(data.Connections, conn)
			}
		}
	}

	// Get recent files (last 10)
	filesResp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
		Store:     name,
		Page:      1,
		PageSize:  10,
		SortBy:    "indexed_at",
		SortOrder: "desc",
	})
	if err == nil && filesResp != nil {
		for _, f := range filesResp.Files {
			file := IndexedFileInfo{
				Path:       f.Path,
				Language:   f.Language,
				Size:       f.Size,
				ChunkCount: int(f.ChunkCount),
				Hash:       f.Hash,
				Status:     "indexed",
			}
			if f.IndexedAt != nil {
				file.IndexedAt = f.IndexedAt.AsTime()
			}
			data.RecentFiles = append(data.RecentFiles, file)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StoreDetailPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render store detail page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
	if store == "" {
		store = "default"
	}

	page := parseIntOr(r.URL.Query().Get("page"), 1)
	pageSize := parseIntOr(r.URL.Query().Get("page_size"), 50)

	data := FilesPageData{
		Layout:   h.getLayoutData(ctx, "Files", "/files"),
		Store:    store,
		Page:     page,
		PageSize: pageSize,
		Filters: FileFilters{
			Store:        store,
			PathPrefix:   r.URL.Query().Get("path"),
			Language:     r.URL.Query().Get("language"),
			ConnectionID: r.URL.Query().Get("connection_id"),
			SortBy:       r.URL.Query().Get("sort_by"),
			SortOrder:    r.URL.Query().Get("sort_order"),
		},
	}

	// Populate available connections for filter dropdown
	if h.connSvc != nil {
		conns, err := h.connSvc.ListConnections(ctx, connection.ConnectionFilter{})
		if err == nil {
			for _, c := range conns {
				name := c.Name
				if name == "" && len(c.ID) > 8 {
					name = c.ID[:8] + "..."
				} else if name == "" {
					name = c.ID
				}
				data.Connections = append(data.Connections, ConnectionOption{
					ID:   c.ID,
					Name: name,
				})
			}
		}
	}

	// Fetch files from gRPC service
	resp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
		Store:      store,
		PathPrefix: data.Filters.PathPrefix,
		Language:   data.Filters.Language,
		Page:       int32(page),
		PageSize:   int32(pageSize),
		SortBy:     data.Filters.SortBy,
		SortOrder:  data.Filters.SortOrder,
	})
	if err != nil {
		h.log.Error("Failed to list files", "store", store, "error", err)
		data.Error = fmt.Sprintf("Failed to list files: %v", err)
	} else {
		// Convert protobuf files to display format
		for _, f := range resp.Files {
			data.Files = append(data.Files, IndexedFileInfo{
				Path:      f.Path,
				Language:  f.Language,
				Size:      f.Size,
				Hash:      f.Hash,
				IndexedAt: f.GetIndexedAt().AsTime(),
				Status:    f.Status,
			})
		}
		data.Total = int(resp.Total)
		data.TotalPages = int(resp.TotalPages)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := FilesPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render files page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// =============================================================================
// File Detail Page
// =============================================================================

func (h *Handler) handleFileDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	storeName := r.PathValue("name")
	filePath := r.PathValue("path")

	data := FileDetailPageData{
		Layout: h.getLayoutData(ctx, filePath, "/files"),
		Store:  storeName,
	}

	// Get file info from the list with path filter
	filesResp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
		Store:      storeName,
		PathPrefix: filePath,
		Page:       1,
		PageSize:   1,
	})
	if err != nil {
		data.Error = fmt.Sprintf("Failed to get file info: %v", err)
	} else if filesResp != nil && len(filesResp.Files) > 0 {
		f := filesResp.Files[0]
		// Only use if exact match
		if f.Path == filePath {
			data.File = IndexedFileInfo{
				Path:         f.Path,
				Language:     f.Language,
				Size:         f.Size,
				ChunkCount:   int(f.ChunkCount),
				ConnectionID: f.ConnectionId,
				Hash:         f.Hash,
				Status:       "indexed",
			}
			if f.IndexedAt != nil {
				data.File.IndexedAt = f.IndexedAt.AsTime()
			}

			// Get connection info if available
			if h.connSvc != nil && f.ConnectionId != "" {
				conn, err := h.connSvc.GetConnection(ctx, f.ConnectionId)
				if err == nil && conn != nil {
					data.Connection = ConnectionDetail{
						ID:           conn.ID,
						Name:         conn.Name,
						FilesIndexed: conn.IndexedFiles,
						SearchCount:  conn.SearchCount,
						LastSeen:     conn.LastSeenAt,
						LastIP:       conn.LastIP,
						CreatedAt:    conn.CreatedAt,
						PCInfo: PCInfoDisplay{
							Hostname: conn.PCInfo.Hostname,
							OS:       conn.PCInfo.OS,
							Arch:     conn.PCInfo.Arch,
							Username: conn.PCInfo.Username,
						},
					}
					data.File.ConnectionName = conn.Name
				}
			}

			// TODO: Get chunks for this file when chunk API is available
			// For now, we show the chunk count but not individual chunks
		} else {
			data.Error = fmt.Sprintf("File not found: %s", filePath)
		}
	} else {
		data.Error = fmt.Sprintf("File not found: %s", filePath)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := FileDetailPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render file detail page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleReindexFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filePath := r.PathValue("path")

	// TODO: Implement file reindex when API is available
	// For now, return a success message indicating the feature is pending
	h.log.Info("Reindex file requested", "path", filePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Re-index requested for %s (feature pending)", filePath)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filePath := r.PathValue("path")

	// TODO: Implement file deletion when API is available
	// For now, return a success message indicating the feature is pending
	h.log.Info("Delete file requested", "path", filePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SuccessMessage(fmt.Sprintf("Delete requested for %s (feature pending)", filePath)).Render(ctx, w); err != nil {
		h.log.Error("Failed to render success message", "error", err)
	}
}

func (h *Handler) handleExportFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	storeName := r.PathValue("name")
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}

	// Get all files from store
	var allFiles []*pb.IndexedFile
	page := int32(1)
	for {
		resp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
			Store:     storeName,
			Page:      page,
			PageSize:  100,
			SortBy:    "path",
			SortOrder: "asc",
		})
		if err != nil {
			http.Error(w, "Failed to list files: "+err.Error(), http.StatusInternalServerError)
			return
		}
		allFiles = append(allFiles, resp.Files...)
		if int32(len(allFiles)) >= resp.Total || len(resp.Files) == 0 {
			break
		}
		page++
	}

	switch format {
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-files.json", storeName))

		type ExportFile struct {
			Path       string `json:"path"`
			Language   string `json:"language"`
			Size       int64  `json:"size"`
			ChunkCount int32  `json:"chunk_count"`
			Hash       string `json:"hash"`
			IndexedAt  string `json:"indexed_at"`
		}

		export := make([]ExportFile, len(allFiles))
		for i, f := range allFiles {
			export[i] = ExportFile{
				Path:       f.Path,
				Language:   f.Language,
				Size:       f.Size,
				ChunkCount: f.ChunkCount,
				Hash:       f.Hash,
				IndexedAt:  f.GetIndexedAt().AsTime().Format(time.RFC3339),
			}
		}

		data, _ := json.MarshalIndent(export, "", "  ")
		w.Write(data)

	default: // CSV
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-files.csv", storeName))

		// Write CSV header
		w.Write([]byte("path,language,size,chunk_count,hash,indexed_at\n"))

		// Write data rows
		for _, f := range allFiles {
			line := fmt.Sprintf("%q,%s,%d,%d,%s,%s\n",
				f.Path, f.Language, f.Size, f.ChunkCount, f.Hash, f.GetIndexedAt().AsTime().Format(time.RFC3339))
			w.Write([]byte(line))
		}
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

func (h *Handler) handleEditMapperModal(w http.ResponseWriter, r *http.Request) {
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

	// Get models for the editor
	var models []ModelInfoDisplay
	if h.modelReg != nil {
		for _, m := range h.modelReg.ListAllModels() {
			models = append(models, ModelInfoDisplay{
				ID:          m.ID,
				Type:        string(m.Type),
				DisplayName: m.DisplayName,
			})
		}
	}

	// Convert mapper to display format
	mapperDisplay := &ModelMapperDisplay{
		ID:             mapper.ID,
		Name:           mapper.Name,
		ModelID:        mapper.ModelID,
		Type:           string(mapper.Type),
		PromptTemplate: mapper.PromptTemplate,
		InputMapping:   mapper.InputMapping,
		OutputMapping:  mapper.OutputMapping,
	}

	// Render modal with mapper data
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := MapperEditorModal(models, mapperDisplay).Render(ctx, w); err != nil {
		h.log.Error("Failed to render mapper editor modal", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
	ctx := r.Context()

	// Check mapper service is available
	if h.mapperSvc == nil {
		http.Error(w, "Mapper service not available", http.StatusServiceUnavailable)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	modelID := r.FormValue("model_id")
	if modelID == "" {
		http.Error(w, "model_id is required", http.StatusBadRequest)
		return
	}

	// Generate mapper using the service
	mapper, err := h.mapperSvc.GenerateMapper(ctx, modelID)
	if err != nil {
		h.log.Error("Failed to generate mapper", "model_id", modelID, "error", err)
		http.Error(w, fmt.Sprintf("Failed to generate mapper: %v", err), http.StatusInternalServerError)
		return
	}

	// Get models list for dropdown
	var modelsList []ModelInfoDisplay
	if h.modelReg != nil {
		for _, m := range h.modelReg.ListAllModels() {
			modelsList = append(modelsList, ModelInfoDisplay{
				ID:          m.ID,
				Type:        string(m.Type),
				DisplayName: m.DisplayName,
			})
		}
	}

	// Convert to display struct
	mapperDisplay := &ModelMapperDisplay{
		ID:             mapper.ID,
		Name:           mapper.Name,
		ModelID:        mapper.ModelID,
		Type:           string(mapper.Type),
		InputMapping:   mapper.InputMapping,
		OutputMapping:  mapper.OutputMapping,
		PromptTemplate: mapper.PromptTemplate,
		CreatedAt:      mapper.CreatedAt,
		UpdatedAt:      mapper.UpdatedAt,
	}

	// Render the mapper editor modal with pre-filled data
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := MapperEditorModal(modelsList, mapperDisplay).Render(ctx, w); err != nil {
		h.log.Error("Failed to render mapper editor modal", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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

		// Determine source for each setting
		data.Sources = h.determineSettingSources(ctx)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminSettingsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render settings page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// determineSettingSources checks each setting to determine if it came from
// admin override, environment variable, config file, or default value
func (h *Handler) determineSettingSources(ctx context.Context) SettingSources {
	sources := SettingSources{}

	// Helper to check if env var is set
	envSet := func(key string) bool {
		_, exists := os.LookupEnv(key)
		return exists
	}

	// Helper to determine source: admin > env > config > default
	determineSource := func(fieldName, envVar string, configValue, defaultValue interface{}) string {
		// Check if admin has override
		if h.settingsSvc != nil && h.settingsSvc.HasAdminOverride(fieldName, configValue) {
			return "admin"
		}
		// Check if env var is set
		if envSet(envVar) {
			return "env"
		}
		// Check if different from default (config file)
		if configValue != defaultValue {
			return "config"
		}
		return "default"
	}

	// Server settings
	sources.ServerHost = determineSource("server_host", "RICE_HOST", h.cfg.Host, "0.0.0.0")
	sources.ServerPort = determineSource("server_port", "RICE_PORT", h.cfg.Port, 8080)
	sources.LogLevel = determineSource("log_level", "RICE_LOG_LEVEL", h.cfg.Log.Level, "info")
	sources.LogFormat = determineSource("log_format", "RICE_LOG_FORMAT", h.cfg.Log.Format, "text")

	// ML settings
	sources.EmbedModel = determineSource("embed_model", "RICE_EMBED_MODEL", h.cfg.ML.EmbedModel, "jinaai/jina-code-embeddings-1.5b")
	sources.RerankModel = determineSource("rerank_model", "RICE_RERANK_MODEL", h.cfg.ML.RerankModel, "jinaai/jina-reranker-v2-base-multilingual")
	sources.QueryModel = determineSource("query_model", "RICE_QUERY_MODEL", h.cfg.ML.QueryModel, "Salesforce/codet5p-220m")
	sources.EmbedGPU = determineSource("embed_gpu", "RICE_EMBED_GPU", h.cfg.ML.EmbedGPU, true)
	sources.RerankGPU = determineSource("rerank_gpu", "RICE_RERANK_GPU", h.cfg.ML.RerankGPU, true)
	sources.QueryGPU = determineSource("query_gpu", "RICE_QUERY_GPU", h.cfg.ML.QueryGPU, false)
	sources.QueryEnabled = determineSource("query_enabled", "RICE_QUERY_MODEL_ENABLED", h.cfg.ML.QueryModelEnabled, false)
	sources.BatchSize = determineSource("batch_size", "RICE_EMBED_BATCH_SIZE", h.cfg.ML.EmbedBatchSize, 32)
	sources.MaxConcurrent = determineSource("max_concurrent", "RICE_INDEX_WORKERS", h.cfg.Index.Workers, 4)

	// Qdrant settings
	sources.QdrantURL = determineSource("qdrant_url", "QDRANT_URL", h.cfg.Qdrant.URL, "http://localhost:6333")
	sources.QdrantCollection = determineSource("qdrant_collection", "QDRANT_COLLECTION_PREFIX", h.cfg.Qdrant.CollectionPrefix, "rice_")
	sources.QdrantTimeout = "default" // No env var for this

	// Search settings - most don't have env vars, will show as admin or default
	sources.DefaultTopK = determineSource("default_top_k", "RICE_DEFAULT_TOP_K", h.cfg.Search.DefaultTopK, 20)
	sources.RerankCandidates = determineSource("rerank_candidates", "RICE_RERANK_CANDIDATES", h.cfg.Search.RerankCandidates, 30)
	sources.MaxChunksPerFile = "default" // No base config equivalent
	sources.DefaultRerank = determineSource("default_rerank", "RICE_ENABLE_RERANKING", h.cfg.Search.EnableReranking, true)
	sources.DefaultDedup = "default"     // Only in admin settings
	sources.DefaultDiversity = "default" // Only in admin settings
	sources.SparseWeight = determineSource("sparse_weight", "RICE_DEFAULT_SPARSE_WEIGHT", h.cfg.Search.DefaultSparseWeight, 0.5)
	sources.DenseWeight = determineSource("dense_weight", "RICE_DEFAULT_DENSE_WEIGHT", h.cfg.Search.DefaultDenseWeight, 0.5)
	sources.DedupThreshold = "default"  // Only in admin settings
	sources.DiversityLambda = "default" // Only in admin settings

	// Index settings
	sources.ChunkSize = determineSource("chunk_size", "RICE_CHUNK_SIZE", h.cfg.Index.ChunkSize, 512)
	sources.ChunkOverlap = determineSource("chunk_overlap", "RICE_CHUNK_OVERLAP", h.cfg.Index.ChunkOverlap, 64)
	sources.MaxFileSize = "default"     // Only in admin settings
	sources.ExcludePatterns = "default" // Only in admin settings
	sources.SupportedLangs = "default"  // Only in admin settings

	// Connection settings
	sources.ConnectionEnabled = determineSource("connection_enabled", "RICE_CONNECTIONS_ENABLED", h.cfg.Connection.Enabled, true)
	sources.MaxInactiveHours = determineSource("max_inactive_hours", "RICE_CONNECTIONS_MAX_INACTIVE", h.cfg.Connection.MaxInactive, 30)

	return sources
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

func (h *Handler) handleExportSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.settingsSvc == nil {
		http.Error(w, "Settings service not available", http.StatusServiceUnavailable)
		return
	}

	// Determine format from query param (default: yaml)
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "yaml"
	}

	var data []byte
	var err error
	var contentType, filename string

	switch format {
	case "json":
		data, err = h.settingsSvc.ExportJSON(ctx)
		contentType = "application/json"
		filename = "settings.json"
	default:
		data, err = h.settingsSvc.ExportYAML(ctx)
		contentType = "application/x-yaml"
		filename = "settings.yaml"
	}

	if err != nil {
		http.Error(w, "Failed to export settings: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(data)
}

func (h *Handler) handleImportSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if h.settingsSvc == nil {
		if err := ErrorMessage("Settings service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Parse multipart form for file upload
	if err := r.ParseMultipartForm(1 << 20); err != nil { // 1MB max
		if err := ErrorMessage("Failed to parse form: "+err.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	file, header, err := r.FormFile("settings_file")
	if err != nil {
		if err := ErrorMessage("No file uploaded").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}
	defer file.Close()

	// Read file content
	data := make([]byte, header.Size)
	if _, err := file.Read(data); err != nil {
		if err := ErrorMessage("Failed to read file: "+err.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	// Determine format from filename
	var importErr error
	if strings.HasSuffix(header.Filename, ".json") {
		importErr = h.settingsSvc.ImportJSON(ctx, data, "admin-import")
	} else {
		importErr = h.settingsSvc.ImportYAML(ctx, data, "admin-import")
	}

	if importErr != nil {
		if err := ErrorMessage("Failed to import settings: "+importErr.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := SuccessMessage("Settings imported successfully").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success", "error", err)
	}
}

func (h *Handler) handleResetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if h.settingsSvc == nil {
		if err := ErrorMessage("Settings service not available").Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := h.settingsSvc.Reset(ctx, "admin-reset"); err != nil {
		if err := ErrorMessage("Failed to reset settings: "+err.Error()).Render(ctx, w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
		return
	}

	if err := SuccessMessage("Settings reset to defaults").Render(ctx, w); err != nil {
		h.log.Error("Failed to render success", "error", err)
	}
}

// =============================================================================
// Settings REST API (JSON)
// =============================================================================

// handleAPIGetSettings returns the current settings as JSON.
// GET /api/v1/settings
func (h *Handler) handleAPIGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.settingsSvc == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "settings service not available"})
		return
	}

	cfg := h.settingsSvc.Get(ctx)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(cfg); err != nil {
		h.log.Error("Failed to encode settings", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to encode settings"})
	}
}

// handleAPIPutSettings updates settings from a JSON body.
// PUT /api/v1/settings
func (h *Handler) handleAPIPutSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	if h.settingsSvc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "settings service not available"})
		return
	}

	// Parse JSON body
	var cfg settings.RuntimeConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	// Update settings
	if err := h.settingsSvc.Update(ctx, cfg, "api"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to update settings: " + err.Error()})
		return
	}

	// Return updated settings
	updatedCfg := h.settingsSvc.Get(ctx)
	json.NewEncoder(w).Encode(updatedCfg)
}

// handleAPIGetSettingsHistory returns the settings version history as JSON.
// GET /api/v1/settings/history
func (h *Handler) handleAPIGetSettingsHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	if h.settingsSvc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "settings service not available"})
		return
	}

	// Parse limit from query param (default: 10)
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := h.settingsSvc.GetHistory(ctx, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get history: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// handleAPIGetSettingsAudit returns the settings audit log as JSON.
// GET /api/v1/settings/audit?limit=50
func (h *Handler) handleAPIGetSettingsAudit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	if h.settingsSvc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "settings service not available"})
		return
	}

	// Parse limit from query param (default: 50)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	entries, err := h.settingsSvc.GetAuditLog(ctx, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to get audit log: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

// handleAPIRollbackSettings rolls back settings to a specific version.
// POST /api/v1/settings/rollback/{version}
func (h *Handler) handleAPIRollbackSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	w.Header().Set("Content-Type", "application/json")

	if h.settingsSvc == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "settings service not available"})
		return
	}

	// Parse version from path parameter
	versionStr := r.PathValue("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid version number"})
		return
	}

	// Perform rollback
	if err := h.settingsSvc.Rollback(ctx, version, "api-rollback"); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "rollback failed: " + err.Error()})
		return
	}

	// Return new current settings
	currentCfg := h.settingsSvc.Get(ctx)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":          "rollback successful",
		"rolled_back_to":   version,
		"new_version":      currentCfg.Version,
		"current_settings": currentCfg,
	})
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
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	data := h.getStatsData(ctx, store, timeRange)
	data.Layout = h.getLayoutData(ctx, "Stats", "/stats")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) handleStatsRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	data := h.getStatsData(ctx, store, timeRange)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsContent(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats content", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) getStatsData(ctx context.Context, storeFilter, timeRange string) StatsPageData {
	data := StatsPageData{
		Components: make(map[string]HealthStatus),
		Store:      storeFilter,
		TimeRange:  timeRange,
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
			healthStatus := HealthStatus{
				Status:  status,
				Message: comp.Message,
				Latency: latency,
			}
			// Extract device info for ML component
			// TODO: Uncomment after regenerating protobuf with DeviceInfo
			// if comp.DeviceInfo != nil {
			// 	healthStatus.Device = comp.DeviceInfo.Device
			// 	healthStatus.ActualDevice = comp.DeviceInfo.ActualDevice
			// 	healthStatus.DeviceFallback = comp.DeviceInfo.DeviceFallback
			// 	healthStatus.RuntimeAvail = comp.DeviceInfo.RuntimeAvailable
			// }
			data.Components[name] = healthStatus
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
		// Populate available stores for the dropdown
		for _, s := range storesResp.Stores {
			data.Stores = append(data.Stores, s.Name)
		}

		// Calculate totals (filtered or all stores)
		data.TotalStores = len(storesResp.Stores)
		for _, s := range storesResp.Stores {
			// If store filter is set, only count matching store
			if storeFilter != "" && s.Name != storeFilter {
				continue
			}

			if s.Stats != nil {
				data.TotalFiles += s.Stats.DocumentCount
				data.TotalChunks += s.Stats.ChunkCount
			}
		}

		// If filtering by store, adjust total stores count
		if storeFilter != "" {
			data.TotalStores = 1
		}
	}

	// Get connections count and stats
	if h.connSvc != nil {
		conns, _ := h.connSvc.ListAllConnections(ctx)
		data.TotalConnections = len(conns)

		// Build per-connection metrics
		for _, c := range conns {
			data.FilesByConnection = append(data.FilesByConnection, ConnectionMetric{
				ConnectionID:   c.ID,
				ConnectionName: c.Name,
				FilesIndexed:   c.IndexedFiles,
				SearchCount:    c.SearchCount,
				LastActive:     c.LastSeenAt,
			})
		}
	}

	// Get time-series data from metrics
	if h.metrics != nil && h.metrics.TimeSeries != nil {
		// Searches over time (5-minute buckets)
		searchRateHistory := h.metrics.TimeSeries.SearchRate.GetHistoryWithCurrent()
		for _, dp := range searchRateHistory {
			data.SearchesOverTime = append(data.SearchesOverTime, TimeSeriesPoint{
				Timestamp: dp.Timestamp,
				Value:     dp.Value,
				Label:     dp.Timestamp.Format("15:04"),
			})
		}

		// Indexing over time
		indexRateHistory := h.metrics.TimeSeries.IndexRate.GetHistoryWithCurrent()
		for _, dp := range indexRateHistory {
			data.IndexingOverTime = append(data.IndexingOverTime, TimeSeriesPoint{
				Timestamp: dp.Timestamp,
				Value:     dp.Value,
				Label:     dp.Timestamp.Format("15:04"),
			})
		}

		// Latency over time (average search latency per bucket)
		latencyHistory := h.metrics.TimeSeries.SearchLatency.GetHistoryWithCurrent()
		for _, dp := range latencyHistory {
			data.LatencyOverTime = append(data.LatencyOverTime, TimeSeriesPoint{
				Timestamp: dp.Timestamp,
				Value:     dp.Value,
				Label:     dp.Timestamp.Format("15:04"),
			})
		}

		// Get total searches from metrics
		data.TotalSearches = h.metrics.SearchRequests.Value()

		// Calculate growth indicators (percentage change from 1 hour ago)
		oneHourAgo := time.Now().Add(-1 * time.Hour)

		// Get historical totals from 1 hour ago
		var prevFiles, prevChunks, prevConnections int64
		indexHistory := h.metrics.TimeSeries.IndexRate.GetHistorySince(oneHourAgo)
		if len(indexHistory) > 0 {
			// Sum up total files indexed in the past hour
			var filesIndexedLastHour int64
			for _, dp := range indexHistory {
				filesIndexedLastHour += int64(dp.Value)
			}
			prevFiles = data.TotalFiles - filesIndexedLastHour
		}

		// Calculate growth percentages
		data.FilesGrowth = calculateGrowth(data.TotalFiles, prevFiles)
		data.ChunksGrowth = calculateGrowth(data.TotalChunks, prevChunks)
		data.ConnectionsGrowth = calculateGrowth(int64(data.TotalConnections), prevConnections)
		// Store growth calculation would require tracking store creation/deletion events
		data.StoresGrowth = 0.0
	}

	// Build per-store metrics (filtered or all)
	if storesResp != nil {
		for _, s := range storesResp.Stores {
			// If store filter is set, only include matching store
			if storeFilter != "" && s.Name != storeFilter {
				continue
			}

			metric := StoreMetric{
				Store: s.Name,
			}
			if s.Stats != nil {
				metric.FileCount = s.Stats.DocumentCount
				metric.ChunkCount = s.Stats.ChunkCount
			}
			data.SearchesByStore = append(data.SearchesByStore, metric)
		}
	}

	// Build language breakdown by aggregating from all indexed files
	// Note: This requires querying Qdrant for language metadata which is expensive
	// For now, we'll provide a placeholder implementation that could be populated
	// via background aggregation or cached stats
	data.FilesByLanguage = h.getLanguageBreakdown(ctx, storeFilter, storesResp)

	return data
}

// =============================================================================
// Stats JSON API Handlers (for Grafana/monitoring)
// =============================================================================

// handleAPIStatsOverview returns overall stats as JSON
// GET /api/v1/stats/overview?store=default&time_range=1h
func (h *Handler) handleAPIStatsOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	// Collect overview data
	var totalFiles, totalChunks, totalSearches int64
	var totalStores, totalConnections int

	// Get stores stats
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err == nil {
		totalStores = len(storesResp.Stores)
		for _, s := range storesResp.Stores {
			if store != "" && s.Name != store {
				continue
			}
			if s.Stats != nil {
				totalFiles += s.Stats.DocumentCount
				totalChunks += s.Stats.ChunkCount
			}
		}
	}

	// Get connections count
	if h.connSvc != nil {
		conns, _ := h.connSvc.ListAllConnections(ctx)
		totalConnections = len(conns)
	}

	// Get total searches from metrics
	if h.metrics != nil {
		totalSearches = h.metrics.SearchRequests.Value()
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"total_stores":      totalStores,
			"total_files":       totalFiles,
			"total_chunks":      totalChunks,
			"total_connections": totalConnections,
			"total_searches":    totalSearches,
		},
		"meta": map[string]interface{}{
			"store":        store,
			"time_range":   timeRange,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode overview stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsSearchTimeseries returns search rate over time as JSON
// GET /api/v1/stats/search-timeseries?store=default&time_range=1h&granularity=5m
func (h *Handler) handleAPIStatsSearchTimeseries(w http.ResponseWriter, r *http.Request) {
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	granularity := r.URL.Query().Get("granularity")
	if timeRange == "" {
		timeRange = "1h"
	}
	if granularity == "" {
		granularity = "5m"
	}

	var dataPoints []map[string]interface{}

	// Get search rate from metrics
	if h.metrics != nil && h.metrics.TimeSeries != nil {
		history := h.metrics.TimeSeries.SearchRate.GetHistoryWithCurrent()
		for _, dp := range history {
			dataPoints = append(dataPoints, map[string]interface{}{
				"timestamp": dp.Timestamp.Format(time.RFC3339),
				"value":     dp.Value,
			})
		}
	}

	response := map[string]interface{}{
		"data": dataPoints,
		"meta": map[string]interface{}{
			"store":        store,
			"time_range":   timeRange,
			"granularity":  granularity,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode search timeseries", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsIndexTimeseries returns indexing rate over time as JSON
// GET /api/v1/stats/index-timeseries?store=default&time_range=1h&granularity=5m
func (h *Handler) handleAPIStatsIndexTimeseries(w http.ResponseWriter, r *http.Request) {
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	granularity := r.URL.Query().Get("granularity")
	if timeRange == "" {
		timeRange = "1h"
	}
	if granularity == "" {
		granularity = "5m"
	}

	var dataPoints []map[string]interface{}

	// Get index rate from metrics
	if h.metrics != nil && h.metrics.TimeSeries != nil {
		history := h.metrics.TimeSeries.IndexRate.GetHistoryWithCurrent()
		for _, dp := range history {
			dataPoints = append(dataPoints, map[string]interface{}{
				"timestamp": dp.Timestamp.Format(time.RFC3339),
				"value":     dp.Value,
			})
		}
	}

	response := map[string]interface{}{
		"data": dataPoints,
		"meta": map[string]interface{}{
			"store":        store,
			"time_range":   timeRange,
			"granularity":  granularity,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode index timeseries", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsLatencyTimeseries returns search latency over time as JSON
// GET /api/v1/stats/latency-timeseries?store=default&time_range=1h&granularity=5m
func (h *Handler) handleAPIStatsLatencyTimeseries(w http.ResponseWriter, r *http.Request) {
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	granularity := r.URL.Query().Get("granularity")
	if timeRange == "" {
		timeRange = "1h"
	}
	if granularity == "" {
		granularity = "5m"
	}

	var dataPoints []map[string]interface{}

	// Get latency from metrics
	if h.metrics != nil && h.metrics.TimeSeries != nil {
		history := h.metrics.TimeSeries.SearchLatency.GetHistoryWithCurrent()
		for _, dp := range history {
			dataPoints = append(dataPoints, map[string]interface{}{
				"timestamp": dp.Timestamp.Format(time.RFC3339),
				"value":     dp.Value,
			})
		}
	}

	response := map[string]interface{}{
		"data": dataPoints,
		"meta": map[string]interface{}{
			"store":        store,
			"time_range":   timeRange,
			"granularity":  granularity,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode latency timeseries", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsStores returns per-store breakdown as JSON
// GET /api/v1/stats/stores?time_range=1h
func (h *Handler) handleAPIStatsStores(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	timeRange := r.URL.Query().Get("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	var storeMetrics []map[string]interface{}

	// Get stores stats
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err == nil {
		for _, s := range storesResp.Stores {
			metric := map[string]interface{}{
				"store":       s.Name,
				"file_count":  int64(0),
				"chunk_count": int64(0),
			}
			if s.Stats != nil {
				metric["file_count"] = s.Stats.DocumentCount
				metric["chunk_count"] = s.Stats.ChunkCount
			}
			storeMetrics = append(storeMetrics, metric)
		}
	}

	response := map[string]interface{}{
		"data": storeMetrics,
		"meta": map[string]interface{}{
			"time_range":   timeRange,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode stores stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsLanguages returns per-language file breakdown as JSON
// GET /api/v1/stats/languages?store=default&time_range=1h
func (h *Handler) handleAPIStatsLanguages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	store := r.URL.Query().Get("store")
	timeRange := r.URL.Query().Get("time_range")
	if store == "" {
		store = "default"
	}
	if timeRange == "" {
		timeRange = "1h"
	}

	// Collect language stats from files
	languageStats := make(map[string]struct {
		FileCount  int64
		ChunkCount int64
	})

	var totalFiles int64

	// Get files and aggregate by language
	page := int32(1)
	for {
		filesResp, err := h.grpc.ListFiles(ctx, &pb.ListFilesRequest{
			Store:    store,
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			break
		}

		for _, f := range filesResp.Files {
			lang := f.Language
			if lang == "" {
				lang = "unknown"
			}
			stats := languageStats[lang]
			stats.FileCount++
			stats.ChunkCount += int64(f.ChunkCount)
			languageStats[lang] = stats
			totalFiles++
		}

		if int32(len(filesResp.Files)) < 100 {
			break
		}
		page++
	}

	// Convert to response format with percentages
	var languageMetrics []map[string]interface{}
	for lang, stats := range languageStats {
		percentage := float64(0)
		if totalFiles > 0 {
			percentage = (float64(stats.FileCount) / float64(totalFiles)) * 100
		}
		languageMetrics = append(languageMetrics, map[string]interface{}{
			"language":    lang,
			"file_count":  stats.FileCount,
			"chunk_count": stats.ChunkCount,
			"percentage":  percentage,
		})
	}

	response := map[string]interface{}{
		"data": languageMetrics,
		"meta": map[string]interface{}{
			"store":        store,
			"time_range":   timeRange,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode language stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAPIStatsConnections returns connection activity metrics as JSON
// GET /api/v1/stats/connections?time_range=1h
func (h *Handler) handleAPIStatsConnections(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	timeRange := r.URL.Query().Get("time_range")
	if timeRange == "" {
		timeRange = "1h"
	}

	var connectionMetrics []map[string]interface{}

	// Get connections
	if h.connSvc != nil {
		conns, err := h.connSvc.ListAllConnections(ctx)
		if err == nil {
			for _, c := range conns {
				connectionMetrics = append(connectionMetrics, map[string]interface{}{
					"connection_id":   c.ID,
					"connection_name": c.Name,
					"files_indexed":   c.IndexedFiles,
					"search_count":    c.SearchCount,
					"is_active":       c.IsActive,
					"last_active":     c.LastSeenAt.Format(time.RFC3339),
				})
			}
		}
	}

	response := map[string]interface{}{
		"data": connectionMetrics,
		"meta": map[string]interface{}{
			"time_range":   timeRange,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode connection stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
// Available for use when request timeout handling is needed beyond the server defaults.
// Usage: mux.Handle("/path", TimeoutMiddleware(30*time.Second, handler))
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

// calculateGrowth calculates percentage change from previous to current value
func calculateGrowth(current, previous int64) float64 {
	if previous == 0 {
		if current > 0 {
			return 100.0 // 100% growth from zero
		}
		return 0.0
	}
	return float64(current-previous) / float64(previous) * 100.0
}

// getLanguageBreakdown aggregates file counts by programming language
func (h *Handler) getLanguageBreakdown(ctx context.Context, storeFilter string, storesResp *pb.ListStoresResponse) []LanguageMetric {
	// For now, return a placeholder implementation
	// TODO: Implement proper language aggregation by:
	// 1. Option A: Query Qdrant for language field aggregation (expensive)
	// 2. Option B: Maintain language stats in StoreStats (requires schema change)
	// 3. Option C: Build language index in background and cache results

	// Placeholder with sample data structure
	// In production, this would query indexed documents and aggregate by language field
	languageCounts := make(map[string]int64)

	// This would be replaced with actual Qdrant query or cached stats lookup
	// For now, we return empty to avoid showing incorrect data
	if storesResp == nil || len(storesResp.Stores) == 0 {
		return nil
	}

	// Calculate total files for percentage
	var totalFiles int64
	for _, s := range storesResp.Stores {
		if storeFilter != "" && s.Name != storeFilter {
			continue
		}
		if s.Stats != nil {
			totalFiles += s.Stats.DocumentCount
		}
	}

	if totalFiles == 0 {
		return nil
	}

	// Build sorted language metrics
	var metrics []LanguageMetric
	for lang, count := range languageCounts {
		metrics = append(metrics, LanguageMetric{
			Language:   lang,
			FileCount:  count,
			ChunkCount: 0, // Would be populated from actual data
			Percentage: float64(count) / float64(totalFiles) * 100.0,
		})
	}

	// Sort by file count descending
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].FileCount > metrics[j].FileCount
	})

	return metrics
}

// =============================================================================
// Event Logging API (for debugging)
// =============================================================================

// handleAPIGetEvents returns logged events as JSON.
// GET /api/v1/events?since=2025-12-29T10:00:00Z&limit=50
func (h *Handler) handleAPIGetEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if h.eventLogger == nil || !h.eventLogger.IsEnabled() {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "event logging is disabled",
		})
		return
	}

	// Parse 'since' timestamp (default: 1 hour ago)
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().Add(-1 * time.Hour)
	if sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = t
		}
	}

	// Parse limit (default: 50, max: 1000)
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
			if limit > 1000 {
				limit = 1000
			}
		}
	}

	// Get events from logger
	events, err := h.eventLogger.GetEvents(since, limit)
	if err != nil {
		h.log.Error("Failed to get events", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to retrieve events: " + err.Error(),
		})
		return
	}

	response := map[string]interface{}{
		"events": events,
		"count":  len(events),
		"meta": map[string]interface{}{
			"since":        since.Format(time.RFC3339),
			"limit":        limit,
			"generated_at": time.Now().Format(time.RFC3339),
		},
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode events response", "error", err)
	}
}
