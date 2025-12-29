package metrics

import (
	"runtime"
	"sync"
	"time"
)

// Metrics holds all application metrics.
type Metrics struct {
	// Search metrics
	SearchRequests      *Counter
	SearchLatency       *Histogram
	SearchResults       *Histogram
	SearchErrors        *CounterVec   // labels: error_type
	SearchStageDuration *HistogramVec // labels: store, stage

	// Index metrics
	IndexedDocuments *Counter
	IndexedChunks    *Counter
	IndexLatency     *Histogram
	IndexErrors      *CounterVec // labels: error_type

	// Model metrics
	EmbedRequests           *Counter
	EmbedLatency            *Histogram
	EmbedBatchSize          *Histogram
	RerankRequests          *Counter
	RerankLatency           *Histogram
	QueryUnderstandRequests *Counter
	QueryUnderstandLatency  *Histogram
	SparseEncodeRequests    *Counter
	SparseEncodeLatency     *Histogram

	// Connection metrics (for gRPC/HTTP connections)
	ActiveConnections *Gauge
	ConnectionsTotal  *Counter
	ConnectionErrors  *CounterVec // labels: error_type

	// Store metrics
	StoresTotal    *Gauge
	DocumentsTotal *GaugeVec // labels: store
	ChunksTotal    *GaugeVec // labels: store

	// System metrics
	GoroutineCount *Gauge
	MemoryUsage    *Gauge // in bytes
	Uptime         *Counter

	// Cache metrics
	CacheHits   *CounterVec // labels: type (embed)
	CacheMisses *CounterVec // labels: type (embed)
	CacheSize   *GaugeVec   // labels: type (embed)

	// Bus metrics
	BusEventsPublished *CounterVec   // labels: topic
	BusEventLatency    *HistogramVec // labels: topic
	BusErrors          *CounterVec   // labels: topic

	// HTTP metrics
	HTTPRequests         *CounterVec   // labels: method, path, status
	HTTPDuration         *HistogramVec // labels: method, path
	HTTPRequestsInFlight *Gauge
	HTTPRequestSize      *HistogramVec // labels: method, path

	// Time-series data for charts
	TimeSeries *TimeSeriesData

	// Redis storage (optional)
	redisStorage *RedisStorage

	startTime time.Time
	mu        sync.RWMutex
}

// New creates a new metrics instance with all metrics initialized.
// Uses in-memory storage only.
func New() *Metrics {
	return NewWithConfig("memory", "")
}

// NewWithRedis creates a new metrics instance with Redis persistence.
// Falls back to in-memory if Redis connection fails.
func NewWithRedis(redisURL string) *Metrics {
	return NewWithConfig("redis", redisURL)
}

