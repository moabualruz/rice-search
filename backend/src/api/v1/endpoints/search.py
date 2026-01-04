from fastapi import APIRouter, HTTPException, Depends, Query
from pydantic import BaseModel
from typing import Optional, Literal, List
from src.services.search.retriever import Retriever
from src.services.rag.engine import RAGEngine
from src.api.v1.dependencies import get_current_user
from src.core.config import settings

router = APIRouter()


class SearchRequest(BaseModel):
    query: str
    mode: Literal["search", "rag"] = "rag"
    limit: int = 10
    # Triple retrieval flags - all default to True
    use_bm25: bool = True
    use_splade: bool = True
    use_bm42: bool = True
    # Legacy
    hybrid: Optional[bool] = None


@router.post("/query")
async def search_post(
    request: SearchRequest,
    user: dict = Depends(get_current_user)
):
    """
    Search or RAG query endpoint (POST).
    
    Args:
        query: Search query
        mode: "search" for retrieval only, "rag" for full Q&A
        limit: Maximum number of results
        use_bm25: Enable BM25 retrieval (default: true)
        use_splade: Enable SPLADE retrieval (default: true)
        use_bm42: Enable BM42 retrieval (default: true)
    """
    return await _perform_search(
        query=request.query,
        mode=request.mode,
        limit=request.limit,
        use_bm25=request.use_bm25,
        use_splade=request.use_splade,
        use_bm42=request.use_bm42,
        hybrid=request.hybrid,
        user=user
    )


@router.get("/query")
async def search_get(
    query: str = Query(..., description="Search query"),
    mode: Literal["search", "rag"] = Query("rag", description="search or rag"),
    limit: int = Query(10, description="Maximum results"),
    use_bm25: bool = Query(True, description="Enable BM25 retrieval"),
    use_splade: bool = Query(True, description="Enable SPLADE retrieval"),
    use_bm42: bool = Query(True, description="Enable BM42 retrieval"),
    user: dict = Depends(get_current_user)
):
    """
    Search or RAG query endpoint (GET).
    
    All retrieval methods are enabled by default. Pass false to disable.
    
    Examples:
        /query?query=test - Uses all retrievers
        /query?query=test&use_bm25=false - Excludes BM25
        /query?query=test&use_splade=false&use_bm42=false - BM25 only
    """
    return await _perform_search(
        query=query,
        mode=mode,
        limit=limit,
        use_bm25=use_bm25,
        use_splade=use_splade,
        use_bm42=use_bm42,
        hybrid=None,
        user=user
    )


async def _perform_search(
    query: str,
    mode: str,
    limit: int,
    use_bm25: bool,
    use_splade: bool,
    use_bm42: bool,
    hybrid: Optional[bool],
    user: dict
):
    """Shared search logic for GET and POST."""
    try:
        org_id = user.get("org_id", "public")

        if mode == "search":
            results = await Retriever.search(
                query=query,
                limit=limit,
                org_id=org_id,
                use_bm25=use_bm25,
                use_splade=use_splade,
                use_bm42=use_bm42,
                hybrid=hybrid
            )
            return {
                "mode": "search",
                "results": results,
                "retrievers": {
                    "bm25": use_bm25,
                    "splade": use_splade,
                    "bm42": use_bm42
                }
            }
        
        elif mode == "rag":
            engine = RAGEngine()
            response = await engine.ask(query, org_id=org_id)
            return {"mode": "rag", **response}
            
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/config")
async def get_search_config(user: dict = Depends(get_current_user)):
    """Get current search configuration."""
    return {
        "sparse_enabled": settings.SPARSE_ENABLED,
        "sparse_model": settings.SPARSE_MODEL,
        "embedding_model": settings.EMBEDDING_MODEL,
        "rrf_k": settings.RRF_K,
        "retrievers": {
            "bm25_enabled": settings.BM25_ENABLED,
            "splade_enabled": settings.SPLADE_ENABLED,
            "bm42_enabled": settings.BM42_ENABLED
        }
    }
