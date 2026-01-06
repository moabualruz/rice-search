"""
Runtime Settings Management API.

Provides endpoints for viewing and updating settings at runtime.
All changes are persisted to Redis and optionally to settings.yaml.
"""
import logging
from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Any, Dict, Optional
from src.core.settings_manager import get_settings_manager
from src.api.deps import requires_role

logger = logging.getLogger(__name__)

router = APIRouter()


class SettingsBulkUpdate(BaseModel):
    """
    Bulk settings update request.

    All updates are automatically persisted to the YAML file.
    """
    settings: Dict[str, Any]


@router.get("/")
async def get_all_settings(prefix: Optional[str] = None):
    """
    Get all settings or settings with a specific prefix.

    Args:
        prefix: Optional prefix filter (e.g., "models" returns all model settings)

    Returns:
        Dictionary of settings
    """
    try:
        manager = get_settings_manager()
        settings = manager.get_all(prefix)
        return {
            "settings": settings,
            "count": len(settings),
            "version": manager.get_version()
        }
    except Exception as e:
        logger.error(f"Failed to get settings: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/{key:path}")
async def get_setting(key: str):
    """
    Get a specific setting by key.

    Args:
        key: Setting key in dot notation (e.g., "models.embedding.dimension")

    Returns:
        Setting value
    """
    try:
        manager = get_settings_manager()
        value = manager.get(key)

        if value is None:
            raise HTTPException(status_code=404, detail=f"Setting {key} not found")

        return {
            "key": key,
            "value": value
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to get setting {key}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.put("/{key:path}", dependencies=[Depends(requires_role("admin"))])
async def update_setting(key: str, update: dict):
    """
    Update a setting at runtime.

    All settings updates are automatically persisted to the YAML file.

    Args:
        key: Setting key in dot notation
        update: Update payload with 'value'

    Returns:
        Updated setting
    """
    try:
        manager = get_settings_manager()
        value = update.get("value")

        if value is None:
            raise HTTPException(status_code=400, detail="Missing 'value' in request body")

        # Update setting (always persist to file)
        manager.set(key, value, persist=True)

        logger.info(f"Setting updated and persisted: {key} = {value}")

        return {
            "message": "Setting updated and persisted to file",
            "key": key,
            "value": value,
            "persisted": True,
            "version": manager.get_version()
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to update setting {key}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/bulk", dependencies=[Depends(requires_role("admin"))])
async def bulk_update_settings(update: SettingsBulkUpdate):
    """
    Update multiple settings at once.

    All settings are automatically persisted to the YAML file after update.

    Args:
        update: Bulk update payload with settings dict

    Returns:
        Update summary
    """
    try:
        manager = get_settings_manager()
        updated_keys = []

        # Update all settings in memory first
        for key, value in update.settings.items():
            manager.set(key, value, persist=False)  # Update in Redis only
            updated_keys.append(key)

        # Persist all changes to file at once
        manager._persist_to_file()

        logger.info(f"Bulk update: {len(updated_keys)} settings updated and persisted")

        return {
            "message": f"{len(updated_keys)} settings updated and persisted to file",
            "updated_keys": updated_keys,
            "persisted": True,
            "version": manager.get_version()
        }
    except Exception as e:
        logger.error(f"Failed to bulk update settings: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.delete("/{key:path}", dependencies=[Depends(requires_role("admin"))])
async def delete_setting(key: str):
    """
    Delete a setting.

    Deletion is automatically persisted to the YAML file.

    Args:
        key: Setting key to delete

    Returns:
        Deletion confirmation
    """
    try:
        manager = get_settings_manager()

        # Check if setting exists
        if manager.get(key) is None:
            raise HTTPException(status_code=404, detail=f"Setting {key} not found")

        # Always persist deletion
        manager.delete(key, persist=True)

        logger.info(f"Setting deleted and persisted: {key}")

        return {
            "message": f"Setting {key} deleted and persisted to file",
            "persisted": True
        }
    except HTTPException:
        raise
    except Exception as e:
        logger.error(f"Failed to delete setting {key}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/reload", dependencies=[Depends(requires_role("admin"))])
async def reload_settings():
    """
    Reload settings from YAML file.

    WARNING: This discards all runtime changes not persisted to file.

    Returns:
        Reload confirmation
    """
    try:
        manager = get_settings_manager()
        manager.reload()

        logger.warning("Settings reloaded from file - runtime changes discarded")

        return {
            "message": "Settings reloaded from file",
            "version": manager.get_version()
        }
    except Exception as e:
        logger.error(f"Failed to reload settings: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/nested/{prefix}")
async def get_settings_nested(prefix: str):
    """
    Get settings as nested dictionary.

    Args:
        prefix: Prefix to filter (e.g., "models")

    Returns:
        Nested dictionary of settings
    """
    try:
        manager = get_settings_manager()
        settings = manager.get_nested(prefix)

        return {
            "prefix": prefix,
            "settings": settings
        }
    except Exception as e:
        logger.error(f"Failed to get nested settings for {prefix}: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/version/current")
async def get_settings_version():
    """
    Get current settings version.

    The version increments with each setting change, useful for
    detecting when settings have been updated.

    Returns:
        Current version number
    """
    try:
        manager = get_settings_manager()
        return {
            "version": manager.get_version()
        }
    except Exception as e:
        logger.error(f"Failed to get settings version: {e}")
        raise HTTPException(status_code=500, detail=str(e))
