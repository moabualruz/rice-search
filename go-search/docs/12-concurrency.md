# Concurrency

## Overview

Go's concurrency primitives power the event-driven architecture. This doc covers worker pools, backpressure, and resource limits.

---

## Concurrency Model

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           CONCURRENCY MODEL                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   HTTP Requests          Event Bus              Worker Pools                │
│   ┌─────────┐           ┌─────────┐           ┌─────────────────┐          │
│   │ Request │──────────▶│  Topic  │──────────▶│ ML Workers (4)  │          │
│   │ Request │──────────▶│  Topic  │──────────▶│ Search Workers  │          │
│   │ Request │──────────▶│  Topic  │──────────▶│ Index Workers   │          │
│   └─────────┘           └─────────┘           └─────────────────┘          │
│        │                     │                        │                     │
│        ▼                     ▼                        ▼                     │
│   Rate Limiter          Buffered Channels       Semaphores                 │
│   (per client)          (backpressure)          (resource limits)          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Worker Pools

### ML Worker Pool

Handles embedding, sparse encoding, reranking.

```go
type MLWorkerPool struct {
    embedWorkers  int // Concurrent embed requests
    sparseWorkers int // Concurrent sparse requests
    rerankWorkers int // Concurrent rerank requests
    sem           *semaphore.Weighted
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `ML_EMBED_WORKERS` | 4 | Concurrent embedding batches |
| `ML_SPARSE_WORKERS` | 4 | Concurrent sparse batches |
| `ML_RERANK_WORKERS` | 2 | Concurrent rerank requests |

**Why limited?** GPU memory is shared. Too many concurrent batches = OOM.

### Search Worker Pool

Handles Qdrant queries.

| Setting | Default | Description |
|---------|---------|-------------|
| `SEARCH_WORKERS` | 16 | Concurrent Qdrant queries |
| `SEARCH_QUEUE_SIZE` | 1000 | Pending search queue |

### Index Worker Pool

Handles document processing.

| Setting | Default | Description |
|---------|---------|-------------|
| `INDEX_WORKERS` | 8 | Concurrent file processing |
| `INDEX_QUEUE_SIZE` | 10000 | Pending index queue |
| `CHUNK_WORKERS` | 4 | Concurrent chunking |

---

## Backpressure

### Event Bus Buffering

```go
// Memory bus with bounded channels
type MemoryBus struct {
    topics map[string]chan Event
}

