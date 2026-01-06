# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Rice Search is a fully local, self-hosted hybrid search platform that combines:
- **BM25 keyword search** via Tantivy (Rust)
- **Dense semantic search** via embeddings (Jina v3)
- **Sparse neural search** via SPLADE/BM42
- **Intelligent retrieval** with query analysis and adaptive strategy routing
- **Multi-pass reranking** for complex queries

The system is designed for code search with AST-aware parsing, multi-language support, and MCP integration for AI assistants.

## Architecture

### Service Layer (Docker Compose)

```
┌─────────────────────────────────────────────────────────────┐
│ FRONTEND (Next.js)              │ BACKEND API (FastAPI)     │
│ Port: 3000                      │ Port: 8000                │
└─────────────────────────────────┴───────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ INFRASTRUCTURE LAYER                                        │
│  Qdrant (6333)  Redis (6379)  MinIO (9000/9001)            │
└─────────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ INFERENCE SERVICES                                          │
│  BentoML (3001)  - Embeddings/Rerank/LLM                   │
│  Tantivy (3002)  - BM25 Rust Service                       │
└─────────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ WORKERS                                                     │
│  Celery Worker - Async indexing tasks                      │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

**Backend (`backend/src/`):**
- `main.py` - FastAPI application entry point
- `api/v1/endpoints/` - REST API endpoints (search, ingest, files, stores, admin)
- `services/`
  - `ingestion/` - File parsing, AST parsing, chunking, indexing
  - `search/` - Query analysis, retrieval, reranking, sparse search
  - `inference/` - Model clients (BentoML, TEI, vLLM, Triton)
  - `mcp/` - Model Context Protocol server (stdio/tcp)
  - `rag/` - RAG engine for chat
  - `query/analyzer.py` - Intent classification & adaptive strategy
  - `model_manager.py` - Model lifecycle with TTL-based auto-unloading
- `tasks/ingestion.py` - Celery tasks for async indexing
- `worker/` - Celery worker startup
- `db/qdrant.py` - Qdrant vector DB client
- `core/` - Config, security, telemetry, device detection
- `cli/ricesearch/` - CLI tool (`ricesearch` command)

**Frontend (`frontend/`):**
- Next.js 14 with TypeScript
- React 18, Mantine UI, Tailwind CSS
- Search UI with file preview, code highlighting, relevance labels

**Deploy (`deploy/`):**
- `docker-compose.yml` - Main orchestration file
- `docker-compose.enterprise.yml` - Enterprise extensions (Keycloak, Jaeger)
- `bentoml/` - BentoML service for unified inference

**Rust Tantivy (`rust-tantivy/`):**
- Standalone BM25 search service in Rust
- HTTP API on port 3002

## Development Commands

### Docker Compose (Recommended)

```bash
# Start all services
make up

# Start with enterprise features (Keycloak, Jaeger)
make up-enterprise

# Stop all services
make down

# View logs
make logs
make api-logs
make worker-logs
```

### Backend Development

```bash
# Install backend dependencies
cd backend
pip install -e .[dev]

# Run tests
pytest                                    # All tests
pytest tests/test_search_services.py     # Specific test file
pytest -k test_query_analyzer            # Specific test pattern
pytest tests/e2e/                        # E2E tests

# Run API locally (requires Docker services running)
cd backend
uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload

# Run Celery worker locally
cd backend
python src/worker/start_worker.py

# CLI tool
ricesearch search "authentication"
ricesearch watch ./src --org-id myorg
ricesearch config show
```

### Frontend Development

```bash
# Install frontend dependencies
cd frontend
npm install

# Run dev server
npm run dev

# Build for production
npm run build

# Lint
npm run lint
```

### Running Tests

```bash
# Backend unit tests
cd backend
pytest

# Backend E2E tests (requires Docker)
make e2e

# Load tests
cd backend/tests/load
locust -f locustfile.py
```

## Key Architectural Patterns

### Triple Retrieval System

The system uses **three parallel retrieval methods** for hybrid search:

1. **BM25 (Tantivy)** - Fast lexical search via Rust service
2. **SPLADE** - Neural sparse vectors (learned term weights)
3. **BM42** - Qdrant-native hybrid sparse+dense

Results are fused using Reciprocal Rank Fusion (RRF) with k=60.

### Adaptive Query Routing

`services/query/analyzer.py` classifies queries by intent and routes to optimal strategy:

- **sparse-only** - Fast BM25 for exact lookups (navigational queries)
- **balanced** - Standard hybrid for general queries
- **dense-heavy** - Semantic-focused for concept searches
- **deep-rerank** - Multi-pass reranking for complex analytical queries

### Two-Stage Reranking

For complex queries, the system uses:
1. **Stage 1**: Fast cross-encoder reranking (top 50 candidates)
2. **Stage 2**: LLM-based reranking with early exit

### AST-Aware Chunking

`services/ingestion/ast_parser.py` uses Tree-sitter to:
- Parse code into AST nodes (functions, classes, methods)
- Chunk at semantic boundaries (not arbitrary line counts)
- Preserve context with parent scope information
- Support 7+ languages (Python, JS/TS, Go, Rust, Java, C++)

### Model Lifecycle Management

`services/model_manager.py` implements:
- **Lazy loading** - Models load on first use
- **TTL-based unloading** - Unused models unload after 5 minutes (configurable)
- **Device awareness** - Auto GPU/CPU detection and allocation
- **Memory tracking** - psutil-based memory monitoring

### Celery Task Queue

Ingestion tasks run asynchronously via Celery:
- `tasks/ingestion.py` defines tasks
- Worker runs via `worker/start_worker.py`
- Redis backend for task queue and results

## Configuration

Configuration is managed via `backend/src/core/config.py` using Pydantic settings.

**Key environment variables:**

```bash
# Infrastructure
QDRANT_URL=http://qdrant:6333
REDIS_URL=redis://redis:6379/0
BENTOML_URL=http://bentoml:3001
TANTIVY_URL=http://tantivy:3002

