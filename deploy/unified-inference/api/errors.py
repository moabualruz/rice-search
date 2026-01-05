"""
Structured error responses.
"""
from typing import List, Optional, Dict, Any
from pydantic import BaseModel


class ErrorDetail(BaseModel):
    """Structured error detail."""
    code: str
    message: str
    hint: Optional[str] = None
    available_models: Optional[List[str]] = None
    preferred_formats: Optional[List[str]] = None
    install_endpoint: Optional[str] = None
    retry_after: Optional[int] = None


class ErrorResponse(BaseModel):
    """Error response wrapper."""
    error: ErrorDetail


def model_not_found_error(model_name: str, available_models: List[str]) -> Dict[str, Any]:
    """Generate model not found error."""
    return {
        "error": {
            "code": "MODEL_NOT_FOUND",
            "message": f"Model '{model_name}' is not registered",
            "available_models": available_models,
            "install_endpoint": "/v1/models/install",
        }
    }


def model_format_error(model_name: str, format: str) -> Dict[str, Any]:
    """Generate format not supported error."""
    return {
        "error": {
            "code": "FORMAT_NOT_SUPPORTED",
            "message": f"Model '{model_name}' uses {format} format which is not supported",
            "preferred_formats": ["hf", "awq"],
            "install_endpoint": "/v1/models/install",
        }
    }


def model_unavailable_error(model_name: str, reason: str = "") -> Dict[str, Any]:
    """Generate model unavailable error."""
    message = f"Model '{model_name}' is unavailable"
    if reason:
        message += f": {reason}"

    return {
        "error": {
            "code": "MODEL_UNAVAILABLE",
            "message": message,
            "hint": "Check service logs for details",
        }
    }


def gpu_overloaded_error(model_name: str) -> Dict[str, Any]:
    """Generate GPU overloaded error."""
    return {
        "error": {
            "code": "GPU_OVERLOADED",
            "message": f"GPU backend for '{model_name}' is at capacity",
            "hint": "Retry after a short delay",
            "retry_after": 5,
        }
    }
