"""
Model selection and routing logic with default model support.
"""
import logging
from typing import Optional, Dict, Any
from fastapi import HTTPException

from lifecycle.manager import LifecycleManager
from config.models import ModelRegistry

logger = logging.getLogger(__name__)


# Endpoint to model type mapping
ENDPOINT_MODEL_TYPE_MAP = {
    "/v1/embeddings": "embedding",
    "/v1/rerank": "rerank",
    "/v1/completions": "llm",
    "/v1/chat/completions": "llm",
    "/v1/generate": "llm",
}


class ModelSelector:
    """Handles model selection from requests with default model support."""

    def __init__(self, model_registry: ModelRegistry, lifecycle_manager: LifecycleManager):
        self.model_registry = model_registry
        self.lifecycle_manager = lifecycle_manager

    def infer_model_type_from_path(self, path: str) -> Optional[str]:
        """
        Infer model type from request path.

        Args:
            path: Request path (e.g., "/v1/embeddings")

        Returns:
            Model type ("embedding", "rerank", "llm") or None
        """
        # Normalize path (remove leading/trailing slashes)
        path = "/" + path.strip("/")

        # Check exact matches
        if path in ENDPOINT_MODEL_TYPE_MAP:
            return ENDPOINT_MODEL_TYPE_MAP[path]

        # Check prefixes
        for endpoint_path, model_type in ENDPOINT_MODEL_TYPE_MAP.items():
            if path.startswith(endpoint_path):
                return model_type

        return None

    def extract_model_name(
        self,
        body: Optional[Dict[str, Any]],
        query_params: Dict[str, str],
        path: str
    ) -> str:
        """
        Extract model name from request or auto-select default.

        Priority:
        1. Explicit 'model' in request body (POST requests)
        2. Explicit 'model' in query params (GET requests)
        3. Auto-select default model based on endpoint path

        Args:
            body: Request body (for POST requests)
            query_params: Query parameters (for GET requests)
            path: Request path

        Returns:
            Model name

        Raises:
            HTTPException: If no model can be determined
        """
        model_name = None

        # Check body first (POST requests)
        if body and "model" in body:
            model_name = body["model"]
            logger.debug(f"Explicit model from body: {model_name}")
            return model_name

        # Check query params (GET requests)
        if "model" in query_params:
            model_name = query_params["model"]
            logger.debug(f"Explicit model from query: {model_name}")
            return model_name

        # Auto-select default model based on endpoint
        model_type = self.infer_model_type_from_path(path)
        if model_type:
            default_model = self.model_registry.get_default_model(model_type)
            if default_model:
                logger.debug(
                    f"Auto-selected default {model_type} model: {default_model.name}"
                )
                return default_model.name
            else:
                raise HTTPException(
                    status_code=500,
                    detail={
                        "error": {
                            "code": "NO_DEFAULT_MODEL",
                            "message": f"No default {model_type} model configured",
                            "hint": "Either specify 'model' parameter or configure a default model"
                        }
                    }
                )

        # Cannot determine model
        raise HTTPException(
            status_code=400,
            detail={
                "error": {
                    "code": "MODEL_REQUIRED",
                    "message": (
                        "Cannot determine model. "
                        "Either use a known endpoint (e.g., /v1/embeddings, /v1/chat/completions) "
                        "or specify 'model' parameter explicitly."
                    ),
                    "hint": "Include 'model' in request body (POST) or query params (GET)",
                }
            }
        )

    def validate_model(self, model_name: str) -> bool:
        """
        Validate that model exists and is compatible.

        Args:
            model_name: Name of the model

        Returns:
            True if valid

        Raises:
            HTTPException: If model is invalid or incompatible
        """
        model_config = self.model_registry.get(model_name)

        if not model_config:
            available_models = [m.name for m in self.model_registry.list_models()]
            raise HTTPException(
                status_code=404,
                detail={
                    "error": {
                        "code": "MODEL_NOT_FOUND",
                        "message": f"Model '{model_name}' is not registered",
                        "available_models": available_models,
                        "install_endpoint": "/v1/models/install",
                    }
                }
            )

        # Check GGUF incompatibility
        if model_config.format == "gguf" and model_config.backend == "sglang":
            raise HTTPException(
                status_code=400,
                detail={
                    "error": {
                        "code": "FORMAT_NOT_SUPPORTED",
                        "message": f"Model '{model_name}' uses GGUF format which is not supported by SGLang",
                        "preferred_formats": ["hf", "awq"],
                        "install_endpoint": "/v1/models/install",
                    }
                }
            )

        return True

    async def select_backend(self, model_name: str):
        """
        Select and get backend for model.

        Args:
            model_name: Name of the model

        Returns:
            Backend instance

        Raises:
            HTTPException: If backend cannot be obtained
        """
        backend = await self.lifecycle_manager.get_backend(model_name, auto_start=True)

        if not backend:
            raise HTTPException(
                status_code=503,
                detail={
                    "error": {
                        "code": "MODEL_UNAVAILABLE",
                        "message": f"Model '{model_name}' could not be started",
                        "hint": "Check logs for startup errors",
                    }
                }
            )

        return backend
