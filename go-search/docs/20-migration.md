# Migration Guide

## Overview

Migrating from the current NestJS + Milvus + Infinity architecture to Go + Qdrant.

---

## Architecture Comparison

| Component | Current | Go Edition |
|-----------|---------|------------|
| API Server | NestJS (TypeScript) | Echo (Go) |
| Vector DB | Milvus (4 containers) | Qdrant (1 container) |
| BM25 Search | Tantivy CLI | SPLADE vectors |
| Embeddings | Infinity (Python) | ONNX Runtime (Go) |
| Reranking | Infinity (Python) | ONNX Runtime (Go) |
| Web UI | Next.js (separate) | Templ + HTMX (integrated) |
| Event Bus | Direct HTTP calls | Event-driven |

---

## Migration Strategy

### Option A: Big Bang (Recommended for small deployments)

```
1. Deploy Go Edition alongside current
2. Reindex all data
3. Switch traffic
4. Decommission old system
```

**Pros:** Simple, clean cut
**Cons:** Downtime during reindex

### Option B: Gradual (For large deployments)

```
1. Deploy Go Edition
2. Sync new writes to both systems
3. Gradually migrate reads
4. Stop writes to old system
5. Decommission
```

**Pros:** Zero downtime
**Cons:** Complex sync logic

---

## Pre-Migration Checklist

- [ ] Inventory all stores and document counts
- [ ] Export store configurations
- [ ] Document custom settings
- [ ] Backup Milvus data
- [ ] Test Go Edition in staging
- [ ] Prepare rollback plan

---

## Step-by-Step Migration

### Step 1: Deploy Go Edition Infrastructure

```bash
# Start Qdrant
docker run -d \
    -p 6333:6333 \
    -v ./data/qdrant:/qdrant/storage \
    qdrant/qdrant:v1.12.4

# Download models
./rice-search models download

# Start Go Edition (different port)
./rice-search serve --port 8090
```

### Step 2: Export Current Data

#### List Stores

```bash
# From current API
curl http://localhost:8080/v1/stores | jq '.[] | .name'
```

#### Export Store Metadata

```python
# scripts/export_stores.py
import requests
import json

stores = requests.get("http://localhost:8080/v1/stores").json()

with open("stores_backup.json", "w") as f:
    json.dump(stores, f, indent=2)

print(f"Exported {len(stores)} stores")
```

#### Export Indexed Files

```python
# scripts/export_files.py
import requests
import json

for store in stores:
    name = store["name"]
    files = requests.get(f"http://localhost:8080/v1/stores/{name}/index/files").json()
    
    with open(f"files_{name}.json", "w") as f:
        json.dump(files, f)
    
    print(f"Exported {len(files)} files from {name}")
```

### Step 3: Create Stores in Go Edition

```bash
# Create each store
for store in $(cat stores_backup.json | jq -r '.[].name'); do
    curl -X POST http://localhost:8090/v1/stores \
        -H "Content-Type: application/json" \
        -d "{\"name\": \"$store\"}"
done
```

### Step 4: Reindex Data

#### Option A: From Source Files

If you have access to original source files:

```bash
# Reindex from source
./rice-search index ./path/to/code --store default --api-url http://localhost:8090
```

#### Option B: From Exported Content

If using exported data:

```python
# scripts/reindex.py
import requests
import json

# Load exported files
with open("files_default.json") as f:
    files = json.load(f)

# Batch index to Go Edition
batch_size = 100
for i in range(0, len(files), batch_size):
    batch = files[i:i+batch_size]
    
    # Load content for each file
    documents = []
    for file in batch:
        try:
            with open(file["path"]) as f:
                content = f.read()
            documents.append({
                "path": file["path"],
                "content": content
            })
        except FileNotFoundError:
            print(f"Warning: {file['path']} not found")
    
    # Index batch
    resp = requests.post(
        "http://localhost:8090/v1/stores/default/index",
        json={"documents": documents}
    )
    
    if resp.status_code == 200:
        result = resp.json()
        print(f"Indexed {result['data']['indexed']} files")
    else:
        print(f"Error: {resp.text}")
```

### Step 5: Verify Migration

#### Compare Counts

```bash
# Current system
curl http://localhost:8080/v1/stores/default/stats

# Go Edition
curl http://localhost:8090/v1/stores/default/stats
```

#### Test Searches

```bash
# Same query to both systems
QUERY='{"query": "authentication handler", "top_k": 10}'

echo "Current system:"
curl -s -X POST http://localhost:8080/v1/stores/default/search \
    -H "Content-Type: application/json" \
    -d "$QUERY" | jq '.results[].path'

echo "Go Edition:"
curl -s -X POST http://localhost:8090/v1/stores/default/search \
    -H "Content-Type: application/json" \
    -d "$QUERY" | jq '.data.results[].path'
```

