"""
Abstract base class for model instance backends.
"""
from abc import ABC, abstractmethod
from typing import Optional, Dict, Any
from config.models import ModelConfig


class BackendStatus:
    """Status of a backend instance."""

    def __init__(self):
        self.is_running: bool = False
        self.is_healthy: bool = False
        self.pid: Optional[int] = None
        self.url: Optional[str] = None
        self.last_request_time: Optional[float] = None
        self.start_time: Optional[float] = None
        self.error: Optional[str] = None


class Backend(ABC):
    """Abstract base class for model instance backends."""

    def __init__(self, model_config: ModelConfig):
        self.model_config = model_config
        self.status = BackendStatus()

    @abstractmethod
    async def start(self) -> bool:
        """
        Start the backend server process.

        Returns:
            True if started successfully, False otherwise
        """
        pass

    @abstractmethod
    async def stop(self) -> bool:
        """
        Stop the backend server process gracefully.

        Returns:
            True if stopped successfully, False otherwise
        """
        pass

    @abstractmethod
    async def health_check(self) -> bool:
        """
        Check if the backend is healthy and ready to serve requests.

        Returns:
            True if healthy, False otherwise
        """
        pass

    @abstractmethod
    async def get_metrics(self) -> Dict[str, Any]:
        """
        Get current metrics from the backend.

        Returns:
            Dictionary with metrics (queue size, memory usage, etc.)
        """
        pass

    @property
    def is_idle(self) -> bool:
        """Check if backend has been idle longer than configured timeout."""
        if not self.status.last_request_time:
            return False

        import time
        idle_duration = time.time() - self.status.last_request_time
        return idle_duration > self.model_config.idle_timeout

    @property
    def url(self) -> str:
        """Get the backend URL."""
        return self.status.url or f"http://localhost:{self.model_config.port}"

    def mark_request(self):
        """Mark that a request was just handled."""
        import time
        self.status.last_request_time = time.time()
