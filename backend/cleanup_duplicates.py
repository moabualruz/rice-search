import logging
import json
from src.services.admin.admin_store import get_admin_store
from src.core.config import settings

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def slugify(name: str) -> str:
    return name.replace("/", "-").lower()

def cleanup_duplicates():
    store = get_admin_store()
    models = store.get_models()
    logger.info(f"Found {len(models)} models before cleanup.")
    
    legacy_keys = {"dense", "sparse", "reranker", "classification"}
    new_models = {}
    
    # Track which types have an active model
    active_types = set()

    # Pass 1: Identify all unique models by name
    # Prioritize legacy keys for 'active' status if they exist
    
    # We want to map: Name -> Best Model Data
    name_map = {}
    
    for mid, data in models.items():
        name = data.get("name")
        if not name:
            continue
            
        # Initialize if new
        if name not in name_map:
            name_map[name] = data.copy()
            # Ensure proper ID format for the new entry
            name_map[name]["id"] = slugify(name)
        else:
            # Merge logic:
            # If current entry is active, and existing isn't, mark existing as active
            if data.get("active"):
                name_map[name]["active"] = True
            
            # Prefer gpu_enabled=True
            if data.get("gpu_enabled"):
                name_map[name]["gpu_enabled"] = True
                
        # If this was a legacy key, we might want to ensure we keep its metadata?
        # The main thing is preserving 'active' state, which we did above.
        
    # Rebuild the dictionary with proper IDs
    cleaned_models = {}
    for name, data in name_map.items():
        mid = slugify(name)
        data["id"] = mid # Enforce ID
        cleaned_models[mid] = data
        
    # Save back
    # We need to manually overwrite the entire hash in Redis to ensure deletions happen
    # AdminStore doesn't expose "replace all", so we use .redis directly
    store.redis.set(store.MODELS_KEY, json.dumps(cleaned_models))
    
    logger.info(f"Cleanup complete. Saved {len(cleaned_models)} models.")
    for m in cleaned_models.values():
        logger.info(f" - {m['id']} (Active: {m.get('active')})")

if __name__ == "__main__":
    cleanup_duplicates()
