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
    Path         string   `json:"path"`                    // File path (unique within store)
    Content      string   `json:"content"`                 // File content
    Language     string   `json:"language"`                // Programming language
    Symbols      []string `json:"symbols"`                 // Extracted symbols (functions, classes)
    Hash         string   `json:"hash"`                    // Content hash (SHA-256)
    Size         int64    `json:"size"`                    // Content size (bytes)
    ConnectionID string   `json:"connection_id,omitempty"` // Originating connection (optional)
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
    ID           string    `json:"id"`                      // Unique chunk ID
    DocumentID   string    `json:"document_id"`             // Parent document ID (path hash)
    Store        string    `json:"store"`                   // Store name
    Path         string    `json:"path"`                    // Source file path
    Language     string    `json:"language"`                // Programming language
    Content      string    `json:"content"`                 // Chunk content
    Symbols      []string  `json:"symbols"`                 // Symbols in this chunk
    StartLine    int       `json:"start_line"`              // Starting line number (1-indexed)
    EndLine      int       `json:"end_line"`                // Ending line number (1-indexed)
    StartChar    int       `json:"start_char"`              // Starting character offset
    EndChar      int       `json:"end_char"`                // Ending character offset
    TokenCount   int       `json:"token_count"`             // Number of tokens
    Hash         string    `json:"hash"`                    // Content hash
    IndexedAt    time.Time `json:"indexed_at"`              // When indexed
    ConnectionID string    `json:"connection_id,omitempty"` // Originating connection (optional)
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

Fixed-size semantic embedding (represented as `[]float32` in Go).

```go
// No DenseVector struct - code uses []float32 directly
// Example: embedding := []float32{0.1, 0.2, ..., 0.9} // 1536 dimensions
```

### Sparse Vector

Variable-size keyword representation (SPLADE) stored in Qdrant's native sparse format.

```go
// Sparse vectors are stored directly in Qdrant
// Go code uses Qdrant client types for sparse vectors
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

Flattened structure with filters and options as direct fields.

```go
type SearchRequest struct {
    Query           string   `json:"query"`
    TopK            int      `json:"top_k,omitempty"`
    
    // Filters (flattened)
    Filter          *Filter  `json:"filter,omitempty"`
    
    // Options (flattened)
    EnableReranking *bool    `json:"enable_reranking,omitempty"`
    RerankTopK      int      `json:"rerank_top_k,omitempty"`
    IncludeContent  bool     `json:"include_content,omitempty"`
    SparseWeight    *float32 `json:"sparse_weight,omitempty"`
    DenseWeight     *float32 `json:"dense_weight,omitempty"`
    GroupByFile     bool     `json:"group_by_file,omitempty"`
    MaxPerFile      int      `json:"max_per_file,omitempty"`
}

type Filter struct {
    PathPrefix   string   `json:"path_prefix,omitempty"`
    Languages    []string `json:"languages,omitempty"`
    ConnectionID string   `json:"connection_id,omitempty"` // Optional connection scope
}
```

### SearchResult

```go
type SearchResult struct {
    ID           string   `json:"id"`
    Path         string   `json:"path"`
    Language     string   `json:"language"`
    StartLine    int      `json:"start_line"`
    EndLine      int      `json:"end_line"`
    Content      string   `json:"content,omitempty"`
    Symbols      []string `json:"symbols,omitempty"`
    ConnectionID string   `json:"connection_id,omitempty"` // Connection that indexed this chunk
    
    // Scoring
    Score       float32  `json:"score"`                // Final fused score
    RerankScore *float32 `json:"rerank_score,omitempty"` // Reranking score (if applied)
    SparseRank  *int     `json:"sparse_rank,omitempty"`  // Rank in sparse results
    DenseRank   *int     `json:"dense_rank,omitempty"`   // Rank in dense results
    FusedScore  float32  `json:"fused_score,omitempty"`  // RRF fusion score
}
```

### SearchResponse

```go
type SearchResponse struct {
    Query    string         `json:"query"`
    Store    string         `json:"store"`
    Results  []SearchResult `json:"results"`
    Total    int            `json:"total"`
    Metadata SearchMetadata `json:"metadata"`
}

type SearchMetadata struct {
    SearchTimeMs       int64 `json:"search_time_ms"`
    EmbedTimeMs        int64 `json:"embed_time_ms"`
    RetrievalTimeMs    int64 `json:"retrieval_time_ms"`
    RerankTimeMs       int64 `json:"rerank_time_ms,omitempty"`
    CandidatesReranked int   `json:"candidates_reranked,omitempty"`
    RerankingApplied   bool  `json:"reranking_applied"`
}
```

---

## Index Structures

### IndexRequest

```go
type IndexRequest struct {
    Store     string      `json:"store"`
    Documents []*Document `json:"documents"`
    Force     bool        `json:"force"` // Re-index even if unchanged
}

