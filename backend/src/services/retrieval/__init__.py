"""
Retrieval services package.

Contains encoders for BM25, SPLADE, BM42, and fusion logic.
"""

from .splade_encoder import SpladeEncoder, get_splade_encoder
from .bm42_encoder import BM42Encoder, get_bm42_encoder
from .tantivy_client import TantivyClient, get_tantivy_client
from .fusion import rrf_fusion, weighted_fusion

__all__ = [
    "SpladeEncoder",
    "get_splade_encoder",
    "BM42Encoder", 
    "get_bm42_encoder",
    "TantivyClient",
    "get_tantivy_client",
    "rrf_fusion",
    "weighted_fusion",
]
