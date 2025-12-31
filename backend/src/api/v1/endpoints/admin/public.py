"""
Public Admin endpoints for development.

These endpoints do NOT require authentication and are for development/testing only.
In production, use the authenticated /api/v1/admin/* endpoints.
"""

from fastapi import APIRouter, HTTPException
from pydantic import BaseModel
from typing import Optional, List, Dict, Any
import json
from pathlib import Path

from src.core.config import settings

router = APIRouter()

# In-memory model registry (would be Redis/DB in production)
_models_registry: Dict[str, dict] = {
    "dense": {
        "id": "dense",
        "name": settings.EMBEDDING_MODEL,
        "type": "embedding",
        "active": True,
        "gpu_enabled": False
    },
    "sparse": {
        "id": "sparse",
        "name": settings.SPARSE_MODEL,
        "type": "sparse_embedding",
        "active": settings.SPARSE_ENABLED,
        "gpu_enabled": True
    }
}

# In-memory config overrides
_config_overrides: Dict[str, Any] = {}


class ModelUpdate(BaseModel):
    """Model update request."""
    active: Optional[bool] = None
    gpu_enabled: Optional[bool] = None


class ConfigUpdate(BaseModel):
    """Configuration update request."""
    sparse_enabled: Optional[bool] = None
    rrf_k: Optional[int] = None
    ast_parsing_enabled: Optional[bool] = None
    mcp_enabled: Optional[bool] = None


# ============== Models Endpoints ==============

@router.get("/models")
async def list_models():
    """List all models (no auth required for development)."""
    return {"models": list(_models_registry.values())}


@router.get("/models/{model_id}")
async def get_model(model_id: str):
    """Get a specific model."""
    if model_id not in _models_registry:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    return _models_registry[model_id]


@router.put("/models/{model_id}")
async def update_model(model_id: str, update: ModelUpdate):
    """Update a model's settings."""
    if model_id not in _models_registry:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    
    model = _models_registry[model_id]
    
    if update.active is not None:
        model["active"] = update.active
    if update.gpu_enabled is not None:
        model["gpu_enabled"] = update.gpu_enabled
    
    return {
        "message": f"Model {model_id} updated",
        "model": model,
        "restart_required": True
    }


@router.post("/models")
async def add_model(model: dict):
    """Add a new model."""
    model_id = model.get("id") or model.get("name", "").replace("/", "-").lower()
    
    if model_id in _models_registry:
        raise HTTPException(status_code=400, detail=f"Model {model_id} already exists")
    
    new_model = {
        "id": model_id,
        "name": model.get("name", model_id),
        "type": model.get("type", "embedding"),
        "active": model.get("active", False),
        "gpu_enabled": model.get("gpu_enabled", False)
    }
    
    _models_registry[model_id] = new_model
    return {"message": f"Model {model_id} added", "model": new_model}


@router.delete("/models/{model_id}")
async def delete_model(model_id: str):
    """Delete a model."""
    if model_id not in _models_registry:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    
    # Don't allow deleting core models
    if model_id in ["dense", "sparse"]:
        raise HTTPException(status_code=400, detail="Cannot delete core models")
    
    del _models_registry[model_id]
    return {"message": f"Model {model_id} deleted"}


# ============== Config Endpoints ==============

@router.get("/config")
async def get_config():
    """Get current configuration (no auth required for development)."""
    return {
        "sparse_enabled": _config_overrides.get("sparse_enabled", settings.SPARSE_ENABLED),
        "sparse_model": settings.SPARSE_MODEL,
        "embedding_model": settings.EMBEDDING_MODEL,
        "rrf_k": _config_overrides.get("rrf_k", settings.RRF_K),
        "ast_parsing_enabled": _config_overrides.get("ast_parsing_enabled", settings.AST_PARSING_ENABLED),
        "mcp_enabled": _config_overrides.get("mcp_enabled", settings.MCP_ENABLED),
        "mcp_transport": settings.MCP_TRANSPORT,
        "mcp_tcp_port": settings.MCP_TCP_PORT,
        "qdrant_url": settings.QDRANT_URL,
        "redis_url": settings.REDIS_URL
    }


@router.put("/config")
async def update_config(update: ConfigUpdate):
    """Update configuration (no auth required for development)."""
    updated = []
    
    if update.sparse_enabled is not None:
        _config_overrides["sparse_enabled"] = update.sparse_enabled
        _models_registry["sparse"]["active"] = update.sparse_enabled
        updated.append(f"sparse_enabled={update.sparse_enabled}")
    
    if update.rrf_k is not None:
        if update.rrf_k < 1 or update.rrf_k > 1000:
            raise HTTPException(status_code=400, detail="rrf_k must be between 1 and 1000")
        _config_overrides["rrf_k"] = update.rrf_k
        updated.append(f"rrf_k={update.rrf_k}")
    
    if update.ast_parsing_enabled is not None:
        _config_overrides["ast_parsing_enabled"] = update.ast_parsing_enabled
        updated.append(f"ast_parsing_enabled={update.ast_parsing_enabled}")
    
    if update.mcp_enabled is not None:
        _config_overrides["mcp_enabled"] = update.mcp_enabled
        updated.append(f"mcp_enabled={update.mcp_enabled}")
    
    if not updated:
        return {"message": "No changes requested"}
    
    return {
        "message": "Configuration updated",
        "updated": updated,
        "restart_required": True
    }


# ============== MCP Endpoints ==============

@router.get("/mcp/status")
async def get_mcp_status():
    """Get MCP server status."""
    return {
        "enabled": _config_overrides.get("mcp_enabled", settings.MCP_ENABLED),
        "transport": settings.MCP_TRANSPORT,
        "tcp_host": settings.MCP_TCP_HOST,
        "tcp_port": settings.MCP_TCP_PORT,
        "tools": ["search", "read_file", "list_files"]
    }


@router.put("/mcp/toggle")
async def toggle_mcp():
    """Toggle MCP server on/off."""
    current = _config_overrides.get("mcp_enabled", settings.MCP_ENABLED)
    _config_overrides["mcp_enabled"] = not current
    return {
        "message": f"MCP {'disabled' if current else 'enabled'}",
        "enabled": not current,
        "restart_required": True
    }


# ============== System Endpoints ==============

@router.get("/system/status")
async def get_system_status():
    """Get system status overview."""
    return {
        "status": "healthy",
        "features": {
            "hybrid_search": _config_overrides.get("sparse_enabled", settings.SPARSE_ENABLED),
            "ast_parsing": _config_overrides.get("ast_parsing_enabled", settings.AST_PARSING_ENABLED),
            "mcp_protocol": _config_overrides.get("mcp_enabled", settings.MCP_ENABLED),
            "opentelemetry": False  # Disabled due to stability issues
        },
        "models": {
            "dense": _models_registry["dense"]["active"],
            "sparse": _models_registry["sparse"]["active"]
        }
    }


@router.post("/system/rebuild-index")
async def rebuild_index():
    """Trigger index rebuild (placeholder)."""
    return {
        "message": "Index rebuild triggered",
        "status": "pending",
        "note": "Full implementation requires Celery task"
    }


@router.post("/system/clear-cache")
async def clear_cache():
    """Clear Redis cache (placeholder)."""
    return {
        "message": "Cache clear triggered",
        "status": "pending",
        "note": "Full implementation requires Redis connection"
    }
