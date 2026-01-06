"""
Inference client modules for Ollama.

Ollama handles:
- Embeddings (dense) - nomic-embed-text
- Chat/LLM - qwen2.5-coder:1.5b

Reranking uses dedicated cross-encoder (sentence-transformers).
"""
from .ollama_client import (
    OllamaClient,
    get_inference_client,
)

# Backward compatibility aliases
get_unified_inference_client = get_inference_client
get_bentoml_client = get_inference_client
UnifiedInferenceClient = OllamaClient
BentoMLClient = OllamaClient

__all__ = [
    "OllamaClient",
    "get_inference_client",
    "get_unified_inference_client",  # Alias
    "BentoMLClient",  # Backward compatibility
    "get_bentoml_client",  # Backward compatibility
    "UnifiedInferenceClient",  # Backward compatibility
]
