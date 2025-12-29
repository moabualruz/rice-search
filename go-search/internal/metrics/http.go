package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

// HTTPMiddleware wraps an HTTP handler to collect metrics.
// It records request count, duration, size, and tracks in-flight requests.
//
// Usage:
//
//	handler := metrics.HTTPMiddleware(metrics, http.HandlerFunc(myHandler))
//	http.Handle("/api/", handler)
func HTTPMiddleware(m *Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Increment in-flight requests
		m.HTTPRequestsInFlight.Inc()
		defer m.HTTPRequestsInFlight.Dec()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // Default to 200
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// Calculate metrics
		duration := time.Since(start).Seconds()
		size := r.ContentLength
		if size < 0 {
			size = 0
		}

		// Record metrics
		m.RecordHTTP(r.Method, r.URL.Path, wrapped.statusCode, duration, size)
	})
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code and calls the underlying WriteHeader.
func (w *responseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write ensures status code is set before writing.
func (w *responseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.WriteHeader(w.statusCode)
	}
	return w.ResponseWriter.Write(b)
}

// normalizePath normalizes HTTP paths to reduce cardinality for metrics.
// Replaces path parameters with placeholders like {id}, {store}, {name}.
//
// Examples:
//   - /v1/stores/default/search -> /v1/stores/{store}/search
//   - /admin/models/abc123/download -> /admin/models/{id}/download
//   - /stores/mystore/files/src/main.go -> /stores/{name}/files/{path}
func normalizePath(path string) string {
	// Fast path: common static routes
	switch path {
	case "/", "/healthz", "/readyz", "/metrics":
		return path
	case "/search", "/stores", "/files", "/stats", "/admin":
		return path
	}

	// Handle path normalization
	normalized := path

	// Pattern: /v1/stores/{store}/...
	normalized = replacePattern(normalized, `/v1/stores/[^/]+/`, "/v1/stores/{store}/")

	// Pattern: /stores/{name}/...
	normalized = replacePattern(normalized, `^/stores/[^/]+/`, "/stores/{name}/")

	// Pattern: /admin/models/{id}/...
	normalized = replacePattern(normalized, `/admin/models/[^/]+/`, "/admin/models/{id}/")

	// Pattern: /admin/mappers/{id}/...
	normalized = replacePattern(normalized, `/admin/mappers/[^/]+/`, "/admin/mappers/{id}/")

	// Pattern: /admin/connections/{id}/...
	normalized = replacePattern(normalized, `/admin/connections/[^/]+/`, "/admin/connections/{id}/")

	// Pattern: /admin/stores/{name}
	normalized = replacePattern(normalized, `/admin/stores/[^/]+$`, "/admin/stores/{name}")

	// Pattern: /admin/settings/rollback/{version}
	normalized = replacePattern(normalized, `/admin/settings/rollback/[^/]+`, "/admin/settings/rollback/{version}")

	// Pattern: /files/{path...} - catch-all for remaining path segments
	normalized = replacePattern(normalized, `/files/.+`, "/files/{path}")

	// Pattern: /stores/{name}/files/{path...}
	normalized = replacePattern(normalized, `/stores/[^/]+/files/.+`, "/stores/{name}/files/{path}")

	return normalized
}

// replacePattern replaces regex pattern in path.
func replacePattern(path, pattern, replacement string) string {
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllString(path, replacement)
}

// statusCode converts HTTP status code to string for metric label.
// Groups codes into categories to reduce cardinality.
func statusCode(code int) string {
	// Fast path: common status codes
	switch code {
	case 200:
		return "200"
	case 201:
		return "201"
	case 204:
		return "204"
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 405:
		return "405"
	case 500:
		return "500"
	case 502:
		return "502"
	case 503:
		return "503"
	}

	// Group by category (reduces cardinality while preserving information)
	if code >= 100 && code < 200 {
		return "1xx"
	}
	if code >= 200 && code < 300 {
		return "2xx"
	}
	if code >= 300 && code < 400 {
		return "3xx"
	}
	if code >= 400 && code < 500 {
		return "4xx"
	}
	if code >= 500 && code < 600 {
		return "5xx"
	}

	// Fallback for invalid codes
	return strconv.Itoa(code)
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it.
func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it.
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}
