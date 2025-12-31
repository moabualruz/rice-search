"# Original Project Prompt - Rice Search Platform"  
""  
"**Project Origin:** This document contains the original prompt used to create the **Rice Search Platform** - a fully local hybrid code search system."  
""  
"Rice Search was created by taking mgrep from Mixedbread.ai and porting it to use a completely local hybrid search provider, adding local URL support and self-hosted infrastructure. The resulting platform combines BM25 (Tantivy) + semantic embeddings (Milvus) with a unified API and ricegrep CLI tool."  
""  
"---"  
""  
"# Original Prompt"  
""  
You are a senior full-stack engineer and systems architect. Build a complete, production-quality, cross-platform local “Mixedbread-like” CODE SEARCH platform that runs on Windows + WSL2 + Docker Desktop, and also runs on Linux/macOS (CPU) without code changes. The solution must be fully local and must not require any cloud services. Do not skip any detail. Deliver the full project as a folder tree with all files, and ensure it builds and runs.
PRIMARY GOAL
- Provide a local code search system with BOTH:
  1) exact keyword/symbol/path search (BM25 / sparse retrieval)
  2) semantic search (dense vectors)
- Merge sparse + dense with a robust hybrid ranker.
- Provide ONE unified API endpoint for all client integrations (mgrep, UI, MCP). Internally it may proxy to other services.
MANDATORY FEATURES
1) Cross-platform and Windows+WSL2 compatible:
   - Must run via Docker Compose on Windows host with WSL2.
   - Must build on Windows+WSL2, Linux, and macOS (CPU).
   - GPU acceleration is OPTIONAL and should be used where available (WSL2 + NVIDIA or Linux + NVIDIA). The system must still work on CPU-only machines.
2) Persistent data on local bind mounts (NOT only Docker volumes):
   - All state must persist on host paths under ./data (bind mounts).
   - Must persist Milvus data, MinIO, etcd, Tantivy index, and API state.
3) One unified API endpoint:
   - Expose a single external API service on localhost:8080.
   - All client traffic goes to this endpoint.
   - Internally, API may call Milvus and embeddings service, but clients never do.
4) Code-first indexing:
   - Must index code repositories and support grep-like workflows.
   - Must support stable incremental updates: re-indexing only changed files.
   - Must support ignore rules (.gitignore + default ignores).
5) Hybrid retrieval:
   - Sparse: BM25-quality retrieval suitable for code tokens, symbols, paths.
   - Dense: semantic embeddings for natural language and conceptual queries.
   - Fusion: RRF + code-specific heuristics (symbol boost, path boost, filename boost).
6) Provide a dashboard:
   - Provide Attu for Milvus inspection and a simple web UI on top of unified API.
7) Provide CLI compatibility:
   - Provide an adaptation plan and file-by-file diff patch for mixedbread-ai/mgrep to use the local API (no cloud auth).
   - CLI must support index, watch, and search against local backend.
8) Provide MCP support:
   - Unified API must expose MCP-compatible endpoints/tools.
   - If easier, include an MCP mode in the unified API; do NOT require a separate external service for MCP (but you may include one optionally).
9) Deliverables:
   - Provide docker-compose.yml
   - Provide unified API code
   - Provide Tantivy sparse index implementation (Option A: embedded Rust binaries invoked from API)
   - Provide Milvus schema + dense search implementation
   - Provide Tree-sitter chunking for code
   - Provide a simple web UI (e.g., Next.js or a minimal static UI) that calls unified API
   - Provide scripts for first-run setup, reindexing, smoke tests
   - Provide complete README with step-by-step commands for Windows+WSL2 and Linux/macOS.
ARCHITECTURE (fixed)
- Services via docker-compose:
  - etcd
  - MinIO
  - Milvus (standalone)
  - Attu
  - embeddings server (HuggingFace TEI or equivalent)
  - unified-api (single endpoint)
- Bind mounts:
  - ./data/etcd → etcd data dir
  - ./data/minio → MinIO data dir
  - ./data/milvus → Milvus data dir
  - ./data/tantivy → Tantivy index dir
  - ./data/api → API state (store registry, checkpoints)
- Exposed ports:
  - unified-api: 8080 (ONLY required external integration endpoint)
  - attu: 8000
  - embeddings: 8081 (optional to expose, but internal is fine)
  - milvus: 19530 (expose for debugging ok)
