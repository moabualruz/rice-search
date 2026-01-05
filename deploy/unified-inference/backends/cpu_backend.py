"""
CPU backend manager for CPU-mode model instances.

This is a PLACEHOLDER implementation.
In production, this would integrate with llama.cpp, ggml, or similar CPU backends.
"""
import logging
from typing import Dict, Any

from backends.base import Backend
from config.models import ModelConfig

logger = logging.getLogger(__name__)


class CPUBackend(Backend):
    """CPU backend for CPU-mode inference (PLACEHOLDER)."""

    def __init__(self, model_config: ModelConfig):
        super().__init__(model_config)
        logger.warning(f"CPUBackend for {model_config.name} is a PLACEHOLDER")

    async def start(self) -> bool:
        """Start the CPU backend server process (PLACEHOLDER)."""
        logger.warning(f"CPUBackend.start() is not implemented for {self.model_config.name}")
        logger.info(
            "To implement CPU backend, integrate with:\n"
            "  - llama.cpp server (GGUF support)\n"
            "  - ggml\n"
            "  - ctransformers\n"
            "  - or similar CPU inference engine"
        )
        return False

    async def stop(self) -> bool:
        """Stop the CPU backend server process (PLACEHOLDER)."""
        logger.info(f"CPUBackend.stop() called for {self.model_config.name}")
        return True

    async def health_check(self) -> bool:
        """Check if CPU backend is healthy (PLACEHOLDER)."""
        return False

    async def get_metrics(self) -> Dict[str, Any]:
        """Get metrics from CPU backend (PLACEHOLDER)."""
        return {
            "queue_length": 0,
            "status": "not_implemented",
        }
