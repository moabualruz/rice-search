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

> ⚠️ **NOT IMPLEMENTED**: ML worker pool semaphores are documented for future implementation. Currently, ML operations (embedding, sparse encoding, reranking) are not limited by worker pools. GPU concurrency is controlled only by ONNX Runtime's internal threading.

```go
// DESIGN - Not yet implemented
type MLWorkerPool struct {
    embedWorkers  int // Concurrent embed requests
    sparseWorkers int // Concurrent sparse requests
    rerankWorkers int // Concurrent rerank requests
    sem           *semaphore.Weighted
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `ML_EMBED_WORKERS` | 4 | Concurrent embedding batches (NOT IMPLEMENTED) |
| `ML_SPARSE_WORKERS` | 4 | Concurrent sparse batches (NOT IMPLEMENTED) |
| `ML_RERANK_WORKERS` | 2 | Concurrent rerank requests (NOT IMPLEMENTED) |

**Why limited?** GPU memory is shared. Too many concurrent batches = OOM.

### Search Worker Pool

> ⚠️ **NOT IMPLEMENTED**: Search worker pool is documented for future implementation. Currently, search operations use simple `errgroup` parallelization for sparse+dense retrieval without a dedicated worker pool or queue.

| Setting | Default | Description |
|---------|---------|-------------|
| `SEARCH_WORKERS` | 16 | Concurrent Qdrant queries (NOT IMPLEMENTED) |
| `SEARCH_QUEUE_SIZE` | 1000 | Pending search queue (NOT IMPLEMENTED) |

### Index Worker Pool

**Status: ✅ IMPLEMENTED** - Located in `internal/index/batch.go`

Handles document processing using semaphore-based worker pool.

| Setting | Default | Description |
|---------|---------|-------------|
| `INDEX_WORKERS` | 4 | Concurrent file processing (via RICE_INDEX_WORKERS) |
| `INDEX_QUEUE_SIZE` | 10000 | Pending index queue (NOT IMPLEMENTED) |
| `CHUNK_WORKERS` | 4 | Concurrent chunking (NOT IMPLEMENTED) |

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

> ⚠️ **NOT IMPLEMENTED**: Global HTTP concurrency semaphore is documented for future implementation. Currently, only per-client rate limiting is implemented (see Rate Limiting section below).

```go
// DESIGN - Not yet implemented
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

### Rate Limiting

**Status: ✅ IMPLEMENTED**

Per-client rate limiting using `golang.org/x/time/rate`.

#### Implementation
Located in `internal/pkg/middleware/ratelimit.go`:

```go
type RateLimiter struct {
    mu       sync.RWMutex
    clients  map[string]*rate.Limiter
    rate     rate.Limit
    burst    int
    cleanup  time.Duration
    lastSeen map[string]time.Time
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        clientIP := getClientIP(r)
        if !rl.getLimiter(clientIP).Allow() {
            // Return 429 Too Many Requests
            apperrors.WriteErrorWithStatus(w, http.StatusTooManyRequests, ...)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

#### Features
- Per-client tracking (by IP address)
- Token bucket algorithm via stdlib
- Automatic cleanup of stale entries (5 min expiry)
- Configurable rate and burst

#### Configuration
```bash
RICE_RATE_LIMIT=100  # 100 requests/second per client (0 = disabled)
```

#### Client Identification
1. X-Forwarded-For header (first IP)
2. X-Real-IP header
3. RemoteAddr (fallback)

#### Defaults
- Rate: 100 requests/second per client
- Burst: 200 requests
- Cleanup: Every 1 minute
- Stale threshold: 5 minutes

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
    embed_workers: 4        # ❌ NOT IMPLEMENTED
    sparse_workers: 4       # ❌ NOT IMPLEMENTED
    rerank_workers: 2       # ❌ NOT IMPLEMENTED
  search:
    workers: 16             # ❌ NOT IMPLEMENTED
    queue_size: 1000        # ❌ NOT IMPLEMENTED
  index:
    workers: 4              # ✅ IMPLEMENTED (RICE_INDEX_WORKERS)
    queue_size: 10000       # ❌ NOT IMPLEMENTED
  
  # Limits
  http:
    max_concurrent: 1000    # ❌ NOT IMPLEMENTED (global semaphore)
    read_timeout: 30s       # ✅ IMPLEMENTED
    write_timeout: 30s      # ✅ IMPLEMENTED
  
  # Backpressure
  bus:
    buffer_size: 1000       # ✅ IMPLEMENTED (event bus channels)
    publish_timeout: 5s     # ✅ IMPLEMENTED (event bus)
  
  # Connections
  qdrant:
    pool_size: 32           # ✅ IMPLEMENTED (HTTP client)
  redis:
    pool_size: 20           # ✅ IMPLEMENTED (Redis pool)
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

---

## Pattern Implementation Status

| Pattern | Status | Location |
|---------|--------|----------|
| Worker Pool | ✅ | internal/index/batch.go |
| Context Propagation | ✅ | Throughout (478 occurrences) |
| Graceful Shutdown | ✅ | cmd/rice-search-server/main.go |
| Mutex/RWMutex | ✅ | Throughout (38 occurrences) |
| Channel Communication | ✅ | internal/bus/memory.go |
| Timeout Handling | ✅ | Throughout (37 occurrences) |
| Select Statements | ✅ | Throughout (25 occurrences) |
| Parallel Processing | ✅ | internal/index/batch.go |
| Backpressure | ✅ | internal/bus/memory.go |
| Rate Limiting | ✅ | internal/pkg/middleware/ratelimit.go |
| Circuit Breaker | ❌ | Documented, not implemented |
