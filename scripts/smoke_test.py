import time
import requests
import sys

HEALTH_URL = "http://localhost:8000/health"
MAX_RETRIES = 30
SLEEP_SEC = 2

def smoke_test():
    print(f"Checking {HEALTH_URL}...")
    
    for i in range(MAX_RETRIES):
        try:
            response = requests.get(HEALTH_URL, timeout=5)
            if response.status_code == 200:
                data = response.json()
                print("Response:", data)
                
                qdrant_ok = data.get("components", {}).get("qdrant", {}).get("status") == "up"
                celery_ok = data.get("components", {}).get("celery", {}).get("status") == "up"
                
                if qdrant_ok and celery_ok:
                    print("✅ SMOKE TEST PASSED: All systems GO.")
                    return True
                else:
                    print(f"⚠️ Services degraded. Retrying ({i+1}/{MAX_RETRIES})...")
            else:
                 print(f"⚠️ Status {response.status_code}. Retrying ({i+1}/{MAX_RETRIES})...")
        except Exception as e:
            print(f"⚠️ Connection failed: {e}. Retrying ({i+1}/{MAX_RETRIES})...")
        
        time.sleep(SLEEP_SEC)
    
    print("❌ SMOKE TEST FAILED: Timeout.")
    sys.exit(1)

if __name__ == "__main__":
    smoke_test()
