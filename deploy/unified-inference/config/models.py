"""
Model registry loader and definitions.
"""
import yaml
from pathlib import Path
from typing import Dict, List, Literal, Optional
from pydantic import BaseModel, Field, validator


class ModelConfig(BaseModel):
    """Configuration for a single model instance."""

    name: str = Field(..., description="Unique model identifier")
    type: Literal["embedding", "rerank", "llm"] = Field(..., description="Model type")
    execution_mode: Literal["gpu", "cpu"] = Field(..., description="Required execution mode")
    backend: Literal["sglang", "cpu_backend"] = Field(..., description="Backend type")

    # Model loading
    model_path: str = Field(..., description="HF model path or local path")
    format: Literal["hf", "awq", "gguf"] = Field(..., description="Model format")

    # Resource allocation
    gpu_id: Optional[int] = Field(None, description="GPU device ID (GPU mode only)")
    port: Optional[int] = Field(None, description="Assigned port (auto-assigned if None)")

    # Lifecycle
    idle_timeout: int = Field(300, description="Seconds before stopping idle model")

    # Default model flag
    default: bool = Field(False, description="Is this the default model for its type")

    # SGLang-specific options (for GPU mode)
    is_embedding: bool = Field(False, description="Use --is-embedding flag")
    trust_remote_code: bool = Field(True, description="Trust remote code")
    dtype: Optional[str] = Field("auto", description="Data type (auto, float16, bfloat16)")

    # CPU offload configuration (GPU mode only)
    cpu_offload_model: Optional[str] = Field(None, description="CPU model to offload to")

    @validator("format")
    def validate_format(cls, v, values):
        """Validate format compatibility with backend."""
        backend = values.get("backend")
        execution_mode = values.get("execution_mode")

        if backend == "sglang" and v == "gguf":
            raise ValueError("SGLang does not support GGUF format. Use 'hf' or 'awq'.")

        if execution_mode == "gpu" and v == "gguf":
            raise ValueError("GPU mode does not support GGUF. Use 'hf' or 'awq'.")

        return v

    class Config:
        extra = "allow"  # Allow additional fields for future extensions


class ModelRegistry:
    """Registry of available models loaded from YAML."""

    def __init__(self, config_path: str):
        self.config_path = Path(config_path)
        self.models: Dict[str, ModelConfig] = {}
        self._port_counter = 30000
        self._load()

    def _load(self):
        """Load models from YAML file."""
        if not self.config_path.exists():
            raise FileNotFoundError(f"Model config not found: {self.config_path}")

        with open(self.config_path, "r") as f:
            data = yaml.safe_load(f)

        if not data or "models" not in data:
            raise ValueError("Invalid models.yaml: missing 'models' key")

        for model_data in data["models"]:
            model = ModelConfig(**model_data)

            # Auto-assign port if not specified
            if model.port is None:
                model.port = self._port_counter
                self._port_counter += 1

            self.models[model.name] = model

    def get(self, name: str) -> Optional[ModelConfig]:
        """Get model config by name."""
        return self.models.get(name)

    def get_default_model(self, model_type: str) -> Optional[ModelConfig]:
        """Get the default model for a given type."""
        for model in self.models.values():
            if model.type == model_type and model.default:
                return model
        return None

    def list_models(
        self,
        execution_mode: Optional[str] = None,
        model_type: Optional[str] = None
    ) -> List[ModelConfig]:
        """List all models, optionally filtered."""
        models = list(self.models.values())

        if execution_mode:
            models = [m for m in models if m.execution_mode == execution_mode]

        if model_type:
            models = [m for m in models if m.type == model_type]

        return models

    def reload(self):
        """Reload models from YAML."""
        self.models.clear()
        self._port_counter = 30000
        self._load()
