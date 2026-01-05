"""
SGLang backend manager for GPU-mode model instances.
"""
import asyncio
import subprocess
import time
import httpx
import logging
from typing import Dict, Any, Optional
from pathlib import Path

from backends.base import Backend
from config.models import ModelConfig
from config.settings import settings

logger = logging.getLogger(__name__)


class SGLangBackend(Backend):
    """SGLang backend for GPU-mode inference."""

    def __init__(self, model_config: ModelConfig):
        super().__init__(model_config)
        self.process: Optional[subprocess.Popen] = None
        self._build_command()

    def _build_command(self):
        """Build the SGLang server command."""
        cmd = [
            "python", "-m", "sglang.launch_server",
            "--model-path", self.model_config.model_path,
            "--host", "0.0.0.0",
            "--port", str(self.model_config.port),
            
            # ENFORCED: Elastic memory configuration
            "--max-running-requests", str(settings.sglang_max_running_requests),
            "--max-total-tokens", str(settings.sglang_max_total_tokens),
        ]

        # GPU configuration
        if self.model_config.gpu_id is not None:
            cmd.extend(["--tp", "1"])  # Single GPU
            # Set via CUDA_VISIBLE_DEVICES in env

        # Model type specific flags
        if self.model_config.is_embedding:
            cmd.append("--is-embedding")

        # Optional flags
        if self.model_config.trust_remote_code:
            cmd.append("--trust-remote-code")

        if self.model_config.dtype:
            cmd.extend(["--dtype", self.model_config.dtype])

        # Format-specific options
        if self.model_config.format == "awq":
            cmd.extend(["--quantization", "awq"])

        self.command = cmd

    async def start(self) -> bool:
        """Start the SGLang server process."""
        if self.status.is_running:
            logger.warning(f"Backend {self.model_config.name} already running")
            return True

        try:
            logger.info(f"Starting SGLang backend: {self.model_config.name}")
            logger.debug(f"Command: {' '.join(self.command)}")

            # Set environment
            env = {
                **subprocess.os.environ.copy(),
                "CUDA_VISIBLE_DEVICES": str(self.model_config.gpu_id or 0),
            }

            # Start process
            self.process = subprocess.Popen(
                self.command,
                env=env,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True,
            )

            self.status.pid = self.process.pid
            self.status.start_time = time.time()
            self.status.url = f"http://localhost:{self.model_config.port}"

            # Wait for startup with timeout
            startup_deadline = time.time() + settings.startup_timeout
            while time.time() < startup_deadline:
                if await self.health_check():
                    self.status.is_running = True
                    self.status.is_healthy = True
                    logger.info(f"Backend {self.model_config.name} started successfully (PID: {self.status.pid})")
                    return True

                # Check if process died
                if self.process.poll() is not None:
                    stdout, stderr = self.process.communicate()
                    logger.error(f"Backend {self.model_config.name} died during startup")
                    logger.error(f"stdout: {stdout}")
                    logger.error(f"stderr: {stderr}")
                    self.status.error = f"Process died: {stderr[:500]}"
                    return False

                await asyncio.sleep(2)

            # Timeout
            logger.error(f"Backend {self.model_config.name} startup timeout")
            await self.stop()
            return False

        except Exception as e:
            logger.error(f"Failed to start backend {self.model_config.name}: {e}")
            self.status.error = str(e)
            return False

    async def stop(self) -> bool:
        """Stop the SGLang server process."""
        if not self.process:
            return True

        try:
            logger.info(f"Stopping SGLang backend: {self.model_config.name}")

            # Graceful shutdown
            self.process.terminate()

            # Wait for graceful shutdown
            try:
                self.process.wait(timeout=settings.shutdown_grace_period)
            except subprocess.TimeoutExpired:
                logger.warning(f"Backend {self.model_config.name} did not stop gracefully, killing")
                self.process.kill()
                self.process.wait()

            self.status.is_running = False
            self.status.is_healthy = False
            self.status.pid = None
            logger.info(f"Backend {self.model_config.name} stopped")
            return True

        except Exception as e:
            logger.error(f"Failed to stop backend {self.model_config.name}: {e}")
            return False

    async def health_check(self) -> bool:
        """Check if SGLang server is healthy."""
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.get(f"{self.url}/health")
                is_healthy = response.status_code == 200
                self.status.is_healthy = is_healthy
                return is_healthy
        except Exception:
            self.status.is_healthy = False
            return False

    async def get_metrics(self) -> Dict[str, Any]:
        """Get metrics from SGLang server."""
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.get(f"{self.url}/get_model_info")
                if response.status_code == 200:
                    data = response.json()
                    return {
                        "queue_length": data.get("num_requests_running", 0),
                        "max_running_requests": settings.sglang_max_running_requests,
                        "model_info": data,
                    }
        except Exception as e:
            logger.debug(f"Failed to get metrics from {self.model_config.name}: {e}")

        return {
            "queue_length": 0,
            "max_running_requests": settings.sglang_max_running_requests,
        }
