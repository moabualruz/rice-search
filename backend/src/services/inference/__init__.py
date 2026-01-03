"""
Inference client modules for Xinference unified model serving.

Single service for all model inference:
- Embeddings (dense)
- Sparse embeddings (via LLM if needed)
- Reranking
- Chat/LLM

All legacy clients (TEI, Triton, vLLM) are deprecated in favor of Xinference.
"""
from .xinference_client import XinferenceClient, get_xinference_client

__all__ = [
    "XinferenceClient",
    "get_xinference_client",
]
