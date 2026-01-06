import requests
try:
    print("Checking root...")
    r = requests.get("http://localhost:8000/")
    print(r.status_code, r.text)
except Exception as e:
    print("Error:", e)
