import logging
import json
from src.services.admin.admin_store import get_admin_store
from src.core.config import settings

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def slugify(name: str) -> str:
    return name.replace("/", "-").lower()

def fix_model_types():
    store = get_admin_store()
    models = store.get_models()
    logger.info(f"Found {len(models)} models. Checking types...")
    
    updates = {}
    removals = []
    
    legacy_ids = {"dense", "sparse", "reranker", "classification"}
    query_model_names = {settings.QUERY_UNDERSTANDING_MODEL, getattr(settings, "QUERY_MODEL", "")}

    for mid, data in models.items():
        name = data.get("name", "")
        mtype = data.get("type")
        
        # 1. Remove Legacy
        if mid in legacy_ids:
            logger.info(f"Marking legacy model {mid} for removal.")
            removals.append(mid)
            continue
            
        # 2. Fix Query Understanding Type
        if name in query_model_names and mtype != "classification":
            logger.info(f"Fixing type for {name}: {mtype} -> classification")
            data["type"] = "classification"
            updates[mid] = data
            
        # 3. Ensure GPU Enabled Default
        if "gpu_enabled" not in data:
            logger.info(f"Enabling GPU for {name}")
            data["gpu_enabled"] = True
            updates[mid] = data
            
        # 4. Check for 'sentence-transformers/all-MiniLM-L6-v2' special case from user rant
        if "all-minilm-l6-v2" in name.lower() and mtype == "embedding":
            # If user INSISTS this is query understanding, we can swap it, 
            # BUT usually it is an embedding model. 
            # However, if the user explicitly mapped valid query models to this, do so.
            # For now, we will trust the config-based check above.
            pass

    # Apply Removals
    for mid in removals:
        if mid in models:
            del models[mid]
            
    # Apply Updates
    for mid, data in updates.items():
        models[mid] = data
        
    # Force add Query Understanding if missing
    qid = slugify(settings.QUERY_UNDERSTANDING_MODEL)
    if qid not in models:
        logger.info(f"Adding missing Query Understanding model: {qid}")
        models[qid] = {
            "id": qid,
            "name": settings.QUERY_UNDERSTANDING_MODEL,
            "type": "classification",
            "active": True,
            "gpu_enabled": True
        }

    # Save
    store.redis.set(store.MODELS_KEY, json.dumps(models))
    logger.info("Fix complete.")

if __name__ == "__main__":
    fix_model_types()
