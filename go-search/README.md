# Rice Search - Go Edition

Pure Go code search platform. Single binary. Event-driven. GPU-first.

## Quick Summary

| Aspect | Value |
|--------|-------|
| **Language** | Go (no Python, no sidecars) |
| **Vector DB** | Qdrant (native hybrid search) |
| **ML Inference** | ONNX Runtime (GPU-first) |
| **Web UI** | templ + HTMX (no JavaScript framework) |
| **Communication** | Event-driven (Go channels) |
| **Deployment** | Single binary + Qdrant |

## Implementation Status

> **Status**: ✅ **Production-Ready** (as of 2025-12-29)

| Component | Status | Highlights |
|-----------|--------|------------|
| **Core Infrastructure** | ✅ Complete | Config, logger, errors, event bus |
| **ML Service** | ✅ Complete | ONNX runtime, GPU-first, per-model toggles |
| **Search Service** | ✅ Complete | Hybrid RRF, reranking, connection scoping |
| **Index Pipeline** | ✅ Complete | Semantic chunking, 60+ languages |
| **Store Management** | ✅ Complete | Full CRUD, per-store configs |
| **Web UI** | ✅ Complete | 48 routes, 8 pages, admin dashboard |
| **Connection Tracking** | ✅ Complete | Unique feature: auto search scoping |
| **Settings System** | ✅ Complete | 80+ settings, export/import |
| **Metrics** | ✅ Complete | 40+ Prometheus metrics |

See [docs/IMPLEMENTATION.md](docs/IMPLEMENTATION.md) for detailed status.  
See [docs/TODO.md](docs/TODO.md) for remaining features.

## Key Features

### 🔍 Hybrid Search
- **Sparse + Dense retrieval** with Qdrant native RRF fusion
- **Neural reranking** with Jina Reranker
- **Query understanding** with intent detection and keyword expansion

### 🖥️ Full Admin UI
- **Dashboard**: Health indicators, quick stats, recent activity
- **Search**: 12+ configurable options, query understanding display
- **Stores**: Create/delete, connected clients, statistics
- **Files**: Paginated browser, advanced filters, bulk operations
- **Models**: Download, GPU toggle, set defaults
- **Settings**: 80+ settings, export/import, source tracking

### 🔗 Connection-Aware (Unique Feature)
- Automatic client identification (MAC/hostname-based)
- **Default search scoping** - each client sees only their indexed files
- Per-connection activity tracking and monitoring
- Optional cross-connection search with explicit opt-out

### ⚡ GPU-First Architecture
- All ML models default to GPU (`cuda`)
- Per-model GPU toggles (embed, rerank, query)
- Transparent CPU fallback with health reporting
- Device status visible in UI and API

### 📊 Comprehensive Observability
- 40+ Prometheus metrics
- Time-series dashboards with 13 presets
- Per-store, per-connection breakdowns
- Auto-refresh stats page

## Quick Start

```bash
# 1. Start Qdrant
docker-compose -f deployments/docker-compose.dev.yml up -d
# Dashboard: http://localhost:6333/dashboard

# 2. Download ML models
./rice-search models download

# 3. Start the server
./rice-search serve

# 4. Access Web UI
open http://localhost:8080

# 5. Index your code
./rice-search index ./src -s myproject

# 6. Search!
./rice-search search "authentication handler" -s myproject
```

## Web UI Pages

| Page | URL | Description |
|------|-----|-------------|
| Dashboard | `/` | Overview, health, quick stats |
| Search | `/search` | Full-featured search with options |
| Stores | `/stores` | Store management |
| Files | `/files` | File browser with filters |
| Models | `/admin/models` | Model management, GPU toggles |
| Mappers | `/admin/mappers` | Model I/O mappings |
| Connections | `/admin/connections` | Client tracking |
| Settings | `/admin/settings` | All 80+ settings |
| Stats | `/stats` | Time-series dashboards |

## CLI Commands

```bash
# Server
rice-search serve                    # Start on :8080
rice-search serve -p 9000            # Custom port

# Stores
rice-search stores list              # List all stores
rice-search stores create myproject  # Create store
rice-search stores stats myproject   # Get statistics
rice-search stores delete myproject  # Delete store

# Indexing
rice-search index ./src              # Index directory
rice-search index ./src -s myproject # Specific store
rice-search index ./src --force      # Force re-index

# Search
rice-search search "query"           # Search default store
rice-search search "query" -s store  # Specific store
rice-search search "query" -k 50     # More results
rice-search search "query" --no-rerank # Disable reranking
rice-search search "query" --format json # JSON output

# Models
rice-search models list              # List models
rice-search models download          # Download all
rice-search models check             # Verify installed
```

