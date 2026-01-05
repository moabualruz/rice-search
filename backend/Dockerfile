FROM python:3.12-slim

WORKDIR /app

# Install system dependencies (for unstructured, etc)
RUN apt-get update && apt-get install -y \
    build-essential \
    libmagic-dev \
    && rm -rf /var/lib/apt/lists/*

COPY pyproject.toml .
# Explicitly install deps to resolve potential pip resolution issues (Updated versions)
RUN --mount=type=cache,target=/root/.cache/pip pip install "opentelemetry-api>=1.25.0" "opentelemetry-sdk>=1.25.0" "opentelemetry-instrumentation-fastapi>=0.46b0" "opentelemetry-instrumentation-requests>=0.46b0"
RUN --mount=type=cache,target=/root/.cache/pip pip install .

COPY . .

# Default command (overridden in docker-compose)
CMD ["uvicorn", "src.main:app", "--host", "0.0.0.0", "--port", "8000"]
