# Performance

## Overview

Performance targets, benchmarks, and optimization guidelines.

---

## Performance Targets

### Latency Targets

| Operation | P50 | P95 | P99 | Max |
|-----------|-----|-----|-----|-----|
| Search (no rerank) | 50ms | 100ms | 200ms | 500ms |
| Search (with rerank) | 100ms | 200ms | 400ms | 1s |
| Embed (single) | 30ms | 50ms | 100ms | 200ms |
| Embed (batch 32) | 100ms | 200ms | 400ms | 1s |
| Sparse encode | 20ms | 40ms | 80ms | 200ms |
| Rerank (30 docs) | 60ms | 100ms | 150ms | 300ms |
| Index (per file) | 200ms | 500ms | 1s | 5s |

### Throughput Targets

| Operation | Target (GPU) | Target (CPU) |
|-----------|--------------|--------------|
| Search | 100 req/s | 30 req/s |
| Embed | 500 texts/s | 50 texts/s |
| Index | 10 files/s | 2 files/s |

### Resource Targets

| Resource | Target |
|----------|--------|
| Memory (idle) | <500MB |
| Memory (loaded) | <4GB |
| CPU (idle) | <5% |
| GPU VRAM | <3GB |

---

## Benchmarks

### Search Benchmark

```bash
# Run benchmark
./rice-search benchmark search \
    --store default \
    --queries benchmark/queries.txt \
    --concurrency 10 \
    --duration 60s
```

**Expected Output:**

```
Search Benchmark Results
========================
Total requests:     5,234
Successful:         5,230
Failed:             4
Duration:           60.0s
Throughput:         87.2 req/s

Latency:
  P50:              58ms
  P95:              124ms
  P99:              203ms
  Max:              412ms

By stage:
  Sparse:           12ms (P50)
  Dense:            28ms (P50)
  Fusion:           3ms (P50)
  Rerank:           45ms (P50)
```

### Index Benchmark

```bash
./rice-search benchmark index \
    --store benchmark \
    --dir ./benchmark/files \
    --batch-size 100
```

**Expected Output:**

```
Index Benchmark Results
=======================
Files processed:    1,000
Chunks created:     5,423
Total time:         95.3s
Throughput:         10.5 files/s
                    56.9 chunks/s

By stage:
  Chunking:         8.2s (8.6%)
  Embedding:        72.1s (75.6%)
  Qdrant insert:    15.0s (15.8%)
```

---

## Optimization Guidelines

### Query Optimization

#### Use Filters Early

```json
// Good: Filter in Qdrant (fast)
{
    "prefetch": [
        {
            "query": sparse_vector,
            "filter": {"key": "language", "match": {"value": "go"}},
            "limit": 100
        }
    ]
}

// Bad: Filter after retrieval (slow)
results = search(query)
filtered = [r for r in results if r.language == "go"]
```

#### Adjust Top-K

```
Small top_k (10-20): Fast, may miss relevant results
Large top_k (50-100): Slower, better recall

Recommendation: top_k=20 for most cases
```

#### Disable Reranking for Speed

```json
{
    "query": "simple keyword search",
    "enable_reranking": false
}
```

When to disable:
- Simple keyword searches
- High throughput requirements
- Known exact matches

---

### Embedding Optimization

#### Batch Aggressively

| Batch Size | Throughput | Latency |
|------------|------------|---------|
| 1 | 20 texts/s | 50ms |
| 8 | 100 texts/s | 80ms |
| 32 | 400 texts/s | 80ms |
| 64 | 500 texts/s | 130ms |

#### Cache Everything

```
Cache hit: 0ms
Cache miss: 50ms

At 70% hit rate:
  Effective latency = 0.7 * 0ms + 0.3 * 50ms = 15ms
```

#### Use FP16 Models

| Precision | Memory | Speed | Quality |
|-----------|--------|-------|---------|
| FP32 | 100% | 1x | 100% |
| FP16 | 50% | 1.5x | ~100% |
| INT8 | 25% | 2x | 99%+ |

