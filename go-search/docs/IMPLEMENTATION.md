# Implementation Plan

## Overview

Total estimated effort: **4-6 weeks** for MVP (single developer)

### Milestones

| Milestone | Target | Description |
|-----------|--------|-------------|
| M0 | Week 0 | Project scaffold, dev environment |
| M1 | Week 1 | Core infrastructure (bus, config, errors) |
| M2 | Week 2 | ML service with ONNX |
| M3 | Week 3 | Search service + Qdrant |
| M4 | Week 4 | API service + CLI |
| M5 | Week 5 | Web UI + Integration |
| M6 | Week 6 | Polish, testing, documentation |

---

## Phase 0: Project Setup (M0)

**Goal:** Bootable Go project with tooling

### Tasks

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 0.1 | Initialize Go module | 15m | - | `go.mod` exists, module path set |
| 0.2 | Create directory structure | 30m | 0.1 | All dirs from 11-structure.md exist |
| 0.3 | Setup Makefile | 30m | 0.2 | `make build`, `make test`, `make lint` work |
| 0.4 | Add linting (golangci-lint) | 15m | 0.1 | `.golangci.yml` configured, `make lint` passes |
| 0.5 | Setup CI (GitHub Actions) | 30m | 0.3 | PR checks run lint + test |
| 0.6 | Create .env.example | 15m | - | All env vars documented |
| 0.7 | Create Dockerfile | 30m | 0.2 | Multi-stage build, minimal image |
| 0.8 | Create docker-compose.yml | 30m | 0.7 | Qdrant + app start with `docker-compose up` |

**Deliverables:**
- [ ] `go.mod`, `go.sum`
- [ ] Directory structure per spec
- [ ] `Makefile` with common targets
- [ ] `.golangci.yml`
- [ ] `.github/workflows/ci.yml`
- [ ] `deployments/docker/Dockerfile`
- [ ] `deployments/docker-compose.yml`
- [ ] `.env.example`

---

## Phase 1: Core Infrastructure (M1)

**Goal:** Foundational packages everyone depends on

### 1A: Configuration

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 1.1 | Define config struct | 1h | 0.2 | All fields from 09-config.md |
| 1.2 | Implement env loader | 1h | 1.1 | Loads from env vars |
| 1.3 | Implement file loader | 1h | 1.1 | Loads from YAML file |
| 1.4 | Add validation | 1h | 1.2-1.3 | Invalid config returns clear error |
| 1.5 | Add defaults | 30m | 1.1 | Sensible defaults per spec |
| 1.6 | Unit tests | 1h | 1.4 | 90%+ coverage |

### 1B: Logger

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 1.7 | Define logger interface | 30m | - | Interface with Debug/Info/Warn/Error |
| 1.8 | Implement slog wrapper | 1h | 1.7 | Structured JSON logging |
| 1.9 | Add request context | 30m | 1.8 | request_id, store in log entries |
| 1.10 | Add log levels | 30m | 1.8 | Configurable via env |
| 1.11 | Unit tests | 30m | 1.10 | Verify output format |

### 1C: Errors

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 1.12 | Define error codes | 30m | - | All codes from 16-errors.md |
| 1.13 | Create error struct | 30m | 1.12 | Code, Message, Details fields |
| 1.14 | Add error constructors | 30m | 1.13 | `NewValidationError()`, etc. |
| 1.15 | Add HTTP status mapping | 30m | 1.14 | Each code maps to HTTP status |
| 1.16 | Unit tests | 30m | 1.15 | All codes tested |

### 1D: Event Bus

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 1.17 | Define Bus interface | 1h | - | Publish, Subscribe, Request per 02-events.md |
| 1.18 | Define event types | 2h | 1.17 | All events from 02-events.md |
| 1.19 | Implement memory bus | 2h | 1.17-1.18 | Go channels implementation |
| 1.20 | Add request/response | 2h | 1.19 | Synchronous request pattern |
| 1.21 | Add topic routing | 1h | 1.19 | Wildcard subscriptions |
| 1.22 | Integration tests | 2h | 1.21 | Concurrent pub/sub works |

