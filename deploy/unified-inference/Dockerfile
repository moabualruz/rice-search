# Rice Search - Unified Inference Service (Flask + SGLang)
# All models in ONE service: Embeddings, Reranking, LLM

FROM nvidia/cuda:12.6.1-devel-ubuntu24.04

ENV DEBIAN_FRONTEND=noninteractive \
    PIP_BREAK_SYSTEM_PACKAGES=1

# Install Python 3.12 (stable with broad compatibility) and system deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    software-properties-common \
    && add-apt-repository ppa:deadsnakes/ppa \
    && apt-get update && apt-get install -y --no-install-recommends \
    python3.12 \
    python3.12-venv \
    python3.12-dev \
    git \
    curl \
    build-essential \
    gcc \
    libnuma1 \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get purge -y python3.10 python3.11 2>/dev/null || true \
    && update-alternatives --install /usr/bin/python python /usr/bin/python3.12 1 \
    && update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.12 1 \
    && ln -sf /usr/bin/python3.12 /usr/bin/python \
    && ln -sf /usr/bin/python3.12 /usr/bin/python3

# Install Rust for building outlines_core (SGLang dependency)
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain stable
ENV PATH="/root/.cargo/bin:${PATH}"

# Bootstrap pip for Python 3.12 (PEP 668 bypassed via ENV)
RUN curl -sS https://bootstrap.pypa.io/get-pip.py | python3.12 \
    && pip3.12 install --upgrade pip setuptools wheel

WORKDIR /app
COPY service.py /app/service.py
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV HF_HOME=/root/.cache/huggingface
ENV FLASK_PORT=3001

EXPOSE 3001

HEALTHCHECK --interval=30s --timeout=30s --start-period=180s --retries=5 \
    CMD curl -sf -X POST http://localhost:3001/health || exit 1

ENTRYPOINT ["/entrypoint.sh"]