# Models
EMBEDDING_MODEL=jina-embeddings-v3
EMBEDDING_DIM=1024
RERANK_MODEL=jinaai/jina-reranker-v2-base-multilingual
LLM_MODEL=codellama/CodeLlama-7b-Instruct-hf

# Triple retrieval flags
BM25_ENABLED=true
SPLADE_ENABLED=true
BM42_ENABLED=true

# Features
AST_PARSING_ENABLED=true
RERANK_ENABLED=true
QUERY_ANALYSIS_ENABLED=true
MCP_ENABLED=false

# Performance
MODEL_TTL_SECONDS=300
MODEL_AUTO_UNLOAD=true
FORCE_GPU=true
```

## Testing Guidelines

### Test Structure

- `tests/` - Unit and integration tests
- `tests/e2e/` - End-to-end UI tests with Playwright
- `tests/load/` - Load tests with Locust

### Running Specific Tests

```bash
# Search functionality
pytest tests/test_search_services.py

# Ingestion pipeline
pytest tests/test_ingestion_*.py

# API endpoints
pytest tests/test_api_*.py

# Query analyzer
pytest tests/test_query_analyzer.py

# E2E workflow
pytest tests/e2e/test_ui_workflow.py
```

## MCP Integration

Rice Search implements the Model Context Protocol for AI assistant integration.

**Start MCP server:**

```bash
# StdIO mode (for Claude Desktop, Continue.dev)
cd backend
python -m src.cli.mcp_stdio

# TCP mode (for networked agents)
python -m src.services.mcp.mcp_daemon
```

**Available MCP tools:**
- `search` - Search indexed code/docs
- `read_file` - Read full file content
- `list_files` - List indexed files with glob patterns

## CLI Tool (ricesearch)

**IMPORTANT: Always use the CLI for indexing files, not direct API calls.**

Installed as `ricesearch` command via `pip install -e .`

### Indexing with CLI

**Use `ricesearch watch` for indexing:**

```bash
# Index current directory (recommended)
ricesearch watch .

# Index specific directory
ricesearch watch ./backend --org-id myorg

# Initial index without watching
ricesearch watch ./src --no-initial

# The CLI handles:
# - File path normalization
# - Automatic .gitignore/.riceignore respect
# - Proper metadata (full_path + filename fields)
# - Incremental updates
# - Error handling
```

**Full reindex after schema changes:**

```bash
# The watch command with initial scan reindexes everything
ricesearch watch ./backend --org-id public

# It handles:
# - Scanning all files
# - Updating existing documents
# - Adding new files
# - Removing deleted files
```

### Searching

```bash
# Search
ricesearch search "authentication" --limit 20

# Search for file names (now works!)
ricesearch search "config.yaml"
ricesearch search "README"
ricesearch search "test_api"

# Dense-only search
ricesearch search "function" --no-hybrid
```

### Config management
ricesearch config show
ricesearch config set backend_url http://localhost:8000
```

Supports `.riceignore` files (gitignore syntax) for filtering.

## Important Development Notes

### Celery Worker Debugging

Worker logs are verbose. Key patterns:
- Tasks run in `tasks/ingestion.py`
- Worker startup in `worker/start_worker.py`
- Check Redis connection: `redis-cli ping`
- Monitor tasks: View logs with `make worker-logs`

### Qdrant Collections

Collections are created dynamically per org_id:
- Dense vectors: `{org_id}_dense`
- Sparse vectors: `{org_id}_sparse`
- Schema: 768 or 1024 dim depending on model

### BentoML Service

Single unified service for all inference:
- Embeddings: `/encode`
- Reranking: `/rerank`
- LLM Chat: `/chat` (vLLM backend)

Configured via `deploy/bentoml/bentofile.yaml` and `deploy/bentoml/service.py`.

### Frontend API Integration

Frontend expects backend at `NEXT_PUBLIC_API_URL` (default: `http://localhost:8000`).

API routes use `/api/v1` prefix.

## Agent Protocols

This project follows TDD workflow and verification protocols defined in `.agent/rules/agent-protocols.md`:

1. **Always use TDD** - Write failing tests before implementation
2. **Runtime verification** - Verify changes work at runtime (not just code inspection)
3. **Hierarchy of truth** - User input > Project reality > Web research > Internal memory
4. **Confidence-based execution** - Ask questions when confidence <70%

## Common Gotchas

1. **Port conflicts** - Frontend (3000), API (8000), BentoML (3001), Tantivy (3002), Qdrant (6333), Redis (6379)
2. **GPU memory** - BentoML loads LLM into VRAM; monitor with `nvidia-smi`
3. **Model downloads** - First run downloads ~5GB of models (Hugging Face cache)
4. **Tantivy index** - Persistent volume `tantivy_data` - clear if schema changes
5. **Shared temp dir** - `/shared/ingest` volume required for worker file sharing
6. **AST parsing** - Requires tree-sitter grammars installed via `tree-sitter-languages`

## Service Ports Reference

| Service | Port | Description |
|---------|------|-------------|
| Frontend | 3000 | Next.js UI |
| Backend API | 8000 | FastAPI |
| BentoML | 3001 | Unified inference |
| Tantivy | 3002 | BM25 search |
| Qdrant | 6333 | Vector DB |
| Redis | 6379 | Task queue |
| MinIO | 9000 | Object storage |
| MinIO Console | 9001 | Admin UI |
