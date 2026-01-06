from fastapi import APIRouter, HTTPException, Body
from typing import List, Dict, Optional
from pydantic import BaseModel
from datetime import datetime

from src.services.admin.admin_store import get_admin_store
from src.db.qdrant import get_qdrant_client
from qdrant_client.models import Filter, FieldCondition, MatchValue

router = APIRouter()

class Store(BaseModel):
    id: str
    name: str
    type: str = "production"  # production, staging, dev
    description: Optional[str] = None
    created_at: Optional[str] = None
    doc_count: Optional[int] = 0

class StoreCreate(BaseModel):
    id: str
    name: str
    type: str = "production"
    description: Optional[str] = None

@router.get("/", response_model=List[Store])
async def list_stores():
    """
    List all configured stores.
    """
    admin_store = get_admin_store()
    stores_data = admin_store.get_stores()
    
    results = []
    # Optionally enrich with Qdrant stats (can be slow if many stores)
    # For now, just return metadata. 
    # Detailed stats can be fetched via /stores/{id}
    for sid, data in stores_data.items():
        results.append(Store(**data))
        
    return results

@router.post("/", response_model=Store)
async def create_store(store: StoreCreate):
    """
    Create a new store (logical index).
    """
    admin_store = get_admin_store()
    stores = admin_store.get_stores()
    
    if store.id in stores:
        raise HTTPException(status_code=400, detail="Store ID already exists")
    
    new_store = store.dict()
    new_store["created_at"] = datetime.now().isoformat()
    
    if admin_store.set_store(store.id, new_store):
        return Store(**new_store)
    else:
        raise HTTPException(status_code=500, detail="Failed to create store")

@router.get("/{store_id}", response_model=Store)
async def get_store(store_id: str):
    """
    Get store details including document count.
    """
    admin_store = get_admin_store()
    stores = admin_store.get_stores()
    
    if store_id not in stores:
        raise HTTPException(status_code=404, detail="Store not found")
    
    store_data = stores[store_id]
    
    # Fetch real stats from Qdrant
    try:
        qdrant = get_qdrant_client()
        # Count points with this org_id
        count_filter = Filter(
            must=[
                FieldCondition(
                    key="org_id",
                    match=MatchValue(value=store_id)
                )
            ]
        )
        count_res = qdrant.count(
            collection_name="rice_chunks",
            count_filter=count_filter,
            exact=False # Approximate count is faster
        )
        store_data["doc_count"] = count_res.count
    except Exception:
        store_data["doc_count"] = -1
        
    return Store(**store_data)

@router.delete("/{store_id}")
async def delete_store(store_id: str):
    """
    Delete a store configuration. 
    Note: Does NOT delete indexed data for safety.
    """
    admin_store = get_admin_store()
    if admin_store.delete_store(store_id):
        return {"status": "success", "message": f"Store {store_id} deleted"}
    else:
        raise HTTPException(status_code=404, detail="Store not found")
