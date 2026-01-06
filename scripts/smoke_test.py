import time
import requests
import sys
import os
import json

API_URL = "http://localhost:8000/api/v1"
HEALTH_URL = "http://localhost:8000/health"
INGEST_URL = f"{API_URL}/ingest/file"
SEARCH_URL = f"{API_URL}/search/query"

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
                if data.get("status") != "degraded": # Allow degraded if components checking logic is strict, but prefer fully up
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
        if response.status_code != 200:
             print("❌ INGESTION FAILED.")
             sys.exit(1)
        
        print("✅ INGESTION QUEUED. Waiting for processing...")
        time.sleep(10) # Wait for Celery

    except Exception as e:
        print(f"❌ INGESTION EXCEPTION: {e}")
        sys.exit(1)
    finally:
        if os.path.exists(dummy_file):
            os.remove(dummy_file)

    # 3. Search / RAG Test
    print(f"\nTesting Search: {SEARCH_URL}")
    query = {"query": "What is Rice Search?", "mode": "rag"}
    
    try:
        response = requests.post(SEARCH_URL, json=query, timeout=10)
        print("Search Response:", response.status_code, response.text)
        
        if response.status_code == 200:
            data = response.json()
            if "answer" in data:
                print("✅ RAG ENDPOINT PASSED.")
                print("Answer:", data["answer"])
            elif "results" in data:
                 print("✅ SEARCH ENDPOINT PASSED.")
            else:
                 print("⚠️ Unexpected response structure.")
        else:
             print("❌ SEARCH FAILED.")
             sys.exit(1)
             
    except Exception as e:
        print(f"❌ SEARCH EXCEPTION: {e}")
        sys.exit(1)

if __name__ == "__main__":
    smoke_test()
