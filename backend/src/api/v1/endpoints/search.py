from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional, Literal
from src.services.search.retriever import Retriever
from src.services.rag.engine import RAGEngine
from src.api.v1.dependencies import get_current_user
from src.core.config import settings

router = APIRouter()

class SearchRequest(BaseModel):
    query: str
    mode: Literal["search", "rag"] = "rag"
    hybrid: Optional[bool] = None  # None = use settings default

@router.post("/query")
async def search_endpoint(
    request: SearchRequest,
    user: dict = Depends(get_current_user)
):
    """
    Search or RAG query endpoint.
    
    Args:
        query: Search query
        mode: "search" for retrieval only, "rag" for full Q&A
        hybrid: Enable hybrid search (overrides settings if provided)
    """
    try:
        # Extract org_id (Phase 7)
        org_id = user.get("org_id", "public")

        if request.mode == "search":
            results = Retriever.search(
                request.query, 
                org_id=org_id,
                hybrid=request.hybrid
            )
            return {
                "mode": "search", 
                "results": results,
                "hybrid_enabled": request.hybrid if request.hybrid is not None else settings.SPARSE_ENABLED
            }
        
        elif request.mode == "rag":
            # Initialize engine (could be singleton in prod)
            engine = RAGEngine()
            response = engine.ask(request.query, org_id=org_id)
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
        "rrf_k": settings.RRF_K
    }
