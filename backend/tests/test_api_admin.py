"""
Integration tests for Admin API endpoints.
"""
import pytest
from fastapi.testclient import TestClient


@pytest.mark.integration
class TestAdminAPI:
    """Test Admin API endpoints."""
    
    def test_get_config(self, api_client):
        """Test getting configuration."""
        response = api_client.get("/api/v1/admin/public/config")
        assert response.status_code == 200
        data = response.json()
        assert "sparse_enabled" in data
        assert "worker_pool" in data
    
    def test_get_models(self, api_client):
        """Test getting all models."""
        response = api_client.get("/api/v1/admin/public/models")
        assert response.status_code == 200
        data = response.json()
        assert "models" in data
        assert len(data["models"]) >= 3  # dense, sparse, reranker
    
    def test_get_metrics(self, api_client):
        """Test metrics endpoint."""
        response = api_client.get("/api/v1/admin/public/metrics")
        assert response.status_code == 200
        data = response.json()
        assert "cpu_usage_percent" in data
        assert "memory_usage_mb" in data
