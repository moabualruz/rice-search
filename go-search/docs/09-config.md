# Configuration

## Overview

Configuration via environment variables, CLI flags, or config file. Priority: CLI > Env > Config file > Defaults.

---

## All Configuration Options

### Server

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_PORT` | `--port` | `8080` | HTTP port |
| `RICE_HOST` | `--host` | `0.0.0.0` | Bind address |
| `RICE_ENABLE_WEB` | - | `true` | Enable Web UI |
| `RICE_ENABLE_ML` | - | `true` | Enable ML services |

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
| `RICE_MODELS_DIR` | `--models-dir` | `./models` | Models directory |
| `RICE_EMBED_MODEL` | - | `jinaai/jina-code-embeddings-1.5b` | Embedding model |
| `RICE_SPARSE_MODEL` | - | `splade-v3` | Sparse model |
| `RICE_RERANK_MODEL` | - | `jinaai/jina-reranker-v2-base-multilingual` | Reranker model |
| `RICE_QUERY_MODEL` | - | `microsoft/codebert-base` | Query understanding model |
| `RICE_EMBED_DIM` | - | `1536` | Embedding dimensions |
| `RICE_EMBED_BATCH_SIZE` | - | `32` | Embedding batch size |
| `RICE_SPARSE_BATCH_SIZE` | - | `32` | Sparse batch size |
| `RICE_RERANK_BATCH_SIZE` | - | `32` | Rerank batch size |
| `RICE_MAX_SEQ_LENGTH` | - | `8192` | Maximum sequence length |
| `RICE_ML_URL` | - | - | External ML service URL (distributed mode) |

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
| `RICE_ML_DEVICE` | `--device` | `cuda` | Device (cpu, cuda, tensorrt) |
| `RICE_ML_CUDA_DEVICE` | - | `0` | GPU index |
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
| `RICE_DEFAULT_TOP_K` | - | `20` | Default results |
| `RICE_DEFAULT_SPARSE_WEIGHT` | - | `0.5` | Default sparse weight |
| `RICE_DEFAULT_DENSE_WEIGHT` | - | `0.5` | Default dense weight |
| `RICE_ENABLE_RERANKING` | - | `true` | Enable reranking by default |
| `RICE_RERANK_CANDIDATES` | - | `30` | Default rerank candidates |
| `RICE_ENABLE_DEDUP` | - | `true` | Enable deduplication |
| `RICE_DEDUP_THRESHOLD` | - | `0.85` | Dedup similarity threshold |
| `RICE_ENABLE_DIVERSITY` | - | `true` | Enable MMR diversity |
| `RICE_DIVERSITY_LAMBDA` | - | `0.7` | Diversity lambda (0=diverse, 1=relevant) |
| `RICE_GROUP_BY_FILE` | - | `false` | Group results by file |
| `RICE_MAX_CHUNKS_PER_FILE` | - | `3` | Max chunks per file when grouping |

### Indexing

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_CHUNK_SIZE` | - | `512` | Target chunk size (tokens) |
| `RICE_CHUNK_OVERLAP` | - | `64` | Chunk overlap (tokens) |
| `RICE_INDEX_WORKERS` | - | `4` | Parallel indexing workers |

### Caching

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_CACHE_TYPE` | - | `memory` | Cache backend (memory, redis) |
| `RICE_CACHE_SIZE` | - | `100000` | Max cache entries |
| `RICE_CACHE_TTL` | - | `0` | Cache TTL in seconds (0 = no expiry) |
| `RICE_REDIS_URL` | - | - | Redis URL for distributed cache |

### Event Bus

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_BUS_TYPE` | `--bus` | `memory` | Bus type (memory, kafka, nats, redis) |
| `RICE_KAFKA_BROKERS` | - | `localhost:9092` | Kafka brokers |
| `RICE_KAFKA_GROUP` | - | `rice-search` | Kafka consumer group |
| `RICE_NATS_URL` | - | `nats://localhost:4222` | NATS URL |
| `RICE_REDIS_STREAM_URL` | - | - | Redis streams URL |
| `RICE_EVENT_LOG_ENABLED` | - | `false` | Enable event logging to file |
| `RICE_EVENT_LOG_PATH` | - | `./logs/events.log` | Event log file path |

