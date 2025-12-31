"""
Admin configuration endpoints.

Provides runtime configuration management for Phase 9+ features.
"""

from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional

from src.api.v1.dependencies import verify_admin
from src.core.config import settings

router = APIRouter()


class ConfigUpdate(BaseModel):
    """Configuration update request."""
    sparse_enabled: Optional[bool] = None
    rrf_k: Optional[int] = None


class ConfigResponse(BaseModel):
    """Configuration response."""
    sparse_enabled: bool
    sparse_model: str
    embedding_model: str
    rrf_k: int
    qdrant_url: str
    redis_url: str


@router.get("/config", response_model=ConfigResponse)
async def get_config(admin: dict = Depends(verify_admin)):
    """
    Get current runtime configuration.
    
    Requires admin role.
    """
    return ConfigResponse(
        sparse_enabled=settings.SPARSE_ENABLED,
        sparse_model=settings.SPARSE_MODEL,
        embedding_model=settings.EMBEDDING_MODEL,
        rrf_k=settings.RRF_K,
        qdrant_url=settings.QDRANT_URL,
        redis_url=settings.REDIS_URL
    )


@router.put("/config")
async def update_config(
    update: ConfigUpdate,
    admin: dict = Depends(verify_admin)
):
    """
    Update runtime configuration.
    
    Note: Some settings require restart to take effect.
    Requires admin role.
    """
    updated_fields = []
    
    # Update sparse_enabled
    if update.sparse_enabled is not None:
        # In a real implementation, this would persist to env/db
        # For now, we just validate and acknowledge
        updated_fields.append(f"sparse_enabled: {update.sparse_enabled}")
    
    # Update rrf_k
    if update.rrf_k is not None:
        if update.rrf_k < 1 or update.rrf_k > 1000:
            raise HTTPException(
                status_code=400, 
                detail="rrf_k must be between 1 and 1000"
            )
        updated_fields.append(f"rrf_k: {update.rrf_k}")
    
    if not updated_fields:
        return {"message": "No changes requested"}
    
    # Note: In production, persist to .env or database
    return {
        "message": "Configuration update acknowledged (restart required for effect)",
        "updated": updated_fields,
        "restart_required": True
    }


@router.get("/models")
async def list_models(admin: dict = Depends(verify_admin)):
    """
    List active models.
    
    Requires admin role.
    """
    return {
        "models": [
            {
                "id": "dense",
                "name": settings.EMBEDDING_MODEL,
                "type": "embedding",
                "active": True,
                "gpu_enabled": False  # sentence-transformers default
            },
            {
                "id": "sparse", 
                "name": settings.SPARSE_MODEL,
                "type": "sparse_embedding",
                "active": settings.SPARSE_ENABLED,
                "gpu_enabled": True  # SPLADE uses GPU when available
            }
        ]
    }
