package search

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultTopK <= 0 {
		t.Error("DefaultTopK should be positive")
	}
	if cfg.PrefetchMultiplier <= 0 {
		t.Error("PrefetchMultiplier should be positive")
	}
	if cfg.RerankTopK <= 0 {
		t.Error("RerankTopK should be positive")
	}
}

func TestGroupByFile(t *testing.T) {
	results := []Result{
		{Path: "a.go", Score: 0.9},
		{Path: "a.go", Score: 0.8},
		{Path: "a.go", Score: 0.7},
		{Path: "a.go", Score: 0.6},
		{Path: "b.go", Score: 0.95},
		{Path: "b.go", Score: 0.85},
		{Path: "c.go", Score: 0.5},
	}

	// Group with max 2 per file
	grouped := GroupByFile(results, 2)

	// Count per file
	counts := make(map[string]int)
	for _, r := range grouped {
		counts[r.Path]++
	}

	if counts["a.go"] > 2 {
		t.Errorf("expected max 2 from a.go, got %d", counts["a.go"])
	}
	if counts["b.go"] > 2 {
		t.Errorf("expected max 2 from b.go, got %d", counts["b.go"])
	}
	if counts["c.go"] != 1 {
		t.Errorf("expected 1 from c.go, got %d", counts["c.go"])
	}

	// Verify top scores are kept
	for _, r := range grouped {
		if r.Path == "a.go" && r.Score < 0.7 {
			t.Error("expected top scores to be kept for a.go")
		}
	}
}

func TestExtractStoreFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/v1/stores/default/search", "default"},
		{"/v1/stores/mystore/search/dense", "mystore"},
		{"/v1/stores/test-store/index", "test-store"},
		{"/v1/stores/", ""},
		{"/other/path", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := extractStoreFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("extractStoreFromPath(%s) = %s, expected %s", tt.path, result, tt.expected)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}

	writeJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "value") {
		t.Errorf("expected body to contain 'value', got %s", body)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	writeError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test error") {
		t.Errorf("expected body to contain 'test error', got %s", body)
	}
}

func TestCORSMiddleware(t *testing.T) {
	handler := CORSMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Test OPTIONS request
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/test", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for OPTIONS, got %d", w.Code)
	}

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin *, got %s", origin)
	}

	// Test regular request
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/test", nil)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET, got %d", w.Code)
	}
}

func TestHealthStatus(t *testing.T) {
	// Test with nil checker (no dependencies)
	checker := NewHealthChecker(nil, nil)
	status := checker.Check(nil)

	// Both components should be unhealthy since they're nil
	if status.Components["ml"].Status != "unhealthy" {
		t.Errorf("expected ML status unhealthy, got %s", status.Components["ml"].Status)
	}
	if status.Components["qdrant"].Status != "unhealthy" {
		t.Errorf("expected Qdrant status unhealthy, got %s", status.Components["qdrant"].Status)
	}

	// Overall status should be unhealthy
	if status.Status != "unhealthy" {
		t.Errorf("expected overall status unhealthy, got %s", status.Status)
	}
}

func TestHealthHandler(t *testing.T) {
	checker := NewHealthChecker(nil, nil)
	handler := NewHealthHandler(checker, "1.0.0")

	// Test /healthz
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.HandleHealth(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for /healthz, got %d", w.Code)
	}

	// Test /v1/version
	w = httptest.NewRecorder()
	r = httptest.NewRequest(http.MethodGet, "/v1/version", nil)
	handler.HandleVersion(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for /v1/version, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "1.0.0") {
		t.Errorf("expected body to contain version, got %s", body)
	}
}

func TestSearchRequest_Defaults(t *testing.T) {
	// Test that Request struct can be created
	req := Request{
		Query: "test query",
		Store: "default",
	}

	if req.Query != "test query" {
		t.Errorf("expected query 'test query', got %s", req.Query)
	}
	if req.TopK != 0 {
		t.Errorf("expected TopK 0 (default), got %d", req.TopK)
	}
}

func TestResult_Fields(t *testing.T) {
	result := Result{
		ID:        "chunk-123",
		Path:      "src/main.go",
		Language:  "go",
		StartLine: 10,
		EndLine:   20,
		Content:   "func main() {}",
		Symbols:   []string{"main"},
		Score:     0.95,
	}

	if result.ID != "chunk-123" {
		t.Errorf("unexpected ID: %s", result.ID)
	}
	if result.Language != "go" {
		t.Errorf("unexpected Language: %s", result.Language)
	}
	if result.Score != 0.95 {
		t.Errorf("unexpected Score: %f", result.Score)
	}
}

func TestResponse_Fields(t *testing.T) {
	resp := Response{
		Query:   "test",
		Store:   "default",
		Results: []Result{},
		Total:   0,
		Metadata: SearchMetadata{
			SearchTimeMs:     100,
			EmbedTimeMs:      10,
			RetrievalTimeMs:  50,
			RerankingApplied: false,
		},
	}

	if resp.Query != "test" {
		t.Errorf("unexpected Query: %s", resp.Query)
	}
	if resp.Metadata.SearchTimeMs != 100 {
		t.Errorf("unexpected SearchTimeMs: %d", resp.Metadata.SearchTimeMs)
	}
}

func TestFilter_Fields(t *testing.T) {
	filter := Filter{
		PathPrefix: "src/",
		Languages:  []string{"go", "python"},
	}

	if filter.PathPrefix != "src/" {
		t.Errorf("unexpected PathPrefix: %s", filter.PathPrefix)
	}
	if len(filter.Languages) != 2 {
		t.Errorf("unexpected Languages length: %d", len(filter.Languages))
	}
}
