
━━━━━━━━━━━━━━━━━━━━━━
DEPLOYMENT REQUIREMENTS
━━━━━━━━━━━━━━━━━━━━━━
- Docker Compose
- All data persisted via bind mounts under `./data`
- No anonymous Docker volumes
- GPU support via Docker + WSL2 where available
- CPU fallback without code changes

Services:
- backend
- qdrant
- frontend
- optional inference services (or embedded in backend)

━━━━━━━━━━━━━━━━━━━━━━
INGESTION DOMAIN (MUST IMPLEMENT)
━━━━━━━━━━━━━━━━━━━━━━
- AST-aware parsing using Tree-sitter
- Languages (initial):
  - Python, JS/TS, Go, Rust, Java, C/C++
- Chunk by:
  - Functions
  - Classes
  - Methods
  - Logical blocks
- Extract:
  - Symbols (functions, classes)
  - Imports
  - Language
  - File path
- Respect ignore rules:
  - .gitignore
  - .riceignore
  - default ignores
- Content-addressable chunks:
  - Stable chunk_id
  - Hash-based deduplication
- Watch mode:
  - Incremental updates
  - Delete removed files
  - Debounced FS events

━━━━━━━━━━━━━━━━━━━━━━
INDEXING & STORAGE (QDRANT)
━━━━━━━━━━━━━━━━━━━━━━
Qdrant is the **single source of truth**.

For each chunk, store:
- Dense vector (embedding)
- Sparse vector (SPLADE)
- Payload:
  - path
  - language
  - symbols
  - chunk_type
  - start_line
  - end_line
  - store
  - content_hash

Collections:
- One collection per store OR partitions per store (pick one, document rationale)

Indexing requirements:
- GPU-aware embedding
- Sparse + dense vectors indexed together
- Delete by path and by store
- Reindex safely

━━━━━━━━━━━━━━━━━━━━━━
SEARCH DOMAIN (MUST IMPLEMENT)
━━━━━━━━━━━━━━━━━━━━━━
- Query understanding:
  - Intent classification (lookup vs explanation)
  - Scope detection (path, language, store)
- Retrieval:
  - Dense search (embedding)
  - Sparse search (SPLADE)
- Fusion:
  - RRF (default, configurable)
- Neural reranking:
  - Cross-encoder on top-K
- Deterministic results

━━━━━━━━━━━━━━━━━━━━━━
INTELLIGENCE / RAG DOMAIN
━━━━━━━━━━━━━━━━━━━━━━
- Line-level citations REQUIRED
- Multi-step retrieval
- MCP-compatible tools:
  - search
  - read_file
  - list_files
- Structured outputs:
  - Markdown
  - JSON

━━━━━━━━━━━━━━━━━━━━━━
MODEL REGISTRY (MANDATORY)
━━━━━━━━━━━━━━━━━━━━━━
Default models (MUST ship enabled):

- Embedding:
  - jinaai/jina-code-embeddings-1.5b
- Sparse (SPLADE):
  - naver/splade-cocondenser-ensembledistil
- Reranker:
  - jinaai/jina-reranker-v2-base-multilingual
- Query Understanding:
  - microsoft/codebert-base

Model lifecycle:
- Download / delete models via Admin UI
- Hot-swap models at runtime
- Per-model GPU/CPU toggle
- Offline execution (ONNX preferred)
- Declarative YAML configs

━━━━━━━━━━━━━━━━━━━━━━
FRONTEND (SEARCH + ADMIN)
━━━━━━━━━━━━━━━━━━━━━━
Single frontend project.

Pages:
- Search Console
- Answer View (RAG)
- File Explorer
- Store Gallery / Store Detail
- Admin (“Mission Control”):
  - Model management
  - GPU toggles
  - Runtime config editor
  - Event bus config
  - Metrics & resource usage

Frontend is:
- Read-only for search
- Control-plane for admin
- No business logic

━━━━━━━━━━━━━━━━━━━━━━
CLI (rice-search)
━━━━━━━━━━━━━━━━━━━━━━
- mgrep-compatible UX
- Commands:
  - rice-search index <path>
  - rice-search watch <path>
  - rice-search search "<query>"
- Configurable backend URL
- No cloud auth
- Uses backend embeddings only

Provide:
- File-by-file diff plan to adapt mixedbread-ai/mgrep into rice-search

━━━━━━━━━━━━━━━━━━━━━━
API REQUIREMENTS
━━━━━━━━━━━━━━━━━━━━━━
Backend exposes ONLY port 8080.

Endpoints (minimum):
- GET  /healthz
- GET  /v1/version
- GET  /v1/stores
- POST /v1/stores/{store}/index
- POST /v1/stores/{store}/search
- POST /v1/stores/{store}/delete
- GET  /v1/stores/{store}/stats
- POST /v1/mcp/tools/search

OpenAPI docs required.



QUERY UNDERSTANDING (MANDATORY)

Before retrieval, the backend MUST perform query understanding:

- Classify intent:
  - lookup (exact symbol / path / token)
  - explanation (why / how / conceptual)
  - exploration (broad discovery)
- Infer scope:
  - language hints
  - path hints
  - symbol hints

The retrieval strategy MUST adapt based on intent:
- lookup → sparse-first
- explanation → dense-first
- exploration → hybrid + reranking

HYBRID RETRIEVAL POLICY

Hybrid fusion is POLICY-DRIVEN:
- Default: Reciprocal Rank Fusion (RRF)
- Alternatives:
  - Sparse-first
  - Dense-first
  - Weighted sum
- Strategy is configurable at runtime via Admin UI
- All strategies must be deterministic

MODEL REGISTRY ENHANCEMENTS

Each model entry MUST define:
- Model type (embedding / sparse / reranker / query-understanding)
- Input format
- Output tensors
- Execution mode (GPU / CPU)
- Runtime switchability without restart

Models MUST be hot-swappable via Admin UI.

DETERMINISM REQUIREMENT

For identical query + identical index state:
- Results MUST be identical
- Ranking MUST be stable
- Fusion MUST be deterministic

CITATION REQUIREMENTS

All search results and RAG answers MUST include:
- File path
- Start line
- End line


━━━━━━━━━━━━━━━━━━━━━━
OBSERVABILITY & OPS
━━━━━━━━━━━━━━━━━━━━━━
- Structured logs
- Prometheus metrics endpoint
- Correlation IDs
- Admin audit log

━━━━━━━━━━━━━━━━━━━━━━
DELIVERABLE
━━━━━━━━━━━━━━━━━━━━━━
- Full repository tree
- docker-compose.yml (+ optional gpu override)
- backend code
- frontend code
- CLI adaptation plan + diffs
- README with step-by-step instructions
- Smoke test scripts

DO NOT:
- Skip Tree-sitter
- Skip sparse retrieval
- Skip model management
- Skip frontend admin
- Split into multiple external APIs
- Use cloud services

Build the full system.
