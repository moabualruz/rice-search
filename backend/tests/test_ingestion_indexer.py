"""
Unit tests for Indexer service.
"""
import pytest
from unittest.mock import MagicMock, patch
from src.services.ingestion.indexer import Indexer

def test_indexer_sanity():
    mock_qdrant = MagicMock()
    mock_model = MagicMock()
    
    indexer = Indexer(mock_qdrant, mock_model)
    
    # Mock parser
    with patch("src.services.ingestion.indexer.DocumentParser") as mock_parser:
        mock_parser.parse_file.return_value = "hello world"
        
        # Mock embeddings
        import numpy as np
        mock_model.encode.return_value = [np.random.rand(384)]
        
        res = indexer.ingest_file("dummy.txt", "repo", "public")
        
        assert res["status"] == "success"
        mock_qdrant.upsert.assert_called_once()
