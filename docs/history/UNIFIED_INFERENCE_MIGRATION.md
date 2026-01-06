# Unified-Inference Migration Summary

## Changes Made

### 1. Backend Client Updated (`backend/src/services/inference/bentoml_client.py`)

**Class Renamed:**
- `BentoMLClient` → `UnifiedInferenceClient`
- Backward compatibility alias maintained

**API Endpoints Updated:**
| Old Endpoint | New Endpoint | Format |
|--------------|--------------|--------|
| `POST /embed` | `POST /v1/embeddings` | OpenAI-compatible |
| `POST /rerank` | `POST /v1/rerank` | SGLang format |
| `POST /chat` | `POST /v1/chat/completions` | OpenAI-compatible |
| `POST /health` | `GET /health` | Standard health check |

**Request Format Changes:**

**Embeddings:**
```python
# OLD (BentoML)
{"request": {"texts": [...], "model": "..."}}

# NEW (Unified-Inference)
{"input": [...], "model": "..."}  # model is optional (defaults to bge-base-en)
```

**Reranking:**
```python
# OLD (BentoML)
{"request": {"query": "...", "documents": [...], "top_n": 10, "model": "..."}}

# NEW (Unified-Inference)
{"query": "...", "documents": [...], "top_n": 10, "model": "..."}  # model is optional (defaults to bge-reranker)
```

**Chat:**
```python
# OLD (BentoML)
{"request": {"messages": [...], "model": "...", "max_tokens": 1024, "temperature": 0.7}}

# NEW (Unified-Inference)
{"messages": [...], "model": "...", "max_tokens": 1024, "temperature": 0.7}  # model is optional (defaults to qwen-coder-1.5b)
```

**Response Format Changes:**

**Embeddings:**
```python
# OLD
{"embeddings": [[...]]}

# NEW (OpenAI format)
{"data": [{"embedding": [...]}, ...]}
```

**Chat:**
```python
# OLD
{"content": "..."}

# NEW (OpenAI format)
{"choices": [{"message": {"content": "..."}}]}
```

### 2. Configuration Updated (`backend/src/core/config.py`)

Added new environment variable while maintaining backward compatibility:
```python
BENTOML_URL: str = "http://localhost:3001"  # Backward compatibility
INFERENCE_URL: str = "http://localhost:3001"  # New name (same value)
```

### 3. Docker Compose Already Configured

Both `backend-api` and `backend-worker` services correctly reference:
```yaml
environment:
  - BENTOML_URL=http://unified-inference:3001
depends_on:
  - unified-inference
```

## Default Model Auto-Selection

The new unified-inference service **automatically selects default models** based on endpoint:

| Endpoint | Default Model | Type |
|----------|--------------|------|
| `/v1/embeddings` | `bge-base-en` | Embedding |
| `/v1/rerank` | `bge-reranker` | Reranking |
| `/v1/chat/completions` | `qwen-coder-1.5b` | LLM |

**The `model` parameter is now OPTIONAL** for these standard endpoints. Custom models still require explicit `model` parameter.

## Testing Checklist

- [ ] **Backend API starts successfully**
  ```bash
  docker-compose up backend-api
  ```

- [ ] **Backend worker starts successfully**
  ```bash
  docker-compose up backend-worker
  ```

- [ ] **Embeddings work**
  - Test via backend API search endpoint
  - Verify default model (bge-base-en) is auto-selected

- [ ] **Reranking works**
  - Test via backend API search with reranking enabled
  - Verify default model (bge-reranker) is auto-selected

- [ ] **LLM chat works**
  - Test via backend RAG/chat endpoint
  - Verify default model (qwen-coder-1.5b) is auto-selected

- [ ] **Health checks pass**
  ```bash
  curl http://localhost:3001/health  # unified-inference
  curl http://localhost:8000/health  # backend-api
  ```

- [ ] **Model lifecycle works**
  - Verify models auto-start on first request
  - Verify models auto-stop after idle timeout (5 minutes)

## Backward Compatibility

✅ **All existing code continues to work** because:
1. `BentoMLClient` class name aliased to `UnifiedInferenceClient`
2. `get_bentoml_client()` function maintained
3. `BENTOML_URL` environment variable still used
4. Client interface (methods, parameters) unchanged

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│ Backend API (Port 8000)                                     │
│ Backend Worker                                              │
│                                                             │
│ ┌─────────────────────────────────────────────────────┐    │
│ │ UnifiedInferenceClient (bentoml_client.py)          │    │
│ │ - embed(texts) → default: bge-base-en               │    │
│ │ - rerank(query, docs) → default: bge-reranker       │    │
│ │ - chat(messages) → default: qwen-coder-1.5b         │    │
│ └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                         │
                         │ HTTP (OpenAI-compatible API)
                         ▼
┌─────────────────────────────────────────────────────────────┐
│ Unified-Inference Service (Port 3001)                      │
│                                                             │
│ ┌─────────────┐  ┌──────────────┐  ┌────────────────┐     │
│ │  Router     │  │  Lifecycle   │  │  Offload       │     │
│ │  (FastAPI)  │──│  Manager     │──│  Policy        │     │
│ └─────────────┘  └──────────────┘  └────────────────┘     │
└─────────────────────────────────────────────────────────────┘
                         │
      ┌──────────────────┼──────────────────┐
      ▼                  ▼                  ▼
┌──────────┐       ┌──────────┐       ┌──────────┐
│ SGLang   │       │ SGLang   │       │ SGLang   │
│ bge-base │       │ bge-     │       │ qwen-    │
│ (30001)  │       │ reranker │       │ coder    │
│          │       │ (30002)  │       │ (30003)  │
└──────────┘       └──────────┘       └──────────┘
```

## Running the Full Stack

```bash
# Start all services
cd deploy
docker-compose up

# Or start specific services
docker-compose up unified-inference backend-api backend-worker frontend

# Check logs
docker-compose logs -f unified-inference
docker-compose logs -f backend-api
docker-compose logs -f backend-worker
```

## Troubleshooting

**Issue: Backend can't connect to unified-inference**
```bash
# Check if unified-inference is running
docker-compose ps unified-inference

# Check unified-inference logs
docker-compose logs unified-inference

# Check backend logs
docker-compose logs backend-api
```

**Issue: Models not starting**
```bash
# Check unified-inference status
curl http://localhost:3001/v1/status

# Check available models
curl http://localhost:3001/v1/models
```

**Issue: Embeddings/reranking failing**
```bash
# Test embeddings directly
curl -X POST http://localhost:3001/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"input": ["test"]}'

# Test reranking directly
curl -X POST http://localhost:3001/v1/rerank \
  -H "Content-Type: application/json" \
  -d '{"query": "test", "documents": ["doc1", "doc2"]}'
```

## Benefits of New Architecture

1. **Elastic Memory**: Models use `--max-total-tokens 0` (no pre-allocation)
2. **Dynamic Lifecycle**: Models auto-start on demand, auto-stop when idle
3. **Default Model Selection**: No need to specify model for standard endpoints
4. **OpenAI Compatibility**: Standard API format, easy to integrate
5. **CPU Offload**: Automatic spillover to CPU when GPU overloaded
6. **Better Resource Utilization**: Only load models when needed
7. **Easier Debugging**: Clear separation of concerns, structured errors

## Next Steps

1. Test the full stack end-to-end
2. Monitor model lifecycle (start/stop behavior)
3. Verify CPU offload works under load
4. Check frontend integration (if applicable)
5. Update any documentation references to BentoML
