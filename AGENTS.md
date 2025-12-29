# Rice Search - Agent Guidelines

## Build & Test Commands

### Local Development (Recommended)

```bash
# 1. Start Docker backend services only
docker-compose up -d milvus embeddings etcd minio

# 2. API (NestJS + Bun) - uses cargo run for Tantivy auto-recompilation
cd api
bun install
bun run start:local                               # Dev server on :8088

# 3. Web UI (Next.js + Bun)
cd web-ui
bun install
bun run dev:local                                 # Dev server on :3001

# Quality checks
cd api && bun run lint && bun run typecheck
cd ricegrep && bun run format && bun run typecheck
```

### Docker (Full Platform)

```bash
docker-compose up -d                               # Start all services
bash scripts/smoke_test.sh                         # End-to-end test
```

### ricegrep CLI

```bash
cd ricegrep
bun install && bun run build                       # Build CLI
bun run format && bun run typecheck                # Quality checks
bun test                                           # Run all tests
bun test --filter "Search"                         # Run specific test pattern
```

## Code Style & Standards

**Formatting**: Biome (ricegrep) + ESLint/Prettier (API) + Next.js ESLint (web-ui)  
**Types**: Strict TypeScript. Never use `any`, `@ts-ignore`, or suppress errors  
**Imports**: Node built-ins with `node:` prefix (`node:fs`, `node:path`)  
**Strings**: Double quotes (API/CLI), single quotes in JSX (web-ui), 2-space indentation  
**Architecture**: Uses bun throughout. Keep services decoupled (api/, ricegrep/, web-ui/)  
**Config**: YAML config files (`.ricegreprc.yaml`), env vars with `RICEGREP_` prefix  
**Error Handling**: Custom error classes, proper validation with zod/class-validator  
**File Organization**: `lib/` for utilities, `commands/` for CLI, `src/` for main code  
**SVG Files**: Never read SVG files directly (causes critical errors in opencode). Use PNG alternatives or file paths only

## Search Tool Priority (CRITICAL)

**ALWAYS use `ricegrep` tool for searching local files.** It is a semantic grep-like search tool that is substantially better than built-in search tools.

### Why ricegrep?
- **Semantic search** - Understands natural language queries, not just keywords
- **Context-aware** - Returns file paths with line ranges for precise results
- **Better relevance** - Uses semantic indexing for more accurate matches

### Usage Examples

```bash
# ✅ CORRECT - Natural language queries
ricegrep "What code parsers are available?"
ricegrep "How are chunks defined?" src/models
ricegrep -m 10 "What is the maximum number of concurrent workers?"

# ❌ WRONG - Too imprecise or unnecessary filters
ricegrep "parser"                                    # Too vague
ricegrep "How are chunks defined?" --type python    # Unnecessary filters
```

### When to Use What

| Scenario | Use This | NOT This |
|----------|----------|----------|
| Find code by intent/meaning | `ricegrep "how does auth work?"` | `grep "auth"` |
| Find implementations | `ricegrep "error handling patterns"` | `find . -name "*.ts"` |
| Understand code structure | `ricegrep "where are API endpoints defined?"` | manual exploration |
| Known exact string | `grep` or `ast_grep` | ricegrep (overkill) |
| Known file pattern | `glob "**/*.ts"` | ricegrep |

### Parameters

| Param | Default | Description |
|-------|---------|-------------|
| `q` | (required) | Natural language search query |
| `m` | 10 | Maximum number of results |
| `a` | false | Search all files (including ignored) |

**Rule**: Default to `ricegrep` for any exploratory search. Only fall back to `grep`/`glob` for exact string matches or known file patterns.

## Architecture Principles

**Quality Over Complexity**: Prioritize search quality even if it means more complex code. Better results justify additional complexity.

**Server-Side Intelligence**: All search/ranking decisions happen on the server (API). ricegrep CLI is a pure client - it forwards user preferences (like `--no-rerank`) to the API but never makes search decisions itself.

**Single Search Pipeline**: Infinity (embeddings + reranking) + Tantivy (BM25) + Milvus (vectors). No mode switching, no alternative backends.  

Run `bun run typecheck` before any significant changes. Never commit without clean diagnostics.

## Service Ports (Default)

| Service | Local Dev | Docker | Description |
|---------|-----------|--------|-------------|
| API | 8088 | 8080 | Rice Search REST API |
| Web UI | 3001 | 3000 | Next.js frontend |
| Attu | 8000 | 8000 | Milvus admin UI |
| Embeddings | 8081 | 8081 | Text embeddings inference |
| Milvus | 19530 | 19530 | Vector database |
| Milvus Metrics | 9091 | 9091 | Milvus health/metrics |
| MinIO | 9000 | 9000 | Object storage |
| MinIO Console | 9001 | 9001 | MinIO admin UI |

## API Endpoints

Base URL: `http://localhost:8080` (Docker) or `http://localhost:8088` (Local Dev)  
Swagger Docs: `http://localhost:8080/docs` or `http://localhost:8088/docs`

### Health

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Simple health check |
| GET | `/v1/version` | API version info |
| GET | `/v1/health` | Detailed health with dependencies |

### Stores

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/stores` | List all stores |
| POST | `/v1/stores` | Create new store |
| GET | `/v1/stores/{store}` | Get store details |
| DELETE | `/v1/stores/{store}` | Delete store |
| GET | `/v1/stores/{store}/stats` | Get store statistics |

### Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/stores/{store}/search` | Intelligent hybrid search (BM25 + semantic + reranking) |

