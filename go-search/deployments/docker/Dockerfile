# Rice Search - Go Edition
# Multi-stage Docker build

# =============================================================================
# Stage 1: Build
# =============================================================================
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make ca-certificates tzdata

# Set working directory
WORKDIR /src

# Copy go mod files first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info
ARG VERSION=dev
ARG COMMIT=unknown
ARG DATE=unknown

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" \
    -o /bin/rice-search \
    ./cmd/rice-search

# =============================================================================
# Stage 2: Runtime (with ONNX export support)
# =============================================================================
FROM python:3.11-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    tzdata \
    wget \
    && rm -rf /var/lib/apt/lists/*

# Install optimum for ONNX export (enables HuggingFace model conversion)
RUN pip install --no-cache-dir \
    optimum[onnx] \
    transformers \
    torch --index-url https://download.pytorch.org/whl/cpu \
    && pip cache purge

# Create non-root user
RUN groupadd -g 1000 rice && \
    useradd -u 1000 -g rice -m rice

# Create directories
RUN mkdir -p /app/models /app/data && \
    chown -R rice:rice /app

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /bin/rice-search /app/rice-search

# Copy model download script (optional)
COPY --from=builder /src/scripts/download-models.sh /app/scripts/

# Switch to non-root user
USER rice

# Expose ports
# 8080 - HTTP API
# 3000 - Web UI (if separate)
EXPOSE 8080 3000

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q --spider http://localhost:8080/healthz || exit 1

# Default environment
ENV RICE_HOST=0.0.0.0 \
    RICE_PORT=8080 \
    RICE_LOG_LEVEL=info \
    RICE_LOG_FORMAT=json

# Entrypoint
ENTRYPOINT ["/app/rice-search"]

# Default command
CMD ["serve"]
