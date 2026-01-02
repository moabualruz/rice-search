"""
API client for Rice Search backend.

Handles HTTP communication with the backend for indexing and search.
"""

import httpx
from pathlib import Path
from typing import List, Dict, Any, Optional
import base64

from src.cli.ricesearch.config import get_config


class APIClient:
    """HTTP client for Rice Search backend."""
    
    def __init__(self, base_url: Optional[str] = None):
        config = get_config()
        self.base_url = base_url or config.backend_url
        self.timeout = 30.0
    
    def _get_client(self) -> httpx.Client:
        """Get HTTP client."""
        config = get_config()
        headers = {"X-User-ID": str(config.user_id)}
        return httpx.Client(base_url=self.base_url, timeout=self.timeout, headers=headers)
    
    def health_check(self) -> bool:
        """Check backend health."""
        try:
            with self._get_client() as client:
                resp = client.get("/health")
                return resp.status_code == 200
        except Exception as e:
            return False
    
    def index_file(
        self,
        file_path: Path,
        org_id: str = "public"
    ) -> Dict[str, Any]:
        """
        Index a file via the backend API.
        
        Args:
            file_path: Path to file to index
            org_id: Organization ID
            
        Returns:
            API response dict
        """
        try:
            with self._get_client() as client:
                with open(file_path, 'rb') as f:
                    files = {'file': (file_path.name, f)}
                    data = {'org_id': org_id}
                    resp = client.post(
                        "/api/v1/ingest/file",
                        files=files,
                        data=data
                    )
                    return resp.json()
        except Exception as e:
            return {"status": "error", "message": str(e)}
    
    def search(
        self,
        query: str,
        limit: int = 10,
        org_id: str = "public",
        hybrid: bool = True
    ) -> List[Dict[str, Any]]:
        """
        Search indexed content.
        
        Args:
            query: Search query
            limit: Max results
            org_id: Organization ID
            hybrid: Use hybrid search
            
        Returns:
            List of search results
        """
        try:
            with self._get_client() as client:
                resp = client.post(
                    "/api/v1/search/query",
                    json={
                        "query": query,
                        "mode": "search",
                        "hybrid": hybrid
                    }
                )
                if resp.status_code != 200:
                    print(f"Backend Error ({resp.status_code}): {resp.text}")
                    return []
                
                data = resp.json()
                return data.get("results", [])[:limit]
        except Exception as e:
            print(f"API Client Error: {e}")
            return []


# Singleton instance
_client: Optional[APIClient] = None

def get_api_client() -> APIClient:
    """Get global API client instance."""
    global _client
    if _client is None:
        _client = APIClient()
    return _client