---

### Qdrant Optimization

#### Collection Settings

```json
{
    "optimizers_config": {
        "indexing_threshold": 20000,
        "memmap_threshold": 50000
    },
    "on_disk_payload": true
}
```

#### Index Settings

```json
{
    "vectors": {
        "dense": {
            "size": 1536,
            "distance": "Cosine",
            "hnsw_config": {
                "m": 16,
                "ef_construct": 100
            }
        }
    }
}
```

| HNSW Parameter | Trade-off |
|----------------|-----------|
| Higher `m` | Better recall, more memory |
| Higher `ef_construct` | Better recall, slower indexing |
| Higher `ef` (search) | Better recall, slower search |

---

### Memory Optimization

#### Reduce Cache Size

```yaml
cache:
  embed_size: 50000   # vs 100000
  sparse_size: 50000  # vs 100000
```

Memory per entry:
- Embed: 1536 * 4 bytes = 6KB
- 100K entries = 600MB

#### Use On-Disk Payload

```json
{
    "on_disk_payload": true
}
```

Saves RAM for large content fields.

#### GPU Memory

```yaml
gpu:
  load_mode: ondemand  # vs all
```

| Mode | VRAM | Latency |
|------|------|---------|
| all | ~2.5GB | Fastest |
| ondemand | ~600MB | +500ms cold start |

---

### Concurrency Optimization

#### HTTP Connection Pool

```yaml
qdrant:
  pool_size: 32        # Match expected concurrency
  idle_timeout: 30s
```

#### Worker Pool Sizing

```
Workers = CPU cores * 2 (I/O bound)
Workers = CPU cores (CPU bound)

ML workers: Min(CPU cores, GPU count * 4)
Search workers: CPU cores * 2
```

---

## Profiling

### CPU Profiling

```bash
# Enable pprof
./rice-search serve --pprof

# Collect profile
go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

# Analyze
(pprof) top
(pprof) web
```

### Memory Profiling

```bash
go tool pprof http://localhost:6060/debug/pprof/heap

(pprof) top
(pprof) list functionName
```

### Trace

```bash
curl -o trace.out http://localhost:6060/debug/pprof/trace?seconds=5
go tool trace trace.out
```

---

## Performance Testing

### Load Test

```bash
# Using hey
hey -n 10000 -c 50 -m POST \
    -H "Content-Type: application/json" \
    -d '{"query": "test"}' \
    http://localhost:8080/v1/search
```

### Stress Test

```bash
# Gradually increase load until failure
for c in 10 20 50 100 200; do
    echo "Concurrency: $c"
    hey -n 1000 -c $c -m POST \
        -d '{"query": "test"}' \
        http://localhost:8080/v1/search
    sleep 5
done
```

### Soak Test

```bash
# Run for extended period
hey -n 1000000 -c 20 -m POST \
    -d '{"query": "test"}' \
    http://localhost:8080/v1/search
```

---

## Scaling Guidelines

### Vertical Scaling

| Scale | Action |
|-------|--------|
| More queries | Add CPU cores |
| Faster embedding | Add/upgrade GPU |
| More data | Add RAM/disk |

### Horizontal Scaling

```
┌───────────────┐
│  Load Balancer │
└───────┬───────┘
        │
┌───────┼───────┐
│       │       │
▼       ▼       ▼
API-1   API-2   API-3
│       │       │
└───────┼───────┘
        │
   Shared Qdrant
```

Stateless API scales horizontally. Qdrant is the bottleneck.

### When to Scale

| Metric | Threshold | Action |
|--------|-----------|--------|
| CPU > 70% | Sustained | Add API instance |
| Memory > 80% | Sustained | Increase memory |
| P99 > 1s | Sustained | Scale out or optimize |
| Queue depth > 100 | Sustained | Add workers |
