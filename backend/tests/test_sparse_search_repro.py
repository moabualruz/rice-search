import pytest
from unittest.mock import MagicMock, patch
import logging

# We will need to import these after implementation, so we use mocks or dynamic imports
# from src.services.search.sparse import SparseEmbedder

@pytest.fixture
def mock_model_manager():
    with patch("src.services.model_manager.get_model_manager") as mock:
        manager = MagicMock()
        mock.return_value = manager
        yield manager

@pytest.fixture
def SparseEmbedder():
    try:
        from src.services.search.sparse import SparseEmbedder
        return SparseEmbedder
    except ImportError:
        return None

def test_sparse_embedder_exists(SparseEmbedder):
    """Verify SparseEmbedder class exists."""
    assert SparseEmbedder is not None, "SparseEmbedder class is missing"

def test_sparse_embed(SparseEmbedder, mock_model_manager):
    """Verify embedding generation."""
    if not SparseEmbedder:
        pytest.fail("SparseEmbedder not implemented")
        
    embedder = SparseEmbedder()
    
    # Mock the transformer model
    mock_model = MagicMock()
    # Mock output of model (Logits)
    import torch
    
    # Create fake logits: batch_size=1, vocab_size=100
    fake_logits = torch.randn(1, 100)
    # Make a few values high to ensure non-zero sparse vector
    fake_logits[0, 10] = 5.0
    fake_logits[0, 20] = 5.0
    
    mock_model.return_value = MagicMock(logits=fake_logits)
    mock_tokenizer = MagicMock()
    mock_tokenizer.return_value = {"input_ids": torch.tensor([[1, 2]]), "attention_mask": torch.tensor([[1, 1]])}
    
    # We must patch where SparseEmbedder gets the model
    with patch("src.services.search.sparse.get_model_manager", return_value=mock_model_manager):
        # Mock loader execution
        mock_model_manager.load_model.side_effect = lambda id, loader: loader()
        # Mock getting the model instance from manager
        mock_model_manager.__getitem__ = MagicMock()
        
        # This is tricky because SparseEmbedder might define loader internally.
        # Let's assume we can mock the `_load_model` method or similar.
        
        with patch.object(SparseEmbedder, "_load_model") as mock_load:
            embedder.model = mock_model
            embedder.tokenizer = mock_tokenizer
            embedder.device = "cpu"
            embedder._loaded = True

            
            result = embedder.embed("test query")
            
            assert hasattr(result, "indices")
            assert hasattr(result, "values")
            assert len(result.indices) > 0
            assert len(result.values) == len(result.indices)
            assert 10 in result.indices or 20 in result.indices

