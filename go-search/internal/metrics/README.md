# Metrics Package

Native Prometheus-compatible metrics implementation for Rice Search (Go edition).

## Overview

This package provides a lightweight, dependency-free metrics system that exports metrics in Prometheus text exposition format. No external Prometheus client library required - keeping the binary small and avoiding dependencies.

## Features

- **Native Implementation**: Pure Go metrics types (Counter, Gauge, Histogram, GaugeVec, CounterVec)
- **Prometheus Compatible**: Exports in standard Prometheus text format
- **Thread-Safe**: All metric operations use atomic operations or mutexes
- **Event Bus Integration**: Automatically updates metrics from event bus
- **Metric Presets**: Predefined queries for common observability scenarios
- **Zero Dependencies**: No external metrics libraries

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Metrics Package                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Types      │  │   Metrics    │  │  Prometheus  │      │
│  │   (types.go) │  │ (metrics.go) │  │(prometheus.go│      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                 │                   │              │
│         └─────────────────┴───────────────────┘              │
│                           │                                  │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Events     │  │  Collector   │  │   Handler    │      │
│  │  (events.go) │  │(collector.go)│  │ (handler.go) │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
│         │                 │                   │              │
│  ┌──────────────────────────────────────────────────┐      │
│  │              Presets (presets.go)                 │      │
│  └──────────────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────────────┘
         │                  │                  │
         ▼                  ▼                  ▼
    Event Bus          Services          HTTP Server
```

## Metric Types

### Counter
Monotonically increasing counter. Can only increase.

```go
counter := NewCounter("requests_total", "Total requests", nil)
counter.Inc()
counter.Add(5)
```

### Gauge
Value that can go up and down.

```go
gauge := NewGauge("active_connections", "Active connections", nil)
gauge.Set(42)
gauge.Inc()
gauge.Dec()
```

### Histogram
Tracks distribution of values in buckets.

```go
buckets := []float64{1, 5, 10, 50, 100, 500, 1000}
histogram := NewHistogram("request_latency_ms", "Request latency", buckets)
histogram.Observe(125.5)
```

### GaugeVec
Gauge with labels for multi-dimensional metrics.

```go
gaugeVec := NewGaugeVec("documents_total", "Total documents", []string{"store"})
gaugeVec.WithLabels("default").Set(1000)
gaugeVec.WithLabels("custom").Set(500)
```

### CounterVec
Counter with labels for multi-dimensional metrics.

```go
counterVec := NewCounterVec("errors_total", "Total errors", []string{"error_type"})
counterVec.WithLabels("timeout").Inc()
counterVec.WithLabels("network").Inc()
```

## Usage

### 1. Create Metrics Instance

```go
import "github.com/ricesearch/rice-search/internal/metrics"

m := metrics.New()
```

### 2. Record Metrics

```go
// Search metrics
m.RecordSearch(latencyMs, resultCount, err)

// Index metrics
m.RecordIndex(docCount, chunkCount, latencyMs, err)

// ML metrics
m.RecordEmbed(batchSize, latencyMs)
m.RecordRerank(candidateCount, latencyMs)

// Store metrics
m.UpdateStoreStats("default", docCount, chunkCount)
m.UpdateStoreCount(3)

// Connection metrics
m.IncrementConnection()
m.DecrementConnection()
```

### 3. Export Prometheus Metrics

```go
// Get Prometheus text format
prometheusText := m.PrometheusFormat()

// Or use HTTP handler
http.Handle("/metrics", m.Handler())
```

### 4. Integrate with Event Bus

```go
subscriber := metrics.NewEventSubscriber(m, eventBus)
subscriber.SubscribeToEvents(ctx)
```

### 5. Collect Stats

```go
collector := metrics.NewCollector(m, storeService, qdrantClient)

// Collect all stats
stats, err := collector.Collect(ctx)

// Collect for specific store
storeStats, err := collector.CollectForStore(ctx, "default")

// Get human-readable summary
summary := collector.Summary(ctx)
```

## Available Metrics

### Search Metrics
- `rice_search_requests_total` - Total search requests
- `rice_search_latency_ms` - Search latency histogram
- `rice_search_results` - Results per search histogram
- `rice_search_errors_total{error_type}` - Search errors by type

### Index Metrics
- `rice_indexed_documents_total` - Total documents indexed
- `rice_indexed_chunks_total` - Total chunks indexed
- `rice_index_latency_ms` - Indexing latency histogram
- `rice_index_errors_total{error_type}` - Indexing errors by type

### ML Metrics
- `rice_embed_requests_total` - Embedding requests
- `rice_embed_latency_ms` - Embedding latency histogram
- `rice_embed_batch_size` - Embedding batch size histogram
- `rice_rerank_requests_total` - Reranking requests
- `rice_rerank_latency_ms` - Reranking latency histogram
- `rice_sparse_encode_requests_total` - Sparse encoding requests
- `rice_sparse_encode_latency_ms` - Sparse encoding latency histogram
- `rice_query_understand_requests_total` - Query understanding requests
- `rice_query_understand_latency_ms` - Query understanding latency histogram

### Connection Metrics
- `rice_active_connections` - Current active connections
- `rice_connections_total` - Total connections
- `rice_connection_errors_total{error_type}` - Connection errors by type

### Store Metrics
- `rice_stores_total` - Total number of stores
- `rice_documents_total{store}` - Documents per store
- `rice_chunks_total{store}` - Chunks per store

### System Metrics
- `rice_goroutines` - Number of goroutines
- `rice_memory_bytes` - Memory usage in bytes
- `rice_uptime_seconds` - Application uptime

## Metric Presets

Predefined metric queries for common observability scenarios:

```go
// Get all presets
presets := metrics.GetAllPresets()

// Get preset by ID
preset := metrics.GetPreset("search_overview")

// Get presets by category
categories := metrics.GetPresetsByCategory()
searchPresets := categories["search"]
```

### Available Presets

**Search**
- `search_overview` - Overall search performance
- `search_latency` - Search latency distribution

**Indexing**
- `index_status` - Current indexing statistics
- `index_performance` - Indexing throughput and latency

**ML**
- `ml_performance` - Embedding and reranking metrics
- `ml_throughput` - ML request rates and batch sizes

**System**
- `system_health` - System resource usage
- `connection_stats` - Connection statistics
- `uptime_status` - Uptime and availability

**Stores**
- `store_stats` - Per-store document and chunk counts
- `top_stores` - Most queried stores

**Errors**
- `error_rates` - Error counts by type

**Performance**
- `latency_percentiles` - P50, P95, P99 latencies

## Prometheus Exposition Format

The package exports metrics in the standard Prometheus text format:

```
# HELP rice_search_requests_total Total number of search requests
# TYPE rice_search_requests_total counter
rice_search_requests_total 1234

# HELP rice_search_latency_ms Search request latency in milliseconds
# TYPE rice_search_latency_ms histogram
rice_search_latency_ms_bucket{le="1.0"} 10
rice_search_latency_ms_bucket{le="5.0"} 50
rice_search_latency_ms_bucket{le="10.0"} 120
rice_search_latency_ms_bucket{le="+Inf"} 150
rice_search_latency_ms_sum 2340.50
rice_search_latency_ms_count 150

# HELP rice_documents_total Total number of documents per store
# TYPE rice_documents_total gauge
rice_documents_total{store="default"} 1000
rice_documents_total{store="custom"} 500
```

## Integration Examples

### With HTTP Server

```go
func (s *Server) setupMetrics() {
    s.metrics = metrics.New()
    http.Handle("/metrics", s.metrics.Handler())
}
```

### With Search Service

```go
func (s *Service) Search(ctx context.Context, req Request) (*Response, error) {
    start := time.Now()
    results, err := s.performSearch(ctx, req)
    latencyMs := time.Since(start).Milliseconds()
    
    s.metrics.RecordSearch(latencyMs, len(results), err)
    return results, err
}
```

### With Index Pipeline

```go
func (p *Pipeline) Index(ctx context.Context, req IndexRequest) (*IndexResult, error) {
    start := time.Now()
    result, err := p.performIndex(ctx, req)
    latencyMs := time.Since(start).Milliseconds()
    
    p.metrics.RecordIndex(result.Indexed, result.ChunksTotal, latencyMs, err)
    return result, err
}
```

## Performance

All metric operations are optimized for high throughput:

- Counters and gauges use atomic operations
- Histograms use mutexes only during observation
- Label-based metrics use read-write locks for cache lookups
- No external dependencies or allocations in hot paths

Benchmarks:
```
BenchmarkCounterInc-8           50000000    25.3 ns/op    0 B/op    0 allocs/op
BenchmarkGaugeSet-8             50000000    26.1 ns/op    0 B/op    0 allocs/op
BenchmarkHistogramObserve-8     10000000    143 ns/op     0 B/op    0 allocs/op
BenchmarkGaugeVecWithLabels-8    5000000    312 ns/op     0 B/op    0 allocs/op
```

## Testing

Run tests:
```bash
go test -v ./internal/metrics/
```

Run benchmarks:
```bash
go test -bench=. ./internal/metrics/
```

## License

Same as Rice Search - CC BY-NC-SA 4.0
