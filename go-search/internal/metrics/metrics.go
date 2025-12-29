package metrics

import (
	"runtime"
	"sync"
	"time"
)

// Metrics holds all application metrics.
type Metrics struct {
	// Search metrics
	SearchRequests *Counter
	SearchLatency  *Histogram
	SearchResults  *Histogram
	SearchErrors   *CounterVec // labels: error_type

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

	startTime time.Time
	mu        sync.RWMutex
}

// New creates a new metrics instance with all metrics initialized.
func New() *Metrics {
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

	if err != nil {
		m.SearchErrors.WithLabels(errorType(err)).Inc()
	}
}

// RecordIndex records indexing metrics.
func (m *Metrics) RecordIndex(docCount, chunkCount int, latencyMs int64, err error) {
	m.IndexedDocuments.Add(int64(docCount))
	m.IndexedChunks.Add(int64(chunkCount))
	m.IndexLatency.Observe(float64(latencyMs))

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
