"""
Unit tests for SparseEmbedder.
"""
import pytest
from unittest.mock import Mock, patch
import torch
from src.services.search.sparse_embedder import SparseEmbedder

@pytest.mark.unit
class TestSparseEmbedder:
    
    @patch("src.services.search.sparse_embedder.get_model_manager")
    def test_embed_batch_optimization(self, mock_get_manager):
        """Test that embed_batch uses vectorized operations instead of loops."""
        # Setup
        manager = Mock()
        mock_get_manager.return_value = manager
        
        mock_model = Mock()
        mock_tokenizer = Mock()
        
        # Manager Setup
        manager.get_model_status.return_value = {"loaded": True}
        manager._models = {
            "sparse": {
                "instance": {
                    "model": mock_model,
                    "tokenizer": mock_tokenizer
                }
            }
        }
        
        embedder = SparseEmbedder()
        texts = ["hello", "world"]
        
        # Mock Tokenizer Return (Object that returns dict on .to())
        # We need inputs["attention_mask"] to be a tensor for masking operations
        inputs_dict = {
            "input_ids": torch.zeros(2, 5), # Batch=2, Seq=5
            "attention_mask": torch.ones(2, 5) # All 1s for simplicity
        }
        
        batch_encoding_obj = Mock()
        batch_encoding_obj.to.return_value = inputs_dict
        mock_tokenizer.return_value = batch_encoding_obj

        # Mock Model Output
        # Batch=2, Seq=5, Vocab=100
        # Logits: (Batch, Seq, Vocab)
        logits = torch.randn(2, 5, 100) 
        mock_output = Mock()
        mock_output.logits = logits
        mock_model.return_value = mock_output
        
        # Run
        # Current implementation should now work
        embedder.embed_batch(texts)
        
        # Verify Tokenizer called with LIST implies batching
        mock_tokenizer.assert_called_once_with(
            texts, 
            return_tensors="pt", 
            truncation=True, 
            padding=True, 
            max_length=512
        )
        
        # Verify Model called ONCE
        mock_model.assert_called_once()
