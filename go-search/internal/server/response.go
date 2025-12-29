// Package server provides HTTP server utilities.
package server

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

// ResponseMeta contains metadata for API responses.
type ResponseMeta struct {
	RequestID string `json:"request_id"`
	LatencyMS int64  `json:"latency_ms"`
	Timestamp string `json:"timestamp"`
}

// WrappedResponse wraps API responses with data and metadata.
type WrappedResponse struct {
	Data interface{}  `json:"data"`
	Meta ResponseMeta `json:"meta"`
}

// responseWrapper captures response body for wrapping.
type responseWrapper struct {
	http.ResponseWriter
	body       *bytes.Buffer
	statusCode int
	wroteBody  bool
}

func newResponseWrapper(w http.ResponseWriter) *responseWrapper {
	return &responseWrapper{
		ResponseWriter: w,
		body:           &bytes.Buffer{},
		statusCode:     http.StatusOK,
	}
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *responseWrapper) Write(b []byte) (int, error) {
	rw.wroteBody = true
	return rw.body.Write(b)
}

// ResponseWrapperMiddleware wraps JSON responses with data/meta structure.
// Only applies to /v1/* endpoints that return JSON.
func ResponseWrapperMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only wrap /v1/* API endpoints
		if len(r.URL.Path) < 3 || r.URL.Path[:3] != "/v1" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip certain endpoints that shouldn't be wrapped
		skipPaths := map[string]bool{
			"/v1/version": true,
			"/v1/health":  true,
		}
		if skipPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		requestID := GenerateRequestID()

		// Wrap response writer
		rw := newResponseWrapper(w)
		next.ServeHTTP(rw, r)

		latencyMS := time.Since(start).Milliseconds()

		// If no body written or error status, just write original
		if !rw.wroteBody || rw.statusCode >= 400 {
			w.WriteHeader(rw.statusCode)
			w.Write(rw.body.Bytes())
			return
		}

		// Try to parse as JSON
		var data interface{}
		if err := json.Unmarshal(rw.body.Bytes(), &data); err != nil {
			// Not JSON, return as-is
			w.WriteHeader(rw.statusCode)
			w.Write(rw.body.Bytes())
			return
		}

		// Wrap response
		wrapped := WrappedResponse{
			Data: data,
			Meta: ResponseMeta{
				RequestID: requestID,
				LatencyMS: latencyMS,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Request-ID", requestID)
		w.WriteHeader(rw.statusCode)
		json.NewEncoder(w).Encode(wrapped)
	})
}

// GenerateRequestID generates a short unique request ID.
func GenerateRequestID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
