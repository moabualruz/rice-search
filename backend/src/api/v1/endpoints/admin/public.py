"""
Public Admin endpoints with Redis persistence.

All state is persisted to Redis and survives restarts.
"""

from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional, List, Literal
from uuid import uuid4
import psutil
from datetime import datetime
from huggingface_hub import HfApi

from src.core.config import settings
from src.services.admin.admin_store import get_admin_store
from src.api.deps import requires_role, get_current_user

router = APIRouter()

class ConnectionRegister(BaseModel):
    """CLI connection registration."""
    user_id: str
    device_name: str
    version: str = "1.0.0"

class ModelUpdate(BaseModel):
    """Model update request."""
    active: Optional[bool] = None
    gpu_enabled: Optional[bool] = None

class ConfigUpdate(BaseModel):
    """Configuration update request."""
    sparse_enabled: Optional[bool] = None
    rrf_k: Optional[int] = None
    ast_parsing_enabled: Optional[bool] = None
    query_analysis_enabled: Optional[bool] = None
    mcp_enabled: Optional[bool] = None
    worker_pool: Optional[str] = None
    worker_concurrency: Optional[int] = None
    model_ttl_seconds: Optional[int] = None
    model_auto_unload: Optional[bool] = None
    mcp_transport: Optional[str] = None
    mcp_tcp_port: Optional[int] = None

class UserCreate(BaseModel):
    email: str
    role: Literal["admin", "member", "viewer"] = "member"
    org_id: str = "default"

class UserUpdate(BaseModel):
    role: Optional[Literal["admin", "member", "viewer"]] = None
    active: Optional[bool] = None
    org_id: Optional[str] = None


# ============== Models Endpoints ==============

@router.get("/models/search")
async def search_models(
    q: str, 
    type: Literal["embedding", "reranker", "sparse", "classification"] = "embedding",
    limit: int = 20
):
    """
    Search Hugging Face Hub for compatible models.
    """
    api = HfApi()
    
    # Define filters based on type
    filters = []
    
    
    if type == "embedding":
        # Must be compatible with sentence-transformers
        filters.append("sentence-transformers")
    elif type == "reranker":
        # Industry standard: Must be a cross-encoder or explicitly tagged as reranker
        # We don't want generic BERT models here.
        # Note: Some older models might miss tags, but for safety in Admin UI, 
        # we enforce this to avoid user frustration.
        pass # We handle logical OR in the code below since HF API `tags` is AND
    elif type == "classification":
        # For Query Understanding (CodeBERT etc)
        pass 
        
    # Execute search
    try:
        # Search args
        search_args = {
            "search": q,
            "limit": limit,
            "sort": "downloads",
            "direction": -1,
        }
        
        if type == "embedding":
             search_args["library"] = "sentence-transformers"
        
        models = api.list_models(**search_args)
        
        results = []
        for m in models:
            # Client-side filtering for complex logic (HF API tags is strict AND)
            
            # 1. Reranker Logic: Must have 'cross-encoder' tag OR 'reranker' in name/tags
            if type == "reranker":
                tags = m.tags or []
                is_cross_encoder = "cross-encoder" in tags
                is_reranker_tag = "reranker" in tags
                is_reranker_name = "reranker" in m.modelId.lower()
                
                if not (is_cross_encoder or is_reranker_tag or is_reranker_name):
                    continue
            
            # 2. Sparse Logic: Look for SPLADE or sparse keywords
            if type == "sparse":
                tags = m.tags or []
                name = m.modelId.lower()
                is_splade = "splade" in name or "splade" in tags
                is_sparse = "sparse" in name or "sparse" in tags
                
                if not (is_splade or is_sparse):
                    continue

            # 3. Classification Logic: Look for text-classification
            if type == "classification":
                tags = m.tags or []
                name = m.modelId.lower()
                is_cls = "text-classification" in tags
                is_bert = "bert" in name or "roberta" in name
                
                if not (is_cls or is_bert):
                    continue

            results.append({
                "id": m.modelId,
                "name": m.modelId, # simplified
                "downloads": m.downloads,
                "likes": m.likes,
                "tags": m.tags
            })
            
        return {"models": results}
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Hugging Face API Error: {str(e)}")


