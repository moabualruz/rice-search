# Centralized Settings Management System

## Overview

Rice Search now uses a **centralized settings management system** where:
- **All settings** are defined in `backend/settings.yaml`
- **No hardcoded values** in code (except log messages and comments)
- **Redis** serves as the single source of truth at runtime
- **Environment variables** can override any setting using `RICE_SEARCH_` prefix
- **Runtime updates** are supported via REST API
- **Automatic file persistence** - ALL changes are immediately saved to YAML file
- **No manual save required** - Every update persists automatically

## Architecture

```
┌─────────────────┐
│ settings.yaml   │ ← Base configuration (version controlled)
└────────┬────────┘
         │
         ↓ (Load on startup)
┌─────────────────┐
│ Environment     │ ← Override via RICE_SEARCH_* env vars
│ Variables       │
└────────┬────────┘
         │
         ↓ (Apply overrides)
┌─────────────────┐
│ Redis           │ ← Single source of truth at runtime
│ (Runtime Store) │   - Fast read/write
└────────┬────────┘   - Version tracking
         │             - Thread-safe
         ↓
┌─────────────────┐
│ SettingsManager │ ← Provides dot-notation access
│ & Settings API  │   - get("models.embedding.dimension")
└─────────────────┘   - Runtime updates via API

         ↓ (AUTOMATIC on every update)
┌─────────────────┐
│ settings.yaml   │ ← All changes automatically persisted
│ (Auto-Updated)  │   to file immediately
└─────────────────┘
```

**Key Feature**: All settings updates are **automatically persisted** to the YAML file. No separate save action required.

## Settings File Structure

**Location:** `backend/settings.yaml`

```yaml
# Application
app:
  name: "Rice Search"
  version: "1.0.0"
  api_prefix: "/api/v1"

# Infrastructure
infrastructure:
  qdrant:
    url: "http://localhost:6333"
  redis:
    url: "redis://localhost:6379/0"

# Models
models:
  embedding:
    name: "jina-embeddings-v3"
    dimension: 1024
    ollama_model: "qwen3-embedding:4b"

# Search Configuration
search:
  default_limit: 10
  hybrid:
    enabled: true
    rrf_k: 60

# Messages (NO hardcoded strings in code)
messages:
  errors:
    model_not_found: "Model {model_name} not found"
  success:
    file_indexed: "File {file_path} indexed successfully"
```

## Environment Variable Overrides

Override any setting using the `RICE_SEARCH_` prefix:

```bash
# Convention: RICE_SEARCH_<SECTION>_<SUBSECTION>_<KEY>
# Maps to: section.subsection.key

# Example: Override embedding dimension
export RICE_SEARCH_MODELS_EMBEDDING_DIMENSION=768

# Example: Override Redis URL
export RICE_SEARCH_INFRASTRUCTURE_REDIS_URL=redis://my-redis:6379/1

# Example: Override search limit
export RICE_SEARCH_SEARCH_DEFAULT_LIMIT=20
```

**Docker Compose Example:**
```yaml
environment:
  - RICE_SEARCH_INFRASTRUCTURE_REDIS_URL=redis://redis:6379/0
  - RICE_SEARCH_INFERENCE_OLLAMA_BASE_URL=http://ollama:11434
  - RICE_SEARCH_MODELS_EMBEDDING_DIMENSION=1024
```

## Using Settings in Code

### Old Way (Hardcoded - DON'T DO THIS)
```python
# ❌ BAD - Hardcoded string
model_name = "jina-embeddings-v3"
```

### New Way (From Settings)
```python
from src.core.config import settings, get_message

# ✅ GOOD - From centralized settings
model_name = settings.EMBEDDING_MODEL
# or
model_name = settings.get("models.embedding.name")

# ✅ GOOD - Messages from settings
error_msg = get_message("errors.model_not_found", model_name="bert")
# Returns: "Model bert not found"
```

### Backward Compatibility

The `Settings` class provides backward-compatible attribute access:

```python
from src.core.config import settings

# Old attribute style (still works)
project_name = settings.PROJECT_NAME  # → "Rice Search"
redis_url = settings.REDIS_URL        # → "redis://localhost:6379/0"

# New dot-notation style (recommended)
project_name = settings.get("app.name")
redis_url = settings.get("infrastructure.redis.url")
```

## Runtime Settings API

**IMPORTANT**: All settings updates are automatically persisted to the YAML file. Changes are immediately written to both Redis (runtime) and `backend/settings.yaml` (persistent storage).

