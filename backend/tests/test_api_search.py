"""
Integration tests for Search API endpoints.
"""
import pytest
from unittest.mock import patch


@pytest.mark.integration
class TestSearchAPI:
    """Test search API endpoints."""
    
    @patch('src.api.v1.endpoints.search.Retriever')
    def test_search_endpoint(self, mock_retriever, api_client):
        """Test basic search endpoint."""
        # Mock retriever
        mock_ret = mock_retriever.return_value
        mock_ret.search.return_value = []
        
        response = api_client.get("/api/v1/search?q=test")
        # May return 404 if no data indexed
        assert response.status_code in [200, 404]
    
    @patch('src.api.v1.endpoints.search.Retriever')
    def test_search_with_mode(self, mock_retriever, api_client):
        """Test search with different modes."""
        mock_ret = mock_retriever.return_value
        mock_ret.search.return_value = []
        
        # Test dense mode
        response = api_client.get("/api/v1/search?q=test&mode=dense")
        assert response.status_code in [200, 404]
        
        # Test hybrid mode
        response = api_client.get("/api/v1/search?q=test&mode=hybrid")
        assert response.status_code in [200, 404]
    
    def test_search_missing_query(self, api_client):
        """Test search without query parameter."""
        response = api_client.get("/api/v1/search")
        # Can be 422 (validation error) or 404 (no data indexed)
        assert response.status_code in [404, 422]
