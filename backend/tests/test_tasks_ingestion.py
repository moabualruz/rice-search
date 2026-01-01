"""
Unit tests for Ingestion Tasks (Delegation).
"""
import pytest
from unittest.mock import patch, MagicMock
from src.worker import celery_app
from src.tasks.ingestion import ingest_file_task

def test_ingest_file_task_delegation():
    """Test that task creates Indexer and delegates."""
    celery_app.conf.task_always_eager = True
    
    with patch("src.tasks.ingestion.Indexer") as MockIndexer, \
         patch("src.tasks.ingestion.get_qdrant"), \
         patch("src.tasks.ingestion.get_dense_model"), \
         patch("src.tasks.ingestion.get_sparse_embedder"):
        
        # Setup mock return
        MockIndexer.return_value.ingest_file.return_value = {"status": "success"}
        
        # Execute via apply (simulates worker execution)
        task_result = ingest_file_task.apply(args=("dummy.py", "repo", "org"))
        result = task_result.result
        
        assert result["status"] == "success"
        MockIndexer.assert_called_once()
        MockIndexer.return_value.ingest_file.assert_called_with("dummy.py", "repo", "org")