#### Run Test Suite

```bash
# Use existing test queries
./scripts/migration_test.sh http://localhost:8090
```

### Step 6: Switch Traffic

#### Update ricegrep Config

```yaml
# .ricegreprc.yaml
api_url: http://localhost:8090  # Changed from 8080
```

#### Update Load Balancer

```nginx
# Before
upstream rice-search {
    server old-api:8080;
}

# After
upstream rice-search {
    server new-api:8080;  # Go Edition
}
```

### Step 7: Decommission Old System

```bash
# Stop old services
docker-compose -f docker-compose.old.yml down

# Remove old data (after backup verification)
rm -rf ./data/milvus
rm -rf ./data/etcd
rm -rf ./data/minio
rm -rf ./data/tantivy
```

---

## API Compatibility

### Compatible Endpoints

| Endpoint | Compatible | Notes |
|----------|------------|-------|
| `POST /v1/stores/{store}/search` | ✅ | Same request/response |
| `POST /v1/stores/{store}/index` | ✅ | Same request/response |
| `DELETE /v1/stores/{store}/index` | ✅ | Same request/response |
| `GET /v1/stores` | ✅ | Same response |
| `POST /v1/stores` | ✅ | Same request/response |
| `GET /healthz` | ✅ | Same response |

### Changed Endpoints

| Endpoint | Change |
|----------|--------|
| Response wrapper | `data` field added |
| Error format | Standardized error codes |
| Metrics | Different metric names |

### Response Format Change

**Current:**
```json
{
    "results": [...],
    "total": 20
}
```

**Go Edition:**
```json
{
    "data": {
        "results": [...],
        "total": 20
    },
    "meta": {
        "request_id": "req_abc123",
        "latency_ms": 50
    }
}
```

#### Compatibility Mode

```bash
# Enable compatibility mode (omits wrapper)
COMPAT_MODE=v1 ./rice-search serve
```

---

## Data Format Changes

### Vectors

| Current (Milvus) | Go Edition (Qdrant) |
|------------------|---------------------|
| Dense only | Dense + Sparse |
| 1536 dims | 1536 dims (same) |
| Separate BM25 index | SPLADE in vectors |

### Chunk IDs

| Current | Go Edition |
|---------|------------|
| UUID | SHA256 hash prefix |
| Random | Deterministic |

**Impact:** Re-indexing required (can't migrate vectors directly)

---

## Configuration Mapping

| Current (.env) | Go Edition |
|----------------|------------|
| `MILVUS_HOST` | `QDRANT_URL` |
| `INFINITY_URL` | N/A (embedded) |
| `TANTIVY_INDEX_DIR` | N/A (SPLADE) |
| `EMBED_MODEL` | `EMBED_MODEL` (same) |
| `RERANK_MODEL` | `RERANK_MODEL` (same) |

---

## Rollback Plan

### If Migration Fails

```bash
# 1. Stop Go Edition
./rice-search stop

# 2. Revert load balancer
# ... update nginx/ALB config ...

# 3. Verify old system
curl http://localhost:8080/healthz

# 4. Investigate and retry
```

### Keep Parallel Systems

During migration, keep both systems running:

```
             ┌──────────────────┐
             │  Load Balancer   │
             └────────┬─────────┘
                      │
         ┌────────────┴────────────┐
         │                         │
         ▼                         ▼
    ┌─────────┐              ┌─────────┐
    │ Current │              │   Go    │
    │  (8080) │              │ (8090)  │
    └─────────┘              └─────────┘
```

---

## Timeline

| Phase | Duration | Activities |
|-------|----------|------------|
| Preparation | 1 day | Backup, inventory, staging test |
| Deploy | 1 hour | Start Qdrant, Go Edition |
| Reindex | 1-4 hours | Depends on data size |
| Verify | 2 hours | Compare results, test queries |
| Switch | 15 min | Update DNS/LB |
| Monitor | 1 day | Watch for issues |
| Decommission | 1 hour | Remove old system |

---

## Troubleshooting

### Search Results Differ

SPLADE vs Tantivy may produce different rankings.

**Solution:** Tune `sparse_weight` and `dense_weight` to match expected behavior.

### Missing Documents

**Check:**
1. Were all files exported?
2. Did any files fail to index?
3. Are file paths matching?

**Debug:**
```bash
# List indexed files
curl http://localhost:8090/v1/stores/default/index/files | jq '.files | length'
```

### Performance Regression

**Check:**
1. Are models loaded? (`/v1/health`)
2. Is GPU being used?
3. Check Qdrant collection settings

**Debug:**
```bash
# Check health
curl http://localhost:8090/v1/health | jq '.checks.models'

# Check GPU
curl http://localhost:8090/v1/health | jq '.checks.models.device'
```
