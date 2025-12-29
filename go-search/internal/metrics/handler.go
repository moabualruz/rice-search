package metrics

import (
	"net/http"
)

// Handler returns an HTTP handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only allow GET requests
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Set content type
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Write metrics
		metrics := m.PrometheusFormat()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(metrics))
	})
}

// ServeHTTP implements http.Handler interface.
func (m *Metrics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.Handler().ServeHTTP(w, r)
}
