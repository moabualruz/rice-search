# Local Code Search - Unified API (NestJS + Bun)
# Multi-stage build for optimized production image

# ============================================
# Stage 1: Build Tantivy CLI (Rust)
# ============================================
FROM rust:1.83-slim AS rust-builder

WORKDIR /build

# Install build dependencies
RUN apt-get update && apt-get install -y \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

# Copy and build Tantivy
COPY tantivy ./tantivy
RUN cd tantivy && cargo build --release

# ============================================
# Stage 2: Build with Bun
# ============================================
FROM oven/bun:1 AS bun-builder

WORKDIR /app

# Install curl for WASM download
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Copy package files
COPY package.json bun.lockb* bunfig.toml ./

# Install dependencies
RUN bun install --frozen-lockfile || bun install

# Copy source code
COPY tsconfig.json tsconfig.build.json nest-cli.json ./
COPY src ./src

# Copy WASM download script and any existing WASM files from repo
COPY scripts/download-wasm.sh ./scripts/
COPY wasm ./wasm

# Download latest WASM files (overrides repo versions)
RUN chmod +x ./scripts/download-wasm.sh && ./scripts/download-wasm.sh ./wasm --force

# Build the application using NestJS CLI via Bun
RUN bun run build

# ============================================
# Stage 3: Production Image
# ============================================
FROM oven/bun:1-slim AS production

WORKDIR /app

# Install runtime dependencies
RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Copy Tantivy binary from rust-builder
COPY --from=rust-builder /build/tantivy/target/release/tantivy-cli /usr/local/bin/

# Copy node_modules, built application, and WASM files
COPY --from=bun-builder /app/node_modules ./node_modules
COPY --from=bun-builder /app/dist ./dist
COPY --from=bun-builder /app/wasm ./wasm
COPY --from=bun-builder /app/package.json ./

# Create data directories
RUN mkdir -p /data /tantivy

# Set environment variables
ENV NODE_ENV=production
ENV PORT=8080

# Bun performance tuning
ENV BUN_CONFIG_MAX_HTTP_CONNECTIONS=1024
ENV BUN_CONFIG_NO_CLEAR_TERMINAL_ON_RELOAD=1

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/healthz || exit 1

# Run the application with Bun
CMD ["bun", "run", "dist/main.js"]
