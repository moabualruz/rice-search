# API Reference

Complete reference for the Rice Search REST API.

## Table of Contents

- [Base URL](#base-url)
- [Authentication](#authentication)
- [Search Endpoints](#search-endpoints)
- [Ingestion Endpoints](#ingestion-endpoints)
- [File Endpoints](#file-endpoints)
- [Settings Endpoints](#settings-endpoints)
- [Health & Metrics](#health--metrics)
- [Error Handling](#error-handling)
- [Rate Limiting](#rate-limiting)
- [Examples](#examples)

---

## Base URL

**Development:**
```
http://localhost:8000/api/v1
```

**Production:**
```
https://your-domain.com/api/v1
```

All endpoints are prefixed with `/api/v1`.

---

## Authentication

### Current Implementation

Authentication is **optional** by default. When `auth.enabled: false` in settings:
- All requests are treated as authenticated
- Default `org_id` is `"public"`
- No tokens or headers required

### Enabling Authentication

When `auth.enabled: true` in settings.yaml:
- Requests must include authentication headers
- User must have appropriate `org_id` and roles
- Admin endpoints require `admin` role

**Headers:**
```bash
Authorization: Bearer <token>
```

**See:** [Security Guide](security.md) for authentication setup

---

## Search Endpoints

### POST /api/v1/search/query

Perform a search or RAG query.

**Request Body:**
```json
{
  "query": "authentication flow",
  "mode": "search",
  "limit": 10,
  "use_bm25": true,
  "use_splade": true,
  "use_bm42": true
}
```

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `query` | string | *required* | Search query text |
| `mode` | string | `"search"` | `"search"` for retrieval, `"rag"` for Q&A |
| `limit` | integer | `10` | Maximum number of results |
| `use_bm25` | boolean | `true` | Enable BM25 lexical search |
| `use_splade` | boolean | `true` | Enable SPLADE sparse vectors |
| `use_bm42` | boolean | `true` | Enable BM42 hybrid vectors |

**Response (mode: search):**
```json
{
  "mode": "search",
  "results": [
    {
      "chunk_id": "550e8400-e29b-41d4-a716-446655440000",
      "score": 0.85,
      "rerank_score": 6.45,
      "text": "Authentication service implementation...",
      "full_path": "F:/work/rice-search/backend/src/services/auth.py",
      "filename": "auth.py",
      "file_path": "F:/work/rice-search/backend/src/services/auth.py",
      "org_id": "public",
      "chunk_type": "function",
      "language": "python",
      "start_line": 45,
      "end_line": 67,
      "symbols": ["authenticate_user"],
      "retriever_scores": {
        "bm25": 12.5,
        "splade": 18.3,
        "bm42": 0.75
      }
    }
  ],
  "retrievers": {
    "bm25": true,
    "splade": true,
    "bm42": true
  }
}
```

**Response (mode: rag):**
```json
{
  "mode": "rag",
  "answer": "Authentication is handled by the authenticate_user function in auth.py...",
  "sources": [
    {
      "full_path": "F:/work/rice-search/backend/src/services/auth.py",
      "score": 0.85,
      "text": "..."
    }
  ],
  "model": "qwen2.5-coder:1.5b"
}
```

**Example:**
```bash
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "user authentication",
    "mode": "search",
    "limit": 5,
    "use_bm25": true,
    "use_splade": true,
    "use_bm42": true
  }'
```

### GET /api/v1/search/query

Same as POST but with query parameters.

**Parameters:**

```
GET /api/v1/search/query?query=authentication&mode=search&limit=10
```

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `query` | string | *required* | Search query (URL encoded) |
| `mode` | string | `"search"` | `"search"` or `"rag"` |
| `limit` | integer | `10` | Max results |
| `use_bm25` | boolean | `true` | Enable BM25 |
| `use_splade` | boolean | `true` | Enable SPLADE |
| `use_bm42` | boolean | `true` | Enable BM42 |

**Example:**
```bash
# Basic search
curl "http://localhost:8000/api/v1/search/query?query=authentication&limit=5"

# Disable SPLADE and BM42 (BM25 only)
curl "http://localhost:8000/api/v1/search/query?query=test&use_splade=false&use_bm42=false"

# RAG mode
curl "http://localhost:8000/api/v1/search/query?query=how%20does%20auth%20work&mode=rag"
```

### GET /api/v1/search/config

Get current search configuration.

**Response:**
```json
{
  "sparse_enabled": true,
  "sparse_model": "naver/splade-cocondenser-ensembledistil",
  "embedding_model": "qwen3-embedding",
  "rrf_k": 60,
  "retrievers": {
    "bm25_enabled": true,
    "splade_enabled": true,
    "bm42_enabled": true
  }
}
```

**Example:**
```bash
curl http://localhost:8000/api/v1/search/config
```

---

## Ingestion Endpoints

### POST /api/v1/ingest/file

Upload and index a file.

**Request:**
- **Content-Type:** `multipart/form-data`
- **Fields:**
  - `file`: File to upload (binary)
  - `org_id`: Organization ID (default: `"public"`)

**Response:**
```json
{
  "status": "queued",
  "task_id": "c8d0f3a1-9b2e-4f67-8a5c-6d3e4f5g6h7i",
  "file": "F:/work/rice-search/backend/src/main.py"
}
```

**Status Codes:**
- `202 Accepted` - File queued for indexing
- `400 Bad Request` - Invalid file or missing parameters
- `500 Internal Server Error` - Indexing failed

**Example:**
```bash
# Upload single file
curl -X POST http://localhost:8000/api/v1/ingest/file \
  -F "file=@backend/src/main.py" \
  -F "org_id=backend"

# Check task status (see Celery/task endpoints)
```

**Supported File Types:**
- Code: `.py`, `.js`, `.ts`, `.tsx`, `.jsx`, `.go`, `.rs`, `.java`, `.cpp`, `.c`, `.h`
- Docs: `.md`, `.txt`, `.rst`, `.adoc`
- Config: `.yaml`, `.yml`, `.json`, `.toml`, `.ini`

---

## File Endpoints

### GET /api/v1/files/list

List all indexed files.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `org_id` | string | `"public"` | Filter by organization ID |
| `pattern` | string | `null` | Glob pattern filter (e.g., `*.py`) |

**Response:**
```json
{
  "files": [
    "F:/work/rice-search/backend/src/main.py",
    "F:/work/rice-search/backend/src/core/config.py",
    "F:/work/rice-search/frontend/src/app/page.tsx"
  ],
  "count": 3
}
```

**Example:**
```bash
# List all files
curl "http://localhost:8000/api/v1/files/list"

# List files for specific org
curl "http://localhost:8000/api/v1/files/list?org_id=backend"

# List files matching pattern
curl "http://localhost:8000/api/v1/files/list?pattern=*.py"
```

### GET /api/v1/files/content

Get content of a specific indexed file.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `path` | string | *required* | Full file path |
| `org_id` | string | `"public"` | Organization ID |

**Response:**
```json
{
  "path": "F:/work/rice-search/backend/src/main.py",
  "content": "from fastapi import FastAPI...",
  "language": "python"
}
```

**Status Codes:**
- `200 OK` - File content returned
- `404 Not Found` - File not indexed or not found

**Example:**
```bash
curl "http://localhost:8000/api/v1/files/content?path=F:/work/rice-search/backend/src/main.py"
```

---

## Settings Endpoints

### GET /api/v1/settings

Get all settings or settings with a prefix.

**Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `prefix` | string | `null` | Filter by prefix (e.g., `"models"`) |

**Response:**
```json
{
  "settings": {
    "models.embedding.dimension": 2560,
    "models.embedding.name": "qwen3-embedding",
    "search.hybrid.rrf_k": 60
  },
  "count": 3,
  "version": 42
}
```

**Example:**
```bash
# Get all settings
curl http://localhost:8000/api/v1/settings

# Get model settings only
curl "http://localhost:8000/api/v1/settings?prefix=models"

# Get search settings
curl "http://localhost:8000/api/v1/settings?prefix=search"
```

### GET /api/v1/settings/{key}

Get a specific setting by key.

**Path Parameter:**
- `key`: Setting key in dot notation (e.g., `models.embedding.dimension`)

**Response:**
```json
{
  "key": "models.embedding.dimension",
  "value": 2560
}
```

**Example:**
```bash
curl http://localhost:8000/api/v1/settings/models.embedding.dimension
```

### PUT /api/v1/settings/{key}

Update a setting at runtime (requires admin role).

**Path Parameter:**
- `key`: Setting key to update

**Request Body:**
```json
{
  "value": 1024
}
```

**Response:**
```json
{
  "message": "Setting updated and persisted to file",
  "key": "models.embedding.dimension",
  "value": 1024,
  "persisted": true,
  "version": 43
}
```

**Example:**
```bash
curl -X PUT http://localhost:8000/api/v1/settings/models.embedding.dimension \
  -H "Content-Type: application/json" \
  -d '{"value": 1024}'
```

### POST /api/v1/settings/bulk

Update multiple settings at once (requires admin role).

**Request Body:**
```json
{
  "settings": {
    "models.embedding.dimension": 1024,
    "search.hybrid.rrf_k": 80,
    "models.reranker.top_k": 20
  }
}
```

**Response:**
```json
{
  "message": "3 settings updated and persisted to file",
  "updated_keys": [
    "models.embedding.dimension",
    "search.hybrid.rrf_k",
    "models.reranker.top_k"
  ],
  "persisted": true,
  "version": 44
}
```

**Example:**
```bash
curl -X POST http://localhost:8000/api/v1/settings/bulk \
  -H "Content-Type: application/json" \
  -d '{
    "settings": {
      "models.embedding.dimension": 1024,
      "search.hybrid.rrf_k": 80
    }
  }'
```

### DELETE /api/v1/settings/{key}

Delete a setting (requires admin role).

**Path Parameter:**
- `key`: Setting key to delete

**Response:**
```json
{
  "message": "Setting models.custom_param deleted and persisted to file",
  "persisted": true
}
```

**Example:**
```bash
curl -X DELETE http://localhost:8000/api/v1/settings/models.custom_param
```

### POST /api/v1/settings/reload

Reload settings from YAML file (requires admin role).

**WARNING:** This discards all runtime changes not persisted to file.

**Response:**
```json
{
  "message": "Settings reloaded from file",
  "version": 45
}
```

**Example:**
```bash
curl -X POST http://localhost:8000/api/v1/settings/reload
```

### GET /api/v1/settings/nested/{prefix}

Get settings as nested dictionary.

**Path Parameter:**
- `prefix`: Prefix to filter (e.g., `"models"`)

**Response:**
```json
{
  "prefix": "models",
  "settings": {
    "embedding": {
      "name": "qwen3-embedding",
      "dimension": 2560,
      "timeout": 60
    },
    "reranker": {
      "enabled": true,
      "top_k": 50
    }
  }
}
```

**Example:**
```bash
curl http://localhost:8000/api/v1/settings/nested/models
```

### GET /api/v1/settings/version/current

Get current settings version.

**Response:**
```json
{
  "version": 42
}
```

**Example:**
```bash
curl http://localhost:8000/api/v1/settings/version/current
```

---

## Health & Metrics

### GET /health

Health check endpoint (outside /api/v1).

**Response:**
```json
{
  "status": "ok",
  "components": {
    "qdrant": {"status": "up", "collections": 1},
    "celery": {"status": "up", "last_task_id": "..."}
  }
}
```

**Example:**
```bash
curl http://localhost:8000/health
```

### GET /metrics

Prometheus metrics endpoint (outside /api/v1).

**Response:**
```
# HELP python_info Python platform information
# TYPE python_info gauge
python_info{implementation="CPython",major="3",minor="12"} 1.0
...
```

**Example:**
```bash
curl http://localhost:8000/metrics
```

---

## Error Handling

### Error Response Format

All errors return JSON with `detail` field:

```json
{
  "detail": "Error message describing what went wrong"
}
```

### HTTP Status Codes

| Code | Meaning | Example |
|------|---------|---------|
| `200 OK` | Request successful | Search results returned |
| `202 Accepted` | Request accepted (async) | File queued for indexing |
| `400 Bad Request` | Invalid request parameters | Missing required field |
| `401 Unauthorized` | Authentication required | Missing auth token |
| `403 Forbidden` | Insufficient permissions | Admin endpoint without admin role |
| `404 Not Found` | Resource not found | File or setting doesn't exist |
| `500 Internal Server Error` | Server error | Database connection failed |

### Common Errors

**Dimension Mismatch:**
```json
{
  "detail": "vector dimension mismatch (expected 2560, got 768)"
}
```

**File Not Found:**
```json
{
  "detail": "File not found in index: F:/path/to/file.py"
}
```

**Setting Not Found:**
```json
{
  "detail": "Setting models.custom_param not found"
}
```

**Embedding Timeout:**
```json
{
  "detail": "Embedding request timed out after 60 seconds"
}
```

---

## Rate Limiting

**Current Implementation:** No rate limiting

**Planned:** Rate limiting will be added based on:
- IP address
- API key (when auth enabled)
- Configurable limits per endpoint

---

## Examples

### Complete Workflow Example

```bash
# 1. Check health
curl http://localhost:8000/health

# 2. Upload a file for indexing
curl -X POST http://localhost:8000/api/v1/ingest/file \
  -F "file=@backend/src/main.py" \
  -F "org_id=backend"

# Response: {"status": "queued", "task_id": "...", "file": "..."}

# 3. Wait for indexing to complete (check worker logs)
docker compose -f deploy/docker-compose.yml logs -f backend-worker

# 4. List indexed files
curl http://localhost:8000/api/v1/files/list

# 5. Search for content
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "FastAPI", "limit": 10}'

# 6. Get specific file content
curl "http://localhost:8000/api/v1/files/content?path=F:/work/backend/src/main.py"

# 7. RAG query
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "How does the API work?", "mode": "rag"}'
```

### Python Client Example

```python
import requests

BASE_URL = "http://localhost:8000/api/v1"

# Search
response = requests.post(
    f"{BASE_URL}/search/query",
    json={
        "query": "authentication",
        "mode": "search",
        "limit": 5
    }
)
results = response.json()

for result in results["results"]:
    print(f"{result['full_path']} (score: {result['score']:.2f})")
    print(f"  {result['text'][:100]}...")

# Upload file
with open("test.py", "rb") as f:
    response = requests.post(
        f"{BASE_URL}/ingest/file",
        files={"file": f},
        data={"org_id": "myproject"}
    )
print(response.json())

# Update setting
response = requests.put(
    f"{BASE_URL}/settings/search.hybrid.rrf_k",
    json={"value": 80}
)
print(response.json())
```

### JavaScript/TypeScript Client Example

```typescript
const BASE_URL = 'http://localhost:8000/api/v1';

// Search
async function search(query: string, limit: number = 10) {
  const response = await fetch(`${BASE_URL}/search/query`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      query,
      mode: 'search',
      limit,
      use_bm25: true,
      use_splade: true,
      use_bm42: true
    })
  });

  return await response.json();
}

// Upload file
async function uploadFile(file: File, orgId: string = 'public') {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('org_id', orgId);

  const response = await fetch(`${BASE_URL}/ingest/file`, {
    method: 'POST',
    body: formData
  });

  return await response.json();
}

// Usage
const results = await search('authentication', 5);
console.log(results);
```

---

## API Documentation (Interactive)

**Swagger UI:** http://localhost:8000/docs

**ReDoc:** http://localhost:8000/redoc

The interactive documentation provides:
- All endpoints with request/response schemas
- Try-it-out functionality
- Schema definitions
- Authentication setup

---

## Summary

**Core Endpoints:**

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/search/query` | POST/GET | Search or RAG query |
| `/api/v1/ingest/file` | POST | Upload and index file |
| `/api/v1/files/list` | GET | List indexed files |
| `/api/v1/files/content` | GET | Get file content |
| `/api/v1/settings` | GET | Get settings |
| `/api/v1/settings/{key}` | PUT | Update setting |
| `/health` | GET | Health check |
| `/metrics` | GET | Prometheus metrics |

**Quick Reference:**
```bash
# Search
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "test", "limit": 10}'

# Upload file
curl -X POST http://localhost:8000/api/v1/ingest/file \
  -F "file=@test.py" \
  -F "org_id=public"

# List files
curl http://localhost:8000/api/v1/files/list

# Get setting
curl http://localhost:8000/api/v1/settings/models.embedding.dimension

# Update setting
curl -X PUT http://localhost:8000/api/v1/settings/search.hybrid.rrf_k \
  -H "Content-Type: application/json" \
  -d '{"value": 80}'
```

For more details:
- [Getting Started](getting-started.md) - Setup and first API calls
- [CLI Guide](cli.md) - Command-line alternative
- [Configuration](configuration.md) - Settings reference
- [Security](security.md) - Authentication and authorization

---

**[Back to Documentation Index](README.md)**
