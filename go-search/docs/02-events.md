# Event System

> **⚠️ IMPLEMENTATION STATUS**: Only `MemoryBus` is fully implemented and production-ready. `KafkaBus` exists but is **NOT FULLY TESTED**. `NATSBus` and `RedisBus` are **NOT IMPLEMENTED** (see factory.go lines 34-38).

## Overview

All internal communication happens via events. The event bus is pluggable:
- **Single process**: Go channels (default, zero latency) - **✅ IMPLEMENTED**
- **Distributed**: Kafka / NATS / Redis Streams (configurable) - **⚠️ PARTIAL/NOT IMPLEMENTED**

---

## Event Bus Interface

```go
type Bus interface {
    // Publish sends an event (fire-and-forget)
    Publish(ctx context.Context, topic string, event any) error
    
    // Subscribe registers a handler for a topic
    Subscribe(ctx context.Context, topic string, handler Handler) error
    
    // Request sends and waits for response (request/reply pattern)
    Request(ctx context.Context, req RequestEvent) (ResponseEvent, error)
    
    // Close shuts down the bus
    Close() error
}

type Handler func(ctx context.Context, event any) error
```

---

## Event Base Structure

All events share this base:

```go
type EventBase struct {
    ID            string    `json:"id"`             // Unique event ID (UUID)
    CorrelationID string    `json:"correlation_id"` // Links request/response
    Timestamp     time.Time `json:"timestamp"`      // Event creation time
    Source        string    `json:"source"`         // Service that emitted
}
```

---

## Event Topics

### Naming Convention

```
{domain}.{action}[.{suffix}]

Examples:
- ml.embed.request
- ml.embed.response
- search.request
- index.chunk.created
- connection.registered
```

**Note:** The actual implementation uses a simpler pattern than `{domain}.{entity}.{action}`. Most topics are just `{domain}.{action}` (e.g., `search.request` not `search.query.request`).

### Topic Registry

**Actual topics from code (bus.go):**

| Topic | Publisher | Subscriber | Pattern |
|-------|-----------|------------|---------|
| `ml.embed.request` | API, Search | ML | Request/Reply |
| `ml.embed.response` | ML | API, Search | Request/Reply |
| `ml.sparse.request` | API, Search | ML | Request/Reply |
| `ml.sparse.response` | ML | API, Search | Request/Reply |
| `ml.rerank.request` | Search | ML | Request/Reply |
| `ml.rerank.response` | ML | Search | Request/Reply |
| `search.request` | API | Search | Request/Reply |
| `search.response` | Search | API | Request/Reply |
| `index.request` | API | Index | Request/Reply |
| `index.response` | Index | API | Request/Reply |
| `index.chunk.created` | Index | Metrics | Fire-and-forget |
| `store.created` | Store | API, External | Fire-and-forget |
| `store.deleted` | Store | API, External | Fire-and-forget |
| `settings.changed` | Settings | Services | Fire-and-forget |
| `model.downloaded` | ML | Admin | Fire-and-forget |
| `model.validated` | ML | Admin | Fire-and-forget |
| `connection.registered` | Connection | Admin | Fire-and-forget |
| `connection.unregistered` | Connection | Admin | Fire-and-forget |
| `connection.activity` | Connection | Metrics | Fire-and-forget |
| `alert.triggered` | Any | Monitoring | Fire-and-forget |

---

## Event Flow Diagrams

### Search → ML Event Flow

When a search request comes in, the Search service uses the event bus to request ML operations:

```
┌────────────┐     ┌────────────┐     ┌────────────┐
│   Client   │     │   Search   │     │     ML     │
│   Request  │     │   Service  │     │   Service  │
└─────┬──────┘     └─────┬──────┘     └─────┬──────┘
      │                  │                   │
      │  POST /search    │                   │
      │─────────────────>│                   │
      │                  │                   │
      │                  │  ml.embed.request │
      │                  │──────────────────>│
      │                  │                   │ (generate embeddings)
      │                  │ ml.embed.response │
      │                  │<──────────────────│
      │                  │                   │
      │                  │ ml.sparse.request │
      │                  │──────────────────>│
      │                  │                   │ (sparse encoding)
      │                  │ml.sparse.response │
      │                  │<──────────────────│
      │                  │                   │
      │                  │  (Qdrant search)  │
      │                  │                   │
      │                  │ ml.rerank.request │
      │                  │──────────────────>│
      │                  │                   │ (rerank documents)
      │                  │ml.rerank.response │
      │                  │<──────────────────│
      │                  │                   │
      │  Search Results  │                   │
      │<─────────────────│                   │
      │                  │                   │
```

### Index → ML Event Flow

