# Directory Structure

## Project Layout

```
go-search/
├── cmd/
│   ├── rice-search/
│   │   └── main.go                 # CLI entry point
│   └── rice-search-server/
│       └── main.go                 # Server entry point (primary)
│
├── internal/
│   ├── bus/                        # Event bus
│   │   ├── bus.go                  # Bus interface + Drainable interface
│   │   ├── factory.go              # Bus factory
│   │   ├── instrumented.go         # Metrics-instrumented wrapper
│   │   ├── kafka.go                # Kafka implementation
│   │   ├── logged_bus.go           # Event logging wrapper
│   │   ├── memory.go               # In-memory (Go channels) + drain support
│   │   └── persistence.go          # Event persistence
│   │
│   ├── config/                     # Configuration
│   │   └── config.go               # Config struct + loader
│   │
│   ├── connection/                 # Connection tracking (UNIQUE feature)
│   │   ├── audit.go                # Audit logging
│   │   ├── events.go               # Connection events
│   │   ├── models.go               # Connection/Device models
│   │   ├── monitoring.go           # Anomaly detection + alerts
│   │   ├── service.go              # Connection service
│   │   └── storage.go              # Persistence
│   │
│   ├── grpcclient/                 # gRPC client
│   │   └── client.go               # Client wrapper
│   │
│   ├── grpcserver/                 # gRPC server
│   │   └── server.go               # Server implementation
│   │
│   ├── index/                      # Indexing pipeline
│   │   ├── batch.go                # Batch processing
│   │   ├── chunker.go              # Code chunking (47 languages)
│   │   ├── document.go             # Document model
│   │   ├── pipeline.go             # Full indexing pipeline
│   │   ├── symbols.go              # Symbol extraction
│   │   └── tracker.go              # File tracking
│   │
│   ├── metrics/                    # Prometheus metrics
│   │   ├── collector.go            # Metric collectors
│   │   ├── events.go               # Event subscriber (13 topics)
│   │   ├── handler.go              # HTTP handlers
│   │   ├── history.go              # Time-series history
│   │   ├── http.go                 # HTTP metric middleware
│   │   ├── metrics.go              # Core metrics service
│   │   ├── presets.go              # Dashboard presets
│   │   ├── prometheus.go           # Prometheus registry
│   │   ├── redis_storage.go        # Redis persistence
│   │   └── types.go                # Type definitions
│   │
│   ├── ml/                         # ML service
│   │   ├── cache.go                # Embedding cache + flush support
│   │   ├── embedder.go             # Dense embedding
│   │   ├── handlers.go             # Event handlers
│   │   ├── models.go               # Model loading
│   │   ├── reranker.go             # Single-pass reranking
│   │   ├── service.go              # Service interface + struct
│   │   └── sparse.go               # SPLADE encoding
│   │
│   ├── models/                     # Model registry
│   │   ├── defaults.go             # Default model configs
│   │   ├── mapper_service.go       # Model I/O mappings
│   │   ├── registry.go             # Model registry
│   │   ├── storage.go              # Model persistence
│   │   └── types.go                # Type definitions
│   │
│   ├── onnx/                       # ONNX runtime
│   │   ├── runtime.go              # Runtime initialization
│   │   ├── runtime_stub.go         # Stub for non-ONNX builds
│   │   ├── session.go              # Model sessions
│   │   ├── tensor.go               # Tensor helpers
│   │   ├── tokenizer.go            # Tokenizer wrapper
│   │   └── tokenizer_stub.go       # Stub for non-ONNX builds
│   │
│   ├── pkg/                        # Shared utilities
│   │   ├── context/                # Context utilities
│   │   │   └── connection.go       # Connection context
│   │   ├── errors/                 # Error types
│   │   │   └── errors.go           # Custom error constructors
│   │   ├── hash/                   # Hashing utilities
│   │   │   └── hash.go             # Content hashing
│   │   ├── logger/                 # Structured logging
│   │   │   └── logger.go           # Logger wrapper
│   │   └── security/               # Security utilities
│   │       ├── security.go         # Sanitization
│   │       └── validation.go       # Input validation
│   │
│   ├── qdrant/                     # Qdrant client
│   │   ├── client.go               # Client wrapper
│   │   ├── collection.go           # Collection management
│   │   ├── point.go                # Point operations
│   │   ├── search.go               # Search operations
│   │   └── types.go                # Type definitions
│   │
│   ├── query/                      # Query understanding
│   │   ├── code_terms.go           # Code-specific term extraction
│   │   ├── keyword_extractor.go    # Keyword extraction
│   │   ├── model_understanding.go  # ML-based understanding
│   │   ├── service.go              # Query service
│   │   ├── types.go                # Type definitions
│   │   └── understanding.go        # Intent/difficulty detection
│   │
│   ├── search/                     # Search service
│   │   ├── fusion/                 # Score fusion
│   │   │   └── rrf.go              # Reciprocal Rank Fusion
│   │   ├── postrank/               # Post-ranking pipeline
│   │   │   ├── aggregation.go      # File aggregation
│   │   │   ├── dedup.go            # Semantic deduplication
│   │   │   ├── diversity.go        # MMR diversity
│   │   │   └── pipeline.go         # Pipeline orchestration
│   │   ├── reranker/               # Multi-pass reranking
│   │   │   ├── adapter.go          # Service adapter
│   │   │   ├── multi_pass.go       # Two-pass with early exit
│   │   │   └── types.go            # Type definitions
│   │   ├── handlers.go             # HTTP handlers
│   │   ├── health.go               # Basic health
│   │   ├── health_detailed.go      # Detailed health checks
│   │   └── service.go              # Search service
│   │
│   ├── server/                     # HTTP server
│   │   ├── index_handler.go        # Index handlers
│   │   ├── response.go             # Response helpers
│   │   ├── server.go               # Server lifecycle
│   │   └── store_handler.go        # Store handlers
│   │
│   ├── settings/                   # Runtime settings
│   │   ├── audit.go                # Settings audit logging
│   │   └── service.go              # Settings service (80+ settings)
│   │
│   ├── store/                      # Store management
│   │   ├── models.go               # Store model
│   │   ├── service.go              # Store CRUD
│   │   └── storage.go              # Store persistence
│   │
│   └── web/                        # Web UI (templ + HTMX)
│       ├── admin.templ             # Admin base
│       ├── admin_connections.templ # Connection management
│       ├── admin_mappers.templ     # Model mappers
│       ├── admin_models.templ      # Model management
│       ├── admin_settings.templ    # Settings (80+ options)
│       ├── components.templ        # Reusable components
│       ├── dashboard.templ         # Dashboard
│       ├── file_detail.templ       # File detail view
│       ├── files.templ             # File browser
│       ├── handlers.go             # HTTP handlers (48 routes)
│       ├── layout.templ            # Base layout
│       ├── mapper_editor.templ     # Mapper editor
│       ├── search.templ            # Search page
│       ├── stats.templ             # Statistics/metrics
│       ├── store_detail.templ      # Store detail view
│       └── stores.templ            # Store management
│
├── api/proto/                      # Protobuf definitions
│   └── search.proto                # gRPC service definitions
│
├── models/                         # ONNX model files
│   └── README.md                   # Download instructions
│
├── docs/                           # Documentation
│   ├── 01-architecture.md
│   ├── 02-events.md
│   ├── ...
│   └── 21-default-connection-scoping.md
│
├── scripts/                        # Build/deploy scripts
│   └── download-models.sh          # Model download script
│
├── deployments/                    # Deployment configs
│   ├── docker/
│   │   └── Dockerfile
│   ├── docker-compose.yml
│   └── docker-compose.dev.yml
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Package Dependencies

```
cmd/rice-search-server
    └── internal/bus
    └── internal/config
    └── internal/connection
    └── internal/grpcserver
    └── internal/index
    └── internal/metrics
    └── internal/ml
    └── internal/models
    └── internal/qdrant
    └── internal/query
    └── internal/search
    └── internal/settings
    └── internal/store
    └── internal/web

