# Testing Guide

Comprehensive testing documentation for Rice Search.

## Table of Contents

- [Testing Philosophy](#testing-philosophy)
- [Test Structure](#test-structure)
- [Running Tests](#running-tests)
- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [End-to-End Tests](#end-to-end-tests)
- [Load Testing](#load-testing)
- [Test Coverage](#test-coverage)
- [Writing Tests](#writing-tests)
- [Continuous Integration](#continuous-integration)

---

## Testing Philosophy

Rice Search follows **Test-Driven Development (TDD)** principles:

1. **Write tests first** - Before implementing features
2. **Red-Green-Refactor** - Failing test → passing test → improve code
3. **Test behavior, not implementation** - Focus on what, not how
4. **Maintain high coverage** - Target: >80% code coverage
5. **Fast feedback** - Tests should run quickly

---

## Test Structure

```text
backend/tests/
├── e2e/                    # End-to-end tests (full system)
│   └── test_ui_workflow.py
│
├── load/                   # Load & performance tests
│   └── locustfile.py
│
├── test_*.py               # Unit and Integration tests (flat)
│   ├── test_chunker.py
│   ├── test_search_services.py
│   └── ...
│
├── conftest.py             # Pytest fixtures
└── pytest.ini              # Pytest configuration
```

**Frontend Tests:**

```text
frontend/__tests__/
├── components/             # Component tests (Jest + React Testing Library)
├── pages/                  # Page tests
└── integration/            # Frontend integration tests
```

---

## Running Tests

### Backend Tests

**All Tests:**

```bash
cd backend
pytest
```

**Unit Tests Only:**

```bash
pytest -m unit -v
```

**Integration Tests:**

```bash
# Requires services running
docker compose -f deploy/docker-compose.yml up -d

pytest -m integration -v
```

**End-to-End Tests:**

```bash
# Requires full system running
make up

pytest tests/e2e/ -v
```

**Specific Test File:**

```bash
pytest tests/test_search_services.py -v
```

**Specific Test:**

```bash
pytest tests/test_search_services.py::test_hybrid_search -v
```

**By Keyword:**

```bash
pytest -k "search" -v
```

**With Coverage:**
```bash
pytest --cov=src --cov-report=html

# Open coverage report
open htmlcov/index.html
```

### Frontend Tests

```bash
cd frontend

# Run all tests
npm test

# Watch mode
npm test -- --watch

# Coverage
npm test -- --coverage

# Specific test
npm test -- SearchBar.test.tsx
```

---

## Unit Tests

**Purpose:** Test individual functions/classes in isolation.

**Characteristics:**

- ✅ Fast (<1ms per test)
- ✅ No external dependencies (mock everything)
- ✅ Deterministic (same input = same output)
- ✅ High code coverage

### Example: Chunker Test

```python
# tests/unit/test_chunker.py
import pytest
from src.services.ingestion.chunker import DocumentChunker

def test_chunk_text_basic():
    """Test basic text chunking."""
    chunker = DocumentChunker(chunk_size=100, chunk_overlap=20)
    text = "A" * 250  # 250 characters
    metadata = {"file_path": "test.txt"}

    chunks = chunker.chunk_text(text, metadata)

    # Should create 3 chunks: [0:100], [80:180], [160:250]
    assert len(chunks) == 3
    assert len(chunks[0]["content"]) == 100
    assert chunks[0]["metadata"]["file_path"] == "test.txt"
    assert chunks[0]["chunk_index"] == 0

def test_chunk_text_with_overlap():
    """Test chunk overlap."""
    chunker = DocumentChunker(chunk_size=100, chunk_overlap=20)
    text = "0123456789" * 25  # 250 characters

    chunks = chunker.chunk_text(text, {})

    # Check overlap: end of chunk[0] should overlap with start of chunk[1]
    assert chunks[0]["content"][80:] == chunks[1]["content"][:20]
```

### Example: Config Test

```python
# tests/unit/test_config.py
import os
import pytest
from src.core.config import settings

def test_settings_default_values():
    """Test default configuration values."""
    assert settings.EMBEDDING_DIM == 2560
    assert settings.RRF_K == 60
    assert settings.CHUNK_SIZE == 1000

def test_settings_env_override(monkeypatch):
    """Test environment variable override."""
    monkeypatch.setenv("EMBEDDING_DIM", "1024")

    # Reload settings
    from src.core.config import Settings
    test_settings = Settings()

    assert test_settings.EMBEDDING_DIM == 1024
```

---

## Integration Tests

**Purpose:** Test multiple components working together.

**Characteristics:**
- Requires running services (Qdrant, Redis, Ollama)
- Tests actual API calls and database operations
- Slower than unit tests (~100ms-1s per test)
- Tests real-world scenarios

### Example: Search Integration Test

```python
# tests/integration/test_search_services.py
import pytest
from src.services.search.retriever import Retriever
from src.services.ingestion.indexer import Indexer
from src.db.qdrant import get_qdrant_client

@pytest.mark.asyncio
async def test_end_to_end_indexing_and_search():
    """Test complete indexing and search pipeline."""
    # Setup
    qdrant = get_qdrant_client()
    indexer = Indexer(qdrant)
    indexer.ensure_collection()

    # Index a test file
    test_file_path = "tests/fixtures/test_code.py"
    result = indexer.ingest_file(
        file_path=test_file_path,
        display_path=test_file_path,
        repo_name="test",
        org_id="test"
    )

    assert result["status"] == "success"
    assert result["chunks_indexed"] > 0

    # Search for content
    results = await Retriever.search(
        query="test function",
        limit=5,
        org_id="test"
    )

    assert len(results) > 0
    assert results[0]["score"] > 0.5
    assert "test_code.py" in results[0]["full_path"]
```

### Example: Qdrant Integration Test

```python
# tests/integration/test_qdrant_client.py
import pytest
from src.db.qdrant import get_qdrant_client
from qdrant_client.models import VectorParams, Distance

@pytest.mark.integration
def test_qdrant_connection():
    """Test Qdrant client connection."""
    client = get_qdrant_client()

    # Should connect successfully
    collections = client.get_collections()
    assert collections is not None

@pytest.mark.integration
def test_create_collection():
    """Test collection creation."""
    client = get_qdrant_client()

    collection_name = "test_collection"

    # Create collection
    client.create_collection(
        collection_name=collection_name,
        vectors_config={
            "dense": VectorParams(size=2560, distance=Distance.COSINE)
        }
    )

    # Verify exists
    collections = client.get_collections().collections
    assert any(c.name == collection_name for c in collections)

    # Cleanup
    client.delete_collection(collection_name)
```

---

## End-to-End Tests

**Purpose:** Test complete user workflows through the UI.

**Characteristics:**
- Tests full system (frontend + backend + services)
- Uses browser automation (Playwright)
- Slowest tests (~5-30s per test)
- Validates real user scenarios

### Example: E2E Test with Playwright

```python
# tests/e2e/test_ui_workflow.py
import pytest
from playwright.sync_api import Page, expect

@pytest.mark.e2e
def test_search_workflow(page: Page):
    """Test complete search workflow through UI."""
    # Navigate to app
    page.goto("http://localhost:3000")

    # Wait for page load
    expect(page.locator("h1")).to_contain_text("Rice Search")

    # Enter search query
    search_input = page.locator('input[placeholder*="Search"]')
    search_input.fill("authentication")

    # Click search button
    search_button = page.locator('button:has-text("Search")')
    search_button.click()

    # Wait for results
    page.wait_for_selector('.search-result')

    # Verify results appear
    results = page.locator('.search-result')
    expect(results).to_have_count(greather_than=0)

    # Check first result has required fields
    first_result = results.first
    expect(first_result.locator('.file-path')).to_be_visible()
    expect(first_result.locator('.score')).to_be_visible()
    expect(first_result.locator('.snippet')).to_be_visible()

@pytest.mark.e2e
def test_file_upload_workflow(page: Page):
    """Test file upload and indexing through UI."""
    page.goto("http://localhost:3000/admin")

    # Upload file
    file_input = page.locator('input[type="file"]')
    file_input.set_input_files("tests/fixtures/test_code.py")

    # Submit
    upload_button = page.locator('button:has-text("Upload")')
    upload_button.click()

    # Wait for success message
    expect(page.locator('.success-message')).to_be_visible()

    # Verify file is searchable
    page.goto("http://localhost:3000")
    search_input = page.locator('input[placeholder*="Search"]')
    search_input.fill("test_code.py")
    page.locator('button:has-text("Search")').click()

    # Should find the uploaded file
    expect(page.locator('.search-result:has-text("test_code.py")')).to_be_visible()
```

### Running E2E Tests

```bash
# Install Playwright
cd backend
pip install playwright
playwright install

# Run E2E tests
pytest tests/e2e/ -v --headed  # Watch browser
pytest tests/e2e/ -v           # Headless mode
```

---

## Load Testing

**Purpose:** Test system performance under load.

**Tool:** Locust (Python-based load testing)

### Example: Locust Test

```python
# tests/load/locustfile.py
from locust import HttpUser, task, between

class SearchUser(HttpUser):
    wait_time = between(1, 3)  # Wait 1-3 seconds between tasks
    host = "http://localhost:8000"

    @task(3)  # Weight: 3 (runs 3x more often)
    def search_query(self):
        """Simulate search requests."""
        self.client.post(
            "/api/v1/search/query",
            json={
                "query": "authentication",
                "limit": 10
            }
        )

    @task(1)  # Weight: 1
    def list_files(self):
        """Simulate file listing."""
        self.client.get("/api/v1/files/list")

    @task(1)
    def health_check(self):
        """Simulate health checks."""
        self.client.get("/health")
```

### Running Load Tests

```bash
# Install Locust
pip install locust

# Start load test
cd backend/tests/load
locust -f locustfile.py

# Open web UI: http://localhost:8089
# Configure:
#   - Number of users: 100
#   - Spawn rate: 10/sec
#   - Host: http://localhost:8000
```

**Metrics to Monitor:**
- Requests per second (RPS)
- Average response time
- P95/P99 latency
- Error rate

---

## Test Coverage

### Check Coverage

```bash
cd backend

# Run with coverage
pytest --cov=src --cov-report=html --cov-report=term

# View HTML report
open htmlcov/index.html
```

### Coverage Targets

| Component | Target |
| :--- | :--- |
| Core services | >90% |
| API endpoints | >80% |
| Utilities | >85% |
| Overall | >80% |

### Example Coverage Report

```text
Name                                    Stmts   Miss  Cover
-----------------------------------------------------------
src/services/search/retriever.py          150     10    93%
src/services/ingestion/indexer.py         200     25    88%
src/api/v1/endpoints/search.py             50      5    90%
src/core/config.py                         80      8    90%
-----------------------------------------------------------
TOTAL                                     1500    150    90%
```

---

## Writing Tests

### Best Practices

1. **Follow AAA Pattern**
   ```python
   def test_example():
       # Arrange: Setup test data
       data = {"key": "value"}

       # Act: Execute function under test
       result = process_data(data)

       # Assert: Verify outcome
       assert result == expected
   ```

2. **Use Descriptive Names**
   ```python
   # ❌ Bad
   def test_1():
       pass

   # ✅ Good
   def test_search_returns_results_for_valid_query():
       pass
   ```

3. **Test One Thing Per Test**
   ```python
   # ❌ Bad: Tests multiple things
   def test_search():
       assert search("test") is not None
       assert search("") raises ValueError
       assert search(None) raises TypeError

   # ✅ Good: Separate tests
   def test_search_returns_results():
       assert search("test") is not None

   def test_search_raises_on_empty_query():
       with pytest.raises(ValueError):
           search("")
   ```

4. **Use Fixtures for Common Setup**
   ```python
   # conftest.py
   @pytest.fixture
   def qdrant_client():
       client = get_qdrant_client()
       yield client
       # Cleanup after test
       client.close()

   # test_file.py
   def test_with_fixture(qdrant_client):
       # Use qdrant_client directly
       result = qdrant_client.get_collections()
       assert result is not None
   ```

5. **Mock External Dependencies**
   ```python
   from unittest.mock import patch


   def test_ollama_embedding(monkeypatch):
       # Mock Ollama API call
       mock_response = [[0.1, 0.2, 0.3]]

       with patch('src.services.inference.ollama_client.generate_embedding') as mock:
           mock.return_value = mock_response

           result = embed_texts(["test"])
           assert result == mock_response
   ```

### Pytest Markers

```python
# Mark slow tests
@pytest.mark.slow
def test_slow_operation():
    pass

# Mark integration tests
@pytest.mark.integration
def test_database_connection():
    pass

# Mark E2E tests
@pytest.mark.e2e
def test_ui_workflow():
    pass

# Run only fast tests
pytest -m "not slow"

# Run only integration tests
pytest -m integration
```

---

## Continuous Integration

### GitHub Actions Example

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest

    services:
      qdrant:
        image: qdrant/qdrant:latest
        ports:
          - 6333:6333

      redis:
        image: redis:8-alpine
        ports:
          - 6379:6379

    steps:
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.12'

      - name: Install dependencies
        run: |
          cd backend
          pip install -e .[dev]

      - name: Run unit tests
        run: |
          cd backend
          pytest tests/unit/ -v

      - name: Run integration tests
        run: |
          cd backend
          pytest tests/integration/ -v

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./backend/coverage.xml
```

---

## Summary

**Test Pyramid:**

```text
        E2E (few, slow)
       /              \
      /  Integration   \
     /   (moderate)     \
    /                    \
   /    Unit (many,      \
  /       fast)           \
 --------------------------
```

**Quick Commands:**

```bash
# Unit tests
pytest -m unit

# Integration tests
pytest -m integration

# E2E tests
pytest tests/e2e/

# All tests with coverage
pytest --cov=src --cov-report=html

# Load tests
locust -f tests/load/locustfile.py
```

**Test Writing Checklist:**

- [ ] Write test before implementation (TDD)
- [ ] Test passes with implementation
- [ ] Test fails when implementation is broken
- [ ] Uses descriptive name
- [ ] Follows AAA pattern
- [ ] Mocks external dependencies
- [ ] No hardcoded values
- [ ] Runs quickly (<1s for unit tests)

For more details:
- [Development Guide](development.md) - Dev workflow
- [API Reference](api.md) - API endpoints to test
- [Architecture](architecture.md) - System components
- [Troubleshooting](troubleshooting.md) - Test issues

---

**[Back to Documentation Index](README.md)**
