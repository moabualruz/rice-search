"""
Unit tests for File Ingestion.
"""
import pytest


@pytest.mark.unit
class TestChunking:
    """Test code chunking."""
    
    def test_chunk_text_exists(self):
        """Test that chunker module exists."""
        from src.services.ingestion.chunker import chunk_text
        
        # Test basic functionality
        text = "a" * 1000
        chunks = chunk_text(text, chunk_size=200)
        
        assert len(chunks) > 0
        assert isinstance(chunks, list)
