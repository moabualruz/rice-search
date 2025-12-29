# Qdrant Schema

## Overview

Qdrant stores both sparse (SPLADE) and dense (Jina) vectors with native RRF fusion.

---

## Collection Naming

```
{prefix}_{store}

Examples:
- rice_default
- rice_myproject
```

---

## Collection Schema

### Create Collection

```json
PUT /collections/rice_default
{
    "vectors": {
        "dense": {
            "size": 1536,
            "distance": "Cosine",
            "on_disk": false
        }
    },
    "sparse_vectors": {
        "sparse": {
            "index": {
                "on_disk": false,
                "full_scan_threshold": 10000
            }
        }
    },
    "on_disk_payload": true,
    "optimizers_config": {
        "indexing_threshold": 20000,
        "memmap_threshold": 50000
    },
    "replication_factor": 1,
    "write_consistency_factor": 1
}
```

### Vector Configuration

| Vector | Type | Size | Distance | Description |
|--------|------|------|----------|-------------|
| `dense` | Named | 1536 | Cosine | Jina embeddings |
| `sparse` | Sparse | Variable | Dot | SPLADE vectors |

---

## Point Structure

### Full Point

```json
{
    "id": "chunk_a1b2c3d4e5f6",
    "vector": {
        "dense": [0.12, 0.34, 0.56, ...],
        "sparse": {
            "indices": [102, 3547, 8923, 15234],
            "values": [0.82, 0.65, 0.43, 0.21]
        }
    },
    "payload": {
        "store": "default",
        "path": "src/auth/handler.go",
        "language": "go",
        "content": "func Authenticate(ctx context.Context) error {\n    ...\n}",
        "symbols": ["Authenticate", "ValidateToken"],
        "start_line": 45,
        "end_line": 72,
        "document_hash": "sha256:abc123...",
        "chunk_hash": "sha256:def456...",
        "indexed_at": "2025-12-29T01:00:00Z"
    }
}
```

### Point ID

```
chunk_id = first 12 chars of SHA256(store + ":" + path + ":" + start_line + ":" + end_line)
```

Example: `chunk_a1b2c3d4e5f6`

---

## Payload Fields

| Field | Type | Indexed | Description |
|-------|------|---------|-------------|
| `store` | keyword | Yes | Store name |
| `path` | text | Yes | File path |
| `language` | keyword | Yes | Programming language |
| `content` | text | No | Chunk content |
| `symbols` | keyword[] | Yes | Function/class names |
| `start_line` | integer | Yes | Start line number |
| `end_line` | integer | Yes | End line number |
| `document_hash` | keyword | Yes | Parent document hash |
| `chunk_hash` | keyword | Yes | Chunk content hash |
| `indexed_at` | datetime | Yes | Index timestamp |

### Payload Indexes

```json
PUT /collections/rice_default/index
{
    "field_name": "path",
    "field_schema": "text"
}

PUT /collections/rice_default/index
{
    "field_name": "language",
    "field_schema": "keyword"
}

PUT /collections/rice_default/index
{
    "field_name": "symbols",
    "field_schema": "keyword"
}

PUT /collections/rice_default/index
{
    "field_name": "document_hash",
    "field_schema": "keyword"
}
```

---

## Search Queries

### Hybrid Search (Sparse + Dense + RRF)

```json
POST /collections/rice_default/points/query
{
    "prefetch": [
        {
            "query": {
                "indices": [102, 3547, 8923],
                "values": [0.82, 0.65, 0.43]
            },
            "using": "sparse",
            "limit": 100
        },
        {
            "query": [0.12, 0.34, 0.56, ...],
            "using": "dense",
            "limit": 100
        }
    ],
    "query": {
        "fusion": "rrf"
    },
    "limit": 30,
    "with_payload": true
}
```

### With Filters

