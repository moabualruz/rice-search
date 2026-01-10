# Development Guide

Complete guide for developing, testing, and contributing to Rice Search.

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Running Services Locally](#running-services-locally)
- [Code Style & Standards](#code-style--standards)
- [Testing](#testing)
- [Debugging](#debugging)
- [Contributing](#contributing)
- [Common Development Tasks](#common-development-tasks)

---

## Development Environment Setup

### Prerequisites

**Required:**
- Docker Desktop 20.10+ (includes Docker Compose)
- Python 3.12+
- Node.js 18+ (for frontend development)
- Git

**Optional:**
- VS Code or PyCharm (recommended IDEs)
- NVIDIA GPU + CUDA (for faster inference)
- Rust 1.70+ (for Tantivy development)

### Initial Setup

**1. Clone Repository**

```bash
git clone https://github.com/yourusername/rice-search.git
cd rice-search
```

**2. Install Backend Dependencies**

```bash
# Create virtual environment
cd backend
python -m venv venv

# Activate virtual environment
source venv/bin/activate  # Linux/macOS
venv\Scripts\activate      # Windows

# Install in development mode
pip install -e .[dev]

# Verify installation
python -c "import src; print('Backend installed successfully')"
```

**3. Install Frontend Dependencies**

```bash
cd frontend
npm install

# Verify installation
npm run lint
```

**4. Start Services**

```bash
# From project root
make up

# Wait for services to be healthy
docker compose -f deploy/docker-compose.yml ps
```

**5. Verify Setup**

```bash
# Check backend health
curl http://localhost:8000/health

# Check frontend (browser)
open http://localhost:3000

# Run tests
cd backend
pytest tests/ -v
```

---

## Project Structure

```
rice-search/
├── backend/                  # Python backend
│   ├── src/                  # Source code
│   │   ├── api/              # REST API endpoints
│   │   ├── services/         # Business logic
│   │   ├── tasks/            # Celery async tasks
│   │   ├── worker/           # Worker startup
│   │   ├── core/             # Config, security
│   │   ├── db/               # Database clients
│   │   └── cli/              # CLI tool
│   ├── tests/                # Unit + integration tests
│   │   ├── unit/             # Unit tests
│   │   ├── integration/      # Integration tests
│   │   └── e2e/              # End-to-end tests
│   ├── settings.yaml         # Configuration
│   ├── pyproject.toml        # Python dependencies
│   ├── pytest.ini            # Pytest config
│   └── requirements.txt      # Frozen dependencies
│
├── frontend/                 # Next.js frontend
│   ├── src/
│   │   ├── app/              # Pages
│   │   ├── components/       # React components
│   │   └── lib/              # Utilities
│   ├── package.json          # Node dependencies
│   ├── tsconfig.json         # TypeScript config
│   └── next.config.js        # Next.js config
│
├── rust-tantivy/             # Rust BM25 service
│   ├── src/                  # Rust source
│   ├── Cargo.toml            # Rust dependencies
│   └── Dockerfile            # Container config
│
├── deploy/                   # Docker orchestration
│   ├── docker-compose.yml    # Main services
│   ├── .env.example          # Environment template
│   └── bentoml/              # BentoML service
│
├── docs/                     # Documentation
├── Makefile                  # Build commands
├── CLAUDE.md                 # AI assistant guide
└── README.md                 # Project overview
```

---

## Development Workflow

### 1. Feature Development

```bash
# 1. Create feature branch
git checkout -b feature/my-feature

# 2. Make changes to code
# Edit backend/src/services/...

# 3. Run tests
cd backend
pytest tests/test_my_feature.py

# 4. Test manually
curl -X POST http://localhost:8000/api/v1/...

# 5. Commit changes
git add .
git commit -m "feat: Add my feature"

# 6. Push and create PR
git push origin feature/my-feature
```

### 2. Bug Fix Workflow

```bash
# 1. Create bug fix branch
git checkout -b fix/issue-123

# 2. Write failing test (TDD)
# tests/test_my_bug.py
def test_reproduces_bug():
    # Test that fails before fix
    pass

# 3. Fix the bug
# src/services/...

# 4. Verify test passes
pytest tests/test_my_bug.py

# 5. Commit and push
git commit -am "fix: Resolve issue #123"
git push origin fix/issue-123
```

### 3. Code Review Process

1. **Self-Review**
   - Run linters: `flake8 backend/src` (backend), `npm run lint` (frontend)
   - Run tests: `make test`
   - Check coverage: `pytest --cov`

2. **Create PR**
   - Clear title: `feat:`, `fix:`, `docs:`, `refactor:`
   - Description with context
   - Link related issues

3. **Address Feedback**
   - Make requested changes
   - Push updates to same branch
   - Re-request review

---

## Running Services Locally

### Start All Services (Docker)

```bash
# Start everything
make up

# View logs
make logs

# Stop everything
make down
```

### Run Backend API Locally (Hot Reload)

```bash
# 1. Start infrastructure only (Qdrant, Redis, Ollama, etc.)
docker compose -f deploy/docker-compose.yml up qdrant redis minio ollama tantivy

# 2. Run backend locally
cd backend
source venv/bin/activate
uvicorn src.main:app --host 0.0.0.0 --port 8000 --reload

# Changes auto-reload!
```

### Run Worker Locally

```bash
# 1. Ensure infrastructure is running
docker compose -f deploy/docker-compose.yml up qdrant redis ollama tantivy

# 2. Run worker
cd backend
source venv/bin/activate
python src/worker/start_worker.py

# Or with Celery CLI:
celery -A src.tasks worker --loglevel=info
```

### Run Frontend Locally (Hot Reload)

```bash
# 1. Ensure backend is running
curl http://localhost:8000/health

# 2. Run frontend dev server
cd frontend
npm run dev

# Frontend: http://localhost:3000
# Changes auto-reload!
```

### Run Individual Services

```bash
# Qdrant only
docker compose -f deploy/docker-compose.yml up qdrant

# Redis only
docker compose -f deploy/docker-compose.yml up redis

# Ollama only
docker compose -f deploy/docker-compose.yml up ollama
```

---

## Code Style & Standards

### Python (Backend)

**Style Guide:** PEP 8 + Black formatter

```bash
# Install dev dependencies
pip install -e .[dev]

# Format code
black backend/src backend/tests

# Check formatting
black --check backend/src

# Lint
flake8 backend/src
```

**Standards:**
- Type hints required for public functions
- Docstrings for classes and public methods (Google style)
- Max line length: 100 characters
- Use async/await for I/O operations

**Example:**
```python
async def search(
    query: str,
    limit: int = 10,
    org_id: str = "public"
) -> List[Dict]:
    """
    Search indexed documents.

    Args:
        query: Search query text
        limit: Maximum results to return
        org_id: Organization identifier

    Returns:
        List of search results with scores and metadata
    """
    # Implementation...
```

### TypeScript (Frontend)

**Style Guide:** ESLint + Prettier

```bash
# Lint
npm run lint

# Fix auto-fixable issues
npm run lint -- --fix

# Format
npm run format
```

**Standards:**
- Strict TypeScript (`strict: true`)
- React functional components with hooks
- Props destructuring
- Named exports preferred

**Example:**
```typescript
interface SearchResultProps {
  result: SearchResult;
  onClick: (id: string) => void;
}

export function SearchResultCard({ result, onClick }: SearchResultProps) {
  // Implementation...
}
```

### Rust (Tantivy Service)

**Style Guide:** rustfmt + clippy

```bash
cd rust-tantivy

# Format
cargo fmt

# Lint
cargo clippy

# Check
cargo check
```

---

## Testing

### Backend Testing

**Run All Tests:**
```bash
cd backend
pytest
```

**Run Specific Tests:**
```bash
# Single file
pytest tests/test_search_services.py

# Single test
pytest tests/test_search_services.py::test_hybrid_search

# By keyword
pytest -k "search"

# With coverage
pytest --cov=src --cov-report=html
```

**Test Categories:**
```bash
# Unit tests only
pytest tests/unit/

# Integration tests (requires services)
pytest tests/integration/

# E2E tests (requires all services)
pytest tests/e2e/
```

### Frontend Testing

```bash
cd frontend

# Run tests
npm test

# Watch mode
npm test -- --watch

# Coverage
npm test -- --coverage
```

### Test-Driven Development (TDD)

1. **Write failing test**
   ```python
   def test_new_feature():
       result = my_function()
       assert result == expected
   ```

2. **Run test (should fail)**
   ```bash
   pytest tests/test_new_feature.py
   ```

3. **Implement feature**
   ```python
   def my_function():
       # Implementation
       pass
   ```

4. **Run test (should pass)**
   ```bash
   pytest tests/test_new_feature.py
   ```

---

## Debugging

### Backend Debugging

**1. VS Code (recommended)**

Create `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "FastAPI",
      "type": "python",
      "request": "launch",
      "module": "uvicorn",
      "args": [
        "src.main:app",
        "--reload",
        "--host", "0.0.0.0",
        "--port", "8000"
      ],
      "cwd": "${workspaceFolder}/backend",
      "env": {
        "QDRANT_URL": "http://localhost:6333",
        "REDIS_URL": "redis://localhost:6379/0"
      }
    }
  ]
}
```

**2. pdb (Python debugger)**

```python
# Add breakpoint
import pdb; pdb.set_trace()

# Or use breakpoint() (Python 3.7+)
breakpoint()
```

**3. Logging**

```python
import logging
logger = logging.getLogger(__name__)

logger.debug("Debug info")
logger.info("Important info")
logger.error("Error occurred")
```

View logs:
```bash
# Docker logs
docker compose -f deploy/docker-compose.yml logs -f backend-api

# Local logs (stdout)
```

### Frontend Debugging

**1. Browser DevTools**
- Chrome DevTools (F12)
- React DevTools extension
- Network tab for API calls

**2. VS Code Debugging**

Create `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Next.js",
      "type": "node",
      "request": "launch",
      "runtimeExecutable": "npm",
      "runtimeArgs": ["run", "dev"],
      "cwd": "${workspaceFolder}/frontend"
    }
  ]
}
```

**3. Console Logging**

```typescript
console.log('Debug:', data);
console.error('Error:', error);
```

### Service Debugging

```bash
# Check service health
docker compose -f deploy/docker-compose.yml ps

# View logs
docker compose -f deploy/docker-compose.yml logs qdrant
docker compose -f deploy/docker-compose.yml logs redis
docker compose -f deploy/docker-compose.yml logs ollama

# Execute commands in container
docker compose -f deploy/docker-compose.yml exec backend-api bash
docker compose -f deploy/docker-compose.yml exec redis redis-cli

# Check network
docker network inspect deploy_rice-net
```

---

## Contributing

### Contribution Guidelines

1. **Fork the repository**
2. **Create feature branch** from `main`
3. **Follow code style** (Black, ESLint)
4. **Write tests** for new features
5. **Update documentation** if needed
6. **Create pull request** with clear description

### Commit Message Format

```
<type>: <subject>

<body>

<footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tooling changes

**Example:**
```
feat: Add support for BM42 hybrid vectors

Implements BM42 encoder for Qdrant-native hybrid search.
Combines sparse and dense representations for improved recall.

Closes #123
```

### Pull Request Checklist

- [ ] Tests pass (`make test`)
- [ ] Linting passes (`flake8`, `npm run lint`)
- [ ] Documentation updated
- [ ] Commit messages follow format
- [ ] No merge conflicts
- [ ] PR description is clear
- [ ] Related issues linked

---

## Common Development Tasks

### Add New API Endpoint

**1. Create endpoint file:**
```python
# backend/src/api/v1/endpoints/my_feature.py
from fastapi import APIRouter

router = APIRouter()

@router.get("/hello")
async def hello():
    return {"message": "Hello World"}
```

**2. Register router:**
```python
# backend/src/main.py
from src.api.v1.endpoints import my_feature

app.include_router(
    my_feature.router,
    prefix="/api/v1/my-feature",
    tags=["my-feature"]
)
```

**3. Test endpoint:**
```bash
curl http://localhost:8000/api/v1/my-feature/hello
```

### Add New Service

**1. Create service file:**
```python
# backend/src/services/my_service.py
class MyService:
    def __init__(self):
        pass

    async def process(self, data: str) -> str:
        # Implementation
        return data.upper()
```

**2. Use in endpoint:**
```python
from src.services.my_service import MyService

@router.post("/process")
async def process_data(data: str):
    service = MyService()
    result = await service.process(data)
    return {"result": result}
```

### Add New Configuration Setting

**1. Update settings.yaml:**
```yaml
my_feature:
  enabled: true
  timeout: 60
```

**2. Add mapping in config.py:**
```python
# backend/src/core/config.py
mappings = {
    # ...
    "MY_FEATURE_ENABLED": "my_feature.enabled",
    "MY_FEATURE_TIMEOUT": "my_feature.timeout",
}
```

**3. Use setting:**
```python
from src.core.config import settings

if settings.MY_FEATURE_ENABLED:
    timeout = settings.MY_FEATURE_TIMEOUT
```

### Add New Test

**1. Create test file:**
```python
# backend/tests/test_my_feature.py
import pytest
from src.services.my_service import MyService

@pytest.mark.asyncio
async def test_my_service():
    service = MyService()
    result = await service.process("hello")
    assert result == "HELLO"
```

**2. Run test:**
```bash
pytest tests/test_my_feature.py -v
```

### Rebuild Docker Image

```bash
# Rebuild specific service
docker compose -f deploy/docker-compose.yml build backend-api

# Rebuild and restart
docker compose -f deploy/docker-compose.yml up --build backend-api

# Rebuild all
docker compose -f deploy/docker-compose.yml build
```

### Clear All Data

```bash
# Delete Qdrant collection
curl -X DELETE http://localhost:6333/collections/rice_chunks

# Clear Tantivy index
docker compose -f deploy/docker-compose.yml exec tantivy rm -rf /data/*

# Clear Redis
docker compose -f deploy/docker-compose.yml exec redis redis-cli FLUSHDB

# Or delete volumes
docker compose -f deploy/docker-compose.yml down -v
```

---

## Summary

**Quick Start Development:**
```bash
# Setup
git clone <repo>
cd rice-search
cd backend && pip install -e .[dev]
cd ../frontend && npm install

# Start services
make up

# Run backend locally (hot reload)
cd backend
uvicorn src.main:app --reload

# Run frontend locally (hot reload)
cd frontend
npm run dev

# Run tests
cd backend
pytest
```

**Development Commands:**
```bash
make up          # Start all services
make down        # Stop all services
make logs        # View all logs
make test        # Run backend tests
# Lint manually: flake8 (backend), npm run lint (frontend)
make api-logs    # View API logs
make worker-logs # View worker logs
```

For more details:
- [Testing Guide](testing.md) - Comprehensive testing documentation
- [Architecture](architecture.md) - System design
- [API Reference](api.md) - API endpoints
- [Configuration](configuration.md) - Settings management

---

**[Back to Documentation Index](README.md)**
