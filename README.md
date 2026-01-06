<div align="center">

<img src=".branding/logo.svg" alt="Rice Search" width="120">

# Rice Search

[![License: CC BY-NC-SA 4.0](https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey.svg)](https://creativecommons.org/licenses/by-nc-sa/4.0/)

**Intelligent hybrid search platform with adaptive retrieval**

[Getting Started](docs/getting-started.md) • [Documentation](docs/README.md) • [API Reference](docs/api.md) • [CLI Guide](docs/cli.md)

</div>

## Overview

Rice Search is a **fully local, self-hosted** hybrid search platform that combines lexical keyword search (BM25) with semantic vector search. Built for code search and document retrieval, it uses intelligent query routing to automatically select the optimal search strategy based on query intent.

### Why Rice Search?

- **🎯 Adaptive Retrieval** - Automatically routes queries to the best search strategy (sparse-only, balanced, dense-heavy, or deep-rerank)
- **🔒 Fully Local** - All data and processing stays on your machine, no external API calls
- **🧠 Multi-Modal Search** - Combines BM25, SPLADE sparse embeddings, and BM42 hybrid vectors
- **🌳 AST-Aware** - Understands code structure across 7+ languages (Python, JS/TS, Go, Rust, Java, C++)
- **⚡ Fast CLI** - Rust-based command-line tool with file watching and auto-indexing
- **🔌 MCP Support** - Integrates with Claude Desktop and other AI assistants via Model Context Protocol

## Key Features

### Intelligent Search

- **Triple Retrieval System**
  - BM25 lexical search via Tantivy (Rust)
  - SPLADE learned sparse embeddings
  - BM42 Qdrant-native hybrid vectors
  - Results fused with Reciprocal Rank Fusion (RRF)

- **Adaptive Query Routing**
  - Intent classification (navigational, factual, exploratory, analytical)
  - Automatic strategy selection based on query characteristics
  - Configurable per-query retriever toggling

- **Advanced Ranking**
  - Two-stage reranking with cross-encoder
  - Optional LLM-based reranking for complex queries
  - Deduplication by file path (highest-scoring chunk per file)

### Code-Aware Indexing

- **AST Parsing** with Tree-sitter - Semantic chunking at function/class boundaries
- **Multi-Language Support** - Python, JavaScript, TypeScript, Go, Rust, Java, C++
- **Smart Chunking** - 1000 chars with 200 char overlap, preserves context
- **File Path Indexing** - Search by filename or full path

### Developer Experience

- **REST API** - FastAPI with OpenAPI documentation
- **Web UI** - Modern Next.js 14 interface with code highlighting
- **Rust CLI** (`ricesearch`) - Fast search and file watching
- **Python CLI** - Embedded in backend for advanced use cases
- **Docker Compose** - One-command deployment with all services

## Quick Start

### Prerequisites

- **Docker & Docker Compose** - For running services
- **10GB disk space** - For vector database and model cache
- **8GB RAM minimum** - 16GB recommended

### 1. Clone and Start

```bash
# Clone the repository
git clone https://github.com/yourusername/rice-search.git
cd rice-search

# Start all services
make up

# View logs
make logs
```

Services will start on:
- **Frontend**: http://localhost:3000
- **API**: http://localhost:8000
- **API Docs**: http://localhost:8000/docs

### 2. Install CLI

```bash
# Install the ricesearch CLI tool
cd backend
pip install -e .

# Verify installation
ricesearch --version
```

### 3. Index Your Code

```bash
# Index the current directory
ricesearch index ./backend

# Or watch for changes (auto-reindex)
ricesearch watch ./backend --org-id myproject
```

### 4. Search

**Via CLI:**
```bash
# Search indexed code
ricesearch search "authentication" --limit 10

# Search for specific files
ricesearch search "config.yaml"
```

**Via Web UI:**
Open http://localhost:3000 and enter your search query.

**Via API:**
```bash
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "user authentication", "limit": 5, "mode": "search"}'
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│ FRONTEND (Next.js)              │ BACKEND API (FastAPI)     │
│ Port: 3000                      │ Port: 8000                │
└─────────────────────────────────┴───────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ INFRASTRUCTURE                                              │
│  Qdrant (6333)  Redis (6379)  MinIO (9000/9001)            │
└─────────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ INFERENCE SERVICES                                          │
│  Ollama (11434)  - Embeddings/Rerank/LLM                   │
│  Tantivy (3002)  - BM25 Rust Service                       │
└─────────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────────┐
│ WORKERS                                                     │
│  Celery Worker - Async indexing tasks                      │
└─────────────────────────────────────────────────────────────┘
```

**Technology Stack:**
- **Backend**: Python 3.12, FastAPI, Celery
- **Frontend**: Next.js 14, React 18, TypeScript
- **Search**: Tantivy (Rust), Qdrant, SPLADE, BM42
- **Inference**: Ollama (qwen3-embedding:4b, qwen2.5-coder:1.5b)
- **Storage**: Redis, MinIO, Qdrant
- **CLI**: Rust (clap, tokio, notify)

## Project Structure

```
rice-search/
├── backend/           # Python FastAPI backend
│   ├── src/
│   │   ├── api/      # REST API endpoints
│   │   ├── services/ # Business logic (search, ingestion, inference)
│   │   ├── cli/      # CLI tools (ricesearch, MCP)
│   │   ├── worker/   # Celery worker
│   │   └── core/     # Configuration, settings manager
│   ├── tests/        # Unit, integration, E2E tests
│   └── settings.yaml # Centralized configuration
├── frontend/         # Next.js 14 web application
│   └── src/
│       ├── app/      # Pages (search, admin, stores)
│       └── components/ # UI components
├── rust-tantivy/     # Rust BM25 search service
│   └── src/          # Axum HTTP server + Tantivy
├── client/           # Rust CLI client (alternative to Python CLI)
│   └── src/          # Search, watch commands
├── deploy/           # Docker orchestration
│   ├── docker-compose.yml           # Main services
│   └── docker-compose.enterprise.yml # Keycloak, Jaeger
├── docs/             # Documentation (you are here)
└── Makefile          # Build commands
```

## Documentation

| Document | Description |
|----------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, setup, first search |
| [Architecture](docs/architecture.md) | System design, components, data flow |
| [Configuration](docs/configuration.md) | Settings reference, environment variables |
| [CLI Guide](docs/cli.md) | Command-line usage (`ricesearch`) |
| [API Reference](docs/api.md) | REST API endpoints |
| [Development](docs/development.md) | Dev workflow, testing, debugging |
| [Deployment](docs/deployment.md) | Production deployment, scaling |
| [Testing](docs/testing.md) | Unit, integration, E2E tests |
| [Troubleshooting](docs/troubleshooting.md) | Common issues, debugging |
| [Security](docs/security.md) | Authentication, authorization |

**Full documentation index:** [docs/README.md](docs/README.md)

## Service Ports

| Service | Port | Description |
|---------|------|-------------|
| Frontend | 3000 | Web UI |
| Backend API | 8000 | REST API |
| Ollama | 11434 | LLM & embeddings |
| Tantivy | 3002 | BM25 search |
| Qdrant | 6333 | Vector database |
| Redis | 6379 | Task queue & cache |
| MinIO | 9000 | Object storage |
| MinIO Console | 9001 | Admin UI |

## Configuration Highlights

**Embedding Model:** qwen3-embedding:4b (2560 dimensions)
**LLM Model:** qwen2.5-coder:1.5b (code-focused)
**Reranker:** cross-encoder/ms-marco-MiniLM-L-12-v2

All settings configurable via `backend/settings.yaml` or environment variables.
See [Configuration Guide](docs/configuration.md) for details.

## Development

```bash
# Install dependencies
make install

# Run tests
make test

# Run E2E tests
make e2e

# View API logs
make api-logs

# View worker logs
make worker-logs
```

See [Development Guide](docs/development.md) for detailed setup instructions.

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](docs/development.md) for:
- Local development setup
- Code style guidelines
- Testing requirements
- Pull request process

**Note:** The current CONTRIBUTING.md is outdated and being replaced by the Development Guide.

## License

CC BY-NC-SA 4.0 - See [LICENSE.md](LICENSE.md)

## Credits

Built with:
- [FastAPI](https://fastapi.tiangolo.com/)
- [Qdrant](https://qdrant.tech/)
- [Tantivy](https://github.com/quickwit-oss/tantivy)
- [Ollama](https://ollama.ai/)
- [Next.js](https://nextjs.org/)
- [Sentence Transformers](https://www.sbert.net/)

---

<div align="center">
Made with ❤️ by the Rice Search Team
</div>
