# BGE-M3 Embedding Service

FastAPI-based embedding service for the BGE-M3 model from FlagEmbedding. Supports multi-functionality (dense, sparse, ColBERT), multi-linguality (100+ languages), and multi-granularity (up to 8192 tokens).

## Features

- **Dense Embeddings**: Single 1024-dim vector per text
- **Sparse Embeddings**: Lexical weights (BM25-like token weights)
- **ColBERT Embeddings**: Multi-vector representations for fine-grained matching
- **Reranking**: ColBERT-based document reranking
- **GPU/CPU Support**: Configurable via `DEVICE` environment variable
- **FP16 Optimization**: Optional half-precision for faster inference

## Quick Start

### Build and Run (CPU)

```bash
docker build -t bge-m3:latest .
docker run -p 8082:80 -e DEVICE=cpu bge-m3:latest
```

### Build and Run (GPU)

```bash
docker build -t bge-m3:latest .
docker run --gpus all -p 8082:80 -e DEVICE=cuda -e USE_FP16=true bge-m3:latest
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MODEL_NAME` | `BAAI/bge-m3` | HuggingFace model ID |
| `DEVICE` | `cuda` (if available) or `cpu` | Device for inference |
| `USE_FP16` | `true` | Use half-precision (GPU only) |
| `BATCH_SIZE` | `32` | Default batch size |
| `MAX_LENGTH` | `8192` | Default max sequence length |

## API Endpoints

### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "model": "BAAI/bge-m3",
  "device": "cuda",
  "use_fp16": true
}
```

### Encode Texts

```bash
POST /encode
Content-Type: application/json

{
  "texts": ["Hello world", "BGE-M3 is awesome"],
  "return_dense": true,
  "return_sparse": false,
  "return_colbert": false,
  "normalize": true
}
```

Response:
```json
{
  "dense": [
    {
      "embedding": [0.123, -0.456, ...],
      "index": 0
    }
  ],
  "model": "BAAI/bge-m3",
  "usage": {
    "texts": 2,
    "time_ms": 45
  }
}
```

### Rerank Documents

```bash
POST /rerank
Content-Type: application/json

{
  "query": "What is machine learning?",
  "documents": [
    "Machine learning is a subset of AI...",
    "Python is a programming language...",
    "Neural networks are used in ML..."
  ],
  "top_k": 2
}
```

Response:
```json
{
  "results": [
    {
      "document": "Machine learning is a subset of AI...",
      "score": 0.89,
      "index": 0
    },
    {
      "document": "Neural networks are used in ML...",
      "score": 0.67,
      "index": 2
    }
  ],
  "model": "BAAI/bge-m3",
  "usage": {
    "query": 1,
    "documents": 3,
    "time_ms": 120
  }
}
```

## Integration with Rice Search

Add to `docker-compose.yml`:

```yaml
bge-m3:
  container_name: rice-bge-m3
  build:
    context: ./docker/bge-m3
    dockerfile: Dockerfile
  environment:
    MODEL_NAME: BAAI/bge-m3
    DEVICE: cpu
    USE_FP16: "false"
    BATCH_SIZE: "32"
    MAX_LENGTH: "8192"
  volumes:
    - ./data/bge-m3-cache:/root/.cache/huggingface
  ports:
    - "8082:80"
  healthcheck:
    test: ["CMD", "curl", "-f", "http://localhost:80/health"]
    interval: 30s
    start_period: 90s
    timeout: 10s
    retries: 3
  restart: unless-stopped
  networks:
    - rice-network
```

For GPU support:

```yaml
bge-m3:
  # ... other config ...
  environment:
    DEVICE: cuda
    USE_FP16: "true"
  deploy:
    resources:
      reservations:
        devices:
          - driver: nvidia
            count: 1
            capabilities: [gpu]
```

## Performance Tips

1. **GPU**: Use GPU with FP16 for ~10x faster inference
2. **Batch Size**: Increase for higher throughput (at cost of latency)
3. **Max Length**: Reduce if you don't need full 8192 tokens
4. **Dense Only**: Set `return_sparse=false` and `return_colbert=false` for faster encoding
5. **Model Caching**: Mount volume to cache model downloads

## Testing

```bash
# Health check
curl http://localhost:8082/health

# Dense embeddings
curl -X POST http://localhost:8082/encode \
  -H "Content-Type: application/json" \
  -d '{
    "texts": ["Hello world"],
    "return_dense": true
  }'

# Sparse embeddings (lexical weights)
curl -X POST http://localhost:8082/encode \
  -H "Content-Type: application/json" \
  -d '{
    "texts": ["machine learning"],
    "return_dense": false,
    "return_sparse": true
  }'

# Reranking
curl -X POST http://localhost:8082/rerank \
  -H "Content-Type: application/json" \
  -d '{
    "query": "python programming",
    "documents": [
      "Python is a versatile language",
      "Java is object-oriented",
      "JavaScript runs in browsers"
    ]
  }'
```

## References

- [BGE-M3 Paper](https://arxiv.org/pdf/2402.03216.pdf)
- [FlagEmbedding GitHub](https://github.com/FlagOpen/FlagEmbedding)
- [BGE-M3 Model](https://huggingface.co/BAAI/bge-m3)
