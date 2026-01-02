"""
Neural Reranker Service.

Implements cross-encoder reranking of top-K results using BAAI/bge-reranker-base.
This improves search quality by re-scoring results with a more accurate model.
"""

import logging
from typing import List, Dict, Any, Optional, Tuple
# from sentence_transformers import CrossEncoder # Lazy

from src.core.config import settings

logger = logging.getLogger(__name__)


class Reranker:
    """Cross-encoder reranker for search results."""
    
    _instance: Optional["Reranker"] = None
    
    def __init__(self, model_name: str = None):
        """
        Initialize reranker.
        
        Args:
            model_name: HuggingFace model name for cross-encoder
        """
        if model_name:
            self.model_name = model_name
        else:
            # Get active reranker model from admin store
            from src.services.admin.admin_store import get_admin_store
            store = get_admin_store()
            self.model_name = store.get_active_model_for_type("reranker") or settings.RERANK_MODEL
    
    @classmethod
    def get_instance(cls) -> "Reranker":
        """Get singleton instance."""
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance
    
    def _get_model(self):
        """Get the model instance from ModelManager, loading it if necessary."""
        from src.services.model_manager import get_model_manager
        manager = get_model_manager()
        
        def loader():
            import gc
            import torch
            from src.core.device import get_device
            from src.services.admin.admin_store import get_admin_store
            from sentence_transformers import CrossEncoder

            # Check if GPU is enabled for reranker
            store = get_admin_store()
            models = store.get_models()
            gpu_enabled = models.get("reranker", {}).get("gpu_enabled", True)
            
            if gpu_enabled:
                device = get_device()
            else:
                device = "cpu"
                
            logger.info(f"Loading reranker model: {self.model_name} on {device}")
            
            # Load model
            # Load model on CPU first
            model = CrossEncoder(self.model_name, device="cpu", trust_remote_code=True)
            
            if device != "cpu":
                 model.model.to(device)
                 model._target_device = torch.device(device)
            
            # Clear CPU memory if on GPU
            if device == "cuda":
                gc.collect()
                if torch.cuda.is_available():
                    torch.cuda.empty_cache()
            
            return model
        
        # Ensure loaded using Manager - use actual model name as ID
        manager.load_model(self.model_name, loader)
        return manager.get_model_instance(self.model_name)
            
    def rerank(
        self,
        query: str,
        results: List[Dict[str, Any]],
        top_k: int = None,
        score_threshold: float = None
    ) -> List[Dict[str, Any]]:
        """
        Rerank search results using cross-encoder.
        """
        if not results:
            return results
        
        if not settings.RERANK_ENABLED:
            return results
        
        try:
            # Get model from manager (ephemeral reference)
            model = self._get_model()
            
            if model is None:
                logger.warning("Reranker model not available, returning original results")
                return results
        
            # Prepare pairs for cross-encoder
            pairs = []
            for result in results:
                text = result.get("text", "")
                if not text:
                    text = result.get("content", "")
                pairs.append((query, text))
            
            # Get cross-encoder scores
            scores = model.predict(pairs)
            
            # Attach scores and sort
            scored_results = []
            for i, result in enumerate(results):
                result_copy = result.copy()
                result_copy["rerank_score"] = float(scores[i])
                result_copy["original_score"] = result.get("score", 0.0)
                scored_results.append(result_copy)
            
            # Sort by rerank score (descending)
            scored_results.sort(key=lambda x: x["rerank_score"], reverse=True)
            
            # Apply threshold filter
            if score_threshold is not None:
                scored_results = [
                    r for r in scored_results 
                    if r["rerank_score"] >= score_threshold
                ]
            
            # Apply top_k
            if top_k is not None:
                scored_results = scored_results[:top_k]
            
            # Update score field to rerank score
            for result in scored_results:
                result["score"] = result["rerank_score"]
            
            return scored_results
            
        except Exception as e:
            logger.error(f"Reranking failed: {e}")
            return results
    
    def score_pair(self, query: str, text: str) -> float:
        """Score a single query-text pair."""
        try:
            model = self._get_model()
            if model is None:
                return 0.0
            score = model.predict([(query, text)])[0]
            return float(score)
        except Exception as e:
            logger.error(f"Pair scoring failed: {e}")
            return 0.0


# Module-level convenience functions

def rerank_results(
    query: str,
    results: List[Dict[str, Any]],
    top_k: int = None
) -> List[Dict[str, Any]]:
    """Rerank results using default reranker."""
    return Reranker.get_instance().rerank(query, results, top_k)


def get_reranker() -> Reranker:
    """Get global reranker instance."""
    return Reranker.get_instance()
