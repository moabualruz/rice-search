from fastapi import HTTPException, status
from jose import jwt, JWTError
import os
import httpx

# Settings - In production, these should be in config.py
KEYCLOAK_URL = os.getenv("KEYCLOAK_URL", "http://keycloak:8080")
REALM = os.getenv("KEYCLOAK_REALM", "rice-search")
ALGORITHM = "RS256"

# Cache for public keys
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
        # Verify signature using JWKS
        payload = jwt.decode(
            token,
            jwks,
            algorithms=[ALGORITHM],
            audience="account", # Default audience in Keycloak
            options={"verify_aud": False} # Relax audience check for MVP
        )
        return payload
    except JWTError as e:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail=f"Invalid token: {str(e)}",
            headers={"WWW-Authenticate": "Bearer"},
        )
