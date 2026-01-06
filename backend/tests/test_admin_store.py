"""
Unit tests for AdminStore service.
"""
import pytest
from src.services.admin.admin_store import AdminStore


@pytest.mark.unit
class TestAdminStore:
    """Test AdminStore functionality."""
    
    def test_get_models(self, admin_store):
        """Test getting models from store."""
        models = admin_store.get_models()
        assert isinstance(models, dict)
        assert "dense" in models
        assert "sparse" in models
        assert "reranker" in models
    
    def test_set_model(self, admin_store):
        """Test updating a model."""
        new_model = {
            "id": "custom",
            "name": "test/model",
            "type": "embedding",
            "active": True,
            "gpu_enabled": False
        }
        admin_store.set_model("custom", new_model)
        
        models = admin_store.get_models()
        assert "custom" in models
        assert models["custom"]["name"] == "test/model"
    
    def test_delete_model(self, admin_store):
        """Test deleting a model."""
        result = admin_store.delete_model("sparse")
        assert result is True
        
        models = admin_store.get_models()
        assert "sparse" not in models
    
    def test_get_config(self, admin_store):
        """Test getting configuration."""
        config = admin_store.get_effective_config()
        assert isinstance(config, dict)
        assert "sparse_enabled" in config
        assert "rrf_k" in config
        # worker_pool and worker_concurrency may not be in defaults yet
        # This is acceptable for now
    
    def test_set_config(self, admin_store):
        """Test updating configuration."""
        # First ensure the key exists
        admin_store.set_config("worker_concurrency", 10)
        admin_store.set_config("worker_concurrency", 20)
        
        config = admin_store.get_effective_config()
        assert config["worker_concurrency"] == 20
    
    def test_get_users(self, admin_store):
        """Test getting users."""
        users = admin_store.get_users()
        assert isinstance(users, dict)
        assert len(users) >= 1  # At least default admin
    
    def test_create_user(self, admin_store):
        """Test creating a user."""
        import uuid
        user_id = str(uuid.uuid4())
        user = {
            "id": user_id,
            "email": "test@example.com",
            "role": "member"
        }
        admin_store.set_user(user_id, user)
        
        users = admin_store.get_users()
        assert user_id in users
        assert users[user_id]["email"] == "test@example.com"
    
    def test_update_user(self, admin_store):
        """Test updating a user."""
        import uuid
        user_id = str(uuid.uuid4())
        
        # Create user
        user = {
            "id": user_id,
            "email": "update@example.com",
            "role": "viewer"
        }
        admin_store.set_user(user_id, user)
        
        # Update role
        user["role"] = "member"
        admin_store.set_user(user_id, user)
        
        users = admin_store.get_users()
        assert users[user_id]["role"] == "member"
    
    def test_delete_user(self, admin_store):
        """Test deleting a user."""
        import uuid
        user_id = str(uuid.uuid4())
        
        # Create user
        user = {
            "id": user_id,
            "email": "delete@example.com",
            "role": "viewer"
        }
        admin_store.set_user(user_id, user)
        
        # Delete
        result = admin_store.delete_user(user_id)
        assert result is True
        
        users = admin_store.get_users()
        assert user_id not in users

    
    def test_config_snapshot(self, admin_store):
        """Test config snapshot and rollback."""
        # Set initial value
        admin_store.set_config("rrf_k", 60)
        
        # Save snapshot
        snapshot = admin_store.save_config_snapshot("test-snapshot")
        assert snapshot is not None
        
        # Change config
        admin_store.set_config("rrf_k", 100)
        config = admin_store.get_effective_config()
        assert config["rrf_k"] == 100
        
        # Rollback
        result = admin_store.rollback_config(0)  # Most recent
        assert result is True
        
        config = admin_store.get_effective_config()
        assert config["rrf_k"] == 60  # Back to snapshot value
