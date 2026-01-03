"""
Retriever service - Xinference backend.

Supports dense-only and hybrid (dense + sparse) search using Qdrant.
All model inference via Xinference unified API.
"""

from typing import List, Dict, Optional
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

COLLECTION_NAME = "rice_chunks"


def embed_texts(texts: List[str]) -> List[List[float]]:
    """
    Embed texts using Xinference.
    
    Args:
        texts: List of texts to embed
        
    Returns:
        List of embedding vectors
        
    Raises:
        RuntimeError: If Xinference is unavailable
    """
    from src.services.inference import get_xinference_client
    
    client = get_xinference_client()
    return client.embed(texts)


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
        Search using dense or hybrid mode with optional reranking.
        
        Args:
            query: Search query
            limit: Number of results
            org_id: Organization filter
            hybrid: Enable hybrid search (dense + sparse)
            rerank: Enable reranking
            analyze_query: Enable query analysis
            
        Returns:
            List of search results with metadata
        """
        import logging
        logger = logging.getLogger(__name__)
        
        # Defaults from settings
        if hybrid is None:
            hybrid = settings.SPARSE_ENABLED
        if rerank is None:
            rerank = settings.RERANK_ENABLED
        if analyze_query is None:
            analyze_query = settings.QUERY_ANALYSIS_ENABLED
        
        qdrant = get_qdrant_client()
        
        # Get query embedding
        query_vector = embed_texts([query])[0]
        
        # Build filter
        search_filter = None
        if org_id and org_id != "public":
            search_filter = Filter(
                must=[FieldCondition(key="org_id", match=MatchValue(value=org_id))]
            )
        
        # Search
        if hybrid and settings.SPARSE_ENABLED:
            # Hybrid search with RRF fusion
            try:
                from src.services.search.sparse import sparse_embed
                sparse_result = sparse_embed(query)
                
                results = qdrant.query_points(
                    collection_name=COLLECTION_NAME,
                    prefetch=[
                        Prefetch(
                            query=query_vector,
                            using="default",
                            limit=limit * 2,
                            filter=search_filter
                        ),
                        Prefetch(
                            query=SparseVector(
                                indices=sparse_result.indices,
                                values=sparse_result.values
                            ),
                            using="sparse",
                            limit=limit * 2,
                            filter=search_filter
                        )
                    ],
                    query=FusionQuery(fusion=Fusion.RRF),
                    limit=limit,
                )
            except Exception as e:
                logger.warning(f"Hybrid search failed, falling back to dense: {e}")
                results = qdrant.query_points(
                    collection_name=COLLECTION_NAME,
                    query=query_vector,
                    using="default",
                    limit=limit,
                    query_filter=search_filter,
                )
        else:
            # Dense-only search
            results = qdrant.query_points(
                collection_name=COLLECTION_NAME,
                query=query_vector,
                using="default",
                limit=limit,
                query_filter=search_filter,
            )
        
        # Convert to list of dicts
        output = []
        for point in results.points:
            item = {
                "id": str(point.id),
                "score": point.score,
                **point.payload
            }
            output.append(item)
        
        # Rerank if enabled
        if rerank and output:
            from src.services.search.reranker import rerank_search_results
            output = rerank_search_results(query, output)
        
        return output
