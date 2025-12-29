package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ricesearch/rice-search/internal/store"
)

// StoreHandler handles store-related HTTP requests.
type StoreHandler struct {
	svc *store.Service
}

// NewStoreHandler creates a new store handler.
func NewStoreHandler(svc *store.Service) *StoreHandler {
	return &StoreHandler{svc: svc}
}

// writeJSON writes a JSON response.
func writeStoreJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Log encoding error - can't return to client after headers written
		// In production, this would use a proper logger
		_ = err // Encoding error after response started
	}
}

// writeError writes an error response.
func writeStoreError(w http.ResponseWriter, status int, message string) {
	writeStoreJSON(w, status, map[string]string{"error": message})
}

// RegisterRoutes registers store routes.
func (h *StoreHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/stores", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.handleList(w, r)
		case http.MethodPost:
			h.handleCreate(w, r)
		default:
			writeStoreError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})

	mux.HandleFunc("/v1/stores/", func(w http.ResponseWriter, r *http.Request) {
		// Extract store name from path
		path := strings.TrimPrefix(r.URL.Path, "/v1/stores/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			writeStoreError(w, http.StatusNotFound, "not found")
			return
		}

		storeName := parts[0]
		subPath := ""
		if len(parts) > 1 {
			subPath = parts[1]
		}

		// Route based on subpath
		switch {
		case subPath == "" || subPath == "/":
			switch r.Method {
			case http.MethodGet:
				h.handleGet(w, r, storeName)
			case http.MethodDelete:
				h.handleDelete(w, r, storeName)
			default:
				writeStoreError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		case subPath == "stats":
			if r.Method == http.MethodGet {
				h.handleStats(w, r, storeName)
			} else {
				writeStoreError(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		default:
			// Let other handlers handle this (search, index)
			// Return 404 only if nothing else handles it
			writeStoreError(w, http.StatusNotFound, "not found")
		}
	})
}

// handleList handles GET /v1/stores
func (h *StoreHandler) handleList(w http.ResponseWriter, r *http.Request) {
	stores, err := h.svc.ListStores(r.Context())
	if err != nil {
		writeStoreError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeStoreJSON(w, http.StatusOK, map[string]interface{}{
		"stores": stores,
	})
}

// handleCreate handles POST /v1/stores
func (h *StoreHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeStoreError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeStoreError(w, http.StatusBadRequest, "name is required")
		return
	}

	newStore := store.NewStore(req.Name)
	newStore.Description = req.Description

	if err := h.svc.CreateStore(r.Context(), newStore); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeStoreError(w, http.StatusConflict, err.Error())
		} else {
			writeStoreError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeStoreJSON(w, http.StatusCreated, newStore)
}

// handleGet handles GET /v1/stores/{name}
func (h *StoreHandler) handleGet(w http.ResponseWriter, r *http.Request, name string) {
	s, err := h.svc.GetStore(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeStoreError(w, http.StatusNotFound, err.Error())
		} else {
			writeStoreError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeStoreJSON(w, http.StatusOK, s)
}

// handleDelete handles DELETE /v1/stores/{name}
func (h *StoreHandler) handleDelete(w http.ResponseWriter, r *http.Request, name string) {
	if err := h.svc.DeleteStore(r.Context(), name); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeStoreError(w, http.StatusNotFound, err.Error())
		} else {
			writeStoreError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleStats handles GET /v1/stores/{name}/stats
func (h *StoreHandler) handleStats(w http.ResponseWriter, r *http.Request, name string) {
	stats, err := h.svc.GetStoreStats(r.Context(), name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeStoreError(w, http.StatusNotFound, err.Error())
		} else {
			writeStoreError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeStoreJSON(w, http.StatusOK, stats)
}