**Deliverables:**
- [ ] `internal/config/` - complete
- [ ] `internal/pkg/logger/` - complete
- [ ] `internal/pkg/errors/` - complete
- [ ] `internal/bus/` - memory implementation
- [ ] Unit tests for all packages

---

## Phase 2: ML Service (M2)

**Goal:** ONNX-based embedding, sparse encoding, reranking

### 2A: ONNX Runtime

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.1 | Add onnxruntime-go dep | 30m | 0.1 | Compiles on Linux/macOS/Windows |
| 2.2 | Create runtime wrapper | 2h | 2.1 | Initialize with GPU/CPU config |
| 2.3 | Implement session manager | 2h | 2.2 | Load/unload models, memory limits |
| 2.4 | Add tensor utilities | 2h | 2.2 | Float32 tensor creation, padding |
| 2.5 | Add tokenizer wrapper | 3h | - | HuggingFace tokenizers (Go binding) |
| 2.6 | Unit tests | 2h | 2.5 | Mock session tests |

### 2B: Model Download

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.7 | Create model manifest | 1h | - | JSON with model URLs, checksums |
| 2.8 | Implement downloader | 2h | 2.7 | Download with progress, verify checksum |
| 2.9 | Add model discovery | 1h | 2.8 | Find models in ./models/ |
| 2.10 | CLI: `models download` | 1h | 2.8 | Download all required models |
| 2.11 | CLI: `models list` | 30m | 2.9 | Show installed models |
| 2.12 | CLI: `models verify` | 30m | 2.9 | Verify checksums |

### 2C: Embedder

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.13 | Define Embedder interface | 30m | - | `Embed([]string) [][]float32` |
| 2.14 | Implement Jina embedder | 4h | 2.3-2.5 | Load Jina model, produce 1536d vectors |
| 2.15 | Add batching | 2h | 2.14 | Configurable batch size |
| 2.16 | Add caching | 2h | 2.14 | SHA256 → embedding cache |
| 2.17 | Add normalization | 1h | 2.14 | L2 normalize output |
| 2.18 | Integration test | 2h | 2.17 | Real model produces correct dims |

### 2D: Sparse Encoder (SPLADE)

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.19 | Define SparseEncoder interface | 30m | - | `Encode([]string) []SparseVector` |
| 2.20 | Implement SPLADE | 4h | 2.3-2.5 | Load SPLADE model, produce sparse vectors |
| 2.21 | Add vocabulary mapping | 2h | 2.20 | Token ID → term mapping |
| 2.22 | Add top-k pruning | 1h | 2.20 | Keep only top-k terms |
| 2.23 | Integration test | 2h | 2.22 | Verify output structure |

### 2E: Reranker

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.24 | Define Reranker interface | 30m | - | `Rerank(query, docs) []RankedDoc` |
| 2.25 | Implement Jina reranker | 3h | 2.3-2.5 | Cross-encoder scoring |
| 2.26 | Add batching | 1h | 2.25 | Process pairs in batches |
| 2.27 | Add early exit | 1h | 2.25 | Stop if confidence high |
| 2.28 | Integration test | 2h | 2.27 | Scores correlate with relevance |

### 2F: ML Service Assembly

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 2.29 | Create ML service struct | 1h | 2.18,2.23,2.28 | Compose embed + sparse + rerank |
| 2.30 | Add event handlers | 2h | 2.29, 1.19 | Handle embed.request, sparse.request, rerank.request |
| 2.31 | Add HTTP handlers | 2h | 2.29 | /v1/ml/embed, /v1/ml/sparse, /v1/ml/rerank |
| 2.32 | Add health check | 1h | 2.29 | Report model status, memory |
| 2.33 | Standalone server mode | 1h | 2.31 | `rice-search ml serve` works |

