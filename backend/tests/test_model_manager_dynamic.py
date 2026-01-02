import pytest
from unittest.mock import MagicMock, patch
from src.services.model_manager import ModelManager

@pytest.fixture
def mock_admin_store():
    with patch("src.services.model_manager.get_admin_store") as mock:
        store = MagicMock()
        mock.return_value = store
        yield store

@pytest.fixture
def manager():
    # Reset singleton
    ModelManager._instance = None
    return ModelManager.get_instance()

def test_manager_initializes_from_admin_store(manager, mock_admin_store):
    """
    Test that the manager polls the admin store for the active configuration
    instead of just using env vars.
    """
    # Setup AdminStore to return a custom model
    mock_admin_store.get_models.return_value = {
        "dense": {
            "id": "dense",
            "name": "custom/model-v1",
            "active": True,
            "type": "embedding",
            "gpu_enabled": False
        }
    }
    
    # Trigger initialization (we might need a method for this if __init__ is too simple)
    # Assuming get_active_model_name("dense") method exists or similar
    
    # We expect the manager to have logic to "resolve" the active model
    model_name = manager.resolve_model_name("dense")
    
    assert model_name == "custom/model-v1"
    mock_admin_store.get_models.assert_called()

def test_download_triggers_hf_hub(manager):
    """
    Test that loading a model triggers the correct HF download calls,
    specifically checking for trust_remote_code.
    """
    with patch("transformers.AutoTokenizer.from_pretrained") as mock_tokenizer, \
         patch("transformers.AutoModel.from_pretrained") as mock_model, \
         patch("sentence_transformers.SentenceTransformer") as mock_st, \
         patch("torch.cuda.is_available", return_value=False):
        
        # Test loading a model that requires trust_remote_code (e.g. jina)
        model_id = "jinaai/jina-code-embeddings-1.5b"
        
        # Call the load method
        # We assume a method `load_model_from_hub(model_id, type, trust_remote_code=True)`
        # Note: "embedding" triggers SentenceTransformer path, NOT AutoModel
        manager.load_model_from_hub(model_id, "embedding", trust_remote_code=True)
        
        # Verify ST call
        mock_st.assert_called_with(model_id, trust_remote_code=True, device="cpu")

def test_runtime_swap(manager):
    """
    Test logic to swap models at runtime.
    """
    manager.unload_model = MagicMock(return_value=True)
    manager.load_model_from_hub = MagicMock(return_value=True)
    
    # Swap "dense" from ModelA to ModelB
    manager.swap_model("dense", "new/model-b")
    
    manager.unload_model.assert_called_with("dense")
    manager.load_model_from_hub.assert_called_with("new/model-b", "embedding", trust_remote_code=False)
