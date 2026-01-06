"""
Ollama backend for unified model serving with automatic memory management.
"""
import asyncio
import httpx
import logging
from typing import Dict, Any, Optional

from backends.base import Backend
from config.models import ModelConfig

logger = logging.getLogger(__name__)


class OllamaBackend(Backend):
    """Ollama backend with automatic memory management and GPU/CPU support."""

    def __init__(self, model_config: ModelConfig):
        super().__init__(model_config)
        # Ollama runs as a shared service, not per-model process
        self.ollama_base_url = "http://ollama:11434"

    async def start(self) -> bool:
        """Ensure model is pulled and ready in Ollama."""
        if self.status.is_running:
            logger.warning(f"Backend {self.model_config.name} already running")
            return True

        try:
            logger.info(f"Ensuring Ollama model {self.model_config.model_path} is available")

            # Check if model exists, pull if not
            async with httpx.AsyncClient(timeout=300.0) as client:
                # List models
                response = await client.get(f"{self.ollama_base_url}/api/tags")
                if response.status_code == 200:
                    models = response.json().get("models", [])
                    model_names = [m["name"] for m in models]

                    if self.model_config.model_path not in model_names:
                        logger.info(f"Pulling Ollama model {self.model_config.model_path}...")
                        # Pull model
                        pull_response = await client.post(
                            f"{self.ollama_base_url}/api/pull",
                            json={"name": self.model_config.model_path},
                            timeout=600.0
                        )

                        if pull_response.status_code != 200:
                            logger.error(f"Failed to pull model: {pull_response.text}")
                            return False

                        logger.info(f"Model {self.model_config.model_path} pulled successfully")
                    else:
                        logger.info(f"Model {self.model_config.model_path} already available")

                # Mark as running
                self.status.is_running = True
                self.status.is_healthy = True
                self.status.url = f"{self.ollama_base_url}/api"

                logger.info(f"Ollama model {self.model_config.name} ready")
                return True

        except Exception as e:
            logger.error(f"Failed to initialize Ollama model {self.model_config.name}: {e}")
            self.status.error = str(e)
            return False

    async def stop(self) -> bool:
        """Stop is a no-op for Ollama (shared service manages models)."""
        logger.info(f"Unloading Ollama model {self.model_config.name}")

        try:
            # Ollama automatically manages memory, but we can explicitly unload if needed
            # For now, just mark as stopped
            self.status.is_running = False
            self.status.is_healthy = False
            logger.info(f"Model {self.model_config.name} marked as unloaded")
            return True

        except Exception as e:
            logger.error(f"Error during Ollama model unload: {e}")
            return False

    async def health_check(self) -> bool:
        """Check if Ollama service and model are healthy."""
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                # Check Ollama service health
                response = await client.get(f"{self.ollama_base_url}/api/tags")
                is_healthy = response.status_code == 200
                self.status.is_healthy = is_healthy
                return is_healthy
        except Exception:
            self.status.is_healthy = False
            return False

    async def get_metrics(self) -> Dict[str, Any]:
        """Get metrics from Ollama."""
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.get(f"{self.ollama_base_url}/api/tags")
                if response.status_code == 200:
                    data = response.json()
                    return {
                        "loaded_models": len(data.get("models", [])),
                        "model_name": self.model_config.model_path,
                    }
        except Exception as e:
            logger.debug(f"Failed to get metrics from Ollama: {e}")

        return {
            "loaded_models": 0,
            "model_name": self.model_config.model_path,
        }
