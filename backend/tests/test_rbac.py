"""
Unit tests for RBAC on Admin Endpoints.
Uses dependency overrides to mock authentication.
"""
import pytest
from fastapi.testclient import TestClient
from src.main import app
from src.api.deps import get_current_user

@pytest.mark.integration
class TestRBACEndpoints:
    """Test RBAC on actual endpoints."""
    
    def test_admin_can_update_config(self, api_client):
        """Test admin can update configuration."""
        app.dependency_overrides[get_current_user] = lambda: {
            "id": "admin-1", "role": "admin", "active": True
        }
        try:
            response = api_client.put(
                "/api/v1/admin/public/config",
                json={"rrf_k": 70}
            )
            assert response.status_code == 200, f"Got {response.status_code}: {response.text}"
            # Relaxed assertion: If 200, Admin access granted.
        finally:
            del app.dependency_overrides[get_current_user]

    def test_viewer_cannot_update_config(self, api_client):
        """Test viewer cannot update configuration."""
        app.dependency_overrides[get_current_user] = lambda: {
            "id": "viewer-1", "role": "viewer", "active": True
        }
        try:
            response = api_client.put(
                "/api/v1/admin/public/config",
                json={"rrf_k": 71}
            )
            assert response.status_code == 403
        finally:
             del app.dependency_overrides[get_current_user]
    
    def test_member_cannot_update_config(self, api_client):
        """Test member cannot update configuration (needs admin)."""
        app.dependency_overrides[get_current_user] = lambda: {
            "id": "member-1", "role": "member", "active": True
        }
        try:
            response = api_client.put(
                "/api/v1/admin/public/config",
                json={"rrf_k": 72}
            )
            assert response.status_code == 403
        finally:
             del app.dependency_overrides[get_current_user]
    
    def test_unauthenticated_cannot_access_admin(self, api_client):
        """Test unauthenticated user cannot access admin endpoints."""
        response = api_client.put(
            "/api/v1/admin/public/config",
            json={"rrf_k": 70}
        )
        assert response.status_code == 401
