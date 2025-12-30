# HTTP API

## Overview

Rice Search exposes a comprehensive HTTP API with:
- **REST API** (`/v1/*`) - JSON endpoints for programmatic access
- **Web UI** (`/`, `/search`, `/admin/*`) - HTML pages with HTMX interactivity
- **Metrics** (`/metrics`) - Prometheus endpoint

Base URL: `http://localhost:8080`

**Total Endpoints: 71**

> **Note**: All endpoints including the 4 Admin HTMX connection management routes (`/admin/connections/{id}/enable|disable|rename` and `/admin/mappers/{id}/yaml`) are registered in `internal/web/handlers.go`.

---

## Table of Contents

1. [Authentication](#authentication)
2. [Health & System](#health--system-endpoints)
3. [Search API](#search-endpoints)
4. [Store API](#store-endpoints)
5. [Index API](#index-endpoints)
6. [ML API](#ml-endpoints)
7. [Settings API](#settings-endpoints)
8. [Stats API](#stats-endpoints)
9. [Web UI Pages](#web-ui-pages)
10. [Admin HTMX API](#admin-htmx-api)
11. [Error Codes](#error-codes)

---

## Authentication

> ⚠️ **NOT IMPLEMENTED**: Authentication is not yet implemented. All endpoints are currently open. For production, use a reverse proxy for authentication.

| Mode | Description | Status |
|------|-------------|--------|
| `none` (default) | No authentication | ✅ Current |
| `api-key` | API key in header | ❌ Not implemented |
| `jwt` | JWT bearer token | ❌ Not implemented |

---

## Health & System Endpoints

### GET /healthz

Liveness probe. Returns 200 if process is running.

**Response:**
```json
{"status": "ok"}
```

---

### GET /readyz

Readiness probe. Returns 200 if ready to serve, 503 if ML unhealthy.

**Response (Ready):**
```json
{"status": "ready"}
```

**Response (Not Ready):**
```json
{"status": "not_ready", "reason": "ml_unhealthy"}
```

---

### GET /v1/version

Version information.

**Response:**
```json
{
    "version": "1.0.0",
    "git_commit": "abc123",
    "build_time": "2025-12-29T01:00:00Z",
    "go_version": "go1.23.0"
}
```

---

### GET /v1/health

Detailed health with ML component status.

**Response:**
```json
{
    "status": "healthy",
    "version": "1.0.0",
    "ml": {
        "healthy": true,
        "embed_loaded": true,
        "rerank_loaded": true,
        "sparse_loaded": true,
        "device": "cuda"
    }
}
```

---

### GET /metrics

Prometheus metrics endpoint.

**Response:** `text/plain` with Prometheus format

```
# HELP rice_search_requests_total Total search requests
# TYPE rice_search_requests_total counter
rice_search_requests_total{store="default"} 1234
...
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
    "sparse_weight": 0.5,
    "dense_weight": 0.5,
    "enable_reranking": true,
    "rerank_top_k": 30,
    "include_content": true
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| query | string | Yes | - | Search query |
| top_k | int | No | 20 | Results to return |
| filters.path_prefix | string | No | - | Filter by path prefix |
| filters.languages | []string | No | - | Filter by languages |
| sparse_weight | float | No | 0.5 | BM25/SPLADE weight |
| dense_weight | float | No | 0.5 | Dense embedding weight |
| enable_reranking | bool | No | true | Enable neural reranking |
| rerank_top_k | int | No | 30 | Candidates for reranking |
| include_content | bool | No | true | Include chunk content |

**Response:**
```json
{
    "query": "authentication handler",
    "store": "default",
    "results": [
        {
            "id": "chunk_abc123",
            "path": "src/auth/handler.go",
            "language": "go",
            "content": "func Authenticate(ctx context.Context) error {...}",
            "symbols": ["Authenticate", "ValidateToken"],
            "start_line": 45,
            "end_line": 72,
            "score": 0.92,
            "sparse_score": 0.85,
            "dense_score": 0.88,
            "rerank_score": 0.95
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
    }
}
```

---

## Store Endpoints

### GET /v1/stores

List all stores.

**Response:**
```json
{
    "stores": [
        {
            "name": "default",
            "display_name": "Default Store",
            "stats": {
                "document_count": 150,
                "chunk_count": 890
            },
            "created_at": "2025-12-29T01:00:00Z"
        }
    ]
}
```

---

### POST /v1/stores

Create a new store.

**Request:**
```json
{
    "name": "my-project",
    "display_name": "My Project",
    "description": "Code search for my project"
}
```

**Response:** `201 Created`
```json
{
    "name": "my-project",
    "display_name": "My Project",
    "created_at": "2025-12-29T01:00:00Z"
}
```

---

### GET /v1/stores/{name}

Get store details.

**Response:**
```json
{
    "name": "my-project",
    "display_name": "My Project",
    "description": "Code search for my project",
    "created_at": "2025-12-29T01:00:00Z"
}
```

---

### DELETE /v1/stores/{name}

Delete store and all its data.

**Response:** `204 No Content`

---

### GET /v1/stores/{name}/stats

Get store statistics.

**Response:**
```json
{
    "document_count": 150,
    "chunk_count": 890,
    "total_size": 5242880,
    "last_indexed": "2025-12-29T02:00:00Z"
}
```

---

## Index Endpoints

### POST /v1/stores/{name}/index

Index documents.

**Request:**
```json
{
    "files": [
        {
            "path": "src/main.go",
            "content": "package main\n\nfunc main() {...}",
            "language": "go"
        }
    ],
    "force": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| files | []File | Yes | Documents to index |
| files[].path | string | Yes | File path |
| files[].content | string | Yes | File content |
| files[].language | string | No | Override language detection |
| force | bool | No | Reindex even if unchanged |

**Response:**
```json
{
    "indexed": 2,
    "skipped": 0,
    "errors": 0,
    "chunks_created": 8
}
```

---

### DELETE /v1/stores/{name}/index

Delete documents from index.

**Request (by paths):**
```json
{
    "paths": ["src/old.go", "src/deprecated.go"]
}
```

**Request (by prefix):**
```json
{
    "path_prefix": "src/deprecated/"
}
```

**Response:**
```json
{
    "deleted_count": 5,
    "paths": ["src/old.go", "src/deprecated.go"]
}
```

---

### POST /v1/stores/{name}/index/sync

Sync index with filesystem (remove deleted files).

**Request:**
```json
{
    "current_paths": ["src/main.go", "src/auth.go", "src/user.go"]
}
```

**Response:**
```json
{
    "removed": ["src/old.go", "src/temp.go"]
}
```

---

### POST /v1/stores/{name}/index/reindex

Clear and rebuild entire index.

**Request:**
```json
{
    "files": [
        {"path": "src/main.go", "content": "..."}
    ]
}
```

**Response:**
```json
{
    "cleared": 150,
    "indexed": 1,
    "chunks_created": 5
}
```

---

### GET /v1/stores/{name}/index/stats

Get indexing statistics.

**Response:**
```json
{
    "total_documents": 150,
    "total_chunks": 890,
    "total_size_bytes": 5242880,
    "last_indexed": "2025-12-29T01:00:00Z",
    "languages": {
        "go": 80,
        "typescript": 45,
        "python": 25
    }
}
```

---

### GET /v1/stores/{name}/index/files

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
}
```

---

## ML Endpoints

Direct access to ML operations for debugging and testing.

### POST /v1/ml/embed

Generate dense embeddings.

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
    "count": 2
}
```

---

### POST /v1/ml/sparse

Generate sparse (SPLADE) vectors.

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
    "count": 2
}
```

---

### POST /v1/ml/rerank

Rerank documents by relevance.

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
        {"index": 0, "score": 0.95},
        {"index": 1, "score": 0.82}
    ],
    "count": 2
}
```

---

## Settings Endpoints

### GET /api/v1/settings

Get current runtime settings.

**Response:**
```json
{
    "server_host": "0.0.0.0",
    "server_port": 8080,
    "log_level": "info",
    "embed_model": "jinaai/jina-code-embeddings-1.5b",
    "rerank_model": "jinaai/jina-reranker-v2-base-multilingual",
    "default_top_k": 20,
    "enable_reranking": true,
    "version": 3,
    "updated_at": "2025-12-29T02:00:00Z"
}
```

---

### PUT /api/v1/settings

Update settings.

**Request:** (partial or full settings object)
```json
{
    "default_top_k": 30,
    "enable_reranking": false
}
```

**Response:** Updated settings object

---

### GET /api/v1/settings/history

Get settings version history.

**Query Parameters:**
- `limit` (int, default 10): Number of versions to return

**Response:**
```json
{
    "history": [
        {"version": 3, "updated_at": "2025-12-29T02:00:00Z", "updated_by": "admin"},
        {"version": 2, "updated_at": "2025-12-29T01:00:00Z", "updated_by": "api"}
    ],
    "count": 2
}
```

---

### GET /api/v1/settings/audit

Get settings audit log.

**Query Parameters:**
- `limit` (int, default 50): Number of entries to return

**Response:**
```json
{
    "entries": [
        {
            "timestamp": "2025-12-29T02:00:00Z",
            "action": "update",
            "actor": "admin",
            "changes": {"default_top_k": {"from": 20, "to": 30}}
        }
    ],
    "count": 1
}
```

---

### POST /api/v1/settings/rollback/{version}

Rollback settings to a specific version.

**Response:**
```json
{
    "message": "rollback successful",
    "rolled_back_to": 2,
    "new_version": 4,
    "current_settings": {...}
}
```

---

## Stats Endpoints

JSON API for monitoring and Grafana integration.

### GET /api/v1/stats/overview

Overall statistics.

**Query Parameters:**
- `store` (string): Filter by store
- `time_range` (string, default "1h"): Time range

**Response:**
```json
{
    "data": {
        "total_stores": 3,
        "total_files": 1500,
        "total_chunks": 8900,
        "total_connections": 5,
        "total_searches": 12345
    },
    "meta": {
        "store": "",
        "time_range": "1h",
        "generated_at": "2025-12-29T02:00:00Z"
    }
}
```

---

### GET /api/v1/stats/search-timeseries

Search rate over time.

**Query Parameters:**
- `store` (string): Filter by store
- `time_range` (string, default "1h"): Time range
- `granularity` (string, default "5m"): Bucket size

**Response:**
```json
{
    "data": [
        {"timestamp": "2025-12-29T01:00:00Z", "value": 45},
        {"timestamp": "2025-12-29T01:05:00Z", "value": 52}
    ],
    "meta": {"time_range": "1h", "granularity": "5m"}
}
```

---

### GET /api/v1/stats/index-timeseries

Indexing rate over time (same format as search-timeseries).

---

### GET /api/v1/stats/latency-timeseries

Search latency over time (same format as search-timeseries).

---

### GET /api/v1/stats/stores

Per-store breakdown.

**Response:**
```json
{
    "data": [
        {"store": "default", "file_count": 150, "chunk_count": 890},
        {"store": "my-project", "file_count": 45, "chunk_count": 234}
    ],
    "meta": {"time_range": "1h"}
}
```

---

### GET /api/v1/stats/languages

Language breakdown.

**Query Parameters:**
- `store` (string, default "default"): Store to analyze

**Response:**
```json
{
    "data": [
        {"language": "go", "file_count": 80, "chunk_count": 450, "percentage": 53.3},
        {"language": "typescript", "file_count": 45, "chunk_count": 280, "percentage": 30.0}
    ],
    "meta": {"store": "default"}
}
```

---

### GET /api/v1/stats/connections

Connection activity metrics.

**Response:**
```json
{
    "data": [
        {
            "connection_id": "abc123",
            "connection_name": "dev-laptop",
            "files_indexed": 150,
            "search_count": 234,
            "is_active": true,
            "last_active": "2025-12-29T02:00:00Z"
        }
    ],
    "meta": {"time_range": "1h"}
}
```

---

### GET /api/v1/events

Get logged events (for debugging).

**Query Parameters:**
- `since` (timestamp, default 1h ago): Start time
- `limit` (int, default 50, max 1000): Number of events

**Response:**
```json
{
    "events": [...],
    "count": 50,
    "meta": {"since": "2025-12-29T01:00:00Z", "limit": 50}
}
```

---

## Web UI Pages

HTML pages with HTMX interactivity.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Dashboard - overview, health, quick stats |
| GET | `/search` | Search page with 12+ options |
| GET | `/stores` | Store management |
| GET | `/stores/{name}` | Store detail with connections |
| GET | `/stores/{name}/files` | File browser for store |
| GET | `/stores/{name}/files/{path...}` | File detail with chunks |
| GET | `/files` | Global file browser |
| GET | `/stats` | Time-series dashboards |
| GET | `/admin` | Redirects to /stores |
| GET | `/admin/models` | Model management (download, GPU toggle) |
| GET | `/admin/mappers` | Model I/O mappings |
| GET | `/admin/connections` | Connection management |
| GET | `/admin/settings` | 80+ runtime settings |

---

## Admin HTMX API

HTMX endpoints that return HTML fragments.

### Search

| Method | Path | Description |
|--------|------|-------------|
| POST | `/search` | Execute search, return results HTML |

### Stores

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/stores` | Create store |
| DELETE | `/admin/stores/{name}` | Delete store |

### Files

| Method | Path | Description |
|--------|------|-------------|
| POST | `/files/{path...}/reindex` | Reindex file (requires CLI) |
| DELETE | `/files/{path...}` | Delete file from index |
| GET | `/stores/{name}/files/export` | Export files (CSV/JSON) |

### Models

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/models/{id}/download` | Start model download |
| POST | `/admin/models/{id}/default` | Set as default for type |
| POST | `/admin/models/{id}/gpu` | Toggle GPU (form: enabled=true/false) |
| DELETE | `/admin/models/{id}` | Delete model files |

### Mappers

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/mappers` | Create mapper |
| PUT | `/admin/mappers/{id}` | Update mapper |
| DELETE | `/admin/mappers/{id}` | Delete mapper |
| GET | `/admin/mappers/{id}/yaml` | Get mapper as YAML |
| POST | `/admin/mappers/generate` | Auto-generate mapper for model |

### Connections

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/connections/{id}/enable` | Enable connection |
| POST | `/admin/connections/{id}/disable` | Disable connection |
| POST | `/admin/connections/{id}/rename` | Rename (form: name=...) |
| DELETE | `/admin/connections/{id}` | Delete connection |

### Settings

| Method | Path | Description |
|--------|------|-------------|
| POST | `/admin/settings` | Save all settings (form data) |
| GET | `/admin/settings/export` | Download as YAML/JSON |
| POST | `/admin/settings/import` | Upload settings file |
| POST | `/admin/settings/reset` | Reset to defaults |

### Stats

| Method | Path | Description |
|--------|------|-------------|
| GET | `/stats/refresh` | Refresh stats content |

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

### Error Response Format

```json
{
    "error": "STORE_NOT_FOUND",
    "message": "Store 'foo' does not exist",
    "code": "STORE_NOT_FOUND"
}
```

---

## Rate Limiting

> ⚠️ **NOT IMPLEMENTED**: Rate limiting is not yet implemented. For production, use a reverse proxy.

---

## Endpoint Summary

| Category | Count | Examples |
|----------|-------|----------|
| Health & System | 5 | `/healthz`, `/readyz`, `/metrics` |
| Search | 2 | `/v1/search`, `/v1/stores/{store}/search` |
| Stores | 5 | `/v1/stores`, `/v1/stores/{name}` |
| Index | 6 | `/v1/stores/{name}/index`, `index/files` |
| ML | 3 | `/v1/ml/embed`, `/v1/ml/rerank` |
| Settings REST | 5 | `/api/v1/settings`, `settings/rollback` |
| Stats REST | 8 | `/api/v1/stats/*` |
| Web UI Pages | 13 | `/`, `/search`, `/admin/*` |
| Admin HTMX | 24 | `/admin/stores/*`, `/admin/models/*` |
| **Total** | **71** | |
