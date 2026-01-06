# Architecture

Complete system design and architecture overview for Rice Search.

## Table of Contents

- [System Overview](#system-overview)
- [Service Architecture](#service-architecture)
- [Component Diagram](#component-diagram)
- [Data Flow](#data-flow)
- [Triple Retrieval System](#triple-retrieval-system)
- [Storage Architecture](#storage-architecture)
- [Search Pipeline](#search-pipeline)
- [Indexing Pipeline](#indexing-pipeline)
- [Technology Stack](#technology-stack)
- [Design Decisions](#design-decisions)
- [Scalability Considerations](#scalability-considerations)

---

## System Overview

Rice Search is a **hybrid search platform** that combines:
- **Lexical search** (BM25 via Tantivy/Rust)
- **Semantic search** (dense embeddings via Ollama)
- **Neural sparse search** (SPLADE and BM42 sparse vectors)
- **Intelligent retrieval** (adaptive query routing)
- **Multi-pass reranking** (cross-encoder + optional LLM)

**Key Characteristics:**
- ✅ Fully local (no external API dependencies)
- ✅ Self-hosted (all services containerized)
- ✅ Code-aware (AST parsing for 7+ languages)
- ✅ Hybrid retrieval (combines multiple search methods)
- ✅ Adaptive (query analysis determines optimal strategy)

---

## Service Architecture

### Service Layer Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ CLIENT LAYER                                                │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────┐   │
│  │   Web UI     │   │     CLI      │   │  MCP Client  │   │
│  │  (Next.js)   │   │ (ricesearch) │   │ (Claude/AI)  │   │
│  │  Port: 3000  │   │   (Python)   │   │   (stdio)    │   │
│  └──────┬───────┘   └──────┬───────┘   └──────┬───────┘   │
└─────────┼───────────────────┼───────────────────┼───────────┘
          │                   │                   │
          └───────────────────┴───────────────────┘
                              │
┌─────────────────────────────┼───────────────────────────────┐
│ APPLICATION LAYER           │                               │
│  ┌──────────────────────────▼──────────────────────────┐   │
│  │         Backend API (FastAPI)                       │   │
│  │         Port: 8000                                  │   │
│  │                                                     │   │
│  │  ├── Search Router    (hybrid search)              │   │
│  │  ├── Ingest Router    (file upload)                │   │
│  │  ├── Files Router     (list/read files)            │   │
│  │  ├── Settings Router  (runtime config)             │   │
│  │  ├── Metrics Router   (prometheus)                 │   │
│  │  └── Health Router    (health checks)              │   │
│  └─────────────────────────────────────────────────────┘   │
│                              │                              │
│  ┌──────────────────────────▼──────────────────────────┐   │
│  │       Celery Worker (Async Tasks)                   │   │
│  │       - File ingestion                              │   │
│  │       - Batch indexing                              │   │
│  │       - AST parsing                                 │   │
│  └─────────────────────────────────────────────────────┘   │
└──────────────────────────────┬──────────────────────────────┘
                               │
┌──────────────────────────────┼──────────────────────────────┐
│ INFRASTRUCTURE LAYER         │                              │
│  ┌────────────┐   ┌──────────▼─────┐   ┌──────────────┐   │
│  │   Qdrant   │   │     Redis      │   │    MinIO     │   │
│  │  (Vectors) │   │ (Queue/Cache)  │   │  (Storage)   │   │
│  │  Port:6333 │   │   Port: 6379   │   │  Port: 9000  │   │
│  └────────────┘   └────────────────┘   └──────────────┘   │
└───────────────────────────────────────────────────────────────┘
                               │
┌──────────────────────────────┼──────────────────────────────┐
│ INFERENCE LAYER              │                              │
│  ┌────────────────────────────▼─────────────────────────┐   │
│  │         Ollama (LLM & Embedding Service)            │   │
│  │         Port: 11434                                  │   │
│  │                                                      │   │
│  │  ├── qwen3-embedding:4b  (2560 dims)                │   │
│  │  └── qwen2.5-coder:1.5b  (code LLM)                 │   │
│  └──────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         Tantivy (BM25 Search Service)               │   │
│  │         Port: 3002                                   │   │
│  │         - Rust-based lexical search                 │   │
│  │         - Fast BM25 ranking                          │   │
│  └──────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────┘
```

### Service Communication

```
Frontend (3000)
  └─HTTP→ Backend API (8000)
          ├─gRPC→ Qdrant (6333)
          ├─TCP→  Redis (6379)
          ├─HTTP→ Ollama (11434)
          ├─HTTP→ Tantivy (3002)
          └─S3→   MinIO (9000)

CLI (ricesearch)
  └─HTTP→ Backend API (8000)

Celery Worker
  ├─TCP→  Redis (6379) [queue/results]
  ├─gRPC→ Qdrant (6333) [vector upsert]
  ├─HTTP→ Ollama (11434) [embeddings]
  ├─HTTP→ Tantivy (3002) [BM25 index]
  └─S3→   MinIO (9000) [file storage]
```

---

## Component Diagram

### Backend Components

```
backend/src/
├── api/                         # REST API Layer
│   └── v1/endpoints/
│       ├── search.py            # Search & RAG endpoints
│       ├── ingest.py            # File upload
│       ├── files.py             # File listing
│       ├── settings.py          # Runtime config
│       └── metrics.py           # Prometheus metrics
│
├── services/                    # Business Logic Layer
│   ├── search/
│   │   ├── retriever.py         # Triple retrieval coordinator
│   │   └── query_analyzer.py   # Intent classification
│   ├── ingestion/
│   │   ├── indexer.py           # Multi-representation indexing
│   │   ├── chunker.py           # Text chunking
│   │   ├── parser.py            # File parsing
│   │   └── ast_parser.py        # AST-aware parsing (Tree-sitter)
│   ├── retrieval/
│   │   ├── splade_encoder.py    # SPLADE sparse vectors
│   │   ├── bm42_encoder.py      # BM42 hybrid vectors
│   │   └── tantivy_client.py    # BM25 client
│   ├── rag/
│   │   └── engine.py            # RAG query engine
│   ├── inference/
│   │   └── ollama_client.py     # Ollama LLM/embedding client
│   └── mcp/
│       ├── server.py            # MCP server (stdio/tcp)
│       └── tools.py             # MCP tool handlers
│
├── tasks/                       # Async Task Layer
│   └── ingestion.py             # Celery tasks
│
├── worker/                      # Worker Process
│   └── start_worker.py          # Celery worker startup
│
├── core/                        # Core Infrastructure
│   ├── config.py                # Settings wrapper
│   ├── settings_manager.py      # Centralized config
│   ├── security.py              # Auth & CORS
│   └── telemetry.py             # Logging & monitoring
│
├── db/                          # Database Clients
│   ├── qdrant.py                # Qdrant vector DB
│   ├── redis.py                 # Redis cache/queue
│   └── minio.py                 # MinIO object storage
│
└── cli/ricesearch/              # CLI Tool
    ├── main.py                  # CLI commands
    ├── watch.py                 # File watcher
    └── config.py                # CLI config
```

---

## Data Flow

### Indexing Flow

```
1. User uploads file or CLI watches directory
   └─HTTP POST /api/v1/ingest/file
      └─ File saved to /tmp/ingest

2. Celery task queued
   └─ Task: ingest_file_task(file_path, org_id)

3. Worker processes file
   ├─ Parse file (AST if code, else text)
   ├─ Chunk content (1000 chars, 200 overlap)
   ├─ Generate embeddings (Ollama: qwen3-embedding)
   ├─ Generate SPLADE vectors (neural sparse)
   ├─ Generate BM42 vectors (hybrid sparse)
   └─ Index in Tantivy (BM25)

4. Store in databases
   ├─ Qdrant: dense + sparse vectors + metadata
   ├─ Tantivy: BM25 lexical index
   └─ MinIO: original file (optional)

5. Return success
   └─ Task status: completed
```

### Search Flow (Standard)

```
1. User submits query
   └─POST /api/v1/search/query
      {"query": "authentication", "limit": 10}

2. Query Analysis (optional)
   ├─ Classify intent (navigational/factual/exploratory)
   ├─ Determine optimal strategy
   └─ Select retrievers (bm25/splade/bm42)

3. Parallel Retrieval
   ├─ BM25 (Tantivy)       → Lexical matches
   ├─ SPLADE (Qdrant)      → Sparse neural matches
   └─ BM42 (Qdrant)        → Hybrid matches

4. Fusion
   └─ Reciprocal Rank Fusion (RRF, k=60)
      → Combined ranked list

5. Reranking (optional)
   ├─ Cross-encoder reranking (top 50)
   └─ LLM reranking (complex queries only)

6. Deduplication
   └─ Keep highest-scoring chunk per file

7. Return results
   └─ JSON with scores, paths, snippets
```

### Search Flow (RAG Mode)

```
1. User submits RAG query
   └─POST /api/v1/search/query
      {"query": "How does auth work?", "mode": "rag"}

2. Retrieval Phase
   └─ Same as standard search (steps 2-6)

3. Context Preparation
   ├─ Extract top K results (default: 10)
   └─ Format as context chunks

4. LLM Generation
   ├─ Ollama: qwen2.5-coder:1.5b
   ├─ System prompt: "Answer based on context..."
   └─ Generate answer with sources

5. Return RAG response
   └─ JSON: {answer, sources, model}
```

---

## Triple Retrieval System

### Why Three Retrievers?

Each retriever excels at different query types:

| Retriever | Strength | Example Query |
|-----------|----------|---------------|
| **BM25** | Exact keyword matches, file names | `"settings.yaml"`, `"def authenticate"` |
| **SPLADE** | Learned sparse representations | `"user login"` → matches "authentication" |
| **BM42** | Hybrid sparse+dense | `"config management"` → semantic + keyword |

**Fusion Strategy:** Reciprocal Rank Fusion (RRF)

```python
def rrf_score(ranks, k=60):
    """
    Combine rankings from multiple retrievers.

    score = sum(1 / (k + rank_i))

    k=60 balances importance between:
    - Top results (high impact)
    - Lower-ranked results (still contribute)
    """
    return sum(1.0 / (k + rank) for rank in ranks)
```

### Adaptive Retrieval

Query analyzer classifies queries and routes to optimal strategy:

```python
# Navigational (file/function lookup)
"settings.yaml" → sparse-only (BM25 fast path)

# Factual (direct answer)
"what is the embedding dimension" → balanced hybrid

# Exploratory (concept search)
"how does authentication work" → dense-heavy + rerank

# Analytical (complex reasoning)
"compare authentication methods" → deep-rerank + LLM
```

---

## Storage Architecture

### Qdrant (Vector Database)

**Schema:**
```python
Collection: rice_chunks

Vectors:
  dense:  VectorParams(size=2560, distance=COSINE)
  splade: SparseVectorParams()
  bm42:   SparseVectorParams()

Payload:
  text: str                    # Chunk content
  full_path: str               # Full file system path
  filename: str                # File name only
  org_id: str                  # Organization ID
  chunk_id: str                # Unique chunk ID
  chunk_index: int             # Position in file
  chunk_type: str              # "function", "class", "text"
  language: str                # "python", "javascript", etc.
  start_line: int              # Start line number
  end_line: int                # End line number
  symbols: List[str]           # Function/class names
  doc_id: str                  # Document ID
  minio_bucket: str            # MinIO bucket (if stored)
  minio_object_name: str       # MinIO object key
```

**Indexing:**
- Dense vectors: HNSW index (fast ANN search)
- Sparse vectors: Inverted index (keyword matching)

### Redis (Cache & Queue)

**Usage:**
1. **Celery Queue** (task distribution)
   - Queue: `celery` (default)
   - Results backend: Redis

2. **Settings Cache** (runtime config)
   - Keys: `settings:*`
   - Example: `settings:models.embedding.dimension` → `2560`

3. **Session Cache** (future: user sessions)

### MinIO (Object Storage)

**Usage:**
- Store original files (optional)
- Bucket: `rice-files`
- Object naming: `{org_id}/{doc_id}/{filename}`

**Benefits:**
- Retrieve original file content
- Backup/restore capabilities
- S3-compatible API

### Tantivy (BM25 Index)

**Schema:**
```rust
Document {
  chunk_id: Text,    // Unique ID (indexed)
  text: Text,        // Chunk content (indexed + stored)
}
```

**Index Settings:**
- BM25 parameters: k1=1.2, b=0.75 (default)
- Position indexing: enabled (phrase queries)
- Tokenizer: English stemming

---

## Search Pipeline

### Pipeline Stages

```
Query Input
  ↓
┌─────────────────────────────────┐
│ 1. Query Analysis (optional)   │
│    - Classify intent            │
│    - Select strategy            │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 2. Query Embedding              │
│    - Dense: Ollama              │
│    - SPLADE: Sparse encoder     │
│    - BM42: Hybrid encoder       │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 3. Parallel Retrieval           │
│    ├─ BM25 (Tantivy)            │
│    ├─ SPLADE (Qdrant)           │
│    └─ BM42 (Qdrant)             │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 4. Result Fusion (RRF)          │
│    - Combine rankings           │
│    - Normalize scores           │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 5. Reranking (optional)         │
│    Stage 1: Cross-encoder       │
│    Stage 2: LLM (complex only)  │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 6. Deduplication                │
│    - Group by full_path         │
│    - Keep highest score per file│
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 7. Score Normalization          │
│    - Sigmoid scaling            │
│    - 12-98% range               │
└──────────────┬──────────────────┘
               ↓
Results Output
```

### Performance Optimizations

1. **Lazy Loading** - Encoders load on first use
2. **Model Caching** - TTL-based auto-unloading (5 min default)
3. **Batch Processing** - Parallel embedding generation
4. **Early Exit** - Skip reranking for simple queries
5. **Result Limiting** - Fetch only top K from each retriever

---

## Indexing Pipeline

### Pipeline Stages

```
File Input (upload or watch)
  ↓
┌─────────────────────────────────┐
│ 1. File Detection               │
│    - Check file type            │
│    - Respect .riceignore        │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 2. Delete Old Chunks            │
│    - Query by full_path         │
│    - Delete from Qdrant         │
│    - Delete from Tantivy        │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 3. Parsing                      │
│    AST (Tree-sitter):           │
│      ├─ Python, JS/TS, Go, Rust │
│      ├─ Extract functions/class │
│      └─ Preserve structure      │
│    Fallback (Text):             │
│      └─ Plain text parsing      │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 4. Chunking                     │
│    - Size: 1000 chars           │
│    - Overlap: 200 chars         │
│    - Preserve boundaries (AST)  │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 5. Embedding Generation         │
│    - Dense: Ollama (2560 dims)  │
│    - SPLADE: Sparse encoder     │
│    - BM42: Hybrid encoder       │
└──────────────┬──────────────────┘
               ↓
┌─────────────────────────────────┐
│ 6. Multi-Index Upsert           │
│    ├─ Qdrant (vectors+metadata) │
│    ├─ Tantivy (BM25)            │
│    └─ MinIO (original file)     │
└──────────────┬──────────────────┘
               ↓
Indexing Complete
```

### Deduplication Strategy

**Problem:** Re-indexing a file creates duplicate chunks.

**Solution:** Two-layer deduplication:

1. **Index-Time** (indexer.py)
   - Before indexing: Delete all chunks with same `full_path`
   - Ensures only latest version exists

2. **Search-Time** (retriever.py)
   - Group results by `full_path`
   - Keep highest-scoring chunk per file

---

## Technology Stack

### Backend
- **Language:** Python 3.12
- **Web Framework:** FastAPI 0.104+
- **Async Tasks:** Celery 5.3+
- **Validation:** Pydantic 2.x

### Frontend
- **Framework:** Next.js 14
- **Language:** TypeScript 5.x
- **UI:** Mantine UI + Tailwind CSS
- **State:** React 18 hooks

### Inference
- **LLM Service:** Ollama
- **Embedding Model:** qwen3-embedding:4b (2560 dims)
- **LLM Model:** qwen2.5-coder:1.5b
- **BM25 Service:** Tantivy (Rust)

### Storage
- **Vector DB:** Qdrant 1.7+
- **Cache/Queue:** Redis 7+
- **Object Storage:** MinIO
- **BM25 Index:** Tantivy (custom Rust service)

### DevOps
- **Containerization:** Docker + Docker Compose
- **Orchestration:** Docker Compose
- **Monitoring:** Prometheus + Grafana (optional)
- **Tracing:** Jaeger (enterprise mode)

---

## Design Decisions

### Why Ollama for Embeddings?

**Pros:**
- ✅ Fully local (no API costs)
- ✅ Easy model switching
- ✅ GPU acceleration
- ✅ Keep-alive reduces latency
- ✅ Standard OpenAI-compatible API

**Cons:**
- ❌ Slower than dedicated TEI/vLLM
- ❌ Limited batch size

**Alternative:** TEI (Text Embeddings Inference) for production scale

### Why Tantivy for BM25?

**Pros:**
- ✅ Fast Rust implementation
- ✅ Low memory footprint
- ✅ Standard BM25 algorithm
- ✅ Easy to deploy (HTTP API)

**Cons:**
- ❌ Custom service required
- ❌ Not as mature as Elasticsearch

**Alternative:** Qdrant BM25 (future support)

### Why Qdrant for Vectors?

**Pros:**
- ✅ Native sparse vector support (SPLADE, BM42)
- ✅ High performance (HNSW + inverted index)
- ✅ Rich filtering (metadata queries)
- ✅ gRPC API (fast)
- ✅ Self-hosted friendly

**Cons:**
- ❌ Younger than Pinecone/Weaviate

**Alternative:** Milvus, Weaviate, Pinecone

### Why FastAPI?

**Pros:**
- ✅ High performance (async)
- ✅ Auto-generated OpenAPI docs
- ✅ Type safety (Pydantic)
- ✅ Easy testing
- ✅ Production-ready

**Cons:**
- ❌ Less mature than Django

**Alternative:** Django REST Framework, Flask

---

## Scalability Considerations

### Horizontal Scaling

**Stateless Services** (can scale horizontally):
- Backend API (multiple replicas)
- Celery Workers (add more workers)

**Stateful Services** (require coordination):
- Qdrant (cluster mode)
- Redis (Redis Cluster or Sentinel)

### Vertical Scaling

**GPU Requirements:**
- Ollama: 8GB VRAM for qwen3-embedding
- SPLADE encoder: 4GB VRAM (optional CPU fallback)

**Memory Requirements:**
- Backend API: 2GB per instance
- Worker: 4GB per worker
- Qdrant: 8GB+ (depends on index size)

### Optimization Strategies

1. **Model Auto-Unloading**
   - TTL: 5 minutes (configurable)
   - Reduces memory footprint

2. **Lazy Loading**
   - Encoders load on first use
   - Reduces startup time

3. **Batch Processing**
   - Indexing: 100 chunks/batch
   - Embedding: 32 texts/batch

4. **Caching**
   - Redis: Settings cache
   - Future: Query result cache

5. **Database Tuning**
   - Qdrant HNSW params (M=16, ef=100)
   - Tantivy BM25 params (k1=1.2, b=0.75)

---

## Summary

**Rice Search Architecture Highlights:**

1. **Hybrid Retrieval** - Combines BM25, SPLADE, BM42 for comprehensive search
2. **Adaptive Routing** - Query analysis selects optimal strategy
3. **Code-Aware** - AST parsing for semantic code chunking
4. **Fully Local** - All inference runs on-premise
5. **Modular Design** - Clean separation: API → Services → Tasks → Storage
6. **Multi-Index** - Parallel indexing to Qdrant, Tantivy, MinIO
7. **Deduplication** - Both index-time and search-time deduplication
8. **Configurable** - Three-layer config (YAML → Env → Runtime)

**Key Patterns:**
- **Service Layer** - FastAPI + Celery for async processing
- **Triple Retrieval** - BM25 + SPLADE + BM42 with RRF fusion
- **Lazy Loading** - Models load on demand, unload after TTL
- **Multi-Index Storage** - Qdrant (vectors) + Tantivy (BM25) + MinIO (files)

For implementation details, see:
- [Development Guide](development.md) - Build and test services
- [Configuration](configuration.md) - Tuning parameters
- [API Reference](api.md) - Endpoint specifications
- [Deployment](deployment.md) - Production setup

---

**[Back to Documentation Index](README.md)**