**Deliverables:**
- [ ] `internal/onnx/` - complete
- [ ] `internal/ml/` - complete
- [ ] `scripts/download-models.sh`
- [ ] `models/README.md` with download instructions
- [ ] ML service can run standalone
- [ ] All 3 model types working (embed, sparse, rerank)

---

## Phase 3: Search Service (M3)

**Goal:** Qdrant integration, hybrid search, indexing

### 3A: Qdrant Client

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 3.1 | Add qdrant-go dep | 30m | 0.1 | Client compiles |
| 3.2 | Create Qdrant wrapper | 2h | 3.1 | Connection pool, health check |
| 3.3 | Collection management | 2h | 3.2 | Create/delete/list collections |
| 3.4 | Upsert vectors | 2h | 3.2 | Insert/update with payloads |
| 3.5 | Delete vectors | 1h | 3.2 | Delete by ID, by filter |
| 3.6 | Search (dense) | 2h | 3.2 | Vector similarity search |
| 3.7 | Search (sparse) | 2h | 3.2 | Sparse vector search |
| 3.8 | Search (hybrid) | 2h | 3.6-3.7 | RRF fusion per 04-search.md |
| 3.9 | Integration tests | 3h | 3.8 | Requires running Qdrant |

### 3B: Store Management

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 3.10 | Define Store model | 1h | - | Per 03-data-models.md |
| 3.11 | Implement store service | 2h | 3.3, 3.10 | CRUD operations |
| 3.12 | Store metadata persistence | 2h | 3.11 | Save/load store configs |
| 3.13 | Store stats | 1h | 3.11 | Document count, size, etc. |
| 3.14 | Unit tests | 1h | 3.13 | Mocked Qdrant |

### 3C: Chunking

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 3.15 | Define Chunk model | 30m | - | Per 03-data-models.md |
| 3.16 | Language detection | 1h | - | Detect from extension/content |
| 3.17 | Line-based chunker | 2h | 3.16 | Split by lines, respect boundaries |
| 3.18 | Symbol extraction | 3h | 3.16 | Extract function/class names |
| 3.19 | Overlap handling | 1h | 3.17 | Configurable overlap |
| 3.20 | Hash generation | 1h | 3.17 | Deterministic chunk IDs |
| 3.21 | Unit tests | 2h | 3.20 | Various languages tested |

### 3D: Indexing Pipeline

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 3.22 | Define indexing events | 1h | 1.18 | index.request, chunk.created, etc. |
| 3.23 | Create pipeline orchestrator | 3h | 3.22 | File → chunks → embed → store |
| 3.24 | Batch processing | 2h | 3.23 | Process files in batches |
| 3.25 | Progress reporting | 1h | 3.23 | Events for progress tracking |
| 3.26 | Deduplication | 2h | 3.23 | Skip unchanged files |
| 3.27 | Error handling | 2h | 3.23 | Continue on partial failures |
| 3.28 | Integration tests | 3h | 3.27 | Full pipeline tested |

### 3E: Search Service Assembly

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 3.29 | Create Search service struct | 2h | 3.8, 3.14, 3.28 | Compose all search functionality |
| 3.30 | Add event handlers | 2h | 3.29, 1.19 | Handle search.request, index.request |
| 3.31 | Add HTTP handlers | 2h | 3.29 | All endpoints from 07-api.md |
| 3.32 | Add health check | 1h | 3.29 | Report Qdrant status |
| 3.33 | Standalone server mode | 1h | 3.31 | `rice-search search serve` works |

**Deliverables:**
- [ ] `internal/search/` - complete
- [ ] `internal/index/` - complete
- [ ] `internal/store/` - complete
- [ ] Hybrid search working
- [ ] Full indexing pipeline working
- [ ] Search service can run standalone

---

## Phase 4: API & CLI (M4)

**Goal:** HTTP gateway and command-line interface

