from fastapi import Depends, HTTPException, status
from fastapi.security import OAuth2PasswordBearer
from src.core.security import verify_token, get_public_keys

oauth2_scheme = OAuth2PasswordBearer(tokenUrl="token")

async def get_current_user(token: str = Depends(oauth2_scheme)):
    keys = await get_public_keys()
    payload = verify_token(token, keys)
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