// NewWithConfig creates a new metrics instance with specified persistence.
// persistence: "memory" or "redis"
// redisURL: Redis URL (only used if persistence = "redis")
func NewWithConfig(persistence, redisURL string) *Metrics {
	var redisStorage *RedisStorage
	var timeSeries *TimeSeriesData

	// Try to initialize Redis if configured
	if persistence == "redis" && redisURL != "" {
		storage, err := NewRedisStorage(redisURL)
		if err != nil {
			// Log warning but continue with in-memory
			// TODO: use logger when available
			println("WARNING: Failed to connect to Redis for metrics persistence:", err.Error())
			println("         Falling back to in-memory metrics")
		} else {
			redisStorage = storage
			timeSeries = NewTimeSeriesDataWithRedis(redisStorage)
		}
	}

	// If Redis not available, use in-memory
	if timeSeries == nil {
		timeSeries = NewTimeSeriesData()
	}

	m := &Metrics{
		// Search metrics
		SearchRequests: NewCounter(
			"rice_search_requests_total",
			"Total number of search requests",
			nil,
		),
		SearchLatency: NewHistogram(
			"rice_search_latency_ms",
			"Search request latency in milliseconds",
			[]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
		),
		SearchResults: NewHistogram(
			"rice_search_results",
			"Number of results per search",
			[]float64{1, 5, 10, 20, 50, 100, 200, 500, 1000},
		),
		SearchErrors: NewCounterVec(
			"rice_search_errors_total",
			"Total number of search errors",
			[]string{"error_type"},
		),
		SearchStageDuration: NewHistogramVec(
			"rice_search_stage_duration_ms",
			"Search stage duration in milliseconds",
			[]string{"store", "stage"},
			[]float64{1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 2500},
		),

		// Index metrics
		IndexedDocuments: NewCounter(
			"rice_indexed_documents_total",
			"Total number of documents indexed",
			nil,
		),
		IndexedChunks: NewCounter(
			"rice_indexed_chunks_total",
			"Total number of chunks indexed",
			nil,
		),
		IndexLatency: NewHistogram(
			"rice_index_latency_ms",
			"Indexing latency in milliseconds per document",
			[]float64{10, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		),
		IndexErrors: NewCounterVec(
			"rice_index_errors_total",
			"Total number of indexing errors",
			[]string{"error_type"},
		),

		// Model metrics
		EmbedRequests: NewCounter(
			"rice_embed_requests_total",
			"Total number of embedding requests",
			nil,
		),
		EmbedLatency: NewHistogram(
			"rice_embed_latency_ms",
			"Embedding generation latency in milliseconds",
			[]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000},
		),
		EmbedBatchSize: NewHistogram(
			"rice_embed_batch_size",
			"Number of texts in embedding batch",
			[]float64{1, 5, 10, 20, 32, 50, 64, 100, 128},
		),
		RerankRequests: NewCounter(
			"rice_rerank_requests_total",
			"Total number of reranking requests",
			nil,
		),
		RerankLatency: NewHistogram(
			"rice_rerank_latency_ms",
			"Reranking latency in milliseconds",
			[]float64{10, 25, 50, 100, 250, 500, 1000, 2500},
		),
		QueryUnderstandRequests: NewCounter(
			"rice_query_understand_requests_total",
			"Total number of query understanding requests",
			nil,
		),
		QueryUnderstandLatency: NewHistogram(
			"rice_query_understand_latency_ms",
			"Query understanding latency in milliseconds",
			[]float64{1, 5, 10, 25, 50, 100, 250},
		),
		SparseEncodeRequests: NewCounter(
			"rice_sparse_encode_requests_total",
			"Total number of sparse encoding requests",
			nil,
		),
		SparseEncodeLatency: NewHistogram(
			"rice_sparse_encode_latency_ms",
			"Sparse encoding latency in milliseconds",
			[]float64{1, 5, 10, 25, 50, 100, 250},
		),

		// Connection metrics
		ActiveConnections: NewGauge(
			"rice_active_connections",
			"Number of active connections",
			nil,
		),
		ConnectionsTotal: NewCounter(
			"rice_connections_total",
			"Total number of connections",
			nil,
		),
		ConnectionErrors: NewCounterVec(
			"rice_connection_errors_total",
			"Total number of connection errors",
			[]string{"error_type"},
		),

		// Store metrics
		StoresTotal: NewGauge(
			"rice_stores_total",
			"Total number of stores",
			nil,
		),
		DocumentsTotal: NewGaugeVec(
			"rice_documents_total",
			"Total number of documents per store",
			[]string{"store"},
		),
		ChunksTotal: NewGaugeVec(
			"rice_chunks_total",
			"Total number of chunks per store",
			[]string{"store"},
		),

		// System metrics
		GoroutineCount: NewGauge(
			"rice_goroutines",
			"Number of goroutines",
			nil,
		),
		MemoryUsage: NewGauge(
			"rice_memory_bytes",
			"Memory usage in bytes",
			nil,
		),
		Uptime: NewCounter(
			"rice_uptime_seconds",
			"Application uptime in seconds",
			nil,
		),

		// Cache metrics
		CacheHits: NewCounterVec(
			"rice_ml_cache_hits_total",
			"Total number of ML cache hits",
			[]string{"type"},
		),
		CacheMisses: NewCounterVec(
			"rice_ml_cache_misses_total",
			"Total number of ML cache misses",
			[]string{"type"},
		),
		CacheSize: NewGaugeVec(
			"rice_ml_cache_size",
			"Current ML cache size",
			[]string{"type"},
		),

		// Bus metrics
		BusEventsPublished: NewCounterVec(
			"rice_bus_events_published_total",
			"Total number of events published to the bus",
			[]string{"topic"},
		),
		BusEventLatency: NewHistogramVec(
			"rice_bus_event_latency_seconds",
			"Event bus latency in seconds",
			[]string{"topic"},
			[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0},
		),
		BusErrors: NewCounterVec(
			"rice_bus_errors_total",
			"Total number of event bus errors",
			[]string{"topic"},
		),

		// HTTP metrics
		HTTPRequests: NewCounterVec(
			"rice_http_requests_total",
			"Total number of HTTP requests",
			[]string{"method", "path", "status"},
		),
		HTTPDuration: NewHistogramVec(
			"rice_http_request_duration_seconds",
			"HTTP request duration in seconds",
			[]string{"method", "path"},
			[]float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0},
		),
		HTTPRequestsInFlight: NewGauge(
			"rice_http_requests_in_flight",
			"Number of HTTP requests currently being processed",
			nil,
		),
		HTTPRequestSize: NewHistogramVec(
			"rice_http_request_size_bytes",
			"HTTP request size in bytes",
			[]string{"method", "path"},
			[]float64{100, 1000, 10000, 100000, 1000000, 10000000},
		),

		// Time-series data for charts
		TimeSeries: timeSeries,

		// Redis storage
		redisStorage: redisStorage,

		startTime: time.Now(),
	}

	// Start background collector for system metrics
	go m.collectSystemMetrics()

	return m
}

