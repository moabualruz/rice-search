"""
Multi-Retriever Service.

Supports THREE retrieval methods simultaneously:
1. BM25 using Rust Tantivy
2. SPLADE (neural sparse retrieval)
3. BM42 (Qdrant-native hybrid sparse + dense retrieval)

All methods can be enabled/disabled at runtime via flags.
Results are fused using Reciprocal Rank Fusion (RRF).
"""

import logging
from typing import List, Dict, Optional, Any

from qdrant_client.models import (
    Filter,
    FieldCondition,
    MatchValue,
    Prefetch,
    FusionQuery,
    Fusion,
    SparseVector,
)

from src.db.qdrant import get_qdrant_client
from src.core.config import settings
from src.services.retrieval.fusion import rrf_fusion, FusedResult

logger = logging.getLogger(__name__)

COLLECTION_NAME = "rice_chunks"


def embed_texts(texts: List[str]) -> List[List[float]]:
    """
    Embed texts using BentoML dense embeddings.
    
    Args:
        texts: List of texts to embed
        
    Returns:
        List of embedding vectors
    """
    from src.services.inference import get_bentoml_client
    
    client = get_bentoml_client()
    return client.embed(texts)


class MultiRetriever:
    """
    Multi-retriever service supporting BM25, SPLADE, and BM42.
    
    Features:
    - Runtime selection of active retrievers
    - All three methods coexist
    - RRF fusion for result merging
    - Optional reranking
    """
    
    def __init__(self):
        self._splade_encoder = None
        self._bm42_encoder = None
        self._tantivy_client = None
    
    @property
    def splade_encoder(self):
        """Lazy-load SPLADE encoder."""
        if self._splade_encoder is None:
            from src.services.retrieval.splade_encoder import get_splade_encoder
            self._splade_encoder = get_splade_encoder()
        return self._splade_encoder
    
    @property
    def bm42_encoder(self):
        """Lazy-load BM42 encoder."""
        if self._bm42_encoder is None:
            from src.services.retrieval.bm42_encoder import get_bm42_encoder
            self._bm42_encoder = get_bm42_encoder()
        return self._bm42_encoder
    
    @property
    def tantivy_client(self):
        """Lazy-load Tantivy client."""
        if self._tantivy_client is None:
            from src.services.retrieval.tantivy_client import get_tantivy_client
            self._tantivy_client = get_tantivy_client()
        return self._tantivy_client
    
    def search(
        self,
        query: str,
        limit: int = 10,
        org_id: str = "public",
        use_bm25: bool = True,
        use_splade: bool = True,
        use_bm42: bool = True,
        rerank: bool = None,
        rrf_k: int = 60,
    ) -> List[Dict[str, Any]]:
        """
        Search using selected retrievers with RRF fusion.
        
        Args:
            query: Search query
            limit: Maximum number of results
            org_id: Organization filter
            use_bm25: Enable BM25 (Tantivy)
            use_splade: Enable SPLADE
            use_bm42: Enable BM42 hybrid
            rerank: Enable reranking (default from settings)
            rrf_k: RRF parameter
            
        Returns:
            List of search results with metadata
        """
        if rerank is None:
            rerank = settings.RERANK_ENABLED
        
        qdrant = get_qdrant_client()
        result_sets: Dict[str, List[Dict]] = {}
        
        # Build organization filter
        search_filter = None
        if org_id and org_id != "public":
            search_filter = Filter(
                must=[FieldCondition(key="org_id", match=MatchValue(value=org_id))]
            )
        
        # 1. BM25 Search (Tantivy)
        if use_bm25:
            try:
                bm25_results = self._search_bm25(query, limit * 2)
                if bm25_results:
                    result_sets["bm25"] = bm25_results
                    logger.debug(f"BM25 returned {len(bm25_results)} results")
            except Exception as e:
                logger.warning(f"BM25 search failed: {e}")
        
        # 2. SPLADE Search (Qdrant sparse)
        if use_splade:
            try:
                splade_results = self._search_splade(
                    query, qdrant, limit * 2, search_filter
                )
                if splade_results:
                    result_sets["splade"] = splade_results
                    logger.debug(f"SPLADE returned {len(splade_results)} results")
            except Exception as e:
                logger.warning(f"SPLADE search failed: {e}")
        
        # 3. BM42 Hybrid Search (Qdrant dense + sparse)
        if use_bm42:
            try:
                bm42_results = self._search_bm42(
                    query, qdrant, limit * 2, search_filter
                )
                if bm42_results:
                    result_sets["bm42"] = bm42_results
                    logger.debug(f"BM42 returned {len(bm42_results)} results")
            except Exception as e:
                logger.warning(f"BM42 search failed: {e}")
        
        # 4. Fusion
        if not result_sets:
            logger.warning("All retrievers failed or returned no results")
            return []
        
        fused_results = rrf_fusion(result_sets, limit=limit, k=rrf_k)
        
        # Convert to output format
        output = self._format_results(fused_results)
        
        # 5. Reranking
        if rerank and output:
            from src.services.search.reranker import rerank_search_results
            output = rerank_search_results(query, output)
        
        return output
    
    def _search_bm25(self, query: str, limit: int) -> List[Dict]:
        """Search using BM25 via Tantivy, then fetch payloads from Qdrant."""
        bm25_results = self.tantivy_client.search(query, limit)
        
        if not bm25_results:
            return []
        
        # Get chunk_ids
        chunk_ids = [r.chunk_id for r in bm25_results]
        scores = {r.chunk_id: r.score for r in bm25_results}
        
        # Fetch full data from Qdrant
        qdrant = get_qdrant_client()
        points = qdrant.retrieve(
            collection_name=COLLECTION_NAME,
            ids=chunk_ids,
            with_payload=True
        )
        
        results = []
        for point in points:
            chunk_id = str(point.id)
            results.append({
                "chunk_id": chunk_id,
                "score": scores.get(chunk_id, 0.0),
                "text": point.payload.get("text", ""),
                **point.payload
            })
        
        # Sort by BM25 score
        results.sort(key=lambda x: x["score"], reverse=True)
        return results
    
    def _search_splade(
        self,
        query: str,
        qdrant,
        limit: int,
        search_filter: Optional[Filter]
    ) -> List[Dict]:
        """Search using SPLADE sparse vectors."""
        # Encode query
        sparse_vec = self.splade_encoder.encode_single(query)
        
        # Search Qdrant
        results = qdrant.query_points(
            collection_name=COLLECTION_NAME,
            query=SparseVector(
                indices=sparse_vec.indices,
                values=sparse_vec.values
            ),
            using="splade",
            limit=limit,
            query_filter=search_filter,
            with_payload=True
        )
        
        return [
            {
                "chunk_id": str(point.id),
                "score": point.score,
                "text": point.payload.get("text", ""),
                **point.payload
            }
            for point in results.points
        ]
    
    def _search_bm42(
        self,
        query: str,
        qdrant,
        limit: int,
        search_filter: Optional[Filter]
    ) -> List[Dict]:
        """Search using BM42 hybrid (dense + sparse)."""
        # Generate query representations
        dense_vec = embed_texts([query])[0]
        bm42_sparse = self.bm42_encoder.encode_single(query)
        
        # Hybrid search with RRF fusion (Qdrant handles BM42 scoring)
        results = qdrant.query_points(
            collection_name=COLLECTION_NAME,
            prefetch=[
                Prefetch(
                    query=dense_vec,
                    using="dense",
                    limit=limit,
                    filter=search_filter
                ),
                Prefetch(
                    query=SparseVector(
                        indices=bm42_sparse.indices,
                        values=bm42_sparse.values
                    ),
                    using="bm42",
                    limit=limit,
                    filter=search_filter
                )
            ],
            query=FusionQuery(fusion=Fusion.RRF),
            limit=limit,
            with_payload=True
        )
        
        return [
            {
                "chunk_id": str(point.id),
                "score": point.score,
                "text": point.payload.get("text", ""),
                **point.payload
            }
            for point in results.points
        ]
    
    def _format_results(self, fused_results: List[FusedResult]) -> List[Dict]:
        """Convert FusedResult objects to output dicts."""
        return [
            {
                "id": r.chunk_id,
                "chunk_id": r.chunk_id,
                "score": r.fused_score,
                "text": r.text,
                "retriever_scores": r.sources,
                **r.payload
            }
            for r in fused_results
        ]