```json
POST /collections/rice_default/points/query
{
    "prefetch": [
        {
            "query": {"indices": [...], "values": [...]},
            "using": "sparse",
            "limit": 100,
            "filter": {
                "must": [
                    {"key": "language", "match": {"any": ["go", "typescript"]}},
                    {"key": "path", "match": {"text": "src/"}}
                ]
            }
        },
        {
            "query": [0.12, 0.34, ...],
            "using": "dense",
            "limit": 100,
            "filter": {
                "must": [
                    {"key": "language", "match": {"any": ["go", "typescript"]}},
                    {"key": "path", "match": {"text": "src/"}}
                ]
            }
        }
    ],
    "query": {"fusion": "rrf"},
    "limit": 30,
    "with_payload": true
}
```

### Sparse Only

```json
POST /collections/rice_default/points/query
{
    "query": {
        "indices": [102, 3547, 8923],
        "values": [0.82, 0.65, 0.43]
    },
    "using": "sparse",
    "limit": 30,
    "with_payload": true
}
```

### Dense Only

```json
POST /collections/rice_default/points/query
{
    "query": [0.12, 0.34, 0.56, ...],
    "using": "dense",
    "limit": 30,
    "with_payload": true
}
```

---

## Upsert Operations

### Batch Upsert

```json
PUT /collections/rice_default/points
{
    "points": [
        {
            "id": "chunk_a1b2c3d4e5f6",
            "vector": {
                "dense": [0.12, 0.34, ...],
                "sparse": {"indices": [102, 3547], "values": [0.82, 0.65]}
            },
            "payload": {...}
        },
        {
            "id": "chunk_b2c3d4e5f6g7",
            "vector": {...},
            "payload": {...}
        }
    ],
    "wait": true
}
```

### Upsert Batch Size

| Batch Size | Throughput | Memory |
|------------|------------|--------|
| 100 | ~500 points/sec | Low |
| 500 | ~1000 points/sec | Medium |
| 1000 | ~1200 points/sec | High |

Recommended: 100-500 per batch.

---

## Delete Operations

### Delete by IDs

```json
POST /collections/rice_default/points/delete
{
    "points": ["chunk_a1b2c3d4e5f6", "chunk_b2c3d4e5f6g7"]
}
```

### Delete by Filter (Path)

```json
POST /collections/rice_default/points/delete
{
    "filter": {
        "must": [
            {"key": "path", "match": {"value": "src/old.go"}}
        ]
    }
}
```

### Delete by Filter (Path Prefix)

```json
POST /collections/rice_default/points/delete
{
    "filter": {
        "must": [
            {"key": "path", "match": {"text": "src/deprecated/"}}
        ]
    }
}
```

### Delete by Document Hash

```json
POST /collections/rice_default/points/delete
{
    "filter": {
        "must": [
            {"key": "document_hash", "match": {"value": "sha256:abc123..."}}
        ]
    }
}
```

---

## Collection Management

### List Collections

```json
GET /collections

Response:
{
    "result": {
        "collections": [
            {"name": "rice_default"},
            {"name": "rice_myproject"}
        ]
    }
}
```

### Collection Info

```json
GET /collections/rice_default

Response:
{
    "result": {
        "status": "green",
        "vectors_count": 45000,
        "points_count": 45000,
        "segments_count": 4,
        "config": {...}
    }
}
```

### Delete Collection

```json
DELETE /collections/rice_default
```

---

## Performance Tuning

### Indexing Threshold

```json
{
    "optimizers_config": {
        "indexing_threshold": 20000
    }
}
```

- Below threshold: Linear scan (fast for small collections)
- Above threshold: HNSW index built

### Memory Mapping

```json
{
    "optimizers_config": {
        "memmap_threshold": 50000
    }
}
```

- Below threshold: In-memory
- Above threshold: Memory-mapped files

### On-Disk Payload

```json
{
    "on_disk_payload": true
}
```

For large payloads (content field), store on disk to save RAM.

---

## Qdrant Resources

| Documents | Chunks | RAM (approx) | Disk |
|-----------|--------|--------------|------|
| 1K | 5K | ~500MB | ~100MB |
| 10K | 50K | ~2GB | ~500MB |
| 100K | 500K | ~10GB | ~3GB |
| 1M | 5M | ~50GB | ~20GB |

Recommendations:
- <100K chunks: Single Qdrant, in-memory
- 100K-1M chunks: Single Qdrant, memmap
- 1M+ chunks: Qdrant cluster or sharding
