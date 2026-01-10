# CLI Guide - ricesearch

Complete guide to the `ricesearch` command-line interface for indexing and searching code.

## Table of Contents

- [Installation](#installation)
- [Commands Overview](#commands-overview)
- [Search Command](#search-command)
- [Watch Command](#watch-command)
- [Config Command](#config-command)
- [Version Command](#version-command)
- [Configuration File](#configuration-file)
- [Ignore Patterns (.riceignore)](#ignore-patterns-riceignore)
- [Common Workflows](#common-workflows)
- [Troubleshooting](#troubleshooting)

---

## Installation

### Prerequisites

- **Python 3.12+**
- **Rice Search backend running** (see [Getting Started](getting-started.md))
- **Backend API accessible** at <http://localhost:8000> (default)

### Install CLI

```bash
# Navigate to backend directory
cd backend

# Install in development mode
pip install -e .

# Verify installation
ricesearch --version
# Expected output: ricesearch 0.1.0
```

The CLI is installed as the `ricesearch` command and available system-wide.

---

## Commands Overview

```bash
ricesearch --help

# Available commands:
ricesearch search <query>     # Search indexed code
ricesearch watch <path>       # Watch directory and auto-index changes
ricesearch config <action>    # Manage configuration
ricesearch version            # Show version information
```

All commands support `--help` for detailed usage:

```bash
ricesearch search --help
ricesearch watch --help
ricesearch config --help
```

---

## Search Command

Search indexed code and display results in the terminal.

### Basic Usage

```bash
ricesearch search "authentication"
```

**Expected output:**
```
1. F:/work/rice-search/backend/src/core/config.py (score: 85%)
   Configuration module - wraps SettingsManager for...

2. F:/work/rice-search/backend/src/services/auth.py (score: 78%)
   Authentication service implementation...

3. F:/work/rice-search/backend/src/api/auth.py (score: 70%)
   Auth API endpoints...
```

### Search Options

```bash
ricesearch search [OPTIONS] <query>

Options:
  --limit INTEGER       Number of results to return (default: 10)
  --org-id TEXT        Filter by organization ID (default: all orgs)
  --no-hybrid          Disable hybrid search (use dense-only)
  --no-color           Disable colored output
  --help               Show help message
```

### Examples

**Limit results:**
```bash
# Get top 20 results
ricesearch search "configuration" --limit 20
```

**Filter by organization:**
```bash
# Search only backend code
ricesearch search "database" --org-id backend

# Search only frontend code
ricesearch search "component" --org-id frontend
```

**Disable hybrid search:**
```bash
# Use dense semantic search only
ricesearch search "machine learning" --no-hybrid
```

**Plain output (no colors):**
```bash
# For piping to files or other commands
ricesearch search "error handling" --no-color > results.txt
```

### Search for Filenames

```bash
# Find specific files
ricesearch search "Dockerfile"
ricesearch search "settings.yaml"
ricesearch search "main.py"

# Search by path pattern
ricesearch search "backend/src/core"
```

### Complex Queries

```bash
# Multi-word queries (use quotes)
ricesearch search "user authentication flow"

# Technical terms
ricesearch search "OAuth implementation"

# Code concepts
ricesearch search "async task processing"
```

### Output Format

Results include:
- **Rank** - Position in results (1, 2, 3...)
- **File path** - Full system path to the file
- **Score** - Relevance score (12-98%)
- **Snippet** - Relevant text excerpt from the file

**Score ranges:**

- **80-98%** - Highly relevant (exact matches, strong semantic similarity)
- **50-79%** - Moderately relevant (partial matches, related concepts)
- **12-49%** - Weakly relevant (tangential matches)

---

## Watch Command

Monitor a directory for file changes and automatically index updates.

### Basic Usage

```bash
# Watch current directory
ricesearch watch

# Watch specific directory
ricesearch watch ./backend
```

**Expected output:**
```
[INFO] Starting file watcher for: ./backend
[INFO] Organization ID: public
[INFO] Initial scan...
[INDEXING] ./backend/src/main.py
[OK] ./backend/src/main.py
[INDEXING] ./backend/src/core/config.py
[OK] ./backend/src/core/config.py
...
[INFO] Indexed 45 files, skipped 12, failed 0
[INFO] Watching for changes... (Ctrl+C to stop)
```

### Watch Options

```bash
ricesearch watch [OPTIONS] [path]

Options:
  path                 Directory to watch (default: current directory)
  --org-id TEXT       Organization ID for indexing (default: "public")
  --no-initial        Skip initial scan, only watch for changes
  --help              Show help message
```

### Examples

**Watch with custom organization:**
```bash
# Watch backend code with org-id "backend"
ricesearch watch ./backend --org-id backend

# Watch frontend code with org-id "frontend"
ricesearch watch ./frontend --org-id frontend
```

**Skip initial scan:**
```bash
# Only index new changes (skip existing files)
ricesearch watch ./src --no-initial
```

**Multiple watchers:**
```bash
# Terminal 1: Watch backend
ricesearch watch ./backend --org-id backend

# Terminal 2: Watch frontend
ricesearch watch ./frontend --org-id frontend

# Terminal 3: Watch Rust code
ricesearch watch ./rust-tantivy --org-id rust
```

### How It Works

1. **Initial Scan** (unless `--no-initial`):
   - Recursively scans directory for supported files
   - Respects `.riceignore` and `.gitignore` patterns
   - Indexes all files in parallel

2. **File Watching**:
   - Monitors for `created`, `modified`, `deleted` events
   - Uses file hashing to detect actual changes (ignores touch events)
   - Debounces rapid changes (500ms delay)
   - Automatically re-indexes changed files

3. **Deduplication**:
   - Tracks file hashes to avoid re-indexing identical content
   - Replaces old file versions in the database
   - Cleans up deleted files from the index

### Supported File Types

Watch automatically detects and indexes:

**Code**: `.py`, `.js`, `.ts`, `.tsx`, `.jsx`, `.go`, `.rs`, `.java`, `.cpp`, `.c`, `.h`, `.hpp`
- **Docs**: `.md`, `.txt`, `.rst`, `.adoc`
- **Config**: `.yaml`, `.yml`, `.json`, `.toml`, `.ini`
- **Scripts**: `.sh`, `.bash`, `.zsh`, `.ps1`

### Stopping Watch

Press `Ctrl+C` to stop the watcher gracefully:

```
^C
[INFO] Stopping file watcher...
[INFO] Cleanup complete. Exited.
```

---

## Config Command

Manage CLI configuration settings.

### Show Configuration

```bash
# Display all settings
ricesearch config show

# Expected output:
backend_url: http://localhost:8000
org_id: public
hybrid_search: true
color_output: true
```

### Set Configuration

```bash
ricesearch config set <key> <value>

# Examples:
ricesearch config set backend_url http://localhost:8000
ricesearch config set org_id myproject
ricesearch config set hybrid_search false
ricesearch config set color_output false
```

### Available Settings

| Setting | Default | Description |
|---------|---------|-------------|
| `backend_url` | `http://localhost:8000` | Backend API endpoint |
| `org_id` | `public` | Default organization ID |
| `hybrid_search` | `true` | Enable hybrid search by default |
| `default_limit` | `10` | Default number of results |
| `user_id` | `admin-1` | User ID for requests |

### Configuration File Location

- **Linux/macOS/Windows**: `~/.ricesearch/config.yaml`

You can also edit this file directly (YAML format).

---

## Version Command

Display version information.

```bash
ricesearch version

# Output:
ricesearch 0.1.0
Python 3.12.1
Backend API: http://localhost:8000
```

Use `--version` shorthand:

```bash
ricesearch --version
```

---

## Configuration File

### config.yaml

The CLI stores persistent configuration in `config.yaml`:

```yaml
backend_url: "http://localhost:8000"
org_id: "public"
hybrid_search: true
default_limit: 10
user_id: "admin-1"
```

**File location:** `~/.ricesearch/config.yaml`

### Environment Variables

Override config via environment variables:

```bash
# Set backend URL
export RICE_BACKEND_URL=http://localhost:8000

# Set default org ID
export RICE_ORG_ID=myproject

# Disable hybrid search
export RICE_HYBRID_SEARCH=false

# Disable colors
export RICE_COLOR_OUTPUT=false
```

**Priority order:** Environment variables > config.json > defaults

---

## Ignore Patterns (.riceignore)

Control which files are indexed using `.riceignore` files.

### Creating .riceignore

Create a `.riceignore` file in your project root:

```bash
# Example .riceignore
node_modules/
__pycache__/
*.pyc
*.log
*.tmp
.git/
.env
venv/
dist/
build/
```

**Syntax:** Same as `.gitignore` (glob patterns)

### How It Works

1. **`.riceignore` discovered**: CLI checks for `.riceignore` in watched directory
2. **`.gitignore` respected**: If no `.riceignore`, falls back to `.gitignore`
3. **Patterns applied**: Files matching patterns are skipped during indexing

### Common Patterns

```bash
# Dependencies
node_modules/
venv/
__pycache__/

# Build artifacts
dist/
build/
target/
*.o
*.so

# Logs and temp files
*.log
*.tmp
*.cache

# Environment files
.env
.env.local
secrets/

# Version control
.git/
.svn/

# IDE files
.vscode/
.idea/
*.swp
```

### Multiple .riceignore Files

Place `.riceignore` in subdirectories for different rules:

```
project/
├── .riceignore          # Root rules (affects all)
├── backend/
│   └── .riceignore      # Backend-specific rules
└── frontend/
    └── .riceignore      # Frontend-specific rules
```

---

## Common Workflows

### Development Workflow

```bash
# 1. Start watching your code
ricesearch watch ./backend --org-id backend

# 2. Make changes to code
# (Files auto-index on save)

# 3. Search for your changes
ricesearch search "new feature"
```

### Multi-Project Indexing

```bash
# Index multiple projects with different org IDs
ricesearch watch ~/projects/backend --org-id backend
ricesearch watch ~/projects/frontend --org-id frontend
ricesearch watch ~/projects/mobile --org-id mobile

# Search specific project
ricesearch search "authentication" --org-id backend
```

### One-Time Indexing

```bash
# Index once without watching
# (Currently use watch then Ctrl+C after initial scan)
ricesearch watch ./src --no-initial
# Then Ctrl+C immediately after scan completes
```

### CI/CD Integration

```bash
# Index docs in CI pipeline
ricesearch watch ./docs --org-id docs --no-initial

# Wait for indexing to complete
# (watch blocks until stopped with Ctrl+C)
```

### Search Workflows

```bash
# 1. Broad search, then narrow down
ricesearch search "database" --limit 50
ricesearch search "database connection pool" --limit 10

# 2. Search by file type
ricesearch search "Dockerfile"
ricesearch search ".github/workflows"

# 3. Search within org
ricesearch search "config" --org-id backend
```

---

## Troubleshooting

### Command Not Found

**Problem:**
```bash
ricesearch: command not found
```

**Solution:**
```bash
# Ensure CLI is installed
cd backend
pip install -e .

# Check Python bin directory is in PATH
which ricesearch  # Should show path to command
```

### Backend Connection Failed

**Problem:**
```
[ERROR] Backend API not reachable: http://localhost:8000
```

**Solution:**
```bash
# 1. Check backend is running
curl http://localhost:8000/health

# 2. Verify backend URL
ricesearch config show

# 3. Update backend URL if needed
ricesearch config set backend_url http://localhost:8000
```

### No Results Found

**Problem:**
```bash
ricesearch search "my code"
# Returns: No results found
```

**Solution:**
```bash
# 1. Check if files are indexed
curl http://localhost:8000/api/v1/files/list

# 2. Verify Qdrant has data
curl http://localhost:6333/collections/rice_chunks

# 3. Re-index files
ricesearch watch ./src
```

### Watch Not Indexing Files

**Problem:** Watch command skips all files.

**Solution:**
```bash
# 1. Check .riceignore patterns
cat .riceignore

# 2. Verify file types are supported
# Only .py, .js, .ts, .go, .rs, .md, etc.

# 3. Check backend worker logs
docker compose -f deploy/docker-compose.yml logs backend-worker

# 4. Test API directly
curl -X POST http://localhost:8000/api/v1/ingest/file \
  -F "file=@test.py" \
  -F "org_id=public"
```

### Slow Indexing

**Problem:** Watch takes very long to index files.

**Solution:**
```bash
# 1. Add .riceignore to skip large directories
echo "node_modules/" >> .riceignore
echo "__pycache__/" >> .riceignore

# 2. Check backend worker status
docker compose -f deploy/docker-compose.yml ps backend-worker

# 3. Monitor worker logs
docker compose -f deploy/docker-compose.yml logs -f backend-worker

# 4. Check Celery queue size
# (Large queue means worker is slow)
```

### Duplicate Results

**Problem:** Search shows same file multiple times.

**Solution:** This should be fixed automatically by deduplication. If you see duplicates:

```bash
# 1. Re-index the file
ricesearch watch ./path/to/file

# 2. Check backend logs for errors
docker compose -f deploy/docker-compose.yml logs backend-api

# 3. Verify Qdrant deduplication
curl -X POST http://localhost:6333/collections/rice_chunks/points/scroll \
  -H "Content-Type: application/json" \
  -d '{"filter": {"must": [{"key": "full_path", "match": {"value": "..."}}]}}'
```

### Colors Not Working

**Problem:** Terminal output has no colors or shows escape codes.

**Solution:**
```bash
# 1. Check terminal supports colors
echo $TERM  # Should show "xterm-256color" or similar

# 2. Disable colors explicitly
ricesearch search "test" --no-color

# 3. Update config
ricesearch config set color_output false
```

---

## Advanced Usage

### Scripting with ricesearch

```bash
#!/bin/bash
# index-all.sh - Index multiple directories

echo "Indexing backend..."
ricesearch watch ./backend --org-id backend &
BACKEND_PID=$!

echo "Indexing frontend..."
ricesearch watch ./frontend --org-id frontend &
FRONTEND_PID=$!

echo "Waiting for Ctrl+C..."
wait $BACKEND_PID $FRONTEND_PID
```

### Parsing Output

```bash
# Extract file paths from search results
ricesearch search "config" --no-color | grep -oP 'F:/[^\s]+'

# Count results
ricesearch search "authentication" | grep -c "^[0-9]\\."

# Export to JSON (via API)
curl -X POST http://localhost:8000/api/v1/search/query \
  -H "Content-Type: application/json" \
  -d '{"query": "authentication", "limit": 10}' | jq .
```

### Custom Backend URL

```bash
# Use remote backend
ricesearch config set backend_url https://rice-search.example.com

# Or use environment variable
export RICE_BACKEND_URL=https://rice-search.example.com
ricesearch search "test"
```

---

## Summary

**Quick Commands:**
```bash
ricesearch search "query"                    # Search
ricesearch watch ./dir --org-id myorg        # Watch and index
ricesearch config show                       # Show config
ricesearch config set backend_url <url>      # Set config
ricesearch version                           # Version info
```

**Key Features:**

- Fast search with hybrid retrieval (BM25 + semantic)
- Auto-indexing with file watching
- Respects `.riceignore` and `.gitignore`
- Colored terminal output
- Organization-based filtering
- Full file path display

For more details, see:
- [Getting Started](getting-started.md) - Installation and first run
- [Configuration](configuration.md) - Backend settings
- [API Reference](api.md) - REST API endpoints
- [Troubleshooting](troubleshooting.md) - Common issues

---

**[Back to Documentation Index](README.md)**