internal/search
    └── internal/bus
    └── internal/ml (events only)
    └── internal/metrics
    └── internal/qdrant
    └── internal/query
    └── internal/search/fusion
    └── internal/search/postrank
    └── internal/search/reranker
    └── internal/pkg/logger

internal/ml
    └── internal/bus
    └── internal/onnx
    └── internal/metrics
    └── internal/pkg/logger

internal/index
    └── internal/bus
    └── internal/ml (events only)
    └── internal/qdrant
    └── internal/pkg/hash
    └── internal/pkg/logger

internal/connection
    └── internal/bus
    └── internal/pkg/logger
    └── internal/pkg/context

internal/metrics
    └── internal/bus
    └── internal/pkg/logger

internal/bus
    └── (no internal deps)

internal/onnx
    └── (no internal deps, external: onnxruntime_go)
```

---

## File Naming Conventions

| Type | Convention | Example |
|------|------------|---------|
| Package | lowercase | `internal/ml` |
| File | snake_case | `hybrid_search.go` |
| Test file | `*_test.go` | `hybrid_search_test.go` |
| Interface | PascalCase, -er suffix | `Embedder`, `Searcher` |
| Struct | PascalCase | `SearchService` |
| Method | PascalCase | `Search()` |
| Variable | camelCase | `topK`, `sparseWeight` |
| Constant | PascalCase | `DefaultTopK` |
| Env var | SCREAMING_SNAKE | `QDRANT_URL` |

---

## Key Interfaces

### internal/bus/bus.go

```go
package bus

