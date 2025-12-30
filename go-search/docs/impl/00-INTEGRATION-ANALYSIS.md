# Implementation Integration Analysis

**Purpose:** Review all implementation plans to identify integration points, workflow enhancements, and additional ideas that enhance the overall system.

---

## Current Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                        rice-search binary                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │   Web UI     │  │   REST API   │  │   MCP Server (planned)   │  │
│  │ templ+HTMX   │  │   /v1/...    │  │   Unix Socket            │  │
│  └──────┬───────┘  └──────┬───────┘  └──────────┬───────────────┘  │
│         │                 │                      │                   │
│  ┌──────┴─────────────────┴──────────────────────┴───────────────┐  │
│  │                        Event Bus                               │  │
│  │   TopicSearchRequest | TopicEmbedRequest | TopicIndexResponse  │  │
│  └────────────────────────┬──────────────────────────────────────┘  │
│                           │                                          │
│     ┌─────────────────────┼─────────────────────┐                   │
│     │                     │                     │                    │
│  ┌──┴──────┐       ┌──────┴──────┐       ┌──────┴──────┐           │
│  │ Search  │       │    Index    │       │     ML      │           │
│  │ Service │       │   Pipeline  │       │   Service   │           │
│  │         │       │             │       │             │           │
│  │ • Query │       │ • Chunker   │       │ • Embed     │           │
│  │ • Fuse  │       │ • Symbols   │       │ • Sparse    │           │
│  │ • Rerank│       │ • Tracker   │       │ • Rerank    │           │
│  │ • Post  │       │             │       │             │           │
│  └────┬────┘       └──────┬──────┘       └──────┬──────┘           │
│       │                   │                     │                    │
│  ┌────┴───────────────────┴─────────────────────┴────────────────┐  │
│  │                        Qdrant Client                           │  │
│  │   Dense Search | Sparse Search | Hybrid RRF | Upsert/Delete   │  │
│  └────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 1. Tree-sitter AST Parsing (01-TREE-SITTER-AST.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| `index.Chunker` | Replace regex chunking with AST-based chunking |
| `index.symbols.go` | Replace `ExtractSymbols()` with AST symbol extraction |
| `index.Pipeline.processDocuments()` | Call new `ast.Parser` for both chunking and symbols |

### Workflow Enhancement: **Unified AST Package**

Create `internal/ast/` as a standalone package that both indexing AND search can use:

```go
// internal/ast/parser.go
type Parser interface {
    Parse(content []byte, lang string) (*Node, error)
    Chunk(content []byte, lang string, maxLines int) ([]Chunk, error)
    ExtractSymbols(content []byte, lang string) ([]string, error)
    GetFunctionAt(content []byte, lang string, line int) (*Function, error) // NEW!
}
```

### 💡 New Feature Idea: **Symbol-Aware Search Highlighting**

When returning search results, use AST to:
1. Identify which function/class the match is in
2. Return the full function bounds (not just chunk bounds)
3. Enable "jump to definition" in Web UI

**Add to Response:**
```go
type Result struct {
    // ... existing fields ...
    ContainingSymbol string `json:"containing_symbol,omitempty"` // "func AuthHandler"
    SymbolType       string `json:"symbol_type,omitempty"`       // "function", "class"
    SymbolStartLine  int    `json:"symbol_start_line,omitempty"`
    SymbolEndLine    int    `json:"symbol_end_line,omitempty"`
}
```

---

## 2. CLI Watch Mode (02-WATCH-MODE.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| `cmd/rice-search/` | Add `watch`, `watch list`, `watch stop` commands |
| `internal/client/` | Use existing HTTP client for API calls |
| Event Bus | Optionally publish `file.changed` events for real-time UI updates |

### Workflow Enhancement: **Watch Events for Web UI**

When watch mode detects changes, publish to event bus so Web UI can show real-time indexing status:

```go
// Publish when file change detected
bus.Publish(ctx, "watch.file_changed", Event{
    Payload: map[string]interface{}{
        "path":       path,
        "event_type": "modified", // created, modified, deleted
        "store":      store,
        "watcher_id": pid,
    },
})

// Web UI subscribes via SSE/WebSocket
// Shows toast: "Indexing 3 files..." → "Indexed src/auth.go"
```

### 💡 New Feature Idea: **Incremental Index Status in Web UI**

Add a "Watchers" section to the admin dashboard:
- List active watchers with status
- Show files being indexed in real-time
- Allow stopping watchers from Web UI

---

## 3. MCP Unix Socket (03-MCP-UNIX-SOCKET.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| Server startup | Start MCP socket listener alongside HTTP server |
| `search.Service` | Reuse for `code_search` tool |
| `index.Pipeline` | Reuse for `index_files` tool |
| `store.Service` | Reuse for `list_stores`, `get_store_stats` |

### Workflow Enhancement: **Unified Tool Interface**

Create `internal/tools/` package that both MCP and potential future CLI tools use:

```go
// internal/tools/interface.go
type ToolExecutor interface {
    ListTools() []Tool
    Execute(ctx context.Context, name string, args json.RawMessage) (string, error)
}

// Used by:
// - MCP handler
// - Future: CLI interactive mode
// - Future: Web UI "Run Tool" feature
```

### 💡 New Feature Idea: **MCP Resources for Files**

Expose indexed files as MCP resources:
```json
{"uri": "rice://stores/default/files/src/auth.go", "name": "src/auth.go"}
```

This allows AI assistants to:
1. List indexed files: `resources/list`
2. Read file content: `resources/read` (returns full file, not just chunks)
3. Subscribe to changes: `resources/subscribe` (notify on reindex)

---

## 4. Query Expansion (04-QUERY-EXPANSION.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| `query.KeywordExtractor` | Extend `expandWithSynonyms()` function |
| `query.code_terms.go` | Add `CodeAbbreviations` map |
| `query.types.go` | Add `QueryType` field to `ParsedQuery` |
| `query.Service.Parse()` | Both model and fallback paths use expansion |

### Workflow Enhancement: **Expansion Weights for Sparse Search**

Pass weights to sparse encoder so original terms get boosted:

```go
// query/types.go
type ExpandedTerm struct {
    Term   string  `json:"term"`
    Weight float32 `json:"weight"` // 1.0, 0.8, 0.7, 0.5
    Source string  `json:"source"` // original, case_split, synonym, abbreviation
}

// When building sparse query, boost original terms
for _, term := range parsedQuery.ExpandedTerms {
    sparseWeight := baseWeight * term.Weight
    // Apply to SPLADE encoding...
}
```

### 💡 New Feature Idea: **User-Defined Synonyms**

Allow users to add custom synonyms via settings:
```json
{
  "custom_synonyms": {
    "svc": ["service", "microservice"],
    "k8s": ["kubernetes"],
    "infra": ["infrastructure"]
  }
}
```

Store in `settings.Service`, load in `query.KeywordExtractor`.

---

## 5. Query Log Export (05-QUERY-LOG-EXPORT.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| `internal/server/` handlers | Add `/v1/observability/export` endpoint |
| Existing query logging | Add `GetQueriesInRange()` method |

### Workflow Enhancement: **Query Analytics Dashboard**

Add query analytics to Web UI stats page:
- Top queries (by frequency)
- Average latency by intent type
- Zero-result queries (opportunity for expansion tuning)
- Query type distribution (code vs natural vs mixed)

### 💡 New Feature Idea: **Query Replay for Testing**

Export format includes all query options. Enable replay:
```bash
rice-search replay queries.jsonl --store default --compare
```

Compares new results against original results to detect:
- Ranking changes
- Missing results
- New results

Useful for testing query expansion or reranking changes.

---

## 6. IR Evaluation (06-IR-EVALUATION.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| `internal/evaluation/` | New package for metrics |
| `search.Service` | Reuse for evaluation queries |
| Admin handlers | Add `/v1/evaluation/` endpoints |

### Workflow Enhancement: **Automated Evaluation on Config Change**

When search config changes (weights, reranking threshold), auto-run evaluation:

```go
// settings/service.go
func (s *Service) UpdateSearchConfig(cfg SearchConfig) error {
    // ... update config ...
    
    // Trigger evaluation if judgments exist
    if s.evaluator.HasJudgments() {
        go func() {
            result := s.evaluator.RunBenchmark(ctx, DefaultKs)
            s.bus.Publish(ctx, "evaluation.completed", result)
        }()
    }
}
```

### 💡 New Feature Idea: **A/B Comparison Mode**

Compare two configurations side-by-side:
```bash
rice-search eval compare \
  --config-a "sparse=0.5,dense=0.5" \
  --config-b "sparse=0.3,dense=0.7" \
  --judgments relevance.jsonl
```

Output:
```
Config A: NDCG@10=0.72, MRR=0.65
Config B: NDCG@10=0.78, MRR=0.71 (+9.2%)
Recommendation: Config B performs better for exploratory queries
```

---

## 7. Answer Mode (07-ANSWER-MODE.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| CLI search command | Add `-a, --answer` flag |
| Output formatters | New `format_answer.go` |

### Workflow Enhancement: **Answer Mode for Web UI**

Add "Copy for AI" button in Web UI that formats results in answer mode:
- One-click copy with citations
- Customizable format (Markdown, plain text)
- Include query context

### 💡 New Feature Idea: **RAG Context Builder**

For AI integrations, provide structured context:

```go
type RAGContext struct {
    Query       string          `json:"query"`
    Intent      string          `json:"intent"`
    Sources     []RAGSource     `json:"sources"`
    TotalTokens int             `json:"total_tokens"`
    Truncated   bool            `json:"truncated"`
}

type RAGSource struct {
    Index   int    `json:"index"`
    Path    string `json:"path"`
    Lines   string `json:"lines"` // "10-25"
    Content string `json:"content"`
    Symbols []string `json:"symbols"`
}
```

API endpoint: `POST /v1/stores/{store}/rag-context`
- Automatically truncates to fit context window
- Prioritizes most relevant content
- Returns token count estimate

---

## 8. Verbose CLI Stats (08-VERBOSE-CLI-STATS.md)

### Integration Points

| Component | Integration |
|-----------|-------------|
| CLI search command | Add `-v, --verbose` flag |
| `search.Response` | Ensure all timing/stats fields populated |
| Output formatters | New `format_verbose.go` |

### Workflow Enhancement: **Stats for All Operations**

Extend verbose mode to other commands:
```bash
rice-search index ./src -v
# Shows: chunking time, embedding time, upsert time, per-file stats

rice-search stores stats default -v
# Shows: index size, shard distribution, memory usage
```

---

## Cross-Cutting Enhancements

### 1. **Unified Progress Reporting**

All long-running operations (index, watch, evaluation) should report progress via event bus:

```go
type ProgressEvent struct {
    Operation   string  // "index", "watch", "evaluate"
    Store       string
    Current     int
    Total       int
    Percent     float32
    CurrentItem string  // "src/auth.go"
    EstimateMs  int64
}

// Subscribe in Web UI for live progress bars
// Subscribe in CLI for spinner/progress display
```

### 2. **Operation History**

Track operation history for debugging/auditing:

```go
type OperationLog struct {
    ID         string
    Type       string    // "search", "index", "delete"
    Store      string
    Timestamp  time.Time
    DurationMs int64
    Success    bool
    Details    map[string]interface{}
}

// API: GET /v1/operations?store=default&type=index&limit=100
// Web UI: Operations History page
```

### 3. **Health-Aware Search Routing**

Use health checks to adjust search behavior:

```go
// If GPU embedding is unhealthy, fall back to CPU
// If reranker is slow (>500ms), skip pass 2
// If Qdrant latency high, reduce prefetch multiplier

type AdaptiveConfig struct {
    GPUHealthy     bool
    RerankHealthy  bool
    QdrantLatency  time.Duration
}

func (s *SearchService) adaptConfig(ctx context.Context) Config {
    health := s.getHealthStatus()
    cfg := s.cfg
    
    if !health.GPUHealthy {
        // Already handled by ML service fallback
    }
    if health.QdrantLatency > 100*time.Millisecond {
        cfg.PrefetchMultiplier = 2 // Reduce from 3
    }
    return cfg
}
```

---

## New Feature Ideas (Not in Current Plans)

### 1. **Semantic Diff**

Compare two versions of a file semantically:
```bash
rice-search diff src/auth.go@v1 src/auth.go@v2
```
- Shows which functions changed
- Highlights semantic similarity score between versions
- Useful for code review

### 2. **Similar Files**

Find files similar to a given file:
```bash
rice-search similar src/auth/handler.go -k 5
```
Uses the file's embedding centroid to find similar files.

### 3. **Cluster Visualization**

Web UI feature to visualize document clusters:
- Use t-SNE/UMAP on embeddings
- Color by language, path prefix, or connection
- Interactive: click cluster to see files

### 4. **Query Suggestions**

Based on query log, suggest related queries:
```
You searched for: "authentication"
Related queries:
  - "JWT token validation"
  - "login handler"
  - "session management"
```

### 5. **Automatic Re-indexing**

When embedding model changes:
1. Detect model change (hash of ONNX file)
2. Warn user: "Embedding model changed. Re-index recommended."
3. Option to auto-reindex or schedule overnight

---

## Priority Ranking for Enhancements

| Enhancement | Impact | Effort | Priority |
|-------------|--------|--------|----------|
| Symbol-aware search results | High | Medium | 🔴 P1 |
| Watch events for Web UI | Medium | Low | 🔴 P1 |
| Expansion weights for sparse | High | Low | 🔴 P1 |
| MCP resources for files | Medium | Medium | 🟡 P2 |
| Query analytics dashboard | High | Medium | 🟡 P2 |
| RAG context builder API | High | Medium | 🟡 P2 |
| Unified progress reporting | Medium | Low | 🟡 P2 |
| A/B comparison mode | Medium | Medium | 🟢 P3 |
| Similar files command | Medium | Low | 🟢 P3 |
| Cluster visualization | Low | High | 🟢 P3 |

---

## Implementation Order Recommendation

Based on dependencies and value:

1. **04-QUERY-EXPANSION.md** - Low effort, immediate search quality improvement
2. **01-TREE-SITTER-AST.md** - Foundation for better chunking + enables symbol-aware results
3. **03-MCP-UNIX-SOCKET.md** - Enables AI integration (high demand)
4. **02-WATCH-MODE.md** - Better developer experience
5. **08-VERBOSE-CLI-STATS.md** - Quick win, improves debugging
6. **07-ANSWER-MODE.md** - Quick win, enables RAG workflows
7. **05-QUERY-LOG-EXPORT.md** - Enables analytics
8. **06-IR-EVALUATION.md** - Enables quality measurement

Then tackle the enhancements in priority order.
