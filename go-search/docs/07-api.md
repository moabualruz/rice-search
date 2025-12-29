# HTTP API

## Overview

Every functionality exposed as HTTP endpoint. RESTful JSON API.

Base URL: `http://localhost:8080`

---

## Authentication

> ⚠️ **NOT IMPLEMENTED**: Authentication is not yet implemented. All endpoints are currently open. For production, use a reverse proxy for authentication.

| Mode | Description | Status |
|------|-------------|--------|
| `none` (default) | No authentication | ✅ Current |
| `api-key` | API key in header | ❌ Not implemented |
| `jwt` | JWT bearer token | ❌ Not implemented |

```bash
# FUTURE - Not yet supported
# API key mode
AUTH_MODE=api-key
API_KEYS=key1,key2,key3

# JWT mode
AUTH_MODE=jwt
JWT_SECRET=your-secret
```

### Headers (Future)

```http
# API key
X-API-Key: your-api-key

# JWT
Authorization: Bearer eyJhbG...
```

---

## Common Response Format

> **Note**: Current implementation returns responses directly without wrappers. The `data`/`meta` wrapper pattern shown in some examples below is a design goal but NOT YET IMPLEMENTED. Responses are returned flat.

### Success (Current - Direct Response)

```json
{
    "query": "authentication handler",
    "store": "default",
    "results": [...],
    "total": 20,
    "metadata": {
        "search_time_ms": 65,
        "reranking_applied": true
    }
}
```

### Success (Future - With Wrapper)

> **NOT IMPLEMENTED**: The wrapper pattern below is a design goal for future versions.

```json
{
    "data": { ... },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 45
    }
}
```

### Error

```json
{
    "error": {
        "code": "STORE_NOT_FOUND",
        "message": "Store 'foo' does not exist",
        "details": { ... }
    }
}
```

---

## Search Endpoints

### POST /v1/search

Convenience endpoint using default store.

**Request:**

```json
{
    "query": "authentication handler",
    "top_k": 20
}
```

**Response:** Same as `/v1/stores/{store}/search`

---

### POST /v1/stores/{store}/search

Full search with all options.

**Request:**