### Security

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_API_KEY` | - | - | API key for authentication (if set, enables auth) |
| `RICE_RATE_LIMIT` | - | `0` | Rate limit per minute (0 = disabled) |
| `RICE_CORS_ORIGINS` | - | `*` | Allowed CORS origins |

### Logging

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_LOG_LEVEL` | `--log-level` | `info` | Level (debug, info, warn, error) |
| `RICE_LOG_FORMAT` | `--log-format` | `text` | Format (text, json) |
| `RICE_LOG_FILE` | - | - | Log file path (if set, logs to file) |

### Observability

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_METRICS_ENABLED` | - | `true` | Enable Prometheus metrics |
| `RICE_METRICS_PATH` | - | `/metrics` | Metrics endpoint |
| `RICE_METRICS_PERSISTENCE` | - | `memory` | Metrics storage (memory, redis) |
| `RICE_METRICS_REDIS_URL` | - | - | Redis URL for metrics persistence |
| `RICE_TRACING_ENABLED` | - | `false` | Enable tracing |
| `RICE_TRACING_ENDPOINT` | - | - | OTLP endpoint |

### Connection Tracking

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_CONNECTIONS_ENABLED` | - | `true` | Enable connection tracking |
| `RICE_CONNECTIONS_PATH` | - | `./data/connections` | Connection storage path |
| `RICE_CONNECTIONS_MAX_INACTIVE` | - | `30` | Days before marking connection inactive |

### Model Registry

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_MODELS_REGISTRY` | - | `./models/registry.json` | Model registry file path |
| `RICE_MODELS_MAPPERS` | - | `./models/mappers` | Model mappers directory |
| `RICE_MODELS_AUTO_DOWNLOAD` | - | `false` | Auto-download missing models |

### Settings System

| Env Var | CLI Flag | Default | Description |
|---------|----------|---------|-------------|
| `RICE_SETTINGS_AUDIT_ENABLED` | - | `true` | Enable settings change audit |
| `RICE_SETTINGS_AUDIT_PATH` | - | `./data/settings-audit.log` | Settings audit log path |

---

## Config File Format

```yaml
# rice-search.yaml

server:
  host: 0.0.0.0
  port: 8080
  enable_web: true
  enable_ml: true

qdrant:
  url: http://localhost:6333
  api_key: ""

ml:
  device: cuda  # cpu, cuda, tensorrt (GPU-first default)
  cuda_device: 0
  embed_model: jinaai/jina-code-embeddings-1.5b
  sparse_model: splade-v3
  rerank_model: jinaai/jina-reranker-v2-base-multilingual
  query_model: microsoft/codebert-base
  embed_dim: 1536
  embed_batch_size: 32
  sparse_batch_size: 32
  rerank_batch_size: 32
  max_seq_length: 8192
  models_dir: ./models
  external_url: ""  # For distributed mode
  embed_gpu: true   # GPU for embeddings
  rerank_gpu: true  # GPU for reranking
  query_gpu: true   # GPU for query understanding
  query_model_enabled: true  # Enable model-based query understanding

connection:
  enabled: true
  storage_path: ./data/connections
  max_inactive: 30  # Days

model_registry:
  registry_path: ./models/registry.json
  mappers_path: ./models/mappers
  auto_download: false

cache:
  type: memory  # memory, redis
  size: 100000
  ttl: 0  # 0 = no expiry
  redis_url: ""

bus:
  type: memory  # memory, kafka, nats, redis
  kafka_brokers: localhost:9092
  kafka_group: rice-search
  nats_url: nats://localhost:4222
  redis_url: ""
  event_log_enabled: false
  event_log_path: ./logs/events.log

index:
  chunk_size: 512
  chunk_overlap: 64
  workers: 4

search:
  default_top_k: 20
  default_sparse_weight: 0.5
  default_dense_weight: 0.5
  enable_reranking: true
  rerank_candidates: 30
  enable_dedup: true
  dedup_threshold: 0.85
  enable_diversity: true
  diversity_lambda: 0.7
  group_by_file: false
  max_chunks_per_file: 3

log:
  level: info
  format: text  # text, json
  file: ""

security:
  api_key: ""
  rate_limit: 0  # 0 = disabled
  cors_origins: "*"

observability:
  metrics_enabled: true
  metrics_path: /metrics
  tracing_enabled: false
  tracing_endpoint: ""

metrics:
  persistence: memory  # memory, redis
  redis_url: ""

settings:
  audit_enabled: true
  audit_path: ./data/settings-audit.log
```

---

## Environment Variable Expansion

Config files support env var expansion:

```yaml
qdrant:
  url: ${QDRANT_URL:-http://localhost:6333}
  api_key: ${QDRANT_API_KEY}

security:
  api_key: ${RICE_API_KEY}
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
