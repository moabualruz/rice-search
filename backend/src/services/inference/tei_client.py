"""
TEI (Text Embeddings Inference) Client.
Provides access to embedding and reranking models via REST API.

ALL model inference is delegated to TEI servers.
No in-process model loading - backend only orchestrates service calls.
"""
import logging
from typing import List, Optional
import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class TeiClient:
    """
    Client for Hugging Face Text Embeddings Inference server.
    Uses REST API for embedding and reranking.
    """
    
    def __init__(self, base_url: str, model_type: str = "embed"):
        """
        Initialize TEI client.
        
        Args:
            base_url: REST endpoint (e.g., "http://localhost:8081")
            model_type: "embed" or "rerank"
        """
        self.base_url = base_url.rstrip("/")
        self.model_type = model_type
    
    def embed(self, texts: List[str]) -> List[List[float]]:
        """
        Generate dense embeddings for texts.
        
        Args:
            texts: List of texts to embed
            
        Returns:
            List of embedding vectors
            
        Raises:
            RuntimeError: If TEI service fails
        """
        try:
            with httpx.Client(timeout=60.0) as client:
                response = client.post(
                    f"{self.base_url}/embed",
                    json={"inputs": texts}
                )
                response.raise_for_status()
                return response.json()
        except Exception as e:
            logger.error(f"TEI embed failed: {e}")
            raise RuntimeError(f"TEI embedding service unavailable: {e}")
    
    def rerank(self, query: str, documents: List[str]) -> List[float]:
        """
        Rerank documents for a query.
        
        Args:
            query: The search query
            documents: List of documents to rerank
            
        Returns:
            List of relevance scores (higher = more relevant)
            
        Raises:
            RuntimeError: If TEI service fails
        """
        try:
            with httpx.Client(timeout=60.0) as client:
                response = client.post(
                    f"{self.base_url}/rerank",
                    json={
                        "query": query,
                        "texts": documents,
                        "return_text": False
                    }
                )
                response.raise_for_status()
                results = response.json()
                # Sort by index and return scores
                sorted_results = sorted(results, key=lambda x: x["index"])
                return [r["score"] for r in sorted_results]
        except Exception as e:
            logger.error(f"TEI rerank failed: {e}")
            raise RuntimeError(f"TEI reranking service unavailable: {e}")
    
    def health_check(self) -> bool:
        """Check if TEI server is healthy."""
        try:
            with httpx.Client(timeout=5.0) as client:
                response = client.get(f"{self.base_url}/health")
                return response.status_code == 200
        except Exception:
            return False


# Singleton instances with lazy loading
_embed_client: Optional[TeiClient] = None
_rerank_client: Optional[TeiClient] = None


def get_tei_embed_client() -> TeiClient:
    """Get singleton TEI embedding client (lazy loaded)."""
    global _embed_client
    if _embed_client is None:
        _embed_client = TeiClient(
            base_url=settings.TEI_EMBED_REST_URL,
            model_type="embed"
        )
    return _embed_client


def get_tei_rerank_client() -> TeiClient:
    """Get singleton TEI reranking client (lazy loaded)."""
    global _rerank_client
    if _rerank_client is None:
        _rerank_client = TeiClient(
            base_url=settings.TEI_RERANK_REST_URL,
            model_type="rerank"
        )
    return _rerank_client
