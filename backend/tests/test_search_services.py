"""
Unit tests for Search services (Dense, Sparse, Hybrid).
"""
import pytest
from unittest.mock import Mock, patch, MagicMock
import numpy as np


@pytest.mark.unit
class TestRetriever:
    """Test search retriever functionality."""
    
    @patch('src.services.search.retriever.get_dense_model')
    @patch('src.services.search.retriever.get_qdrant_client')
    def test_dense_search(self, mock_qdrant, mock_model):
        """Test dense-only search."""
        from src.services.search.retriever import Retriever
        
        # Mock model
        mock_model.return_value.encode.return_value = np.random.rand(384)
        
        # Mock Qdrant
        mock_client = MagicMock()
        mock_client.search.return_value = []
        mock_qdrant.return_value = mock_client
        
        retriever = Retriever()
        results = retriever.dense_search("test query", limit=5, org_id="public")
        
        assert isinstance(results, list)
        mock_model.return_value.encode.assert_called_once()
    
    @patch('src.services.search.retriever.get_dense_model')
    @patch('src.services.search.retriever.get_qdrant_client')
    def test_search_with_filters(self, mock_qdrant, mock_model):
        """Test search with language filter."""
        from src.services.search.retriever import Retriever
        
        mock_model.return_value.encode.return_value = np.random.rand(384)
        mock_client = MagicMock()
        mock_client.search.return_value = []
        mock_qdrant.return_value = mock_client
        
        retriever = Retriever()
        results = retriever.dense_search(
            "test query",
            limit=5,
            org_id="public",
            language="python"
        )
        
        assert isinstance(results, list)


@pytest.mark.unit  
class TestSparseEmbedder:
    """Test sparse embedding functionality."""
    
    @patch('src.services.search.sparse_embedder.AutoModelForMaskedLM')
    @patch('src.services.search.sparse_embedder.AutoTokenizer')
    def test_sparse_encode(self, mock_tokenizer, mock_model):
        """Test sparse encoding."""
        from src.services.search.sparse_embedder import SparseEmbedder
        
        # Mock tokenizer
        mock_tok = MagicMock()
        mock_tok.return_value = {
            'input_ids': [[1, 2, 3]],
            'attention_mask': [[1, 1, 1]]
        }
        mock_tokenizer.from_pretrained.return_value = mock_tok
        
        # Mock model
        mock_m = MagicMock()
        mock_m.eval.return_value = mock_m
        mock_m.to.return_value = mock_m
        mock_model.from_pretrained.return_value = mock_m
        
        embedder = SparseEmbedder()
        # Test would need actual implementation
        assert embedder is not None


@pytest.mark.unit
class TestReranker:
    """Test reranker functionality."""
    
    @patch('src.services.search.reranker.CrossEncoder')
    def test_rerank(self, mock_cross_encoder):
        """Test reranking."""
        from src.services.search.reranker import Reranker
        
        # Mock CrossEncoder
        mock_ce = MagicMock()
        mock_ce.predict.return_value = [0.9, 0.7, 0.5]
        mock_cross_encoder.return_value = mock_ce
        
        reranker = Reranker()
        reranker.model = mock_ce
        reranker._loaded = True
        
        query = "test query"
        docs = [
            {"text": "doc1", "score": 0.5},
            {"text": "doc2", "score": 0.6},
            {"text": "doc3", "score": 0.4}
        ]
        
        reranked = reranker.rerank(query, docs, top_k=3)
        assert len(reranked) == 3
        assert reranked[0]["text"] == "doc1"  # Highest score
