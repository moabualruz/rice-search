# Rice Search Client

A command-line tool for indexing and searching code with the Rice Search platform.

## Installation

```bash
cd backend
pip install -e .
```

## Commands

### Search

```bash
# Basic search
ricesearch search "authentication"

# With options
ricesearch search "database connection" --limit 20 --org-id myorg

# Dense-only search (no hybrid)
ricesearch search "function" --no-hybrid
```

**Output format:** `file:chunk_index: content_preview (score)`

### Watch

```bash
# Watch current directory
ricesearch watch

# Watch specific path
ricesearch watch ./src

# Skip initial scan
ricesearch watch ./src --no-initial

# With organization
ricesearch watch . --org-id myorg
```

Features:

- Real-time filesystem monitoring
- Incremental indexing (skips unchanged files)
- Respects `.gitignore` and `.riceignore`
- Debounced updates (waits 500ms after last change)

### Config

```bash
# Show config
ricesearch config show

# Set backend URL
ricesearch config set backend_url http://localhost:8000

# Set default org
ricesearch config set org_id myorg
```

Config stored in: `~/.ricesearch/config.yaml`

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
