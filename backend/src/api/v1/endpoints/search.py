from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional, Literal
from src.services.search.retriever import Retriever
from src.services.rag.engine import RAGEngine
from src.api.v1.dependencies import get_current_user

router = APIRouter()

class SearchRequest(BaseModel):
    query: str
    mode: Literal["search", "rag"] = "rag"

@router.post("/query")
async def search_endpoint(
    request: SearchRequest,
    user: dict = Depends(get_current_user)
):
    try:
        # Extract org_id (Phase 7)
        org_id = user.get("org_id", "public")

        if request.mode == "search":
            results = Retriever.search(request.query, org_id=org_id)
            return {"mode": "search", "results": results}
        
        elif request.mode == "rag":
            # Initialize engine (could be singleton in prod)
            engine = RAGEngine()
            response = engine.ask(request.query, org_id=org_id)
            return {"mode": "rag", **response}
            
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
