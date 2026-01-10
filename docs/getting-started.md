# Getting Started with Rice Search

This guide will help you install, configure, and run Rice Search for the first time.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [First Run](#first-run)
- [Indexing Your First Files](#indexing-your-first-files)
- [Running Your First Search](#running-your-first-search)
- [Stopping and Restarting](#stopping-and-restarting)
- [Next Steps](#next-steps)

---

## Prerequisites

### Required

- **Docker Desktop** (version 20.10+)
  - [Download for Windows/Mac](https://www.docker.com/products/docker-desktop)
  - For Linux: [Install Docker Engine](https://docs.docker.com/engine/install/)
- **Docker Compose** (included with Docker Desktop)
- **10GB free disk space** (for Docker images and vector database)
- **8GB RAM minimum** (16GB recommended for optimal performance)

### Optional

- **NVIDIA GPU** (for faster inference)
  - Requires [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/install-guide.html)
  - Automatically detected and used by Ollama service
- **Git** (for cloning the repository)

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| RAM | 8GB | 16GB+ |
| Disk Space | 10GB | 20GB+ |
| CPU | 4 cores | 8 cores+ |
| GPU | None (CPU mode) | NVIDIA GPU (faster inference) |

---

## Installation

### 1. Clone the Repository

```bash
git clone https://github.com/yourusername/rice-search.git
cd rice-search
```

### 2. Verify Docker is Running

```bash
# Check Docker version
docker --version
# Expected output: Docker version 20.10.x or higher

# Check Docker Compose
docker compose version
# Expected output: Docker Compose version v2.x.x or higher
```

### 3. Review Configuration (Optional)

The default configuration works out of the box. If you want to customize settings:

```bash
# View default settings
cat backend/settings.yaml

# Settings you might want to change:
# - models.embedding.dimension (default: 2560)
# - search.hybrid.rrf_k (default: 60)
# - inference.ollama.keep_alive (default: 5m)
```

See [Configuration Guide](configuration.md) for detailed options.

---

## First Run

### 1. Start All Services

```bash
# Start Rice Search (this will download Docker images on first run)
make up

# Expected output:
# [+] Building...
# [+] Running 8/8
#  ✔ Network rice-net            Created
#  ✔ Container deploy-qdrant-1   Started
#  ✔ Container deploy-redis-1    Started
#  ✔ Container deploy-minio-1    Started
#  ✔ Container deploy-ollama-1   Started
#  ✔ Container deploy-tantivy-1  Started
#  ✔ Container deploy-backend-api-1    Started
#  ✔ Container deploy-backend-worker-1 Started
#  ✔ Container deploy-frontend-1       Started
```

**First run will take 5-10 minutes:**

- Docker images download (~5GB)
- Ollama downloads embedding model (~2GB)
- Services initialize

### 2. Verify Services Are Running

```bash
# Check service status
docker compose -f deploy/docker-compose.yml ps

# Expected output (all services "Up" and "healthy"):
# NAME                      IMAGE                   STATUS
# deploy-backend-api-1      deploy-backend-api      Up
# deploy-backend-worker-1   deploy-backend-worker   Up
# deploy-frontend-1         deploy-frontend         Up
# deploy-qdrant-1           qdrant/qdrant:latest    Up (healthy)
# deploy-redis-1            redis:8-alpine          Up
# deploy-ollama-1           ollama/ollama:latest    Up (healthy)
# deploy-tantivy-1          deploy-tantivy          Up (healthy)
# deploy-minio-1            minio/minio             Up
```

### 3. Access the Web Interface

Open your browser to:
- **Frontend UI**: <http://localhost:3000>
- **API Documentation**: <http://localhost:8000/docs>
- **Health Check**: <http://localhost:8000/health>

**Expected health check response:**
```json
{
  "status": "ok",
  "components": {
    "qdrant": {"status": "up", "collections": 0},
    "celery": {"status": "up", "last_task_id": "..."}
  }
}
```

### 4. View Logs (Optional)

```bash
# View all logs
make logs

# View specific service logs
make api-logs       # Backend API
make worker-logs    # Celery worker

# View frontend logs
docker compose -f deploy/docker-compose.yml logs -f frontend
```

---

## Indexing Your First Files

Rice Search needs to index your files before you can search them.

### Install the CLI Tool

```bash
# Navigate to backend directory
cd backend

# Install ricesearch CLI
pip install -e .

# Verify installation
ricesearch --version
# Expected output: ricesearch 0.1.0 (or similar)
```

### Index Files

**Option 1: Index a directory once**
```bash
# Index the backend code
ricesearch index ./backend

# Expected output:
# [INDEXING] ./backend/src/main.py
# [OK] ./backend/src/main.py
# [INDEXING] ./backend/src/core/config.py
# [OK] ./backend/src/core/config.py
# ...
# [INFO] Scan complete.
```

**Option 2: Watch for changes (recommended for development)**
```bash
# Watch backend directory for changes
ricesearch watch ./backend --org-id myproject

# Expected output:
# [INFO] Starting file watcher for: ./backend
# [INFO] Organization ID: myproject
# [INFO] Initial scan...
# [INDEXING] ./backend/src/main.py
# [OK] ./backend/src/main.py
# ...
# [INFO] Watching for changes... (Ctrl+C to stop)
```

**What gets indexed:**

- Supported file types: `.py`, `.js`, `.ts`, `.go`, `.rs`, `.java`, `.cpp`, `.md`, `.txt`
- Respects `.gitignore` and `.riceignore` patterns
- Files are chunked intelligently (AST-aware for code)
- Full file paths are stored and searchable

### Verify Indexing

```bash
# Check Qdrant collection
curl -s http://localhost:6333/collections/rice_chunks | grep points_count
# Expected output: "points_count":X (where X > 0)

# Check via API
curl http://localhost:8000/api/v1/files/list
# Expected: JSON list of indexed files
```

---

## Running Your First Search

### Via Web UI (Easiest)

1. Open <http://localhost:3000>
2. Enter a query in the search box (e.g., "configuration settings")
3. Click **Search** or press Enter
4. View results with:
   - Full file paths
   - Relevance scores
   - Code highlighting
   - Expandable file previews

**Search Modes:**

- **Search**: Returns ranked results (default)
- **RAG**: Returns AI-generated answer with sources

### Via CLI

```bash
# Basic search
ricesearch search "configuration" --limit 5

# Expected output:
# 1. F:/work/rice-search/backend/src/core/config.py (score: 0.85)
#    Configuration module - wraps SettingsManager for...
#
# 2. F:/work/rice-search/backend/settings.yaml (score: 0.78)
#    app:
#      name: Rice Search
#      version: 1.0.0
# ...

# Search for specific files
ricesearch search "Dockerfile"

# Search with more results
ricesearch search "authentication" --limit 20
```

### Via API

```bash
# Basic search
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "user authentication",
    "limit": 5,
    "mode": "search"
  }'

# RAG search (AI-generated answer)
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{
    "query": "How does authentication work?",
    "limit": 10,
    "mode": "rag"
  }'
```

**Expected response:**
```json
{
  "mode": "search",
  "results": [
    {
      "id": "chunk-id-123",
      "chunk_id": "chunk-id-123",
      "score": 0.85,
      "rerank_score": 6.45,
      "text": "Configuration module content...",
      "full_path": "F:/work/rice-search/backend/src/core/config.py",
      "filename": "config.py",
      "file_path": "F:/work/rice-search/backend/src/core/config.py",
      "org_id": "public",
      "retriever_scores": {
        "bm25": 12.5,
        "splade": 18.3,
        "bm42": 0.75
      }
    }
  ],
  "retrievers": {
    "bm25": true,
    "splade": true,
    "bm42": true
  }
}
```

---

## Stopping and Restarting

### Stop Services

```bash
# Stop all services
make down

# Expected output:
# [+] Running 9/9
#  ✔ Container deploy-frontend-1       Removed
#  ✔ Container deploy-backend-worker-1 Removed
#  ✔ Container deploy-backend-api-1    Removed
#  ✔ Container deploy-tantivy-1        Removed
#  ✔ Container deploy-ollama-1         Removed
#  ✔ Container deploy-minio-1          Removed
#  ✔ Container deploy-redis-1          Removed
#  ✔ Container deploy-qdrant-1         Removed
#  ✔ Network rice-net                  Removed
```

**Data is preserved** in `data/` directory:

- `data/qdrant/` - Vector database
- `data/tantivy/` - BM25 indexes
- `data/ollama/` - Model cache
- `data/redis/` - Task queue data

### Restart Services

```bash
# Restart (using existing data)
make up

# Services start in ~30 seconds (no downloads needed)
```

### Clean Restart

If you want to start fresh and remove all data:

```bash
# WARNING: This deletes all indexed data!
make down
rm -rf data/*
make up

# You'll need to re-index your files
```

---

## Next Steps

### Learn More

- **[CLI Guide](cli.md)** - Master the `ricesearch` command
- **[Configuration](configuration.md)** - Customize settings
- **[Architecture](architecture.md)** - Understand how it works
- **[API Reference](api.md)** - Explore the REST API

### Common Tasks

**Index more directories:**
```bash
ricesearch watch ./frontend --org-id frontend
ricesearch watch ./rust-tantivy --org-id rust
```

**Search with filters:**
```bash
# Search only specific organization
curl -X POST http://localhost:8000/api/v1/search/query \
  -d '{"query": "config", "org_id": "backend"}'
```

**Enable enterprise features:**
```bash
# Start with Keycloak (SSO) and Jaeger (tracing)
make up-enterprise
```

**Monitor services:**
```bash
# Real-time logs
make logs

# Check metrics
curl http://localhost:8000/metrics
```

---

## Troubleshooting Quick Reference

### Services won't start
```bash
# Check Docker is running
docker ps

# Check disk space
df -h

# View error logs
docker compose -f deploy/docker-compose.yml logs
```

### Port already in use
```bash
# Windows: find what's using port 8000
netstat -ano | findstr :8000
taskkill /F /PID <PID>

# Linux/macOS
lsof -i :8000
kill -9 <PID>
```

### Ollama model download stuck
```bash
# Check Ollama logs
docker compose -f deploy/docker-compose.yml logs ollama

# Manually pull model
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen3-embedding:4b
```

### Search returns no results
```bash
# Verify files are indexed
curl http://localhost:8000/api/v1/files/list

# Check Qdrant collection
curl http://localhost:6333/collections/rice_chunks

# Re-index
ricesearch index ./backend
```

For detailed troubleshooting, see [Troubleshooting Guide](troubleshooting.md).

---

## Summary

You've successfully:

- ✅ Installed Rice Search
- ✅ Started all services
- ✅ Indexed your first files
- ✅ Ran your first searches
- ✅ Learned basic operations

**Quick Commands Reference:**
```bash
make up                  # Start services
make down                # Stop services
make logs                # View logs
ricesearch index ./dir   # Index directory
ricesearch search "query" # Search
```

Welcome to Rice Search! 🎉

---

**[Back to Documentation Index](README.md)**