```json
{
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

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| query | string | Yes | - | Search query |
| top_k | int | No | 20 | Results to return |
| filters.path_prefix | string | No | - | Filter by path prefix |
| filters.languages | []string | No | - | Filter by languages |
| options.sparse_weight | float | No | 0.5 | Sparse retriever weight |
| options.dense_weight | float | No | 0.5 | Dense retriever weight |
| options.enable_reranking | bool | No | true | Enable reranking |
| options.rerank_top_k | int | No | 30 | Candidates for reranking |

**Response:**

```json
{
    "data": {
        "query": "authentication handler",
        "store": "default",
        "results": [
            {
                "doc_id": "chunk_abc123",
                "path": "src/auth/handler.go",
                "language": "go",
                "content": "func Authenticate(ctx context.Context) error {\n    ...\n}",
                "symbols": ["Authenticate", "ValidateToken"],
                "start_line": 45,
                "end_line": 72,
                "score": 0.92,
                "sparse_score": 0.85,
                "dense_score": 0.88
            }
        ],
        "total": 156,
        "metadata": {
            "search_time_ms": 65,
            "embed_time_ms": 20,
            "retrieval_time_ms": 30,
            "rerank_time_ms": 15,
            "candidates_reranked": 30,
            "reranking_applied": true
        },
        "parsed_query": {
            "original": "authentication handler",
            "normalized": "authentication handler",
            "keywords": ["authentication", "handler"],
            "code_terms": ["handler"],
            "action_intent": "find",
            "target_type": "function",
            "expanded": ["auth", "authenticate", "login"],
            "search_query": "authentication handler auth authenticate login",
            "confidence": 0.92,
            "used_model": true
        }
    },
    "meta": {
        "request_id": "req_xyz789",
        "latency_ms": 65
    }
}
```

---

## Index Endpoints

### POST /v1/stores/{store}/index

Index documents.

**Request:**

```json
{
    "documents": [
        {
            "path": "src/main.go",
            "content": "package main\n\nfunc main() {\n    ...\n}"
        },
        {
            "path": "src/auth.go",
            "content": "package auth\n\nfunc Authenticate() {\n    ...\n}"
        }
    ],
    "options": {
        "force": false
    }
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| documents | []Document | Yes | Documents to index |
| documents[].path | string | Yes | File path |
| documents[].content | string | Yes | File content |
| options.force | bool | No | Reindex even if unchanged |

**Response:**

```json
{
    "data": {
        "indexed": 2,
        "skipped": 0,
        "errors": 0,
        "chunks_created": 8
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 1500
    }
}
```

---

### DELETE /v1/stores/{store}/index

Delete documents from index.

**Request:**

```json
{
    "paths": ["src/old.go", "src/deprecated.go"]
}
```

Or delete by prefix:

```json
{
    "path_prefix": "src/deprecated/"
}
```

**Response:**

```json
{
    "data": {
        "deleted": 5
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 50
    }
}
```

---

### POST /v1/stores/{store}/index/sync

Sync index with filesystem (remove deleted files).

**Request:**

```json
{
    "current_paths": [
        "src/main.go",
        "src/auth.go",
        "src/user.go"
    ]
}
```

**Response:**

```json
{
    "data": {
        "removed": 3,
        "removed_paths": ["src/old.go", "src/temp.go", "src/test.go"]
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 100
    }
}
```

---

### POST /v1/stores/{store}/index/reindex

Clear and rebuild entire index for a store.

**Request:**

```json
{
    "documents": [
        {
            "path": "src/main.go",
            "content": "package main\n\nfunc main() {\n    ...\n}"
        }
    ]
}
```

**Response:**

```json
{
    "data": {
        "cleared": 150,
        "indexed": 1,
        "chunks_created": 5
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 2000
    }
}
```

---

### GET /v1/stores/{store}/index/stats

Get indexing statistics for a store.

**Response:**

```json
{
    "data": {
        "total_documents": 150,
        "total_chunks": 890,
        "total_size_bytes": 5242880,
        "last_indexed": "2025-12-29T01:00:00Z",
        "languages": {
            "go": 80,
            "typescript": 45,
            "python": 25
        }
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 15
    }
}
```

---

### GET /v1/stores/{store}/index/files

List indexed files with pagination.

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| page | int | 1 | Page number (1-indexed) |
| page_size | int | 50 | Results per page |
| path | string | - | Filter by path substring |
| language | string | - | Filter by language |
| sort_by | string | path | Sort field (path, size, indexed_at) |
| sort_order | string | asc | Sort order (asc, desc) |

**Response:**

```json
{
    "data": {
        "files": [
            {
                "path": "src/auth.go",
                "language": "go",
                "size": 2048,
                "hash": "a1b2c3d4e5f6",
                "indexed_at": "2025-12-29T01:00:00Z",
                "chunk_count": 5
            }
        ],
        "total": 150,
        "page": 1,
        "page_size": 50,
        "total_pages": 3
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 25
    }
}
```

---

## ML Endpoints

ML operations are available as HTTP endpoints for direct invocation and debugging.

### POST /v1/ml/embed

Generate dense embeddings for texts.

**Request:**

```json
{
    "texts": ["function authenticate(user)", "class UserService"]
}
```

**Response:**

```json
{
    "embeddings": [
        [0.123, 0.456, ...],
        [0.789, 0.012, ...]
    ],
    "model": "jina-embeddings-v3",
    "dimensions": 1536,
    "latency_ms": 45
}
```

---

### POST /v1/ml/sparse

Generate sparse (SPLADE) vectors for texts.

**Request:**

```json
{
    "texts": ["authentication handler", "user login function"]
}
```

**Response:**

```json
{
    "vectors": [
        {"indices": [123, 456, 789], "values": [1.2, 0.8, 0.5]},
        {"indices": [234, 567], "values": [1.5, 0.9]}
    ],
    "model": "splade-pp-en-v1",
    "latency_ms": 30
}
```

---

### POST /v1/ml/rerank

Rerank documents by relevance to query.

**Request:**

```json
{
    "query": "authentication handler",
    "documents": [
        "func Authenticate(ctx context.Context) error",
        "func HandleLogin(user string) bool",
        "type Config struct { Port int }"
    ],
    "top_k": 2
}
```

**Response:**

```json
{
    "results": [
        {"index": 0, "score": 0.95, "document": "func Authenticate..."},
        {"index": 1, "score": 0.82, "document": "func HandleLogin..."}
    ],
    "model": "jina-reranker-v2",
    "latency_ms": 25
}
```

---

> **Note**: For high-throughput scenarios, ML operations are also available via the internal event bus. The search and index services use the event bus for better batching and resource management.

---

## Store Endpoints

### GET /v1/stores

List all stores.

**Response:**

```json
{
    "data": {
        "stores": [
            {
                "name": "default",
                "document_count": 150,
                "chunk_count": 890,
                "created_at": "2025-12-29T01:00:00Z"
            },
            {
                "name": "my-project",
                "document_count": 45,
                "chunk_count": 234,
                "created_at": "2025-12-29T02:00:00Z"
            }
        ]
    }
}
```

---

### POST /v1/stores

Create store.

**Request:**

```json
{
    "name": "my-project",
    "display_name": "My Project",
    "description": "Code search for my project"
}
```

**Response:**

```json
{
    "data": {
        "name": "my-project",
        "display_name": "My Project",
        "created_at": "2025-12-29T01:00:00Z"
    }
}
```

---

### GET /v1/stores/{store}

Get store details.

**Response:**

```json
{
    "data": {
        "name": "my-project",
        "display_name": "My Project",
        "description": "Code search for my project",
        "config": {
            "embed_model": "jina-embed-v3",
            "sparse_model": "splade-v1",
            "chunk_size": 512,
            "chunk_overlap": 64
        },
        "stats": {
            "document_count": 45,
            "chunk_count": 234,
            "total_size_bytes": 1250000,
            "last_indexed": "2025-12-29T01:00:00Z"
        },
        "created_at": "2025-12-29T01:00:00Z",
        "updated_at": "2025-12-29T02:00:00Z"
    }
}
```

---

### DELETE /v1/stores/{store}

Delete store and all its data.

**Response:**

```json
{
    "data": {
        "deleted": true
    }
}
```

---

## Health Endpoints

### GET /healthz

Liveness probe. Returns 200 if process is running.

**Response:**

```json
{
    "status": "ok"
}
```

---

### GET /readyz

Readiness probe. Returns 200 if ready to serve requests.

**Response:**

```json
{
    "status": "ready",
    "checks": {
        "qdrant": "ok",
        "models": "ok",
        "event_bus": "ok"
    }
}
```

If not ready:

```json
{
    "status": "not_ready",
    "checks": {
        "qdrant": "ok",
        "models": "loading",
        "event_bus": "ok"
    }
}
```

---

### GET /v1/version

Version information.

**Response:**

```json
{
    "data": {
        "version": "1.0.0",
        "git_commit": "abc123",
        "build_time": "2025-12-29T01:00:00Z",
        "go_version": "1.23.0"
    }
}
```

---

### GET /metrics

Prometheus metrics endpoint.

**Response:** (text/plain)

```
# HELP rice_search_requests_total Total search requests
# TYPE rice_search_requests_total counter
rice_search_requests_total{store="default"} 1234

# HELP rice_search_latency_seconds Search latency
# TYPE rice_search_latency_seconds histogram
rice_search_latency_seconds_bucket{le="0.1"} 500
rice_search_latency_seconds_bucket{le="0.5"} 950
...
```

---

## Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_REQUEST` | 400 | Malformed request body |
| `VALIDATION_FAILED` | 400 | Request validation failed |
| `UNAUTHORIZED` | 401 | Missing or invalid auth |
| `STORE_NOT_FOUND` | 404 | Store doesn't exist |
| `DOCUMENT_NOT_FOUND` | 404 | Document doesn't exist |
| `STORE_EXISTS` | 409 | Store already exists |
| `RATE_LIMITED` | 429 | Too many requests |
| `INTERNAL_ERROR` | 500 | Server error |
| `SERVICE_UNAVAILABLE` | 503 | Dependency unavailable |

---

## Rate Limiting

> ⚠️ **NOT IMPLEMENTED**: Rate limiting is not yet implemented. For production, use a reverse proxy for rate limiting.

| Endpoint | Planned Limit | Status |
|----------|---------------|--------|
| Search | 100 req/min | ❌ Not implemented |
| Index | 20 req/min | ❌ Not implemented |
| ML endpoints | 200 req/min | ❌ Not implemented |
| Other | 300 req/min | ❌ Not implemented |

Headers (Future):

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1735430400
```
