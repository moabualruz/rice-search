# Directory Structure

## Project Layout

```
go-search/
├── cmd/
│   └── rice-search/
│       └── main.go                 # CLI entry point
│
├── internal/
│   ├── api/                        # API service
│   │   ├── service.go              # Service struct, New()
│   │   ├── handlers.go             # HTTP handlers
│   │   ├── routes.go               # Route registration
│   │   ├── middleware.go           # Auth, logging, etc.
│   │   └── server.go               # HTTP server lifecycle
│   │
│   ├── ml/                         # ML service
│   │   ├── service.go              # Service interface + struct
│   │   ├── embedder.go             # Dense embedding
│   │   ├── sparse.go               # SPLADE encoding
│   │   ├── reranker.go             # Reranking
│   │   ├── handlers.go             # HTTP handlers (standalone)
│   │   └── server.go               # HTTP server (standalone)
│   │
│   ├── search/                     # Search service
│   │   ├── service.go              # Service interface + struct
│   │   ├── hybrid.go               # Hybrid search + RRF
│   │   ├── qdrant.go               # Qdrant client
│   │   ├── handlers.go             # HTTP handlers (standalone)
│   │   └── server.go               # HTTP server (standalone)
│   │
│   ├── index/                      # Indexing logic
│   │   ├── service.go              # Indexing service
│   │   ├── chunker.go              # Code chunking
│   │   ├── symbols.go              # Symbol extraction
│   │   └── pipeline.go             # Full indexing pipeline
│   │
│   ├── store/                      # Store management
│   │   ├── service.go              # Store CRUD
│   │   └── metadata.go             # Store metadata
│   │
│   ├── web/                        # Web UI service
│   │   ├── service.go              # Service struct
│   │   ├── handlers.go             # Page handlers
│   │   ├── server.go               # HTTP server
│   │   └── templates/              # Templ templates
│   │       ├── layout.templ
│   │       ├── search.templ
│   │       ├── results.templ
│   │       ├── stores.templ
│   │       └── admin.templ
│   │
│   ├── bus/                        # Event bus
│   │   ├── bus.go                  # Bus interface
│   │   ├── memory.go               # In-memory (Go channels)
│   │   ├── kafka.go                # Kafka implementation
│   │   ├── nats.go                 # NATS implementation
│   │   ├── redis.go                # Redis Streams
│   │   └── events.go               # Event type definitions
│   │
│   ├── onnx/                       # ONNX runtime
│   │   ├── runtime.go              # Runtime initialization
│   │   ├── session.go              # Model sessions
│   │   ├── tensor.go               # Tensor helpers
│   │   └── tokenizer.go            # Tokenizer wrapper
│   │
│   ├── cache/                      # Caching
│   │   ├── cache.go                # Cache interface
│   │   ├── memory.go               # In-memory LRU
│   │   └── redis.go                # Redis cache
│   │
│   ├── config/                     # Configuration
│   │   ├── config.go               # Config struct
│   │   ├── loader.go               # Load from env/file
│   │   └── validate.go             # Validation
│   │
│   └── pkg/                        # Shared utilities
│       ├── hash/                   # Hashing utilities
│       │   └── hash.go
│       ├── logger/                 # Structured logging
│       │   └── logger.go
│       └── errors/                 # Error types
│           └── errors.go
│
├── models/                         # ONNX model files
│   ├── .gitkeep
│   └── README.md                   # Download instructions
│
├── docs/                           # Documentation
│   ├── 01-architecture.md
│   ├── 02-events.md
│   ├── ...
│   └── 20-migration.md
│
├── scripts/                        # Build/deploy scripts
│   ├── download-models.sh
│   ├── build.sh
│   └── docker-build.sh
│
├── deployments/                    # Deployment configs
│   ├── docker/
│   │   ├── Dockerfile
│   │   └── Dockerfile.dev
│   ├── docker-compose.yml
│   ├── docker-compose.dev.yml
│   └── kubernetes/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── configmap.yaml
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Package Dependencies

```
cmd/rice-search
    └── internal/api
    └── internal/ml
    └── internal/search
    └── internal/web
    └── internal/config

internal/api
    └── internal/bus
    └── internal/pkg/logger
    └── internal/pkg/errors

internal/ml
    └── internal/bus
    └── internal/onnx
    └── internal/cache
    └── internal/pkg/logger

internal/search
    └── internal/bus
    └── internal/ml (events only)
    └── internal/index
    └── internal/store
    └── internal/pkg/logger

internal/index
    └── internal/ml (events only)
    └── internal/pkg/hash

internal/bus
    └── (no internal deps)

internal/onnx
    └── (no internal deps, external: onnxruntime_go)

internal/cache
    └── (no internal deps)
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

## Key Files

### cmd/rice-search/main.go

```go
package main

import (
    "github.com/spf13/cobra"
    // subcommands
)

func main() {
    rootCmd := &cobra.Command{Use: "rice-search"}
    
    rootCmd.AddCommand(
        serveCmd(),    // monolith
        apiCmd(),      // api service
        mlCmd(),       // ml service
        searchCmd(),   // search service
        webCmd(),      // web service
        modelsCmd(),   // model management
        indexCmd(),    // cli indexing
        queryCmd(),    // cli search
        storesCmd(),   // store management
        versionCmd(),  // version info
    )
    
    rootCmd.Execute()
}
```

### internal/bus/bus.go

```go
package bus

type Bus interface {
    Publish(ctx context.Context, topic string, event any) error
    Subscribe(ctx context.Context, topic string, handler Handler) error
    Request(ctx context.Context, req RequestEvent) (ResponseEvent, error)
    Close() error
}

type Handler func(ctx context.Context, event any) error
```

### internal/ml/service.go

```go
package ml

type Service interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    SparseEncode(ctx context.Context, texts []string) ([]SparseVector, error)
    Rerank(ctx context.Context, query string, docs []Document, topK int) ([]RankedDoc, error)
    Health() HealthStatus
}
```

### internal/search/service.go

```go
package search

type Service interface {
    Search(ctx context.Context, req SearchRequest) (*SearchResponse, error)
    Index(ctx context.Context, req IndexRequest) (*IndexResponse, error)
    Delete(ctx context.Context, req DeleteRequest) (*DeleteResponse, error)
    CreateStore(ctx context.Context, name string, config StoreConfig) error
    DeleteStore(ctx context.Context, name string) error
    ListStores(ctx context.Context) ([]Store, error)
    Health() HealthStatus
}
```

---

## Build Tags

| Tag | Description |
|-----|-------------|
| `cuda` | Enable CUDA support |
| `tensorrt` | Enable TensorRT |
| `kafka` | Include Kafka bus |
| `nats` | Include NATS bus |

```bash
# Build with CUDA + Kafka
go build -tags "cuda,kafka" ./cmd/rice-search
```

---

## Generated Files

| File | Generator | Description |
|------|-----------|-------------|
| `*_templ.go` | templ | Template code |
| `*.pb.go` | protoc | Protobuf (if used) |

Generated files should be committed for reproducible builds.