### 4A: API Service

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 4.1 | Add Echo framework | 30m | 0.1 | Server boots |
| 4.2 | Setup middleware | 2h | 4.1 | Logging, CORS, recovery, request ID |
| 4.3 | Setup routes | 2h | 4.2 | All routes from 07-api.md |
| 4.4 | Request validation | 2h | 4.3 | Validate all request bodies |
| 4.5 | Response formatting | 1h | 4.3 | Standard response wrapper |
| 4.6 | Error handling | 1h | 4.3, 1.15 | Map errors to HTTP responses |
| 4.7 | Health endpoints | 1h | 4.3 | /healthz, /v1/health |
| 4.8 | Metrics endpoint | 2h | 4.3 | /metrics Prometheus format |
| 4.9 | API tests | 3h | 4.8 | Integration tests |

### 4B: CLI Commands

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 4.10 | Add Cobra framework | 30m | 0.1 | Root command works |
| 4.11 | `serve` command | 2h | 4.10, 4.9 | Start monolith |
| 4.12 | `api serve` command | 1h | 4.11 | Start API only |
| 4.13 | `ml serve` command | 1h | 2.33 | Start ML only |
| 4.14 | `search serve` command | 1h | 3.33 | Start Search only |
| 4.15 | `index` command | 2h | 4.10 | Index files from CLI |
| 4.16 | `query` command | 2h | 4.10 | Search from CLI |
| 4.17 | `stores` commands | 1h | 4.10 | list, create, delete |
| 4.18 | `version` command | 30m | 4.10 | Show version info |
| 4.19 | Global flags | 1h | 4.10 | --config, --verbose, --format |
| 4.20 | Shell completion | 1h | 4.10 | bash, zsh, fish |

### 4C: Service Integration

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 4.21 | Wire API → Bus | 2h | 4.9, 1.19 | API publishes events |
| 4.22 | Wire ML handlers | 1h | 4.21, 2.30 | ML subscribes to events |
| 4.23 | Wire Search handlers | 1h | 4.21, 3.30 | Search subscribes to events |
| 4.24 | Graceful shutdown | 2h | 4.21 | Proper drain per 15-shutdown.md |
| 4.25 | Integration tests | 3h | 4.24 | Full request flow tested |

**Deliverables:**
- [ ] `internal/api/` - complete
- [ ] `cmd/rice-search/` - all commands
- [ ] Full monolith working
- [ ] All microservice modes working
- [ ] CLI feature complete

---

## Phase 5: Web UI (M5)

**Goal:** Integrated web interface

### 5A: Templ Templates

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 5.1 | Setup Templ | 1h | 0.1 | `templ generate` works |
| 5.2 | Base layout | 2h | 5.1 | HTML layout with header, nav |
| 5.3 | Search page | 3h | 5.2 | Query input, results display |
| 5.4 | Results component | 2h | 5.3 | Code highlighting, metadata |
| 5.5 | Stores page | 2h | 5.2 | List/manage stores |
| 5.6 | Admin page | 2h | 5.2 | Stats, health, settings |
| 5.7 | Error pages | 1h | 5.2 | 404, 500, etc. |

### 5B: HTMX Integration

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 5.8 | Add HTMX + Alpine.js | 1h | 5.2 | JS loaded |
| 5.9 | Live search | 2h | 5.8, 5.3 | Search as you type |
| 5.10 | Infinite scroll | 2h | 5.8 | Load more results |
| 5.11 | Form handling | 1h | 5.8 | Store creation, settings |
| 5.12 | Toast notifications | 1h | 5.8 | Success/error messages |

### 5C: Web Service

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 5.13 | Create Web service struct | 1h | 5.7 | Serve templates |
| 5.14 | Add HTTP handlers | 2h | 5.13 | Page routes |
| 5.15 | Static file serving | 1h | 5.13 | CSS, JS, images |
| 5.16 | Session handling | 1h | 5.13 | Flash messages |
| 5.17 | Standalone server mode | 1h | 5.14 | `rice-search web serve` |
| 5.18 | Monolith integration | 1h | 5.17 | Mount on /ui path |

**Deliverables:**
- [ ] `internal/web/` - complete
- [ ] `internal/web/templates/` - all templates
- [ ] Static assets (CSS, JS)
- [ ] Web UI functional
- [ ] Integrated in monolith and standalone