When indexing documents, the Index pipeline uses the event bus for embeddings:

```
┌────────────┐     ┌────────────┐     ┌────────────┐     ┌────────────┐
│   Client   │     │   Index    │     │     ML     │     │   Qdrant   │
│   Request  │     │  Pipeline  │     │   Service  │     │            │
└─────┬──────┘     └─────┬──────┘     └─────┬──────┘     └─────┬──────┘
      │                  │                   │                  │
      │  POST /index     │                   │                  │
      │─────────────────>│                   │                  │
      │                  │                   │                  │
      │                  │ (chunk documents) │                  │
      │                  │                   │                  │
      │                  │  ml.embed.request │                  │
      │                  │──────────────────>│                  │
      │                  │                   │ (batch embed)    │
      │                  │ ml.embed.response │                  │
      │                  │<──────────────────│                  │
      │                  │                   │                  │
      │                  │ ml.sparse.request │                  │
      │                  │──────────────────>│                  │
      │                  │                   │ (sparse encode)  │
      │                  │ml.sparse.response │                  │
      │                  │<──────────────────│                  │
      │                  │                   │                  │
      │                  │                   │ upsert points    │
      │                  │──────────────────────────────────────>│
      │                  │                   │                  │
      │  Index Result    │                   │                  │
      │<─────────────────│                   │                  │
      │                  │                   │                  │
```

### Fallback Behavior

Both Search and Index services implement graceful fallback:

1. **Event bus available**: Uses `bus.Request()` for ML operations
2. **Event bus unavailable**: Falls back to direct ML service calls
3. **Event bus timeout**: Falls back to direct ML service calls

This ensures the system remains functional even if the event bus is temporarily unavailable.

---

## ML Events

### ml.embed.request

Request dense embeddings from ML service.

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "api",
    
    "texts": ["function authenticate(user)", "class UserService"],
    "normalize": true,
    "model": "jina-embed-v3"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| texts | []string | Yes | Texts to embed |
| normalize | bool | No | L2 normalize output (default: true) |
| model | string | No | Model override (default: config) |

### ml.embed.response

```json
{
    "id": "evt_def456",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "ml",
    
    "embeddings": [[0.1, 0.2, ...], [0.3, 0.4, ...]],
    "dimensions": 1536,
    "model": "jina-embed-v3",
    "latency_ms": 45,
    "error": null
}
```

| Field | Type | Description |
|-------|------|-------------|
| embeddings | [][]float32 | Dense vectors (1536 dims each) |
| dimensions | int | Vector dimensions |
| model | string | Model used |
| latency_ms | int64 | Processing time |
| error | string | Error message if failed |

### ml.sparse.request

Request sparse (SPLADE) encodings.

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "texts": ["authentication handler", "user login"]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| texts | []string | Yes | Texts to encode |

### ml.sparse.response

```json
{
    "id": "evt_def456",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "ml",
    
    "vectors": [
        {"indices": [102, 3547, 8923], "values": [0.8, 0.6, 0.4]},
        {"indices": [205, 1122], "values": [0.9, 0.3]}
    ],
    "latency_ms": 30,
    "error": null
}
```

| Field | Type | Description |
|-------|------|-------------|
| vectors | []SparseVector | Sparse vectors |
| vectors[].indices | []int32 | Token IDs with non-zero weights |
| vectors[].values | []float32 | Corresponding weights |
| latency_ms | int64 | Processing time |
| error | string | Error message if failed |

### ml.rerank.request

Request document reranking.

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "query": "authentication handler",
    "documents": [
        {"id": "doc_1", "content": "func authenticate() {...}"},
        {"id": "doc_2", "content": "func login() {...}"}
    ],
    "top_k": 10
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| query | string | Yes | Search query |
| documents | []Document | Yes | Documents to rerank |
| documents[].id | string | Yes | Document ID |
| documents[].content | string | Yes | Document content |
| top_k | int | No | Return top K (default: all) |

### ml.rerank.response

```json
{
    "id": "evt_def456",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "ml",
    
    "results": [
        {"id": "doc_1", "score": 0.95, "rank": 1},
        {"id": "doc_2", "score": 0.72, "rank": 2}
    ],
    "latency_ms": 80,
    "error": null
}
```

---

## Search Events

### search.request

Execute hybrid search. **Note:** The actual topic is `search.request` (not `search.query.request`).

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "api",
    
    "store": "default",
    "query": "authentication handler",
    "top_k": 20,
    "filters": {
        "path_prefix": "src/",
        "languages": ["go", "typescript"]
    },
    "options": {
        "sparse_weight": 0.5,
        "dense_weight": 0.5,
        "enable_reranking": true,
        "rerank_top_k": 30
    }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| store | string | Yes | Store name |