### Get Settings

```bash
# Get all settings
GET /api/v1/settings/

# Get specific setting
GET /api/v1/settings/models.embedding.dimension

# Get all settings with prefix
GET /api/v1/settings/?prefix=models

# Get nested structure
GET /api/v1/settings/nested/models
```

### Update Settings (Admin only)

```bash
# Update single setting (automatically persisted to file)
PUT /api/v1/settings/search.default_limit
{
  "value": 20
}

# Bulk update (automatically persisted to file)
POST /api/v1/settings/bulk
{
  "settings": {
    "search.default_limit": 20,
    "search.max_limit": 200
  }
}
```

### Delete Settings

```bash
# Delete setting (automatically persisted to file)
DELETE /api/v1/settings/custom.feature.flag
```

### Reload Settings

```bash
# Reload from YAML (discards non-persisted runtime changes)
POST /api/v1/settings/reload
```

### Get Version

```bash
# Check if settings have changed (version increments on each update)
GET /api/v1/settings/version/current
```

## Settings Manager API

For direct programmatic access:

```python
from src.core.settings_manager import get_settings_manager

manager = get_settings_manager()

# Read
value = manager.get("models.embedding.dimension")
all_models = manager.get_all("models")
nested = manager.get_nested("models")

# Write (always persists to file)
manager.set("search.default_limit", 20, persist=True)  # persist=True is default
manager.delete("old.setting", persist=True)  # persist=True is default

# Reload
manager.reload()  # Reload from YAML file (discards runtime changes)

# Version tracking
version = manager.get_version()  # Increments on each change
```

**Note**: While the API still accepts a `persist` parameter for backward compatibility, it's recommended to always use `persist=True` (the default) to ensure changes are saved to the file.

## Benefits

### 1. **Single Source of Truth**
- No conflicting values scattered across code
- Easy to find and update any setting
- Clear configuration hierarchy

### 2. **No Hardcoded Values**
- All strings, numbers, URLs in settings.yaml
- Easy to change without code modifications
- Better for internationalization

### 3. **Environment Flexibility**
- Easy dev/staging/prod configuration
- Docker-friendly with env var overrides
- Secrets can be injected via env vars

### 4. **Runtime Updates**
- Change settings without restart
- Admin UI can modify configuration
- Settings persist across restarts

### 5. **Version Tracking**
- Know when settings changed
- Audit trail in Redis
- Can detect stale cached values

### 6. **Type Safety**
- Automatic type conversion
- Schema validation possible
- IDE autocomplete works

## Migration Guide

### Converting Hardcoded Values

**Before:**
```python
# Old hardcoded value
CHUNK_SIZE = 512
ERROR_MESSAGE = "File not found"

def process_file(path):
    chunks = split_text(content, chunk_size=CHUNK_SIZE)
    if not os.path.exists(path):
        raise ValueError(ERROR_MESSAGE)
```

**After:**
```python
from src.core.config import settings, get_message

def process_file(path):
    chunk_size = settings.get("indexing.chunk_size")
    chunks = split_text(content, chunk_size=chunk_size)

    if not os.path.exists(path):
        msg = get_message("errors.file_not_found", path=path)
        raise ValueError(msg)
```

### Adding New Settings

1. **Add to settings.yaml:**
```yaml
my_feature:
  enabled: true
  timeout: 30
  message: "Feature is {status}"
```

2. **Add mapping in config.py (for backward compatibility):**
```python
# In Settings.__getattr__ mappings dict:
"MY_FEATURE_ENABLED": "my_feature.enabled",
"MY_FEATURE_TIMEOUT": "my_feature.timeout",
```

3. **Use in code:**
```python
if settings.MY_FEATURE_ENABLED:
    timeout = settings.get("my_feature.timeout")
    msg = get_message("my_feature.message", status="active")
```

## Best Practices

1. **Always use settings, never hardcode:**
   ```python
   # ❌ BAD
   url = "http://localhost:8000"

   # ✅ GOOD
   url = settings.get("server.url")
   ```

2. **Use messages from settings:**
   ```python
   # ❌ BAD
   logger.error("Model not found")

   # ✅ GOOD
   logger.error(get_message("errors.model_not_found"))
   ```

3. **Provide defaults:**
   ```python
   limit = settings.get("search.limit", 10)  # Default to 10
   ```

4. **All changes are automatically persisted:**
   ```python
   # Changes are automatically saved to both Redis and YAML file
   manager.set("models.active", "new-model")

   # No need to specify persist=True (it's automatic)
   ```

