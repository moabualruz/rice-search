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
| Phase 2: ML | ✅ | ONNX runtime, embeddings, reranking |
| Phase 3: Search | ✅ | Qdrant, hybrid search, indexing |
| Phase 4: API/CLI | ✅ | HTTP gateway, CLI commands |
| Phase 5: Web UI | ⏭️ | Templ + HTMX interface (skipped - CLI sufficient) |
| Phase 6: Polish | ✅ | Server tests, client tests, documentation |

**Implementation complete!** The system is fully functional with CLI commands for searching, indexing, and store management. All 14 packages have passing tests.

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
# 1. Start Qdrant
make dev-up
# Qdrant API:       http://localhost:6333
# Qdrant Dashboard: http://localhost:6333/dashboard

# 2. Download ML models
./rice-search models download

# 3. Start the server
./rice-search serve

# 4. Index your code
./rice-search index ./src -s myproject

# 5. Search!
./rice-search search "authentication handler" -s myproject
```

### CLI Commands

```bash
# Server
rice-search serve                    # Start API server on :8080
rice-search serve -p 9000            # Custom port

# Stores
rice-search stores list              # List all stores
rice-search stores create myproject  # Create a new store
rice-search stores stats myproject   # Get store statistics
rice-search stores delete myproject  # Delete a store

# Indexing
rice-search index ./src              # Index directory (default store)
rice-search index ./src -s myproject # Index into specific store
rice-search index ./src --force      # Force re-index unchanged files

# Search
rice-search search "query"           # Search default store
rice-search search "query" -s mystore # Search specific store
rice-search search "query" -k 50     # More results
rice-search search "query" --content # Include content in results
rice-search search "query" --no-rerank # Disable reranking
rice-search search "query" --format json # JSON output

# Models
rice-search models list              # List available models
rice-search models download          # Download all models
rice-search models check             # Check installed models
```

### Development Commands

```bash
make dev-up       # Start Qdrant
make dev-down     # Stop Qdrant
make dev-reset    # Reset Qdrant (delete data and restart)
make dev-logs     # View logs
make build        # Build the binary
make test         # Run tests
```

## License

CC BY-NC-SA 4.0
