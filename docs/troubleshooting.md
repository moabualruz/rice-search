# Troubleshooting Guide

Common issues and solutions for Rice Search deployment, development, and operations.

## Table of Contents

- [Installation & Setup Issues](#installation--setup-issues)
- [Service Startup Problems](#service-startup-problems)
- [Search Issues](#search-issues)
- [Indexing Problems](#indexing-problems)
- [Performance Issues](#performance-issues)
- [Docker & Container Issues](#docker--container-issues)
- [Model & Inference Issues](#model--inference-issues)
- [Database Issues](#database-issues)
- [Network & Connectivity](#network--connectivity)
- [Development Issues](#development-issues)

---

## Installation & Setup Issues

### Docker Not Installed

**Symptoms:**
```bash
make up
# Error: docker: command not found
```

**Solution:**
```bash
# Install Docker Desktop (Windows/Mac)
# Visit: https://www.docker.com/products/docker-desktop

# Linux - Install Docker Engine
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Verify installation
docker --version
docker compose version
```

### Port Already in Use

**Symptoms:**
```
Error: bind: address already in use (0.0.0.0:8000)
Error: bind: address already in use (0.0.0.0:3000)
```

**Solution:**

**Windows:**
```powershell
# Find process using port
netstat -ano | findstr :8000

# Kill process by PID
taskkill /F /PID <PID>
```

**Linux/macOS:**
```bash
# Find process using port
lsof -i :8000

# Kill process
kill -9 <PID>

# Or use Docker-specific port
# Edit deploy/docker-compose.yml:
services:
  backend-api:
    ports:
      - "8001:8000"  # Use different external port
```

### Insufficient Disk Space

**Symptoms:**
```
Error: no space left on device
```

**Solution:**
```bash
# Check disk space
df -h

# Clean Docker cache
docker system prune -a --volumes

# Check Docker disk usage
docker system df

# Remove unused volumes
docker volume ls
docker volume rm <volume_name>
```

### Out of Memory

**Symptoms:**
```
services crashed: ollama exited with code 137
```

**Solution:**
```bash
# Check available RAM
free -h  # Linux
vm_stat  # macOS

# Increase Docker memory limit
# Docker Desktop → Settings → Resources → Memory
# Set to at least 8GB (16GB recommended)

# Or disable memory-intensive features
# In backend/settings.yaml:
models:
  sparse:
    enabled: false  # Disable SPLADE
  reranker:
    enabled: false  # Disable reranking
```

---

## Service Startup Problems

### Services Won't Start

**Symptoms:**
```bash
make up
# Some services fail to start or repeatedly restart
```

**Solution:**
```bash
# 1. Check service logs
docker compose -f deploy/docker-compose.yml logs

# 2. Check specific service
docker compose -f deploy/docker-compose.yml logs backend-api
docker compose -f deploy/docker-compose.yml logs ollama

# 3. Check service status
docker compose -f deploy/docker-compose.yml ps

# 4. Restart individual service
docker compose -f deploy/docker-compose.yml restart backend-api

# 5. Full restart
make down
make up
```

### Ollama Model Download Stuck

**Symptoms:**
```
ollama: pulling model qwen3-embedding:4b... (stuck at 0%)
```

**Solution:**
```bash
# 1. Check Ollama logs
docker compose -f deploy/docker-compose.yml logs -f ollama

# 2. Manually pull model
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen3-embedding:4b

# 3. Check disk space
df -h

# 4. Check network connectivity
docker compose -f deploy/docker-compose.yml exec ollama curl -I https://ollama.ai

# 5. Restart Ollama service
docker compose -f deploy/docker-compose.yml restart ollama
```

### Backend API Won't Start

**Symptoms:**
```
backend-api exited with code 1
```

**Solution:**
```bash
# 1. Check logs for error details
docker compose -f deploy/docker-compose.yml logs backend-api | tail -50

# Common errors:

# Error: Cannot connect to Qdrant
# → Wait for Qdrant to be healthy:
docker compose -f deploy/docker-compose.yml ps qdrant
# Status should be "Up (healthy)"

# Error: Cannot connect to Redis
# → Check Redis is running:
docker compose -f deploy/docker-compose.yml ps redis

# Error: Module not found
# → Rebuild backend image:
docker compose -f deploy/docker-compose.yml build backend-api
docker compose -f deploy/docker-compose.yml up -d backend-api

# 2. Check environment variables
docker compose -f deploy/docker-compose.yml config | grep QDRANT_URL
```

### Worker Not Processing Tasks

**Symptoms:**
```bash
# Files indexed via CLI but never complete
ricesearch watch ./backend
# Shows "INDEXING" but never "OK"
```

**Solution:**
```bash
# 1. Check worker is running
docker compose -f deploy/docker-compose.yml ps backend-worker

# 2. Check worker logs
docker compose -f deploy/docker-compose.yml logs -f backend-worker

# 3. Check Redis connection
docker compose -f deploy/docker-compose.yml exec redis redis-cli ping
# Expected: PONG

# 4. Check Celery queue
docker compose -f deploy/docker-compose.yml exec redis redis-cli LLEN celery

# 5. Restart worker
docker compose -f deploy/docker-compose.yml restart backend-worker
```

---

## Search Issues

### No Search Results

**Symptoms:**
```bash
ricesearch search "authentication"
# Returns: No results found
```

**Solution:**
```bash
# 1. Verify files are indexed
curl http://localhost:8000/api/v1/files/list
# Should return JSON array of files

# 2. Check Qdrant collection
curl http://localhost:6333/collections/rice_chunks
# Check "points_count" > 0

# 3. Check Tantivy index
curl http://localhost:3002/health
# Should return "ok"

# 4. Re-index files
ricesearch watch ./backend

# 5. Verify backend is reachable
curl http://localhost:8000/health
```

### Search Returns Empty Array

**Symptoms:**
```json
{
  "mode": "search",
  "results": [],
  "retrievers": {"bm25": true, "splade": true, "bm42": true}
}
```

**Solution:**
```bash
# 1. Check if query is too specific
# Try broader query:
ricesearch search "config"

# 2. Verify retrievers are enabled
curl http://localhost:8000/api/v1/settings?prefix=search.hybrid

# 3. Check Qdrant has vectors
curl -X POST http://localhost:6333/collections/rice_chunks/points/scroll \
  -H "Content-Type: application/json" \
  -d '{"limit": 10}'

# 4. Check embedding service
curl http://localhost:11434/api/tags
# Should list qwen3-embedding model
```

### Search is Slow

**Symptoms:**
- Search takes >5 seconds to return results
- Timeout errors

**Solution:**
```bash
# 1. Check which retriever is slow
# View backend API logs during search:
docker compose -f deploy/docker-compose.yml logs -f backend-api

# 2. Disable slow retrievers temporarily
# In settings.yaml:
search:
  hybrid:
    use_bm25: true
    use_splade: false  # Try disabling SPLADE
    use_bm42: false

# 3. Reduce reranking candidates
models:
  reranker:
    top_k: 20  # Reduce from 50

# 4. Check Qdrant performance
curl http://localhost:6333/collections/rice_chunks
# Check "optimizer_status"

# 5. Increase timeouts
models:
  embedding:
    timeout: 120
search:
  bm25:
    timeout: 30
```

### Duplicate Results

**Symptoms:**
- Same file appears multiple times in results

**Solution:**
```bash
# This should be automatically handled by deduplication
# If you see duplicates:

# 1. Check deduplication is enabled (should be by default)
# View retriever.py logs

# 2. Re-index the file to clean up old chunks
ricesearch watch ./path/to/file

# 3. Check Qdrant for duplicate chunks
curl -X POST http://localhost:6333/collections/rice_chunks/points/scroll \
  -H "Content-Type: application/json" \
  -d '{
    "filter": {
      "must": [
        {"key": "full_path", "match": {"value": "F:/path/to/file.py"}}
      ]
    },
    "limit": 100
  }'
# Should see ~5-10 chunks per file, not 20+
```

---

## Indexing Problems

### Files Not Indexing

**Symptoms:**
```bash
ricesearch watch ./backend
# Shows: Indexed 0 files, skipped 45
```

**Solution:**
```bash
# 1. Check .riceignore patterns
cat .riceignore
# Ensure your files aren't ignored

# 2. Check file types are supported
# Only .py, .js, .ts, .go, .rs, .md, etc. are indexed

# 3. Check backend worker is running
docker compose -f deploy/docker-compose.yml ps backend-worker

# 4. Check worker logs for errors
docker compose -f deploy/docker-compose.yml logs backend-worker | grep ERROR

# 5. Test API directly
curl -X POST http://localhost:8000/api/v1/ingest/file \
  -F "file=@test.py" \
  -F "org_id=public"
```

### Indexing Fails with Errors

**Symptoms:**
```
[ERROR] Failed to index file: F:/work/test.py
```

**Solution:**
```bash
# 1. Check specific error in worker logs
docker compose -f deploy/docker-compose.yml logs backend-worker | grep test.py

# Common errors:

# Error: Embedding failed
# → Check Ollama is running and model is loaded:
curl http://localhost:11434/api/tags

# Error: Qdrant connection refused
# → Check Qdrant is healthy:
docker compose -f deploy/docker-compose.yml ps qdrant

# Error: AST parsing failed
# → Disable AST for that file type or globally:
# settings.yaml:
ast:
  enabled: false

# Error: File too large
# → Skip large files in .riceignore:
echo "*.log" >> .riceignore
echo "large-file.json" >> .riceignore
```

### Watch Command Not Detecting Changes

**Symptoms:**
- Modified file but watch doesn't re-index

**Solution:**
```bash
# 1. Check watchdog is working
# Touch a file and watch logs:
touch test.py
# Should see "[INDEXING] test.py"

# 2. Check file hash-based deduplication
# If content didn't change, watch skips re-indexing
# Actually modify file content

# 3. Restart watch command
# Ctrl+C and restart:
ricesearch watch ./backend

# 4. Use manual index instead
ricesearch watch ./backend --no-initial
# Then modify file
```

---

## Performance Issues

### High Memory Usage

**Symptoms:**
```bash
docker stats
# Shows ollama or backend using >8GB RAM
```

**Solution:**
```bash
# 1. Enable model auto-unloading
# settings.yaml:
model_management:
  auto_unload: true
  ttl_seconds: 60  # Unload after 1 minute idle

# 2. Disable memory-intensive features
models:
  sparse:
    enabled: false
  reranker:
    enabled: false

# 3. Reduce batch sizes
indexing:
  batch_size: 50  # Reduce from 100
models:
  sparse:
    batch_size: 16  # Reduce from 32

# 4. Use CPU instead of GPU for SPLADE
models:
  sparse:
    device: "cpu"
    precision: "fp32"

# 5. Restart services to clear memory
docker compose -f deploy/docker-compose.yml restart ollama
```

### High CPU Usage

**Symptoms:**
- Backend or worker using 100% CPU
- System becomes slow

**Solution:**
```bash
# 1. Check what's causing high CPU
docker stats

# 2. Reduce worker concurrency
# Edit deploy/docker-compose.yml:
services:
  backend-worker:
    command: celery -A src.tasks worker -c 2  # Reduce from 4

# 3. Disable CPU-heavy features temporarily
search:
  query_analysis:
    enabled: false

# 4. Reduce indexing batch size
indexing:
  batch_size: 50
```

### Slow Embedding Generation

**Symptoms:**
- Indexing takes minutes per file

**Solution:**
```bash
# 1. Check if GPU is being used
docker compose -f deploy/docker-compose.yml exec ollama nvidia-smi
# Should show ollama process using GPU

# 2. Enable GPU for SPLADE
models:
  sparse:
    device: "cuda"
    precision: "fp16"

# 3. Increase batch sizes
indexing:
  batch_size: 200
models:
  sparse:
    batch_size: 64

# 4. Reduce chunk size
indexing:
  chunk_size: 500  # Reduce from 1000
  chunk_overlap: 100
```

---

## Docker & Container Issues

### Container Keeps Restarting

**Symptoms:**
```bash
docker compose ps
# Shows: Restarting (exit code 137)
```

**Solution:**
```bash
# Exit code 137 = Out of Memory (OOM killed)

# 1. Increase Docker memory limit
# Docker Desktop → Settings → Resources → Memory: 16GB

# 2. Check container logs before crash
docker compose -f deploy/docker-compose.yml logs --tail=100 <service>

# 3. Reduce memory usage (see Performance Issues)
```

### Cannot Remove Container

**Symptoms:**
```bash
make down
# Error: container is in use
```

**Solution:**
```bash
# 1. Force remove
docker compose -f deploy/docker-compose.yml down --remove-orphans

# 2. Stop and remove manually
docker stop $(docker ps -aq)
docker rm $(docker ps -aq)

# 3. Prune system
docker system prune -f
```

### Volume Permission Issues

**Symptoms:**
```
Error: Permission denied: /data/qdrant
```

**Solution:**
```bash
# Linux - Fix permissions
sudo chown -R $USER:$USER data/

# Or run with sudo
sudo docker compose -f deploy/docker-compose.yml up

# Windows/Mac - Usually not an issue
# Ensure Docker has permission to mount drives
# Docker Desktop → Settings → Resources → File Sharing
```

---

## Model & Inference Issues

### Model Not Found

**Symptoms:**
```
Error: model 'qwen3-embedding:4b' not found
```

**Solution:**
```bash
# 1. Pull model manually
docker compose -f deploy/docker-compose.yml exec ollama ollama pull qwen3-embedding:4b

# 2. List available models
docker compose -f deploy/docker-compose.yml exec ollama ollama list

# 3. Check Ollama logs
docker compose -f deploy/docker-compose.yml logs ollama

# 4. Restart Ollama
docker compose -f deploy/docker-compose.yml restart ollama
```

### Dimension Mismatch Error

**Symptoms:**
```
RuntimeError: vector dimension mismatch (expected 2560, got 768)
```

**Solution:**
```bash
# 1. Check current embedding model dimension
docker compose -f deploy/docker-compose.yml exec ollama ollama show qwen3-embedding:4b
# qwen3-embedding:4b = 2560 dims

# 2. Update settings.yaml to match
models:
  embedding:
    dimension: 2560  # Must match model!

# 3. Delete and recreate Qdrant collection
curl -X DELETE http://localhost:6333/collections/rice_chunks

# 4. Restart backend
docker compose -f deploy/docker-compose.yml restart backend-api backend-worker

# 5. Re-index files
ricesearch watch ./backend
```

### Embedding Timeout

**Symptoms:**
```
Error: Embedding request timed out after 60 seconds
```

**Solution:**
```bash
# 1. Increase timeout in settings.yaml
models:
  embedding:
    timeout: 180

inference:
  ollama:
    timeout: 180

# 2. Restart backend
docker compose -f deploy/docker-compose.yml restart backend-api

# 3. Check Ollama is responsive
curl http://localhost:11434/api/tags

# 4. Reduce batch size
indexing:
  batch_size: 50
```

---

## Database Issues

### Qdrant Connection Refused

**Symptoms:**
```
Error: [Errno 111] Connection refused (qdrant:6333)
```

**Solution:**
```bash
# 1. Check Qdrant is running
docker compose -f deploy/docker-compose.yml ps qdrant

# 2. Check Qdrant health
curl http://localhost:6333/health
# Expected: {"status":"ok"}

# 3. Check Qdrant logs
docker compose -f deploy/docker-compose.yml logs qdrant

# 4. Restart Qdrant
docker compose -f deploy/docker-compose.yml restart qdrant

# 5. Verify Qdrant URL in config
# Should be "http://qdrant:6333" (Docker network name)
# NOT "http://localhost:6333"
```

### Redis Connection Failed

**Symptoms:**
```
Error: Redis connection error
```

**Solution:**
```bash
# 1. Check Redis is running
docker compose -f deploy/docker-compose.yml ps redis

# 2. Test Redis connection
docker compose -f deploy/docker-compose.yml exec redis redis-cli ping
# Expected: PONG

# 3. Check Redis URL in config
echo $REDIS_URL
# Should be: redis://redis:6379/0

# 4. Clear Redis data if corrupted
docker compose -f deploy/docker-compose.yml exec redis redis-cli FLUSHDB

# 5. Restart Redis
docker compose -f deploy/docker-compose.yml restart redis
```

### Qdrant Collection Not Found

**Symptoms:**
```
Error: Collection 'rice_chunks' not found
```

**Solution:**
```bash
# Collection is created automatically on first index
# If missing:

# 1. Index a file to trigger creation
ricesearch watch ./backend

# 2. Or create manually via API
curl -X POST http://localhost:8000/api/v1/admin/ensure-collection

# 3. Verify collection exists
curl http://localhost:6333/collections/rice_chunks
```

---

## Network & Connectivity

### Cannot Access Frontend (http://localhost:3000)

**Symptoms:**
```
This site can't be reached
localhost refused to connect
```

**Solution:**
```bash
# 1. Check frontend is running
docker compose -f deploy/docker-compose.yml ps frontend

# 2. Check port mapping
docker compose -f deploy/docker-compose.yml ps | grep frontend
# Should show: 0.0.0.0:3000->3000/tcp

# 3. Check frontend logs
docker compose -f deploy/docker-compose.yml logs frontend

# 4. Restart frontend
docker compose -f deploy/docker-compose.yml restart frontend

# 5. Try explicit IP
http://127.0.0.1:3000
```

### API Returns CORS Error

**Symptoms:**
```
Access to fetch at 'http://localhost:8000' from origin 'http://localhost:3000'
has been blocked by CORS policy
```

**Solution:**
```bash
# 1. Add frontend origin to CORS
# settings.yaml:
server:
  cors_origins:
    - "http://localhost:3000"
    - "http://127.0.0.1:3000"

# 2. Restart backend
docker compose -f deploy/docker-compose.yml restart backend-api

# 3. Or use environment variable
# .env:
BACKEND_CORS_ORIGINS=["http://localhost:3000"]
```

### Services Can't Communicate

**Symptoms:**
```
Error: Could not resolve host: qdrant
```

**Solution:**
```bash
# Services must use Docker network names, not localhost

# ✅ Correct:
QDRANT_URL=http://qdrant:6333
REDIS_URL=redis://redis:6379/0
OLLAMA_BASE_URL=http://ollama:11434

# ❌ Wrong:
QDRANT_URL=http://localhost:6333

# Check docker network
docker network ls
docker network inspect deploy_rice-net
```

---

## Development Issues

### Tests Failing

**Symptoms:**
```bash
pytest
# Multiple test failures
```

**Solution:**
```bash
# 1. Ensure services are running
make up

# 2. Check test configuration
cat backend/pytest.ini

# 3. Run specific test to isolate issue
pytest tests/test_search_services.py -v

# 4. Check test dependencies
cd backend
pip install -e .[dev]

# 5. Clear pytest cache
rm -rf backend/.pytest_cache
```

### Import Errors in Development

**Symptoms:**
```python
ImportError: No module named 'src'
```

**Solution:**
```bash
# 1. Install backend in editable mode
cd backend
pip install -e .

# 2. Verify PYTHONPATH
export PYTHONPATH="${PYTHONPATH}:$(pwd)/backend"

# 3. Check you're in correct directory
pwd  # Should be in rice-search/backend
```

### Hot Reload Not Working

**Symptoms:**
- Changed code but API doesn't update

**Solution:**
```bash
# 1. Check --reload flag is set
# deploy/docker-compose.yml:
services:
  backend-api:
    command: uvicorn src.main:app --reload

# 2. Restart service
docker compose -f deploy/docker-compose.yml restart backend-api

# 3. Check volume mounts are correct
docker compose -f deploy/docker-compose.yml config | grep volumes
```

---

## Getting Help

### Collecting Debug Information

When reporting issues, include:

```bash
# 1. System info
docker --version
docker compose version
python --version

# 2. Service status
docker compose -f deploy/docker-compose.yml ps

# 3. Service logs
docker compose -f deploy/docker-compose.yml logs > logs.txt

# 4. Configuration
cat backend/settings.yaml
env | grep -E "(QDRANT|REDIS|OLLAMA|EMBEDDING)"

# 5. Resource usage
docker stats --no-stream

# 6. Qdrant status
curl http://localhost:6333/collections/rice_chunks

# 7. Backend health
curl http://localhost:8000/health
```

### Enable Debug Logging

```yaml
# settings.yaml
app:
  debug: true  # Enable verbose logging
```

```bash
# Restart services
docker compose -f deploy/docker-compose.yml restart

# View debug logs
docker compose -f deploy/docker-compose.yml logs -f backend-api | grep DEBUG
```

---

## Summary

**Quick Diagnostics:**
```bash
# Check all services are up
docker compose -f deploy/docker-compose.yml ps

# Check logs for errors
docker compose -f deploy/docker-compose.yml logs | grep ERROR

# Test each service
curl http://localhost:8000/health      # Backend API
curl http://localhost:6333/health      # Qdrant
curl http://localhost:11434/api/tags   # Ollama
curl http://localhost:3002/health      # Tantivy
docker exec deploy-redis-1 redis-cli ping  # Redis

# Full restart
make down && make up
```

**Common Solutions:**
1. **Service won't start** → Check logs, verify dependencies are running
2. **Out of memory** → Increase Docker memory, disable features
3. **No search results** → Verify files are indexed, check Qdrant collection
4. **Slow search** → Disable retrievers, reduce rerank candidates
5. **Connection refused** → Use Docker service names, not localhost
6. **Dimension mismatch** → Update settings.yaml to match model, recreate collection

For more help:
- [Getting Started](getting-started.md) - Basic setup
- [Configuration](configuration.md) - Settings reference
- [CLI Guide](cli.md) - Command-line troubleshooting
- [Architecture](architecture.md) - Understanding system components
- [GitHub Issues](https://github.com/yourusername/rice-search/issues) - Report bugs

---

**[Back to Documentation Index](README.md)**