INDEXING REQUIREMENTS (do not skip)
- Repository indexing:
  - Input: local path(s) (from host) for indexing.
  - API must support indexing via:
    A) client sending file contents (preferred for mgrep)
    B) server-side indexing of mounted repo paths (optional)
- Ignore rules:
  - Respect .gitignore (parse it)
  - Default ignores: node_modules, dist, build, .venv, .git, __pycache__, target, .next, etc.
- File handling:
  - Skip binaries
  - Skip huge files with configurable threshold
  - Track file hashes/mtimes for incremental indexing
- Chunking:
  - Use Tree-sitter for supported languages to chunk by functions/classes/blocks where possible.
  - Fall back to line-based chunking with overlap.
  - Produce chunk metadata: start_line, end_line, language, symbols extracted.
- Symbol extraction:
  - Extract function names, class names, exported symbols where feasible.
  - If Tree-sitter supports symbol nodes, use it; otherwise implement heuristics by language.
- Stable IDs:
  - Each chunk must have stable doc_id = hash(store + path + chunk_id + content_hash) or equivalent.
  - Must allow delete-by-path and reindex-by-path.
SPARSE INDEX (Tantivy) REQUIREMENTS (fully implement)
- Use Rust Tantivy (Option A) as embedded binaries invoked by unified API:
  - Provide a Rust crate under unified-api/tantivy/
  - Build two binaries or one multi-command binary:
    - index: add/update/delete documents
    - search: return top docs for query
- Tantivy schema MUST include:
  - doc_id (stored, indexed)
  - path (stored, indexed; support prefix/path queries)
  - language (stored, indexed)
  - symbols (stored, indexed; tokenization should preserve exact-ish matches)
  - content (stored, indexed)
  - start_line, end_line (stored)
- Tantivy query language:
  - Support plain text queries
  - Support filters: path:<prefix>, lang:<lang>
  - If user includes tokens matching symbols, boost symbol field matches.
- Output for sparse search must include:
  - doc_id, path, start_line, end_line, snippet, bm25_score
- Incremental updates:
  - Upsert by doc_id
  - Delete by doc_id and delete-by-path (bulk)
DENSE INDEX (Milvus) REQUIREMENTS (fully implement)
- Embeddings:
  - Use TEI server for embeddings; unified-api calls it; clients never embed.
  - Model: default to BAAI/bge-base-en-v1.5 (dim=768) but make configurable.
- Milvus schema:
  - collection per store OR partitions per store (choose one; document why)
  - fields:
    - doc_id (primary key string)
    - embedding (float vector dim)
    - path (string)
    - language (string)
    - chunk_id (int)
    - start_line (int)
    - end_line (int)
- Create index (choose HNSW or IVF; configurable) with cosine similarity.
- Implement:
  - collection creation/migration on startup
  - upsert vectors
  - delete by doc_id and delete-by-path
  - vector search topK with optional filters (path prefix, language)
- Return dense search results including:
  - doc_id, path, start_line, end_line, snippet, dense_score
HYBRID RANKER (fully implement)
- Candidate retrieval:
  - sparse_topk default 200
  - dense_topk default 80
  - final_topk default 20
- Fusion:
  - Implement Reciprocal Rank Fusion (RRF) with configurable k (default 60).
- Heuristics (code-specific):
  - Boost exact symbol matches (symbols field) significantly.
  - Boost filename/path matches.
  - If query contains something like “foo.py” or “src/auth/”, boost those.
  - Optional: downrank vendor directories if not filtered.
- Dedup:
  - Provide option to group results by file path (top chunk per file).
- Output:
  - results sorted with final score, include per-source contributions (sparse rank, dense rank) for debugging.
UNIFIED API (NestJs recommended) REQUIREMENTS
- Provide OpenAPI docs at /docs.
- Endpoints (minimum):
  - GET /healthz → {status:"ok"}
  - GET /v1/version → build info
  - GET /v1/stores → list stores
  - POST /v1/stores/{store}/index → index payload (batch)
  - POST /v1/stores/{store}/search → hybrid search
  - POST /v1/stores/{store}/delete → delete by path/doc_id
  - POST /v1/stores/{store}/reindex → optional destructive rebuild (guarded)
  - GET /v1/stores/{store}/stats → counts, index sizes, timestamps
