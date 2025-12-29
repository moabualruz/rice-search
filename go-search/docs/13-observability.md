# Observability

## Overview

Three pillars: Metrics, Logging, Tracing.

---

## Metrics (Prometheus)

### Endpoint

```
GET /metrics
Content-Type: text/plain
```

### Metric Naming

```
rice_{subsystem}_{name}_{unit}

Examples:
- rice_search_requests_total
- rice_search_latency_seconds
- rice_ml_embed_batch_size
```

---

### HTTP Metrics

```prometheus
# Request counter
rice_http_requests_total{method="POST", path="/v1/search", status="200"} 1234

# Latency histogram
rice_http_request_duration_seconds_bucket{method="POST", path="/v1/search", le="0.1"} 500
rice_http_request_duration_seconds_bucket{method="POST", path="/v1/search", le="0.5"} 950
rice_http_request_duration_seconds_bucket{method="POST", path="/v1/search", le="1.0"} 1000
rice_http_request_duration_seconds_sum{method="POST", path="/v1/search"} 125.5
rice_http_request_duration_seconds_count{method="POST", path="/v1/search"} 1000

# In-flight requests
rice_http_requests_in_flight{method="POST", path="/v1/search"} 5

# Request size
rice_http_request_size_bytes_bucket{path="/v1/index", le="1024"} 100
rice_http_request_size_bytes_bucket{path="/v1/index", le="10240"} 500
```

---

### Search Metrics

```prometheus
# Search requests by store
rice_search_requests_total{store="default", reranking="true"} 5000

# Search latency by stage
rice_search_stage_duration_seconds{store="default", stage="sparse"} 
rice_search_stage_duration_seconds{store="default", stage="dense"}
rice_search_stage_duration_seconds{store="default", stage="fusion"}
rice_search_stage_duration_seconds{store="default", stage="rerank"}

# Results count
rice_search_results_total{store="default"} 125000
rice_search_results_per_query_bucket{store="default", le="10"} 2000
rice_search_results_per_query_bucket{store="default", le="20"} 4500
```

---

### ML Metrics

```prometheus
# Embedding requests
rice_ml_embed_requests_total 10000
rice_ml_embed_texts_total 50000
rice_ml_embed_latency_seconds_bucket{le="0.1"} 8000

# Sparse encoding
rice_ml_sparse_requests_total 10000
rice_ml_sparse_latency_seconds_bucket{le="0.05"} 9000

# Reranking
rice_ml_rerank_requests_total 5000
rice_ml_rerank_documents_total 150000
rice_ml_rerank_latency_seconds_bucket{le="0.1"} 4500

# Batch sizes
rice_ml_embed_batch_size_bucket{le="8"} 1000
rice_ml_embed_batch_size_bucket{le="32"} 8000

# Cache
rice_ml_cache_hits_total{type="embed"} 25000
rice_ml_cache_misses_total{type="embed"} 10000
rice_ml_cache_size{type="embed"} 50000

# GPU (if available)
rice_ml_gpu_memory_used_bytes 2147483648
rice_ml_gpu_utilization_percent 75
```

---

### Index Metrics

```prometheus
# Index operations
rice_index_requests_total{store="default"} 500
rice_index_documents_total{store="default"} 10000
rice_index_chunks_total{store="default"} 50000

# Index latency
rice_index_duration_seconds_bucket{store="default", le="1"} 400
rice_index_duration_seconds_bucket{store="default", le="10"} 490

# Errors
rice_index_errors_total{store="default", reason="parse_error"} 15
rice_index_errors_total{store="default", reason="too_large"} 5
```

---

### Event Bus Metrics

```prometheus
# Events published
rice_bus_events_published_total{topic="ml.embed.request"} 10000
rice_bus_events_published_total{topic="search.query.request"} 5000

# Event latency (publish to handle)
rice_bus_event_latency_seconds{topic="ml.embed.request"} 

# Buffer utilization
rice_bus_buffer_size{topic="ml.embed.request"} 1000
rice_bus_buffer_used{topic="ml.embed.request"} 50

# Errors
rice_bus_errors_total{topic="ml.embed.request", error="timeout"} 5
```

---

### System Metrics

```prometheus
# Go runtime
go_goroutines 150
go_memstats_alloc_bytes 104857600
go_memstats_heap_inuse_bytes 83886080

# Process
process_cpu_seconds_total 125.5
process_resident_memory_bytes 524288000
process_open_fds 50

# Qdrant connection
rice_qdrant_connections_active 10
rice_qdrant_requests_total{operation="search"} 5000
rice_qdrant_errors_total{operation="search"} 5
```

---

## Logging

### Format

**Development (text):**
```
2025-12-29T01:00:00.000Z INFO  [api] Search request received store=default query="auth handler"
2025-12-29T01:00:00.050Z INFO  [search] Hybrid search completed results=20 latency_ms=50
```

**Production (JSON):**
```json
{
    "time": "2025-12-29T01:00:00.000Z",
    "level": "info",
    "service": "api",
    "msg": "Search request received",
    "request_id": "req_abc123",
    "store": "default",
    "query": "auth handler",
    "client_ip": "10.0.0.1"
}
```

