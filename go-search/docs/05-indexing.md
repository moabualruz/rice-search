# Indexing Pipeline

## Overview

The indexing pipeline transforms source files into searchable chunks with vectors.

---

## Pipeline Flow

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            INDEXING PIPELINE                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  1. INPUT                                                                   │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  { path: "src/auth.go", content: "package auth\n\nfunc..." }    │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  2. DEDUPLICATION CHECK                                                     │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  hash = SHA256(content)                                          │    │
│     │  if hash == stored_hash → SKIP (unless force=true)               │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  3. LANGUAGE DETECTION                                                      │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  language = detectLanguage(path) → "go"                          │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  4. SYMBOL EXTRACTION                                                       │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  symbols = extractSymbols(content, language)                     │    │
│     │  → ["Authenticate", "ValidateToken", "User"]                     │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  5. CHUNKING                                                                │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  chunks = chunkCode(content, language, chunkSize, overlap)       │    │
│     │  → [chunk1, chunk2, chunk3]                                      │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                    │                                        │
│                                    ▼                                        │
│  6. EMBEDDING (parallel per chunk)                                          │
│     ┌─────────────────────┐    ┌─────────────────────┐                     │
│     │   SPLADE Encode     │    │   Dense Embed       │                     │
│     │   → sparse vector   │    │   → dense vector    │                     │
│     └──────────┬──────────┘    └──────────┬──────────┘                     │
│                │                          │                                 │
│                └────────────┬─────────────┘                                 │
│                             ▼                                               │
│  7. STORE IN QDRANT                                                         │
│     ┌─────────────────────────────────────────────────────────────────┐    │
│     │  qdrant.upsert(collection, points=[                              │    │
│     │      {id, vector: {sparse, dense}, payload: {path, content...}} │    │
│     │  ])                                                              │    │
│     └─────────────────────────────────────────────────────────────────┘    │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Chunking Strategy

### Semantic Chunking

Chunks are created at semantic boundaries (functions, classes, blocks).

```
┌─────────────────────────────────────────────────────────────────┐
│  Original File (src/auth.go)                                    │
├─────────────────────────────────────────────────────────────────┤
│  package auth                                                   │
│                                                                 │
│  import "context"                          ─┐                   │
│                                             │ Chunk 1           │
│  func Authenticate(ctx context.Context,    │ (imports +        │
│      user string, pass string) error {     │  Authenticate)    │
│      // validate user                       │                   │
│      if user == "" {                       │                   │
│          return ErrEmptyUser               │                   │
│      }                                     │                   │
│      // check password                      │                   │
│      return validatePassword(user, pass)  ─┘                   │
│  }                                                              │
│                                            ─┐                   │
│  func ValidateToken(token string) bool {   │ Chunk 2           │
│      // parse token                         │ (ValidateToken)   │
│      claims, err := parseJWT(token)        │                   │
│      if err != nil {                       │                   │
│          return false                       │                   │
│      }                                     │                   │
│      return claims.Valid()                 │                   │
│  }                                        ─┘                   │
└─────────────────────────────────────────────────────────────────┘
```

### Chunking Rules

| Rule | Description |
|------|-------------|
| Respect boundaries | Never split mid-function or mid-class |
| Target size | ~512 tokens per chunk |
| Overlap | 64 tokens overlap between chunks |
| Minimum size | 32 tokens minimum per chunk |
| Maximum size | 2048 tokens maximum per chunk |
| Small files | Keep as single chunk if < 512 tokens |

### Language-Specific Boundaries

