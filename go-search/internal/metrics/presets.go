package metrics

import (
	"time"
)

// MetricPreset defines a predefined metric query for the UI.
type MetricPreset struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Metrics     []string `json:"metrics"`
	ChartType   string   `json:"chart_type"` // line, bar, gauge, table, pie
	Filters     []string `json:"filters"`    // available filter options
	TimeRange   string   `json:"time_range"` // default time range
}

// DefaultPresets returns the default metric presets for the UI.
var DefaultPresets = []MetricPreset{
	{
		ID:          "search_overview",
		Name:        "Search Overview",
		Description: "Overall search performance metrics",
		Metrics: []string{
			"rice_search_requests_total",
			"rice_search_latency_ms",
			"rice_search_results",
		},
		ChartType: "line",
		Filters:   []string{"time_range", "store"},
		TimeRange: "1h",
	},
	{
		ID:          "search_latency",
		Name:        "Search Latency Distribution",
		Description: "Histogram of search latency over time",
		Metrics: []string{
			"rice_search_latency_ms_bucket",
			"rice_search_latency_ms_sum",
			"rice_search_latency_ms_count",
		},
		ChartType: "bar",
		Filters:   []string{"time_range", "percentile"},
		TimeRange: "1h",
	},
	{
		ID:          "index_status",
		Name:        "Index Status",
		Description: "Current indexing statistics",
		Metrics: []string{
			"rice_indexed_documents_total",
			"rice_indexed_chunks_total",
			"rice_index_errors_total",
		},
		ChartType: "table",
		Filters:   []string{"store"},
		TimeRange: "all",
	},
	{
		ID:          "index_performance",
		Name:        "Indexing Performance",
		Description: "Indexing throughput and latency",
		Metrics: []string{
			"rice_indexed_documents_total",
			"rice_indexed_chunks_total",
			"rice_index_latency_ms",
		},
		ChartType: "line",
		Filters:   []string{"time_range", "store"},
		TimeRange: "1h",
	},
	{
		ID:          "ml_performance",
		Name:        "ML Model Performance",
		Description: "Embedding and reranking metrics",
		Metrics: []string{
			"rice_embed_requests_total",
			"rice_embed_latency_ms",
			"rice_rerank_requests_total",
			"rice_rerank_latency_ms",
		},
		ChartType: "line",
		Filters:   []string{"time_range", "model"},
		TimeRange: "1h",
	},
	{
		ID:          "ml_throughput",
		Name:        "ML Throughput",
		Description: "ML request rates and batch sizes",
		Metrics: []string{
			"rice_embed_requests_total",
			"rice_embed_batch_size",
			"rice_rerank_requests_total",
			"rice_sparse_encode_requests_total",
		},
		ChartType: "line",
		Filters:   []string{"time_range"},
		TimeRange: "1h",
	},
	{
		ID:          "system_health",
		Name:        "System Health",
		Description: "System resource usage",
		Metrics: []string{
			"rice_goroutines",
			"rice_memory_bytes",
			"rice_active_connections",
		},
		ChartType: "line",
		Filters:   []string{"time_range"},
		TimeRange: "1h",
	},
	{
		ID:          "store_stats",
		Name:        "Store Statistics",
		Description: "Per-store document and chunk counts",
		Metrics: []string{
			"rice_stores_total",
			"rice_documents_total",
			"rice_chunks_total",
		},
		ChartType: "table",
		Filters:   []string{"store"},
		TimeRange: "all",
	},
	{
		ID:          "error_rates",
		Name:        "Error Rates",
		Description: "Error counts by type",
		Metrics: []string{
			"rice_search_errors_total",
			"rice_index_errors_total",
			"rice_connection_errors_total",
		},
		ChartType: "bar",
		Filters:   []string{"time_range", "error_type"},
		TimeRange: "1h",
	},
	{
		ID:          "connection_stats",
		Name:        "Connection Statistics",
		Description: "Active and total connections",
		Metrics: []string{
			"rice_active_connections",
			"rice_connections_total",
			"rice_connection_errors_total",
		},
		ChartType: "line",
		Filters:   []string{"time_range"},
		TimeRange: "1h",
	},
	{
		ID:          "top_stores",
		Name:        "Top Stores by Usage",
		Description: "Most queried stores",
		Metrics: []string{
			"rice_documents_total",
			"rice_chunks_total",
		},
		ChartType: "pie",
		Filters:   []string{"limit"},
		TimeRange: "all",
	},
	{
		ID:          "latency_percentiles",
		Name:        "Latency Percentiles",
		Description: "P50, P95, P99 latencies for search and indexing",
		Metrics: []string{
			"rice_search_latency_ms",
			"rice_index_latency_ms",
			"rice_embed_latency_ms",
			"rice_rerank_latency_ms",
		},
		ChartType: "gauge",
		Filters:   []string{"time_range", "percentile"},
		TimeRange: "1h",
	},
	{
		ID:          "uptime_status",
		Name:        "Uptime & Availability",
		Description: "System uptime and request success rates",
		Metrics: []string{
			"rice_uptime_seconds",
			"rice_search_requests_total",
			"rice_search_errors_total",
		},
		ChartType: "table",
		Filters:   []string{},
		TimeRange: "all",
	},
}

