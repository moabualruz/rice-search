"""
BM42 Sparse Vector Encoder.

Uses Qdrant's FastEmbed for generating BM42-compatible sparse vectors.
BM42 is a Qdrant-native scoring algorithm that combines:
- Sparse lexical overlap
- IDF statistics (maintained by Qdrant)
- Dense vector similarity

This encoder ONLY generates the sparse vectors. BM42 scoring is done internally by Qdrant.
"""

import logging
from typing import List, NamedTuple, Optional

logger = logging.getLogger(__name__)


class SparseVector(NamedTuple):
    """Sparse vector representation for Qdrant."""
    indices: List[int]
    values: List[float]


class BM42Encoder:
    """
    BM42-compatible sparse vector encoder.
    
    Uses FastEmbed's sparse text embedding model optimized for BM42.
    """
    
    # Default model for BM42 sparse embeddings
    DEFAULT_MODEL = "Qdrant/bm42-all-minilm-l6-v2-attentions"
    
    def __init__(self, model_name: str = None):
        self.model_name = model_name or self.DEFAULT_MODEL
        self.model = None
        self._load_model()
    
    def _load_model(self):
        """Load the FastEmbed sparse model."""
        try:
            from fastembed import SparseTextEmbedding
            
            logger.info(f"Loading BM42 sparse model: {self.model_name}")
            self.model = SparseTextEmbedding(model_name=self.model_name)
            logger.info("BM42 sparse model loaded successfully")
            
        except ImportError:
            logger.error("fastembed not installed. Install with: pip install fastembed")
            raise
        except Exception as e:
            logger.error(f"Failed to load BM42 model: {e}")
            raise
    
    def encode(self, texts: List[str]) -> List[SparseVector]:
        """
        Encode texts to BM42-compatible sparse vectors.
        
        Args:
            texts: List of texts to encode
            
        Returns:
            List of SparseVector namedtuples
        """
        if not texts:
            return []
        
        if self.model is None:
            raise RuntimeError("BM42 model not loaded")
        
        results = []
        
        # FastEmbed returns generator
        embeddings = list(self.model.embed(texts))
        
        for emb in embeddings:
            # FastEmbed SparseEmbedding has .indices and .values attributes
            results.append(SparseVector(
                indices=emb.indices.tolist(),
                values=emb.values.tolist()
            ))
        
        return results
    
    def encode_single(self, text: str) -> SparseVector:
        """Encode a single text."""
        return self.encode([text])[0]


# Singleton instance
_bm42_encoder: Optional[BM42Encoder] = None


def get_bm42_encoder() -> BM42Encoder:
    """Get or create the singleton BM42 encoder."""
    global _bm42_encoder
    
    if _bm42_encoder is None:
        _bm42_encoder = BM42Encoder()
    
    return _bm42_encoder
