# Rice Search - Agent Guidelines

## Environment Variables

**`.env.example`** is the source of truth for all environment variables. It is tracked in git and contains all variables with sensible defaults for local development.

**Rules for agents:**
1. **NEVER** create or modify `.env` files (gitignored, invisible to agents)
2. **ALWAYS** add new env vars to `.env.example` with documentation
3. When a new env var is needed, update `.env.example` and ask user to sync their `.env`

**User workflow:**
```bash
cp .env.example .env    # First time only
# Edit .env to customize values (API keys, paths, etc.)
```

## Build & Test Commands

### Docker Compose Profiles

The docker-compose.yml uses profiles to control GPU/CPU mode and which services start.

**First-time setup**: Copy `.env.example` to `.env` (ready for local dev, defaults to `COMPOSE_PROFILES=gpu,dev`)

| Command | Services Started | Use Case |
|---------|------------------|----------|
| `docker-compose up -d` | GPU infrastructure + Attu | **Local dev** (default from .env.example) |
| `docker-compose --profile gpu --profile full up -d` | GPU infrastructure + API + Web UI | Full platform in Docker |
| `docker-compose --profile cpu --profile dev up -d` | CPU infrastructure + Attu | Local dev (no GPU) |
| `docker-compose --profile cpu --profile full up -d` | CPU infrastructure + API + Web UI | Full platform (no GPU) |

**GPU Requirements**: NVIDIA GPU + nvidia-container-toolkit + Docker nvidia runtime

### Local Development (Default)

The `.env.example` is pre-configured for local development. Just copy and go:

```bash
# 1. Copy environment config (first time only - user action, not agent)
cp .env.example .env

# 2. Start infrastructure (uses gpu,dev from .env)
docker-compose up -d

# 3. Verify services are running
docker-compose ps    # Should show: etcd, redis, minio, milvus, infinity, attu
                     # Should NOT show: api, web-ui

# 3. Wait for services to be healthy (~2-3 min on first run for model downloads)
docker-compose ps                                  # Check status

# 4. API (NestJS + Bun) - runs on :8080
cd api
bun install
bun run start:local                               # Hot reload enabled

# 5. Web UI (Next.js + Bun) - runs on :3000
cd web-ui
bun install
bun run dev:local                                 # Hot reload enabled

# Quality checks (run before commits)
cd api && bun run lint && bun run typecheck
cd ricegrep && bun run format && bun run typecheck
```

### Docker (Full Platform)

```bash
docker-compose up -d                               # Full GPU platform (uses .env defaults)
docker-compose --profile cpu --profile full up -d  # Full CPU platform
docker-compose logs -f api                         # Watch API logs
bash scripts/smoke_test.sh                         # End-to-end test
```

### Troubleshooting & Reset

```bash
# View logs
docker-compose logs -f infinity                    # Embedding server logs
docker-compose logs -f api                         # API logs (if running in Docker)
docker logs rice-milvus --tail 100                 # Milvus logs

# Restart specific service (clears internal queues)
docker-compose restart infinity                    # Reset embedding queue backlog

# Full reset (clears all data)
docker-compose down -v
rm -rf ./data
docker-compose up -d                               # Or --profile cpu --profile full

# Check service health
docker-compose ps
curl http://localhost:8081/health                  # Infinity health
curl http://localhost:9091/healthz                 # Milvus health (correct)
```

### ricegrep CLI

```bash
cd ricegrep
bun install && bun run build                       # Build CLI
bun run format && bun run typecheck                # Quality checks
bun test                                           # Run all tests
bun test --filter "Search"                         # Run specific test pattern
```

## Terminology (CRITICAL)

**Generic, Not Code-Specific**: Rice Search is optimized for code but works for any documents. Keep terminology generic:

| ❌ Avoid | ✅ Use Instead |
|----------|---------------|
| "code search" | "semantic search" or "search" |
| "code files" | "files" or "documents" |
| "codebase" | "repository" or "file set" |
| "source code" | "content" or "text" |

