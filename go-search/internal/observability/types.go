package observability

import "time"

// QueryLogEntry represents a logged search query.
type QueryLogEntry struct {
	Timestamp       time.Time `json:"timestamp"`
	Store           string    `json:"store"`
	Query           string    `json:"query"`
	Intent          string    `json:"intent"`
	Strategy        string    `json:"strategy"`
	Difficulty      string    `json:"difficulty"`
	Confidence      float32   `json:"confidence"`
	ResultCount     int       `json:"result_count"`
	LatencyMs       int64     `json:"latency_ms"`
	RerankEnabled   bool      `json:"rerank_enabled"`
	RerankLatencyMs int64     `json:"rerank_latency_ms"`
}
