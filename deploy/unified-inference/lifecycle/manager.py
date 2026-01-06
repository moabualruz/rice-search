"""
Lifecycle manager for model instance backends.
"""
import asyncio
import logging
from typing import Dict, Optional
from datetime import datetime

from backends.base import Backend
from backends.sglang import SGLangBackend
from backends.cpu_backend import CPUBackend
from backends.ollama import OllamaBackend
from config.models import ModelConfig, ModelRegistry
from config.settings import settings

logger = logging.getLogger(__name__)


class LifecycleManager:
    """Manages lifecycle of model instance backends."""

    def __init__(self, model_registry: ModelRegistry):
        self.model_registry = model_registry
        self.backends: Dict[str, Backend] = {}
        self._starting: Dict[str, bool] = {}  # Track models currently starting
        self._health_check_task: Optional[asyncio.Task] = None
        self._idle_check_task: Optional[asyncio.Task] = None

    def _create_backend(self, model_config: ModelConfig) -> Backend:
        """Create appropriate backend instance based on model config."""
        if model_config.backend == "sglang":
            return SGLangBackend(model_config)
        elif model_config.backend == "cpu_backend":
            return CPUBackend(model_config)
        elif model_config.backend == "ollama":
            return OllamaBackend(model_config)
        else:
            raise ValueError(f"Unknown backend type: {model_config.backend}")

    async def start_model(self, model_name: str) -> bool:
        """Start a model instance backend."""
        # Check if already running in backends dict
        if model_name in self.backends:
            backend = self.backends[model_name]
            if backend.status.is_running and backend.status.is_healthy:
                logger.info(f"Model {model_name} is already running and healthy")
                return True
            elif backend.status.is_running:
                # Running but not healthy - try health check first
                is_healthy = await backend.health_check()
                if is_healthy:
                    logger.info(f"Model {model_name} is running and now healthy")
                    return True
                else:
                    logger.warning(f"Model {model_name} exists but is unhealthy, will restart")
                    await self.stop_model(model_name)

        # Wait if another request is already starting this model
        while self._starting.get(model_name, False):
            logger.debug(f"Model {model_name} is being started by another request, waiting...")
            await asyncio.sleep(0.5)
            # Re-check if it's now running
            if model_name in self.backends and self.backends[model_name].status.is_running:
                logger.info(f"Model {model_name} started by another request")
                return True

        # Mark as starting
        self._starting[model_name] = True

        try:
            model_config = self.model_registry.get(model_name)
            if not model_config:
                logger.error(f"Model {model_name} not found in registry")
                return False

            if model_config.execution_mode != settings.execution_mode:
                logger.error(
                    f"Model {model_name} requires {model_config.execution_mode}, "
                    f"but service is in {settings.execution_mode} mode"
                )
                return False

            # Create backend and check if already running on port (from previous crash)
            backend = self._create_backend(model_config)

            # Try health check first - if port is already serving this model, reuse it
            logger.debug(f"Checking if {model_name} is already running on port {model_config.port}")
            is_already_running = await backend.health_check()
            if is_already_running:
                logger.info(f"Model {model_name} found already running on port {model_config.port}, reusing instance")
                backend.status.is_running = True
                backend.status.is_healthy = True
                self.backends[model_name] = backend
                return True

            # Not running, start fresh
            success = await backend.start()

            if success:
                self.backends[model_name] = backend
                logger.info(f"Model {model_name} started successfully")
            else:
                logger.error(f"Failed to start model {model_name}")

            return success

        except Exception as e:
            logger.error(f"Exception starting model {model_name}: {e}", exc_info=True)
            return False

        finally:
            # Clear starting flag
            self._starting[model_name] = False

    async def stop_model(self, model_name: str) -> bool:
        """Stop a model instance backend."""
        if model_name not in self.backends:
            logger.warning(f"Model {model_name} is not running")
            return True

        backend = self.backends[model_name]
        success = await backend.stop()

        if success:
            del self.backends[model_name]
            logger.info(f"Model {model_name} stopped successfully")
        else:
            logger.error(f"Failed to stop model {model_name}")

        return success

    async def get_backend(self, model_name: str, auto_start: bool = True) -> Optional[Backend]:
        """Get a backend instance, optionally starting it if not running."""
        if model_name in self.backends:
            backend = self.backends[model_name]
            if backend.status.is_running and backend.status.is_healthy:
                return backend

        if auto_start:
            success = await self.start_model(model_name)
            if success:
                return self.backends.get(model_name)

        return None

    async def health_check_loop(self):
        """Background task to monitor backend health."""
        while True:
            try:
                await asyncio.sleep(settings.health_check_interval)

                for model_name, backend in list(self.backends.items()):
                    is_healthy = await backend.health_check()

                    if not is_healthy:
                        logger.warning(f"Model {model_name} failed health check")

                        if backend.process and backend.process.poll() is not None:
                            logger.error(f"Model {model_name} process died")
                            await self.stop_model(model_name)
                            await self.start_model(model_name)

            except Exception as e:
                logger.error(f"Error in health check loop: {e}", exc_info=True)

    async def idle_check_loop(self):
        """Background task to stop idle models."""
        while True:
            try:
                await asyncio.sleep(30)

                for model_name, backend in list(self.backends.items()):
                    # Skip models with idle_timeout=0 (never unload)
                    if backend.model_config.idle_timeout == 0:
                        continue

                    if backend.is_idle:
                        logger.info(f"Model {model_name} is idle, stopping")
                        await self.stop_model(model_name)

            except Exception as e:
                logger.error(f"Error in idle check loop: {e}", exc_info=True)

    async def start_background_tasks(self):
        """Start background monitoring tasks."""
        if not self._health_check_task:
            self._health_check_task = asyncio.create_task(self.health_check_loop())
            logger.info("Started health check loop")

        if not self._idle_check_task:
            self._idle_check_task = asyncio.create_task(self.idle_check_loop())
            logger.info("Started idle check loop")

    async def stop_background_tasks(self):
        """Stop background monitoring tasks."""
        if self._health_check_task:
            self._health_check_task.cancel()
            try:
                await self._health_check_task
            except asyncio.CancelledError:
                pass
            self._health_check_task = None

        if self._idle_check_task:
            self._idle_check_task.cancel()
            try:
                await self._idle_check_task
            except asyncio.CancelledError:
                pass
            self._idle_check_task = None

        logger.info("Stopped background tasks")

    async def stop_all_models(self):
        """Stop all running models."""
        for model_name in list(self.backends.keys()):
            await self.stop_model(model_name)

    def get_status(self) -> Dict:
        """Get status of all backends."""
        return {
            model_name: {
                "is_running": backend.status.is_running,
                "is_healthy": backend.status.is_healthy,
                "pid": backend.status.pid,
                "url": backend.url,
                "uptime": (
                    datetime.now().timestamp() - backend.status.start_time
                    if backend.status.start_time
                    else None
                ),
                "last_request_time": backend.status.last_request_time,
                "error": backend.status.error,
            }
            for model_name, backend in self.backends.items()
        }
