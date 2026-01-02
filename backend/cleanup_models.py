import logging
import json
from src.services.admin.admin_store import get_admin_store
from src.core.config import settings
from src.api.v1.endpoints.admin.public import get_protected_models

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def debug_protection():
    store = get_admin_store()
    models = store.get_models()
    protected = get_protected_models()
    
    logger.info(f"Protected Set: {protected}")
    
    defaults = [
        settings.EMBEDDING_MODEL,
        settings.SPARSE_MODEL,
        settings.RERANK_MODEL,
        settings.QUERY_UNDERSTANDING_MODEL
    ]
    logger.info(f"Settings Defaults: {defaults}")

    for mid, data in models.items():
        is_protected = mid in protected
        logger.info(f"Model {mid}: Protected={is_protected} (Name: {data.get('name')})")
        
        if data.get('name') in defaults and not is_protected:
            logger.error(f"MISMATCH: {mid} should be protected but is not!")
            
    # Cleanup Logic
    # Remove anything not in protected set? 
    # User said: "if there is any other model in the system... it can be removed"
    # So we strictly keep only the 4 defaults.
    
    for mid in list(models.keys()):
        if mid not in protected:
            logger.warning(f"Removing non-default model: {mid}")
            del models[mid]
            
    store.redis.set(store.MODELS_KEY, json.dumps(models))
    logger.info("Cleanup complete.")

if __name__ == "__main__":
    debug_protection()
