package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPMiddleware(t *testing.T) {
	m := New()

	// Create a test handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Wrap with middleware
	wrapped := HTTPMiddleware(m, handler)

	// Create test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Execute request
	wrapped.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify metrics were recorded
	if m.HTTPRequests == nil {
		t.Fatal("HTTPRequests metric is nil")
	}

	if m.HTTPRequestsInFlight.Value() != 0 {
		t.Errorf("expected in-flight requests to be 0, got %f", m.HTTPRequestsInFlight.Value())
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "static root",
			input:    "/",
			expected: "/",
		},
		{
			name:     "health endpoint",
			input:    "/healthz",
			expected: "/healthz",
		},
		{
			name:     "store with name",
			input:    "/v1/stores/default/search",
			expected: "/v1/stores/{store}/search",
		},
		{
			name:     "admin model with id",
			input:    "/admin/models/abc123/download",
			expected: "/admin/models/{id}/download",
		},
		{
			name:     "file path",
			input:    "/files/src/main.go",
			expected: "/files/{path}",
		},
		{
			name:     "store files",
			input:    "/stores/mystore/files/src/main.go",
			expected: "/stores/{name}/files/{path}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePath(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStatusCode(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "200"},
		{201, "201"},
		{404, "404"},
		{500, "500"},
		{503, "503"},
		{150, "1xx"},
		{250, "2xx"},
		{350, "3xx"},
		{450, "4xx"},
		{550, "5xx"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := statusCode(tt.code)
			if result != tt.expected {
				t.Errorf("statusCode(%d) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	wrapped := &responseWriter{
		ResponseWriter: rec,
		statusCode:     http.StatusOK,
	}

	// Test WriteHeader
	wrapped.WriteHeader(http.StatusCreated)
	if wrapped.statusCode != http.StatusCreated {
		t.Errorf("expected status 201, got %d", wrapped.statusCode)
	}

	// Test Write auto-calls WriteHeader
	wrapped2 := &responseWriter{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     http.StatusOK,
	}
	wrapped2.Write([]byte("test"))
	if !wrapped2.written {
		t.Error("expected written flag to be true")
	}
	if wrapped2.statusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", wrapped2.statusCode)
	}
}

func BenchmarkHTTPMiddleware(b *testing.B) {
	m := New()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := HTTPMiddleware(m, handler)

	req := httptest.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}
}

func BenchmarkNormalizePath(b *testing.B) {
	paths := []string{
		"/v1/stores/default/search",
		"/admin/models/abc123/download",
		"/files/src/main.go",
		"/healthz",
		"/stores/mystore/files/src/utils/helper.go",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			_ = normalizePath(path)
		}
	}
}
