# syntax=docker/dockerfile:1.4
# Rice Search - BentoML Inference Service
# GPU-enabled image with BuildKit pip caching

FROM nvidia/cuda:12.1.0-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive

# Install Python and system deps (including C compiler for vLLM)
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.10 \
    python3.10-venv \
    python3.10-dev \
    python3-pip \
    git \
    curl \
    build-essential \
    gcc \
    && rm -rf /var/lib/apt/lists/* \
    && update-alternatives --install /usr/bin/python python /usr/bin/python3.10 1 \
    && update-alternatives --install /usr/bin/pip pip /usr/bin/pip3 1

# Upgrade pip
RUN pip install --upgrade pip setuptools wheel

# Install PyTorch with CUDA (cached)
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install torch==2.3.0 --index-url https://download.pytorch.org/whl/cu121

# Install ML dependencies (cached)
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install \
    bentoml>=1.2.0 \
    sentence-transformers>=2.2.0 \
    transformers>=4.40.0 \
    accelerate>=0.20.0 \
    pydantic>=2.0.0 \
    einops \
    httpx

# Install vLLM (cached)
RUN --mount=type=cache,target=/root/.cache/pip \
    pip install vllm>=0.4.0

WORKDIR /app
COPY service.py .

ENV HF_HOME=/root/.cache/huggingface
ENV EMBEDDING_MODEL=BAAI/bge-base-en-v1.5
ENV RERANK_MODEL=BAAI/bge-reranker-base
ENV LLM_MODEL=codellama/CodeLlama-7b-Instruct-hf
ENV BENTOML_PORT=3001

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=30s --start-period=120s --retries=3 \
    CMD curl -sf -X POST http://localhost:3001/health || exit 1

CMD ["bentoml", "serve", "service:RiceInferenceService", "--host", "0.0.0.0", "--port", "3001"]
