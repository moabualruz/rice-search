"""
Integration tests for Stores API.
"""
import pytest
import uuid
from unittest.mock import patch

@pytest.mark.integration
class TestStoresAPI:
    """Test store management endpoints."""
    
    def test_create_and_list_store(self, api_client):
        """Test creating and listing stores."""
        store_id = f"test-store-{uuid.uuid4()}"
        store_data = {
            "id": store_id,
            "name": "Test Store",
            "type": "staging"
        }
        
        # Create
        response = api_client.post("/api/v1/stores", json=store_data)
        assert response.status_code == 200, response.text
        data = response.json()
        assert data["id"] == store_id
        
        # List
        response = api_client.get("/api/v1/stores")
        assert response.status_code == 200
        data = response.json()
        assert isinstance(data, list)
        
        ids = [s["id"] for s in data]
        assert store_id in ids
    
    def test_get_store_details(self, api_client):
        """Test getting store details with Qdrant count."""
        store_id = f"detail-store-{uuid.uuid4()}"
        api_client.post("/api/v1/stores", json={"id": store_id, "name": "Detail"})
        
        with patch("src.api.v1.endpoints.stores.get_qdrant_client") as mock_q:
            mock_q.return_value.count.return_value.count = 42
            
            response = api_client.get(f"/api/v1/stores/{store_id}")
            assert response.status_code == 200
            data = response.json()
            assert data["doc_count"] == 42

    def test_delete_store(self, api_client):
        store_id = f"del-store-{uuid.uuid4()}"
        api_client.post("/api/v1/stores", json={"id": store_id, "name": "Delete Me"})
        
        response = api_client.delete(f"/api/v1/stores/{store_id}")
        assert response.status_code == 200
        assert response.json()["status"] == "success"
        
        # Verify gone
        response = api_client.get(f"/api/v1/stores/{store_id}")
        assert response.status_code == 404
