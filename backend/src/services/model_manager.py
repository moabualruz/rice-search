"""
Model Manager Service.

Centralized registry for managing the lifecycle of AI models (loading, unloading, memory management).
Enables dynamic GPU resource management (pause/resume).
"""

import logging
import gc
import torch
from typing import Dict, Any, Optional, Callable
import psutil
import subprocess

logger = logging.getLogger(__name__)

class ModelManager:
    """
    Singleton manager for tracking and controlling loaded AI models.
    """
    _instance: Optional["ModelManager"] = None
    
    def __init__(self):
        # Registry of registered model loaders
        # format: {model_id: {"loader": config_loader_func, "instance": model_obj, "type": str}}
        self._models: Dict[str, Dict[str, Any]] = {}
    
    @classmethod
    def get_instance(cls) -> "ModelManager":
        if cls._instance is None:
            cls._instance = cls()
        return cls._instance

    def register_model(self, model_id: str, model_type: str, instance: Any = None):
        """
        Register a model to be managed. 
        If instance is provided, it's considered loaded.
        """
        if model_id not in self._models:
            self._models[model_id] = {
                "type": model_type,
                "instance": instance,
                "loaded": instance is not None
            }
            logger.info(f"Registered model: {model_id} ({model_type})")
        else:
            # Update instance if re-registering
            if instance is not None:
                self._models[model_id]["instance"] = instance
                self._models[model_id]["loaded"] = True

    def unload_model(self, model_id: str) -> bool:
        """
        Unload a model from memory (move to CPU and delete).
        Returns True if unloaded, False if not found or already unloaded.
        """
        if model_id not in self._models:
            return False
            
        model_entry = self._models[model_id]
        if not model_entry["loaded"] or model_entry["instance"] is None:
            return True # Already unloaded
            
        logger.info(f"Unloading model: {model_id}")
        
        try:
            # 1. Access the instance
            instance = model_entry["instance"]
            
            # 2. Specific unload logic based on type/library
            # Most HF models / SentenceTransformers differ slightly, 
            # but generally we want to move to CPU and delete.
            
            if hasattr(instance, "cpu"):
                instance.cpu()
            elif hasattr(instance, "model") and hasattr(instance.model, "cpu"):
                # SentenceTransformer / CrossEncoder wrapper
                instance.model.cpu()
                
            # 3. Release reference
            self._models[model_id]["instance"] = None
            self._models[model_id]["loaded"] = False
            
            # 4. Force GC
            del instance
            gc.collect()
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
                
            logger.info(f"Model {model_id} unloaded successfully")
            return True
        except Exception as e:
            logger.error(f"Failed to unload model {model_id}: {e}")
            return False

    def load_model(self, model_id: str, loader_func: Callable[[], Any]) -> bool:
        """
        Load a model using the provided loader function.
        """
        if model_id not in self._models:
             # Auto-register placeholder
             self.register_model(model_id, "unknown")
             
        if self._models[model_id]["loaded"]:
            return True
            
        logger.info(f"Loading model: {model_id}")
        try:
            instance = loader_func()
            self._models[model_id]["instance"] = instance
            self._models[model_id]["loaded"] = True
            return True
        except Exception as e:
            logger.error(f"Failed to load model {model_id}: {e}")
            return False

    def get_model_status(self, model_id: str) -> Dict[str, Any]:
        """Get status of a specific model."""
        if model_id not in self._models:
            return {"id": model_id, "loaded": False, "registered": False}
        return {
            "id": model_id,
            "type": self._models[model_id]["type"],
            "loaded": self._models[model_id]["loaded"]
        }

    def get_all_models(self) -> Dict[str, Dict[str, Any]]:
        """Get status of all registered models."""
        return {
            mid: {
                "type": data["type"], 
                "loaded": data["loaded"]
            } 
            for mid, data in self._models.items()
        }

    def get_gpu_usage(self) -> Dict[str, Any]:
        """Get current GPU memory usage metrics."""
        usage = {
            "available": False,
            "used_mb": 0,
            "total_mb": 0,
            "percent": 0
        }
        
        if not torch.cuda.is_available():
            return usage
            
        try:
            # Use torch for basic info
            usage["available"] = True
            
            # NVIDIA-SMI for system-wide stats (more accurate for process/driver overhead)
            result = subprocess.run(
                ["nvidia-smi", "--query-gpu=memory.used,memory.total,utilization.gpu", "--format=csv,noheader,nounits"],
                capture_output=True, text=True, timeout=2
            )
            if result.returncode == 0:
                # Output format: used, total, utilization
                parts = result.stdout.strip().split(", ")
                if len(parts) >= 3:
                    usage["used_mb"] = int(parts[0])
                    usage["total_mb"] = int(parts[1])
                    usage["percent"] = int(parts[2])
            else:
                # Fallback to torch properties
                idx = torch.cuda.current_device()
                mem_alloc = torch.cuda.memory_allocated(idx) / 1024 / 1024
                # Total isn't cleanly available via torch.cuda without pynvml, so we skip or mock
                usage["used_mb"] = int(mem_alloc)
                
        except Exception as e:
            logger.warning(f"Failed to get GPU usage: {e}")
            
        return usage

# Module level helper
def get_model_manager() -> ModelManager:
    return ModelManager.get_instance()
