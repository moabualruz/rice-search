# Data Models

## Overview

Core data structures used throughout the system.

---

## Store

A store is an isolated search index (like a database).

```go
type Store struct {
    Name        string       `json:"name"`         // Unique identifier
    DisplayName string       `json:"display_name"` // Human-readable name
    Description string       `json:"description"`  // Optional description
    Config      StoreConfig  `json:"config"`       // Store configuration
    Stats       StoreStats   `json:"stats"`        // Current statistics
    CreatedAt   time.Time    `json:"created_at"`
    UpdatedAt   time.Time    `json:"updated_at"`
}

type StoreConfig struct {
    EmbedModel   string `json:"embed_model"`   // Dense embedding model
    SparseModel  string `json:"sparse_model"`  // Sparse encoding model
    ChunkSize    int    `json:"chunk_size"`    // Target chunk size (tokens)
    ChunkOverlap int    `json:"chunk_overlap"` // Overlap between chunks
}

type StoreStats struct {
    DocumentCount int64     `json:"document_count"` // Number of source files
    ChunkCount    int64     `json:"chunk_count"`    // Number of indexed chunks
    TotalSize     int64     `json:"total_size"`     // Total content size (bytes)
    LastIndexed   time.Time `json:"last_indexed"`   // Last index operation
}
```

### Store Name Rules

| Rule | Example |
|------|---------|
| Lowercase alphanumeric | `myproject` |
| Hyphens allowed | `my-project` |
| Max 64 characters | - |
| Must start with letter | `a-project` ✓, `1-project` ✗ |

### Default Store

- Name: `default`
- Created automatically on first use
- Cannot be deleted

---

## Document

A source file to be indexed.

```go
type Document struct {
    Path     string   `json:"path"`     // File path (unique within store)
    Content  string   `json:"content"`  // File content
    Language string   `json:"language"` // Programming language
    Symbols  []string `json:"symbols"`  // Extracted symbols (functions, classes)
    Hash     string   `json:"hash"`     // Content hash (SHA-256)
    Size     int64    `json:"size"`     // Content size (bytes)
}
```

### Language Detection

Languages detected from file extension:

| Extension | Language |
|-----------|----------|
| `.go` | go |
| `.ts`, `.tsx` | typescript |
| `.js`, `.jsx` | javascript |
| `.py` | python |
| `.rs` | rust |
| `.java` | java |
| `.c`, `.h` | c |
| `.cpp`, `.cc`, `.hpp` | cpp |
| `.rb` | ruby |
| `.php` | php |
| `.swift` | swift |
| `.kt` | kotlin |
| `.scala` | scala |
| `.cs` | csharp |
| `.md` | markdown |
| `.json` | json |
| `.yaml`, `.yml` | yaml |
| `.toml` | toml |
| `.sql` | sql |
| `.sh`, `.bash` | bash |
| Other | `unknown` |

---

## Chunk

A searchable unit extracted from a document.

```go
type Chunk struct {
    ID          string    `json:"id"`           // Unique chunk ID
    DocumentID  string    `json:"document_id"`  // Parent document ID (path hash)
    Store       string    `json:"store"`        // Store name
    Path        string    `json:"path"`         // Source file path
    Language    string    `json:"language"`     // Programming language
    Content     string    `json:"content"`      // Chunk content
    Symbols     []string  `json:"symbols"`      // Symbols in this chunk
    StartLine   int       `json:"start_line"`   // Starting line number (1-indexed)
    EndLine     int       `json:"end_line"`     // Ending line number (1-indexed)
    StartChar   int       `json:"start_char"`   // Starting character offset
    EndChar     int       `json:"end_char"`     // Ending character offset
    TokenCount  int       `json:"token_count"`  // Number of tokens
    Hash        string    `json:"hash"`         // Content hash
    IndexedAt   time.Time `json:"indexed_at"`   // When indexed
}
```

### Chunk ID Generation

```
chunk_id = sha256(store + ":" + path + ":" + start_line + ":" + end_line)[:16]
```

Ensures:
- Same chunk always gets same ID
- Re-indexing updates existing chunk
- No duplicates

---

## Vectors

### Dense Vector

Fixed-size semantic embedding.

```go
type DenseVector struct {
    Values     []float32 `json:"values"`     // 1536 floats
    Dimensions int       `json:"dimensions"` // Always 1536 for Jina
    Normalized bool      `json:"normalized"` // L2 normalized
}
```

### Sparse Vector