### Log Levels

| Level | When to Use |
|-------|-------------|
| `debug` | Detailed debugging, request bodies |
| `info` | Normal operations, request lifecycle |
| `warn` | Recoverable issues, degraded performance |
| `error` | Failures requiring attention |

### What to Log

| Event | Level | Fields |
|-------|-------|--------|
| Request received | info | request_id, method, path, client_ip |
| Request completed | info | request_id, status, latency_ms |
| Search executed | info | request_id, store, results, latency_ms |
| Index completed | info | request_id, store, indexed, errors |
| Error occurred | error | request_id, error_code, message, stack |
| Service started | info | version, config (sanitized) |
| Service stopping | info | reason |

### Structured Fields

```go
logger.Info("search completed",
    "request_id", requestID,
    "store", store,
    "query", query,
    "results", len(results),
    "sparse_ms", stages.SparseMS,
    "dense_ms", stages.DenseMS,
    "rerank_ms", stages.RerankMS,
    "total_ms", totalMS,
)
```

### Sensitive Data

**Never log:**
- Full file contents
- Embeddings
- API keys/tokens
- Full query (truncate to 100 chars)

**Always sanitize:**
```go
func sanitizeQuery(q string) string {
    if len(q) > 100 {
        return q[:100] + "..."
    }
    return q
}
```

---

## Tracing (OpenTelemetry)

### Configuration

```yaml
tracing:
  enabled: true
  endpoint: http://jaeger:4318/v1/traces
  sample_rate: 0.1  # 10% of requests
  service_name: rice-search
```

### Span Hierarchy

```
[HTTP Request /v1/search]
├── [Parse Request]
├── [Publish search.request]
│   └── [Search Service Handler]
│       ├── [Encode Query]
│       │   ├── [ML Sparse Encode]
│       │   └── [ML Dense Embed]
│       ├── [Qdrant Hybrid Search]
│       │   ├── [Sparse Search]
│       │   └── [Dense Search]
│       ├── [RRF Fusion]
│       └── [Rerank]
│           └── [ML Rerank]
└── [Send Response]
```

### Span Attributes

```go
span.SetAttributes(
    attribute.String("store", store),
    attribute.String("query", sanitizeQuery(query)),
    attribute.Int("top_k", topK),
    attribute.Int("results", len(results)),
    attribute.Int64("latency_ms", latencyMS),
)
```

### Error Recording

```go
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

---

## Health Endpoints

### GET /healthz (Liveness)

Is the process running?

```json
{"status": "ok"}
```

Returns 200 if process is alive.

### GET /readyz (Readiness)

Can the service handle requests?

```json
{
    "status": "ready",
    "checks": {
        "qdrant": {"status": "ok", "latency_ms": 5},
        "models": {"status": "ok", "loaded": ["embed", "sparse", "rerank"]},
        "event_bus": {"status": "ok"}
    }
}
```

Returns 200 if ready, 503 if not.

### GET /v1/health (Detailed)

Full health information.

```json
{
    "status": "healthy",
    "version": "1.0.0",
    "uptime_seconds": 3600,
    "checks": {
        "qdrant": {
            "status": "ok",
            "url": "http://localhost:6333",
            "latency_ms": 5
        },
        "models": {
            "status": "ok",
            "embed": {"loaded": true, "memory_mb": 600},
            "sparse": {"loaded": true, "memory_mb": 250},
            "rerank": {"loaded": true, "memory_mb": 500}
        },
        "cache": {
            "embed_entries": 50000,
            "sparse_entries": 50000,
            "hit_rate": 0.72
        }
    }
}
```

---

## Alerting Rules

### Critical

| Condition | Alert |
|-----------|-------|
| Error rate > 5% | High error rate |
| P99 latency > 5s | Severe latency |
| Qdrant unreachable | Database down |
| OOM killed | Service crashed |

### Warning

| Condition | Alert |
|-----------|-------|
| Error rate > 1% | Elevated errors |
| P99 latency > 1s | High latency |
| Cache hit rate < 50% | Low cache efficiency |
| Queue > 80% | Backpressure building |

### Example Prometheus Rules

```yaml
groups:
  - name: rice-search
    rules:
      - alert: HighErrorRate
        expr: rate(rice_http_requests_total{status=~"5.."}[5m]) / rate(rice_http_requests_total[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: High error rate detected

      - alert: HighLatency
        expr: histogram_quantile(0.99, rate(rice_http_request_duration_seconds_bucket[5m])) > 5
        for: 5m
        labels:
          severity: critical
```

---

## Dashboard Panels

### Overview Dashboard

1. Request rate (req/s)
2. Error rate (%)
3. P50/P95/P99 latency
4. Active connections

### Search Dashboard

1. Searches per minute
2. Latency by stage (sparse, dense, fusion, rerank)
3. Results per query distribution
4. Cache hit rate

### ML Dashboard

1. Embedding throughput (texts/s)
2. Batch size distribution
3. GPU utilization
4. Model memory usage

### Index Dashboard

1. Documents indexed per minute
2. Chunks created
3. Index errors
4. Queue depth
