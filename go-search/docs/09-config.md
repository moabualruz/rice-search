# Configuration

## Overview

Configuration via environment variables, CLI flags, or config file. Priority: CLI > Env > Config file > Defaults.

---

## All Configuration Options

### Server

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `PORT` | `--port` | `8080` | HTTP port |
| `HOST` | `--host` | `0.0.0.0` | Bind address |
| `READ_TIMEOUT` | - | `30s` | HTTP read timeout |
| `WRITE_TIMEOUT` | - | `30s` | HTTP write timeout |
| `SHUTDOWN_TIMEOUT` | - | `10s` | Graceful shutdown timeout |

### Qdrant

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `QDRANT_URL` | `--qdrant-url` | `http://localhost:6333` | Qdrant URL |
| `QDRANT_API_KEY` | - | - | Qdrant API key (if auth enabled) |
| `QDRANT_TIMEOUT` | - | `30s` | Request timeout |
| `QDRANT_COLLECTION_PREFIX` | - | `rice` | Collection name prefix |

### Models

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `MODELS_DIR` | `--models-dir` | `./models` | Models directory |
| `EMBED_MODEL` | - | `jinaai/jina-code-embeddings-1.5b` | Embedding model |
| `SPARSE_MODEL` | - | `splade-v3` | Sparse model |
| `RERANK_MODEL` | - | `jinaai/jina-reranker-v2-base-multilingual` | Reranker model |
| `QUERY_MODEL` | - | `microsoft/codebert-base` | Query understanding model |

### Query Understanding

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_QUERY_MODEL` | - | `microsoft/codebert-base` | Query understanding model |
| `RICE_QUERY_MODEL_ENABLED` | - | `true` | Enable model-based query understanding |
| `RICE_QUERY_GPU` | - | `true` | Run query model on GPU |

**Model Options:**

| Model | Size | Speed | Quality | Use Case |
|-------|------|-------|---------|----------|
| `microsoft/codebert-base` | 438MB | ~50ms GPU | ⭐⭐⭐⭐⭐ | **Default** - Code-specialized |
| `Salesforce/codet5p-220m` | 220MB | ~40ms GPU | ⭐⭐⭐⭐ | Alternative encoder-decoder |
| (heuristic fallback) | 0MB | ~1ms | ⭐⭐⭐ | Disabled or fallback mode |

**When to disable:**
- Memory-constrained environments (saves ~438MB VRAM)
- Keyword-only queries (no semantic benefit needed)
- Maximum throughput requirements

### Device / GPU (GPU-First Architecture)

All models default to GPU for maximum performance.

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `DEVICE` | `--device` | `cuda` | Device (cpu, cuda, tensorrt) |
| `CUDA_VISIBLE_DEVICES` | - | `0` | GPU index |
| `GPU_LOAD_MODE` | `--load-mode` | `all` | Model loading (all, ondemand, lru) |
| `GPU_UNLOAD_TIMEOUT` | - | `60s` | Unload timeout (ondemand mode) |
| `GPU_LRU_SIZE` | - | `2` | Models to keep (lru mode) |
| `ONNX_PROVIDER` | - | `cuda` | ONNX provider (cpu, cuda, tensorrt) |
| `RICE_EMBED_GPU` | - | `true` | Embedding model on GPU |
| `RICE_RERANK_GPU` | - | `true` | Reranking model on GPU |
| `RICE_QUERY_GPU` | - | `true` | Query understanding on GPU |

**VRAM Requirements (GPU-First):**

| Configuration | Total VRAM | Models Loaded |
|---------------|------------|---------------|
| All GPU (default) | ~3GB | Embed + Sparse + Rerank + Query |
| No Query GPU | ~2.5GB | Embed + Sparse + Rerank |
| Embed + Rerank only | ~2.3GB | Embed + Rerank |
| CPU fallback | 0GB | All on CPU (slower) |

### Search

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `DEFAULT_TOP_K` | - | `20` | Default results |
| `MAX_TOP_K` | - | `100` | Maximum results |
| `SPARSE_WEIGHT` | - | `0.5` | Default sparse weight |
| `DENSE_WEIGHT` | - | `0.5` | Default dense weight |
| `RRF_K` | - | `60` | RRF smoothing constant |
| `ENABLE_RERANKING` | - | `true` | Enable reranking by default |
| `RERANK_TOP_K` | - | `30` | Default rerank candidates |

### Indexing

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `CHUNK_SIZE` | - | `512` | Target chunk size (tokens) |
| `CHUNK_OVERLAP` | - | `64` | Chunk overlap (tokens) |
| `MAX_FILE_SIZE` | - | `10485760` | Max file size (10MB) |
| `EMBED_BATCH_SIZE` | - | `32` | Embedding batch size |
| `EMBED_PARALLEL` | - | `4` | Parallel embedding batches |
| `INDEX_TIMEOUT` | - | `30m` | Full reindex timeout |

### Caching

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `EMBED_CACHE_SIZE` | - | `100000` | Max cached embeddings |
| `SPARSE_CACHE_SIZE` | - | `100000` | Max cached sparse vectors |
| `CACHE_BACKEND` | - | `memory` | Cache backend (memory, redis) |
| `REDIS_URL` | - | - | Redis URL for distributed cache |

### Event Bus

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `EVENT_BUS` | `--bus` | `memory` | Bus type (memory, kafka, nats, redis) |
| `KAFKA_BROKERS` | - | `localhost:9092` | Kafka brokers |
| `KAFKA_GROUP` | - | `rice-search` | Kafka consumer group |
| `NATS_URL` | - | `nats://localhost:4222` | NATS URL |
| `REDIS_STREAM_URL` | - | - | Redis streams URL |