---

## Phase 6: Polish & Testing (M6)

**Goal:** Production ready

### 6A: Testing

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 6.1 | Unit test coverage | 4h | All | >80% coverage |
| 6.2 | Integration tests | 4h | All | All services tested together |
| 6.3 | E2E tests | 4h | All | Full user flows |
| 6.4 | Load tests | 2h | All | Measure throughput |
| 6.5 | Benchmark tests | 2h | All | Measure latency |

### 6B: Documentation

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 6.6 | Update README | 2h | All | Quick start works |
| 6.7 | API documentation | 2h | 4.9 | OpenAPI spec generated |
| 6.8 | CLI documentation | 1h | 4.20 | All commands documented |
| 6.9 | Deployment guide | 2h | All | Docker, K8s instructions |
| 6.10 | Migration guide | 2h | All | Step-by-step migration |

### 6C: Operations

| ID | Task | Est | Deps | Acceptance |
|----|------|-----|------|------------|
| 6.11 | Prometheus metrics | 2h | 4.8 | All metrics exposed |
| 6.12 | Grafana dashboard | 2h | 6.11 | Pre-built dashboard JSON |
| 6.13 | Alert rules | 1h | 6.11 | Prometheus alert config |
| 6.14 | Docker image optimization | 2h | 0.7 | <100MB image size |
| 6.15 | K8s manifests | 2h | 0.8 | Deployable to K8s |

**Deliverables:**
- [ ] Test coverage >80%
- [ ] All docs updated
- [ ] Observability complete
- [ ] Production-ready Docker image
- [ ] K8s deployment configs

---

## Dependency Graph

```
Phase 0 (Setup)
    │
    ▼
Phase 1 (Core) ────────────────────────────────────┐
    │                                               │
    ├──────────────────┬───────────────────┐       │
    ▼                  ▼                   ▼       │
Phase 2 (ML)     Phase 3 (Search)         │       │
    │                  │                   │       │
    └─────────┬────────┘                   │       │
              ▼                            │       │
        Phase 4 (API/CLI) ◄────────────────┘       │
              │                                     │
              ├──────────────────┐                 │
              ▼                  ▼                 │
        Phase 5 (Web)      Phase 6 (Polish) ◄─────┘
```

---

## Critical Path

The minimum path to a working system:

```
0.1 → 0.2 → 1.1 → 1.17 → 1.19 → 2.1 → 2.14 → 2.20 → 2.25 → 3.1 → 3.8 → 3.23 → 4.1 → 4.11
```

**Estimated critical path duration: 3 weeks**

This gets you a working monolith with:
- Config loading
- In-memory event bus
- Dense embedding
- Sparse encoding
- Reranking
- Qdrant hybrid search
- Indexing pipeline
- HTTP API
- `serve` command

---

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| ONNX Go bindings issues | High | Test early (2.1-2.3), have backup plan |
| Model size (downloads) | Medium | Pre-download in CI, cache in Docker |
| Qdrant API changes | Low | Pin version, integration tests |
| Tokenizer complexity | Medium | Use existing Go tokenizer libs |
| Windows ONNX support | Medium | Test on Windows early |

---

## Definition of Done

Each phase is "done" when:

1. All tasks completed
2. Unit tests pass
3. Integration tests pass (where applicable)
4. Code reviewed (if team)
5. Documentation updated
6. No critical linter errors

---

## Next Steps

1. **Start with Phase 0** - Get the project scaffold ready
2. **Spike on ONNX** - Validate onnxruntime-go works for your models early
3. **Get Qdrant running locally** - Ensure dev environment is ready
4. **Follow the task IDs** - Each task is designed to be atomic

Run:
```bash
cd go-search
go mod init github.com/yourusername/rice-search
mkdir -p cmd/rice-search internal/{api,ml,search,index,store,bus,onnx,cache,config,web,pkg/{hash,logger,errors}}
```
