# ricegrep - Rice Search CLI

A grep-like command-line tool for indexing and searching code with Rice Search.

## Installation

```bash
cd backend
pip install -e .
```

## Commands

### Search

```bash
# Basic search
ricegrep search "authentication"

# With options
ricegrep search "database connection" --limit 20 --org-id myorg

# Dense-only search (no hybrid)
ricegrep search "function" --no-hybrid
```

**Output format:** `file:chunk_index: content_preview (score)`

### Watch

```bash
# Watch current directory
ricegrep watch

# Watch specific path
ricegrep watch ./src

# Skip initial scan
ricegrep watch ./src --no-initial

# With organization
ricegrep watch . --org-id myorg
```

Features:

- Real-time filesystem monitoring
- Incremental indexing (skips unchanged files)
- Respects `.gitignore` and `.riceignore`
- Debounced updates (waits 500ms after last change)

### Config

```bash
# Show config
ricegrep config show

# Set backend URL
ricegrep config set backend_url http://localhost:8000

# Set default org
ricegrep config set org_id myorg
```

Config stored in: `~/.ricegrep/config.yaml`

## Ignore Rules

Create `.riceignore` in your project root:

```gitignore
# Ignore test fixtures
test/fixtures/

# Ignore generated files
*.generated.py

# But include this important file
!important.py
```

Default ignores:

- `.git/`, `node_modules/`, `__pycache__/`
- `.venv/`, `dist/`, `build/`
- `*.pyc`, `*.log`, `.DS_Store`

## Requirements

- Rice Search backend running on `http://localhost:8000`
- Python 3.11+
