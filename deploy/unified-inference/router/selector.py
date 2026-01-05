"""
Model selection and routing logic.
"""
import logging
from typing import Optional, Dict, Any
from fastapi import HTTPException

from lifecycle.manager import LifecycleManager
from config.models import ModelRegistry

logger = logging.getLogger(__name__)


class ModelSelector:
    """Handles model selection from requests."""

    def __init__(self, model_registry: ModelRegistry, lifecycle_manager: LifecycleManager):
        self.model_registry = model_registry
        self.lifecycle_manager = lifecycle_manager

    def extract_model_name(self, body: Optional[Dict[str, Any]], query_params: Dict[str, str]) -> str:
        """
        Extract model name from request.

        Args:
            body: Request body (for POST requests)
            query_params: Query parameters (for GET requests)

        Returns:
            Model name

        Raises:
            HTTPException: If model name is missing
        """
        model_name = None

        # Check body first (POST requests)
        if body and "model" in body:
            model_name = body["model"]

        # Check query params (GET requests)
        if not model_name and "model" in query_params:
            model_name = query_params["model"]

        if not model_name:
            raise HTTPException(
                status_code=400,
                detail={
                    "error": {
                        "code": "MODEL_REQUIRED",
                        "message": "Missing 'model' parameter. All requests must specify a model.",
                        "hint": "Include 'model' in request body (POST) or query params (GET)",
                    }
                }
            )

        return model_name

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
