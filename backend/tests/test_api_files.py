"""
Integration tests for Files API.
"""
import pytest
from fastapi.testclient import TestClient

@pytest.mark.integration
class TestFilesAPI:
    """Test file management endpoints."""
    
    def test_list_files_empty(self, api_client):
        """Test listing files when empty."""
        response = api_client.get("/api/v1/files/list")
        assert response.status_code == 200, f"Got {response.status_code}: {response.text}"
        data = response.json()
        assert "files" in data
        assert isinstance(data["files"], list)
        
    def test_upload_file(self, api_client):
        """Test file upload."""
        # Override dependency for this test
        from src.main import app
        from src.api.v1.dependencies import verify_admin
        
        # Override to mock authenticated admin user
        app.dependency_overrides[verify_admin] = lambda: {"org_id": "public", "realm_access": {"roles": ["admin"]}}
        
        try:
            files = {
                'file': ('test.py', 'print("hello")', 'text/x-python')
            }
            response = api_client.post("/api/v1/ingest/file", files=files)
            assert response.status_code == 202, f"Got {response.status_code}: {response.text}"
            data = response.json()
            assert data["status"] == "queued"
        finally:
            del app.dependency_overrides[verify_admin]
        
    def test_delete_file_method_not_allowed(self, api_client):
        """Test file deletion (files endpoint doesn't usually have delete, check spec)."""
        response = api_client.delete("/api/v1/files/list") # Method not allowed
        assert response.status_code == 405
