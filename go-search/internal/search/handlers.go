package search

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ricesearch/rice-search/internal/observability"
)

// Handler provides HTTP handlers for search operations.
type Handler struct {
	svc           *Service
	observability *observability.Service
}

// NewHandler creates a new search handler.
func NewHandler(svc *Service, obs *observability.Service) *Handler {
	return &Handler{
		svc:           svc,
		observability: obs,
	}
}

// SearchRequest is the JSON request body for search.
type SearchRequest struct {
	Query           string   `json:"query"`
	TopK            int      `json:"top_k,omitempty"`
	Filter          *Filter  `json:"filter,omitempty"`
	EnableReranking *bool    `json:"enable_reranking,omitempty"`
	RerankTopK      int      `json:"rerank_top_k,omitempty"`
	IncludeContent  bool     `json:"include_content,omitempty"`
	SparseWeight    *float32 `json:"sparse_weight,omitempty"`
	DenseWeight     *float32 `json:"dense_weight,omitempty"`
	GroupByFile     bool     `json:"group_by_file,omitempty"`
	MaxPerFile      int      `json:"max_per_file,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

// HandleSearch handles POST /v1/stores/{store}/search
func (h *Handler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get store from URL path (assumes mux extracts it)
	store := r.PathValue("store")
	if store == "" {
		// Fallback: try to parse from URL
		// Pattern: /v1/stores/{store}/search
		store = extractStoreFromPath(r.URL.Path)
	}
	if store == "" {
		writeError(w, http.StatusBadRequest, "store parameter is required")
		return
	}

	// Parse request body
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	// Apply default connection scoping
	h.applyDefaultConnectionScope(r, &req)

	// Build search request
	searchReq := Request{
		Query:           req.Query,
		Store:           store,
		TopK:            req.TopK,
		Filter:          req.Filter,
		EnableReranking: req.EnableReranking,
		RerankTopK:      req.RerankTopK,
		IncludeContent:  req.IncludeContent,
		SparseWeight:    req.SparseWeight,
		DenseWeight:     req.DenseWeight,
	}

	resp, err := h.svc.Search(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Apply grouping if requested
	if req.GroupByFile {
		maxPerFile := req.MaxPerFile
		if maxPerFile <= 0 {
			maxPerFile = 3
		}
		resp.Results = GroupByFile(resp.Results, maxPerFile)
	}

	// Log query
	if h.observability != nil {
		var intent string
		if resp.ParsedQuery != nil {
			intent = string(resp.ParsedQuery.ActionIntent)
		}

		go h.observability.LogQuery(observability.QueryLogEntry{
			Timestamp:       time.Now(),
			Store:           store,
			Query:           req.Query,
			Intent:          intent,
			ResultCount:     len(resp.Results),
			LatencyMs:       resp.Metadata.SearchTimeMs,
			RerankEnabled:   req.EnableReranking != nil && *req.EnableReranking,
			RerankLatencyMs: resp.Metadata.RerankTimeMs,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleDenseSearch handles POST /v1/stores/{store}/search/dense
func (h *Handler) HandleDenseSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	store := r.PathValue("store")
	if store == "" {
		store = extractStoreFromPath(r.URL.Path)
	}
	if store == "" {
		writeError(w, http.StatusBadRequest, "store parameter is required")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	searchReq := Request{
		Query:          req.Query,
		Store:          store,
		TopK:           req.TopK,
		Filter:         req.Filter,
		IncludeContent: req.IncludeContent,
	}

	resp, err := h.svc.SearchDenseOnly(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleSparseSearch handles POST /v1/stores/{store}/search/sparse
func (h *Handler) HandleSparseSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	store := r.PathValue("store")
	if store == "" {
		store = extractStoreFromPath(r.URL.Path)
	}
	if store == "" {
		writeError(w, http.StatusBadRequest, "store parameter is required")
		return
	}

	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	searchReq := Request{
		Query:          req.Query,
		Store:          store,
		TopK:           req.TopK,
		Filter:         req.Filter,
		IncludeContent: req.IncludeContent,
	}

	resp, err := h.svc.SearchSparseOnly(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// extractStoreFromPath extracts store name from URL path.
// Pattern: /v1/stores/{store}/...
func extractStoreFromPath(path string) string {
	// Simple extraction - in production, use a proper router
	const prefix = "/v1/stores/"
	if len(path) < len(prefix) {
		return ""
	}

	path = path[len(prefix):]
	for i, c := range path {
		if c == '/' {
			return path[:i]
		}
	}
	return path
}

// RegisterRoutes registers search routes with the given mux.
// Note: This uses Go 1.22+ ServeMux patterns.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/stores/{store}/search", h.HandleSearch)
	mux.HandleFunc("POST /v1/stores/{store}/search/dense", h.HandleDenseSearch)
	mux.HandleFunc("POST /v1/stores/{store}/search/sparse", h.HandleSparseSearch)
	mux.HandleFunc("GET /v1/observability/export", h.handleObservabilityExport)
}

func (h *Handler) handleSearchWithStore(w http.ResponseWriter, r *http.Request, store string) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Apply default connection scoping
	h.applyDefaultConnectionScope(r, &req)

	searchReq := Request{
		Query:           req.Query,
		Store:           store,
		TopK:            req.TopK,
		Filter:          req.Filter,
		EnableReranking: req.EnableReranking,
		RerankTopK:      req.RerankTopK,
		IncludeContent:  req.IncludeContent,
	}

	resp, err := h.svc.Search(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.GroupByFile {
		resp.Results = GroupByFile(resp.Results, req.MaxPerFile)
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleDenseWithStore(w http.ResponseWriter, r *http.Request, store string) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	searchReq := Request{
		Query:          req.Query,
		Store:          store,
		TopK:           req.TopK,
		Filter:         req.Filter,
		IncludeContent: req.IncludeContent,
	}

	resp, err := h.svc.SearchDenseOnly(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleSparseWithStore(w http.ResponseWriter, r *http.Request, store string) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	searchReq := Request{
		Query:          req.Query,
		Store:          store,
		TopK:           req.TopK,
		Filter:         req.Filter,
		IncludeContent: req.IncludeContent,
	}

	resp, err := h.svc.SearchSparseOnly(r.Context(), searchReq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// applyDefaultConnectionScope applies default connection scoping from X-Connection-ID header.
// If the request has no explicit connection filter, it uses the header value.
// Users can opt out by setting connection_id to "*" or "all" to search all connections.
func (h *Handler) applyDefaultConnectionScope(r *http.Request, req *SearchRequest) {
	// Extract connection ID from header
	headerConnID := r.Header.Get("X-Connection-ID")
	if headerConnID == "" {
		return // No header, no default scoping
	}

	// Initialize filter if needed
	if req.Filter == nil {
		req.Filter = &Filter{}
	}

	// Apply default scoping only if not explicitly set
	if req.Filter.ConnectionID == "" {
		// Use header connection ID as default
		req.Filter.ConnectionID = headerConnID
		// Log when default scoping is applied (use structured logging in production)
		// h.log.Debug("Applied default connection scoping", "connection_id", headerConnID)
	} else if req.Filter.ConnectionID == "*" || req.Filter.ConnectionID == "all" {
		// Explicit opt-out: search all connections
		req.Filter.ConnectionID = ""
	}
	// If ConnectionID is explicitly set to a specific value, respect it
}

// Middleware provides common HTTP middleware.

// LoggingMiddleware logs request details.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// In production, use structured logging
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware adds CORS headers.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
