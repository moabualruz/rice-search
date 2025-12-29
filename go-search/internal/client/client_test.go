package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:8080")
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 30*time.Second)
	}
}

func TestClientNew(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		c := New(Config{})
		if c.baseURL != "http://localhost:8080" {
			t.Errorf("baseURL = %q, want %q", c.baseURL, "http://localhost:8080")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		c := New(Config{
			BaseURL: "http://custom:9000",
			Timeout: 60 * time.Second,
		})
		if c.baseURL != "http://custom:9000" {
			t.Errorf("baseURL = %q, want %q", c.baseURL, "http://custom:9000")
		}
	})
}

func TestClientHealth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/healthz")
		}
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want %q", r.Method, http.MethodGet)
		}

		if err := json.NewEncoder(w).Encode(HealthResponse{
			Status:  "ok",
			Version: "1.0.0",
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	resp, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
	if resp.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", resp.Version, "1.0.0")
	}
}

func TestClientListStores(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/stores" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/stores")
		}

		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"stores": []Store{
				{Name: "default", CreatedAt: "2025-01-01T00:00:00Z"},
				{Name: "test", Description: "Test store", CreatedAt: "2025-01-02T00:00:00Z"},
			},
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	stores, err := c.ListStores(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stores) != 2 {
		t.Errorf("len(stores) = %d, want %d", len(stores), 2)
	}
	if stores[0].Name != "default" {
		t.Errorf("stores[0].Name = %q, want %q", stores[0].Name, "default")
	}
}

func TestClientCreateStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}

		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req["name"] != "mystore" {
			t.Errorf("name = %q, want %q", req["name"], "mystore")
		}

		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(Store{
			Name:        "mystore",
			Description: req["description"],
			CreatedAt:   "2025-01-01T00:00:00Z",
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	store, err := c.CreateStore(context.Background(), "mystore", "My store")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.Name != "mystore" {
		t.Errorf("Name = %q, want %q", store.Name, "mystore")
	}
}

func TestClientDeleteStore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want %q", r.Method, http.MethodDelete)
		}
		if r.URL.Path != "/v1/stores/mystore" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/stores/mystore")
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	err := c.DeleteStore(context.Background(), "mystore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/stores/default/search" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/stores/default/search")
		}

		var req SearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if req.Query != "authentication" {
			t.Errorf("Query = %q, want %q", req.Query, "authentication")
		}

		if err := json.NewEncoder(w).Encode(SearchResponse{
			Query: req.Query,
			Store: "default",
			Results: []SearchResult{
				{
					ID:        "chunk1",
					Path:      "src/auth.go",
					Language:  "go",
					StartLine: 10,
					EndLine:   25,
					Score:     0.95,
				},
			},
			Total: 1,
			Metadata: SearchMetadata{
				SearchTimeMs:     45,
				EmbedTimeMs:      10,
				RetrievalTimeMs:  30,
				RerankingApplied: true,
			},
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	resp, err := c.Search(context.Background(), "default", SearchRequest{
		Query: "authentication",
		TopK:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Query != "authentication" {
		t.Errorf("Query = %q, want %q", resp.Query, "authentication")
	}
	if len(resp.Results) != 1 {
		t.Errorf("len(Results) = %d, want %d", len(resp.Results), 1)
	}
	if resp.Results[0].Path != "src/auth.go" {
		t.Errorf("Results[0].Path = %q, want %q", resp.Results[0].Path, "src/auth.go")
	}
}

func TestClientIndex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want %q", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/stores/default/index" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/v1/stores/default/index")
		}

		var req IndexRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
		}

		if len(req.Files) != 2 {
			t.Errorf("len(Files) = %d, want %d", len(req.Files), 2)
		}

		if err := json.NewEncoder(w).Encode(IndexResult{
			Store:       "default",
			Indexed:     2,
			Skipped:     0,
			Failed:      0,
			ChunksTotal: 5,
			DurationMs:  100,
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	result, err := c.Index(context.Background(), "default", IndexRequest{
		Files: []IndexFile{
			{Path: "main.go", Content: "package main"},
			{Path: "util.go", Content: "package util"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Indexed != 2 {
		t.Errorf("Indexed = %d, want %d", result.Indexed, 2)
	}
	if result.ChunksTotal != 5 {
		t.Errorf("ChunksTotal = %d, want %d", result.ChunksTotal, 5)
	}
}

func TestClientAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if err := json.NewEncoder(w).Encode(APIError{
			Code:    "NOT_FOUND",
			Message: "store not found",
		}); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := New(Config{BaseURL: server.URL})
	_, err := c.GetStore(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("Code = %q, want %q", apiErr.Code, "NOT_FOUND")
	}
}

func TestClientConnectionError(t *testing.T) {
	c := New(Config{
		BaseURL: "http://localhost:99999", // Invalid port
		Timeout: 1 * time.Second,
	})

	_, err := c.Health(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAPIErrorString(t *testing.T) {
	err := &APIError{
		Code:    "TEST_ERROR",
		Message: "test message",
	}

	expected := "TEST_ERROR: test message"
	if err.Error() != expected {
		t.Errorf("Error() = %q, want %q", err.Error(), expected)
	}
}
