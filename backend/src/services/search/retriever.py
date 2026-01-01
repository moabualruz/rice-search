"""
Retriever service with hybrid search support.

Supports both dense-only and hybrid (dense + sparse) search using Qdrant.
"""

from typing import List, Dict, Optional
from sentence_transformers import SentenceTransformer
from qdrant_client.models import (
    Filter,
    FieldCondition,
    MatchValue,
    Prefetch,
    FusionQuery,
    Fusion,
    SparseVector,
    NamedVector,
    NamedSparseVector,
)

from src.db.qdrant import get_qdrant_client
from src.core.config import settings

from src.services.model_manager import get_model_manager

# Lazy load dense model via Manager
qdrant = get_qdrant_client()
COLLECTION_NAME = "rice_chunks"

def get_dense_model():
    """Get or load dense model via ModelManager."""
    manager = get_model_manager()
    
    def loader():
        import gc
        import torch
        import logging
        from src.core.device import get_device
        from src.services.admin.admin_store import get_admin_store
        
        logger = logging.getLogger(__name__)
        
        # Check if GPU is enabled for this model
        store = get_admin_store()
        models = store.get_models()
        gpu_enabled = models.get("dense", {}).get("gpu_enabled", True)
        
        if gpu_enabled and torch.cuda.is_available():
            device = "cuda"
        else:
            device = "cpu"
            
        logger.info(f"Loading dense model on {device}")
        
        # Load model - initially loads on CPU
        model = SentenceTransformer(settings.EMBEDDING_MODEL)
        
        # Move to target device and clear CPU copy
        if device == "cuda":
            model = model.to(device)
            # Force garbage collection to free CPU tensors
            gc.collect()
            torch.cuda.empty_cache()
        
        return model
    
    # Register/Load
    manager.load_model("dense", loader)
    status = manager.get_model_status("dense")
    
    # Return instance if loaded
    if status["loaded"]:
        return manager._models["dense"]["instance"]
    raise RuntimeError("Failed to load dense model")

# Lazy load sparse embedder
_sparse_embedder = None

def get_sparse_embedder():
    """Lazy load sparse embedder."""
    global _sparse_embedder
    if _sparse_embedder is None and settings.SPARSE_ENABLED:
        from src.services.search.sparse_embedder import SparseEmbedder
        _sparse_embedder = SparseEmbedder.get_instance()
    return _sparse_embedder


