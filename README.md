# Rice Search Platform

A fully local, production-ready code search platform with hybrid BM25 + semantic search. Self-hosted hybrid search for code with ricegrep CLI support.

## Features

- **Hybrid Search**: Combines BM25 (keyword) + semantic embeddings for best-of-both-worlds results
- **Fully Local**: All components run locally - no external API calls
- **GPU Optional**: Works on CPU, with optional GPU acceleration
- **Cross-Platform**: Windows (WSL2), Linux, macOS
- **MCP Compatible**: Model Context Protocol support for AI assistant integration
- **Persistent Storage**: All data stored in `./data/` via bind mounts

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                         Clients                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ Web UI   │  │ ricegrep │  │ MCP Tools│  │ REST API         │  │
│  │ :3000    │  │   CLI    │  │          │  │ curl/scripts     │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────────┬─────────┘  │
└───────┼─────────────┼─────────────┼─────────────────┼────────────┘
        │             │             │                 │
        └─────────────┴─────────────┴─────────────────┘
                              │
                     ┌─────────▼─────────┐
                    │    Rice API       │
                    │   (NestJS :8080)  │
                    │   + MCP Server    │
                    └─────────┬─────────┘
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
┌───────▼───────┐   ┌─────────▼─────────┐   ┌──────▼──────┐
│   Tantivy     │   │      Milvus       │   │    TEI      │
│   (BM25)      │   │   (Vectors)       │   │ (Embeddings)│
│   Rust CLI    │   │   :19530          │   │    :8081    │
└───────────────┘   └─────────┬─────────┘   └─────────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
        ┌─────▼─────┐   ┌─────▼─────┐   ┌─────▼─────┐
        │   etcd    │   │   MinIO   │   │   Attu    │
        │ (metadata)│   │ (storage) │   │  (Admin)  │
        │           │   │ :9000     │   │  :8000    │
        └───────────┘   └───────────┘   └───────────┘
```

## Quick Start

### Prerequisites

- Docker & Docker Compose
- 8GB+ RAM recommended
- For Windows: WSL2 with Docker Desktop

### 1. Start the Platform

```bash
# Clone and start
git clone <repo>
cd rice-search

# Start all services (first run downloads ~5GB of images)
docker-compose up -d

# Wait for services to be healthy (~2-3 minutes)
docker-compose ps
```

### 2. Index Your Code

```bash
# Using the reindex script (incremental by default)
python scripts/reindex.py /path/to/your/repo

# Force re-index all files
python scripts/reindex.py /path/to/your/repo --force

# Sync deleted files (remove from index files no longer on disk)
python scripts/reindex.py /path/to/your/repo --sync

# Show indexing statistics
python scripts/reindex.py /path/to/your/repo --stats

# Or via API
curl -X POST http://localhost:8080/v1/stores/default/index \
  -H "Content-Type: application/json" \
  -d '{
    "files": [
      {"path": "src/main.py", "content": "def hello():\n    print(\"world\")"}
    ]
  }'
```

### 3. Search

**Web UI**: Open http://localhost:3000

**API**:
```bash
curl -X POST http://localhost:8080/v1/search/default \
  -H "Content-Type: application/json" \
  -d '{"query": "hello world function", "top_k": 10}'
```

**MCP Tool Call**:
```bash
curl -X POST http://localhost:8080/mcp/tools/call \
  -H "Content-Type: application/json" \
  -d '{
    "name": "code_search",
    "arguments": {"query": "authentication handler", "top_k": 5}
  }'
```

### 4. Verify Installation

```bash
bash scripts/smoke_test.sh
```

## API Reference

### Search

```
POST /v1/search/{store}
```

Request body:
```json
{
  "query": "search query",
  "top_k": 20,
  "include_content": true,
  "filters": {
    "path_prefix": "src/",
    "languages": ["python", "typescript"]
  },
  "sparse_weight": 0.5,
  "dense_weight": 0.5
}
```

### Index Files

```
POST /v1/stores/{store}/index
```

Request body:
```json
{
  "files": [
    {"path": "relative/path.py", "content": "file content..."}
  ],
  "force": false
}
```

Response:
```json
{
  "files_processed": 100,
  "chunks_indexed": 45,
  "skipped_unchanged": 55,
  "time_ms": 1234
}
```

**Incremental Indexing**: By default, only changed files are re-indexed. Use `"force": true` to re-index all files.

### Sync Deleted Files

Remove files from index that no longer exist on disk:

```
POST /v1/stores/{store}/index/sync
```

Request body:
```json
{
  "current_paths": ["src/main.py", "src/utils.py", ...]
}
```

### Get Index Stats

```
GET /v1/stores/{store}/index/stats
```

Response:
```json
{
  "tracked_files": 150,
  "total_size": 1048576,
  "last_updated": "2025-12-27T02:30:00.000Z"
}
```

### MCP Endpoints

Rice Search provides native Model Context Protocol (MCP) support for AI assistant integration:

```
GET  /mcp/tools           # List available MCP tools
POST /mcp/tools/call      # Call an MCP tool directly
POST /mcp                 # JSON-RPC 2.0 endpoint for MCP protocol
```

**Available MCP Tools:**
- `code_search` - Hybrid semantic + keyword code search
- `index_files` - Index code files into a store
- `list_stores` - List available search indexes

**MCP Protocol Support:**
- Full JSON-RPC 2.0 compliance
- Tool discovery via `/mcp/tools`
- Direct tool execution via `/mcp/tools/call`
- Standard MCP server interface via `/mcp`

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `MILVUS_HOST` | `milvus` | Milvus server host |
| `MILVUS_PORT` | `19530` | Milvus server port |
| `EMBEDDINGS_URL` | `http://embeddings:80` | TEI service URL |
| `EMBEDDING_DIM` | `768` | Embedding dimension |
| `SPARSE_TOPK` | `200` | BM25 candidates |
| `DENSE_TOPK` | `80` | Vector search candidates |
| `DATA_DIR` | `/data` | Data directory |

