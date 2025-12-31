from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional, Literal
from src.services.search.retriever import Retriever
from src.services.rag.engine import RAGEngine

router = APIRouter()

class SearchRequest(BaseModel):
    query: str
    mode: Literal["search", "rag"] = "rag"

@router.post("/query")
async def search_endpoint(request: SearchRequest):
    try:
        if request.mode == "search":
            results = Retriever.search(request.query)
            return {"mode": "search", "results": results}
        
        elif request.mode == "rag":
            # Initialize engine (could be singleton in prod)
            engine = RAGEngine()
            response = engine.ask(request.query)
            return {"mode": "rag", **response}
            
    except Exception as e:
        raise HTTPException(status_code=500, detail=str(e))