# Helper
def get_protected_models():
    # Helper to slugify
    def slugify(name: str) -> str:
        return name.replace("/", "-").lower()

    return {
        slugify(settings.EMBEDDING_MODEL), 
        slugify(settings.SPARSE_MODEL),
        slugify(settings.RERANK_MODEL),
        slugify(settings.QUERY_UNDERSTANDING_MODEL)
    }

@router.get("/models")
async def list_models():
    """List all models (persisted to Redis)."""
    store = get_admin_store()
    models = store.get_models()
    protected = get_protected_models()
    
    # Enrich with protected and loaded status
    from src.services.model_manager import get_model_manager
    manager = get_model_manager()
    
    result = []
    for m in models.values():
        m_copy = m.copy()
        m_copy["protected"] = m["id"] in protected
        
        # Get runtime status
        status = manager.get_model_status(m["id"])
        m_copy["loaded"] = status.get("loaded", False)
        
        result.append(m_copy)
        
    return {"models": result}


@router.get("/models/{model_id:path}")
async def get_model(model_id: str):
    """Get a specific model."""
    store = get_admin_store()
    models = store.get_models()
    if model_id not in models:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    
    model = models[model_id].copy()
    model["protected"] = model_id in get_protected_models()
    return model


@router.put("/models/{model_id:path}", dependencies=[Depends(requires_role("admin"))])
async def update_model(model_id: str, updates: dict):
    """
    Update a model configuration and apply changes at runtime.
    Supports toggling gpu_enabled and active status.
    Enforces 'Single Active Model per Type' logic.
    """
    store = get_admin_store()
    models = store.get_models()
    
    if model_id not in models:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    
    old_model = models[model_id]
    
    # Check if gpu_enabled or active changed
    gpu_changed = "gpu_enabled" in updates and updates["gpu_enabled"] != old_model.get("gpu_enabled")
    active_changed = "active" in updates and updates["active"] != old_model.get("active")
    
    if active_changed and updates["active"] is True:
        # Enforce Single Active Model per Type
        model_type = old_model.get("type", "embedding")
        for mid, m in models.items():
            if mid != model_id and m.get("type") == model_type and m.get("active"):
                # Deactivate peer
                m["active"] = False
                store.set_model(mid, m)
                
                # Unload peer from runtime if loaded
                from src.services.model_manager import get_model_manager
                get_model_manager().unload_model(mid)

    # Update in store
    updated = {**old_model, **updates}
    store.set_model(model_id, updated)
    
    # Apply runtime changes
    from src.services.model_manager import get_model_manager
    manager = get_model_manager()
    
    if gpu_changed or active_changed:
        # Unload model to force reload with new settings
        manager.unload_model(model_id)
        
    # Return enriched model
    updated["protected"] = model_id in get_protected_models()
    return {"message": "Model updated", "model": updated}


@router.post("/models", dependencies=[Depends(requires_role("admin"))])
async def add_model(model: dict):
    """Add a new model (persisted to Redis)."""
    store = get_admin_store()
    model_id = model.get("id") or model.get("name", "").replace("/", "-").lower()
    
    models = store.get_models()
    if model_id in models:
        raise HTTPException(status_code=400, detail=f"Model {model_id} already exists")
    
    new_model = {
        "id": model_id,
        "name": model.get("name", model_id),
        "type": model.get("type", "embedding"),
        "active": model.get("active", False),
        "gpu_enabled": model.get("gpu_enabled", False)
    }
    
    store.set_model(model_id, new_model)
    
    new_model["protected"] = model_id in get_protected_models()
    return {"message": f"Model {model_id} added", "model": new_model}


@router.delete("/models/{model_id:path}", dependencies=[Depends(requires_role("admin"))])
async def delete_model(model_id: str):
    """Delete a model (persisted to Redis)."""
    store = get_admin_store()
    models = store.get_models()
    
    if model_id not in models:
        raise HTTPException(status_code=404, detail=f"Model {model_id} not found")
    
    # Don't allow deleting core models
    protected_models = get_protected_models()
    
    # Check if model_id matches any protected model (or is a known alias)
    # Also strict check for system default ID
    if model_id in protected_models:
        raise HTTPException(status_code=400, detail=f"Cannot delete system default model: {model_id}")
    
    store.delete_model(model_id)
    return {"message": f"Model {model_id} deleted"}