### Auth

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `AUTH_MODE` | - | `none` | Auth mode (none, api-key, jwt) |
| `API_KEYS` | - | - | Comma-separated API keys |
| `JWT_SECRET` | - | - | JWT signing secret |
| `JWT_ISSUER` | - | `rice-search` | JWT issuer |

### Rate Limiting

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RATE_LIMIT_ENABLED` | - | `false` | Enable rate limiting |
| `RATE_LIMIT_SEARCH` | - | `100` | Search requests/min |
| `RATE_LIMIT_INDEX` | - | `20` | Index requests/min |
| `RATE_LIMIT_ML` | - | `200` | ML requests/min |

### Logging

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `LOG_LEVEL` | `--log-level` | `info` | Level (debug, info, warn, error) |
| `LOG_FORMAT` | `--log-format` | `text` | Format (text, json) |

### Observability

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `METRICS_ENABLED` | - | `true` | Enable Prometheus metrics |
| `METRICS_PATH` | - | `/metrics` | Metrics endpoint |
| `TRACING_ENABLED` | - | `false` | Enable tracing |
| `TRACING_ENDPOINT` | - | - | OTLP endpoint |
| `TRACING_SAMPLE_RATE` | - | `0.1` | Trace sample rate |

---

## Config File Format

```yaml
# rice-search.yaml

server:
  port: 8080
  host: 0.0.0.0
  read_timeout: 30s
  write_timeout: 30s
  shutdown_timeout: 10s

qdrant:
  url: http://localhost:6333
  api_key: ""
  timeout: 30s
  collection_prefix: rice

models:
  dir: ./models
  embed: jinaai/jina-code-embeddings-1.5b
  sparse: splade-v3
  rerank: jinaai/jina-reranker-v2-base-multilingual
  query: microsoft/codebert-base  # Query understanding model

device:
  type: cuda  # cpu, cuda, tensorrt (GPU-first default)
  cuda_device: 0
  load_mode: all  # all, ondemand, lru
  unload_timeout: 60s
  lru_size: 2

ml:
  embed_gpu: true      # GPU for embeddings
  rerank_gpu: true     # GPU for reranking
  query_gpu: true      # GPU for query understanding
  query_enabled: true  # Enable CodeBERT query understanding

search:
  default_top_k: 20
  max_top_k: 100
  sparse_weight: 0.5
  dense_weight: 0.5
  rrf_k: 60
  enable_reranking: true
  rerank_top_k: 30

indexing:
  chunk_size: 512
  chunk_overlap: 64
  max_file_size: 10485760
  embed_batch_size: 32
  embed_parallel: 4
  timeout: 30m

cache:
  embed_size: 100000
  sparse_size: 100000
  backend: memory  # memory, redis
  redis_url: ""

event_bus:
  type: memory  # memory, kafka, nats, redis
  kafka:
    brokers: [localhost:9092]
    group: rice-search
  nats:
    url: nats://localhost:4222
  redis:
    url: ""

auth:
  mode: none  # none, api-key, jwt
  api_keys: []
  jwt:
    secret: ""
    issuer: rice-search

rate_limit:
  enabled: false
  search: 100
  index: 20
  ml: 200

logging:
  level: info
  format: text  # text, json

observability:
  metrics:
    enabled: true
    path: /metrics
  tracing:
    enabled: false
    endpoint: ""
    sample_rate: 0.1
```

---

## Environment Variable Expansion

Config files support env var expansion:

```yaml
qdrant:
  url: ${QDRANT_URL:-http://localhost:6333}
  api_key: ${QDRANT_API_KEY}

auth:
  jwt:
    secret: ${JWT_SECRET}
```

---

## Validation

On startup, configuration is validated:

| Check | Error |
|-------|-------|
| Port in valid range | "port must be 1-65535" |
| Models directory exists | "models directory not found" |
| Required models present | "model not found: embed" |
| Qdrant reachable | "cannot connect to Qdrant" |
| Valid device | "invalid device: xyz" |
| Valid log level | "invalid log level: xyz" |

---

## Defaults Per Mode

### Monolith Mode

```bash
rice-search serve
# Uses all defaults, auto device detection
```

### Microservices Mode

```bash
# API service
rice-search api --bus kafka://kafka:9092

# Defaults:
# - EVENT_BUS=kafka://kafka:9092
# - PORT=8080
```

```bash
# ML service  
rice-search ml --bus kafka://kafka:9092 --device cuda

# Defaults:
# - EVENT_BUS=kafka://kafka:9092
# - PORT=8081
# - DEVICE=cuda
# - GPU_LOAD_MODE=all
```

---

## Sensitive Values

Never log or expose:

| Value | Handling |
|-------|----------|
| `API_KEYS` | Masked in logs |
| `JWT_SECRET` | Masked in logs |
| `QDRANT_API_KEY` | Masked in logs |
| `REDIS_URL` (with password) | Password masked |