**Request Body:**
```json
{
  "query": "search text",           // Required
  "top_k": 20,                      // Number of results (default: 20)
  "filters": {
    "path_prefix": "src/",          // Optional path filter
    "languages": ["typescript"]     // Optional language filter
  },
  "include_content": true,          // Include content (default: true)
  
  // Retrieval options
  "sparse_weight": 0.5,             // BM25 weight 0-1 (default: 0.5)
  "dense_weight": 0.5,              // Semantic weight 0-1 (default: 0.5)
  "enable_reranking": true,         // Neural reranking (default: true)
  "rerank_candidates": 30,          // Candidates for reranking (default: 30)
  
  // Post-processing options
  "enable_dedup": true,             // Semantic deduplication (default: true)
  "dedup_threshold": 0.85,          // Similarity threshold 0-1 (default: 0.85)
  "enable_diversity": true,         // MMR diversity (default: true)
  "diversity_lambda": 0.7,          // 0=diverse, 1=relevant (default: 0.7)
  "group_by_file": false,           // Group by file (default: false)
  "max_chunks_per_file": 3,         // Max chunks per file when grouping (default: 3)
  
  // Query processing
  "enable_expansion": true          // Query expansion (default: true)
}
```

**Response:**
```json
{
  "query": "search text",
  "results": [
    {
      "doc_id": "abc123",
      "path": "src/auth.ts",
      "language": "typescript",
      "start_line": 10,
      "end_line": 25,
      "content": "...",
      "symbols": ["authenticate", "validateToken"],
      "final_score": 0.85,
      "sparse_score": 12.5,
      "dense_score": 0.82,
      "sparse_rank": 1,
      "dense_rank": 3,
      "aggregation": {              // When group_by_file=true
        "is_representative": true,
        "related_chunks": 2,
        "file_score": 0.9,
        "chunk_rank_in_file": 1
      }
    }
  ],
  "total": 20,
  "store": "default",
  "search_time_ms": 45,
  "intelligence": {
    "intent": "navigational",       // navigational|factual|exploratory|analytical
    "difficulty": "easy",           // easy|medium|hard
    "strategy": "balanced",         // sparse-only|balanced|dense-heavy|deep-rerank
    "confidence": 0.85
  },
  "reranking": {
    "enabled": true,
    "candidates": 30,
    "pass1_applied": true,
    "pass1_latency_ms": 15,
    "pass2_applied": false,
    "pass2_latency_ms": 0,
    "early_exit": true,
    "early_exit_reason": "high_confidence"
  },
  "postrank": {
    "dedup": { "input_count": 30, "output_count": 25, "removed": 5, "latency_ms": 3 },
    "diversity": { "enabled": true, "avg_diversity": 0.72, "latency_ms": 2 },
    "aggregation": { "unique_files": 15, "chunks_dropped": 5 },
    "total_latency_ms": 8
  }
}
```

### Index

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/stores/{store}/index` | Index files |
| DELETE | `/v1/stores/{store}/index` | Delete files from index |
| POST | `/v1/stores/{store}/index/reindex` | Clear and rebuild index |
| POST | `/v1/stores/{store}/index/sync` | Sync index (remove deleted files) |
| GET | `/v1/stores/{store}/index/stats` | Get indexing statistics |
| GET | `/v1/stores/{store}/index/files` | List indexed files (paginated) |

**Index Request Body:**
```json
{
  "files": [
    { "path": "src/main.ts", "content": "..." }
  ],
  "force": false  // Force re-index unchanged files
}
```

**Delete Request Body:**
```json
{
  "paths": ["src/old.ts"],     // Specific files
  "path_prefix": "deprecated/" // Or by prefix
}
```

**Sync Request Body:**
```json
{
  "current_paths": ["src/main.ts", "src/utils.ts"]  // Files that exist
}
```

**List Files Query Parameters:**
| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `page` | int | 1 | Page number (1-indexed) |
| `page_size` | int | 50 | Results per page |
| `path` | string | - | Filter by path substring |
| `language` | string | - | Filter by language (typescript, python, rust, etc.) |
| `sort_by` | string | path | Sort field: path, size, indexed_at |
| `sort_order` | string | asc | Sort order: asc, desc |

**List Files Response:**
```json
{
  "files": [
    {
      "path": "src/auth.ts",
      "size": 2048,
      "hash": "a1b2c3d4e5f6g7h8",
      "indexed_at": "2025-12-28T02:00:00Z",
      "chunk_count": 5,
      "language": "typescript"
    }
  ],
  "total": 150,
  "page": 1,
  "page_size": 50,
  "total_pages": 3
}
```

### Observability

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/metrics` | Prometheus metrics endpoint |
| GET | `/v1/observability/stats` | Aggregated telemetry stats |
| GET | `/v1/observability/query-stats` | Query log statistics (params: store, days) |
| GET | `/v1/observability/recent-queries` | Recent queries (params: store, limit) |
| GET | `/v1/observability/telemetry` | Recent telemetry records (params: store, limit) |

### MCP (Model Context Protocol)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/mcp/tools` | List available MCP tools |
| POST | `/mcp` | Handle MCP JSON-RPC request |
| POST | `/mcp/tools/call` | Call MCP tool directly |