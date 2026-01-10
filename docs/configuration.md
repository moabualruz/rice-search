# Configuration Guide

Complete reference for configuring Rice Search, including settings files, environment variables, and runtime configuration.

## Table of Contents

- [Configuration System](#configuration-system)
- [Configuration Layers](#configuration-layers)
- [Settings File (settings.yaml)](#settings-file-settingsyaml)
- [Environment Variables](#environment-variables)
- [Runtime Configuration](#runtime-configuration)
- [Common Settings](#common-settings)
- [Service-Specific Configuration](#service-specific-configuration)
- [Performance Tuning](#performance-tuning)
- [Security Settings](#security-settings)
- [Troubleshooting Configuration](#troubleshooting-configuration)

---

## Configuration System

Rice Search uses a **three-layer configuration system** with the following priority:

```bash
1. Runtime Settings (Redis)         ← Highest priority
2. Environment Variables (.env)
3. Settings File (settings.yaml)    ← Lowest priority
```

**How it works:**

- Settings are loaded from `backend/settings.yaml` at startup
- Environment variables override YAML settings
- Runtime changes (via API/CLI) are stored in Redis and take precedence
- All settings use dot-notation keys (e.g., `models.embedding.dimension`)

---

## Configuration Layers

### Layer 1: settings.yaml (Base Configuration)

**Location:** `backend/settings.yaml`

The primary configuration file with all default settings organized by category:

```yaml
app:
  name: "Rice Search"
  version: "1.0.0"
  api_prefix: "/api/v1"

models:
  embedding:
    name: "qwen3-embedding"
    dimension: 2560

search:
  hybrid:
    rrf_k: 60
    use_bm25: true
```

**When to use:**

- Setting project defaults
- Defining application structure
- Committing shared configuration to Git

### Layer 2: Environment Variables (Deployment Overrides)

**Location:** `deploy/.env` (create from `deploy/.env.example`)

Override YAML settings per environment (dev, staging, prod):

```bash
# .env file
EMBEDDING_MODEL=jinaai/jina-embeddings-v3
EMBEDDING_DIM=1024
LLM_MODEL=google/codegemma-7b-it
RERANK_MODE=local
```

**When to use:**

- Environment-specific settings (dev vs prod)
- Secrets and credentials (not in Git)
- Docker deployment configuration

### Layer 3: Runtime Configuration (Live Changes)

**Location:** Redis (`settings:*` keys)

Modify settings at runtime via API or CLI:

```bash
# Via API
curl -X POST http://localhost:8000/api/v1/settings/models.embedding.dimension \
  -H "Content-Type: application/json" \
  -d '{"value": 1024}'

# Via Python (internal)
from src.core.config import settings
settings.set("models.embedding.dimension", 1024)
```

**When to use:**

- Live updates without restarting services
- A/B testing different configurations
- Dynamic feature toggling

---

## Settings File (settings.yaml)

Full structure of `backend/settings.yaml`:

### Application Settings

```yaml
app:
  name: "Rice Search"                # Application name
  version: "1.0.0"                   # Version string
  api_prefix: "/api/v1"              # API route prefix
  debug: false                       # Debug mode (verbose logging)
```

### Server Settings

```yaml
server:
  host: "0.0.0.0"                    # Bind address
  port: 8000                         # API port
  workers: 1                         # Uvicorn workers
  cors_origins:                      # Allowed CORS origins
    - "http://localhost:3000"
    - "http://localhost:8000"
```

### Infrastructure

```yaml
infrastructure:
  qdrant:
    url: "http://qdrant:6333"        # Qdrant vector DB URL
    timeout: 30                      # Connection timeout (seconds)

  redis:
    url: "redis://redis:6379/0"      # Redis URL
    socket_timeout: 10               # Socket timeout (seconds)

  minio:
    endpoint: "minio:9000"           # MinIO object storage
    access_key: "minioadmin"
    secret_key: "minioadmin"
    secure: false                    # Use HTTPS
```

### Models Configuration

#### Embedding Model

```yaml
models:
  embedding:
    name: "qwen3-embedding"          # Model name
    dimension: 2560                  # Vector dimension (MUST match model!)
    fallback_dimension: 768          # Fallback for old data
    timeout: 60                      # Embedding timeout (seconds)
```

**Supported models:**

- `qwen3-embedding:4b` → 2560 dimensions
- `jinaai/jina-embeddings-v3` → 1024 dimensions
- `BAAI/bge-base-en-v1.5` → 768 dimensions

#### Sparse Model (SPLADE)

```yaml
models:
  sparse:
    enabled: true                    # Enable SPLADE sparse vectors
    model: "naver/splade-cocondenser-ensembledistil"
    lightweight_model: "naver/efficient-splade-VI-BT-large-doc"
    device: "auto"                   # "cuda", "cpu", or "auto"
    precision: "fp16"                # "fp32", "fp16", or "int8"
    batch_size: 32                   # Encoding batch size
    max_tokens: 512                  # Max token length
    vocab_size: 30522                # BERT vocab size
    min_word_length: 2               # Min word length for indexing
```

#### BM42 Model

```yaml
models:
  bm42:
    model: "qdrant/bm42-all-minilm-l6-v2-attentions"  # Qdrant native hybrid model
```

#### Reranker Model

```yaml
models:
  reranker:
    enabled: true                    # Enable reranking
    mode: "local"                    # "local" (cross-encoder) or "llm"
    model: "cross-encoder/ms-marco-MiniLM-L-12-v2"
    top_k: 50                        # Rerank top K candidates
    doc_preview_length: 200          # Preview length for LLM mode
    llm_max_tokens: 500              # Max tokens for LLM reranking
    llm_temperature: 0.0             # Temperature for LLM reranking
```

#### LLM Model

```yaml
models:
  llm:
    max_tokens: 2048                 # Max output tokens
    temperature: 0.7                 # Sampling temperature
    chat_timeout: 120                # Chat timeout (seconds)
```

#### Query Analysis Model

```yaml
models:
  query_analysis:
    model: "qwen2.5-coder:1.5b"      # Query classification model
    llm_max_tokens: 500
    llm_temperature: 0.0
```

### Inference Configuration

```yaml
inference:
  ollama:
    base_url: "http://ollama:11434"  # Ollama service URL
    embedding_model: "qwen3-embedding:4b"
    llm_model: "qwen2.5-coder:1.5b"
    keep_alive: "5m"                 # Keep model in memory
    timeout: 120
```

### Search Configuration

```yaml
search:
  collection_prefix: "rice_chunks"   # Qdrant collection name
  default_limit: 10                  # Default search limit
  default_mode: "search"             # "search" or "rag"

  hybrid:
    rrf_k: 60                        # Reciprocal Rank Fusion K
    use_bm25: true                   # Enable BM25 retrieval
    use_splade: true                 # Enable SPLADE retrieval
    use_bm42: true                   # Enable BM42 retrieval

  bm25:
    enabled: true
    url: "http://tantivy:3002"       # Tantivy BM25 service
    timeout: 10

  query_analysis:
    enabled: true                    # Enable adaptive query routing
```

### AST Parsing

```yaml
ast:
  enabled: true                      # Enable AST-aware chunking
  languages:                         # Supported languages
    - python
    - javascript
    - typescript
    - go
    - rust
    - java
    - cpp
  max_chunk_lines: 100               # Max lines per chunk
```

### Indexing Configuration

```yaml
indexing:
  chunk_size: 1000                   # Characters per chunk
  chunk_overlap: 200                 # Overlap between chunks
  batch_size: 100                    # Batch size for indexing
  temp_dir: "/tmp/rice-ingest"       # Temp directory for uploads
```

### RAG Configuration

```yaml
rag:
  enabled: true                      # Enable RAG mode
  max_tokens: 2048                   # Max RAG response tokens
  temperature: 0.7                   # RAG temperature
  system_prompt: "You are a helpful code assistant..."
```

### MCP Configuration

```yaml
mcp:
  enabled: false                     # Enable MCP server
  transport: "stdio"                 # "stdio" or "tcp"
  tcp:
    host: "0.0.0.0"
    port: 3100
  sse:
    port: 3101
```

### Model Management

```yaml
model_management:
  force_gpu: true                    # Force GPU usage (fail if unavailable)
  ttl_seconds: 300                   # Model auto-unload TTL (5 minutes)
  auto_unload: true                  # Enable auto-unloading of idle models
```

### Admin & Metrics

```yaml
admin:
  persist_dir: "/data/admin"         # Admin data persistence
  redis_key_prefix: "settings:"      # Redis key prefix

metrics:
  enabled: true                      # Enable Prometheus metrics
  psutil_interval: 5                 # psutil polling interval (seconds)
```

### CLI Settings

```yaml
cli:
  default_limit: 10                  # Default search results
  default_hybrid: true               # Default hybrid search
```

---

## Environment Variables

Environment variables override settings.yaml values.

### Naming Convention

Convert dot-notation to SCREAMING_SNAKE_CASE:

```
models.embedding.dimension  → EMBEDDING_DIM
search.hybrid.rrf_k         → RRF_K
infrastructure.qdrant.url   → QDRANT_URL
```

### Common Environment Variables

#### Infrastructure

```bash
# Qdrant Vector Database
QDRANT_URL=http://qdrant:6333

# Redis Cache & Queue
REDIS_URL=redis://redis:6379/0
REDIS_SOCKET_TIMEOUT=10

# MinIO Object Storage
MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=minioadmin
MINIO_SECRET_KEY=minioadmin
```

#### Models

```bash
# Embedding Model
EMBEDDING_MODEL=qwen3-embedding
EMBEDDING_DIM=2560
EMBEDDING_TIMEOUT=60

# LLM Model
LLM_MODEL=qwen2.5-coder:1.5b
LLM_MAX_TOKENS=2048
LLM_TEMPERATURE=0.7

# Reranker
RERANK_ENABLED=true
RERANK_MODE=local
RERANK_MODEL=cross-encoder/ms-marco-MiniLM-L-12-v2

# Sparse Models
SPARSE_ENABLED=true
SPLADE_MODEL=naver/splade-cocondenser-ensembledistil
BM42_ENABLED=true
```

#### Inference Services

```bash
# Ollama
OLLAMA_BASE_URL=http://ollama:11434
EMBEDDING_MODEL_NAME=qwen3-embedding:4b

# Tantivy BM25
TANTIVY_URL=http://tantivy:3002
BM25_ENABLED=true
```

#### Search Configuration

```bash
# Hybrid Search
RRF_K=60
DEFAULT_USE_BM25=true
DEFAULT_USE_SPLADE=true
DEFAULT_USE_BM42=true

# Query Analysis
QUERY_ANALYSIS_ENABLED=true
```

#### Features

```bash
# AST Parsing
AST_PARSING_ENABLED=true

# MCP Server
MCP_ENABLED=false
MCP_TRANSPORT=stdio

# Authentication
AUTH_ENABLED=false

# RAG Mode
RAG_ENABLED=true
```

#### Model Management

```bash
# GPU Settings
FORCE_GPU=true

# Auto-Unloading
MODEL_TTL_SECONDS=300
MODEL_AUTO_UNLOAD=true
```

### Setting Environment Variables

#### Docker Compose

Create `deploy/.env` from `deploy/.env.example`:

```bash
# Copy template
cp deploy/.env.example deploy/.env

# Edit variables
nano deploy/.env

# Restart services
docker compose -f deploy/docker-compose.yml down
docker compose -f deploy/docker-compose.yml up -d
```

#### Shell Export

```bash
# Linux/macOS
export EMBEDDING_DIM=1024
export RRF_K=60

# Windows PowerShell
$env:EMBEDDING_DIM="1024"
$env:RRF_K="60"
```

#### Python Script

```python
import os

os.environ["EMBEDDING_DIM"] = "1024"
os.environ["RRF_K"] = "60"
```

---

## Runtime Configuration

Modify settings at runtime without restarting services.

### Via API

```bash
# Get setting
curl http://localhost:8000/api/v1/settings/models.embedding.dimension

# Set setting
curl -X POST http://localhost:8000/api/v1/settings/models.embedding.dimension \
  -H "Content-Type: application/json" \
  -d '{"value": 1024}'

# Get all settings
curl http://localhost:8000/api/v1/settings

# Get settings by prefix
curl http://localhost:8000/api/v1/settings?prefix=models
```

### Via Python (Internal)

```python
from src.core.config import settings

# Get setting
dim = settings.get("models.embedding.dimension")
print(dim)  # 2560

# Set setting (persists to Redis)
settings.set("models.embedding.dimension", 1024)

# Set without persisting
settings.set("models.embedding.dimension", 1024, persist=False)

# Get all settings
all_settings = settings.get_all()

# Get by prefix
model_settings = settings.get_all(prefix="models")

# Reload from file
settings.reload()
```

### Via Redis (Manual)

```bash
# Connect to Redis
docker exec -it deploy-redis-1 redis-cli

# Get setting
GET settings:models.embedding.dimension

# Set setting
SET settings:models.embedding.dimension 1024

# List all settings
KEYS settings:*
```

---

## Common Settings

### Changing Embedding Model

When changing the embedding model, you **must** update the dimension:

```yaml
# Option 1: qwen3-embedding:4b (2560 dims)
models:
  embedding:
    name: "qwen3-embedding"
    dimension: 2560

inference:
  ollama:
    embedding_model: "qwen3-embedding:4b"
```

```yaml
# Option 2: Jina v3 (1024 dims)
models:
  embedding:
    name: "jinaai/jina-embeddings-v3"
    dimension: 1024

inference:
  ollama:
    embedding_model: "jinaai/jina-embeddings-v3"
```

```yaml
# Option 3: BGE Base (768 dims)
models:
  embedding:
    name: "BAAI/bge-base-en-v1.5"
    dimension: 768

inference:
  ollama:
    embedding_model: "BAAI/bge-base-en-v1.5"
```

**After changing the model:**

1. Delete Qdrant collection: `curl -X DELETE http://localhost:6333/collections/rice_chunks`
2. Restart services: `make down && make up`
3. Re-index files: `ricesearch watch ./backend`

### Disabling Features

```yaml
# Disable AST parsing (use simple chunking)
ast:
  enabled: false

# Disable reranking
models:
  reranker:
    enabled: false

# Disable SPLADE sparse vectors
models:
  sparse:
    enabled: false

# Disable BM42
search:
  hybrid:
    use_bm42: false

# Disable query analysis (no adaptive routing)
search:
  query_analysis:
    enabled: false
```

### Adjusting Search Strategy

```yaml
# BM25-heavy (fast keyword search)
search:
  hybrid:
    use_bm25: true
    use_splade: false
    use_bm42: false

# Semantic-heavy (concept search)
search:
  hybrid:
    use_bm25: false
    use_splade: true
    use_bm42: true

# Balanced hybrid (default)
search:
  hybrid:
    use_bm25: true
    use_splade: true
    use_bm42: true
    rrf_k: 60
```

---

## Service-Specific Configuration

### Qdrant (Vector Database)

```yaml
infrastructure:
  qdrant:
    url: "http://qdrant:6333"
    timeout: 30
```

**External Qdrant cluster:**
```yaml
infrastructure:
  qdrant:
    url: "https://qdrant-cluster.example.com:6333"
    api_key: "your-api-key"  # Add to settings.yaml if needed
```

### Redis (Cache & Queue)

```yaml
infrastructure:
  redis:
    url: "redis://redis:6379/0"
    socket_timeout: 10
```

**Redis with password:**
```yaml
infrastructure:
  redis:
    url: "redis://:password@redis:6379/0"
```

**Redis Cluster:**
```yaml
infrastructure:
  redis:
    url: "redis://redis-cluster:6379/0"
    cluster_mode: true
```

### Ollama (LLM & Embeddings)

```yaml
inference:
  ollama:
    base_url: "http://ollama:11434"
    embedding_model: "qwen3-embedding:4b"
    llm_model: "qwen2.5-coder:1.5b"
    keep_alive: "5m"
    timeout: 120
```

**Remote Ollama:**
```yaml
inference:
  ollama:
    base_url: "https://ollama.example.com"
```

### Tantivy (BM25 Search)

```yaml
search:
  bm25:
    enabled: true
    url: "http://tantivy:3002"
    timeout: 10
```

---

## Performance Tuning

### Memory Management

```yaml
# Aggressive unloading (low memory)
model_management:
  ttl_seconds: 60      # Unload after 1 minute
  auto_unload: true

# Keep models loaded (high memory)
model_management:
  ttl_seconds: 3600    # Unload after 1 hour
  auto_unload: false
```

### GPU Allocation

```yaml
# Force GPU (fail if unavailable)
model_management:
  force_gpu: true

# Allow CPU fallback
model_management:
  force_gpu: false
```

**SPLADE GPU usage:**
```yaml
models:
  sparse:
    device: "cuda"       # Force GPU
    device: "cpu"        # Force CPU
    device: "auto"       # Auto-detect
    precision: "fp16"    # Faster, less VRAM
```

### Batch Sizes

```yaml
# Indexing batch size
indexing:
  batch_size: 100       # Larger = faster but more memory

# SPLADE batch size
models:
  sparse:
    batch_size: 32      # Adjust for VRAM
```

### Timeout Configuration

```yaml
# Embedding timeout
models:
  embedding:
    timeout: 60         # Increase for large batches

# Tantivy timeout
search:
  bm25:
    timeout: 10         # Increase for slow queries

# Ollama timeout
inference:
  ollama:
    timeout: 120        # Increase for slow models
```

---

## Security Settings

### Authentication

```yaml
auth:
  enabled: true         # Enable authentication
```

**Environment variable:**
```bash
AUTH_ENABLED=true
```

### CORS Origins

```yaml
server:
  cors_origins:
    - "http://localhost:3000"
    - "https://app.example.com"
```

**Production:**
```yaml
server:
  cors_origins:
    - "https://rice-search.example.com"
```

### Secrets Management

**DO NOT commit secrets to Git!**

Use environment variables for sensitive data:

```bash
# .env (NOT in Git)
MINIO_ACCESS_KEY=your-access-key
MINIO_SECRET_KEY=your-secret-key
REDIS_PASSWORD=your-redis-password
QDRANT_API_KEY=your-qdrant-key
```

---

## Troubleshooting Configuration

### Settings Not Applied

**Problem:** Changed settings.yaml but no effect.

**Solution:**
1. Restart services: `make down && make up`
2. Check environment variables override: `docker compose config`
3. Clear Redis settings: `docker exec -it deploy-redis-1 redis-cli FLUSHDB`

### Dimension Mismatch Error

**Problem:**
```text
RuntimeError: vector dimension mismatch (expected 2560, got 768)
```

**Solution:**
```bash
# 1. Check current dimension
curl http://localhost:6333/collections/rice_chunks

# 2. Update settings.yaml
models:
  embedding:
    dimension: 2560  # Match model output!

# 3. Delete collection and restart
curl -X DELETE http://localhost:6333/collections/rice_chunks
make down && make up
```

### Cannot Connect to Services

**Problem:** Backend cannot reach Qdrant/Redis/Ollama.

**Solution:**
```yaml
# Use Docker service names (inside containers)
infrastructure:
  qdrant:
    url: "http://qdrant:6333"  # NOT localhost!
  redis:
    url: "redis://redis:6379/0"

inference:
  ollama:
    base_url: "http://ollama:11434"
```

### Check Configuration at Runtime

```bash
# View all settings via API
curl http://localhost:8000/api/v1/settings | jq .

# Check specific setting
curl http://localhost:8000/api/v1/settings/models.embedding.dimension

# Check loaded config in container
docker exec deploy-backend-api-1 cat /app/backend/settings.yaml
```

---

## Configuration Best Practices

1. **Use settings.yaml for defaults** - Commit shared configuration
2. **Use .env for deployment** - Environment-specific overrides
3. **Use runtime config for experiments** - Quick A/B testing
4. **Never commit secrets** - Use `.env` (add to `.gitignore`)
5. **Document custom settings** - Add comments to `settings.yaml`
6. **Test configuration changes** - Validate before deploying
7. **Monitor resource usage** - Adjust batch sizes and timeouts based on metrics

---

## Summary

**Configuration Hierarchy:**
```text
Runtime (Redis) > Environment Variables (.env) > Settings File (settings.yaml)
```

**Key Files:**
- `backend/settings.yaml` - Base configuration
- `deploy/.env` - Environment overrides
- `~/.config/ricesearch/config.json` - CLI settings

**Common Tasks:**
```bash
# Change embedding model
# 1. Update settings.yaml (dimension + model name)
# 2. Delete Qdrant collection
# 3. Restart services
# 4. Re-index files

# Disable a feature
# 1. Set enabled: false in settings.yaml
# 2. Restart services

# Performance tuning
# 1. Adjust batch_size, ttl_seconds, precision
# 2. Monitor metrics: curl http://localhost:8000/metrics
```

For more details, see:
- [Architecture](architecture.md) - How components use configuration
- [Development](development.md) - Testing configuration changes
- [Deployment](deployment.md) - Production configuration
- [Troubleshooting](troubleshooting.md) - Configuration issues

---

**[Back to Documentation Index](README.md)**