**Why?** The same hybrid search (BM25 + embeddings + reranking) works for documentation, configs, logs, or any text. Code-specific features (Tree-sitter, language detection) are **optimizations**, not requirements.

**When writing docs/comments:**
- Frame code features as "optimized for" not "exclusively for"
- Mention that non-code files still work (with regex/generic parsing)
- Tool descriptions should welcome all document types

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

All services use the same ports in both Docker and local dev mode:

| Service | Port | Description |
|---------|------|-------------|
| API | 8080 | Rice Search REST API |
| Web UI | 3000 | Next.js frontend |
| Attu | 8000 | Milvus admin UI (dev profile only) |
| Infinity | 8081 | Embeddings + Reranking server |
| Milvus | 19530 | Vector database |
| Milvus Metrics | 9091 | Milvus health endpoint |
| Redis | 6379 | Job queue |
| MinIO | 9000 | Object storage |
| MinIO Console | 9001 | MinIO admin UI |

### Optional Tools (Profile-based)

Attu (Milvus Admin UI) is available for debugging and comparison but not started by default:

```bash
# Start all services INCLUDING Attu
docker compose --profile tools up -d

# Or start only Attu (after core services are running)
docker compose --profile tools up -d attu

# Access Attu at http://localhost:8000
# Connect to: milvus:19530 (pre-configured via MILVUS_URL env var)
```

**Attu Features for Comparison:**
- Collection management and schema viewer
- Vector search interface with filters
- Data import/export (CSV/JSON)
- System topology and node metrics
- Index management

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

---

## Go Search (go-search/)

Pure Go rewrite of Rice Search. Single binary, no Python/Node dependencies.

### Tech Stack

| Component | Technology |
|-----------|------------|
| Language | Go 1.21+ |
| Vector DB | Qdrant (not Milvus) |
| ML Inference | ONNX Runtime |
| Web UI | templ + HTMX + Tailwind |
| Communication | Event-driven (Go channels) |

### Local Development

```bash
cd go-search

# 1. Start Qdrant and Redis
docker-compose -f deployments/docker-compose.dev.yml up -d

# 2. Verify services are running
curl http://localhost:6333/healthz    # Qdrant
docker exec rice-redis-dev redis-cli ping  # Redis
# Dashboard: http://localhost:6333/dashboard

# 3. Download ML models (first time only)
./rice-search models download

# 4. Run server from code (hot reload with air, or manual restart)
go run ./cmd/rice-search-server
# Or build and run:
go build -o rice-search.exe ./cmd/rice-search && ./rice-search.exe serve

# 5. Access Web UI at http://localhost:8080
```

### Build Commands

```bash
cd go-search

# Build
make build                    # Build binary
go build ./...                # Verify compilation

# Code Generation
templ generate ./internal/web/   # Generate templ Go files
make proto                       # Regenerate protobuf (requires protoc)

# Test
make test                     # Run all tests
go test ./internal/...        # Test specific packages
```

### Go Quality Checks (CRITICAL)

**ALWAYS use these CLI tools for verification - NEVER trust IDE warnings (IntelliJ/VSCode/GoLand cache stale data).**

```bash
cd go-search

# REQUIRED: Must pass before any PR/completion
go build ./...                # Must compile - BLOCKING
go vet ./...                  # Static analysis - BLOCKING

# RECOMMENDED: Check for new issues in changed files
golangci-lint run             # Comprehensive linting

# OPTIONAL: Deeper analysis for specific packages
staticcheck ./...             # Additional static analysis  
gopls check ./internal/...    # LSP-based diagnostics
```

| Tool | Purpose | Blocking? |
|------|---------|-----------|
| `go build ./...` | Compilation | **YES** - must pass |
| `go vet ./...` | Built-in static analysis | **YES** - must pass |
| `golangci-lint run` | Comprehensive linting | **NO** - check for new issues only |
| `staticcheck ./...` | Deep static analysis | **NO** - informational |
| `gopls check` | LSP diagnostics | **NO** - for specific investigation |

