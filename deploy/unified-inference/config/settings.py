"""
Global settings for unified-inference service.
"""
from pydantic_settings import BaseSettings
from typing import Literal


class Settings(BaseSettings):
    """Global configuration for unified-inference orchestrator."""

    # Execution mode: STATIC per deployment, set via env var
    # This determines which backends are available for model instances
    execution_mode: Literal["gpu", "cpu"] = "gpu"

    # Router configuration
    router_host: str = "0.0.0.0"
    router_port: int = 3001

    # SGLang backend configuration
    sglang_base_port: int = 30000  # Auto-increment for each model instance
    sglang_max_running_requests: int = 3  # ENFORCED (LLM models)
    sglang_max_running_requests_embedding: int = 20  # Higher for embedding/rerank models (handle concurrent tasks)
    sglang_max_total_tokens: int = 16384  # Max tokens for LLM (allows reasonable context while fitting 3 models on GPU)
    sglang_mem_fraction_static: float = 0.15  # Minimal static allocation (15% of GPU) to fit embedding + reranker + LLM
    sglang_schedule_conservativeness: float = 0.5  # Lower = more aggressive scheduling (default 1.0), applies to ALL models

    # Model registry
    models_config_path: str = "/app/config/models.yaml"

    # Lifecycle management
    default_idle_timeout: int = 300  # 5 minutes
    health_check_interval: int = 10  # seconds
    startup_timeout: int = 120  # seconds to wait for backend startup
    shutdown_grace_period: int = 30  # seconds to wait for graceful shutdown

    # CPU offload policy (only applies in GPU mode)
    enable_cpu_offload: bool = True
    offload_queue_threshold: int = 3  # Trigger offload if GPU queue >= this
    offload_on_gpu_oom: bool = True

    # Request handling
    request_timeout: int = 300  # 5 minutes max per request
    proxy_timeout: int = 310  # Slightly higher than request timeout

    # Logging
    log_level: str = "INFO"

    class Config:
        env_prefix = ""
        case_sensitive = False


settings = Settings()
