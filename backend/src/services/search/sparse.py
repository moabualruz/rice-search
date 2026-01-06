"""
Sparse Embedding Service.

For sparse embeddings, we use a simple keyword-based approach.
SPLADE support can be added to unified-inference service when needed.
"""
import logging
from typing import List
from collections import namedtuple

from src.core.config import settings

logger = logging.getLogger(__name__)

SparseEmbedding = namedtuple("SparseEmbedding", ["indices", "values"])


def sparse_embed(text: str) -> SparseEmbedding:
    """
    Generate sparse embedding for text.
    
    Uses a simple keyword-based approach for now.
    
    TODO: Add SPLADE model to unified-inference when needed.
    """
    # Simple keyword-based sparse embedding
    # Uses word frequencies as sparse vector
    words = text.lower().split()
    word_counts = {}
    for word in words:
        if len(word) > 2:  # Skip short words
            word_counts[word] = word_counts.get(word, 0) + 1
    
    # Convert to sparse format (word hash -> count)
    indices = []
    values = []
    for word, count in word_counts.items():
        # Use hash of word as index (modulo vocab size)
        idx = hash(word) % 30000  # Typical vocab size
        indices.append(idx)
        values.append(float(count))
    
    return SparseEmbedding(indices=indices, values=values)


def batch_sparse_embed(texts: List[str]) -> List[SparseEmbedding]:
    """Embed multiple texts."""
    return [sparse_embed(text) for text in texts]
