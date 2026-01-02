"""
API Dependencies for Authentication and RBAC.
"""
from typing import Generator, Optional, Callable
from fastapi import Depends, HTTPException, Header, status
import logging

from src.services.admin.admin_store import get_admin_store

logger = logging.getLogger(__name__)

async def get_current_user(
    x_user_id: Optional[str] = Header(None, alias="X-User-ID"),
    store = Depends(get_admin_store)
) -> dict:
    """
    Get current user from X-User-ID header (simulated auth for now).
    Verifies user exists in Redis.
    """
    from src.core.config import settings
    if not settings.AUTH_ENABLED:
        return {
            "id": "admin-local",
            "email": "admin@rice.local",
            "role": "admin",
            "org_id": "default",
            "active": True
        }

    if not x_user_id:
        # Check all headers manually if getting none
        # from fastapi import Request
        # but here we rely on Header
        # Default to anonymous/viewer if no header? 
        # Or force login? For admin API, let's enforce it or default to a read-only viewer.
        # For now, let's assume if missing, it's strictly unauthorized for admin actions.
        # But we might want to allow dev mode flexibility.
        # Let's return a strictly limited "viewer" user if no header is present, 
        # or raise 401. Given "Admin Mission Control", 401 is safer.
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Missing X-User-ID header"
        )

    # store is injected via Depends
    users = store.get_users()
    
    if x_user_id not in users:
        # Fallback for "admin-1" bootstrapping if Redis is empty or fresh
        if x_user_id == "admin-1" and not users:
             return {
                "id": "admin-1",
                "email": "admin@rice.local",
                "role": "admin",
                "org_id": "default",
                "active": True
            }

        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="User not found or inactive"
        )
    
    user = users[x_user_id]
    if not user.get("active", True):
        raise HTTPException(
            status_code=status.HTTP_403_FORBIDDEN,
            detail="User is inactive"
        )
        
    return user

def requires_role(required_role: str) -> Callable:
    """
    Dependency to enforce RBAC.
    Roles hierarchy: admin > member > viewer
    """
    def role_checker(user: dict = Depends(get_current_user)):
        user_role = user.get("role", "viewer")
        
        # Hierarchy definition
        hierarchy = {"admin": 3, "member": 2, "viewer": 1}
        
        if hierarchy.get(user_role, 0) < hierarchy.get(required_role, 0):
             raise HTTPException(
                status_code=status.HTTP_403_FORBIDDEN,
                detail=f"Insufficient permissions. Required: {required_role}"
            )
        return user
    return role_checker