**Minimum verification before completing Go work:**
```bash
go build ./... && go vet ./...
```

**For golangci-lint:** The codebase has ~250 existing style warnings (comment periods, naming conventions). When making changes:
1. Don't introduce NEW errors in files you modify
2. Don't worry about pre-existing warnings in untouched code
3. Run `golangci-lint run ./path/to/changed/...` to check only your changes

### TODO Policy (CRITICAL)

**No unfinished work. When you encounter a TODO comment:**

1. **Small/Medium scope** → Implement it immediately
2. **Large scope (big context switch)** → Ask user: "Found TODO: [description]. This requires [effort]. Should I implement now or defer?"
3. **After implementing** → Remove the TODO comment

**Periodically scan for TODOs:**
```bash
rg "TODO|FIXME|XXX|HACK" --type go
```

**Never leave TODOs you created. Never ignore TODOs you find.**

### Service Ports (go-search)

| Service | Port | Description |
|---------|------|-------------|
| Rice Search Server | 8080 | HTTP API + Web UI |
| Qdrant HTTP | 6333 | Vector DB API + Dashboard |
| Qdrant gRPC | 6334 | Vector DB gRPC |
| Redis | 6379 | Cache/Bus (distributed mode) |

### Directory Structure

```
go-search/
├── api/proto/              # Protobuf definitions
├── cmd/
│   ├── rice-search/        # CLI binary
│   └── rice-search-server/ # Server binary
├── deployments/
│   └── docker-compose.dev.yml  # Qdrant + Redis (dev)
├── internal/
│   ├── bus/                # Event bus
│   ├── client/             # HTTP client
│   ├── config/             # Configuration
│   ├── connection/         # PC/connection tracking
│   ├── index/              # Indexing pipeline
│   ├── metrics/            # Prometheus metrics
│   ├── ml/                 # ML inference (ONNX)
│   ├── models/             # Model registry & mappers
│   ├── onnx/               # ONNX runtime wrapper
│   ├── qdrant/             # Qdrant client
│   ├── query/              # Query understanding
│   ├── search/             # Search service
│   ├── server/             # HTTP/gRPC server
│   ├── store/              # Store management
│   └── web/                # Web UI (templ + HTMX)
├── models/                 # Downloaded ONNX models
└── docs/                   # Documentation
```

### Key Differences from Main (NestJS) Version

| Aspect | Main (api/) | Go (go-search/) |
|--------|-------------|-----------------|
| Vector DB | Milvus | Qdrant |
| ML Runtime | Infinity server | ONNX Runtime (embedded) |
| Web Framework | NestJS | net/http + templ |
| Frontend | Next.js (separate) | templ + HTMX (embedded) |
| BM25 | Tantivy (sidecar) | Built-in sparse vectors |
| Deployment | Multiple containers | Single binary |

### Configuration

Environment variables with `RICE_` prefix:

```bash
RICE_HOST=0.0.0.0
RICE_PORT=8080
RICE_LOG_LEVEL=debug          # debug, info, warn, error
RICE_LOG_FORMAT=text          # text, json
QDRANT_URL=http://localhost:6333
RICE_ML_DEVICE=cpu            # cpu, cuda
RICE_CACHE_TYPE=memory        # memory, redis
```

### Troubleshooting (go-search)

```bash
# Reset all dev data (loses all data)
cd go-search
docker-compose -f deployments/docker-compose.dev.yml down
rm -rf data/qdrant-dev data/redis-dev
docker-compose -f deployments/docker-compose.dev.yml up -d

# Check Qdrant collections
curl http://localhost:6333/collections

# View server logs
./rice-search serve 2>&1 | tee server.log

# Regenerate templates after changes
templ generate ./internal/web/
```