import requests
import time
import sys

KEYCLOAK_URL = "http://localhost:8080"
ADMIN_USER = "admin"
ADMIN_PASS = "admin"
REALM_NAME = "rice-search"
CLIENT_ID = "rice-search"

def get_admin_token():
    url = f"{KEYCLOAK_URL}/realms/master/protocol/openid-connect/token"
    payload = {
        "username": ADMIN_USER,
        "password": ADMIN_PASS,
        "grant_type": "password",
        "client_id": "admin-cli"
    }
    try:
        resp = requests.post(url, data=payload)
        resp.raise_for_status()
        return resp.json()["access_token"]
    except Exception as e:
        print(f"Error getting admin token: {e}")
        return None

def create_realm(token):
    url = f"{KEYCLOAK_URL}/admin/realms"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    payload = {
        "id": REALM_NAME,
        "realm": REALM_NAME,
        "enabled": True
    }
    resp = requests.post(url, json=payload, headers=headers)
    if resp.status_code == 201:
        print(f"Realm '{REALM_NAME}' created.")
    elif resp.status_code == 409:
        print(f"Realm '{REALM_NAME}' already exists.")
    else:
        print(f"Failed to create realm: {resp.text}")

def create_client(token):
    url = f"{KEYCLOAK_URL}/admin/realms/{REALM_NAME}/clients"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    payload = {
        "clientId": CLIENT_ID,
        "enabled": True,
        "directAccessGrantsEnabled": True,
        "standardFlowEnabled": True,
        "publicClient": True,
        "redirectUris": ["http://localhost:3000/*"],
        "webOrigins": ["*"]
    }
    resp = requests.post(url, json=payload, headers=headers)
    if resp.status_code == 201:
        print(f"Client '{CLIENT_ID}' created.")
    elif resp.status_code == 409:
        print(f"Client '{CLIENT_ID}' already exists.")
    else:
        print(f"Failed to create client: {resp.text}")

def main():
    print("Waiting for Keycloak...")
    for _ in range(30):
        try:
            if requests.get(KEYCLOAK_URL).status_code == 200:
                break
        except:
            time.sleep(2)
            print(".", end="", flush=True)
    print("\nKeycloak is up.")

    token = get_admin_token()
    if not token:
        sys.exit(1)

    create_realm(token)
    create_client(token)
    
    # Optionally create a demo user, but we can reuse 'admin' if we assign it to realm...
    # Actually, let's just use the realm default content. For testing, we might need a user in that realm.
    # But NextAuth Keycloak provider usually defaults to the realm, so we need a user IN rice-search realm.
    
    # Create User
    url = f"{KEYCLOAK_URL}/admin/realms/{REALM_NAME}/users"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    user_payload = {
        "username": "user",
        "enabled": True,
        "credentials": [{"type": "password", "value": "password", "temporary": False}],
        "firstName": "Demo",
        "lastName": "User"
    }
    resp = requests.post(url, json=user_payload, headers=headers)
    if resp.status_code == 201:
        print("User 'user' created with password 'password'.")
    elif resp.status_code == 409:
        print("User 'user' already exists.")

if __name__ == "__main__":
    main()