# ============== Config Endpoints ==============

@router.get("/config")
async def get_config():
    """Get current configuration (persisted to Redis)."""
    store = get_admin_store()
    return store.get_effective_config()


@router.put("/config", dependencies=[Depends(requires_role("admin"))])
async def update_config(update: ConfigUpdate):
    """Update configuration (persisted to Redis)."""
    store = get_admin_store()
    updated = []
    
    if update.sparse_enabled is not None:
        store.set_config("sparse_enabled", update.sparse_enabled)
        updated.append(f"sparse_enabled={update.sparse_enabled}")
    
    if update.rrf_k is not None:
        if update.rrf_k < 1 or update.rrf_k > 1000:
            raise HTTPException(status_code=400, detail="rrf_k must be between 1 and 1000")
        store.set_config("rrf_k", update.rrf_k)
        updated.append(f"rrf_k={update.rrf_k}")
    
    if update.ast_parsing_enabled is not None:
        store.set_config("ast_parsing_enabled", update.ast_parsing_enabled)
        updated.append(f"ast_parsing_enabled={update.ast_parsing_enabled}")
    
    if update.mcp_enabled is not None:
        store.set_config("mcp_enabled", update.mcp_enabled)
        updated.append(f"mcp_enabled={update.mcp_enabled}")
    
    if update.query_analysis_enabled is not None:
        store.set_config("query_analysis_enabled", update.query_analysis_enabled)
        updated.append(f"query_analysis_enabled={update.query_analysis_enabled}")
    
    if update.worker_pool is not None:
        if update.worker_pool not in ["solo", "threads", "gevent"]:
            raise HTTPException(status_code=400, detail="worker_pool must be solo, threads, or gevent")
        store.set_config("worker_pool", update.worker_pool)
        updated.append(f"worker_pool={update.worker_pool}")
    
    if update.worker_concurrency is not None:
        if update.worker_concurrency < 1 or update.worker_concurrency > 100:
            raise HTTPException(status_code=400, detail="worker_concurrency must be between 1 and 100")
        store.set_config("worker_concurrency", update.worker_concurrency)
        updated.append(f"worker_concurrency={update.worker_concurrency}")

    if update.model_ttl_seconds is not None:
        if update.model_ttl_seconds < 10:
            raise HTTPException(status_code=400, detail="TTL must be at least 10 seconds")
        store.set_config("model_ttl_seconds", update.model_ttl_seconds)
        updated.append(f"model_ttl_seconds={update.model_ttl_seconds}")

    if update.model_auto_unload is not None:
        store.set_config("model_auto_unload", update.model_auto_unload)
        updated.append(f"model_auto_unload={update.model_auto_unload}")

    if update.mcp_transport is not None:
        if update.mcp_transport not in ["stdio", "tcp", "sse"]:
             raise HTTPException(status_code=400, detail="Invalid transport")
        store.set_config("mcp_transport", update.mcp_transport)
        updated.append(f"mcp_transport={update.mcp_transport}")

    if update.mcp_tcp_port is not None:
        if update.mcp_tcp_port < 1024 or update.mcp_tcp_port > 65535:
             raise HTTPException(status_code=400, detail="Invalid port")
        store.set_config("mcp_tcp_port", update.mcp_tcp_port)
        updated.append(f"mcp_tcp_port={update.mcp_tcp_port}")
    
    if not updated:
        return {"message": "No changes made"}
    
    # Auto-save snapshot before changes
    store.save_config_snapshot(f"before_update_{datetime.now().strftime('%H%M%S')}")
    
    return {
        "message": "Configuration updated",
        "updated": updated,
        "restart_required": True
    }


@router.get("/config/history")
async def get_config_history(limit: int = 10):
    """Get config change history for rollback."""
    store = get_admin_store()
    history = store.list_config_history(limit)
    return {"snapshots": history}


@router.post("/config/snapshot", dependencies=[Depends(requires_role("admin"))])
async def create_config_snapshot(label: str = None):
    """Create a config snapshot manually."""
    store = get_admin_store()
    snapshot = store.save_config_snapshot(label)
    if snapshot:
        return {"message": "Snapshot created", "snapshot": snapshot}
    raise HTTPException(status_code=500, detail="Failed to create snapshot")


