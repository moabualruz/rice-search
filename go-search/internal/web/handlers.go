// Package web provides the web UI using templ templates and HTMX.
package web

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	pb "github.com/ricesearch/rice-search/api/proto/ricesearchpb"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// GRPCClient interface defines the gRPC methods needed by the web handlers.
// This allows us to use either the gRPC server directly or a gRPC client.
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
	grpc GRPCClient
	log  *logger.Logger
}

// NewHandler creates a new web handler.
func NewHandler(grpc GRPCClient, log *logger.Logger) *Handler {
	return &Handler{
		grpc: grpc,
		log:  log,
	}
}

// RegisterRoutes registers all web routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Pages
	mux.HandleFunc("GET /", h.handleSearchPage)
	mux.HandleFunc("GET /admin", h.handleAdminPage)
	mux.HandleFunc("GET /stats", h.handleStatsPage)

	// Search API (HTMX)
	mux.HandleFunc("POST /search", h.handleSearch)

	// Admin API (HTMX)
	mux.HandleFunc("POST /admin/stores", h.handleCreateStore)
	mux.HandleFunc("DELETE /admin/stores/{name}", h.handleDeleteStore)

	// Stats API (HTMX)
	mux.HandleFunc("GET /stats/refresh", h.handleStatsRefresh)
}

// handleSearchPage renders the search page.
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

	data := SearchPageData{
		Store:  "default",
		Stores: stores,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SearchPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render search page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleSearch handles search requests from the form.
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

	// Parse options
	topK := 20
	if v := r.FormValue("top_k"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			topK = parsed
		}
	}

	enableRerank := r.FormValue("rerank") == "on"
	includeContent := r.FormValue("content") == "on"

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
	for i, r := range resp.Results {
		res := SearchResult{
			ID:        r.ID,
			Path:      r.Path,
			Language:  r.Language,
			StartLine: int(r.StartLine),
			EndLine:   int(r.EndLine),
			Content:   r.Content,
			Score:     r.Score,
		}
		if r.RerankScore != nil {
			res.RerankScore = *r.RerankScore
		}
		results[i] = res
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

// renderSearchResults renders just the results section for HTMX.
func (h *Handler) renderSearchResults(w http.ResponseWriter, r *http.Request, data SearchPageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := SearchResults(data).Render(r.Context(), w); err != nil {
		h.log.Error("Failed to render search results", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleAdminPage renders the admin page.
func (h *Handler) handleAdminPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	data := AdminPageData{}

	// Get all stores
	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err != nil {
		data.Error = err.Error()
	} else {
		for _, s := range storesResp.Stores {
			info := StoreInfo{
				Name:        s.Name,
				DisplayName: s.DisplayName,
				Description: s.Description,
			}
			if s.Stats != nil {
				info.DocumentCount = s.Stats.DocumentCount
				info.ChunkCount = s.Stats.ChunkCount
				info.TotalSize = s.Stats.TotalSize
				if s.Stats.LastIndexed != nil {
					info.LastIndexed = s.Stats.LastIndexed.AsTime()
				}
			}
			if s.CreatedAt != nil {
				info.CreatedAt = s.CreatedAt.AsTime()
			}
			data.Stores = append(data.Stores, info)
		}
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := AdminPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render admin page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleCreateStore handles store creation.
func (h *Handler) handleCreateStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := r.ParseForm(); err != nil {
		h.renderStoresList(w, r, nil, "Invalid form data")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	displayName := strings.TrimSpace(r.FormValue("display_name"))
	description := strings.TrimSpace(r.FormValue("description"))

	if name == "" {
		h.renderStoresList(w, r, nil, "Name is required")
		return
	}

	// Create store
	_, err := h.grpc.CreateStore(ctx, &pb.CreateStoreRequest{
		Name:        name,
		DisplayName: displayName,
		Description: description,
	})
	if err != nil {
		h.renderStoresList(w, r, nil, err.Error())
		return
	}

	// Refresh store list
	h.refreshStoresList(w, r)
}

// handleDeleteStore handles store deletion.
func (h *Handler) handleDeleteStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	name := r.PathValue("name")
	if name == "" {
		h.renderStoresList(w, r, nil, "Store name is required")
		return
	}

	if name == "default" {
		h.renderStoresList(w, r, nil, "Cannot delete default store")
		return
	}

	_, err := h.grpc.DeleteStore(ctx, &pb.DeleteStoreRequest{Name: name})
	if err != nil {
		h.renderStoresList(w, r, nil, err.Error())
		return
	}

	// Refresh store list
	h.refreshStoresList(w, r)
}

// refreshStoresList fetches stores and renders the list.
func (h *Handler) refreshStoresList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storesResp, err := h.grpc.ListStores(ctx, &pb.ListStoresRequest{})
	if err != nil {
		h.renderStoresList(w, r, nil, err.Error())
		return
	}

	var stores []StoreInfo
	for _, s := range storesResp.Stores {
		info := StoreInfo{
			Name:        s.Name,
			DisplayName: s.DisplayName,
			Description: s.Description,
		}
		if s.Stats != nil {
			info.DocumentCount = s.Stats.DocumentCount
			info.ChunkCount = s.Stats.ChunkCount
			info.TotalSize = s.Stats.TotalSize
			if s.Stats.LastIndexed != nil {
				info.LastIndexed = s.Stats.LastIndexed.AsTime()
			}
		}
		if s.CreatedAt != nil {
			info.CreatedAt = s.CreatedAt.AsTime()
		}
		stores = append(stores, info)
	}

	h.renderStoresList(w, r, stores, "")
}

// renderStoresList renders just the stores list for HTMX.
func (h *Handler) renderStoresList(w http.ResponseWriter, r *http.Request, stores []StoreInfo, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if errMsg != "" {
		// Render error followed by current stores
		if err := ErrorMessage(errMsg).Render(r.Context(), w); err != nil {
			h.log.Error("Failed to render error", "error", err)
		}
	}

	if err := StoresList(stores).Render(r.Context(), w); err != nil {
		h.log.Error("Failed to render stores list", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleStatsPage renders the stats page.
func (h *Handler) handleStatsPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := h.getStatsData(ctx)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsPage(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats page", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleStatsRefresh handles stats refresh for HTMX.
func (h *Handler) handleStatsRefresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := h.getStatsData(ctx)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := StatsContent(data).Render(ctx, w); err != nil {
		h.log.Error("Failed to render stats content", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getStatsData collects all stats data.
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
		data.GoVersion = "unknown"
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
				data.TotalDocs += s.Stats.DocumentCount
				data.TotalChunks += s.Stats.ChunkCount
			}
		}
	}

	return data
}

// TimeoutMiddleware adds a timeout to requests.
func TimeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
