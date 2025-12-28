# Contributing to Rice Search

## Local Development Setup

This guide covers running Rice Search locally for development, without Docker for the API/Web UI components.

### Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| [Bun](https://bun.sh) | 1.0+ | JavaScript runtime & package manager |
| [Rust](https://rustup.rs) | 1.70+ | Tantivy BM25 search engine |
| [Docker](https://docker.com) | 20+ | Milvus & embeddings services |
| [Python](https://python.org) | 3.10+ | Indexing scripts (optional) |

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Local Development                                      │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────────┐   │
│  │ Web UI   │  │   API    │  │  Tantivy (cargo run) │   │
│  │ :3001    │  │  :8088   │  │  (spawned by API)    │   │
│  └──────────┘  └──────────┘  └──────────────────────┘   │
└─────────────────────────────────────────────────────────┘
                         │
┌─────────────────────────────────────────────────────────┐
│  Docker Services                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐              │
│  │ Milvus   │  │   TEI    │  │  MinIO   │  + etcd     │
│  │ :19530   │  │  :8081   │  │  :9000   │              │
│  └──────────┘  └──────────┘  └──────────┘              │
└─────────────────────────────────────────────────────────┘
```

## Quick Start

### 1. Start Docker Services

The vector database (Milvus) and embeddings service (TEI) run in Docker:

```bash
# Start only the backend services (not API/web-ui)
docker-compose up -d milvus embeddings etcd minio

# Verify services are healthy
docker-compose ps
```

Wait for Milvus to be ready (~1-2 minutes on first run).

### 2. Setup API

```bash
cd api

# Install dependencies
bun install

# Create local environment file (if not exists)
# See "Environment Configuration" below for details
```

### 3. Setup Web UI

```bash
cd web-ui

# Install dependencies
bun install
```

### 4. Run Development Servers

**Terminal 1 - API:**
```bash
cd api
bun run start:local
```

**Terminal 2 - Web UI:**
```bash
cd web-ui
bun run dev:local
```

**Access:**
- API: http://localhost:8088
- API Swagger Docs: http://localhost:8088/docs
- Web UI: http://localhost:3001

## Environment Configuration

### API (`api/.env.dev`)

Create or update `api/.env.dev`:

```env
# Server
PORT=8088
NODE_ENV=development

# Milvus (Docker)
MILVUS_HOST=localhost
MILVUS_PORT=19530

# Embeddings (Docker)
EMBEDDINGS_URL=http://localhost:8081

# Data directories (local paths)
DATA_DIR=F:/work/rice-search/api/data
TANTIVY_INDEX_DIR=F:/work/rice-search/api/data/tantivy

# Tantivy - use cargo run for auto-recompilation
TANTIVY_USE_CARGO=true
TANTIVY_PROJECT_DIR=F:/work/rice-search/api/tantivy

# Alternative: use pre-built binary (faster, no auto-recompile)
# TANTIVY_USE_CARGO=false
# TANTIVY_CLI_PATH=F:/work/rice-search/api/tantivy/target/release/tantivy-cli.exe
```

**Note:** Adjust paths for your OS:
- Windows: `F:/work/rice-search/...` or `C:/Users/.../rice-search/...`
- Linux/macOS: `/home/user/rice-search/...`

### Web UI (`web-ui/.env.dev`)

Create or update `web-ui/.env.dev`:

```env
NEXT_PUBLIC_API_URL=http://localhost:8088
```

## Tantivy Development

The Tantivy BM25 search engine is a Rust CLI located in `api/tantivy/`.

### Cargo Run Mode (Recommended for Dev)

With `TANTIVY_USE_CARGO=true`, the API spawns Tantivy via `cargo run --release`. This means:

- **Auto-recompilation**: Changes to Rust code are compiled on next API call
- **Slight delay**: First call after changes triggers compilation (~2-5s)
- **No manual rebuild**: Just edit and save

### Pre-built Binary Mode (Faster Execution)

For faster execution without recompilation:

```bash
cd api/tantivy
cargo build --release
```

Then set in `.env.dev`:
```env
TANTIVY_USE_CARGO=false
TANTIVY_CLI_PATH=F:/work/rice-search/api/tantivy/target/release/tantivy-cli.exe
```

### Testing Tantivy CLI Directly

```bash
cd api/tantivy

# Run with cargo
cargo run --release -- --help
cargo run --release -- search --help

# Or use built binary
./target/release/tantivy-cli --help
```

## Project Structure

```
rice-search/
├── api/                    # NestJS API server
│   ├── src/
│   │   ├── config/         # Configuration
│   │   ├── health/         # Health checks
│   │   ├── index/          # Indexing endpoints
│   │   ├── mcp/            # MCP protocol support
│   │   ├── search/         # Search endpoints
│   │   ├── services/       # Core services
│   │   │   ├── tantivy.service.ts    # BM25 search
│   │   │   ├── milvus.service.ts     # Vector search
│   │   │   └── embeddings.service.ts # Text embeddings
│   │   └── stores/         # Store management
│   ├── tantivy/            # Rust BM25 CLI
│   │   ├── src/main.rs
│   │   └── Cargo.toml
│   └── .env.dev            # Local dev config
├── web-ui/                 # Next.js frontend
│   ├── src/app/
│   └── .env.dev            # Local dev config
├── ricegrep/               # CLI tool
│   └── src/
└── scripts/                # Utility scripts
```

## Package.json Scripts

### API (`api/package.json`)

| Script | Description |
|--------|-------------|
| `bun run start:local` | Start dev server with `.env.dev` |
| `bun run start:dev` | Start dev server (Docker config) |
| `bun run build` | Build for production |
| `bun run typecheck` | Run TypeScript type checking |
| `bun run lint` | Run ESLint |
| `bun test` | Run tests |

### Web UI (`web-ui/package.json`)

| Script | Description |
|--------|-------------|
| `bun run dev:local` | Start dev server with `.env.dev` |
| `bun run dev` | Start dev server (default config) |
| `bun run build` | Build for production |
| `bun run lint` | Run ESLint |

### ricegrep (`ricegrep/package.json`)

| Script | Description |
|--------|-------------|
| `bun run build` | Build CLI |
| `bun run typecheck` | Run TypeScript type checking |
| `bun run format` | Format code with Biome |
| `bun test` | Run tests |

## Code Quality

### Before Committing

```bash
# API
cd api
bun run typecheck
bun run lint

# Web UI
cd web-ui
bun run lint

# ricegrep
cd ricegrep
bun run typecheck
bun run format
```

### Type Safety

- **Never** use `any`, `@ts-ignore`, or `@ts-expect-error`
- **Always** run `bun run typecheck` before committing

## Testing

### API Tests

```bash
cd api
bun test                        # All tests
bun test --filter "Search"      # Specific tests
```

### ricegrep Tests

```bash
cd ricegrep
bun test
```

### End-to-End Smoke Test

```bash
# Requires all services running
bash scripts/smoke_test.sh
```

## Troubleshooting

### Port Already in Use

```bash
# Windows - find process using port
netstat -ano | findstr :8088
taskkill /F /PID <PID>

# Linux/macOS
lsof -i :8088
kill -9 <PID>
```

### Milvus Connection Failed

1. Check Docker is running: `docker ps`
2. Check Milvus health: `docker-compose logs milvus`
3. Wait for Milvus startup (~1-2 min on first run)

### Tantivy Compilation Errors

```bash
cd api/tantivy
cargo clean
cargo build --release
```

### Embeddings Service Not Responding

1. Check TEI is running: `docker-compose logs embeddings`
2. TEI downloads model on first run (~2GB, takes a few minutes)
3. Test directly: `curl http://localhost:8081/health`

## Default Ports

| Service | Dev Port | Docker Port | Description |
|---------|----------|-------------|-------------|
| API | 8088 | 8088 | Rice Search API |
| Web UI | 3001 | 3000 | Next.js frontend |
| Milvus | 19530 | 19530 | Vector database |
| TEI | 8081 | 8081 | Embeddings service |
| MinIO | 9000 | 9000 | Object storage |
| MinIO Console | 9001 | 9001 | MinIO admin |
| Attu | 8000 | 8000 | Milvus admin UI |

## Pull Request Guidelines

1. Create a feature branch from `main`
2. Make your changes
3. Run type checking and linting
4. Run relevant tests
5. Update documentation if needed
6. Submit PR with clear description

## Questions?

Open an issue on GitHub or check existing discussions.