## Configuration

### Environment Variables

```bash
# Server
RICE_HOST=0.0.0.0
RICE_PORT=8080
RICE_LOG_LEVEL=info          # debug, info, warn, error

# ML (GPU-first defaults)
RICE_ML_DEVICE=cuda          # cuda, cpu, tensorrt
RICE_EMBED_GPU=true          # Per-model GPU toggles
RICE_RERANK_GPU=true
RICE_QUERY_GPU=true

# Qdrant
QDRANT_URL=http://localhost:6333

# Search
RICE_DEFAULT_TOP_K=20
RICE_ENABLE_RERANKING=true
```

### Default Models

| Type | Model | Size |
|------|-------|------|
| Embed | `jinaai/jina-code-embeddings-1.5b` | ~1.5GB |
| Rerank | `jinaai/jina-reranker-v2-base-multilingual` | ~800MB |
| Query | `microsoft/codebert-base` | ~438MB |

## Development

```bash
# Start infrastructure
make dev-up

# Build
make build
go build ./...

# Test
make test
go test ./internal/...

# Lint
go vet ./...
golangci-lint run

# Generate templates
templ generate ./internal/web/
```

## Architecture

```
┌────────────────────────────────────────────────────────┐
│                  rice-search binary                     │
├────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────┐  │
│  │                  Web UI (templ + HTMX)          │  │
│  │  Dashboard | Search | Stores | Admin | Stats   │  │
│  └─────────────────────────────────────────────────┘  │
│                         │                              │
│  ┌─────────────────────────────────────────────────┐  │
│  │                  HTTP Server                     │  │
│  │  /v1/search | /v1/stores | /admin | /metrics   │  │
│  └─────────────────────────────────────────────────┘  │
│                         │                              │
│  ┌─────────────────────────────────────────────────┐  │
│  │                  Event Bus (Go channels)         │  │
│  │  Request/Reply | Pub/Sub | Graceful Fallback   │  │
│  └─────────────────────────────────────────────────┘  │
│          │              │              │               │
│     ┌────┴────┐    ┌────┴────┐    ┌────┴────┐        │
│     │   ML    │    │ Search  │    │  Index  │        │
│     │ Service │    │ Service │    │ Service │        │
│     └────┬────┘    └────┬────┘    └────┬────┘        │
│          │              │              │               │
│     ┌────┴────┐    ┌────┴─────────────┴────┐        │
│     │  ONNX   │    │        Qdrant         │        │
│     │ Runtime │    │   (Hybrid Search)     │        │
│     └─────────┘    └───────────────────────┘        │
└────────────────────────────────────────────────────────┘
```

## Comparison with NestJS Version

| Aspect | NestJS (api/) | Go (go-search/) |
|--------|---------------|-----------------|
| Containers | 6+ (12GB+) | 2 (~4GB) |
| Vector DB | Milvus | Qdrant |
| ML Runtime | Infinity (Python) | ONNX (embedded) |
| BM25 | Tantivy sidecar | SPLADE vectors |
| Web UI | Next.js (separate) | templ + HTMX (embedded) |
| Connection Tracking | ❌ None | ✅ Full support |
| Admin Settings | ❌ Limited | ✅ 80+ settings |

## Documentation

| Category | Documents |
|----------|-----------|
| **Architecture** | [Overview](docs/01-architecture.md), [Events](docs/02-events.md), [Data Models](docs/03-data-models.md) |
| **Features** | [Search](docs/04-search.md), [Indexing](docs/05-indexing.md), [ML](docs/06-ml.md) |
| **Interfaces** | [API](docs/07-api.md), [CLI](docs/08-cli.md), [Config](docs/09-config.md) |
| **Operations** | [Observability](docs/13-observability.md), [Health](docs/14-health.md), [Errors](docs/16-errors.md) |
| **Status** | [Implementation](docs/IMPLEMENTATION.md), [TODO](docs/TODO.md) |

## Service Ports

| Service | Port | Description |
|---------|------|-------------|
| Rice Search | 8080 | HTTP API + Web UI |
| Qdrant HTTP | 6333 | Vector DB + Dashboard |
| Qdrant gRPC | 6334 | Vector DB gRPC |
| Redis | 6379 | Optional (metrics persistence) |

## License

CC BY-NC-SA 4.0