// Document is defined in index package
type Document struct {
    Path         string   `json:"path"`
    Content      string   `json:"content"`
    Language     string   `json:"language,omitempty"`
    Hash         string   `json:"hash,omitempty"`          // Optional content hash
    ConnectionID string   `json:"connection_id,omitempty"` // Originating connection (set via header or document)
}
```

**Note:** The `ConnectionID` is typically set automatically:
- Via HTTP request header: `X-Connection-ID` 
- Propagated to all documents and chunks created from that request
- Used for connection-scoped search filtering

### IndexResult

```go
type IndexResult struct {
    Store        string        `json:"store"`
    Indexed      int           `json:"indexed"`      // Files successfully indexed
    Skipped      int           `json:"skipped"`      // Files skipped (unchanged)
    Failed       int           `json:"failed"`       // Files that failed
    ChunksTotal  int           `json:"chunks_total"` // Total chunks created
    Duration     time.Duration `json:"duration"`
    Errors       []IndexError  `json:"errors,omitempty"`
    DocumentInfo []DocInfo     `json:"document_info,omitempty"`
}

type IndexError struct {
    Path    string `json:"path"`
    Message string `json:"message"`
}

type DocInfo struct {
    Path       string `json:"path"`
    Hash       string `json:"hash"`
    ChunkCount int    `json:"chunk_count"`
    Status     string `json:"status"` // indexed, skipped, failed
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

---

## Connection Tracking

### Overview

Rice Search tracks the **originating connection** for all indexed content via the `ConnectionID` field. This enables connection-scoped search and activity monitoring.

### ConnectionID Field

The `ConnectionID` field appears in:

| Structure | Field | Purpose |
|-----------|-------|---------|
| **Document** | `ConnectionID string` | Tracks which connection indexed this file |
| **Chunk** | `ConnectionID string` | Inherited from parent document |
| **SearchRequest.Filter** | `ConnectionID string` | Filter results by connection |
| **SearchResult** | `ConnectionID string` | Shows which connection indexed this result |

### ConnectionID Flow

**During Indexing:**
```
1. Client sends HTTP request with X-Connection-ID header
2. Server extracts header and sets doc.ConnectionID for each document
3. Chunker copies ConnectionID to all chunks created from that document
4. Qdrant stores ConnectionID in point payload
```

**During Search:**
```
1. Client can optionally filter by ConnectionID via Filter.ConnectionID
2. Default behavior: scope search to requesting connection's data only
3. Special values:
   - "*" or "all" = search across all connections
   - Specific ID = search only that connection's data
   - Empty + no header = search all connections
```

### Automatic Connection Scoping

By default, searches are automatically scoped to the requesting connection:

```go
// If no explicit filter.ConnectionID provided
if req.Filter.ConnectionID == "" {
    // Use connection from request header
    req.Filter.ConnectionID = r.Header.Get("X-Connection-ID")
}

// To search all connections, explicitly pass "*" or "all"
if req.Filter.ConnectionID == "*" || req.Filter.ConnectionID == "all" {
    req.Filter.ConnectionID = "" // Empty = no filter
}
```

### ConnectionID Generation

Deterministic ID based on client machine:

```go
func GenerateConnectionID(pcInfo PCInfo) string {
    // Combines: hostname + OS + arch + MAC address
    // Example: "conn_abc123def456"
    return fmt.Sprintf("conn_%s", sha256(pcInfo)[:12])
}
```

### Use Cases

| Use Case | Implementation |
|----------|----------------|
| **Multi-tenant isolation** | Each client only sees their own indexed files |
| **Team workspaces** | Share ConnectionID across team members |
| **Cross-connection search** | Use `filter.connection_id: "*"` to search all |
| **Connection monitoring** | Track per-connection activity and resource usage |
| **Audit trails** | Know which connection indexed or searched what |

### API Examples

**Index with ConnectionID:**
```bash
curl -X POST http://localhost:8080/v1/stores/default/index \
  -H "X-Connection-ID: conn_abc123" \
  -H "Content-Type: application/json" \
  -d '{"files": [{"path": "main.go", "content": "..."}]}'
```

**Search scoped to connection:**
```bash
curl -X POST http://localhost:8080/v1/stores/default/search \
  -H "X-Connection-ID: conn_abc123" \
  -H "Content-Type: application/json" \
  -d '{"query": "authentication"}'
# Returns only results indexed by conn_abc123
```

**Search across all connections:**
```bash
curl -X POST http://localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "authentication",
    "filter": {"connection_id": "*"}
  }'
# Returns results from all connections
```

**Search specific connection:**
```bash
curl -X POST http://localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "authentication",
    "filter": {"connection_id": "conn_xyz789"}
  }'
# Returns only results indexed by conn_xyz789
```
