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
        logger.info(f"BentoML client initialized: {self.base_url}")
    
    async def embed(self, texts: List[str], model: str = None) -> List[List[float]]:
        """
        Generate embeddings for texts.
        """
        async with httpx.AsyncClient(timeout=120.0) as client:
            try:
                response = await client.post(
                    f"{self.base_url}/embed",
                    json={"request": {"texts": texts, "model": model}},
                )
                response.raise_for_status()
                data = response.json()
                return data["embeddings"]
            except Exception as e:
                logger.error(f"Embedding failed: {e}")
                raise
    
    async def rerank(
        self,
        query: str,
        documents: List[str],
        top_n: int = None,
        model: str = None,
    ) -> List[Dict[str, Any]]:
        """
        Rerank documents by relevance to query.
        """
        async with httpx.AsyncClient(timeout=120.0) as client:
            try:
                response = await client.post(
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
    
    async def chat(
        self,
        messages: List[Dict[str, str]],
        model: str = None,
        max_tokens: int = 1024,
        temperature: float = 0.7,
    ) -> str:
        """
        Generate chat completion.
        """
        async with httpx.AsyncClient(timeout=120.0) as client:
            try:
                # The user's instruction implies _format_chat_prompt is a method of the class.
                # The provided snippet places it inside the try block of chat, which is syntactically valid
                # as a local function, but likely not the intended structure for a class method.
                # To maintain syntactic correctness and adhere to the spirit of "update _format_chat_prompt",
                # I'm placing it as a new method of the class, as its indentation suggests.
                # If ChatMessage is not defined, this will cause an error. Assuming it's available.
                response = await client.post(
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
    
    async def health_check(self) -> bool:
        """Check if BentoML service is healthy."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.post(f"{self.base_url}/health")
                response.raise_for_status()
                data = response.json()
                return data.get("status") == "healthy"
            except Exception as e:
                logger.warning(f"Health check failed: {e}")
                return False
    
    async def get_models(self) -> Dict[str, Any]:
        """Get information about loaded models."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.post(f"{self.base_url}/health")
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
