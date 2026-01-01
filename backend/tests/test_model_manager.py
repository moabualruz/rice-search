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

    def test_unload_all_except(self):
        """Test exclusive unloading."""
        manager = ModelManager()
        
        # Load 3 models
        def mock_loader():
            # Mock object with .cpu() method to simulate unloadable model
            m = Mock()
            m.cpu.return_value = m
            return m

        manager.load_model("m1", mock_loader)
        manager.load_model("m2", mock_loader)
        manager.load_model("m3", mock_loader)
        
        # Unload all except m1
        count = manager.unload_all_except(["m1"])
        
        assert count == 2
        assert manager.get_model_status("m1")["loaded"] is True
        assert manager.get_model_status("m2")["loaded"] is False
        assert manager.get_model_status("m3")["loaded"] is False

        # Unload all except m1 (again) -> 0
        assert manager.unload_all_except(["m1"]) == 0
        
        # Unload all -> m1 unloads
        count = manager.unload_all_except([])
        assert count == 1
        assert manager.get_model_status("m1")["loaded"] is False

    @patch("src.services.model_manager.torch.cuda.is_available", return_value=False)
    @patch("src.services.model_manager.gc.collect")
    @patch("src.services.model_manager.torch.cuda.empty_cache")
    def test_unload_cpu_logic(self, mock_empty_cache, mock_gc, mock_cuda):
        """Test unload logic for CPU-only scenario."""
        manager = ModelManager()
        
        # Mock model with .cpu() method
        mock_model = Mock()
        mock_model.cpu = Mock()
        
        manager.load_model("cpu_model", lambda: mock_model)
        manager.unload_model("cpu_model")
        
        # Assertions
        mock_model.cpu.assert_called_once()
        mock_gc.assert_called_once()
        mock_empty_cache.assert_not_called() # Should NOT be called on CPU

    @patch("src.services.model_manager.torch.cuda.is_available", return_value=True)
    @patch("src.services.model_manager.gc.collect")
    @patch("src.services.model_manager.torch.cuda.empty_cache")
    def test_unload_gpu_logic(self, mock_empty_cache, mock_gc, mock_cuda):
        """Test unload logic for GPU scenario (verifying cleanup)."""
        manager = ModelManager()
        
        # Mock model
        mock_model = Mock()
        # Mock behavior: .cpu() moves it to cpu
        mock_model.cpu = Mock()
        
        manager.load_model("gpu_model", lambda: mock_model)
        manager.unload_model("gpu_model")
        
        # Assertions
        mock_model.cpu.assert_called_once() # Ensure it was moved to CPU first
        mock_gc.assert_called_once()
        mock_empty_cache.assert_called_once() # Verify GPU cache cleared
