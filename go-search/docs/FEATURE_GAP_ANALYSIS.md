# Feature Gap Analysis: Old Implementations vs go-search

**Generated:** December 30, 2025  
**Purpose:** Document features in api/, ricegrep/, web-ui/ that are missing or different in go-search/

---

## Executive Summary

| Source | Total Features | In go-search | Missing | Gap % |
|--------|---------------|--------------|---------|-------|
| **api/** (NestJS) | ~50 major | ~40 | ~10 | 20% |
| **ricegrep/** (CLI) | ~25 major | ~15 | ~10 | 40% |
| **web-ui/** (Next.js) | ~20 major | ~18 | ~2 | 10% |

**go-search has unique features not in old implementations:**
- Connection tracking & default scoping
- Per-model GPU toggles
- 80+ admin settings with versioning
- Single binary deployment
- Embedded web UI (no separate frontend)

---

## 🔴 HIGH PRIORITY GAPS (Should Implement)

### 1. MCP via Unix Domain Socket
**Status:** ❌ NOT IN GO-SEARCH  
**Impact:** High - Required for AI assistant integration

**Old Implementation:**
- `api/src/mcp/mcp.service.ts` (819 lines)
- `api/src/mcp/mcp.controller.ts` (151 lines)

**Features:**
- 5 Tools: `code_search`, `index_files`, `delete_files`, `list_stores`, `get_store_stats`
- 3 Resource Templates: files list, file content, stats
- 4 Prompts: `code_review`, `explain_code`, `find_similar`, `search_and_summarize`
- JSON-RPC 2.0 handler

**go-search Implementation Plan:**

Use **Unix Domain Socket** for near-zero latency stdio-like experience:

```
┌─────────────────┐     Unix Socket      ┌─────────────────┐
│   AI Assistant  │ ←──────────────────→ │  rice-search    │
│   (MCP Client)  │  ~/.local/run/rice/  │    server       │
│                 │     mcp.sock         │                 │
└─────────────────┘                       └─────────────────┘
```

**Why Unix Socket instead of gRPC/HTTP:**

| Aspect | gRPC/TCP | Unix Socket |
|--------|----------|-------------|
| Latency | ~0.1-1ms | ~0.01ms (10-100x faster) |
| Overhead | HTTP/2 + protobuf | Raw bytes (zero framing) |
| Feel | Network call | Like reading/writing a file |
| Setup | Port binding | File path |

**Server Implementation:**
```go
// Package: internal/mcp/

// MCP socket listener (separate from HTTP)
listener, _ := net.Listen("unix", "~/.local/run/rice-search/mcp.sock")
for {
    conn, _ := listener.Accept()
    go handleMCPConnection(conn)  // Each conn behaves like stdio
}

func handleMCPConnection(conn net.Conn) {
    reader := bufio.NewReader(conn)
    for {
        line, _ := reader.ReadBytes('\n')  // JSON-RPC line
        response := handleMCPRequest(line)
        conn.Write(response)
        conn.Write([]byte("\n"))
    }
}
```

**AI Assistant Configuration:**
```json
{
  "mcpServers": {
    "rice-search": {
      "command": "socat",
      "args": ["-", "UNIX-CONNECT:~/.local/run/rice-search/mcp.sock"]
    }
  }
}
```

Or with netcat: `nc -U ~/.local/run/rice-search/mcp.sock`

**Socket Locations:**
- Linux/macOS: `~/.local/run/rice-search/mcp.sock`
- Windows: `\\.\pipe\rice-search-mcp` (named pipe)

**Benefits:**
- Feels exactly like stdio (line-based JSON-RPC)
- Single server process, multiple concurrent clients
- No separate MCP process needed per AI assistant
- 10-100x faster than TCP/gRPC
- Auto-cleanup on server shutdown

---

### 2. CLI Watch Mode with Daemon Support
**Status:** ❌ NOT IN GO-SEARCH CLI  
**Impact:** High - Essential for keeping index in sync

**Old Implementation:**
- `ricegrep/src/commands/watch.ts`
- `ricegrep/src/commands/watch_mcp.ts`

**Features in ricegrep:**
- `ricegrep watch` - Continuous file monitoring
- Incremental sync (only uploads changed files)
- Respects `.gitignore` and `.ricegrepignore`
- xxhash64 hashing for efficient change detection
- Batch uploads (15 files per batch)

**go-search Implementation Plan:**

```bash
# Start watcher in foreground
rice-search watch ./src -s myproject

# Start watcher as daemon (background)
rice-search watch ./src -s myproject --daemon
# Returns: "Watcher started with PID 12345"

# List all active watchers
rice-search watch list
# Output:
# PID     STORE      PATH              STARTED           FILES
# 12345   myproject  /home/user/src    2025-12-30 10:00  1,234
# 12346   default    /home/user/other  2025-12-30 09:30  567

# Stop a specific watcher
rice-search watch stop 12345
# Output: "Stopped watcher 12345"

# Stop all watchers
rice-search watch stop --all
# Output: "Stopped 2 watchers"
```

**Implementation Details:**
- Package: `internal/watch/`
- PID file: `~/.local/state/rice-search/watchers/{pid}.json`
- Metadata: PID, store, path, started time, connection ID
- Uses `fsnotify` for file system events
- Graceful shutdown on SIGTERM/SIGINT
- Auto-cleanup of stale PID files on `watch list`

---

### 3. Query Expansion with Code Abbreviations
**Status:** ⚠️ PARTIAL (90 synonyms vs 134 abbreviations)  
**Impact:** Medium - Improves recall for code searches

**Old Implementation:**
- `api/src/sparse/query-expansion.service.ts` (521 lines)

**Features Missing:**
- 134 code abbreviations (auth→authentication, btn→button, cfg→config, etc.)
- CamelCase/snake_case/kebab-case splitting
- Expansion type detection (code/natural/mixed)
- BM25-optimized vs Dense-optimized expansion modes
- Weight-based term prioritization

**go-search has:**
- 90 code term synonyms in `internal/query/code_terms.go`
- 18 term families

**Recommendation:** Port abbreviation dictionary from api/

---

### 4. Query Logging & Export
**Status:** ⚠️ PARTIAL (event logging exists, no export)  
**Impact:** Medium - Useful for analysis and replay

**Old Implementation:**
- `api/src/observability/query-log.service.ts` (421 lines)

**Features Missing:**
- Daily JSONL rotation (YYYY-MM-DD.jsonl)
- `getRecentQueries()` - Last N queries
- `getUniqueQueries()` - Deduplicated queries  
- `exportQueries()` - Export as JSONL or CSV
- Per-store directories
- Buffered async writes

**go-search has:**
- Event logging to JSON file (optional)
- Recent queries in Web UI
- No export functionality

**Recommendation:** Add export to `/api/v1/observability/export`

---

### 5. IR Evaluation Framework
**Status:** ❌ NOT IN GO-SEARCH  
**Impact:** Medium - Important for quality assessment

**Old Implementation:**
- `api/src/observability/evaluation.service.ts` (447 lines)

**Features:**
- Metrics: NDCG@K, Recall@K, MRR, Precision@K, MAP
- `evaluateQuery()` - Single query evaluation
- `summarize()` - Aggregate metrics
- Relevance judgment loading
- Click-to-judgment conversion

**Recommendation:** Add `internal/evaluation/` package

---

## 🟡 MEDIUM PRIORITY GAPS

### 6. Answer Mode (RAG)
**Status:** ❌ NOT IN GO-SEARCH CLI  
**Impact:** Medium - Nice for AI-assisted workflows

**Old Implementation:**
- `ricegrep/src/commands/search.ts` (-a flag)
- `ricegrep/src/lib/local-store.ts` (formatAskResponse)

**Features:**
- `-a, --answer` flag for RAG-style responses
- Citation parsing (`<cite i="X"/>` tags)
- Source extraction and formatting

**Recommendation:** Add `--answer` flag to `rice-search search`

---

### 7. Verbose Intelligence Stats in CLI
**Status:** ⚠️ PARTIAL (Web UI has it, CLI doesn't)  
**Impact:** Low-Medium - CLI convenience

**Old Implementation:**
- `ricegrep/src/commands/search.ts` (-v flag)
- Shows intent, strategy, difficulty, confidence
- Reranking pass details
- PostRank statistics

**go-search has:**
- Full stats in Web UI search results
- CLI `search` command has no verbose mode

**Recommendation:** Add `-v, --verbose` to `rice-search search`

---

### 8. Tree-Sitter AST-Aware Chunking
**Status:** ⚠️ DIFFERENT APPROACH  
**Impact:** Medium - Affects chunk quality

**Old Implementation:**
- `api/src/services/treesitter-chunker.service.ts` (~900 lines)
- WASM-based Tree-sitter
- 25+ languages with AST parsing

**go-search has:**
- Regex-based chunking (`internal/index/chunker.go`)
- Brace/indentation/heading strategies
- No AST parsing

**Trade-off:** Tree-sitter is more accurate but heavier. go-search's approach is faster and simpler.

**Recommendation:** Consider optional Tree-sitter support via cgo in future

---

## 🟢 NOT NEEDED / ARCHITECTURAL DIFFERENCES

### Tantivy CLI (Rust)
**Status:** ❌ NOT NEEDED  

**Old Implementation:**
- `api/tantivy/src/main.rs` (513 lines)
- Rust binary for BM25

**go-search uses:**
- Qdrant native sparse vectors (SPLADE)
- No separate BM25 index

**Decision:** Keep current approach (simpler)

---

### Infinity Server Integration
**Status:** ❌ NOT NEEDED  

**Old Implementation:**
- `api/src/services/infinity.service.ts`
- HTTP client to external embedding server

**go-search uses:**
- Embedded ONNX Runtime
- No external ML server

**Decision:** Keep current approach (single binary)

---

### Milvus Integration
**Status:** ❌ NOT NEEDED  

**Old Implementation:**
- `api/src/services/milvus.service.ts`
- Milvus vector database

**go-search uses:**
- Qdrant vector database
- Native RRF fusion

**Decision:** Keep current approach

---

### Background Job Queues (BullMQ)
**Status:** ❌ NOT NEEDED  

**Old Implementation:**
- `api/src/services/embedding-queue.service.ts`
- `api/src/services/tantivy-queue.service.ts`
- BullMQ-based with Redis

**go-search uses:**
- Synchronous indexing
- Event bus for async ML operations

**Decision:** Keep current approach for single-node. Add Redis queue only for distributed mode if needed.

---

## ✅ FEATURES GO-SEARCH HAS THAT OLD IMPLEMENTATIONS DON'T

| Feature | go-search | api/ | ricegrep/ |
|---------|-----------|------|-----------|
| **Connection Tracking** | ✅ Full | ❌ | ❌ |
| **Default Search Scoping** | ✅ Per-client | ❌ | ❌ |
| **Per-Model GPU Toggles** | ✅ 4 toggles | ❌ | ❌ |
| **80+ Admin Settings** | ✅ With UI | ❌ | ❌ |
| **Settings Versioning** | ✅ History/rollback | ❌ | ❌ |
| **Single Binary** | ✅ ~50MB | ❌ 6+ containers | ❌ Node.js |
| **Embedded Web UI** | ✅ templ+HTMX | ❌ Separate | ❌ N/A |
| **Zero External Services** | ✅ Qdrant only | ❌ Milvus+Infinity+Redis+Tantivy | ❌ Needs API |
| **gRPC API** | ✅ 18 methods | ❌ | ❌ |
| **Stats Dashboard** | ✅ 13 presets | ❌ | ❌ |

---

## Implementation Priority Matrix

| Feature | Effort | Impact | Priority |
|---------|--------|--------|----------|
| MCP via Unix Socket | Medium | High | 🔴 P1 |
| CLI Watch Mode + Daemon | Medium | High | 🔴 P1 |
| Query Expansion (134 abbrevs) | Low | Medium | 🟡 P2 |
| Query Log Export | Low | Medium | 🟡 P2 |
| IR Evaluation | Medium | Medium | 🟡 P2 |
| Answer Mode | Low | Low | 🟢 P3 |
| Verbose CLI Stats | Low | Low | 🟢 P3 |

---

## File Reference Index

### api/ Key Files (for reference)
| Feature | Files |
|---------|-------|
| MCP Server | `src/mcp/mcp.service.ts`, `src/mcp/mcp.controller.ts` |
| Query Expansion | `src/sparse/query-expansion.service.ts` |
| Query Logging | `src/observability/query-log.service.ts` |
| IR Evaluation | `src/observability/evaluation.service.ts` |
| Tree-Sitter | `src/services/treesitter-chunker.service.ts` |
| Intent Classification | `src/intelligence/intent-classifier.service.ts` |
| Strategy Selection | `src/intelligence/strategy-selector.service.ts` |
| Multi-Pass Rerank | `src/ranking/multi-pass-reranker.service.ts` |

### ricegrep/ Key Files (for reference)
| Feature | Files |
|---------|-------|
| Search Command | `src/commands/search.ts` |
| Watch Command | `src/commands/watch.ts` |
| Sync Utils | `src/lib/sync-helpers.ts` |
| File System | `src/lib/file.ts` |
| Local Store | `src/lib/local-store.ts` |

### web-ui/ Key Files (for reference)
| Feature | Files |
|---------|-------|
| Search Page | `src/app/page.tsx` |
| Admin Page | `src/app/admin/page.tsx` |
| Observability | `src/app/admin/observability/page.tsx` |
| API Client | `src/lib/api.ts` |
| Types | `src/types/index.ts` |

---

## Conclusion

**go-search is ~90% feature-complete** compared to the combined old implementations.

**Critical gaps to address (P1):**
1. MCP via Unix Domain Socket (for AI assistant integration, stdio-like latency)
2. CLI Watch Mode with daemon support (for keeping index in sync)

**Nice-to-have gaps (P2):**
3. Extended query expansion (134 abbreviations)
4. Query log export functionality
5. IR evaluation framework

**Unique advantages of go-search:**
- Single binary deployment
- Connection-aware multi-tenancy
- Comprehensive admin UI
- Lower resource requirements (~4GB vs 12GB+)