| Language | Chunk Boundaries |
|----------|------------------|
| Go | Functions, methods, type declarations |
| TypeScript/JS | Functions, classes, exports |
| Python | Functions, classes, top-level statements |
| Rust | Functions, impl blocks, modules |
| Java | Classes, methods |
| C/C++ | Functions, structs, classes |
| Markdown | Headings (##) |
| JSON/YAML | Top-level keys |

---

## Symbol Extraction

Symbols help boost search relevance.

### What's Extracted

| Language | Symbols |
|----------|---------|
| Go | Functions, methods, types, constants |
| TypeScript | Functions, classes, interfaces, exports |
| Python | Functions, classes, decorators |
| Rust | Functions, structs, enums, traits |

### Extraction Example

```go
// Input
func Authenticate(ctx context.Context) error { ... }
type User struct { ... }
const MaxRetries = 3

// Extracted symbols
["Authenticate", "User", "MaxRetries"]
```

---

## Deduplication

### Content Hash

```go
hash = SHA256(content)
```

### Dedup Logic

```
if existingDoc.Hash == newDoc.Hash:
    if force:
        delete existing chunks
        reindex
    else:
        skip (no changes)
else:
    delete existing chunks
    reindex
```

### Chunk-Level Dedup

Each chunk also has a hash to avoid re-embedding unchanged chunks:

```
chunk_hash = SHA256(store + path + start_line + end_line + content)
```

---

## Embedding

### Batching

For efficiency, embeddings are batched:

| Setting | Default | Description |
|---------|---------|-------------|
| Batch size | 32 | Chunks per embedding call |
| Max parallel | 4 | Concurrent embedding batches |

### Caching

Embeddings are cached by content hash:

```
cache_key = SHA256(model + content)
```

If cache hit, skip embedding. Cache is permanent (same content = same embedding).

---

## Qdrant Storage

### Point Structure

```json
{
    "id": "chunk_abc123def456",
    "vector": {
        "dense": [0.12, 0.34, ...],
        "sparse": {
            "indices": [102, 3547, 8923],
            "values": [0.8, 0.6, 0.4]
        }
    },
    "payload": {
        "store": "default",
        "path": "src/auth.go",
        "language": "go",
        "content": "func Authenticate(...) { ... }",
        "symbols": ["Authenticate"],
        "start_line": 5,
        "end_line": 15,
        "document_hash": "sha256_of_file",
        "indexed_at": "2025-12-29T01:00:00Z"
    }
}
```

### Upsert Strategy

```python
# Upsert = insert or update by ID
qdrant.upsert(
    collection_name=f"rice_{store}",
    points=points,
    wait=True  # Wait for indexing
)
```

---

## Delete Operations

### Delete by Path

```python
qdrant.delete(
    collection_name=f"rice_{store}",
    points_selector=FilterSelector(
        filter=Filter(
            must=[
                FieldCondition(key="path", match=MatchValue(value=path))
            ]
        )
    )
)
```

### Delete by Path Prefix

```python
qdrant.delete(
    collection_name=f"rice_{store}",
    points_selector=FilterSelector(
        filter=Filter(
            must=[
                FieldCondition(key="path", match=MatchText(text="src/deprecated/"))
            ]
        )
    )
)
```

### Sync (Remove Deleted Files)

```python
# Get all paths in store
stored_paths = get_all_paths(store)

# Find paths that no longer exist
deleted = stored_paths - current_paths

# Delete removed paths
for path in deleted:
    delete_by_path(store, path)
```

---

## Error Handling

### Recoverable Errors

| Error | Action |
|-------|--------|
| Embedding timeout | Retry 3 times with backoff |
| Qdrant connection error | Retry 3 times with backoff |
| Single file parse error | Skip file, log error, continue |

### Fatal Errors

| Error | Action |
|-------|--------|
| Store doesn't exist | Return error |
| Qdrant unavailable after retries | Return error, rollback |
| Out of memory | Return error |

### Partial Failure

Index operation returns detailed results:

```json
{
    "indexed": 95,
    "skipped": 3,
    "errors": 2,
    "error_details": [
        {"path": "src/broken.go", "error": "parse error at line 45"},
        {"path": "src/huge.go", "error": "file too large (>10MB)"}
    ]
}
```

---

## Performance

### Throughput

| Files | Chunks | Expected Time |
|-------|--------|---------------|
| 100 | ~500 | 10-20s |
| 1,000 | ~5,000 | 1-2 min |
| 10,000 | ~50,000 | 10-20 min |

### Bottlenecks

| Stage | Time % | Optimization |
|-------|--------|--------------|
| Embedding | 60-70% | Batching, caching, GPU |
| Qdrant upsert | 20-30% | Batch upsert, async |
| Chunking | 5-10% | Parallel per file |
| Symbol extraction | 1-5% | Parallel per file |

---

## Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `CHUNK_SIZE` | 512 | Target tokens per chunk |
| `CHUNK_OVERLAP` | 64 | Token overlap between chunks |
| `EMBED_BATCH_SIZE` | 32 | Chunks per embedding batch |
| `EMBED_PARALLEL` | 4 | Concurrent embedding batches |
| `MAX_FILE_SIZE` | 10MB | Skip files larger than this |
| `INDEX_TIMEOUT` | 30min | Timeout for full reindex |
