"""
CPU offload policy for GPU backends.
"""
import logging
from typing import Optional
from fastapi import HTTPException

from backends.base import Backend
from lifecycle.manager import LifecycleManager
from config.models import ModelRegistry
from config.settings import settings

logger = logging.getLogger(__name__)


class OffloadPolicy:
    """Handles CPU offload policy for GPU backends."""

    def __init__(self, model_registry: ModelRegistry, lifecycle_manager: LifecycleManager):
        self.model_registry = model_registry
        self.lifecycle_manager = lifecycle_manager

    async def should_offload(self, backend: Backend) -> bool:
        """
        Determine if request should be offloaded to CPU.

        Args:
            backend: GPU backend to check

        Returns:
            True if should offload to CPU
        """
        if not settings.enable_cpu_offload:
            return False

        if settings.execution_mode != "gpu":
            return False

        # Check backend metrics
        try:
            metrics = await backend.get_metrics()
            queue_length = metrics.get("queue_length", 0)
            max_requests = metrics.get("max_running_requests", settings.sglang_max_running_requests)

            # Offload if queue is at or above threshold
            if queue_length >= settings.offload_queue_threshold:
                logger.info(
                    f"GPU backend {backend.model_config.name} queue={queue_length}, "
                    f"triggering CPU offload"
                )
                return True

        except Exception as e:
            logger.error(f"Error checking offload condition: {e}")

        return False

    async def get_cpu_offload_backend(self, gpu_model_name: str) -> Optional[Backend]:
        """
        Get CPU offload backend for a GPU model.

        Args:
            gpu_model_name: Name of the GPU model

        Returns:
            CPU backend instance or None if not configured
        """
        # Get GPU model config
        gpu_model = self.model_registry.get(gpu_model_name)
        if not gpu_model:
            return None

        # Check if CPU offload is configured
        cpu_offload_model = gpu_model.cpu_offload_model
        if not cpu_offload_model:
            logger.debug(f"No CPU offload configured for {gpu_model_name}")
            return None

        # Get CPU backend
        cpu_backend = await self.lifecycle_manager.get_backend(
            cpu_offload_model,
            auto_start=True
        )

        if not cpu_backend:
            logger.warning(f"Failed to get CPU offload backend: {cpu_offload_model}")
            return None

        logger.info(f"Using CPU offload backend: {cpu_offload_model}")
        return cpu_backend

    async def select_backend_with_offload(
        self,
        model_name: str,
        gpu_backend: Backend
    ) -> Backend:
        """
        Select backend with offload policy applied.

        Args:
            model_name: Requested model name
            gpu_backend: GPU backend instance

        Returns:
            Backend to use (GPU or CPU)

        Raises:
            HTTPException: If GPU is overloaded and no CPU offload available
        """
        # Check if should offload
        if await self.should_offload(gpu_backend):
            cpu_backend = await self.get_cpu_offload_backend(model_name)

            if cpu_backend:
                logger.info(f"Offloading request to CPU backend")
                return cpu_backend
            else:
                # No CPU offload available - reject request
                raise HTTPException(
                    status_code=429,
                    detail={
                        "error": {
                            "code": "GPU_OVERLOADED",
                            "message": f"GPU backend for '{model_name}' is at capacity",
                            "hint": "Retry after a short delay or configure CPU offload",
                            "retry_after": 5,
                        }
                    }
                )

        # Use GPU backend
        return gpu_backend
