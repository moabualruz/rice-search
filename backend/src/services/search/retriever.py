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

# Load dense model
model = SentenceTransformer(settings.EMBEDDING_MODEL)
qdrant = get_qdrant_client()
COLLECTION_NAME = "rice_chunks"

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
        hybrid: bool = None
    ) -> List[Dict]:
        """
        Search using dense or hybrid mode.
        
        Args:
            query: Search query
            limit: Max results
            org_id: Organization ID for multi-tenancy filtering
            hybrid: Enable hybrid search (default: from settings)
            
        Returns:
            List of search results with score, text, and metadata
        """
        # Determine if hybrid search should be used
        use_hybrid = hybrid if hybrid is not None else settings.SPARSE_ENABLED
        
        if use_hybrid:
            return Retriever.hybrid_search(query, limit, org_id)
        else:
            return Retriever.dense_search(query, limit, org_id)
    
    @staticmethod
    def dense_search(query: str, limit: int = 5, org_id: str = "public") -> List[Dict]:
        """
        Dense-only semantic search.
        """
        # 1. Encode query
        vector = model.encode(query).tolist()

        # 2. Build filter
        query_filter = Filter(
            must=[
                FieldCondition(
                    key="org_id",
                    match=MatchValue(value=org_id)
                )
            ]
        )
        
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
        rrf_k: int = None
    ) -> List[Dict]:
        """
        Hybrid search combining dense and sparse retrieval with RRF fusion.
        
        Uses Qdrant's Query API with prefetch for multi-vector search.
        """
        rrf_k = rrf_k or settings.RRF_K
        
        # 1. Generate dense embedding
        dense_vector = model.encode(query).tolist()
        
        # 2. Generate sparse embedding
        sparse_embedder = get_sparse_embedder()
        if not sparse_embedder:
            # Fallback to dense-only
            return Retriever.dense_search(query, limit, org_id)
        
        sparse_result = sparse_embedder.embed(query)
        
        # 3. Build filter
        query_filter = Filter(
            must=[
                FieldCondition(
                    key="org_id",
                    match=MatchValue(value=org_id)
                )
            ]
        )
        
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
