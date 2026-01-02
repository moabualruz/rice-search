
import pytest
import time
from unittest.mock import patch, MagicMock
from src.services.model_manager import ModelManager

@pytest.fixture
def manager():
    # consistent singleton reset
    ModelManager._instance = None
    mgr = ModelManager()
    yield mgr
    ModelManager._instance = None

def test_model_registration_initializes_access_time(manager):
    """Verify registration initializes last_accessed."""
    # Case 1: Loaded model
    manager.register_model("loaded-model", "embedding", instance="mock")
    assert manager._models["loaded-model"]["last_accessed"] > 0
    
    # Case 2: Unloaded model
    manager.register_model("unloaded-model", "embedding")
    assert manager._models["unloaded-model"]["last_accessed"] == 0

def test_get_model_updates_access_time(manager):
    """Verify accessing a model updates its timestamp."""
    manager.register_model("test-model", "embedding", instance="mock_instance")
    
    # Manually set old time
    old_time = time.time() - 1000
    manager._models["test-model"]["last_accessed"] = old_time
    
    # Access
    manager.get_model_instance("test-model")
    
    # Verify update
    new_time = manager._models["test-model"]["last_accessed"]
    assert new_time > old_time
    # Should be close to now
    assert abs(new_time - time.time()) < 1.0

def test_check_ttl_unloads_expired(manager):
    """Verify expired models are unloaded."""
    # Register and Mock load
    model_id = "expired-model"
    manager.register_model(model_id, "embedding", instance=MagicMock())
    
    # Set time to be EXPIRED (TTL = 300s default)
    # So set last_accessed to 301 seconds ago
    manager._models[model_id]["last_accessed"] = time.time() - 301
    
    # Run Check (Not implemented yet -> RED)
    # Assuming config TTL is 300
    unloaded_count = manager.check_ttl(ttl_seconds=300)
    
    assert unloaded_count == 1
    assert manager._models[model_id]["loaded"] is False

def test_check_ttl_keeps_active(manager):
    """Verify active models are kept."""
    model_id = "active-model"
    manager.register_model(model_id, "embedding", instance=MagicMock())
    
    # Set time to be ACTIVE (100s ago)
    manager._models[model_id]["last_accessed"] = time.time() - 100
    
    unloaded_count = manager.check_ttl(ttl_seconds=300)
    
    assert unloaded_count == 0
    assert manager._models[model_id]["loaded"] is True