# Legacy compatibility - wraps MultiRetriever in Retriever interface
class Retriever:
    """
    Retriever service supporting both dense and hybrid search.
    Backward compatible with old interface.
    """
    
    _multi_retriever: Optional[MultiRetriever] = None
    
    @classmethod
    def get_multi_retriever(cls) -> MultiRetriever:
        """Get or create MultiRetriever instance."""
        if cls._multi_retriever is None:
            cls._multi_retriever = MultiRetriever()
        return cls._multi_retriever
    
    @staticmethod
    def search(
        query: str, 
        limit: int = 5, 
        org_id: str = "public",
        hybrid: bool = None,
        rerank: bool = None,
        analyze_query: bool = None,
        # New multi-retriever flags
        use_bm25: bool = True,
        use_splade: bool = True,
        use_bm42: bool = True,
    ) -> List[Dict]:
        """
        Search using dense or hybrid mode with optional reranking.
        
        Args:
            query: Search query
            limit: Number of results
            org_id: Organization filter
            hybrid: Enable hybrid search (legacy, maps to SPLADE)
            rerank: Enable reranking
            analyze_query: Enable query analysis
            use_bm25: Enable BM25 retriever
            use_splade: Enable SPLADE retriever  
            use_bm42: Enable BM42 retriever
            
        Returns:
            List of search results with metadata
        """
        # Legacy hybrid flag maps to SPLADE
        if hybrid is not None:
            use_splade = hybrid
        
        retriever = Retriever.get_multi_retriever()
        return retriever.search(
            query=query,
            limit=limit,
            org_id=org_id,
            use_bm25=use_bm25,
            use_splade=use_splade,
            use_bm42=use_bm42,
            rerank=rerank,
        )