| query | string | Yes | Search query |
| top_k | int | No | Results to return (default: 20) |
| filters.path_prefix | string | No | Filter by path prefix |
| filters.languages | []string | No | Filter by languages |
| options.sparse_weight | float32 | No | BM25/sparse weight (default: 0.5) |
| options.dense_weight | float32 | No | Semantic weight (default: 0.5) |
| options.enable_reranking | bool | No | Enable reranking (default: true) |
| options.rerank_top_k | int | No | Candidates for reranking (default: 30) |

### search.response

```json
{
    "id": "evt_def456",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "results": [
        {
            "doc_id": "chunk_abc123",
            "path": "src/auth/handler.go",
            "language": "go",
            "content": "func authenticate(ctx context.Context) {...}",
            "symbols": ["authenticate", "validateToken"],
            "start_line": 45,
            "end_line": 72,
            "score": 0.92,
            "sparse_score": 0.85,
            "dense_score": 0.88
        }
    ],
    "total": 156,
    "latency_ms": 65,
    "stages": {
        "sparse_ms": 15,
        "dense_ms": 25,
        "fusion_ms": 5,
        "rerank_ms": 20
    },
    "error": null
}
```

---

## Index Events

### index.request

Index documents into a store.

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "api",
    
    "store": "default",
    "documents": [
        {
            "path": "src/main.go",
            "content": "package main\n\nfunc main() {...}",
            "language": "go"
        }
    ],
    "options": {
        "force": false,
        "chunk_size": 512,
        "chunk_overlap": 64
    }
}
```

### index.progress

Progress update during indexing.

```json
{
    "id": "evt_def456",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "store": "default",
    "processed": 50,
    "total": 100,
    "current_file": "src/auth/handler.go"
}
```

### index.complete

Indexing finished.

```json
{
    "id": "evt_ghi789",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "store": "default",
    "indexed": 95,
    "skipped": 3,
    "errors": 2,
    "chunks_created": 450,
    "latency_ms": 5000,
    "error": null
}
```

---

## Store Events

### store.created

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "name": "my-project",
    "config": {
        "embedding_model": "jina-embed-v3",
        "sparse_model": "splade-v1"
    }
}
```

### store.deleted

```json
{
    "id": "evt_abc123",
    "correlation_id": "req_xyz789",
    "timestamp": "2025-12-29T01:00:00Z",
    "source": "search",
    
    "name": "my-project"
}
```

---

## Event Bus Implementations

### MemoryBus (Default) - ✅ IMPLEMENTED

For single-process deployment. **This is the only production-ready implementation.**

| Aspect | Value |
|--------|-------|
| Latency | ~0μs |
| Persistence | No |
| Ordering | FIFO per topic |
| Use case | Monolith, development, production |
| Status | **✅ Fully implemented and tested** |

### KafkaBus - ⚠️ PARTIAL IMPLEMENTATION

For distributed deployment with persistence. **Code exists but NOT fully tested or production-ready.**

| Aspect | Value |
|--------|-------|
| Latency | 1-5ms |
| Persistence | Yes |
| Ordering | Per partition |
| Use case | Production, event replay |
| Status | **⚠️ Code exists (kafka.go) but NOT TESTED** |
| Note | Uses IBM/sarama library |

### NATSBus - ❌ NOT IMPLEMENTED

For lightweight distributed deployment. **Returns error: "NATS bus not implemented yet"** (factory.go:35)

| Aspect | Value |
|--------|-------|
| Status | **❌ NOT IMPLEMENTED** |
| Error | `"NATS bus not implemented yet"` |

### RedisBus - ❌ NOT IMPLEMENTED

For deployments already using Redis. **Returns error: "Redis Streams bus not implemented yet"** (factory.go:38)

| Aspect | Value |
|--------|-------|
| Status | **❌ NOT IMPLEMENTED** |
| Error | `"Redis Streams bus not implemented yet"` |

---

## Configuration

```bash
# Single process (default) - ✅ WORKS
EVENT_BUS=memory

# Kafka - ⚠️ CODE EXISTS BUT UNTESTED
EVENT_BUS=kafka
KAFKA_BROKERS=localhost:9092
KAFKA_CONSUMER_GROUP=rice-search

# NATS - ❌ NOT IMPLEMENTED (will return error)
EVENT_BUS=nats
NATS_URL=nats://localhost:4222

# Redis - ❌ NOT IMPLEMENTED (will return error)
EVENT_BUS=redis
REDIS_URL=redis://localhost:6379
```

**Recommendation:** Use `EVENT_BUS=memory` (default) for all deployments until distributed bus implementations are fully tested.
