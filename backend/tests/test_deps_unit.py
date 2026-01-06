"""
Unit tests for RBAC dependency logic.
"""
import pytest
from fastapi import HTTPException
from src.api.deps import requires_role

def test_requires_role_hierarchy():
    # Admin required
    check_admin = requires_role("admin")
    
    # Init user mocks
    admin_user = {"role": "admin"}
    member_user = {"role": "member"}
    viewer_user = {"role": "viewer"}
    
    # 1. Admin accessing Admin -> Pass
    assert check_admin(admin_user) == admin_user
    
    # 2. Member accessing Admin -> Fail
    with pytest.raises(HTTPException) as exc:
        check_admin(member_user)
    assert exc.value.status_code == 403
    
    # 3. Viewer accessing Admin -> Fail
    with pytest.raises(HTTPException) as exc:
        check_admin(viewer_user)
    assert exc.value.status_code == 403

def test_requires_role_member_endpoint():
    # Member required
    check_member = requires_role("member")
    
    # Admin accessing Member -> Pass (Hierarchy)
    assert check_member({"role": "admin"})
    
    # Member accessing Member -> Pass
    assert check_member({"role": "member"})
    
    # Viewer accessing Member -> Fail
    with pytest.raises(HTTPException) as exc:
        check_member({"role": "viewer"})
    assert exc.value.status_code == 403