func NewMemoryBus(bufferSize int) *MemoryBus {
    // Each topic has buffered channel
    // When full, Publish blocks (backpressure)
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `BUS_BUFFER_SIZE` | 1000 | Events per topic buffer |
| `BUS_PUBLISH_TIMEOUT` | 5s | Timeout before rejecting |

### Backpressure Signals

```go
// Check if overloaded
if len(topic) > bufferSize * 0.8 {
    // Return 503 to new requests
    // Or slow down producers
}
```

### HTTP Backpressure

```go
// Limit concurrent HTTP requests
var httpSem = semaphore.NewWeighted(1000)

func Handler(w http.ResponseWriter, r *http.Request) {
    if !httpSem.TryAcquire(1) {
        http.Error(w, "server overloaded", 503)
        return
    }
    defer httpSem.Release(1)
    
    // Handle request
}
```

---

## Resource Limits

### Memory Limits

| Resource | Limit | Action When Exceeded |
|----------|-------|---------------------|
| Embedding cache | 100K entries | LRU eviction |
| Sparse cache | 100K entries | LRU eviction |
| Request body | 10MB | Reject with 413 |
| Batch size | 100 documents | Split into multiple |

### Goroutine Limits

| Component | Max Goroutines | Purpose |
|-----------|----------------|---------|
| HTTP handlers | 1000 | Concurrent requests |
| Event handlers | 100 per topic | Event processing |
| ML inference | 8 | GPU/CPU bound |
| Qdrant client | 32 | I/O bound |

### Connection Limits

| Resource | Limit | Description |
|----------|-------|-------------|
| Qdrant connections | 32 | HTTP connection pool |
| Kafka connections | 10 | Per broker |
| Redis connections | 20 | Connection pool |

---

## Timeouts

### Request Timeouts

| Operation | Timeout | Description |
|-----------|---------|-------------|
| HTTP read | 30s | Read request body |
| HTTP write | 30s | Write response |
| Search | 10s | Full search operation |
| Index (per file) | 30s | Single file indexing |
| Embed batch | 30s | Embedding batch |
| Rerank | 10s | Reranking |

### Event Timeouts

| Operation | Timeout | Description |
|-----------|---------|-------------|
| Event publish | 5s | Publish to bus |
| Event request/reply | 30s | Wait for response |
| Handler execution | 60s | Max handler time |

### Implementation

```go
// Context with timeout
ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

// Use context in operations
result, err := search.Execute(ctx, query)
if errors.Is(err, context.DeadlineExceeded) {
    return ErrTimeout("search")
}
```

---

## Graceful Degradation

### When Overloaded

| Condition | Action |
|-----------|--------|
| Queue 80% full | Log warning |
| Queue 90% full | Reject new requests (503) |
| Queue 100% full | Block publishers |
| Memory 80% | Evict caches aggressively |
| Memory 90% | Reject large requests |

### Circuit Breaker

> ⚠️ **NOT IMPLEMENTED**: Circuit breaker pattern is documented for future implementation. Currently, failed operations are retried without circuit breaking.

```go
// DESIGN - Not yet implemented
type CircuitBreaker struct {
    state            State // closed, open, half-open
    failures         int
    failureThreshold int
    successThreshold int
    timeout          time.Duration
    lastFailure      time.Time
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    if cb.state == Open {
        if time.Since(cb.lastFailure) > cb.timeout {
            cb.state = HalfOpen
        } else {
            return ErrCircuitOpen
        }
    }
    
    err := fn()
    if err != nil {
        cb.recordFailure()
    } else {
        cb.recordSuccess()
    }
    return err
}
```

---

## Parallel Processing

### Parallel Embedding

```go
func EmbedBatch(texts []string) ([][]float32, error) {
    // Split into sub-batches
    batches := splitIntoBatches(texts, batchSize)
    
    // Process in parallel
    results := make([][]float32, len(texts))
    g, ctx := errgroup.WithContext(ctx)
    
    for i, batch := range batches {
        i, batch := i, batch
        g.Go(func() error {
            embeddings, err := embedder.Embed(ctx, batch)
            if err != nil {
                return err
            }
            copy(results[i*batchSize:], embeddings)
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    return results, nil
}
```

### Parallel Search

```go
func HybridSearch(query string) (*Results, error) {
    var sparseResults, denseResults []Result
    
    g, ctx := errgroup.WithContext(ctx)
    
    // Sparse search
    g.Go(func() error {
        var err error
        sparseResults, err = sparseSearch(ctx, query)
        return err
    })
    
    // Dense search  
    g.Go(func() error {
        var err error
        denseResults, err = denseSearch(ctx, query)
        return err
    })
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return fusionRRF(sparseResults, denseResults), nil
}
```

---

## Configuration

```yaml
concurrency:
  # Worker pools
  ml:
    embed_workers: 4
    sparse_workers: 4
    rerank_workers: 2
  search:
    workers: 16
    queue_size: 1000
  index:
    workers: 8
    queue_size: 10000
  
  # Limits
  http:
    max_concurrent: 1000
    read_timeout: 30s
    write_timeout: 30s
  
  # Backpressure
  bus:
    buffer_size: 1000
    publish_timeout: 5s
  
  # Connections
  qdrant:
    pool_size: 32
  redis:
    pool_size: 20
```

---

## Monitoring

### Metrics to Track

| Metric | Description |
|--------|-------------|
| `worker_pool_active` | Active workers per pool |
| `worker_pool_queued` | Queued tasks per pool |
| `event_bus_buffer_used` | Buffer utilization per topic |
| `goroutine_count` | Total goroutines |
| `http_concurrent_requests` | Active HTTP requests |
| `circuit_breaker_state` | Per-dependency state |

### Alerts

| Condition | Alert |
|-----------|-------|
| Queue > 80% | Warning |
| Queue > 95% | Critical |
| Goroutines > 10000 | Warning |
| Circuit breaker open | Critical |
