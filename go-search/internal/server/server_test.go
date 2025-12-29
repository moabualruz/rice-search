package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ricesearch/rice-search/internal/store"
)

func TestParseQdrantURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		wantHost string
		wantPort int
		wantErr  bool
	}{
		{
			name:     "default localhost",
			url:      "http://localhost:6333",
			wantHost: "localhost",
			wantPort: 6334, // gRPC port = HTTP + 1
		},
		{
			name:     "custom host and port",
			url:      "http://qdrant.example.com:7777",
			wantHost: "qdrant.example.com",
			wantPort: 7778,
		},
		{
			name:     "https URL",
			url:      "https://qdrant.cloud:443",
			wantHost: "qdrant.cloud",
			wantPort: 444,
		},
		{
			name:     "no port specified",
			url:      "http://localhost",
			wantHost: "localhost",
			wantPort: 6334, // default 6333 + 1
		},
		{
			name:    "invalid URL",
			url:     "://invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := parseQdrantURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.wantHost {
				t.Errorf("host = %q, want %q", host, tt.wantHost)
			}
			if port != tt.wantPort {
				t.Errorf("port = %d, want %d", port, tt.wantPort)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %q, want %q", cfg.Host, "0.0.0.0")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.Version != "dev" {
		t.Errorf("Version = %q, want %q", cfg.Version, "dev")
	}
	if cfg.ReadTimeout == 0 {
		t.Error("ReadTimeout should not be zero")
	}
	if cfg.WriteTimeout == 0 {
		t.Error("WriteTimeout should not be zero")
	}
	if cfg.ShutdownTimeout == 0 {
		t.Error("ShutdownTimeout should not be zero")
	}
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	w := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

	// Test default status
	if w.status != http.StatusOK {
		t.Errorf("initial status = %d, want %d", w.status, http.StatusOK)
	}

	// Test WriteHeader
	w.WriteHeader(http.StatusNotFound)
	if w.status != http.StatusNotFound {
		t.Errorf("status after WriteHeader = %d, want %d", w.status, http.StatusNotFound)
	}
}

// TestStoreHandler tests the store handler without Qdrant.
func TestStoreHandler(t *testing.T) {
	// Create in-memory store service without Qdrant
	svc, err := store.NewService(nil, store.ServiceConfig{
		EnsureDefault: true,
	})
	if err != nil {
		t.Fatalf("failed to create store service: %v", err)
	}

	handler := NewStoreHandler(svc)
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	t.Run("list stores", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/stores", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp struct {
			Stores []store.Store `json:"stores"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// Should have at least the default store
		if len(resp.Stores) == 0 {
			t.Error("expected at least one store")
		}
	})

	t.Run("create store", func(t *testing.T) {
		body := `{"name": "test-store", "description": "Test store"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/stores", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
		}

		var created store.Store
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if created.Name != "test-store" {
			t.Errorf("name = %q, want %q", created.Name, "test-store")
		}
	})

	t.Run("create duplicate store", func(t *testing.T) {
		body := `{"name": "test-store"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/stores", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusConflict)
		}
	})

	t.Run("get store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/stores/test-store", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var s store.Store
		if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if s.Name != "test-store" {
			t.Errorf("name = %q, want %q", s.Name, "test-store")
		}
	})

	t.Run("get nonexistent store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/stores/nonexistent", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("get store stats", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/stores/test-store/stats", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}
	})

	t.Run("delete store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/stores/test-store", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}

		// Verify deleted
		_, err := svc.GetStore(context.Background(), "test-store")
		if err == nil {
			t.Error("store should have been deleted")
		}
	})

	t.Run("delete default store fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/stores/default", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})

	t.Run("invalid method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/v1/stores", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("invalid request body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/stores", bytes.NewBufferString("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("missing name", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/stores", bytes.NewBufferString(`{"description": "no name"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})
}