5. **Use environment variables for secrets:**
   ```bash
   export RICE_SEARCH_AUTH_JWT_SECRET=my-secret-key
   # Don't commit secrets to settings.yaml!
   ```

6. **Reload settings only when necessary:**
   ```python
   # Only reload if you need to discard runtime changes
   # and revert to file version
   manager.reload()  # Warning: discards all runtime changes!
   ```

## Troubleshooting

### Settings not loading
```bash
# Check Redis connection
docker exec backend-api redis-cli -u redis://redis:6379/0 ping

# Check settings file exists
ls -la backend/settings.yaml

# Check logs
docker logs backend-api | grep -i settings
```

### Environment override not working
```bash
# Verify env var format
echo $RICE_SEARCH_MODELS_EMBEDDING_DIMENSION

# Must use RICE_SEARCH_ prefix and underscores
# Correct: RICE_SEARCH_MODELS_EMBEDDING_DIMENSION
# Wrong: MODELS_EMBEDDING_DIMENSION
```

### Changes not persisting
```bash
# Ensure persist=true
curl -X PUT http://localhost:8000/api/v1/settings/my.key \
  -H "Content-Type: application/json" \
  -d '{"value": "new-value", "persist": true}'

# Check file was updated
cat backend/settings.yaml | grep my.key
```

## Security Considerations

1. **Protect the settings API** - Requires admin role
2. **Don't commit secrets** - Use env vars for sensitive data
3. **Validate input** - Settings API validates types
4. **Audit changes** - Redis stores version history
5. **Backup settings.yaml** - Include in version control

## Admin UI - Settings Manager

A comprehensive web-based settings editor is available at `/admin/settings`:

### Features

1. **Visual Editor**
   - Browse all settings organized by category
   - Real-time search and filtering
   - Category-based navigation
   - Type-aware editing (string, number, boolean, arrays, objects)

2. **Change Tracking**
   - See which settings have been modified
   - Visual indicators for unsaved changes
   - Bulk save all changes at once
   - Reset individual or all changes

3. **Version Control**
   - Settings version displayed in header
   - Version increments on each save
   - Track when configuration was last updated

4. **Actions**
   - **Save All Changes** - Persist all modifications to Redis and YAML
   - **Reload from File** - Discard runtime changes and reload from settings.yaml
   - **Reset Changes** - Discard unsaved modifications
   - **Individual Edit** - Edit and save settings one at a time

5. **Category Icons**
   - 🚀 App - Application metadata
   - 🖥️ Server - Server configuration
   - 🗄️ Infrastructure - Database and cache settings
   - 🔐 Auth - Authentication settings
   - 🤖 Inference - AI inference configuration
   - 🧠 Models - Model parameters
   - 🔍 Search - Search and retrieval settings
   - 🌳 AST - Code parsing settings
   - 🔌 MCP - Model Context Protocol
   - ⚡ Model Management - Resource optimization
   - 💬 RAG - RAG engine settings
   - 📑 Indexing - Document processing
   - ⚙️ Worker - Celery worker configuration
   - 📝 Logging - Log settings
   - 📊 Telemetry - Observability
   - ✨ Features - Feature flags
   - 👑 Admin - Admin settings
   - 📈 Metrics - Metrics configuration
   - 💻 CLI - CLI tool settings

### Access

```bash
# Navigate to settings page
http://localhost:3000/admin/settings

# Or from admin dashboard
http://localhost:3000/admin → Click "Settings Manager"
```

### UI Screenshots

**Main Settings View:**
- Grouped by category with collapsible sections
- Search bar for quick filtering
- Category dropdown for focused browsing
- Action buttons in top bar

**Setting Row:**
- Setting key in monospace font
- Type badge (string, number, boolean, array, object)
- Modified indicator for unsaved changes
- Edit button to toggle editing mode
- Save/Cancel buttons when editing

**Value Display:**
- Booleans: Toggle switch with visual state
- Numbers: Numeric input with validation
- Strings: Text input
- Arrays: List view with JSON editing
- Objects: JSON editor with syntax support

## Future Enhancements

- [x] Settings editor in admin UI
- [x] Settings diff viewer (modified indicator)
- [ ] Schema validation with Pydantic
- [ ] Settings change webhooks
- [ ] Import/export configuration profiles
- [ ] Encrypted secrets in Redis
- [ ] Multi-environment profiles (dev/staging/prod)
- [ ] Settings history/audit log
- [ ] Rollback to previous versions