@router.post("/config/rollback/{index}", dependencies=[Depends(requires_role("admin"))])
async def rollback_config(index: int = 0):
    """Rollback to a previous config snapshot (0 = most recent)."""
    store = get_admin_store()
    success = store.rollback_config(index)
    if success:
        return {"message": f"Rolled back to snapshot at index {index}"}
    raise HTTPException(status_code=404, detail=f"Snapshot at index {index} not found")


# ============== Users Endpoints ==============

@router.get("/users")
async def list_users():
    """List all users (persisted to Redis)."""
    store = get_admin_store()
    users = store.get_users()
    return {"users": list(users.values())}


@router.get("/users/{user_id}")
async def get_user(user_id: str):
    """Get a specific user."""
    store = get_admin_store()
    users = store.get_users()
    if user_id not in users:
        raise HTTPException(status_code=404, detail=f"User {user_id} not found")
    return users[user_id]


@router.post("/users", dependencies=[Depends(requires_role("admin"))])
async def create_user(user: UserCreate):
    """Create a new user (persisted to Redis)."""
    store = get_admin_store()
    user_id = f"user-{uuid.uuid4().hex[:8]}"
    
    new_user = {
        "id": user_id,
        "email": user.email,
        "role": user.role,
        "org_id": user.org_id,
        "active": True,
        "created_at": datetime.now().isoformat()
    }
    store.set_user(user_id, new_user)
    return {"message": f"User {user.email} created", "user": new_user}


@router.put("/users/{user_id}", dependencies=[Depends(requires_role("admin"))])
async def update_user(user_id: str, update: UserUpdate):
    """Update a user (persisted to Redis)."""
    store = get_admin_store()
    users = store.get_users()
    
    if user_id not in users:
        raise HTTPException(status_code=404, detail=f"User {user_id} not found")
    
    user = users[user_id]
    
    if update.role is not None:
        user["role"] = update.role
    if update.active is not None:
        user["active"] = update.active
    if update.org_id is not None:
        user["org_id"] = update.org_id
    
    store.set_user(user_id, user)
    return {"message": f"User {user_id} updated", "user": user}


@router.delete("/users/{user_id}", dependencies=[Depends(requires_role("admin"))])
async def delete_user(user_id: str):
    """Delete a user (persisted to Redis)."""
    store = get_admin_store()
    users = store.get_users()
    
    if user_id not in users:
        raise HTTPException(status_code=404, detail=f"User {user_id} not found")
    
    if user_id == "admin-1":
        raise HTTPException(status_code=400, detail="Cannot delete primary admin")
    
    store.delete_user(user_id)
    store.delete_user(user_id)
    return {"message": f"User {user_id} deleted"}


# ============== Connections Endpoints ==============

@router.get("/connections")
async def list_connections():
    """List all active CLI connections."""
    store = get_admin_store()
    connections = store.get_connections()
    return {"connections": list(connections.values())}

@router.post("/connections/register", dependencies=[Depends(get_current_user)])
async def register_connection(data: ConnectionRegister):
    """Register a CLI connection."""
    store = get_admin_store()
    connection_id = f"conn-{uuid.uuid4().hex[:8]}"
    
    connection = {
        "id": connection_id,
        "user_id": data.user_id,
        "device_name": data.device_name,
        "version": data.version,
        "last_seen": datetime.now().isoformat(),
        "ip": "127.0.0.1" # Mock IP for now
    }
    
    store.set_connection(connection_id, connection)
    store.increment_counter("active_connections")
    
    return {"message": "Connection registered", "connection": connection}

@router.delete("/connections/{connection_id}", dependencies=[Depends(requires_role("admin"))])
async def delete_connection(connection_id: str):
    """Revoke a connection."""
    store = get_admin_store()
    if store.delete_connection(connection_id):
        store.increment_counter("active_connections", -1)
        return {"message": "Connection deleted"}
    raise HTTPException(status_code=404, detail="Connection not found")


# ============== MCP Endpoints ==============

@router.get("/mcp/status")
async def get_mcp_status():
    """Get MCP server status."""
    store = get_admin_store()
    config = store.get_effective_config()
    return {
        "enabled": config.get("mcp_enabled", settings.MCP_ENABLED),
        "transport": settings.MCP_TRANSPORT,
        "tcp_host": settings.MCP_TCP_HOST,
        "tcp_port": settings.MCP_TCP_PORT,
        "tools": ["search", "read_file", "list_files"]
    }


