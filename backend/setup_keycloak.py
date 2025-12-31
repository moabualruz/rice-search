import json
import time
import urllib.request
import urllib.error
import os

# Internal Docker URL
KEYCLOAK_URL = os.getenv("KEYCLOAK_URL", "http://keycloak:8080")
ADMIN_USER = "admin"
ADMIN_PASS = "admin"
REALM_NAME = "rice-search"
CLIENT_ID = "rice-search"

def request(method, url, data=None, headers=None):
    if headers is None:
        headers = {}
    if data:
        data = json.dumps(data).encode('utf-8')
        headers['Content-Type'] = 'application/json'
    
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req) as response:
            return response.status, json.loads(response.read().decode()) if response.read() else None
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()

def get_admin_token():
    url = f"{KEYCLOAK_URL}/realms/master/protocol/openid-connect/token"
    data = f"username={ADMIN_USER}&password={ADMIN_PASS}&grant_type=password&client_id=admin-cli"
    req = urllib.request.Request(url, data=data.encode(), method="POST")
    try:
        with urllib.request.urlopen(req) as response:
            return json.loads(response.read().decode())["access_token"]
    except Exception as e:
        print(f"Error getting token: {e}")
        return None

def main():
    print(f"Connecting to {KEYCLOAK_URL}...")
    for _ in range(30):
        try:
            urllib.request.urlopen(f"{KEYCLOAK_URL}", timeout=2)
            break
        except:
            time.sleep(2)
            print(".", end="", flush=True)
    print("\nKeycloak is up.")

    token = get_admin_token()
    if not token:
        print("Failed to get admin token")
        return

    headers = {"Authorization": f"Bearer {token}"}

    # Create Realm
    print(f"Creating realm '{REALM_NAME}'...")
    status, _ = request("POST", f"{KEYCLOAK_URL}/admin/realms", {
        "id": REALM_NAME,
        "realm": REALM_NAME,
        "enabled": True
    }, headers)
    if status == 201: print("Realm created.")
    elif status == 409: print("Realm exists.")
    else: print(f"Realm error: {status}")

    # Create Client
    print(f"Creating client '{CLIENT_ID}'...")
    status, _ = request("POST", f"{KEYCLOAK_URL}/admin/realms/{REALM_NAME}/clients", {
        "clientId": CLIENT_ID,
        "enabled": True,
        "directAccessGrantsEnabled": True,
        "standardFlowEnabled": True,
        "publicClient": True,
        "redirectUris": ["http://localhost:3000/*"],
        "webOrigins": ["*"]
    }, headers)
    if status == 201: print("Client created.")
    elif status == 409: print("Client exists.")
    else: print(f"Client error: {status}")

    # Create User
    print("Creating user 'user'...")
    status, _ = request("POST", f"{KEYCLOAK_URL}/admin/realms/{REALM_NAME}/users", {
        "username": "user",
        "enabled": True,
        "credentials": [{"type": "password", "value": "password", "temporary": False}],
        "firstName": "Demo",
        "lastName": "User"
    }, headers)
    if status == 201: print("User created.")
    elif status == 409: print("User exists.")
    else: print(f"User error: {status}")

if __name__ == "__main__":
    main()
