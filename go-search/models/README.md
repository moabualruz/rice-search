# Rice Search - ML Models

This directory contains ONNX models required for Rice Search.

## Required Models

| Model | Purpose | Size | Download |
|-------|---------|------|----------|
| `jina-embeddings-v3/` | Dense embeddings (1536d) | ~1.2GB | [HuggingFace](https://huggingface.co/jinaai/jina-embeddings-v3) |
| `splade-v3/` | Sparse vectors (BM25 replacement) | ~500MB | [HuggingFace](https://huggingface.co/naver/splade-v3) |
| `jina-reranker-v2/` | Cross-encoder reranking | ~800MB | [HuggingFace](https://huggingface.co/jinaai/jina-reranker-v2-base-multilingual) |

## Automatic Download

```bash
# Download all required models
./scripts/download-models.sh

# Or use the CLI
rice-search models download
```

## Manual Download

### Option 1: From HuggingFace (Recommended)

```bash
# Install huggingface-cli
pip install huggingface-hub

# Download models
huggingface-cli download jinaai/jina-embeddings-v3 --local-dir ./models/jina-embeddings-v3
huggingface-cli download naver/splade-v3 --local-dir ./models/splade-v3
huggingface-cli download jinaai/jina-reranker-v2-base-multilingual --local-dir ./models/jina-reranker-v2
```

### Option 2: Export to ONNX

If models aren't available in ONNX format, export them:

```python
from optimum.onnxruntime import ORTModelForFeatureExtraction
from transformers import AutoTokenizer

# Export embedding model
model = ORTModelForFeatureExtraction.from_pretrained(
    "jinaai/jina-embeddings-v3",
    export=True,
    provider="CPUExecutionProvider"
)
model.save_pretrained("./models/jina-embeddings-v3")

tokenizer = AutoTokenizer.from_pretrained("jinaai/jina-embeddings-v3")
tokenizer.save_pretrained("./models/jina-embeddings-v3")
```

## Directory Structure

Each model directory should contain:

```
models/
├── jina-embeddings-v3/
│   ├── model.onnx           # ONNX model file
│   ├── tokenizer.json       # Tokenizer config
│   ├── config.json          # Model config
│   └── vocab.txt            # Vocabulary
├── splade-v3/
│   ├── model.onnx
│   ├── tokenizer.json
│   └── config.json
└── jina-reranker-v2/
    ├── model.onnx
    ├── tokenizer.json
    └── config.json
```

## Verification

Verify model checksums after download:

```bash
rice-search models verify
```

## GPU Acceleration

For GPU inference, ensure you have:
- CUDA 11.x or 12.x
- cuDNN 8.x
- ONNX Runtime GPU provider

Set environment variable:
```bash
export RICE_ML_DEVICE=cuda
```

## Troubleshooting

### Model Not Found

```
Error: model not found: jina-embeddings-v3
```

Run `rice-search models download` or manually place models in this directory.

### CUDA Out of Memory

Reduce batch size:
```bash
export RICE_EMBED_BATCH_SIZE=8
```

### Tokenizer Error

Ensure `tokenizer.json` exists in the model directory. Some models use `tokenizer_config.json` instead - Rice Search looks for both.
