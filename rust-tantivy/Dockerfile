# Rice Search - Tantivy BM25 Service
# Lightweight Rust container for lexical search

FROM rust:1.83-slim-bookworm AS builder

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Copy source
COPY Cargo.toml .
COPY src/ src/

# Build release binary
RUN cargo build --release

# Runtime stage
FROM debian:bookworm-slim

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/target/release/rice-tantivy /app/rice-tantivy

# Create data directory
RUN mkdir -p /data

# Environment
ENV RUST_LOG=info
ENV TANTIVY_DATA_DIR=/data
ENV PORT=3002
ENV HOST=0.0.0.0

EXPOSE 3002

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -sf http://localhost:3002/health || exit 1

# Install curl for healthcheck
RUN apt-get update && apt-get install -y --no-install-recommends curl \
    && rm -rf /var/lib/apt/lists/*

CMD ["/app/rice-tantivy"]
