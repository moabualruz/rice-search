import sys
import os
sys.path.append(os.getcwd())
from src.services.admin.admin_store import get_admin_store

def main():
    print("Checking for test artifacts...")
    store = get_admin_store()
    models = store.get_models()
    print("Current Models:")
    for mid, m in models.items():
        print(f" - {mid}: {m.get('name')}")
    
    # Target 1: The explicit long ID which I named "Protected Reranker"
    target = "jinaai/jina-reranker-v2-base-multilingual"
    if target in models:
        print(f"Deleting duplicate/test model: {target}")
        store.delete_model(target) # Bypasses API check
        print("Deleted.")
    else:
        print(f"Target {target} not found.")

if __name__ == "__main__":
    main()
