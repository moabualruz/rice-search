package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/ricesearch/rice-search/internal/index"
	"github.com/ricesearch/rice-search/internal/store"
)

// IndexHandler handles index-related HTTP requests.
type IndexHandler struct {
	pipeline *index.Pipeline
	stores   *store.Service
}

// NewIndexHandler creates a new index handler.
func NewIndexHandler(pipeline *index.Pipeline, stores *store.Service) *IndexHandler {
	return &IndexHandler{
		pipeline: pipeline,
		stores:   stores,
	}
}

// writeJSON writes a JSON response.
func writeIndexJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Log encoding error - can't return to client after headers written
		// In production, this would use a proper logger
		_ = err // Encoding error after response started
	}
}

// writeError writes an error response.
func writeIndexError(w http.ResponseWriter, status int, message string) {
	writeIndexJSON(w, status, map[string]string{"error": message})
}

// RegisterRoutes registers index routes.
func (h *IndexHandler) RegisterRoutes(mux *http.ServeMux) {
	// The routes are registered under /v1/stores/{store}/index
	// We rely on the main router to pass us the request
	mux.HandleFunc("/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		// Extract store name and subpath
		path := strings.TrimPrefix(r.URL.Path, "/v1/stores/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) < 2 {
			return // Let other handlers deal with this
		}

		storeName := parts[0]
		subPath := parts[1]

		// Only handle index endpoints
		if !strings.HasPrefix(subPath, "index") {
			return
		}

		// Route based on subpath
		switch subPath {
		case "index":
			switch r.Method {
			case http.MethodPost:
				h.handleIndex(w, r, storeName)
			case http.MethodDelete:
				h.handleDelete(w, r, storeName)
			default:
				writeIndexError(w, http.StatusMethodNotAllowed, "method not allowed")
			}

		case "index/reindex":
			if r.Method == http.MethodPost {
				h.handleReindex(w, r, storeName)
			} else {
				writeIndexError(w, http.StatusMethodNotAllowed, "method not allowed")
			}

		case "index/sync":
			if r.Method == http.MethodPost {
				h.handleSync(w, r, storeName)
			} else {
				writeIndexError(w, http.StatusMethodNotAllowed, "method not allowed")
			}

		case "index/stats":
			if r.Method == http.MethodGet {
				h.handleStats(w, r, storeName)
			} else {
				writeIndexError(w, http.StatusMethodNotAllowed, "method not allowed")
			}

		case "index/files":
			if r.Method == http.MethodGet {
				h.handleListFiles(w, r, storeName)
			} else {
				writeIndexError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		}
	})
}

// IndexRequest is the JSON body for indexing files.
type IndexRequest struct {
	Files []FileInput `json:"files"`
	Force bool        `json:"force,omitempty"`
}

// FileInput represents a file to index.
type FileInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// handleIndex handles POST /v1/stores/{store}/index
func (h *IndexHandler) handleIndex(w http.ResponseWriter, r *http.Request, storeName string) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIndexError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Files) == 0 {
		writeIndexError(w, http.StatusBadRequest, "files array is required")
		return
	}

	// Convert to documents
	docs := make([]*index.Document, len(req.Files))
	for i, f := range req.Files {
		docs[i] = index.NewDocument(f.Path, f.Content)
	}

	// Index
	result, err := h.pipeline.Index(r.Context(), index.IndexRequest{
		Store:     storeName,
		Documents: docs,
		Force:     req.Force,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeIndexError(w, http.StatusNotFound, err.Error())
		} else {
			writeIndexError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeIndexJSON(w, http.StatusOK, result)
}

// DeleteRequest is the JSON body for deleting files.
type DeleteRequest struct {
	Paths      []string `json:"paths,omitempty"`
	PathPrefix string   `json:"path_prefix,omitempty"`
}

// handleDelete handles DELETE /v1/stores/{store}/index
func (h *IndexHandler) handleDelete(w http.ResponseWriter, r *http.Request, storeName string) {
	var req DeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIndexError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Paths) == 0 && req.PathPrefix == "" {
		writeIndexError(w, http.StatusBadRequest, "paths or path_prefix is required")
		return
	}

	var err error
	var deleted int

	if len(req.Paths) > 0 {
		err = h.pipeline.Delete(r.Context(), storeName, req.Paths)
		deleted = len(req.Paths)
	} else {
		err = h.pipeline.DeleteByPrefix(r.Context(), storeName, req.PathPrefix)
		deleted = -1 // Unknown
	}

	if err != nil {
		writeIndexError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeIndexJSON(w, http.StatusOK, map[string]interface{}{
		"deleted": deleted,
	})
}

// handleReindex handles POST /v1/stores/{store}/index/reindex
func (h *IndexHandler) handleReindex(w http.ResponseWriter, r *http.Request, storeName string) {
	var req IndexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIndexError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Convert to documents
	docs := make([]*index.Document, len(req.Files))
	for i, f := range req.Files {
		docs[i] = index.NewDocument(f.Path, f.Content)
	}

	result, err := h.pipeline.Reindex(r.Context(), index.IndexRequest{
		Store:     storeName,
		Documents: docs,
	})
	if err != nil {
		writeIndexError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeIndexJSON(w, http.StatusOK, result)
}

// SyncRequest is the JSON body for syncing files.
type SyncRequest struct {
	CurrentPaths []string `json:"current_paths"`
}

// handleSync handles POST /v1/stores/{store}/index/sync
func (h *IndexHandler) handleSync(w http.ResponseWriter, r *http.Request, storeName string) {
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIndexError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	removed, err := h.pipeline.Sync(r.Context(), storeName, req.CurrentPaths)
	if err != nil {
		writeIndexError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeIndexJSON(w, http.StatusOK, map[string]interface{}{
		"removed": removed,
	})
}

// handleStats handles GET /v1/stores/{store}/index/stats
func (h *IndexHandler) handleStats(w http.ResponseWriter, r *http.Request, storeName string) {
	stats, err := h.pipeline.GetStats(r.Context(), storeName)
	if err != nil {
		writeIndexError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeIndexJSON(w, http.StatusOK, stats)
}

// handleListFiles handles GET /v1/stores/{store}/index/files
func (h *IndexHandler) handleListFiles(w http.ResponseWriter, r *http.Request, storeName string) {
	// Parse query parameters
	page := parseInt(r.URL.Query().Get("page"), 1)
	pageSize := parseInt(r.URL.Query().Get("page_size"), 50)

	// Get files from pipeline
	files, total := h.pipeline.ListFiles(storeName, page, pageSize)

	// Convert to response format
	fileList := make([]map[string]interface{}, len(files))
	for i, f := range files {
		fileList[i] = map[string]interface{}{
			"path":       f.Path,
			"hash":       f.Hash,
			"indexed_at": f.IndexedAt,
		}
	}

	totalPages := total / pageSize
	if total%pageSize > 0 {
		totalPages++
	}

	writeIndexJSON(w, http.StatusOK, map[string]interface{}{
		"files":       fileList,
		"total":       total,
		"page":        page,
		"page_size":   pageSize,
		"total_pages": totalPages,
	})
}

// parseInt parses an int from string with default.
func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