Variable-size keyword representation (SPLADE).

```go
type SparseVector struct {
    Indices []int32   `json:"indices"` // Token IDs (from BERT vocab)
    Values  []float32 `json:"values"`  // Token weights
}
```

### Sparse Vector Properties

| Property | Value |
|----------|-------|
| Vocab size | 30,522 (BERT) |
| Typical non-zero | 50-200 per text |
| Value range | 0.0 - ~5.0 |

---

## Search Structures

### SearchRequest

```go
type SearchRequest struct {
    Store   string        `json:"store"`
    Query   string        `json:"query"`
    TopK    int           `json:"top_k"`
    Filters SearchFilters `json:"filters"`
    Options SearchOptions `json:"options"`
}

type SearchFilters struct {
    PathPrefix string   `json:"path_prefix"` // e.g., "src/auth/"
    Languages  []string `json:"languages"`   // e.g., ["go", "typescript"]
}

type SearchOptions struct {
    SparseWeight    float32 `json:"sparse_weight"`     // 0.0 - 1.0
    DenseWeight     float32 `json:"dense_weight"`      // 0.0 - 1.0
    EnableReranking bool    `json:"enable_reranking"`
    RerankTopK      int     `json:"rerank_top_k"`
}
```

### SearchResult

```go
type SearchResult struct {
    DocID       string   `json:"doc_id"`
    Path        string   `json:"path"`
    Language    string   `json:"language"`
    Content     string   `json:"content"`
    Symbols     []string `json:"symbols"`
    StartLine   int      `json:"start_line"`
    EndLine     int      `json:"end_line"`
    Score       float32  `json:"score"`        // Final score
    SparseScore float32  `json:"sparse_score"` // BM25/SPLADE score
    DenseScore  float32  `json:"dense_score"`  // Semantic score
}
```

### SearchResponse

```go
type SearchResponse struct {
    Query      string          `json:"query"`
    Results    []SearchResult  `json:"results"`
    Total      int             `json:"total"`       // Total matches (before top_k)
    LatencyMS  int64           `json:"latency_ms"`
    Stages     SearchStages    `json:"stages"`
}

type SearchStages struct {
    SparseMS  int64 `json:"sparse_ms"`
    DenseMS   int64 `json:"dense_ms"`
    FusionMS  int64 `json:"fusion_ms"`
    RerankMS  int64 `json:"rerank_ms"`
}
```

---

## Index Structures

### IndexRequest

```go
type IndexRequest struct {
    Store     string          `json:"store"`
    Documents []IndexDocument `json:"documents"`
    Options   IndexOptions    `json:"options"`
}

type IndexDocument struct {
    Path    string `json:"path"`
    Content string `json:"content"`
}

type IndexOptions struct {
    Force        bool `json:"force"`         // Re-index even if unchanged
    ChunkSize    int  `json:"chunk_size"`    // Override default
    ChunkOverlap int  `json:"chunk_overlap"` // Override default
}
```

### IndexResponse

```go
type IndexResponse struct {
    Store         string `json:"store"`
    Indexed       int    `json:"indexed"`        // Files indexed
    Skipped       int    `json:"skipped"`        // Files skipped (unchanged)
    Errors        int    `json:"errors"`         // Files with errors
    ChunksCreated int    `json:"chunks_created"` // Total chunks created
    LatencyMS     int64  `json:"latency_ms"`
}
```

---

## Cache Structures

### EmbeddingCache

```go
type CachedEmbedding struct {
    Hash       string      `json:"hash"`       // sha256(text)
    Vector     DenseVector `json:"vector"`
    Model      string      `json:"model"`
    CachedAt   time.Time   `json:"cached_at"`
}
```

### SparseCache

```go
type CachedSparse struct {
    Hash     string       `json:"hash"`
    Vector   SparseVector `json:"vector"`
    Model    string       `json:"model"`
    CachedAt time.Time    `json:"cached_at"`
}
```

---

## Validation Rules

### Content Limits

| Field | Limit |
|-------|-------|
| Document content | 10MB max |
| Chunk content | 8192 tokens max |
| Query | 4096 tokens max |
| Path | 1024 characters max |
| Store name | 64 characters max |
| Symbols per chunk | 100 max |

### Required Fields

| Structure | Required Fields |
|-----------|-----------------|
| Document | path, content |
| Chunk | id, store, path, content, start_line, end_line |
| SearchRequest | store, query |
| IndexRequest | store, documents |
