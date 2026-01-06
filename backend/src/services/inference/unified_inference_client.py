"""
Unified Inference Client.

Client for the unified-inference service (SGLang-based).
Provides embeddings, reranking, and LLM chat with default model auto-selection.
"""
import logging
from typing import List, Dict, Any, Optional

import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class UnifiedInferenceClient:
    """
    Client for unified-inference service (OpenAI-compatible API).

    Endpoints:
    - POST /v1/embeddings - Text embeddings (default: bge-base-en)
    - POST /v1/rerank - Document reranking (default: bge-reranker)
    - POST /v1/chat/completions - LLM chat (default: qwen-coder-1.5b)
    - GET /health - Health check

    Default models are auto-selected based on endpoint.
    Custom models can be specified via 'model' parameter.
    """

    def __init__(self, base_url: str = None):
        """
        Initialize unified-inference client.

        Args:
            base_url: Unified-inference server URL (e.g., "http://localhost:3001")
        """
        self.base_url = (base_url or settings.INFERENCE_URL).rstrip("/")
        logger.info(f"Unified-inference client initialized: {self.base_url}")
    
    async def embed(self, texts: List[str], model: str = None) -> List[List[float]]:
        """
        Generate embeddings for texts using OpenAI-compatible API.

        Args:
            texts: List of texts to embed
            model: Optional model name (defaults to bge-base-en if not specified)

        Returns:
            List of embedding vectors
        """
        async with httpx.AsyncClient(timeout=310.0) as client:
            try:
                # OpenAI-compatible format
                payload = {"input": texts}
                if model:
                    payload["model"] = model

                response = await client.post(
                    f"{self.base_url}/v1/embeddings",
                    json=payload,
                )
                response.raise_for_status()
                data = response.json()

                # Extract embeddings from OpenAI format
                # Response: {"data": [{"embedding": [...]}, ...]}
                return [item["embedding"] for item in data["data"]]
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

        Args:
            query: Search query
            documents: List of documents to rerank
            top_n: Number of top results to return
            model: Optional model name (defaults to bge-reranker if not specified)

        Returns:
            List of reranked results with scores
        """
        async with httpx.AsyncClient(timeout=310.0) as client:
            try:
                # SGLang rerank format
                payload = {
                    "query": query,
                    "documents": documents,
                }
                if top_n:
                    payload["top_n"] = top_n
                if model:
                    payload["model"] = model

                response = await client.post(
                    f"{self.base_url}/v1/rerank",
                    json=payload,
                )
                response.raise_for_status()
                data = response.json()

                # Return results (format: [{"index": 0, "relevance_score": 0.9}, ...])
                return data.get("results", [])
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
        Generate chat completion using OpenAI-compatible API.

        Args:
            messages: Chat messages in OpenAI format [{"role": "user", "content": "..."}]
            model: Optional model name (defaults to qwen-coder-1.5b if not specified)
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature

        Returns:
            Generated text response
        """
        async with httpx.AsyncClient(timeout=120.0) as client:
            try:
                # OpenAI-compatible format
                payload = {
                    "messages": messages,
                    "max_tokens": max_tokens,
                    "temperature": temperature,
                }
                if model:
                    payload["model"] = model

                response = await client.post(
                    f"{self.base_url}/v1/chat/completions",
                    json=payload,
                )
                response.raise_for_status()
                data = response.json()

                # Extract content from OpenAI format
                # Response: {"choices": [{"message": {"content": "..."}}]}
                return data["choices"][0]["message"]["content"]
            except Exception as e:
                logger.error(f"Chat failed: {e}")
                raise
    
    async def health_check(self) -> bool:
        """Check if unified-inference service is healthy."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.get(f"{self.base_url}/health")
                response.raise_for_status()
                data = response.json()
                return data.get("status") == "healthy"
            except Exception as e:
                logger.warning(f"Health check failed: {e}")
                return False

    async def get_models(self) -> Dict[str, Any]:
        """Get information about available models."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.get(f"{self.base_url}/v1/models")
                response.raise_for_status()
                data = response.json()
                return data
            except Exception as e:
                logger.warning(f"Failed to get models: {e}")
                return {}


# Singleton instance
_unified_inference_client: Optional[UnifiedInferenceClient] = None


def get_inference_client() -> UnifiedInferenceClient:
    """Get singleton unified-inference client."""
    global _unified_inference_client
    if _unified_inference_client is None:
        _unified_inference_client = UnifiedInferenceClient()
    return _unified_inference_client


# Backward compatibility aliases
get_unified_inference_client = get_inference_client
get_bentoml_client = get_inference_client
BentoMLClient = UnifiedInferenceClient