type Bus interface {
    Publish(ctx context.Context, topic string, event Event) error
    Subscribe(ctx context.Context, topic string, handler Handler) error
    Request(ctx context.Context, topic string, event Event, timeout time.Duration) (Event, error)
    Close() error
}

type Drainable interface {
    DrainTimeout(timeout time.Duration) bool
    InFlightCount() int64
}

type Handler func(ctx context.Context, event Event) error
```

### internal/ml/service.go

```go
package ml

type Service interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    SparseEncode(ctx context.Context, texts []string) ([]SparseVector, error)
    Rerank(ctx context.Context, query string, docs []string, topK int) ([]RankedResult, error)
    Health() HealthStatus
    Close() error
}
```

### internal/search/service.go

```go
package search

type SearchService interface {
    Search(ctx context.Context, req Request) (*Response, error)
    Health() HealthStatus
}
```

### internal/store/service.go

```go
package store

type StoreService interface {
    CreateStore(ctx context.Context, store *Store) error
    GetStore(ctx context.Context, name string) (*Store, error)
    ListStores(ctx context.Context) ([]*Store, error)
    DeleteStore(ctx context.Context, name string) error
    GetStoreStats(ctx context.Context, name string) (*StoreStats, error)
}
```

---

## Build Tags

| Tag | Description |
|-----|-------------|
| `cuda` | Enable CUDA support |
| `tensorrt` | Enable TensorRT |
| `kafka` | Include Kafka bus |

```bash
# Build with CUDA + Kafka
go build -tags "cuda,kafka" ./cmd/rice-search-server
```

---

## Generated Files

| File | Generator | Description |
|------|-----------|-------------|
| `*_templ.go` | templ | Template code |
| `*.pb.go` | protoc | Protobuf (if used) |

Generated files should be committed for reproducible builds.

---

## Notes

1. **No `internal/api/` package** - Server routes are in `cmd/rice-search-server/main.go` and `internal/server/`
2. **No `internal/cache/` package** - Caching is embedded in `internal/ml/cache.go`
3. **`postrank/` is under `search/`** - Not a top-level internal package
4. **`fusion/` and `reranker/` are under `search/`** - Search-specific subpackages
