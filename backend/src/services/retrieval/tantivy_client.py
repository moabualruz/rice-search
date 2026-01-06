"""
Tantivy BM25 HTTP Client.

Communicates with the Rust Tantivy service for lexical BM25 search.
"""

import logging
from typing import List, Dict, Optional, Tuple
from dataclasses import dataclass

import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


@dataclass
class BM25Result:
    """Result from BM25 search."""
    chunk_id: str
    score: float


class TantivyClient:
    """
    HTTP client for the Rust Tantivy BM25 service.
    
    Endpoints:
    - POST /index - Index a chunk
    - POST /index/batch - Batch index chunks
    - POST /search - BM25 search
    - DELETE /index/{chunk_id} - Delete chunk
    - POST /index/clear - Clear index
    - GET /health - Health check
    """
    
    def __init__(self, base_url: str = None, timeout: float = None):
        self.base_url = base_url or settings.TANTIVY_URL
        self.timeout = timeout if timeout is not None else settings.TANTIVY_TIMEOUT
        self._client = None
    
    @property
    def client(self) -> httpx.Client:
        """Get or create HTTP client."""
        if self._client is None:
            self._client = httpx.Client(
                base_url=self.base_url,
                timeout=self.timeout
            )
        return self._client
    
    def health(self) -> Dict:
        """Check service health."""
        try:
            response = self.client.get("/health")
            response.raise_for_status()
            return response.json()
        except Exception as e:
            logger.error(f"Tantivy health check failed: {e}")
            return {"status": "unhealthy", "error": str(e)}
    
    def index(self, chunk_id: str, text: str) -> bool:
        """
        Index a single chunk.
        
        Args:
            chunk_id: Unique identifier for the chunk
            text: Text content to index
            
        Returns:
            True if successful
        """
        try:
            response = self.client.post(
                "/index",
                json={"chunk_id": chunk_id, "text": text}
            )
            response.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Failed to index chunk {chunk_id}: {e}")
            return False
    
    def batch_index(self, chunks: List[Tuple[str, str]]) -> bool:
        """
        Index multiple chunks in a batch.
        
        Args:
            chunks: List of (chunk_id, text) tuples
            
        Returns:
            True if successful
        """
        try:
            payload = {
                "chunks": [
                    {"chunk_id": cid, "text": text}
                    for cid, text in chunks
                ]
            }
            response = self.client.post("/index/batch", json=payload)
            response.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Failed to batch index {len(chunks)} chunks: {e}")
            return False
    
    def search(self, query: str, limit: int = 10, min_score: Optional[float] = None) -> List[BM25Result]:
        """
        Search using BM25.

        Args:
            query: Search query
            limit: Maximum number of results
            min_score: Minimum score threshold (filters out lower scores)

        Returns:
            List of BM25Result objects
        """
        try:
            payload = {"query": query, "limit": limit}
            if min_score is not None:
                payload["min_score"] = min_score

            response = self.client.post("/search", json=payload)
            response.raise_for_status()
            data = response.json()

            return [
                BM25Result(chunk_id=r["chunk_id"], score=r["score"])
                for r in data.get("results", [])
            ]
        except Exception as e:
            logger.error(f"BM25 search failed: {e}")
            return []
    
    def delete(self, chunk_id: str) -> bool:
        """Delete a chunk from the index."""
        try:
            response = self.client.delete(f"/index/{chunk_id}")
            response.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Failed to delete chunk {chunk_id}: {e}")
            return False
    
    def clear(self) -> bool:
        """Clear the entire index."""
        try:
            response = self.client.post("/index/clear")
            response.raise_for_status()
            return True
        except Exception as e:
            logger.error(f"Failed to clear index: {e}")
            return False
    
    def close(self):
        """Close the HTTP client."""
        if self._client is not None:
            self._client.close()
            self._client = None


# Singleton instance
_tantivy_client: Optional[TantivyClient] = None


def get_tantivy_client() -> TantivyClient:
    """Get or create the singleton Tantivy client."""
    global _tantivy_client
    
    if _tantivy_client is None:
        _tantivy_client = TantivyClient()
    
    return _tantivy_client
