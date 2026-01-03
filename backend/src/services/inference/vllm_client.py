"""
vLLM OpenAI-compatible Client.
Provides access to LLM for RAG chat and query understanding.
"""
import logging
from typing import List, Dict, Optional, Any
import httpx

from src.core.config import settings

logger = logging.getLogger(__name__)


class VllmClient:
    """
    Client for vLLM server with OpenAI-compatible API.
    Used for RAG chat responses and optional query understanding.
    """
    
    def __init__(self, base_url: str):
        """
        Initialize vLLM client.
        
        Args:
            base_url: vLLM OpenAI-compatible API URL (e.g., "http://localhost:8084/v1")
        """
        self.base_url = base_url.rstrip("/")
        self._connected = False
        self._model_name: Optional[str] = None
    
    def _ensure_connected(self):
        """Lazy check for vLLM availability."""
        if self._connected:
            return
        
        try:
            with httpx.Client(timeout=10.0) as client:
                response = client.get(f"{self.base_url}/models")
                response.raise_for_status()
                models = response.json()
                if models.get("data"):
                    self._model_name = models["data"][0]["id"]
                    self._connected = True
                    logger.info(f"vLLM connected: {self.base_url}, model: {self._model_name}")
        except Exception as e:
            logger.warning(f"vLLM not available: {e}")
            raise
    
    def chat(
        self,
        messages: List[Dict[str, str]],
        max_tokens: int = 1024,
        temperature: float = 0.7,
        stream: bool = False
    ) -> str:
        """
        Generate chat completion.
        
        Args:
            messages: List of {"role": "...", "content": "..."} messages
            max_tokens: Maximum tokens to generate
            temperature: Sampling temperature
            stream: Whether to stream response (not implemented)
            
        Returns:
            Generated response text
        """
        self._ensure_connected()
        
        try:
            with httpx.Client(timeout=120.0) as client:
                response = client.post(
                    f"{self.base_url}/chat/completions",
                    json={
                        "model": self._model_name,
                        "messages": messages,
                        "max_tokens": max_tokens,
                        "temperature": temperature,
                    }
                )
                response.raise_for_status()
                result = response.json()
                return result["choices"][0]["message"]["content"]
        except Exception as e:
            logger.error(f"vLLM chat failed: {e}")
            raise
    
    def rag_answer(
        self,
        query: str,
        context: List[str],
        system_prompt: Optional[str] = None
    ) -> str:
        """
        Generate RAG-based answer using retrieved context.
        
        Args:
            query: User's question
            context: List of relevant code/document snippets
            system_prompt: Optional custom system prompt
            
        Returns:
            Generated answer
        """
        if system_prompt is None:
            system_prompt = """You are a helpful code search assistant. 
Answer the user's question based on the provided code context. 
Be concise and accurate. Reference specific code when relevant."""
        
        # Build context string
        context_str = "\n\n---\n\n".join([
            f"**Snippet {i+1}:**\n```\n{snippet}\n```"
            for i, snippet in enumerate(context[:5])  # Limit to 5 snippets
        ])
        
        messages = [
            {"role": "system", "content": system_prompt},
            {"role": "user", "content": f"Context:\n{context_str}\n\nQuestion: {query}"}
        ]
        
        return self.chat(messages)
    
    def classify_query(self, query: str, categories: List[str]) -> str:
        """
        Classify a query into one of the given categories.
        Useful for query understanding.
        
        Args:
            query: The search query
            categories: List of possible categories
            
        Returns:
            The predicted category
        """
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
    
    def health_check(self) -> bool:
        """Check if vLLM server is healthy."""
        try:
            with httpx.Client(timeout=5.0) as client:
                response = client.get(f"{self.base_url}/models")
                return response.status_code == 200
        except Exception:
            return False
    
    def get_model_info(self) -> Optional[Dict[str, Any]]:
        """Get information about the loaded model."""
        try:
            self._ensure_connected()
            with httpx.Client(timeout=5.0) as client:
                response = client.get(f"{self.base_url}/models")
                if response.status_code == 200:
                    return response.json()
        except Exception:
            pass
        return None


# Singleton instance with lazy loading
_vllm_client: Optional[VllmClient] = None


def get_vllm_client() -> VllmClient:
    """Get singleton vLLM client (lazy loaded)."""
    global _vllm_client
    if _vllm_client is None:
        _vllm_client = VllmClient(base_url=settings.VLLM_URL)
    return _vllm_client
