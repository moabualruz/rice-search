"""
Ollama Inference Client.

Client for Ollama with automatic memory management and smart queuing.

Architecture:
- Dense Embeddings: qwen3-embedding:4b (code-focused, 100+ langs, 32k tokens)
- Sparse Embeddings: SPLADE via sentence-transformers (naver/splade-cocondenser-ensembledistil)
- Reranking: Cross-encoder via sentence-transformers (ms-marco-MiniLM-L-12-v2)
- LLM Chat: qwen2.5-coder:1.5b via Ollama

Note: Ollama doesn't natively support rerankers or sparse embeddings, so we use
dedicated models via sentence-transformers for those tasks.
"""
import logging
from typing import List, Dict, Any, Optional

import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class OllamaClient:
    """
    Client for Ollama inference server.

    Endpoints:
    - POST /api/embeddings - Text embeddings
    - POST /api/generate - Text generation (for reranking)
    - POST /api/chat - LLM chat
    - GET /api/tags - List models
    """

    def __init__(self, base_url: str = None):
        """
        Initialize Ollama client.

        Args:
            base_url: Ollama server URL (e.g., "http://localhost:11434")
        """
        self.base_url = (base_url or settings.OLLAMA_BASE_URL).rstrip("/")
        logger.info(f"Ollama client initialized: {self.base_url}")

    async def embed(self, texts: List[str], model: str = None) -> List[List[float]]:
        """
        Generate embeddings for texts using Ollama.

        Args:
            texts: List of texts to embed
            model: Optional model name (defaults to nomic-embed-text)

        Returns:
            List of embedding vectors
        """
        model = model or settings.EMBEDDING_MODEL_NAME

        async with httpx.AsyncClient(timeout=310.0) as client:
            try:
                # Ollama embeddings API
                embeddings = []
                for text in texts:
                    response = await client.post(
                        f"{self.base_url}/api/embeddings",
                        json={
                            "model": model,
                            "prompt": text
                        },
                    )
                    response.raise_for_status()
                    data = response.json()
                    embeddings.append(data["embedding"])

                return embeddings
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

        Uses dedicated cross-encoder (ms-marco-MiniLM) for fast, accurate reranking.

        Args:
            query: Search query
            documents: List of documents to rerank
            top_n: Number of top results to return
            model: Optional model name (ignored - uses cross-encoder)

        Returns:
            List of reranked results with scores
        """
        from .local_reranker import get_local_reranker

        try:
            reranker = get_local_reranker()
            return await reranker.rerank(query, documents, top_n)
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
        Generate chat completion using Ollama.

        Args:
            messages: Chat messages [{"role": "user", "content": "..."}]
            model: Optional model name (defaults to qwen2.5-coder:1.5b)
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature

        Returns:
            Generated text response
        """
        model = model or settings.LLM_MODEL

        async with httpx.AsyncClient(timeout=120.0) as client:
            try:
                # Ollama chat API
                response = await client.post(
                    f"{self.base_url}/api/chat",
                    json={
                        "model": model,
                        "messages": messages,
                        "stream": False,
                        "options": {
                            "num_predict": max_tokens,
                            "temperature": temperature,
                        }
                    },
                )
                response.raise_for_status()
                data = response.json()

                # Extract content from Ollama format
                return data["message"]["content"]
            except Exception as e:
                logger.error(f"Chat failed: {e}")
                raise

    async def health_check(self) -> bool:
        """Check if Ollama service is healthy."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.get(f"{self.base_url}/api/tags")
                response.raise_for_status()
                return True
            except Exception as e:
                logger.warning(f"Health check failed: {e}")
                return False

    async def get_models(self) -> Dict[str, Any]:
        """Get information about available models."""
        async with httpx.AsyncClient(timeout=5.0) as client:
            try:
                response = await client.get(f"{self.base_url}/api/tags")
                response.raise_for_status()
                data = response.json()
                return data
            except Exception as e:
                logger.warning(f"Failed to get models: {e}")
                return {}

    async def pull_model(self, model_name: str) -> bool:
        """Pull (download) a model if not already available."""
        async with httpx.AsyncClient(timeout=600.0) as client:
            try:
                logger.info(f"Pulling Ollama model: {model_name}")
                response = await client.post(
                    f"{self.base_url}/api/pull",
                    json={"name": model_name},
                )
                response.raise_for_status()
                logger.info(f"Model {model_name} pulled successfully")
                return True
            except Exception as e:
                logger.error(f"Failed to pull model {model_name}: {e}")
                return False


# Singleton instance
_ollama_client: Optional[OllamaClient] = None


def get_inference_client() -> OllamaClient:
    """Get singleton Ollama client."""
    global _ollama_client
    if _ollama_client is None:
        _ollama_client = OllamaClient()
    return _ollama_client


# Backward compatibility aliases
get_unified_inference_client = get_inference_client
get_bentoml_client = get_inference_client
