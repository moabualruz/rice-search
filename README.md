<div align="center">

<img src=".branding/logo.svg" alt="Rice Search" width="120">

# **🔍Rice Search Platform🔎**

[![License: CC BY-NC-SA 4.0](https://img.shields.io/badge/License-CC%20BY--NC--SA%204.0-lightgrey.svg)](https://creativecommons.org/licenses/by-nc-sa/4.0/)

**Intelligent hybrid code search with adaptive retrieval**

</div>

## Overview

Rice Search is a fully local, self-hosted code search platform combining BM25 keyword search with semantic embeddings. Unlike static hybrid search, Rice Search uses **retrieval intelligence** to adapt its search strategy based on query characteristics.

## Key Features

### Intelligent Retrieval
- **Intent Classification** - Detects query type (navigational, factual, exploratory, analytical)
- **Adaptive Strategy** - Routes queries to optimal retrieval path:
  - `sparse-only` - Fast BM25 for exact lookups
  - `balanced` - Standard hybrid for general queries
  - `dense-heavy` - Semantic-focused for concept searches
  - `deep-rerank` - Multi-pass reranking for complex queries
- **Multi-Pass Reranking** - Two-stage neural reranking with early exit for efficiency
- **Query Expansion** - Automatic synonym expansion for better recall

### Post-Processing Pipeline
- **Semantic Deduplication** - Removes near-duplicate chunks (configurable threshold)
- **MMR Diversity** - Maximal Marginal Relevance ensures varied results
- **File Aggregation** - Groups chunks by file with representative selection

### Infrastructure
- **Fully Local** - No external API calls, all data stays on your machine
- **GPU Optional** - CPU by default, GPU acceleration available
- **MCP Support** - Model Context Protocol for AI assistant integration
- **ricegrep CLI** - Fast command-line search with watch mode

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Clients: Web UI (:3000) | ricegrep CLI | MCP | REST API    │
└─────────────────────────────┬───────────────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │    Rice API       │
                    │  (NestJS :8080)   │
                    │                   │
                    │ ┌───────────────┐ │
                    │ │  Intelligence │ │  ← Intent + Strategy
                    │ │    Layer      │ │
                    │ └───────────────┘ │
                    │ ┌───────────────┐ │
                    │ │   Retrieval   │ │  ← Hybrid Search
                    │ │    Layer      │ │
                    │ └───────────────┘ │
                    │ ┌───────────────┐ │
                    │ │   PostRank    │ │  ← Dedup + Diversity
                    │ │   Pipeline    │ │
                    │ └───────────────┘ │
                    └─────────┬─────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
  ┌─────▼─────┐       ┌───────▼───────┐     ┌──────▼──────┐
  │  Tantivy  │       │    Milvus     │     │  Infinity   │
  │  (BM25)   │       │  (Vectors)    │     │ (Embed/Rank)│
  └───────────┘       └───────────────┘     └─────────────┘
```

## Quick Start

### Prerequisites
- Docker & Docker Compose
- 8GB+ RAM (16GB recommended)

### 1. Start Services

```bash
git clone <repo> && cd rice-search
docker-compose up -d
# Wait ~3 minutes for model downloads on first run
```

### 2. Index Your Code

```bash
# Using Python script
python scripts/reindex.py /path/to/your/repo

# Or via API
curl -X POST http://localhost:8080/v1/stores/default/index \
  -H "Content-Type: application/json" \
  -d '{"files": [{"path": "src/main.py", "content": "..."}]}'
```

### 3. Search

**Web UI**: http://localhost:3000

**API**:
```bash
curl -X POST http://localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{"query": "authentication handler"}'
```

**ricegrep CLI**:
```bash
cd ricegrep && npm install -g .
ricegrep "auth middleware"
```

## Search API

```
POST /v1/stores/{store}/search
```

```json
{
  "query": "search text",
  "top_k": 20,
  "filters": {
    "path_prefix": "src/",
    "languages": ["typescript"]
  },
  
  "sparse_weight": 0.5,
  "dense_weight": 0.5,
  "enable_reranking": true,
  
  "enable_dedup": true,
  "dedup_threshold": 0.85,
  "enable_diversity": true,
  "diversity_lambda": 0.7,
  "group_by_file": false,
  "enable_expansion": true
}
```

Response includes intelligence metadata:
```json
{
  "results": [...],
  "intelligence": {
    "intent": "exploratory",
    "difficulty": "medium", 
    "strategy": "dense-heavy",
    "confidence": 0.85
  },
  "reranking": {
    "pass1_applied": true,
    "pass2_applied": false,
    "early_exit": true
  },
  "postrank": {
    "dedup": { "removed": 5 },
    "diversity": { "avg_diversity": 0.72 }
  }
}
```

## ricegrep CLI

```bash
ricegrep "query"              # Basic search
ricegrep -k 50 "query"        # More results
ricegrep --no-rerank "query"  # Skip reranking
ricegrep --no-dedup "query"   # Keep duplicates
ricegrep --group-by-file      # Group results by file
ricegrep -v "query"           # Verbose with stats
ricegrep watch                # Watch mode for indexing
ricegrep mcp                  # MCP server mode
```

## MCP Integration

Rice Search supports the Model Context Protocol for AI assistant integration.

**Available Tools:**
- `code_search` - Hybrid search with all options
- `index_files` - Index code files
- `delete_files` - Remove files from index
- `list_stores` - List search indexes
- `get_store_stats` - Index statistics

**Configuration:**
```json
{
  "mcpServers": {
    "rice-search": {
      "url": "http://localhost:8080/mcp",
      "transport": "http"
    }
  }
}
```

Or use ricegrep as MCP server:
```bash
ricegrep install-claude-code   # Auto-configure for Claude
ricegrep install-opencode      # Auto-configure for OpenCode
```

## GPU Acceleration

```bash
docker-compose -f docker-compose.yml -f docker-compose.gpu.yml up -d
```

Requires NVIDIA GPU with nvidia-container-toolkit.

## Configuration

### Models (via .env)

```bash
# Embeddings (1536d, code-optimized)
EMBED_MODEL=jinaai/jina-code-embeddings-1.5b

# Reranker (fast, code-aware)
RERANK_MODEL=jinaai/jina-reranker-v2-base-multilingual

# Dimension must match embedding model
EMBEDDING_DIM=1536
```

### Service Ports

| Service | Port | Description |
|---------|------|-------------|
| API | 8080 | REST API + MCP |
| Web UI | 3000 | Search interface |
| Milvus | 19530 | Vector database |
| MinIO | 9001 | Storage console |

## Data Persistence

All data in `./data/`:
```
./data/
├── milvus/         # Vector index
├── tantivy/        # BM25 index  
├── api/            # Store metadata
└── infinity-cache/ # Model cache
```

Reset: `rm -rf ./data && docker-compose down -v`

## Development

```bash
# Start infrastructure only
docker-compose up -d milvus infinity etcd minio redis

# Run API locally
cd api && bun install && bun run start:local  # :8088

# Run Web UI locally  
cd web-ui && bun install && bun run dev:local # :3001

# Type checking
cd api && bun run typecheck
cd ricegrep && bun run typecheck
```

## License

CC BY-NC-SA 4.0

## Credits

- [Tantivy](https://github.com/quickwit-oss/tantivy) - BM25 search
- [Milvus](https://milvus.io/) - Vector database
- [Infinity](https://github.com/michaelfeil/infinity) - Embedding/reranking server
- [Jina AI](https://jina.ai/) - Code-optimized models
