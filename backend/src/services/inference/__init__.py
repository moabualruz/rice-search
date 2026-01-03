"""
Inference client modules for BentoML unified model serving.

Single service for all model inference:
- Embeddings (dense)
- Reranking
- Chat/LLM

All legacy clients (Xinference, TEI, Triton, vLLM) are deprecated in favor of BentoML.
"""
from .bentoml_client import BentoMLClient, get_bentoml_client

__all__ = [
    "BentoMLClient",
    "get_bentoml_client",
]
