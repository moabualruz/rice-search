"""
FastAPI routes for unified-inference.
"""
import logging
from typing import Dict, Any, Optional
from fastapi import APIRouter, Request, Response, HTTPException

from lifecycle.manager import LifecycleManager
from router.selector import ModelSelector
from router.proxy import RequestProxy
from router.offload import OffloadPolicy
from config.models import ModelRegistry
from config.settings import settings

logger = logging.getLogger(__name__)

router = APIRouter()


# Global state (initialized in app.py)
lifecycle_manager: Optional[LifecycleManager] = None
model_selector: Optional[ModelSelector] = None
request_proxy: Optional[RequestProxy] = None
offload_policy: Optional[OffloadPolicy] = None
model_registry: Optional[ModelRegistry] = None


def init_routes(
    lm: LifecycleManager,
    ms: ModelSelector,
    rp: RequestProxy,
    op: OffloadPolicy,
    mr: ModelRegistry,
):
    """Initialize route dependencies."""
    global lifecycle_manager, model_selector, request_proxy, offload_policy, model_registry
    lifecycle_manager = lm
    model_selector = ms
    request_proxy = rp
    offload_policy = op
    model_registry = mr


@router.get("/health")
async def health_check():
    """Health check endpoint."""
    return {"status": "healthy", "execution_mode": settings.execution_mode}


@router.get("/v1/models")
async def list_models(model: Optional[str] = None):
    """
    List available models.

    OpenAI-compatible endpoint with optional model filter.
    """
    if model:
        # Get specific model info
        model_config = model_registry.get(model)
        if not model_config:
            raise HTTPException(status_code=404, detail=f"Model '{model}' not found")

        return {
            "object": "model",
            "id": model_config.name,
            "type": model_config.type,
            "execution_mode": model_config.execution_mode,
            "backend": model_config.backend,
            "format": model_config.format,
        }

    # List all models
    models = model_registry.list_models(execution_mode=settings.execution_mode)
    return {
        "object": "list",
        "data": [
            {
                "id": m.name,
                "object": "model",
                "type": m.type,
                "execution_mode": m.execution_mode,
                "backend": m.backend,
            }
            for m in models
        ]
    }


@router.get("/v1/models/install")
async def installation_guidance():
    """
    Model installation guidance endpoint.

    DOES NOT download or install models.
    ONLY provides documentation.
    """
    return {
        "message": "Model installation guidance",
        "supported_formats": {
            "gpu_mode": {
                "formats": ["hf", "awq"],
                "backend": "sglang",
                "examples": [
                    "Qwen/Qwen2.5-Coder-1.5B-Instruct-AWQ",
                    "BAAI/bge-base-en-v1.5",
                    "BAAI/bge-reranker-v2-m3",
                ],
            },
            "cpu_mode": {
                "formats": ["gguf"],
                "backend": "cpu_backend (llama.cpp)",
                "examples": [
                    "TheBloke/Llama-2-7B-GGUF",
                ],
                "note": "CPU backend is a PLACEHOLDER and needs implementation",
            },
        },
        "unsupported": {
            "gguf_on_sglang": "GGUF format cannot be used with SGLang (GPU mode)",
        },
        "requirements": {
            "gpu": "NVIDIA GPU with CUDA 11.8+",
            "memory": "Varies by model (1.5B ~3GB, 7B ~14GB, 70B ~140GB)",
        },
        "installation_steps": [
            "1. Download model from HuggingFace",
            "2. Add model entry to config/models.yaml",
            "3. Restart unified-inference service",
            "4. Model will auto-start on first request",
        ],
    }


@router.get("/v1/status")
async def get_status():
    """Get status of all model backends."""
    return {
        "execution_mode": settings.execution_mode,
        "backends": lifecycle_manager.get_status(),
    }


@router.post("/v1/admin/models/{model_name}/start")
async def start_model(model_name: str):
    """Manually start a model."""
    success = await lifecycle_manager.start_model(model_name)
    if success:
        return {"status": "started", "model": model_name}
    else:
        raise HTTPException(status_code=500, detail=f"Failed to start {model_name}")


@router.post("/v1/admin/models/{model_name}/stop")
async def stop_model(model_name: str):
    """Manually stop a model."""
    success = await lifecycle_manager.stop_model(model_name)
    if success:
        return {"status": "stopped", "model": model_name}
    else:
        raise HTTPException(status_code=500, detail=f"Failed to stop {model_name}")


@router.api_route("/{path:path}", methods=["GET", "POST", "PUT", "DELETE", "PATCH"])
async def proxy_to_backend(request: Request, path: str):
    """
    Proxy all other requests to appropriate backend.

    This is the main routing endpoint that:
    1. Extracts model name from request
    2. Validates model
    3. Gets backend (with auto-start)
    4. Applies offload policy (if GPU mode)
    5. Proxies request to backend
    """
    try:
        # Parse body if POST
        body = None
        if request.method in ("POST", "PUT", "PATCH"):
            try:
                body = await request.json()
            except Exception:
                body = {}

        # Extract model name
        model_name = model_selector.extract_model_name(body, dict(request.query_params))

        # Validate model
        model_selector.validate_model(model_name)

        # Get backend
        backend = await model_selector.select_backend(model_name)

        # Apply offload policy (GPU mode only)
        if settings.execution_mode == "gpu" and settings.enable_cpu_offload:
            backend = await offload_policy.select_backend_with_offload(model_name, backend)

        # Proxy request
        return await request_proxy.proxy_request(
            request=request,
            backend=backend,
            path=f"/{path}",
            body=body,
        )

    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Error in proxy handler: {e}", exc_info=True)
        raise HTTPException(
            status_code=500,
            detail={
                "error": {
                    "code": "INTERNAL_ERROR",
                    "message": str(e),
                }
            }
        )