class Retriever:
    """
    Retriever service supporting both dense and hybrid search.
    """
    
    @staticmethod
    def search(
        query: str, 
        limit: int = 5, 
        org_id: str = "public",
        hybrid: bool = None,
        rerank: bool = None,
        analyze_query: bool = None
    ) -> List[Dict]:
        """
        Search using dense or hybrid mode with optional reranking and query analysis.
        
        Args:
            query: Search query
            limit: Max results
            org_id: Organization ID for multi-tenancy filtering
            hybrid: Enable hybrid search (default: from settings)
            rerank: Enable reranking (default: from settings)
            analyze_query: Enable query intent analysis (default: from settings)
            
        Returns:
            List of search results with score, text, and metadata
        """
        # Query analysis for intent and scope extraction
        use_analysis = analyze_query if analyze_query is not None else settings.QUERY_ANALYSIS_ENABLED
        language_filter = None
        processed_query = query
        
        if use_analysis:
            from src.services.search.query_analyzer import analyze_query as do_analyze
            analysis = do_analyze(query)
            processed_query = analysis.processed_query
            if analysis.language_hints:
                language_filter = analysis.language_hints[0]
        
        # Determine if hybrid search should be used
        use_hybrid = hybrid if hybrid is not None else settings.SPARSE_ENABLED
        use_rerank = rerank if rerank is not None else settings.RERANK_ENABLED
        
        # If reranking, over-fetch for better results
        fetch_limit = limit * 3 if use_rerank else limit
        
        if use_hybrid:
            results = Retriever.hybrid_search(processed_query, fetch_limit, org_id, language_filter)
        else:
            results = Retriever.dense_search(processed_query, fetch_limit, org_id, language_filter)
        
        # Apply reranking if enabled
        if use_rerank and results:
            from src.services.search.reranker import rerank_results
            results = rerank_results(query, results, top_k=limit)
        
        # Ensure we return only the requested limit
        return results[:limit]
    
    @staticmethod
    def dense_search(query: str, limit: int = 5, org_id: str = "public", language: str = None) -> List[Dict]:
        """
        Dense-only semantic search with optional language filter.
        """
        # 1. Encode query
        model = get_dense_model()
        vector = model.encode(query).tolist()

        # 2. Build filter
        filter_conditions = [
            FieldCondition(
                key="org_id",
                match=MatchValue(value=org_id)
            )
        ]
        
        if language:
            filter_conditions.append(
                FieldCondition(
                    key="language",
                    match=MatchValue(value=language)
                )
            )
        
        query_filter = Filter(must=filter_conditions)
        
        # 3. Search
        try:
            results = qdrant.search(
                collection_name=COLLECTION_NAME,
                query_vector=("default", vector),
                query_filter=query_filter,
                limit=limit
            )
        except Exception:
            return []

        # 4. Format results
        return Retriever._format_results(results)

    
    @staticmethod
    def hybrid_search(
        query: str, 
        limit: int = 5, 
        org_id: str = "public",
        language_filter: str = None,
        rrf_k: int = None
    ) -> List[Dict]:
        """
        Hybrid search combining dense and sparse retrieval with RRF fusion.
        
        Uses Qdrant's Query API with prefetch for multi-vector search.
        """
        rrf_k = rrf_k or settings.RRF_K
        
        # 1. Generate dense embedding
        model = get_dense_model()
        dense_vector = model.encode(query).tolist()
        
        # 2. Generate sparse embedding
        sparse_embedder = get_sparse_embedder()
        if not sparse_embedder:
            # Fallback to dense-only
            return Retriever.dense_search(query, limit, org_id, language_filter)
        
        sparse_result = sparse_embedder.embed(query)
        
        # 3. Build filter with optional language
        filter_conditions = [
            FieldCondition(
                key="org_id",
                match=MatchValue(value=org_id)
            )
        ]
        
        if language_filter:
            filter_conditions.append(
                FieldCondition(
                    key="language",
                    match=MatchValue(value=language_filter)
                )
            )
        
        query_filter = Filter(must=filter_conditions)
        
        # 4. Execute hybrid search with RRF fusion
        try:
            # Qdrant Query API with prefetch for hybrid search
            results = qdrant.query_points(
                collection_name=COLLECTION_NAME,
                prefetch=[
                    # Dense prefetch
                    Prefetch(
                        query=dense_vector,
                        using="default",
                        limit=limit * 4,  # Over-fetch for better fusion
                        filter=query_filter
                    ),
                    # Sparse prefetch
                    Prefetch(
                        query=SparseVector(
                            indices=sparse_result.indices,
                            values=sparse_result.values
                        ),
                        using="sparse",
                        limit=limit * 4,
                        filter=query_filter
                    )
                ],
                query=FusionQuery(fusion=Fusion.RRF),
                limit=limit
            )
        except Exception as e:
            # Fallback to dense search on error
            print(f"Hybrid search error: {e}, falling back to dense")
            return Retriever.dense_search(query, limit, org_id)

        # 5. Format results
        return Retriever._format_results(results.points)
    
    @staticmethod
    def _format_results(results) -> List[Dict]:
        """Format Qdrant results to standard output."""
        output = []
        for hit in results:
            payload = hit.payload or {}
            output.append({
                "score": getattr(hit, "score", 0.0),
                "text": payload.get("text", ""),
                "metadata": {k: v for k, v in payload.items() if k != "text"}
            })
        return output
