# CLI Reference

## Overview

Two binaries are provided:

| Binary | Purpose | Location |
|--------|---------|----------|
| `rice-search` | CLI client for indexing/searching | `cmd/rice-search/` |
| `rice-search-server` | HTTP server with Web UI | `cmd/rice-search-server/` |

```bash
# Server (start the HTTP server + Web UI)
rice-search-server [flags]

# CLI client (connects to running server)
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

### rice-search-server (Server Binary)

Run the Rice Search server with gRPC, HTTP, and Web UI.

```bash
rice-search-server [flags]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--http-port` | `RICE_PORT` | `8080` | HTTP/Web UI port |
| `--grpc-port` | `RICE_GRPC_PORT` | `50051` | gRPC API port |
| `--host` | `RICE_HOST` | `0.0.0.0` | Bind address |
| `--qdrant` | `QDRANT_URL` | `http://localhost:6333` | Qdrant URL |
| `--unix-socket` | - | - | Unix socket path (non-Windows only) |
| `--config`, `-c` | `CONFIG_FILE` | - | Config file path |
| `--verbose`, `-v` | - | `false` | Enable debug logging |

**Examples:**

```bash
# Default (HTTP on 8080, gRPC on 50051)
rice-search-server

# Custom ports
rice-search-server --http-port 3000 --grpc-port 9000

# With custom Qdrant URL
rice-search-server --qdrant http://qdrant:6333

# Verbose mode with config file
rice-search-server -v --config rice-search.yaml

# Version info
rice-search-server version
```

---

### api

> **⚠️ NOT IMPLEMENTED**: Microservices mode is not available. Use `rice-search-server` for monolith mode.

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

> **⚠️ NOT IMPLEMENTED**: Standalone ML service mode is not available. ML is embedded in `rice-search-server`.

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

Search indexed code from command line.

```bash
rice-search search [flags] <query>
```

| Flag | Default | Description |
|------|---------|-------------|
| `--store`, `-s` | `default` | Target store |
| `--top-k`, `-k` | `20` | Number of results to return |
| `--no-rerank` | false | Disable neural reranking |
| `--content` | false | Include content in results |
| `--path-prefix` | - | Filter by path prefix |
| `--lang` | - | Filter by language (comma-separated) |

**Examples:**

```bash
# Basic search
rice-search search "authentication handler"

# More results
rice-search search "error handling" -k 50

# Search specific store
rice-search search "database connection" -s myproject

# Filter by path and language
rice-search search "func main" --path-prefix cmd/ --lang go

# Disable reranking for speed
rice-search search "user login" --no-rerank

# JSON output
rice-search search "api endpoint" --format json
```

**Output (text):**

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

### web

> **⚠️ NOT IMPLEMENTED**: Standalone web service mode is not available. Web UI is embedded in `rice-search-server`.
>
> **Note**: Access the Web UI at `http://localhost:8080` after running `rice-search-server`.

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

### health

Check server health status.

```bash
rice-search health [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `text` | Output format (text, json) |

**Examples:**

```bash
# Basic health check
rice-search health

# JSON output
rice-search health --format json
```

**Output (text):**

```
✓ Server: healthy (v1.0.0)

Components:
  ✓ qdrant     healthy
  ✓ ml         healthy
  ✓ index      healthy
  ✓ search     healthy
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

#### models check

Check installed models and their status.

```bash
rice-search models check
```

**Output:**

```
MODEL STATUS:

✓ Jina Code Embeddings v3 (embed)
  Default model for embed
  GPU acceleration: enabled

✓ Jina Reranker v2 (rerank)
  Default model for rerank
  GPU acceleration: enabled

✗ SPLADE++ (sparse) - not downloaded

Total: 2/3 models downloaded (2.1 GB)

Run 'rice-search models download' to download missing models
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
rice-search-server --config rice-search.yaml
```