- MCP:
  - Provide MCP tool endpoint(s) under /v1/mcp/...
  - Implement at least code_search tool: {query, top_k, store, filters} returning results.
- Security:
  - Default AUTH_MODE=none, but structure code to add API keys later.
- Robustness:
  - Timeouts, retries for TEI and Milvus
  - Batch embeddings
  - Background jobs for large indexing (optional) but provide synchronous mode
- Logging:
  - Structured logs for indexing/search
  - Request IDs
WEB UI (simple, but real)
- Provide a minimal web UI that runs locally and calls unified-api:
  - Search box, store selector, filters, results list with snippet and path, copy/open actions.
  - Show whether result came from sparse/dense/hybrid contributions.
- You may implement:
  - A minimal Next.js app OR a static HTML/JS UI served by unified-api.
  - Must be included in docker-compose as an optional service OR served by unified-api.
- Must not require external services.
## RICEGREP - PORT OF MGREP

**ricegrep** is a complete port of mixedbread-ai/mgrep that uses Rice Search as its backend:

### What Was Changed:
- Renamed executable from `mgrep` to `ricegrep`
- Replaced cloud API calls with local Rice Search API (http://localhost:8080)
- Removed cloud authentication flows (login/logout still present but configured for local mode)
- Added config/env var RICEGREP_BASE_URL (default http://localhost:8080)
- Implement store selection flag --store (default "default")
- Map CLI actions to local unified-api:
  - `ricegrep watch` → index files and watch for changes, calling /v1/stores/{store}/index
  - `ricegrep search "<query>"` → call /v1/search/{store} and print results with lines/snippets
  - Output remains grep-like (paths, line numbers, context)

### What Was Added:
- Local URL support for connecting to self-hosted Rice Search
- Incremental indexing with file tracking (hash-based change detection)
- Support for .ricegrepignore files (in addition to .gitignore)
- Full integration with local Milvus (vectors) and Tantivy (BM25)
DOCKER COMPOSE (fully implement)
- Provide docker-compose.yml with:
  - bind mounts to ./data/*
  - unified-api built from source
  - embeddings service
  - milvus + etcd + minio + attu
  - healthchecks where reasonable
- Support CPU-only by default.
- Add optional GPU compose override file:
  - docker-compose.gpu.yml enabling --gpus where supported, but system must run without it.
SCRIPTS (provide)
- scripts/smoke_test.sh (or .ps1 for Windows) that:
  - Starts stack
  - Indexes a small demo repo (or sample files)
  - Runs a search query
  - Validates non-empty results
- scripts/reindex.py or equivalent to rebuild store
- scripts/init_store.py to create store indexes/collections
- Provide Windows-friendly instructions in README (WSL2 path tips).
## WHAT WAS DELIVERED

Complete implementation of Rice Search platform with:

### Core Components:
- ✅ **Tantivy** indexing/search binaries (Rust CLI invoked by unified-api)
- ✅ **Milvus** schema + dense vector search with HNSW indexing
- ✅ **Tree-sitter** chunking + line mapping for code-aware parsing
- ✅ **ricegrep** CLI - complete port of mgrep with local backend support
- ✅ **Web UI** - Next.js interface for search
- ✅ **Unified API** - NestJS service on port 8080 with OpenAPI docs
- ✅ **Hybrid ranking** - RRF fusion with symbol/path boosting

### Infrastructure:
- ✅ **Docker Compose** setup for all services
- ✅ **Persistent storage** via bind mounts in ./data/
- ✅ **Cross-platform** support (Windows+WSL2, Linux, macOS)
- ✅ **GPU optional** with docker-compose.gpu.yml override
- ✅ **Fully local** - no cloud dependencies, runs offline after initial image pull

### How to Use:
```bash
# Start Rice Search platform
docker compose up -d

# Install ricegrep CLI
cd ricegrep && npm install -g .

# Index your code
ricegrep watch

# Search semantically  
ricegrep "where do we handle authentication?"
```

External interface: ONE API endpoint on http://localhost:8080
All data persisted in ./data via bind mounts
Everything self-contained and runnable offline once images are built.