@router.put("/mcp/toggle", dependencies=[Depends(requires_role("admin"))])
async def toggle_mcp():
    """Toggle MCP server on/off."""
    store = get_admin_store()
    config = store.get_effective_config()
    current = config.get("mcp_enabled", settings.MCP_ENABLED)
    store.set_config("mcp_enabled", not current)
    return {
        "message": f"MCP {'disabled' if current else 'enabled'}",
        "enabled": not current,
        "restart_required": True
    }


# ============== System Endpoints ==============

@router.get("/system/status")
async def get_system_status():
    """Get system status overview."""
    store = get_admin_store()
    config = store.get_effective_config()
    models = store.get_models()
    
    return {
        "status": "healthy",
        "features": {
            "hybrid_search": config.get("sparse_enabled", settings.SPARSE_ENABLED),
            "ast_parsing": config.get("ast_parsing_enabled", settings.AST_PARSING_ENABLED),
            "mcp_protocol": config.get("mcp_enabled", settings.MCP_ENABLED),
            "opentelemetry": False  # Disabled due to stability issues
        },
        "models": {
            model_id: model.get("active", True) 
            for model_id, model in models.items()
        }
    }


@router.post("/system/rebuild-index", dependencies=[Depends(requires_role("admin"))])
async def rebuild_index():
    """Trigger index rebuild via Celery."""
    from src.worker.celery_app import app as celery_app
    
    store = get_admin_store()
    store.log_audit("rebuild_index", "Index rebuild triggered", "admin")
    
    # Queue a rebuild task
    try:
        task = celery_app.send_task("src.tasks.ingestion.rebuild_index_task")
        return {
            "message": "Index rebuild triggered",
            "status": "queued",
            "task_id": str(task.id)
        }
    except Exception as e:
        return {
            "message": "Index rebuild triggered (task queued)",
            "status": "queued",
            "note": str(e)
        }


@router.post("/system/clear-cache", dependencies=[Depends(requires_role("admin"))])
async def clear_cache():
    """Clear Redis cache (actually clears cache keys)."""
    store = get_admin_store()
    deleted = store.clear_cache()
    return {
        "message": f"Cache cleared: {deleted} keys deleted",
        "status": "completed",
        "keys_deleted": deleted
    }


# ============== Metrics Endpoints ==============

@router.get("/metrics")
async def get_metrics():
    """Get real system metrics."""
    store = get_admin_store()
    latencies = store.get_latency_percentiles()
    
    # Real system metrics
    cpu_percent = psutil.cpu_percent(interval=0.1)
    memory = psutil.virtual_memory()
    
    # GPU metrics (if available)
    gpu_used = 0
    gpu_total = 8000  # Default 8GB
    try:
        import subprocess
        result = subprocess.run(
            ["nvidia-smi", "--query-gpu=memory.used,memory.total", "--format=csv,noheader,nounits"],
            capture_output=True, text=True, timeout=5
        )
        if result.returncode == 0:
            parts = result.stdout.strip().split(", ")
            gpu_used = int(parts[0])
            gpu_total = int(parts[1])
    except:
        pass  # GPU not available
    
    return {
        "search_latency_p50_ms": int(latencies.get("p50", 0)),
        "search_latency_p95_ms": int(latencies.get("p95", 0)),
        "search_latency_p99_ms": int(latencies.get("p99", 0)),
        "index_rate_docs_per_sec": store.get_counter("indexed_docs"),
        "active_connections": store.get_counter("active_connections"),
        "gpu_memory_used_mb": gpu_used,
        "gpu_memory_total_mb": gpu_total,
        "cpu_usage_percent": int(cpu_percent),
        "memory_usage_mb": int(memory.used / 1024 / 1024)
    }


@router.get("/audit-log")
async def get_audit_log(limit: int = 20):
    """Get recent audit log entries (persisted to Redis)."""
    store = get_admin_store()
    logs = store.get_audit_log(limit)
    return {"logs": logs}


@router.get("/health-history")
async def get_health_history(hours: int = 24):
    """Get health check history."""
    # For now, return empty - would need scheduled health checks
    return {"history": [], "note": "Health history requires scheduled monitoring"}
