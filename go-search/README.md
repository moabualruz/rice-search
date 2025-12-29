# Rice Search - Go Edition

Pure Go code search platform. Single binary. Event-driven microservices.

## Quick Summary

| Aspect | Value |
|--------|-------|
| **Language** | Go (no Python, no sidecars) |
| **Vector DB** | Qdrant |
| **ML Inference** | ONNX Runtime |
| **Communication** | Event-driven (Go channels / Kafka / NATS) |
| **Deployment** | Monolith or Microservices |

## Implementation Status

| Phase | Status | Description |
|-------|--------|-------------|
| [Implementation Plan](docs/IMPLEMENTATION.md) | ✅ | Complete task breakdown |
| Phase 0: Setup | ✅ | Project scaffold, tooling, Docker |
| Phase 1: Core | ✅ | Config, logger, errors, event bus |
| Phase 2: ML | ⬜ | ONNX runtime, embeddings, reranking |
| Phase 3: Search | ⬜ | Qdrant, hybrid search, indexing |
| Phase 4: API/CLI | ⬜ | HTTP gateway, CLI commands |
| Phase 5: Web UI | ⬜ | Templ + HTMX interface |
| Phase 6: Polish | ⬜ | Testing, docs, observability |

**Estimated timeline:** 4-6 weeks (single developer)

## Documentation Index

### Architecture & Design

| Document | Description | Status |
|----------|-------------|--------|
| [Architecture Overview](docs/01-architecture.md) | System design, deployment modes, components | ✅ |
| [Event System](docs/02-events.md) | Event bus design, all event schemas | ✅ |
| [Data Models](docs/03-data-models.md) | Document, Chunk, Store, Vector structures | ✅ |

### Core Features

| Document | Description | Status |
|----------|-------------|--------|
| [Search Algorithm](docs/04-search.md) | Hybrid search flow, RRF fusion, reranking | ✅ |
| [Indexing Pipeline](docs/05-indexing.md) | Chunking, embedding, storage pipeline | ✅ |
| [ML Inference](docs/06-ml.md) | ONNX models, GPU loading, caching | ✅ |

### Interfaces

| Document | Description | Status |
|----------|-------------|--------|
| [HTTP API](docs/07-api.md) | All endpoints, request/response schemas | ✅ |
| [CLI Reference](docs/08-cli.md) | Commands, flags, examples | ✅ |
| [Configuration](docs/09-config.md) | All config options, env vars, defaults | ✅ |

### Infrastructure

| Document | Description | Status |
|----------|-------------|--------|
| [Qdrant Schema](docs/10-qdrant.md) | Collections, indexes, payload structure | ✅ |
| [Directory Structure](docs/11-structure.md) | Package layout, file organization | ✅ |
| [Concurrency](docs/12-concurrency.md) | Worker pools, backpressure, limits | ✅ |

### Operations

| Document | Description | Status |
|----------|-------------|--------|
| [Observability](docs/13-observability.md) | Metrics, logging, tracing | ✅ |
| [Health Checks](docs/14-health.md) | Liveness, readiness, dependencies | ✅ |
| [Graceful Shutdown](docs/15-shutdown.md) | Shutdown sequence, drain, cleanup | ✅ |
| [Error Handling](docs/16-errors.md) | Error codes, response format, retries | ✅ |

### Production

| Document | Description | Status |
|----------|-------------|--------|
| [Security](docs/17-security.md) | Auth, rate limiting, input validation | ✅ |
| [Performance](docs/18-performance.md) | Targets, benchmarks, optimization | ✅ |
| [Testing](docs/19-testing.md) | Unit, integration, e2e strategy | ✅ |
| [Migration](docs/20-migration.md) | Migrating from current NestJS/Milvus | ✅ |

## Quick Start

```bash
# Option 1: Dev mode (ephemeral data, includes Qdrant Web UI)
make dev-up
# Qdrant API: http://localhost:6333
# Qdrant Dashboard: http://localhost:6333/dashboard
# Qdrant Web UI: http://localhost:8001

# Option 2: Production mode (persistent data)
make compose-up

# Run Rice Search locally
make run
# Or: ./rice-search serve
```

### Development Commands

```bash
make dev-up       # Start Qdrant + Web UI (data resets on down)
make dev-down     # Stop and lose all data
make dev-restart  # Fresh restart with clean data
make dev-logs     # View logs
```

## License

CC BY-NC-SA 4.0
