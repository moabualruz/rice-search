# CLI Reference

## Overview

Single binary with subcommands for different modes.

```bash
rice-search [command] [flags]
```

---

## Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--config` | `CONFIG_FILE` | - | Config file path |
| `--log-level` | `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `--log-format` | `LOG_FORMAT` | `text` | Log format (text, json) |

---

## Commands

### serve

Run all services in monolith mode.

```bash
rice-search serve [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `PORT` | `8080` | HTTP port |
| `--host` | `HOST` | `0.0.0.0` | Bind address |
| `--qdrant-url` | `QDRANT_URL` | `http://localhost:6333` | Qdrant URL |
| `--models-dir` | `MODELS_DIR` | `./models` | Models directory |
| `--data-dir` | `DATA_DIR` | `./data` | Data directory |
| `--device` | `DEVICE` | `auto` | Device (auto, cpu, cuda) |

**Examples:**

```bash
# Default (port 8080, auto device detection)
rice-search serve

# Custom port, force CPU
rice-search serve --port 3000 --device cpu

# Production with all options
rice-search serve \
    --port 8080 \
    --qdrant-url http://qdrant:6333 \
    --models-dir /models \
    --device cuda \
    --log-level info \
    --log-format json
```

---

### api

Run API service only (microservices mode).

```bash
rice-search api [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `PORT` | `8080` | HTTP port |
| `--bus` | `EVENT_BUS` | `memory` | Event bus URL |
| `--ml-url` | `ML_URL` | - | ML service URL (if remote) |

**Examples:**

```bash
# Standalone with Kafka bus
rice-search api --port 8080 --bus kafka://localhost:9092

# With remote ML service
rice-search api --port 8080 --ml-url http://ml-server:8081
```

---

### ml

Run ML service only (microservices mode).

```bash
rice-search ml [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `PORT` | `8081` | HTTP port |
| `--bus` | `EVENT_BUS` | `memory` | Event bus URL |
| `--models-dir` | `MODELS_DIR` | `./models` | Models directory |
| `--device` | `DEVICE` | `auto` | Device (auto, cpu, cuda) |
| `--load-mode` | `GPU_LOAD_MODE` | `all` | Model loading (all, ondemand, lru) |

**Examples:**

```bash
# GPU server
rice-search ml --port 8081 --device cuda

# With Kafka bus
rice-search ml --port 8081 --bus kafka://localhost:9092 --device cuda

# Limited VRAM
rice-search ml --port 8081 --device cuda --load-mode ondemand
```

---

### search

Run search service only (microservices mode).

```bash
rice-search search [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `PORT` | `8082` | HTTP port |
| `--bus` | `EVENT_BUS` | `memory` | Event bus URL |
| `--qdrant-url` | `QDRANT_URL` | `http://localhost:6333` | Qdrant URL |

**Examples:**

```bash
# Standalone
rice-search search --port 8082 --qdrant-url http://qdrant:6333

# With Kafka
rice-search search --bus kafka://localhost:9092 --qdrant-url http://qdrant:6333
```

---

### web

Run web UI service only (microservices mode).

```bash
rice-search web [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--port` | `PORT` | `3000` | HTTP port |
| `--api-url` | `API_URL` | `http://localhost:8080` | API service URL |

**Examples:**

```bash
# Connect to local API
rice-search web --port 3000 --api-url http://localhost:8080

# Connect to remote API
rice-search web --port 3000 --api-url http://api-server:8080
```

---

### models

Manage ML models.

#### models download

Download models from registry.

```bash
rice-search models download [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--dir` | `./models` | Download directory |
| `--model` | all | Specific model (embed, sparse, rerank) |
| `--force` | false | Overwrite existing |

**Examples:**

```bash
# Download all models
rice-search models download

# Download specific model
rice-search models download --model embed

# Force re-download
rice-search models download --force
```

#### models list

List available and downloaded models.

```bash
rice-search models list
```

**Output:**

```
AVAILABLE MODELS:
  embed     jina-embeddings-v3      1536 dims   600MB   ✓ downloaded
  sparse    splade-pp-en-v1         variable    250MB   ✓ downloaded
  rerank    jina-reranker-v2        score       500MB   ✗ not downloaded
```

#### models info

Show model information.

```bash
rice-search models info <model>
```

**Output:**

```
Model: jina-embeddings-v3
Type: Dense Embedding
Dimensions: 1536
Max Tokens: 8192
Size: 600MB
Format: ONNX (FP16)
Downloaded: Yes
Path: ./models/jina-embeddings-v3.onnx
```

---

### index

Index files from command line.

```bash
rice-search index [flags] <paths...>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--store` | `default` | Target store |
| `--api-url` | `http://localhost:8080` | API URL |
| `--recursive` | true | Recurse directories |
| `--include` | `*` | Include glob pattern |
| `--exclude` | - | Exclude glob pattern |
| `--force` | false | Force reindex |

**Examples:**

```bash
# Index directory
rice-search index ./src

# Index specific files
rice-search index ./src/main.go ./src/auth.go

# Index with filters
rice-search index ./src --include "*.go" --exclude "*_test.go"

# Index to specific store
rice-search index ./src --store my-project
```

---

### query

Search from command line.

```bash
rice-search query [flags] <query>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--store` | `default` | Target store |
| `--api-url` | `http://localhost:8080` | API URL |
| `--top-k` | `10` | Results to return |
| `--no-rerank` | false | Disable reranking |
| `--format` | `table` | Output format (table, json) |

**Examples:**

```bash
# Basic search
rice-search query "authentication handler"

# More results, JSON output
rice-search query "error handling" --top-k 20 --format json

# Search specific store
rice-search query "user login" --store my-project
```

**Output (table):**

```
RESULTS (5 matches in 65ms)

#1 [0.92] src/auth/handler.go:45-72
   func Authenticate(ctx context.Context) error {
       // validate user credentials
       ...

#2 [0.85] src/auth/token.go:12-34
   func ValidateToken(token string) (*Claims, error) {
       ...
```

---

### stores

Manage stores.

#### stores list

```bash
rice-search stores list
```

**Output:**

```
STORES:
  default      150 docs   890 chunks   1.2 MB
  my-project    45 docs   234 chunks   456 KB
```

#### stores create

```bash
rice-search stores create <name> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--display-name` | - | Display name |
| `--description` | - | Description |

#### stores delete

```bash
rice-search stores delete <name> [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | false | Skip confirmation |

#### stores stats

```bash
rice-search stores stats <name>
```

---

### version

Show version information.

```bash
rice-search version
```

**Output:**

```
rice-search version 1.0.0
  Git commit: abc123def
  Build time: 2025-12-29T01:00:00Z
  Go version: go1.23.0
  OS/Arch:    linux/amd64
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Invalid arguments |
| 3 | Configuration error |
| 4 | Connection error |
| 5 | Authentication error |

---

## Configuration File

Optional YAML config file.

```yaml
# rice-search.yaml

server:
  port: 8080
  host: 0.0.0.0

qdrant:
  url: http://localhost:6333

models:
  dir: ./models
  device: cuda
  load_mode: all

search:
  default_top_k: 20
  sparse_weight: 0.5
  dense_weight: 0.5
  rerank_top_k: 30

logging:
  level: info
  format: json
```

Load with:

```bash
rice-search serve --config rice-search.yaml
```
