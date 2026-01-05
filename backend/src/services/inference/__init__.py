"""
Inference client modules for unified-inference service.

Single service for all model inference:
- Embeddings (dense) - default: bge-base-en
- Reranking - default: bge-reranker
- Chat/LLM - default: qwen-coder-1.5b

Uses OpenAI-compatible API with default model auto-selection.
"""
from .unified_inference_client import (
    UnifiedInferenceClient,
    get_unified_inference_client,
    get_bentoml_client,  # Backward compatibility
    BentoMLClient,  # Backward compatibility alias
)

__all__ = [
    "UnifiedInferenceClient",
    "get_unified_inference_client",
    "BentoMLClient",  # Backward compatibility
    "get_bentoml_client",  # Backward compatibility
]
