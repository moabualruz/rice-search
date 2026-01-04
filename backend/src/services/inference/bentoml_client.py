"""
BentoML Inference Client.

Client for the unified BentoML inference service.
Provides embeddings, reranking, and LLM chat.
"""
import logging
from typing import List, Dict, Any, Optional

import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class BentoMLClient:
    """
    Client for BentoML unified inference service.
    
    Endpoints:
    - POST /embed - Text embeddings
    - POST /rerank - Document reranking
    - POST /chat - LLM chat completion
    - POST /health - Health check
    """
    
    def __init__(self, base_url: str = None):
        """
        Initialize BentoML client.
        
        Args:
            base_url: BentoML server URL (e.g., "http://localhost:3001")
        """
        self.base_url = (base_url or settings.BENTOML_URL).rstrip("/")
        self.client = httpx.Client(timeout=120.0)
        logger.info(f"BentoML client initialized: {self.base_url}")
    
    def embed(self, texts: List[str], model: str = None) -> List[List[float]]:
        """
        Generate embeddings for texts.
        
        Args:
            texts: List of texts to embed
            model: Model name (optional, uses server default)
            
        Returns:
            List of embedding vectors
        """
        try:
            # BentoML requires request wrapper
            response = self.client.post(
                f"{self.base_url}/embed",
                json={"request": {"texts": texts, "model": model}},
            )
            response.raise_for_status()
            data = response.json()
            return data["embeddings"]
        except Exception as e:
            logger.error(f"Embedding failed: {e}")
            raise
    
    def rerank(
        self,
        query: str,
        documents: List[str],
        top_n: int = None,
        model: str = None,
    ) -> List[Dict[str, Any]]:
        """
        Rerank documents by relevance to query.
        
        Args:
            query: Search query
            documents: Documents to rerank
            top_n: Return only top N results
            model: Model name (optional)
            
        Returns:
            List of {index, score, text} dicts sorted by score
        """
        try:
            # BentoML requires request wrapper
            response = self.client.post(
                f"{self.base_url}/rerank",
                json={
                    "request": {
                        "query": query,
                        "documents": documents,
                        "top_n": top_n,
                        "model": model,
                    }
                },
            )
            response.raise_for_status()
            data = response.json()
            return data["results"]
        except Exception as e:
            logger.error(f"Rerank failed: {e}")
            raise
    
    def chat(
        self,
        messages: List[Dict[str, str]],
        model: str = None,
        max_tokens: int = 1024,
        temperature: float = 0.7,
    ) -> str:
        """
        Generate chat completion.
        
        Args:
            messages: List of {"role": "...", "content": "..."} messages
            model: Model name (optional)
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature
            
        Returns:
            Generated response text
        """
        try:
            # BentoML requires request wrapper
            response = self.client.post(
                f"{self.base_url}/chat",
                json={
                    "request": {
                        "messages": messages,
                        "model": model,
                        "max_tokens": max_tokens,
                        "temperature": temperature,
                    }
                },
            )
            response.raise_for_status()
            data = response.json()
            return data["content"]
        except Exception as e:
            logger.error(f"Chat failed: {e}")
            raise
    
    def health_check(self) -> bool:
        """Check if BentoML service is healthy."""
        try:
            response = self.client.post(f"{self.base_url}/health", timeout=5.0)
            response.raise_for_status()
            data = response.json()
            return data.get("status") == "healthy"
        except Exception as e:
            logger.warning(f"Health check failed: {e}")
            return False
    
    def get_models(self) -> Dict[str, Any]:
        """Get information about loaded models."""
        try:
            response = self.client.post(f"{self.base_url}/health", timeout=5.0)
            response.raise_for_status()
            data = response.json()
            return data.get("models", {})
        except Exception as e:
            logger.warning(f"Failed to get models: {e}")
            return {}


# Singleton instance
_bentoml_client: Optional[BentoMLClient] = None


def get_bentoml_client() -> BentoMLClient:
    """Get singleton BentoML client."""
    global _bentoml_client
    if _bentoml_client is None:
        _bentoml_client = BentoMLClient()
    return _bentoml_client
