import time
import requests
import sys
import os

API_URL = "http://localhost:8000/api/v1"
HEALTH_URL = "http://localhost:8000/health"
INGEST_URL = f"{API_URL}/ingest/file"

MAX_RETRIES = 30
SLEEP_SEC = 2

def smoke_test():
    print(f"Checking {HEALTH_URL}...")
    
    # 1. Health Test
    health_ok = False
    for i in range(MAX_RETRIES):
        try:
            response = requests.get(HEALTH_URL, timeout=5)
            if response.status_code == 200:
                data = response.json()
                print("Health:", data)
                
                qdrant_ok = data.get("components", {}).get("qdrant", {}).get("status") == "up"
                celery_ok = data.get("components", {}).get("celery", {}).get("status") == "up"
                
                if qdrant_ok and celery_ok:
                    print("✅ HEALTH CHECK PASSED.")
                    health_ok = True
                    break
        except Exception:
            pass
        print(f"Waiting for services... ({i+1}/{MAX_RETRIES})")
        time.sleep(SLEEP_SEC)
    
    if not health_ok:
        print("❌ HEALTH CHECK FAILED.")
        sys.exit(1)

    # 2. Ingestion Test
    print(f"\nTesting Ingestion: {INGEST_URL}")
    dummy_file = "test_ingest.txt"
    with open(dummy_file, "w") as f:
        f.write("Rice Search is an open source enterprise neural search engine.")

    try:
        with open(dummy_file, "rb") as f:
            files = {"file": (dummy_file, f, "text/plain")}
            response = requests.post(INGEST_URL, files=files, timeout=10)
            
        print("Ingest Response:", response.status_code, response.text)
        
        if response.status_code == 200:
            print("✅ INGESTION ENDPOINT PASSED (Queued).")
            # In a real smoke test we'd poll the task ID, but for now proving API connectivity is mostly sufficient
            # ensuring imports like 'unstructured' didn't crash the app at startup.
        else:
            print("❌ INGESTION FAILED.")
            sys.exit(1)

    except Exception as e:
        print(f"❌ INGESTION EXCEPTION: {e}")
        sys.exit(1)
    finally:
        if os.path.exists(dummy_file):
            os.remove(dummy_file)

if __name__ == "__main__":
    smoke_test()
