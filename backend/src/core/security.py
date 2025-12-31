import jwt
import httpx
from jwt.algorithms import RSAAlgorithm
import json
from fastapi import HTTPException, status
import os

# Settings
KEYCLOAK_URL = os.getenv("KEYCLOAK_URL", "http://keycloak:8080")
REALM = os.getenv("KEYCLOAK_REALM", "rice-search")
ALGORITHM = "RS256"

# Cache
jwks_cache = None

async def get_public_keys():
    global jwks_cache
    if jwks_cache:
        return jwks_cache
    
    url = f"{KEYCLOAK_URL}/realms/{REALM}/protocol/openid-connect/certs"
    try:
        async with httpx.AsyncClient() as client:
            resp = await client.get(url)
            resp.raise_for_status()
            jwks_cache = resp.json()
            return jwks_cache
    except Exception as e:
        print(f"Failed to fetch JWKS: {e}")
        return None

def verify_token(token: str, jwks: dict) -> dict:
    if not jwks:
         raise HTTPException(
            status_code=status.HTTP_500_INTERNAL_SERVER_ERROR,
            detail="Auth configuration error"
        )
    
    try:
        # 1. Get Key ID (kid) from Header
        header = jwt.get_unverified_header(token)
        kid = header.get("kid")
        
        if not kid:
            raise Exception("No 'kid' in header")

        # 2. Find matching key in JWKS
        key_data = None
        for key in jwks.get("keys", []):
            if key.get("kid") == kid:
                key_data = key
                break
        
        if not key_data:
            raise Exception("Public key not found")

        # 3. Convert JWK to PEM Public Key
        public_key = RSAAlgorithm.from_jwk(json.dumps(key_data))

        # 4. Verify & Decode
        payload = jwt.decode(
            token,
            public_key, # PyJWT expects the key object here, not dict
            algorithms=[ALGORITHM],
            audience="account",
            options={"verify_aud": False}
        )
        return payload

    except jwt.ExpiredSignatureError:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Token expired",
            headers={"WWW-Authenticate": "Bearer"},
        )
    except Exception as e:
         raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Invalid token: {str(e)}",
            headers={"WWW-Authenticate": "Bearer"},
        )
