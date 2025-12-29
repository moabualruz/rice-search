# Architecture Overview

## Problem Statement

Current Rice Search (NestJS + Milvus + Infinity + Tantivy):
- 6+ containers, 12GB+ memory
- Bun/Windows socket issues with Infinity
- Multiple languages (TypeScript, Rust, Python)
- Complex operational overhead

## Design Goals

1. **Simplicity** - Fewer moving parts
2. **Single language** - Go only
3. **Flexibility** - Monolith or microservices
4. **Decoupling** - Event-driven communication
5. **Completeness** - HTTP endpoint for everything

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            RICE SEARCH - GO                                 │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│                              ┌─────────────┐                                │
│                              │   Clients   │                                │
│                              │ ricegrep/UI │                                │
│                              └──────┬──────┘                                │
│                                     │ HTTP                                  │
│                                     ▼                                       │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                          HTTP LAYER                                   │ │
│  │                                                                       │ │
│  │   Every functionality exposed as HTTP endpoint                        │ │
│  │   /v1/search, /v1/ml/embed, /v1/stores, /healthz, etc.               │ │
│  │                                                                       │ │
│  └───────────────────────────────────┬───────────────────────────────────┘ │
│                                      │ publishes                            │
│                                      ▼                                      │
│  ┌───────────────────────────────────────────────────────────────────────┐ │
│  │                          EVENT BUS                                    │ │
│  │                                                                       │ │
│  │   Single process: Go channels (zero latency)                          │ │
│  │   Distributed: Kafka / NATS / Redis (configurable)                    │ │
│  │                                                                       │ │
│  └───┬───────────────┬───────────────┬───────────────┬───────────────────┘ │
│      │               │               │               │                      │
│      ▼               ▼               ▼               ▼                      │
│  ┌───────┐      ┌───────┐      ┌───────┐      ┌───────┐                    │
│  │  API  │      │  ML   │      │Search │      │  Web  │                    │
│  │Service│      │Service│      │Service│      │Service│                    │
│  └───────┘      └───┬───┘      └───┬───┘      └───────┘                    │
│                     │              │                                        │
│                     ▼              ▼                                        │
│               ┌──────────┐   ┌──────────┐                                  │
│               │   ONNX   │   │  Qdrant  │                                  │
│               │ Runtime  │   │  :6333   │                                  │
│               └──────────┘   └──────────┘                                  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Deployment Modes

### Mode 1: Monolith (Default)

Single binary, all services in one process.

```
┌─────────────────────────────────────┐
│         rice-search binary          │
│                                     │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐  │
│  │ API │ │ ML  │ │ Src │ │ Web │  │
│  └──┬──┘ └──┬──┘ └──┬──┘ └──┬──┘  │
│     └───────┴───────┴───────┘      │
│              │                      │
│        Go Channels                  │
│        (zero latency)               │
└─────────────────────────────────────┘
              │
        ┌─────▼─────┐
        │  Qdrant   │
        └───────────┘

Containers: 2 (rice-search, qdrant)
Memory: ~4GB
```

### Mode 2: Microservices

Separate binaries, distributed event bus.

```
┌───────┐  ┌───────┐  ┌───────┐  ┌───────┐
│  API  │  │  ML   │  │Search │  │  Web  │
│ :8080 │  │ :8081 │  │ :8082 │  │ :3000 │
└───┬───┘  └───┬───┘  └───┬───┘  └───┬───┘
    └──────────┴──────────┴──────────┘
                    │
              Kafka / NATS
                    │
              ┌─────▼─────┐
              │  Qdrant   │
              └───────────┘

Containers: 4-5 (api, ml, search, web, qdrant)
Memory: ~5GB
```

### Mode 3: Hybrid

Some services local, some remote.

```bash
# GPU server runs ML
./rice-search ml serve --port 8081 --device cuda

# CPU server runs everything else
./rice-search serve --ml-url http://gpu-server:8081
```

---

## Services

### API Service

| Aspect | Detail |
|--------|--------|
| **Purpose** | HTTP gateway, request validation, event publishing |
| **Port** | 8080 (default) |
| **Framework** | Echo v4 |
| **Responsibilities** | Route HTTP → Events → HTTP response |

### ML Service

| Aspect | Detail |
|--------|--------|
| **Purpose** | Machine learning inference |
| **Port** | 8081 (standalone) |
| **Runtime** | ONNX Runtime (Go bindings) |
| **Models** | SPLADE (sparse), Jina Embed (dense), Jina Rerank |
| **Responsibilities** | Embed, sparse encode, rerank |

### Search Service

| Aspect | Detail |
|--------|--------|
| **Purpose** | Search operations against Qdrant |
| **Port** | 8082 (standalone) |
| **Backend** | Qdrant |
| **Responsibilities** | Hybrid search, indexing, store management |

### Web Service

| Aspect | Detail |
|--------|--------|
| **Purpose** | Web UI |
| **Port** | 3000 (standalone) or 8080/ (monolith) |
| **Stack** | Templ + HTMX + Alpine.js |
| **Responsibilities** | SSR pages, search UI, admin UI |

---

## Technology Stack

| Layer | Technology | Rationale |
|-------|------------|-----------|
| Language | Go 1.23+ | Single language, native concurrency |
| HTTP | Echo v4 | Fast, minimal, middleware support |
| Templates | Templ | Type-safe, compiled, Go-native |
| Frontend | HTMX + Alpine.js | No build step, SSR-first |
| Vector DB | Qdrant | Simple, fast, native hybrid search |
| ML Runtime | ONNX Runtime | Native Go, GPU support |
| Event Bus | Go channels / Kafka / NATS | Pluggable, configurable |

---

## Communication Principles

### Rule 1: HTTP for Everything

Every functionality has an HTTP endpoint, even if primarily used internally.

**Why:**
- External clients can call anything
- Easy testing and debugging
- No hidden functionality

### Rule 2: Events Internally

Services never call each other directly. Always via event bus.

**Why:**
- Loose coupling
- Easy to add new services
- Easy to scale independently
- External systems can subscribe

### Rule 3: Pluggable Bus

Event bus is interface-based. Configure any implementation.

**Why:**
- Go channels for single process (fastest)
- Kafka/NATS/Redis for distributed
- Easy to switch or add new implementations

---

## Comparison with Current System

| Aspect | Current (NestJS + Milvus) | Go Edition |
|--------|---------------------------|------------|
| Languages | TypeScript, Rust, Python | Go |
| Containers | 6+ | 2-5 |
| Memory | 12GB+ | 4-5GB |
| BM25 | Tantivy subprocess | SPLADE vectors |
| Vectors | Milvus (4 containers) | Qdrant (1 container) |
| ML | Infinity (Python HTTP) | ONNX Runtime (native) |
| Internal comm | Direct HTTP | Event-driven |
| Deployment | Complex | Single binary possible |