### GPU Acceleration

For NVIDIA GPU support:

```bash
docker-compose -f docker-compose.yml -f docker-compose.gpu.yml up -d
```

Requirements:
- NVIDIA GPU with CUDA support
- nvidia-container-toolkit installed

## Data Persistence

All data is stored in `./data/`:

```
./data/
├── etcd/           # Milvus metadata
├── minio/          # Milvus object storage
├── milvus/         # Milvus data files
├── tantivy/        # BM25 index
├── api/            # Store metadata
└── embeddings-cache/  # Model cache
```

To reset: `rm -rf ./data && docker-compose down -v`

## Stores

Stores are isolated search indexes. Default store is `default`.

```bash
# Create a new store
python scripts/init_store.py my-project --description "My Project"

# List stores
python scripts/init_store.py --list

# Index into specific store
python scripts/reindex.py /path/to/repo --store my-project
```

## MCP Integration

Rice Search provides full Model Context Protocol (MCP) support, allowing AI assistants to search and index code directly.

### Using with MCP Clients

Add to your MCP client configuration:

```json
{
  "mcpServers": {
    "rice-search": {
      "url": "http://localhost:8080/mcp",
      "transport": "http"
    }
  }
}
```

### Using with ricegrep CLI

**ricegrep** includes built-in MCP server support for local AI assistant integration. The CLI can act as an MCP server, allowing coding assistants to use Rice Search directly:

```bash
# Install ricegrep CLI
cd ricegrep
npm install -g .

# Start MCP server mode (used by AI assistants)
ricegrep watch-mcp

# Or use ricegrep's assisted installation for popular AI assistants:
ricegrep install-claude-code   # Claude Code integration
ricegrep install-opencode       # OpenCode integration
ricegrep install-codex          # Codex integration
ricegrep install-droid          # Factory Droid integration
```

When running in MCP mode, ricegrep provides:
- **Real-time indexing** - Automatically indexes files as you code
- **Semantic search** - AI assistants can search using natural language
- **Local-first** - All data stays on your machine
- **Zero configuration** - Works out of the box with Rice Search backend

### MCP Tools Available

All MCP tools (whether via HTTP API or ricegrep CLI):

- **`code_search`** - Hybrid search combining BM25 + semantic embeddings
  - Parameters: `query`, `top_k`, `store`, `filters`
  - Returns: Ranked code chunks with paths, line numbers, and snippets

- **`index_files`** - Index code files for search
  - Parameters: `files[]`, `store`, `force`
  - Returns: Indexing statistics

- **`list_stores`** - List available code search indexes
  - Returns: Array of store names and metadata

## Troubleshooting

### Services not starting

```bash
# Check logs
docker-compose logs milvus
docker-compose logs embeddings

# Restart services
docker-compose restart
```

### Out of memory

Reduce embedding batch size in `.env`:
```
MAX_FILES_PER_BATCH=20
```

### WSL2 Performance

Mount your code inside WSL2 for best performance:
```bash
# Inside WSL2
cp -r /mnt/c/code/myproject ~/myproject
python scripts/reindex.py ~/myproject
```

### Search returns no results

1. Check if files were indexed:
   ```bash
   curl http://localhost:8080/v1/stores/default/stats
   ```

2. Try a simpler query
3. Check index logs: `docker-compose logs unified-api`

## Development

### Build locally

```bash
cd unified-api
npm install
npm run build
npm run start:dev
```

### Run tests

```bash
npm test
```

## License

MIT

## Credits

- [Tantivy](https://github.com/quickwit-oss/tantivy) - BM25 search engine
- [Milvus](https://milvus.io/) - Vector database
- [TEI](https://github.com/huggingface/text-embeddings-inference) - Embedding service
- [NestJS](https://nestjs.com/) - API framework
