"""
Unit tests for ModelManager service.
"""
import pytest
from unittest.mock import Mock, patch
from src.services.model_manager import ModelManager, get_model_manager


@pytest.mark.unit
class TestModelManager:
    """Test ModelManager functionality."""
    
    def test_singleton_pattern(self):
        """Test that ModelManager follows singleton pattern."""
        manager1 = get_model_manager()
        manager2 = get_model_manager()
        assert manager1 is manager2
    
    def test_register_model(self):
        """Test model registration."""
        manager = ModelManager()
        manager.register_model("test-model", "embedding", instance=None)
        
        status = manager.get_model_status("test-model")
        assert status["id"] == "test-model"
        assert status["type"] == "embedding"
        assert status["loaded"] is False
    
    def test_load_model_success(self):
        """Test successful model loading."""
        manager = ModelManager()
        
        def mock_loader():
            return Mock(device="cpu")
        
        result = manager.load_model("test-model", mock_loader)
        assert result is True
        
        status = manager.get_model_status("test-model")
        assert status["loaded"] is True
    
    def test_load_model_failure(self):
        """Test model loading failure."""
        manager = ModelManager()
        
        def failing_loader():
            raise RuntimeError("Model not found")
        
        result = manager.load_model("test-model", failing_loader)
        assert result is False
    
    def test_unload_model(self):
        """Test model unloading."""
        manager = ModelManager()
        
        # Load model first
        def mock_loader():
            mock = Mock()
            mock.cpu = Mock(return_value=mock)
            return mock
        
        manager.load_model("test-model", mock_loader)
        
        # Unload it
        result = manager.unload_model("test-model")
        assert result is True
        
        status = manager.get_model_status("test-model")
        assert status["loaded"] is False
    
    def test_get_all_models(self):
        """Test getting all registered models."""
        manager = ModelManager()
        manager.register_model("model1", "embedding")
        manager.register_model("model2", "reranker")
        
        all_models = manager.get_all_models()
        assert len(all_models) == 2
        assert "model1" in all_models
        assert "model2" in all_models
    
    @patch("src.services.model_manager.torch.cuda.is_available", return_value=False)
    def test_get_gpu_usage_no_cuda(self, mock_cuda):
        """Test GPU usage when CUDA not available."""
        manager = ModelManager()
        usage = manager.get_gpu_usage()
        
        assert usage["available"] is False
        assert usage["used_mb"] == 0