// GetPreset returns a preset by ID.
func GetPreset(id string) *MetricPreset {
	for i := range DefaultPresets {
		if DefaultPresets[i].ID == id {
			return &DefaultPresets[i]
		}
	}
	return nil
}

// GetPresetsByCategory returns presets grouped by category.
func GetPresetsByCategory() map[string][]MetricPreset {
	categories := map[string][]MetricPreset{
		"search": {
			DefaultPresets[0], // search_overview
			DefaultPresets[1], // search_latency
		},
		"indexing": {
			DefaultPresets[2], // index_status
			DefaultPresets[3], // index_performance
		},
		"ml": {
			DefaultPresets[4], // ml_performance
			DefaultPresets[5], // ml_throughput
		},
		"system": {
			DefaultPresets[6],  // system_health
			DefaultPresets[9],  // connection_stats
			DefaultPresets[12], // uptime_status
		},
		"stores": {
			DefaultPresets[7],  // store_stats
			DefaultPresets[10], // top_stores
		},
		"errors": {
			DefaultPresets[8], // error_rates
		},
		"performance": {
			DefaultPresets[11], // latency_percentiles
		},
	}
	return categories
}

// GetAllPresets returns all available presets.
func GetAllPresets() []MetricPreset {
	return DefaultPresets
}

// MetricQuery represents a query for specific metrics.
type MetricQuery struct {
	PresetID    string            `json:"preset_id,omitempty"`
	Metrics     []string          `json:"metrics"`
	TimeRange   string            `json:"time_range"`  // 5m, 15m, 1h, 6h, 24h, 7d, 30d, all
	Filters     map[string]string `json:"filters"`     // e.g., {"store": "default", "error_type": "timeout"}
	Aggregation string            `json:"aggregation"` // sum, avg, min, max, p50, p95, p99
	GroupBy     []string          `json:"group_by"`    // e.g., ["store", "error_type"]
}

// MetricQueryResult represents the result of a metric query.
type MetricQueryResult struct {
	Query     MetricQuery            `json:"query"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
	Series    []MetricSeries         `json:"series,omitempty"`
	Summary   map[string]float64     `json:"summary,omitempty"`
}

// MetricSeries represents a time series of metric values.
type MetricSeries struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Points []MetricPoint     `json:"points"`
}

// MetricPoint represents a single data point in a time series.
type MetricPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// ExecuteQuery executes a metric query and returns results.
// This is a placeholder - actual implementation would query time-series data.
func (m *Metrics) ExecuteQuery(query MetricQuery) (*MetricQueryResult, error) {
	result := &MetricQueryResult{
		Query:     query,
		Timestamp: time.Now().Unix(),
		Data:      make(map[string]interface{}),
		Summary:   make(map[string]float64),
	}

	// If preset ID is provided, use preset metrics
	if query.PresetID != "" {
		preset := GetPreset(query.PresetID)
		if preset != nil {
			query.Metrics = preset.Metrics
		}
	}

	// Collect current values for requested metrics
	// In a real implementation, this would query historical data
	for _, metricName := range query.Metrics {
		result.Data[metricName] = m.getCurrentValue(metricName)
	}

	return result, nil
}

// getCurrentValue gets the current value of a metric by name.
func (m *Metrics) getCurrentValue(name string) interface{} {
	switch name {
	case "rice_search_requests_total":
		return m.SearchRequests.Value()
	case "rice_indexed_documents_total":
		return m.IndexedDocuments.Value()
	case "rice_indexed_chunks_total":
		return m.IndexedChunks.Value()
	case "rice_embed_requests_total":
		return m.EmbedRequests.Value()
	case "rice_rerank_requests_total":
		return m.RerankRequests.Value()
	case "rice_active_connections":
		return m.ActiveConnections.Value()
	case "rice_connections_total":
		return m.ConnectionsTotal.Value()
	case "rice_stores_total":
		return m.StoresTotal.Value()
	case "rice_goroutines":
		return m.GoroutineCount.Value()
	case "rice_memory_bytes":
		return m.MemoryUsage.Value()
	case "rice_uptime_seconds":
		return m.Uptime.Value()
	default:
		return nil
	}
}
