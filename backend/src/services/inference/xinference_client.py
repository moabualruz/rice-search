"""
Xinference Unified Client.

Single client for all model inference:
- Embeddings (dense)
- Reranking
- Chat/LLM

Uses OpenAI-compatible REST API.
"""
import logging
from typing import List, Dict, Any, Optional
import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class XinferenceClient:
    """
    Unified client for Xinference model serving.
    
    Xinference provides OpenAI-compatible API for all model types.
    Models are loaded dynamically via API, not at container startup.
    """
    
    def __init__(self, base_url: str):
        """
        Initialize Xinference client.
        
        Args:
            base_url: Xinference server URL (e.g., "http://localhost:9997")
        """
        self.base_url = base_url.rstrip("/")
        self._loaded_models: Dict[str, str] = {}  # model_type -> model_uid
    
    # =========================================================================
    # Model Management
    # =========================================================================
    
    def launch_model(
        self,
        model_name: str,
        model_type: str = "LLM",
        model_format: str = "pytorch",
        model_size_in_billions: int = None,
        quantization: str = None,
        replica: int = 1,
    ) -> str:
        """
        Launch a model on Xinference.
        
        Args:
            model_name: HuggingFace model name
            model_type: "LLM", "embedding", or "rerank"
            model_format: "pytorch", "ggml", etc.
            
        Returns:
            Model UID for subsequent requests
        """
        try:
            payload = {
                "model_name": model_name,
                "model_type": model_type,
                "model_format": model_format,
                "replica": replica,
            }
            if model_size_in_billions:
                payload["model_size_in_billions"] = model_size_in_billions
            if quantization:
                payload["quantization"] = quantization
            
            with httpx.Client(timeout=300.0) as client:  # Long timeout for model loading
                response = client.post(
                    f"{self.base_url}/v1/models",
                    json=payload
                )
                response.raise_for_status()
                result = response.json()
                model_uid = result.get("model_uid", model_name)
                logger.info(f"Launched model: {model_name} -> {model_uid}")
                return model_uid
        except Exception as e:
            logger.error(f"Failed to launch model {model_name}: {e}")
            raise RuntimeError(f"Failed to launch model: {e}")
    
    def list_models(self) -> List[Dict[str, Any]]:
        """List all running models."""
        try:
            with httpx.Client(timeout=10.0) as client:
                response = client.get(f"{self.base_url}/v1/models")
                response.raise_for_status()
                return response.json().get("data", [])
        except Exception as e:
            logger.error(f"Failed to list models: {e}")
            return []
    
    def get_or_launch_model(self, model_name: str, model_type: str) -> str:
        """Get model UID if running, or launch it."""
        # Check if already running
        models = self.list_models()
        for m in models:
            if m.get("model_name") == model_name or m.get("id") == model_name:
                return m.get("id", model_name)
        
        # Launch model
        return self.launch_model(model_name, model_type)
    
    # =========================================================================
    # Embeddings
    # =========================================================================
    
    def embed(self, texts: List[str], model: str = None) -> List[List[float]]:
        """
        Generate embeddings using Xinference.
        
        Args:
            texts: List of texts to embed
            model: Model UID or name (uses default if not specified)
            
        Returns:
            List of embedding vectors
        """
        model = model or settings.EMBEDDING_MODEL
        
        try:
            with httpx.Client(timeout=60.0) as client:
                response = client.post(
                    f"{self.base_url}/v1/embeddings",
                    json={
                        "model": model,
                        "input": texts
                    }
                )
                response.raise_for_status()
                result = response.json()
                return [d["embedding"] for d in result.get("data", [])]
        except Exception as e:
            logger.error(f"Embedding failed: {e}")
            raise RuntimeError(f"Embedding service unavailable: {e}")
    
    # =========================================================================
    # Reranking
    # =========================================================================
    
    def rerank(self, query: str, documents: List[str], model: str = None) -> List[float]:
        """
        Rerank documents using Xinference.
        
        Args:
            query: Search query
            documents: Documents to rerank
            model: Rerank model UID
            
        Returns:
            List of relevance scores
        """
        model = model or settings.RERANK_MODEL
        
        try:
            with httpx.Client(timeout=60.0) as client:
                response = client.post(
                    f"{self.base_url}/v1/rerank",
                    json={
                        "model": model,
                        "query": query,
                        "documents": documents
                    }
                )
                response.raise_for_status()
                result = response.json()
                # Sort by index and return scores
                results = result.get("results", [])
                sorted_results = sorted(results, key=lambda x: x.get("index", 0))
                return [r.get("relevance_score", 0) for r in sorted_results]
        except Exception as e:
            logger.error(f"Rerank failed: {e}")
            raise RuntimeError(f"Rerank service unavailable: {e}")
    
    # =========================================================================
    # Chat/LLM
    # =========================================================================
    
    def chat(
        self,
        messages: List[Dict[str, str]],
        model: str = None,
        max_tokens: int = 1024,
        temperature: float = 0.7,
    ) -> str:
        """
        Generate chat completion using Xinference.
        
        Args:
            messages: List of {"role": "...", "content": "..."} messages
            model: LLM model UID
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature
            
        Returns:
            Generated response text
        """
        model = model or settings.LLM_MODEL
        
        try:
            with httpx.Client(timeout=120.0) as client:
                response = client.post(
                    f"{self.base_url}/v1/chat/completions",
                    json={
                        "model": model,
                        "messages": messages,
                        "max_tokens": max_tokens,
                        "temperature": temperature,
                    }
                )
                response.raise_for_status()
                result = response.json()
                return result["choices"][0]["message"]["content"]
        except Exception as e:
            logger.error(f"Chat failed: {e}")
            raise RuntimeError(f"Chat service unavailable: {e}")
    
    def classify_query(self, query: str, categories: List[str]) -> str:
        """Classify query using LLM."""
        categories_str = ", ".join(categories)
        messages = [
            {
                "role": "system",
                "content": f"Classify the following code search query into one of these categories: {categories_str}. Respond with only the category name."
            },
            {"role": "user", "content": query}
        ]
        
        result = self.chat(messages, max_tokens=50, temperature=0.1)
        return result.strip()
    
    # =========================================================================
    # Health Check
    # =========================================================================
    
    def health_check(self) -> bool:
        """Check if Xinference is healthy."""
        try:
            with httpx.Client(timeout=5.0) as client:
                response = client.get(f"{self.base_url}/v1/models")
                return response.status_code == 200
        except Exception:
            return False


# Singleton instance
_xinference_client: Optional[XinferenceClient] = None


def get_xinference_client() -> XinferenceClient:
    """Get singleton Xinference client."""
    global _xinference_client
    if _xinference_client is None:
        _xinference_client = XinferenceClient(base_url=settings.XINFERENCE_URL)
    return _xinference_client