// collectSystemMetrics periodically collects system metrics.
func (m *Metrics) collectSystemMetrics() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Update goroutine count
		m.GoroutineCount.Set(float64(runtime.NumGoroutine()))

		// Update memory usage
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		m.MemoryUsage.Set(float64(memStats.Alloc))

		// Update uptime (in seconds)
		m.Uptime.Add(15)
	}
}

// RecordSearch records search metrics.
func (m *Metrics) RecordSearch(latencyMs int64, resultCount int, err error) {
	m.SearchRequests.Inc()
	m.SearchLatency.Observe(float64(latencyMs))
	m.SearchResults.Observe(float64(resultCount))

	// Record time-series data for charts
	if m.TimeSeries != nil {
		m.TimeSeries.RecordSearch(float64(latencyMs))
	}

	if err != nil {
		m.SearchErrors.WithLabels(errorType(err)).Inc()
	}
}

// RecordSearchStage records the duration of a specific search stage.
// stage should be one of: "sparse", "dense", "fusion", "rerank", "postrank"
func (m *Metrics) RecordSearchStage(store, stage string, latencyMs int64) {
	m.SearchStageDuration.WithLabels(store, stage).Observe(float64(latencyMs))
}

// RecordIndex records indexing metrics.
func (m *Metrics) RecordIndex(docCount, chunkCount int, latencyMs int64, err error) {
	m.IndexedDocuments.Add(int64(docCount))
	m.IndexedChunks.Add(int64(chunkCount))
	m.IndexLatency.Observe(float64(latencyMs))

	// Record time-series data for charts
	if m.TimeSeries != nil {
		m.TimeSeries.RecordIndex(docCount)
	}

	if err != nil {
		m.IndexErrors.WithLabels(errorType(err)).Inc()
	}
}

// RecordEmbed records embedding generation metrics.
func (m *Metrics) RecordEmbed(batchSize int, latencyMs int64) {
	m.EmbedRequests.Inc()
	m.EmbedLatency.Observe(float64(latencyMs))
	m.EmbedBatchSize.Observe(float64(batchSize))
}

// RecordRerank records reranking metrics.
func (m *Metrics) RecordRerank(candidateCount int, latencyMs int64) {
	m.RerankRequests.Inc()
	m.RerankLatency.Observe(float64(latencyMs))
}

// RecordSparseEncode records sparse encoding metrics.
func (m *Metrics) RecordSparseEncode(batchSize int, latencyMs int64) {
	m.SparseEncodeRequests.Inc()
	m.SparseEncodeLatency.Observe(float64(latencyMs))
}

