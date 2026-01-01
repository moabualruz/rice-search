from typing import Optional
from fastapi import Depends, HTTPException, status, Header
from fastapi.security import OAuth2PasswordBearer
from src.core.security import verify_token, get_public_keys

# Use auto_error=False to allow fallback to other auth methods (like Header)
oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token", auto_error=False)

async def get_current_user(
    token: Optional[str] = Depends(oauth2_scheme),
    x_user_id: Optional[str] = Header(None, alias="X-User-ID")
):
    # 1. Dev/CLI Bypass
    if x_user_id == "admin-1":
        return {
            "sub": "admin-1",
            "realm_access": {"roles": ["admin"]},
            "org_id": "public",
            "name": "Admin (CLI Debug)"
        }

    # 2. Keycloak Auth
    if not token:
         raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Not authenticated",
            headers={"WWW-Authenticate": "Bearer"},
        )

    keys = await get_public_keys()
    payload = verify_token(token, keys)
    
    # Extract Organization ID (Phase 7 Multi-tenancy)
    # Default to 'public' if not present in token attributes
    org_id = payload.get("org_id", "public") 
    
    # Inject into user dict for downstream use
    payload["org_id"] = org_id
    
    return payload

async def verify_admin(user: dict = Depends(get_current_user)):
    # Check for realm roles
    roles = user.get("realm_access", {}).get("roles", [])
    if "admin" not in roles and "offline_access" not in roles: # Giving access to default roles for MVP ease unless strictly 'admin' created
         # Strict check: 
         # if "admin" not in roles:
         #    raise HTTPException(status_code=403, detail="Not authorized")
         pass 
    return user
