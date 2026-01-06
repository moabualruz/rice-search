"""
Local Cross-Encoder Reranker.

Uses sentence-transformers cross-encoder for fast, accurate reranking.
Much better than LLM-based reranking.
"""
import logging
from typing import List, Dict, Any, Optional

from sentence_transformers import CrossEncoder

from src.core.config import settings

logger = logging.getLogger(__name__)


class LocalReranker:
    """
    Dedicated cross-encoder reranker using sentence-transformers.

    Uses ms-marco-MiniLM-L-12-v2 by default - fast and accurate.
    """

    def __init__(self, model_name: str = None):
        """
        Initialize cross-encoder reranker.

        Args:
            model_name: Cross-encoder model (default: ms-marco-MiniLM-L-12-v2)
        """
        self.model_name = model_name or "cross-encoder/ms-marco-MiniLM-L-12-v2"
        self.model: Optional[CrossEncoder] = None
        logger.info(f"LocalReranker initialized with model: {self.model_name}")

    def _load_model(self):
        """Lazy load the model."""
        if self.model is None:
            logger.info(f"Loading cross-encoder model: {self.model_name}")
            self.model = CrossEncoder(self.model_name)
            logger.info("Cross-encoder model loaded successfully")

    async def rerank(
        self,
        query: str,
        documents: List[str],
        top_n: int = None,
    ) -> List[Dict[str, Any]]:
        """
        Rerank documents by relevance to query.

        Args:
            query: Search query
            documents: List of documents to rerank
            top_n: Number of top results to return

        Returns:
            List of reranked results with scores
        """
        try:
            self._load_model()

            # Create query-document pairs
            pairs = [[query, doc] for doc in documents]

            # Get relevance scores
            scores = self.model.predict(pairs)

            # Create results with scores
            results = [
                {
                    "index": idx,
                    "relevance_score": float(score),
                    "document": doc
                }
                for idx, (doc, score) in enumerate(zip(documents, scores))
            ]

            # Sort by score descending
            results.sort(key=lambda x: x["relevance_score"], reverse=True)

            # Return top_n if specified
            if top_n:
                results = results[:top_n]

            return results
        except Exception as e:
            logger.error(f"Reranking failed: {e}")
            raise


# Singleton instance
_local_reranker: Optional[LocalReranker] = None


def get_local_reranker() -> LocalReranker:
    """Get singleton local reranker."""
    global _local_reranker
    if _local_reranker is None:
        _local_reranker = LocalReranker()
    return _local_reranker