// RecordQueryUnderstand records query understanding metrics.
func (m *Metrics) RecordQueryUnderstand(latencyMs int64) {
	m.QueryUnderstandRequests.Inc()
	m.QueryUnderstandLatency.Observe(float64(latencyMs))
}

// UpdateStoreStats updates per-store metrics.
func (m *Metrics) UpdateStoreStats(store string, docCount, chunkCount int64) {
	m.DocumentsTotal.WithLabels(store).Set(float64(docCount))
	m.ChunksTotal.WithLabels(store).Set(float64(chunkCount))
}

// UpdateStoreCount updates the total number of stores.
func (m *Metrics) UpdateStoreCount(count int) {
	m.StoresTotal.Set(float64(count))
}

// IncrementConnection increments active connections.
func (m *Metrics) IncrementConnection() {
	m.ActiveConnections.Inc()
	m.ConnectionsTotal.Inc()
}

// DecrementConnection decrements active connections.
func (m *Metrics) DecrementConnection() {
	m.ActiveConnections.Dec()
}

// RecordConnectionError records a connection error.
func (m *Metrics) RecordConnectionError(err error) {
	m.ConnectionErrors.WithLabels(errorType(err)).Inc()
}

// RecordBusPublish records event bus publish metrics.
func (m *Metrics) RecordBusPublish(topic string, latencyMs int64, err error) {
	m.BusEventsPublished.WithLabels(topic).Inc()

	// Convert milliseconds to seconds for Prometheus convention
	latencySeconds := float64(latencyMs) / 1000.0
	m.BusEventLatency.WithLabels(topic).Observe(latencySeconds)

	if err != nil {
		m.BusErrors.WithLabels(topic).Inc()
	}
}

// RecordCacheHit records a cache hit.
func (m *Metrics) RecordCacheHit(cacheType string) {
	m.CacheHits.WithLabels(cacheType).Inc()
}

// RecordCacheMiss records a cache miss.
func (m *Metrics) RecordCacheMiss(cacheType string) {
	m.CacheMisses.WithLabels(cacheType).Inc()
}

// UpdateCacheSize updates the cache size.
func (m *Metrics) UpdateCacheSize(cacheType string, size int) {
	m.CacheSize.WithLabels(cacheType).Set(float64(size))
}

// RecordHTTP records HTTP request metrics.
// This is called by the HTTP middleware.
func (m *Metrics) RecordHTTP(method, path string, status int, durationSeconds float64, sizeBytes int64) {
	// Normalize path to reduce cardinality
	normalizedPath := normalizePath(path)

	// Record request count with labels
	m.HTTPRequests.WithLabels(method, normalizedPath, statusCode(status)).Inc()

	// Record duration
	m.HTTPDuration.WithLabels(method, normalizedPath).Observe(durationSeconds)

	// Record request size
	if sizeBytes > 0 {
		m.HTTPRequestSize.WithLabels(method, normalizedPath).Observe(float64(sizeBytes))
	}
}

// errorType extracts error type from error.
func errorType(err error) string {
	if err == nil {
		return "unknown"
	}
	// Simple error type extraction - could be enhanced
	return "generic"
}

// Reset resets all metrics to zero (useful for testing).
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Reset counters
	m.SearchRequests.Reset()
	m.IndexedDocuments.Reset()
	m.IndexedChunks.Reset()
	m.EmbedRequests.Reset()
	m.RerankRequests.Reset()
	m.QueryUnderstandRequests.Reset()
	m.SparseEncodeRequests.Reset()
	m.ConnectionsTotal.Reset()
	m.Uptime.Reset()

	// Reset gauges
	m.ActiveConnections.Set(0)
	m.StoresTotal.Set(0)
	m.GoroutineCount.Set(0)
	m.MemoryUsage.Set(0)

	m.startTime = time.Now()
}

// Close closes the metrics instance and releases resources.
// Must be called when shutting down if Redis is used.
func (m *Metrics) Close() error {
	if m.redisStorage != nil {
		return m.redisStorage.Close()
	}
	return nil
}

// IsRedisPersisted returns true if metrics are persisted to Redis.
func (m *Metrics) IsRedisPersisted() bool {
	return m.redisStorage != nil
}
