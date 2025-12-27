# Rice Search - Agent Guidelines

## Build & Test Commands

```bash
# API (NestJS + Bun)
cd api
bun install && bun run build && bun run start:dev  # Dev server
bun run lint && bun run typecheck                  # Quality checks
bun test                                           # Run all tests

# ricegrep CLI (TypeScript + Bun)  
cd ricegrep
bun install && bun run build                       # Build CLI
bun run format && bun run typecheck                # Quality checks
bun test                                           # Run all tests
bun test --filter "Search"                         # Run specific test pattern

# Web UI (Next.js + Bun)
cd web-ui
bun install && bun run build                       # Build for production
bun run dev                                        # Dev server (http://localhost:3000)
bun run lint                                       # ESLint checks

# Full Platform
docker-compose up -d                               # Start all services
bash scripts/smoke_test.sh                         # End-to-end test
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

Run `bun run typecheck` before any significant changes. Never commit without clean diagnostics.

## Service Ports (Default)

| Service | Port | Description |
|---------|------|-------------|
| API | 8080 | Rice Search REST API |
| Web UI | 3000 | Next.js frontend |
| Attu | 8000 | Milvus admin UI |
| Embeddings | 8081 | Text embeddings inference |
| Milvus | 19530 | Vector database |
| Milvus Metrics | 9091 | Milvus health/metrics |
| MinIO | 9000 | Object storage |
| MinIO Console | 9001 | MinIO admin UI |

## API Endpoints

Base URL: `http://localhost:8080`  
Swagger Docs: `http://localhost:8080/docs`

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
| POST | `/v1/stores/{store}/search` | Hybrid search (BM25 + semantic) |

**Request Body:**
```json
{
  "query": "search text",           // Required
  "top_k": 20,                      // Number of results (default: 20)
  "sparse_weight": 0.5,             // BM25 weight (default: 0.5)
  "dense_weight": 0.5,              // Semantic weight (default: 0.5)
  "group_by_file": false,           // Group by file (default: false)
  "include_content": true,          // Include content (default: true)
  "filters": {
    "path_prefix": "src/",          // Optional path filter
    "languages": ["typescript"]     // Optional language filter
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

### MCP (Model Context Protocol)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/mcp/tools` | List available MCP tools |
| POST | `/mcp` | Handle MCP JSON-RPC request |
| POST | `/mcp/tools/call` | Call MCP tool directly |