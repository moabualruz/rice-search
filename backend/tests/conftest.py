"""
Pytest configuration and shared fixtures.
"""
import pytest
import redis
from typing import Generator
from fastapi.testclient import TestClient

from src.main import app
from src.core.config import settings
from src.services.admin.admin_store import AdminStore


@pytest.fixture(scope="session")
def test_redis_client():
    """Redis client for testing (separate DB)."""
    client = redis.from_url(f"{settings.REDIS_URL}/15")  # Use DB 15 for tests
    yield client
    # Cleanup
    client.flushdb()
    client.close()


@pytest.fixture(scope="function")
def clean_redis(test_redis_client):
    """Clean Redis DB before each test."""
    test_redis_client.flushdb()
    yield test_redis_client


@pytest.fixture(scope="session")
def api_client():
    """FastAPI test client."""
    with TestClient(app) as client:
        yield client


@pytest.fixture
def admin_store(clean_redis):
    """Admin store with clean Redis."""
    store = AdminStore(redis_client=clean_redis)
    return store


@pytest.fixture
def sample_models():
    """Sample model configuration for testing."""
    return {
        "dense": {
            "id": "dense",
            "name": "sentence-transformers/all-MiniLM-L6-v2",
            "type": "embedding",
            "active": True,
            "gpu_enabled": False  # CPU for tests
        },
        "sparse": {
            "id": "sparse",
            "name": "naver/splade-v3",
            "type": "sparse_embedding",
            "active": False,
            "gpu_enabled": False
        }
    }


@pytest.fixture
def sample_users():
    """Sample users for RBAC testing."""
    return {
        "admin-user": {
            "id": "admin-user",
            "email": "admin@test.com",
            "role": "admin"
        },
        "member-user": {
            "id": "member-user",
            "email": "member@test.com",
            "role": "member"
        },
        "viewer-user": {
            "id": "viewer-user",
            "email": "viewer@test.com",
            "role": "viewer"
        }
    }


@pytest.fixture
def sample_documents():
    """Sample documents for search testing."""
    return [
        {
            "text": "def hello_world():\n    print('Hello, World!')",
            "metadata": {
                "file_path": "src/hello.py",
                "start_line": 1,
                "end_line": 2,
                "language": "python",
                "org_id": "public"
            }
        },
        {
            "text": "class Calculator:\n    def add(self, a, b):\n        return a + b",
            "metadata": {
                "file_path": "src/calc.py",
                "start_line": 1,
                "end_line": 3,
                "language": "python",
                "org_id": "public"
            }
        }
    ]


@pytest.fixture
def mock_model():
    """Mock ML model for testing without actual model loading."""
    class MockModel:
        def encode(self, text):
            # Return dummy embedding
            import numpy as np
            return np.random.rand(384)
        
        def to(self, device):
            return self
        
        @property
        def device(self):
            return "cpu"
    
    return MockModel()
