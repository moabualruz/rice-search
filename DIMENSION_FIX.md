# Vector Dimension Mismatch Fix

## Problem

After rebuilding the system, all search queries returned no results. Logs showed critical errors:

```
Vector dimension error: expected dim: 768, got 2560
bm42 search failed: Vector dimension error
All retrievers failed or returned no results
Worker: ApiException on upsert to Qdrant
```

## Root Cause

**Mismatch between configured dimension and actual embedding dimension:**

- **settings.yaml** had: `dimension: 1024`, `fallback_dimension: 768`
- **Actual qwen3-embedding:4b output**: **2560 dimensions**
- **Qdrant collection** was created with 768 dims (using fallback)
- **Embeddings** being indexed had 2560 dims → **Dimension mismatch error**

## Solution

### 1. Determined Actual Embedding Dimension

```bash
curl -s -X POST http://localhost:11434/api/embeddings \
  -H "Content-Type: application/json" \
  -d '{"model": "qwen3-embedding:4b", "prompt": "test"}' \
  | grep -o ',' | wc -l
# Output: 2559 commas → 2560 dimensions
```

### 2. Updated settings.yaml

**Before:**
```yaml
models:
  embedding:
    dimension: 1024
    fallback_dimension: 768
    ollama_model: qwen3-embedding:4b
```

**After:**
```yaml
models:
  embedding:
    dimension: 2560
    fallback_dimension: 2560
    ollama_model: qwen3-embedding:4b
```

### 3. Deleted Old Collection

```bash
curl -X DELETE http://localhost:6333/collections/rice_chunks
# Response: {"result":true,"status":"ok"}
```

### 4. Restarted Services

```bash
cd deploy
docker-compose restart backend-api backend-worker
```

### 5. Reindexed All Files

```bash
cd backend
ricesearch index .
```

## Verification

### Collection Created with Correct Dimension

```bash
curl -s http://localhost:6333/collections/rice_chunks | grep -o '"size":[0-9]*'
# Output: "size":2560 ✅
```

### Points Indexed Successfully

```bash
curl -s http://localhost:6333/collections/rice_chunks | grep -o '"points_count":[0-9]*'
# Output: "points_count":330 ✅
```

### Search Working

```bash
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "embedding model", "limit": 2, "mode": "search"}'
```

**Result:**
- ✅ Returns results with proper file paths
- ✅ Rerank scores present (6.45, 4.78)
- ✅ All retrievers working (BM25, SPLADE, BM42)
- ✅ New metadata fields present (full_path, filename)

### File Name Search Working

```bash
curl -X POST http://localhost:8000/api/v1/search/query \
  -d '{"query": "ollama_client.py", "mode": "search"}'
```

**Result:**
- ✅ Returns `ollama_client.py` as top results
- ✅ File path indexing enhancement working correctly

## Key Learnings

1. **Always verify embedding dimensions** when using new models
2. **qwen3-embedding:4b** produces 2560-dim embeddings (not documented clearly)
3. **Collection dimension must match** embedding output exactly
4. **Settings cache** in Redis/backend may require restart to pick up changes
5. **Delete collection before reindexing** after dimension changes

## Files Modified

- ✅ `backend/settings.yaml` - Updated dimension from 1024/768 to 2560

## Impact

- **Storage**: 2560-dim vectors use ~2.5x more space than 1024-dim
  - 330 chunks × 2560 floats × 4 bytes = ~3.3 MB for dense vectors
  - Plus SPLADE and BM42 sparse vectors
  - Total collection size acceptable for local deployment
- **Performance**: No noticeable impact on search speed
- **Quality**: Potentially better semantic search with higher-dim embeddings

## Summary

System now fully operational:
- ✅ Vector dimensions aligned (2560 across the board)
- ✅ 330 chunks indexed successfully
- ✅ All three retrievers working (BM25, SPLADE, BM42)
- ✅ Reranking functional
- ✅ File name search working
- ✅ Regular search working

**Search is back online and functioning correctly!** 🎉
