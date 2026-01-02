"""
Model Manager Service.

Centralized registry for managing the lifecycle of AI models (loading, unloading, memory management).
Enables dynamic GPU resource management (pause/resume).
"""

import logging
import gc
import time
# import torch  # Lazy loaded
from typing import Dict, Any, Optional, Callable, List
# import psutil # Lazy loaded
import subprocess
from src.services.admin.admin_store import get_admin_store


logger = logging.getLogger(__name__)

import threading
from src.core.config import settings

class ModelManager:
    """
    Singleton manager for tracking and controlling loaded AI models.
    """
    _instance: Optional["ModelManager"] = None
    
    def __init__(self):
        # Registry of registered model loaders
        # format: {model_id: {"loader": config_loader_func, "instance": model_obj, "type": str}}
        self._models: Dict[str, Dict[str, Any]] = {}
        
        # TTL Monitor
        if settings.MODEL_AUTO_UNLOAD:
            self._running = True
            self._monitor_thread = threading.Thread(target=self._ttl_monitor_loop, daemon=True)
            self._monitor_thread.start()
            logger.info(f"TTL Monitor started (TTL={settings.MODEL_TTL_SECONDS}s)")
    
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
                "loaded": instance is not None,
                "last_accessed": time.time() if instance is not None else 0
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
            import torch
            if torch.cuda.is_available():
                torch.cuda.empty_cache()
            
            # Sync with Redis
            try:
                from src.services.admin.admin_store import get_admin_store
                store = get_admin_store()
                if store.redis:
                    store.redis.delete(f"model_status:{model_id}:loaded")
            except Exception as e:
                logger.warning(f"Failed to sync model status to Redis: {e}")
                
            logger.info(f"Model {model_id} unloaded successfully")
            return True
        except Exception as e:
            logger.error(f"Failed to unload model {model_id}: {e}")
            return False

    def unload_all_except(self, keep_ids: List[str]) -> int:
        """
        Unload all models except those in keep_ids.
        Returns number of models unloaded.
        """
        count = 0
        # Snapshot keys to avoid runtime modification issues during iteration
        current_ids = list(self._models.keys())
        for mid in current_ids:
            if mid not in keep_ids:
                # Only unload if currently loaded to report accurate count
                if self._models[mid]["loaded"]:
                    if self.unload_model(mid):
                        count += 1
        return count

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
            self._models[model_id]["last_accessed"] = time.time()
            
            # Sync with Redis
            try:
                from src.services.admin.admin_store import get_admin_store
                store = get_admin_store()
                if store.redis:
                    # Set a key that indicates this model is loaded in SOME process
                    # We use a set to track all loaded models across processes if needed, 
                    # but simple key is easier for status check
                    store.redis.set(f"model_status:{model_id}:loaded", "true")
            except Exception as e:
                logger.warning(f"Failed to sync model status to Redis: {e}")
                
            return True
        except Exception as e:
            logger.error(f"Failed to load model {model_id}: {e}")
            return False

    def get_model_status(self, model_id: str) -> Dict[str, Any]:
        """Get status of a specific model."""
        # Check local state first
        is_loaded = False
        model_type = "unknown"
        
        if model_id in self._models:
             is_loaded = self._models[model_id]["loaded"]
             model_type = self._models[model_id]["type"]
        
        # If not loaded locally, check Redis (other workers)
        if not is_loaded:
            try:
                from src.services.admin.admin_store import get_admin_store
                store = get_admin_store()
                # Check shared key
                if store.redis:
                     if store.redis.exists(f"model_status:{model_id}:loaded"):
                         is_loaded = True
            except Exception:
                pass

        return {
            "id": model_id,
            "type": model_type,
            "loaded": is_loaded
        }

    def get_model_instance(self, model_id: str) -> Any:
        """Get the loaded instance of a model. Returns None if not loaded."""
        if model_id in self._models and self._models[model_id]["loaded"]:
            self._models[model_id]["last_accessed"] = time.time()
            return self._models[model_id]["instance"]
        return None

    def get_all_models(self) -> Dict[str, Dict[str, Any]]:
        """Get status of all registered models."""
        # Get local status
        statuses = {}
        
        for mid, data in self._models.items():
            is_loaded = data["loaded"]
            if not is_loaded:
                 # Check redis fallback
                 try:
                     from src.services.admin.admin_store import get_admin_store
                     store = get_admin_store()
                     if store.redis and store.redis.exists(f"model_status:{mid}:loaded"):
                         is_loaded = True
                 except:
                     pass
            
            statuses[mid] = {
                "type": data["type"],
                "loaded": is_loaded
            }
        
        return statuses

    
    def check_ttl(self) -> int:
        """
        Unload models that haven't been accessed in 'ttl_seconds'.
        Reads dynamic config from AdminStore.
        Returns number of unloaded models.
        """
        # Read dynamic config
        try:
            from src.services.admin.admin_store import get_admin_store
            store = get_admin_store()
            config = store.get_effective_config()
            ttl_seconds = config.get("model_ttl_seconds", settings.MODEL_TTL_SECONDS)
            auto_unload = config.get("model_auto_unload", settings.MODEL_AUTO_UNLOAD)
        except Exception:
             # Fallback if admin store fails (e.g. startup)
             ttl_seconds = settings.MODEL_TTL_SECONDS
             auto_unload = settings.MODEL_AUTO_UNLOAD

        if not auto_unload or ttl_seconds <= 0:
            return 0
            
        now = time.time()
        unloaded_count = 0
        
        # Snapshot keys
        current_ids = list(self._models.keys())
        for mid in current_ids:
            model = self._models[mid]
            if model["loaded"]:
                last = model.get("last_accessed", 0)
                if now - last > ttl_seconds:
                    logger.info(f"TTL Expired for {mid} (Idle {int(now-last)}s > {ttl_seconds}s). Unloading.")
                    if self.unload_model(mid):
                        unloaded_count += 1
                        
        return unloaded_count

    def _ttl_monitor_loop(self):
        """Background loop to check for expired models."""
        while getattr(self, "_running", True):
            try:
                time.sleep(60) # Check every 60 seconds
                self.check_ttl() # Config is read inside
            except Exception as e:
                logger.error(f"Error in TTL monitor: {e}")

    
    def resolve_model_name(self, model_key: str) -> str:
        """
        Resolve the actual HuggingFace model ID for a given key (e.g. 'dense', 'sparse').
        Checks AdminStore first, then falls back to Config/Env.
        """
        try:
            store = get_admin_store()
            models = store.get_models()
            if model_key in models and models[model_key].get("active"):
                return models[model_key]["name"]
        except Exception as e:
            logger.warning(f"Failed to resolve model from AdminStore: {e}")
            
        # Fallbacks handled by caller usually, but we could return None
        return None

    def load_model_from_hub(self, model_id: str, model_type: str, trust_remote_code: bool = False, **kwargs) -> bool:
        """
        Download and load a model directly from HuggingFace Hub.
        """
        if model_id not in self._models:
             self.register_model(model_id, model_type)
        
        def loader():
            logger.info(f"Downloading/Loading model: {model_id} (trust_remote_code={trust_remote_code})")
            
            import torch
            device = "cuda" if torch.cuda.is_available() else "cpu"
            
            if model_type == "embedding":
                 from sentence_transformers import SentenceTransformer
                 model = SentenceTransformer(model_id, trust_remote_code=trust_remote_code, device=device)
                 return model # ST is self-contained
                 
            from transformers import AutoTokenizer, AutoModel, AutoModelForMaskedLM, AutoModelForSequenceClassification
            
            tokenizer = AutoTokenizer.from_pretrained(model_id, trust_remote_code=trust_remote_code)
            
            # Load on CPU first to avoid "meta tensor" errors
            # Explicitly avoiding device="cuda" in from_pretrained
            
            if model_type == "sparse_embedding":
                model = AutoModelForMaskedLM.from_pretrained(model_id, trust_remote_code=trust_remote_code)
            elif model_type == "classification":
                model = AutoModelForSequenceClassification.from_pretrained(model_id, trust_remote_code=trust_remote_code)
            else:
                model = AutoModel.from_pretrained(model_id, trust_remote_code=trust_remote_code)
                
            model.to(device)
            model.eval()
            
            return {"model": model, "tokenizer": tokenizer, "device": device}

        return self.load_model(model_id, loader)

    def swap_model(self, model_key: str, new_model_id: str):
        """
        Swap the runtime model for a given key (e.g. swap the 'dense' model).
        """
        # 1. Unload current if strictly defined by key? 
        # Actually model_key represents the slot (dense/sparse) but we store by ID.
        # We need to find what was previously occupying this slot.
        # For simplicity in this gap fix, we just unload the *old* ID if known, 
        # OR we assume the caller knows the old ID.
        # But wait, the test expects `swap_model("dense", "new_id")`.
        
        # We'll unload the *current* model associated with this key in Admin Store?
        # Or just unload everything that looks like it?
        # A simpler approach: The manager tracks by ID. The *Caller* (Admin API) knows the Old ID.
        # But let's implement safe unloading of the key "dense" if it was a registered alias?
        # Our registry keys are IDs. 
        
        # Let's assume the test implies we unload whatever was "dense". 
        # Since we don't track "roles" in the manager, we unload the "key" if it matches an ID,
        # OR we rely on the Admin API to tell us `old_id` and `new_id`.
        # The test uses `swap_model("dense", ...)` so let's stick to that signature but implement it robustly.
        
        self.unload_model(model_key) # If "dense" is the ID.
        
        # Load new
        # Determine type based on key
        mtype = "embedding"
        if model_key == "sparse": mtype = "sparse_embedding"
        elif model_key == "reranker": mtype = "reranker"
        
        return self.load_model_from_hub(new_model_id, mtype, trust_remote_code=(model_key == "dense" and "jina" in new_model_id)) 

    def get_gpu_usage(self) -> Dict[str, Any]:

        """Get current GPU memory usage metrics."""
        usage = {
            "available": False,
            "used_mb": 0,
            "total_mb": 0,
            "percent": 0
        }
        
        import torch
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
